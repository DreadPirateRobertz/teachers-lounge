package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/teacherslounge/user-service/internal/auth"
	"github.com/teacherslounge/user-service/internal/billing"
	"github.com/teacherslounge/user-service/internal/cache"
	"github.com/teacherslounge/user-service/internal/config"
	"github.com/teacherslounge/user-service/internal/handlers"
	tlmetrics "github.com/teacherslounge/user-service/internal/metrics"
	"github.com/teacherslounge/user-service/internal/middleware"
	"github.com/teacherslounge/user-service/internal/models"
	"github.com/teacherslounge/user-service/internal/ratelimit"
	"github.com/teacherslounge/user-service/internal/store"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		slog.Error("loading config", "err", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// ── Data stores ─────────────────────────────────────────
	db, err := store.New(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("connecting to postgres", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	redisClient := cache.New(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB, cache.Options{
		TLSEnabled:    cfg.RedisTLSEnabled,
		TLSServerName: cfg.RedisTLSServerName,
	})
	if err := redisClient.Ping(context.Background()); err != nil {
		slog.Error("connecting to redis", "err", err)
		os.Exit(1)
	}
	defer func() {
		if err := redisClient.Close(); err != nil {
			slog.Error("closing redis client", "err", err)
		}
	}()

	// ── Service layer ────────────────────────────────────────
	jwtManager := auth.NewJWTManager(cfg.JWTSecret, cfg.AccessTokenDuration, cfg.RefreshTokenDuration)

	billingClient := billing.NewClient(
		cfg.StripeSecretKey,
		billing.PlanPrices{
			Monthly:    cfg.StripePriceMonthly,
			Quarterly:  cfg.StripePriceQuarterly,
			Semesterly: cfg.StripePriceSemesterly,
		},
		cfg.StripeWebhookSecret,
		db,
	)

	// ── Handlers ─────────────────────────────────────────────
	authH := handlers.NewAuthHandler(db, redisClient, jwtManager, billingClient, cfg)
	usersH := handlers.NewUsersHandler(db, redisClient)
	subsH := handlers.NewSubscriptionsHandler(db, billingClient)
	webhookH := handlers.NewWebhookHandler(billingClient)
	teachersH := handlers.NewTeachersHandler(db)
	adminH := handlers.NewAdminHandler(db)

	// ── Router ───────────────────────────────────────────────
	r := chi.NewRouter()
	r.Use(chimw.RealIP)
	r.Use(chimw.RequestID)
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(chimw.Timeout(30 * time.Second))
	r.Use(tlmetrics.HTTPMiddleware)

	// Health check (no auth)
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	// Prometheus metrics endpoint
	r.Handle("/metrics", promhttp.Handler())

	// Token-bucket rate limiter for unauthenticated registration (keyed by IP).
	regLimiter := ratelimit.New(redisClient.Cmdable())

	// Auth routes (no JWT required — these issue tokens)
	r.Route("/auth", func(r chi.Router) {
		r.With(middleware.IPRateLimit(regLimiter, ratelimit.BucketUserCreate)).Post("/register", authH.Register)
		r.Post("/login", authH.Login)
		r.Post("/refresh", authH.Refresh)
		r.Post("/logout", authH.Logout)
	})

	// Stripe webhook (Stripe signature auth, not JWT)
	r.Post("/webhooks/stripe", webhookH.StripeWebhook)

	// User routes (JWT required + self-only access)
	r.Route("/users/{id}", func(r chi.Router) {
		r.Use(middleware.Authenticate(jwtManager))
		r.Use(middleware.RequireSelf(func(req *http.Request) string {
			return chi.URLParam(req, "id")
		}))

		r.With(middleware.AuditLog(db, models.AuditActionReadProfile, "user_profile,learning_profile", "ferpa_compliance")).
			Get("/profile", usersH.GetProfile)
		r.Patch("/preferences", usersH.UpdatePreferences)
		r.Patch("/onboarding", usersH.CompleteOnboarding)
		r.Get("/export", usersH.DownloadExport)
		r.Post("/export", usersH.ExportData)
		r.Get("/export/{jobID}", usersH.GetExport)
		r.Delete("/", usersH.DeleteAccount)
		r.Get("/consent", usersH.GetConsent)
		r.Patch("/consent", usersH.UpdateConsent)

		r.With(middleware.AuditLog(db, models.AuditActionReadSubscription, "subscription", "billing_support")).
			Get("/subscription", subsH.GetSubscription)
		r.Post("/subscription/cancel", subsH.CancelSubscription)
		r.Post("/subscription/reactivate", subsH.ReactivateSubscription)

		// Consent management (FERPA K-12)
		r.Get("/consent", usersH.GetConsent)
		r.Patch("/consent", usersH.UpdateConsent)

		// Teacher profile (self-only; no teacher profile required to create one)
		r.Post("/teacher-profile", teachersH.CreateTeacherProfile)
		r.Get("/teacher-profile", teachersH.GetTeacherProfile)
	})

	// Admin routes (JWT + is_admin required + rate limited)
	r.Route("/admin", func(r chi.Router) {
		r.Use(middleware.Authenticate(jwtManager))
		r.Use(middleware.RequireAdmin(db))
		r.With(middleware.RateLimit(redisClient, "admin_audit", 100, 1*time.Hour)).
			Get("/audit", adminH.GetAuditLog)
	})

	// Teacher routes (JWT + self + teacher profile required)
	r.Route("/teachers/{id}", func(r chi.Router) {
		r.Use(middleware.Authenticate(jwtManager))
		r.Use(middleware.RequireSelf(func(req *http.Request) string {
			return chi.URLParam(req, "id")
		}))
		r.Use(middleware.RequireTeacherProfile(db))

		// Classes
		r.Post("/classes", teachersH.CreateClass)
		r.Get("/classes", teachersH.ListClasses)
		r.Get("/classes/{class_id}", teachersH.GetClass)
		r.Patch("/classes/{class_id}", teachersH.UpdateClass)
		r.Delete("/classes/{class_id}", teachersH.DeleteClass)

		// Roster
		r.Post("/classes/{class_id}/students", teachersH.AddStudent)
		r.Delete("/classes/{class_id}/students/{student_id}", teachersH.RemoveStudent)
		r.Get("/classes/{class_id}/students", teachersH.ListRoster)

		// Progress
		r.Get("/classes/{class_id}/students/{student_id}/progress", teachersH.GetStudentProgress)

		// Material assignments
		r.Post("/classes/{class_id}/materials", teachersH.AssignMaterial)
		r.Delete("/classes/{class_id}/materials/{material_id}", teachersH.UnassignMaterial)
		r.Get("/classes/{class_id}/materials", teachersH.ListAssignedMaterials)
	})

	// ── Server ───────────────────────────────────────────────
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Port),
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		slog.Info("user-service starting", "port", cfg.Port, "env", cfg.Env)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	// ── Graceful shutdown ────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down gracefully...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("graceful shutdown failed", "err", err)
	}
}
