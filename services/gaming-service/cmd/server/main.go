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

	"github.com/teacherslounge/gaming-service/internal/handler"
	"github.com/teacherslounge/gaming-service/internal/middleware"
	"github.com/teacherslounge/gaming-service/internal/ratelimit"
	"github.com/teacherslounge/gaming-service/internal/rival"
	"github.com/teacherslounge/gaming-service/internal/store"
)

func main() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	cfg := loadConfig()

	// Postgres
	pool, err := pgxpool.New(context.Background(), cfg.databaseURL)
	if err != nil {
		logger.Fatal("connect postgres", zap.Error(err))
	}
	defer pool.Close()

	// Redis
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.redisAddr,
		Password: cfg.redisPassword,
	})
	defer rdb.Close()

	if err := rdb.Ping(context.Background()).Err(); err != nil {
		logger.Fatal("connect redis", zap.Error(err))
	}

	st := store.New(pool, rdb)
	h := handler.New(st, logger)
	rl := ratelimit.New(rdb)

	// Seed simulated rivals into the leaderboard (idempotent — ZAddNX).
	if err := st.SeedRivals(context.Background(), rival.Roster); err != nil {
		logger.Warn("seed rivals", zap.Error(err))
	}
	// Tick rivals daily so they stay competitive over time.
	go tickRivalsDaily(st, logger)

	r := chi.NewRouter()
	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(chimw.Recoverer)
	r.Use(chimw.Timeout(30 * time.Second))

	r.Get("/health", h.Health)

	// Authenticated routes
	r.Group(func(r chi.Router) {
		r.Use(middleware.Authenticate(cfg.jwtSecret))

		r.With(middleware.RateLimit(rl, ratelimit.BucketXP, logger)).Post("/gaming/xp", h.GainXP)
		r.Get("/gaming/profile/{userId}", h.GetProfile)
		r.Post("/gaming/streak/checkin", h.StreakCheckin)
		r.Post("/gaming/leaderboard/update", h.LeaderboardUpdate)
		r.Get("/gaming/leaderboard", h.GetLeaderboard)
		r.Get("/gaming/leaderboard/friends", h.GetFriendLeaderboard)
		r.Get("/gaming/leaderboard/course/{courseId}", h.GetCourseLeaderboard)
		r.Get("/gaming/quotes/random", h.RandomQuote)

		// Quiz system
		r.With(middleware.RateLimit(rl, ratelimit.BucketQuizStart, logger)).Post("/gaming/quiz/start", h.StartQuiz)
		r.Get("/gaming/quiz/sessions/{sessionId}", h.GetQuizSession)
		r.With(middleware.RateLimit(rl, ratelimit.BucketQuizAnswer, logger)).Post("/gaming/quiz/sessions/{sessionId}/answer", h.SubmitAnswer)
		r.Get("/gaming/quiz/sessions/{sessionId}/hint", h.GetHint)
		r.Get("/gaming/quests/daily", h.GetDailyQuests)
		r.Post("/gaming/quests/progress", h.UpdateQuestProgress)

		// Boss battle routes
		r.Post("/gaming/boss/start", h.StartBattle)
		r.Get("/gaming/boss/session/{sessionId}", h.GetBattleSession)
		r.Post("/gaming/boss/attack", h.Attack)
		r.Post("/gaming/boss/powerup", h.ActivatePowerUp)
		r.Post("/gaming/boss/forfeit", h.ForfeitBattle)

		// Achievement / loot routes
		r.Get("/gaming/achievements/{userId}", h.GetAchievements)

		// Learning style assessment
		r.Post("/gaming/assessment/start", h.StartAssessment)
		r.Get("/gaming/assessment/sessions/{sessionId}", h.GetAssessmentSession)
		r.Post("/gaming/assessment/sessions/{sessionId}/answer", h.SubmitAssessmentAnswer)
	})

	srv := &http.Server{
		Addr:         ":" + cfg.port,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		logger.Info("gaming-service listening", zap.String("port", cfg.port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("listen", zap.Error(err))
		}
	}()

	<-done
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("shutdown", zap.Error(err))
	}
	logger.Info("gaming-service stopped")
}

type config struct {
	port          string
	databaseURL   string
	redisAddr     string
	redisPassword string
	jwtSecret     string
}

func loadConfig() config {
	return config{
		port:          getEnv("PORT", "8083"),
		databaseURL:   requireEnv("DATABASE_URL"),
		redisAddr:     getEnv("REDIS_ADDR", "localhost:6379"),
		redisPassword: getEnv("REDIS_PASSWORD", ""),
		jwtSecret:     requireEnv("JWT_SECRET"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func requireEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		panic(fmt.Sprintf("required environment variable %q is not set", key))
	}
	return v
}

// tickRivalsDaily advances all rival XP scores once every 24 hours.
// It runs as a background goroutine for the lifetime of the process.
func tickRivalsDaily(st *store.Store, logger *zap.Logger) {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	for range ticker.C {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := st.TickRivals(ctx, rival.Roster); err != nil {
			logger.Warn("tick rivals", zap.Error(err))
		}
		cancel()
	}
}
