package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"pipelineops/ingestion-service/internal/cache"
	"pipelineops/ingestion-service/internal/db"
	"pipelineops/ingestion-service/internal/handlers"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestGetenv_ReturnsEnvValueWhenSet(t *testing.T) {
	t.Setenv("PIPELINEOPS_TEST_VAR", "from-env")
	if got := getenv("PIPELINEOPS_TEST_VAR", "fallback"); got != "from-env" {
		t.Fatalf("getenv = %q, want %q", got, "from-env")
	}
}

func TestGetenv_ReturnsFallbackWhenUnset(t *testing.T) {
	_ = os.Unsetenv("PIPELINEOPS_TEST_VAR_UNSET")
	if got := getenv("PIPELINEOPS_TEST_VAR_UNSET", "fallback"); got != "fallback" {
		t.Fatalf("getenv = %q, want %q", got, "fallback")
	}
}

func TestGetenv_ReturnsFallbackWhenEmptyString(t *testing.T) {
	// An explicitly-set-but-empty env var should behave like "unset" here —
	// getenv treats "" as absent, not as a deliberate empty override.
	t.Setenv("PIPELINEOPS_TEST_VAR_EMPTY", "")
	if got := getenv("PIPELINEOPS_TEST_VAR_EMPTY", "fallback"); got != "fallback" {
		t.Fatalf("getenv = %q, want %q", got, "fallback")
	}
}

func TestSlogRequestLogger_LogsMethodPathAndStatus(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	router := gin.New()
	router.Use(slogRequestLogger(logger))
	router.GET("/v1/jobs/:job/heartbeat/latest", func(c *gin.Context) {
		c.JSON(http.StatusTeapot, gin.H{"ok": true})
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/jobs/nightly-etl/heartbeat/latest", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusTeapot {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusTeapot)
	}

	logLine := buf.String()
	if !strings.Contains(logLine, `"method":"GET"`) {
		t.Fatalf("log line missing method: %s", logLine)
	}
	if !strings.Contains(logLine, `"path":"/v1/jobs/nightly-etl/heartbeat/latest"`) {
		t.Fatalf("log line missing path: %s", logLine)
	}
	if !strings.Contains(logLine, `"status":418`) {
		t.Fatalf("log line missing status 418: %s", logLine)
	}

	var parsed map[string]any
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("log line is not valid JSON: %v", err)
	}
	if _, ok := parsed["duration_ms"]; !ok {
		t.Fatalf("log line missing duration_ms field: %s", logLine)
	}
}

func TestSlogRequestLogger_CallsDownstreamHandler(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	handlerCalled := false
	router := gin.New()
	router.Use(slogRequestLogger(logger))
	router.POST("/v1/heartbeat", func(c *gin.Context) {
		handlerCalled = true
		c.Status(http.StatusCreated)
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/heartbeat", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if !handlerCalled {
		t.Fatal("slogRequestLogger did not call through to the downstream handler")
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusCreated)
	}
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
}

// noopSleep counts calls instead of actually blocking, so retry-loop tests
// run instantly regardless of the loop's real backoff duration.
func noopSleep(counter *int) func(time.Duration) {
	return func(time.Duration) {
		*counter++
	}
}

func TestConnectWithRetry_SucceedsOnFirstTry(t *testing.T) {
	wantPool := &db.Pool{}
	calls := 0
	var sleeps int

	pool, err := connectWithRetry(discardLogger(), func() (*db.Pool, error) {
		calls++
		return wantPool, nil
	}, noopSleep(&sleeps))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pool != wantPool {
		t.Fatalf("pool = %v, want %v", pool, wantPool)
	}
	if calls != 1 {
		t.Fatalf("connect called %d times, want 1", calls)
	}
	if sleeps != 0 {
		t.Fatalf("sleep called %d times, want 0", sleeps)
	}
}

