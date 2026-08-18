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

	pool, err := connectWithRetry(logger, func() (*db.Pool, error) { return db.Connect(ctx, databaseURL) }, time.Sleep)
	if err != nil {
		logger.Error("could not connect to postgres", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	redisClient, err := connectRedisWithRetry(logger, func() (*cache.Client, error) { return cache.Connect(ctx, redisURL) }, time.Sleep)
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

// connectWithRetry calls connect up to 15 times, sleeping 2 seconds between
// attempts, until it succeeds or the attempts are exhausted. connect and
// sleep are injected so tests can substitute a fake connector and a
// non-blocking sleep; main passes the real db.Connect and time.Sleep.
func connectWithRetry(logger *slog.Logger, connect func() (*db.Pool, error), sleep func(time.Duration)) (*db.Pool, error) {
	var lastErr error
	for i := 0; i < 15; i++ {
		pool, err := connect()
		if err == nil {
			return pool, nil
		}
		lastErr = err
		logger.Warn("postgres not ready, retrying", "attempt", i+1, "error", err)
		sleep(2 * time.Second)
	}
	return nil, lastErr
}

// connectRedisWithRetry mirrors connectWithRetry for the Redis client.
func connectRedisWithRetry(logger *slog.Logger, connect func() (*cache.Client, error), sleep func(time.Duration)) (*cache.Client, error) {
	var lastErr error
	for i := 0; i < 15; i++ {
		client, err := connect()
		if err == nil {
			return client, nil
		}
		lastErr = err
		logger.Warn("redis not ready, retrying", "attempt", i+1, "error", err)
		sleep(2 * time.Second)
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
