package main

import (
	"log/slog"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"pipelineops/ingestion-service/internal/cache"
	"pipelineops/ingestion-service/internal/db"
	"pipelineops/ingestion-service/internal/handlers"
)

// Everything main() decides lives here rather than inside main() itself, so it
// can be tested. What's left in main.go is the part that genuinely can't be:
// binding a port, waiting on a signal, and exiting.

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// newRouter builds the full route table. It's separated from main so the
// wiring is covered by a test — a route registered at the wrong path or bound
// to the wrong handler is a real bug, and one that a compiler can't catch and
// a running service only reveals in production.
func newRouter(deps handlers.Deps, logger *slog.Logger) *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery(), slogRequestLogger(logger))

	router.GET("/healthz", handlers.Healthz(deps))
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	v1 := router.Group("/v1")
	{
		v1.POST("/heartbeat", deps.PostHeartbeat)
		v1.GET("/jobs/:job/heartbeat/latest", deps.GetLatestHeartbeat)
	}
	return router
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
