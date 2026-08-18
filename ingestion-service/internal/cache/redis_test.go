package cache

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestKey(t *testing.T) {
	got := key("abc-123")
	want := "pipelineops:last_heartbeat:abc-123"
	if got != want {
		t.Fatalf("key(%q) = %q, want %q", "abc-123", got, want)
	}
}

// testRedisURL returns the Redis URL to run integration tests against. These
// tests need a real Redis instance (there is no interface to fake here — this
// package IS the Redis wrapper) so they're skipped when one isn't reachable,
// e.g. in the CI job for this service which doesn't run a Redis container.
func testRedisURL(t *testing.T) string {
	t.Helper()
	url := os.Getenv("INGESTION_TEST_REDIS_URL")
	if url == "" {
		url = "redis://localhost:6379/0"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	client, err := Connect(ctx, url)
	if err != nil {
		t.Skipf("skipping: no reachable redis at %s: %v", url, err)
	}
	_ = client.Close()
	return url
}

func TestClient_SetAndGetLastHeartbeat(t *testing.T) {
	url := testRedisURL(t)
	ctx := context.Background()

	client, err := Connect(ctx, url)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer func() { _ = client.Close() }()

	jobID := "11111111-1111-1111-1111-111111111111"
	receivedAt := time.Now().UTC().Truncate(time.Second)

	hb := LastHeartbeat{JobID: jobID, Status: "success", ReceivedAt: receivedAt}
	if err := client.SetLastHeartbeat(ctx, hb); err != nil {
		t.Fatalf("SetLastHeartbeat: %v", err)
	}

	got, err := client.GetLastHeartbeat(ctx, jobID)
	if err != nil {
		t.Fatalf("GetLastHeartbeat: %v", err)
	}
	if got == nil {
		t.Fatal("GetLastHeartbeat returned nil, want the heartbeat we just set")
	}
	if got.JobID != jobID || got.Status != "success" || !got.ReceivedAt.Equal(receivedAt) {
		t.Fatalf("GetLastHeartbeat = %+v, want JobID=%s Status=success ReceivedAt=%s", got, jobID, receivedAt)
	}
}

func TestClient_GetLastHeartbeat_MissReturnsNilNoError(t *testing.T) {
	url := testRedisURL(t)
	ctx := context.Background()

	client, err := Connect(ctx, url)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer func() { _ = client.Close() }()

	got, err := client.GetLastHeartbeat(ctx, "does-not-exist-job-id")
	if err != nil {
		t.Fatalf("GetLastHeartbeat should not error on a cache miss, got: %v", err)
	}
	if got != nil {
		t.Fatalf("GetLastHeartbeat = %+v, want nil on a cache miss", got)
	}
}

func TestClient_Ping(t *testing.T) {
	url := testRedisURL(t)
	ctx := context.Background()

	client, err := Connect(ctx, url)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer func() { _ = client.Close() }()

	if err := client.Ping(ctx); err != nil {
		t.Fatalf("Ping: %v", err)
	}
}

func TestConnect_InvalidURL(t *testing.T) {
	_, err := Connect(context.Background(), "not-a-valid-redis-url")
	if err == nil {
		t.Fatal("Connect with an invalid URL should return an error")
	}
}