func TestConnectWithRetry_SucceedsAfterNFailures(t *testing.T) {
	wantPool := &db.Pool{}
	calls := 0
	var sleeps int
	failuresBeforeSuccess := 4

	pool, err := connectWithRetry(discardLogger(), func() (*db.Pool, error) {
		calls++
		if calls <= failuresBeforeSuccess {
			return nil, errors.New("connection refused")
		}
		return wantPool, nil
	}, noopSleep(&sleeps))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pool != wantPool {
		t.Fatalf("pool = %v, want %v", pool, wantPool)
	}
	if calls != failuresBeforeSuccess+1 {
		t.Fatalf("connect called %d times, want %d", calls, failuresBeforeSuccess+1)
	}
	if sleeps != failuresBeforeSuccess {
		t.Fatalf("sleep called %d times, want %d", sleeps, failuresBeforeSuccess)
	}
}

func TestConnectWithRetry_ExhaustsRetriesAndReturnsLastError(t *testing.T) {
	calls := 0
	var sleeps int
	wantErr := errors.New("connection refused")

	pool, err := connectWithRetry(discardLogger(), func() (*db.Pool, error) {
		calls++
		return nil, wantErr
	}, noopSleep(&sleeps))

	if pool != nil {
		t.Fatalf("pool = %v, want nil", pool)
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
	if calls != 15 {
		t.Fatalf("connect called %d times, want 15", calls)
	}
	if sleeps != 15 {
		t.Fatalf("sleep called %d times, want 15", sleeps)
	}
}

func TestConnectRedisWithRetry_SucceedsOnFirstTry(t *testing.T) {
	wantClient := &cache.Client{}
	calls := 0
	var sleeps int

	client, err := connectRedisWithRetry(discardLogger(), func() (*cache.Client, error) {
		calls++
		return wantClient, nil
	}, noopSleep(&sleeps))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client != wantClient {
		t.Fatalf("client = %v, want %v", client, wantClient)
	}
	if calls != 1 {
		t.Fatalf("connect called %d times, want 1", calls)
	}
	if sleeps != 0 {
		t.Fatalf("sleep called %d times, want 0", sleeps)
	}
}

func TestConnectRedisWithRetry_SucceedsAfterNFailures(t *testing.T) {
	wantClient := &cache.Client{}
	calls := 0
	var sleeps int
	failuresBeforeSuccess := 7

	client, err := connectRedisWithRetry(discardLogger(), func() (*cache.Client, error) {
		calls++
		if calls <= failuresBeforeSuccess {
			return nil, errors.New("dial tcp: connection refused")
		}
		return wantClient, nil
	}, noopSleep(&sleeps))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client != wantClient {
		t.Fatalf("client = %v, want %v", client, wantClient)
	}
	if calls != failuresBeforeSuccess+1 {
		t.Fatalf("connect called %d times, want %d", calls, failuresBeforeSuccess+1)
	}
	if sleeps != failuresBeforeSuccess {
		t.Fatalf("sleep called %d times, want %d", sleeps, failuresBeforeSuccess)
	}
}

func TestConnectRedisWithRetry_ExhaustsRetriesAndReturnsLastError(t *testing.T) {
	calls := 0
	var sleeps int
	wantErr := errors.New("dial tcp: connection refused")

	client, err := connectRedisWithRetry(discardLogger(), func() (*cache.Client, error) {
		calls++
		return nil, wantErr
	}, noopSleep(&sleeps))

	if client != nil {
		t.Fatalf("client = %v, want nil", client)
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
	if calls != 15 {
		t.Fatalf("connect called %d times, want 15", calls)
	}
	if sleeps != 15 {
		t.Fatalf("sleep called %d times, want 15", sleeps)
	}
}

// --- route wiring -----------------------------------------------------------
//
// newRouter is the one part of startup where a mistake is invisible until
// production: a route registered at the wrong path, or bound to the wrong
// handler, compiles perfectly and fails only when a real client calls it.
// These tests assert the route table and then prove each route reaches the
// handler it claims to, by checking behaviour only that handler produces.

type stubDB struct {
	job     *db.Job
	findErr error
	pingErr error
}

func (s stubDB) FindJobByNameOrID(_ context.Context, _ string) (*db.Job, error) {
	return s.job, s.findErr
}

func (s stubDB) InsertHeartbeat(_ context.Context, _ db.InsertHeartbeatParams) (int64, time.Time, error) {
	return 1, time.Unix(0, 0).UTC(), nil
}

func (s stubDB) Ping(_ context.Context) error { return s.pingErr }

type stubCache struct {
	hb      *cache.LastHeartbeat
	pingErr error
}

func (s stubCache) SetLastHeartbeat(_ context.Context, _ cache.LastHeartbeat) error { return nil }

func (s stubCache) GetLastHeartbeat(_ context.Context, _ string) (*cache.LastHeartbeat, error) {
	return s.hb, nil
}

func (s stubCache) Ping(_ context.Context) error { return s.pingErr }

func testDeps(d stubDB, c stubCache) handlers.Deps {
	return handlers.Deps{DB: d, Cache: c, Logger: discardLogger()}
}

func TestNewRouter_RegistersEveryRoute(t *testing.T) {
	router := newRouter(testDeps(stubDB{}, stubCache{}), discardLogger())

	registered := map[string]bool{}
	for _, r := range router.Routes() {
		registered[r.Method+" "+r.Path] = true
	}

	for _, want := range []string{
		"GET /healthz",
		"GET /metrics",
		"POST /v1/heartbeat",
		"GET /v1/jobs/:job/heartbeat/latest",
	} {
		if !registered[want] {
			t.Errorf("route %q is not registered; got %v", want, registered)
		}
	}
	if len(router.Routes()) != 4 {
		t.Errorf("router has %d routes, want exactly 4 — an unintended route is as much a bug as a missing one", len(router.Routes()))
	}
}

func TestNewRouter_HealthzReachesTheHealthHandler(t *testing.T) {
	t.Run("healthy when both dependencies answer", func(t *testing.T) {
		router := newRouter(testDeps(stubDB{}, stubCache{}), discardLogger())
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}
		if !strings.Contains(rec.Body.String(), `"status":"ok"`) {
			t.Fatalf("body = %s", rec.Body.String())
		}
	})

	// A 503 here can only come from Healthz itself, which is what makes this
	// a test of the binding rather than of the path.
	t.Run("degraded when postgres is down", func(t *testing.T) {
		router := newRouter(testDeps(stubDB{pingErr: errors.New("down")}, stubCache{}), discardLogger())
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
		}
		if !strings.Contains(rec.Body.String(), `"db":"down"`) {
			t.Fatalf("body = %s", rec.Body.String())
		}
	})
}

