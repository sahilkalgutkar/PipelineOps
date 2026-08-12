// PipelineOps ingestion service — a small, high-throughput HTTP surface that
// receives job heartbeats and writes them to Postgres, updating Redis so
// dashboard reads stay fast. Runs independently of the Django core-api,
// sharing only the Postgres schema Django's migrations own.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"pipelineops/ingestion-service/internal/cache"
	"pipelineops/ingestion-service/internal/db"
	"pipelineops/ingestion-service/internal/handlers"
)

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	port := getenv("PORT", "8080")
	databaseURL := getenv("DATABASE_URL", "postgres://pipelineops:pipelineops@localhost:5432/pipelineops")
	redisURL := getenv("REDIS_URL", "redis://localhost:6379/0")
	ginMode := getenv("GIN_MODE", "release")
	gin.SetMode(ginMode)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := connectWithRetry(ctx, logger, databaseURL)
	if err != nil {
		logger.Error("could not connect to postgres", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	redisClient, err := connectRedisWithRetry(ctx, logger, redisURL)
	if err != nil {
		logger.Error("could not connect to redis", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := redisClient.Close(); err != nil {
			logger.Warn("error closing redis client", "error", err)
		}
	}()

	deps := handlers.Deps{DB: pool, Cache: redisClient, Logger: logger}

	router := gin.New()
	router.Use(gin.Recovery(), slogRequestLogger(logger))

	router.GET("/healthz", handlers.Healthz(deps))
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))
	v1 := router.Group("/v1")
	{
		v1.POST("/heartbeat", deps.PostHeartbeat)
		v1.GET("/jobs/:job/heartbeat/latest", deps.GetLatestHeartbeat)
	}

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("ingestion service listening", "port", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	logger.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
	}
}

func connectWithRetry(ctx context.Context, logger *slog.Logger, dsn string) (*db.Pool, error) {
	var lastErr error
	for i := 0; i < 15; i++ {
		pool, err := db.Connect(ctx, dsn)
		if err == nil {
			return pool, nil
		}
		lastErr = err
		logger.Warn("postgres not ready, retrying", "attempt", i+1, "error", err)
		time.Sleep(2 * time.Second)
	}
	return nil, lastErr
}

func connectRedisWithRetry(ctx context.Context, logger *slog.Logger, addr string) (*cache.Client, error) {
	var lastErr error
	for i := 0; i < 15; i++ {
		client, err := cache.Connect(ctx, addr)
		if err == nil {
			return client, nil
		}
		lastErr = err
		logger.Warn("redis not ready, retrying", "attempt", i+1, "error", err)
		time.Sleep(2 * time.Second)
	}
	return nil, lastErr
}

func slogRequestLogger(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		logger.Info("request",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"duration_ms", time.Since(start).Milliseconds(),
		)
	}
}
