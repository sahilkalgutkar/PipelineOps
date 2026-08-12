// Package db wraps the Postgres connection pool and the small set of raw SQL
// statements the ingestion service needs. It reads and writes the same
// jobs_job / jobs_heartbeat tables that the Django core-api owns migrations
// for — this service never runs migrations itself.
package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrJobNotFound = errors.New("job not found")

type Job struct {
	ID                      uuid.UUID
	Name                    string
	ExpectedIntervalSeconds int
	GracePeriodSeconds      int
	IsActive                bool
}

type Pool struct {
	pool *pgxpool.Pool
}

func Connect(ctx context.Context, dsn string) (*Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	cfg.MaxConns = 10
	pgxPool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	if err := pgxPool.Ping(ctx); err != nil {
		return nil, err
	}
	return &Pool{pool: pgxPool}, nil
}

func (p *Pool) Close() {
	p.pool.Close()
}

func (p *Pool) Ping(ctx context.Context) error {
	return p.pool.Ping(ctx)
}

// FindJobByNameOrID resolves a job from either its UUID (if identifier
// parses as one) or its unique name.
func (p *Pool) FindJobByNameOrID(ctx context.Context, identifier string) (*Job, error) {
	const baseQuery = `SELECT id, name, expected_interval_seconds, grace_period_seconds, is_active
		FROM jobs_job WHERE %s LIMIT 1`

	var row pgx.Row
	if id, err := uuid.Parse(identifier); err == nil {
		row = p.pool.QueryRow(ctx, fmt.Sprintf(baseQuery, "id = $1"), id)
	} else {
		row = p.pool.QueryRow(ctx, fmt.Sprintf(baseQuery, "name = $1"), identifier)
	}

	var j Job
	if err := row.Scan(&j.ID, &j.Name, &j.ExpectedIntervalSeconds, &j.GracePeriodSeconds, &j.IsActive); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrJobNotFound
		}
		return nil, err
	}
	return &j, nil
}

type InsertHeartbeatParams struct {
	JobID      uuid.UUID
	Status     string
	DurationMs *int
	Metadata   []byte // raw JSON
}

// InsertHeartbeat writes the heartbeat row and updates the job's
// denormalized last-heartbeat fields in a single transaction so dashboard
// reads never observe a heartbeat without the job status reflecting it.
func (p *Pool) InsertHeartbeat(ctx context.Context, params InsertHeartbeatParams) (int64, time.Time, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return 0, time.Time{}, err
	}
	defer tx.Rollback(ctx)

	receivedAt := time.Now().UTC()

	var heartbeatID int64
	err = tx.QueryRow(ctx, `
		INSERT INTO jobs_heartbeat (job_id, status, duration_ms, metadata, received_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id`,
		params.JobID, params.Status, params.DurationMs, params.Metadata, receivedAt,
	).Scan(&heartbeatID)
	if err != nil {
		return 0, time.Time{}, err
	}

	jobStatus := "healthy"
	if params.Status == "failure" {
		jobStatus = "failed"
	}

	_, err = tx.Exec(ctx, `
		UPDATE jobs_job
		SET last_heartbeat_at = $1, last_heartbeat_status = $2, updated_at = $1
		WHERE id = $3`,
		receivedAt, jobStatus, params.JobID,
	)
	if err != nil {
		return 0, time.Time{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, time.Time{}, err
	}
	return heartbeatID, receivedAt, nil
}