func TestNewRouter_MetricsServesPrometheus(t *testing.T) {
	router := newRouter(testDeps(stubDB{}, stubCache{}), discardLogger())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !strings.Contains(rec.Body.String(), "# HELP") {
		t.Fatalf("/metrics did not return a Prometheus exposition body: %s", rec.Body.String()[:200])
	}
}

func TestNewRouter_HeartbeatRouteReachesPostHeartbeat(t *testing.T) {
	router := newRouter(testDeps(stubDB{}, stubCache{}), discardLogger())

	// An empty body fails PostHeartbeat's binding on the required "job"
	// field. Only that handler can produce this 400.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/heartbeat", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestNewRouter_LatestHeartbeatRouteBindsTheJobParam(t *testing.T) {
	jobID := uuid.New()
	router := newRouter(
		testDeps(stubDB{job: &db.Job{ID: jobID, Name: "nightly-etl"}}, stubCache{}),
		discardLogger(),
	)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/jobs/nightly-etl/heartbeat/latest", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	// The handler echoes the resolved job id, so seeing it here proves the
	// :job path segment was actually bound and passed through.
	if !strings.Contains(rec.Body.String(), jobID.String()) {
		t.Fatalf("body = %s, want it to contain the resolved job id %s", rec.Body.String(), jobID)
	}
	if got := rec.Header().Get("X-Cache"); got != "MISS" {
		t.Fatalf("X-Cache = %q, want MISS", got)
	}
}

func TestNewRouter_UnknownRouteIs404(t *testing.T) {
	router := newRouter(testDeps(stubDB{}, stubCache{}), discardLogger())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/nope", nil))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}
