package db

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
)

// testDSN returns the Postgres DSN to run integration tests against. This
// package wraps a concrete *pgxpool.Pool with no interface seam, so these
// behaviors can only be verified against a real database. They're skipped
// when one isn't reachable (e.g. in the CI job for this service, which
// doesn't run a Postgres container — core-api's Django tests own that).
func testDSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("INGESTION_TEST_DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://pipelineops:pipelineops@localhost:5432/pipelineops"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	pool, err := Connect(ctx, dsn)
	if err != nil {
		t.Skipf("skipping: no reachable postgres at %s: %v", dsn, err)
	}
	pool.Close()
	return dsn
}

func withTestJob(t *testing.T, pool *Pool, name string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	ctx := context.Background()
	_, err := pool.pool.Exec(ctx, `
		INSERT INTO jobs_job (id, name, description, schedule_description,
			expected_interval_seconds, grace_period_seconds, is_active,
			last_heartbeat_status, created_at, updated_at)
		VALUES ($1, $2, '', '', 3600, 60, true, 'unknown', now(), now())`,
		id, name,
	)
	if err != nil {
		t.Fatalf("failed to insert fixture job: %v", err)
	}
	t.Cleanup(func() {
		// Heartbeats FK-reference the job, so they must go first.
		_, _ = pool.pool.Exec(context.Background(), `DELETE FROM jobs_heartbeat WHERE job_id = $1`, id)
		_, _ = pool.pool.Exec(context.Background(), `DELETE FROM jobs_job WHERE id = $1`, id)
	})
	return id
}

func TestFindJobByNameOrID(t *testing.T) {
	dsn := testDSN(t)
	ctx := context.Background()
	pool, err := Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	// Registered before withTestJob's delete-fixture cleanup below, so it
	// runs LAST (t.Cleanup is LIFO) — the fixture must be deleted while the
	// pool is still open.
	t.Cleanup(pool.Close)

	jobID := withTestJob(t, pool, "db-test-nightly-etl")

	t.Run("finds by name", func(t *testing.T) {
		job, err := pool.FindJobByNameOrID(ctx, "db-test-nightly-etl")
		if err != nil {
			t.Fatalf("FindJobByNameOrID by name: %v", err)
		}
		if job.ID != jobID {
			t.Fatalf("job.ID = %s, want %s", job.ID, jobID)
		}
		if job.ExpectedIntervalSeconds != 3600 || job.GracePeriodSeconds != 60 || !job.IsActive {
			t.Fatalf("unexpected job fields: %+v", job)
		}
	})

	t.Run("finds by uuid", func(t *testing.T) {
		job, err := pool.FindJobByNameOrID(ctx, jobID.String())
		if err != nil {
			t.Fatalf("FindJobByNameOrID by id: %v", err)
		}
		if job.Name != "db-test-nightly-etl" {
			t.Fatalf("job.Name = %q, want %q", job.Name, "db-test-nightly-etl")
		}
	})

	t.Run("returns ErrJobNotFound for an unknown name", func(t *testing.T) {
		_, err := pool.FindJobByNameOrID(ctx, "does-not-exist")
		if !errors.Is(err, ErrJobNotFound) {
			t.Fatalf("err = %v, want ErrJobNotFound", err)
		}
	})

	t.Run("returns ErrJobNotFound for an unknown uuid", func(t *testing.T) {
		_, err := pool.FindJobByNameOrID(ctx, uuid.New().String())
		if !errors.Is(err, ErrJobNotFound) {
			t.Fatalf("err = %v, want ErrJobNotFound", err)
		}
	})
}

func TestInsertHeartbeat(t *testing.T) {
	dsn := testDSN(t)
	ctx := context.Background()
	pool, err := Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	t.Cleanup(pool.Close)

	jobID := withTestJob(t, pool, "db-test-insert-heartbeat")
	duration := 250

	id, receivedAt, err := pool.InsertHeartbeat(ctx, InsertHeartbeatParams{
		JobID:      jobID,
		Status:     "failure",
		DurationMs: &duration,
		Metadata:   []byte(`{"exit_code": 1}`),
	})
	if err != nil {
		t.Fatalf("InsertHeartbeat: %v", err)
	}
	if id == 0 {
		t.Fatal("InsertHeartbeat returned a zero heartbeat id")
	}
	if receivedAt.IsZero() {
		t.Fatal("InsertHeartbeat returned a zero received_at")
	}

	// InsertHeartbeat is documented to also update the job's denormalized
	// last_heartbeat_status/last_heartbeat_at fields in the same transaction.
	job, err := pool.FindJobByNameOrID(ctx, jobID.String())
	if err != nil {
		t.Fatalf("FindJobByNameOrID after insert: %v", err)
	}
	if job == nil {
		t.Fatal("job lookup after insert returned nil")
	}

	var status string
	var lastHeartbeatAt time.Time
	err = pool.pool.QueryRow(ctx,
		`SELECT last_heartbeat_status, last_heartbeat_at FROM jobs_job WHERE id = $1`, jobID,
	).Scan(&status, &lastHeartbeatAt)
	if err != nil {
		t.Fatalf("querying updated job row: %v", err)
	}
	if status != "failed" {
		t.Fatalf("last_heartbeat_status = %q, want %q (a 'failure' heartbeat maps to job status 'failed')", status, "failed")
	}
	if !lastHeartbeatAt.Equal(receivedAt) {
		t.Fatalf("last_heartbeat_at = %s, want %s", lastHeartbeatAt, receivedAt)
	}
}

func TestPing(t *testing.T) {
	dsn := testDSN(t)
	ctx := context.Background()
	pool, err := Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("Ping: %v", err)
	}
}

func TestConnect_InvalidDSN(t *testing.T) {
	_, err := Connect(context.Background(), "not-a-valid-dsn")
	if err == nil {
		t.Fatal("Connect with an invalid DSN should return an error")
	}
}
