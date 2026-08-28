// PipelineOps ingestion service — a small, high-throughput HTTP surface that
// receives job heartbeats and writes them to Postgres, updating Redis so
// dashboard reads stay fast. Runs independently of the Django core-api,
// sharing only the Postgres schema Django's migrations own.
//
// main() is deliberately nothing but wiring: read the environment, dial the
// two dependencies, serve, and shut down cleanly. Every decision it makes
// lives in bootstrap.go, where it is tested.
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

	"pipelineops/ingestion-service/internal/cache"
	"pipelineops/ingestion-service/internal/db"
	"pipelineops/ingestion-service/internal/handlers"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	port := getenv("PORT", "8080")
	databaseURL := getenv("DATABASE_URL", "postgres://pipelineops:pipelineops@localhost:5432/pipelineops")
	redisURL := getenv("REDIS_URL", "redis://localhost:6379/0")
	gin.SetMode(getenv("GIN_MODE", "release"))

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

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           newRouter(handlers.Deps{DB: pool, Cache: redisClient, Logger: logger}, logger),
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
