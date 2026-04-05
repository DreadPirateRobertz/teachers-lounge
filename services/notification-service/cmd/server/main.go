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
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/DreadPirateRobertz/teachers-lounge/services/notification-service/internal/handler"
	"github.com/DreadPirateRobertz/teachers-lounge/services/notification-service/internal/middleware"
	"github.com/DreadPirateRobertz/teachers-lounge/services/notification-service/internal/push"
	"github.com/DreadPirateRobertz/teachers-lounge/services/notification-service/internal/store"
)

func main() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

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
	defer rdb.Close()

	// ── Dependencies ─────────────────────────────────────────────────────────
	notifStore := store.New(db)
	rateLimiter := store.NewRateLimiter(rdb)

	// Use FCMPusher when a server key is configured; fall back to LogPusher so
	// the rest of the stack works without Firebase credentials (local dev).
	var pusher push.Pusher
	if cfg.fcmServerKey != "" {
		pusher = push.NewFCMPusher(cfg.fcmServerKey)
		logger.Info("FCM push enabled")
	} else {
		pusher = push.LogPusher{}
		logger.Warn("FCM_SERVER_KEY not set — push notifications will not be delivered")
	}

	h := handler.New(notifStore, pusher, logger)

	// ── Router ────────────────────────────────────────────────────────────────
	r := chi.NewRouter()
	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(chimw.Timeout(30 * time.Second))

	r.Get("/health", h.Health)

	r.Route("/notify", func(r chi.Router) {
		// Stub auth middleware: reads X-User-ID header and injects into context.
		// Replace with real JWT validation once user-service is wired.
		r.Use(stubAuthMiddleware)

		// Push — rate limited; token registration
		r.With(middleware.PushRateLimit(rateLimiter)).Post("/push", h.Push)
		r.Post("/push/token", h.RegisterToken)

		// Email — no rate limit at stub stage
		r.Post("/email", h.Email)

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
	port          string
	databaseURL   string
	redisAddr     string
	redisPassword string
	fcmServerKey  string
}

func configFromEnv() config {
	return config{
		port:          envOr("PORT", "9000"),
		databaseURL:   envOr("DATABASE_URL", "postgres://tl_app:localdevpassword@postgres:5432/teacherslounge"),
		redisAddr:     envOr("REDIS_ADDR", "redis:6379"),
		redisPassword: envOr("REDIS_PASSWORD", "localredispassword"),
		fcmServerKey:  os.Getenv("FCM_SERVER_KEY"), // optional; empty = LogPusher
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
