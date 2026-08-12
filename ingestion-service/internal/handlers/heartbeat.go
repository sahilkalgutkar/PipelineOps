package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"pipelineops/ingestion-service/internal/cache"
	"pipelineops/ingestion-service/internal/db"
	"pipelineops/ingestion-service/internal/metrics"
)

// DBClient is the subset of *db.Pool the handlers need. Declared here
// (rather than accepting *db.Pool directly) so tests can supply a fake
// without touching Postgres.
type DBClient interface {
	FindJobByNameOrID(ctx context.Context, identifier string) (*db.Job, error)
	InsertHeartbeat(ctx context.Context, params db.InsertHeartbeatParams) (int64, time.Time, error)
	Ping(ctx context.Context) error
}

// CacheClient is the subset of *cache.Client the handlers need, mirroring DBClient.
type CacheClient interface {
	SetLastHeartbeat(ctx context.Context, hb cache.LastHeartbeat) error
	GetLastHeartbeat(ctx context.Context, jobID string) (*cache.LastHeartbeat, error)
	Ping(ctx context.Context) error
}

type Deps struct {
	DB     DBClient
	Cache  CacheClient
	Logger *slog.Logger
}

type heartbeatRequest struct {
	Job        string         `json:"job" binding:"required"` // job UUID or unique name
	Status     string         `json:"status"`                 // "success" (default) or "failure"
	DurationMs *int           `json:"duration_ms"`
	Metadata   map[string]any `json:"metadata"`
}

type heartbeatResponse struct {
	HeartbeatID int64     `json:"heartbeat_id"`
	JobID       string    `json:"job_id"`
	Status      string    `json:"status"`
	ReceivedAt  time.Time `json:"received_at"`
}

func (d Deps) PostHeartbeat(c *gin.Context) {
	timer := prometheusTimer()
	defer timer()

	var req heartbeatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		metrics.HeartbeatsRejected.WithLabelValues("invalid_body").Inc()
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Status == "" {
		req.Status = "success"
	}
	if req.Status != "success" && req.Status != "failure" {
		metrics.HeartbeatsRejected.WithLabelValues("invalid_status").Inc()
		c.JSON(http.StatusBadRequest, gin.H{"error": "status must be 'success' or 'failure'"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	job, err := d.DB.FindJobByNameOrID(ctx, req.Job)
	if err != nil {
		if errors.Is(err, db.ErrJobNotFound) {
			metrics.HeartbeatsRejected.WithLabelValues("job_not_found").Inc()
			c.JSON(http.StatusNotFound, gin.H{"error": "job not found: " + req.Job})
			return
		}
		d.Logger.Error("lookup job failed", "error", err)
		metrics.HeartbeatsRejected.WithLabelValues("db_error").Inc()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	metadataJSON, err := json.Marshal(req.Metadata)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid metadata"})
		return
	}

	heartbeatID, receivedAt, err := d.DB.InsertHeartbeat(ctx, db.InsertHeartbeatParams{
		JobID:      job.ID,
		Status:     req.Status,
		DurationMs: req.DurationMs,
		Metadata:   metadataJSON,
	})
	if err != nil {
		d.Logger.Error("insert heartbeat failed", "error", err, "job", job.Name)
		metrics.HeartbeatsRejected.WithLabelValues("db_error").Inc()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	metrics.HeartbeatsTotal.WithLabelValues(req.Status).Inc()

	if err := d.Cache.SetLastHeartbeat(ctx, cache.LastHeartbeat{
		JobID:      job.ID.String(),
		Status:     req.Status,
		ReceivedAt: receivedAt,
	}); err != nil {
		// Cache is best-effort: log and continue, Postgres is the source of truth.
		d.Logger.Warn("redis cache update failed", "error", err, "job", job.Name)
	}

	c.JSON(http.StatusCreated, heartbeatResponse{
		HeartbeatID: heartbeatID,
		JobID:       job.ID.String(),
		Status:      req.Status,
		ReceivedAt:  receivedAt,
	})
}

func (d Deps) GetLatestHeartbeat(c *gin.Context) {
	jobIdentifier := c.Param("job")
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	job, err := d.DB.FindJobByNameOrID(ctx, jobIdentifier)
	if err != nil {
		if errors.Is(err, db.ErrJobNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	if hb, err := d.Cache.GetLastHeartbeat(ctx, job.ID.String()); err == nil && hb != nil {
		c.Header("X-Cache", "HIT")
		c.JSON(http.StatusOK, hb)
		return
	}

	c.Header("X-Cache", "MISS")
	c.JSON(http.StatusOK, gin.H{"job_id": job.ID.String(), "status": "unknown"})
}

func Healthz(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()

		if err := deps.DB.Ping(ctx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "degraded", "db": "down"})
			return
		}
		if err := deps.Cache.Ping(ctx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "degraded", "cache": "down"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
}

func prometheusTimer() func() {
	start := time.Now()
	return func() {
		metrics.IngestDuration.Observe(time.Since(start).Seconds())
	}
}
