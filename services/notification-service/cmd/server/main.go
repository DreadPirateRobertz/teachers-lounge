package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/DreadPirateRobertz/teachers-lounge/services/notification-service/internal/auth"
	"github.com/DreadPirateRobertz/teachers-lounge/services/notification-service/internal/email"
	"github.com/DreadPirateRobertz/teachers-lounge/services/notification-service/internal/handler"
	"github.com/DreadPirateRobertz/teachers-lounge/services/notification-service/internal/handlers"
	"github.com/DreadPirateRobertz/teachers-lounge/services/notification-service/internal/hub"
	tlmetrics "github.com/DreadPirateRobertz/teachers-lounge/services/notification-service/internal/metrics"
	"github.com/DreadPirateRobertz/teachers-lounge/services/notification-service/internal/middleware"
	"github.com/DreadPirateRobertz/teachers-lounge/services/notification-service/internal/push"
	"github.com/DreadPirateRobertz/teachers-lounge/services/notification-service/internal/store"
)

func main() {
	logger, _ := zap.NewProduction()
	defer func() { _ = logger.Sync() }()

	cfg := configFromEnv()

	// ── Postgres ─────────────────────────────────────────────────────────────
	ctx := context.Background()
	db, err := pgxpool.New(ctx, cfg.databaseURL)
	if err != nil {
		logger.Fatal("connect postgres", zap.Error(err))
	}
	defer db.Close()

	if err := store.Migrate(ctx, db); err != nil {
		logger.Fatal("migrate", zap.Error(err))
	}

	// ── Redis ─────────────────────────────────────────────────────────────────
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.redisAddr,
		Password: cfg.redisPassword,
	})
	if err := rdb.Ping(ctx).Err(); err != nil {
		logger.Fatal("connect redis", zap.Error(err))
	}
	defer func() { _ = rdb.Close() }() //nolint:errcheck // best-effort cleanup on shutdown

	// ── Dependencies ─────────────────────────────────────────────────────────
	notifStore := store.New(db)
	rateLimiter := store.NewRateLimiter(rdb)

	// FCM push — use real sender when FCM_SERVER_KEY is configured.
	var pusher push.Pusher
	if cfg.fcmServerKey != "" {
		pusher = push.NewFCMPusher(cfg.fcmServerKey)
		logger.Info("FCM push enabled")
	} else {
		pusher = push.LogPusher{}
		logger.Warn("FCM_SERVER_KEY not set — push notifications will not be delivered")
	}

	// Email — use SendGrid when SENDGRID_API_KEY is configured.
	var emailer email.Sender
	if cfg.sendgridAPIKey != "" {
		fromAddr := cfg.fromEmail
		if fromAddr == "" {
			fromAddr = "noreply@teacherslounge.app"
		}
		emailer = email.NewSendGridSender(cfg.sendgridAPIKey, fromAddr)
		logger.Info("SendGrid email enabled", zap.String("from", fromAddr))
	} else {
		emailer = email.LogSender{}
		logger.Warn("SENDGRID_API_KEY not set — emails will not be delivered")
	}

	h := handler.New(notifStore, pusher, emailer, rateLimiter, logger)

	// ── WebSocket hub ─────────────────────────────────────────────────────────
	wsHub := hub.New()

	// JWT authenticator for WebSocket connections.
	jwtAuth, err := auth.NewJWTAuthenticator([]byte(cfg.jwtSecret))
	if err != nil {
		logger.Fatal("jwt authenticator", zap.Error(err))
	}
	wsHandler := handlers.NewWSHandler(wsHub, jwtAuth, logger)
	notifyHandler := handlers.NewNotifyHandler(wsHub, logger)

	// ── Router ────────────────────────────────────────────────────────────────
	r := chi.NewRouter()
	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(chimw.Timeout(30 * time.Second))
	r.Use(tlmetrics.HTTPMiddleware)

	r.Get("/health", h.Health)
	r.Handle("/metrics", promhttp.Handler())

	// WebSocket endpoint — no timeout middleware (long-lived connection).
	r.Get("/ws", wsHandler.ServeHTTP)

	// Internal endpoints — called by peer services (gaming-service, tutoring-service).
	// Not exposed via the public ingress; secured at the network layer.
	r.Route("/internal", func(r chi.Router) {
		r.Post("/notify/boss-unlock", notifyHandler.BossUnlock)
	})

	r.Route("/notify", func(r chi.Router) {
		// Stub auth middleware: reads X-User-ID header and injects into context.
		// Replace with real JWT validation once user-service is wired.
		r.Use(stubAuthMiddleware)

		// Push — rate limited; token registration
		r.With(middleware.PushRateLimit(rateLimiter)).Post("/push", h.Push)
		r.Post("/push/token", h.RegisterToken)

		// Email — SendGrid delivery
		r.Post("/email", h.Email)

		// Event trigger — push + email + in-app from a single game event
		// Rate limit is enforced inside the handler (not middleware) so the
		// trigger can apply it per-user without requiring auth context.
		r.Post("/trigger", h.Trigger)

		// In-app — Postgres backed
		r.Post("/in-app", h.InApp)
		r.Get("/{userId}", h.ListUnread)
	})

	// ── Server ────────────────────────────────────────────────────────────────
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.port),
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		logger.Info("notification-service starting", zap.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("listen", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		logger.Error("graceful shutdown", zap.Error(err))
	}
	logger.Info("notification-service stopped")
}

// stubAuthMiddleware reads X-User-ID from the request header and injects it
// into the context. Replace with JWT validation in Phase 2.
func stubAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := r.Header.Get("X-User-ID")
		if userID == "" {
			http.Error(w, "X-User-ID header required", http.StatusUnauthorized)
			return
		}
		ctx := middleware.WithUserID(r.Context(), userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

type config struct {
	port           string
	databaseURL    string
	redisAddr      string
	redisPassword  string
	fcmServerKey   string
	sendgridAPIKey string
	fromEmail      string
	jwtSecret      string
}

func configFromEnv() config {
	return config{
		port:           envOr("PORT", "9000"),
		databaseURL:    envOr("DATABASE_URL", "postgres://tl_app:localdevpassword@postgres:5432/teacherslounge"),
		redisAddr:      envOr("REDIS_ADDR", "redis:6379"),
		redisPassword:  envOr("REDIS_PASSWORD", "localredispassword"),
		fcmServerKey:   os.Getenv("FCM_SERVER_KEY"),   // optional; empty = LogPusher
		sendgridAPIKey: os.Getenv("SENDGRID_API_KEY"), // optional; empty = LogSender
		fromEmail:      os.Getenv("FROM_EMAIL"),       // optional; defaults to noreply@teacherslounge.app
		// JWT_SECRET signs WebSocket auth tokens. Falls back to a dev-only default;
		// must be set in production via environment or secret manager.
		jwtSecret: envOr("JWT_SECRET", "dev-secret-change-me-in-production"),
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
