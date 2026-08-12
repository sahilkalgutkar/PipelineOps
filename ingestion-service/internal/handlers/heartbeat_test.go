package handlers

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"pipelineops/ingestion-service/internal/cache"
	"pipelineops/ingestion-service/internal/db"
)

func init() {
	gin.SetMode(gin.TestMode)
}

var errBoom = errors.New("boom")

type fakeDB struct {
	job          *db.Job
	findErr      error
	insertID     int64
	insertAt     time.Time
	insertErr    error
	pingErr      error
	insertCalled bool
}

func (f *fakeDB) FindJobByNameOrID(_ context.Context, _ string) (*db.Job, error) {
	if f.findErr != nil {
		return nil, f.findErr
	}
	return f.job, nil
}

func (f *fakeDB) InsertHeartbeat(_ context.Context, _ db.InsertHeartbeatParams) (int64, time.Time, error) {
	f.insertCalled = true
	if f.insertErr != nil {
		return 0, time.Time{}, f.insertErr
	}
	return f.insertID, f.insertAt, nil
}

func (f *fakeDB) Ping(_ context.Context) error { return f.pingErr }

type fakeCache struct {
	setErr  error
	getResp *cache.LastHeartbeat
	getErr  error
	pingErr error
}

func (f *fakeCache) SetLastHeartbeat(_ context.Context, _ cache.LastHeartbeat) error {
	return f.setErr
}

func (f *fakeCache) GetLastHeartbeat(_ context.Context, _ string) (*cache.LastHeartbeat, error) {
	return f.getResp, f.getErr
}

func (f *fakeCache) Ping(_ context.Context) error { return f.pingErr }

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newTestJob() *db.Job {
	return &db.Job{
		ID:                      uuid.New(),
		Name:                    "nightly-etl",
		ExpectedIntervalSeconds: 3600,
		GracePeriodSeconds:      60,
		IsActive:                true,
	}
}

func doRequest(deps Deps, method, path string, body string) *httptest.ResponseRecorder {
	router := gin.New()
	router.POST("/v1/heartbeat", deps.PostHeartbeat)
	router.GET("/v1/heartbeat/:job", deps.GetLatestHeartbeat)
	router.GET("/healthz", Healthz(deps))

	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestPostHeartbeat(t *testing.T) {
	job := newTestJob()

	tests := []struct {
		name       string
		body       string
		db         *fakeDB
		cache      *fakeCache
		wantStatus int
		wantBody   string // substring
	}{
		{
			name:       "malformed json",
			body:       `{"job":`,
			db:         &fakeDB{job: job},
			cache:      &fakeCache{},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing required job field",
			body:       `{"status":"success"}`,
			db:         &fakeDB{job: job},
			cache:      &fakeCache{},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid status value",
			body:       `{"job":"nightly-etl","status":"crashed"}`,
			db:         &fakeDB{job: job},
			cache:      &fakeCache{},
			wantStatus: http.StatusBadRequest,
			wantBody:   "status must be",
		},
		{
			name:       "status defaults to success when omitted",
			body:       `{"job":"nightly-etl"}`,
			db:         &fakeDB{job: job, insertID: 1, insertAt: time.Now()},
			cache:      &fakeCache{},
			wantStatus: http.StatusCreated,
			wantBody:   `"status":"success"`,
		},
		{
			name:       "job not found",
			body:       `{"job":"does-not-exist"}`,
			db:         &fakeDB{findErr: db.ErrJobNotFound},
			cache:      &fakeCache{},
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "db error looking up job",
			body:       `{"job":"nightly-etl"}`,
			db:         &fakeDB{findErr: errBoom},
			cache:      &fakeCache{},
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:       "db error inserting heartbeat",
			body:       `{"job":"nightly-etl","status":"success"}`,
			db:         &fakeDB{job: job, insertErr: errBoom},
			cache:      &fakeCache{},
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:       "success is not blocked by a cache write failure",
			body:       `{"job":"nightly-etl","status":"success","duration_ms":4231}`,
			db:         &fakeDB{job: job, insertID: 42, insertAt: time.Now()},
			cache:      &fakeCache{setErr: errBoom},
			wantStatus: http.StatusCreated,
			wantBody:   `"heartbeat_id":42`,
		},
		{
			name:       "failure status heartbeat is accepted",
			body:       `{"job":"nightly-etl","status":"failure"}`,
			db:         &fakeDB{job: job, insertID: 2, insertAt: time.Now()},
			cache:      &fakeCache{},
			wantStatus: http.StatusCreated,
			wantBody:   `"status":"failure"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			deps := Deps{DB: tc.db, Cache: tc.cache, Logger: testLogger()}
			rec := doRequest(deps, http.MethodPost, "/v1/heartbeat", tc.body)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d (body: %s)", rec.Code, tc.wantStatus, rec.Body.String())
			}
			if tc.wantBody != "" && !strings.Contains(rec.Body.String(), tc.wantBody) {
				t.Fatalf("body %q does not contain %q", rec.Body.String(), tc.wantBody)
			}
		})
	}
}

func TestPostHeartbeat_InsertNotCalledOnValidationFailure(t *testing.T) {
	fdb := &fakeDB{job: newTestJob()}
	deps := Deps{DB: fdb, Cache: &fakeCache{}, Logger: testLogger()}

	doRequest(deps, http.MethodPost, "/v1/heartbeat", `{"job":"nightly-etl","status":"bogus"}`)

	if fdb.insertCalled {
		t.Fatal("InsertHeartbeat should not be called when status validation fails")
	}
}

func TestGetLatestHeartbeat(t *testing.T) {
	job := newTestJob()

	tests := []struct {
		name       string
		db         *fakeDB
		cache      *fakeCache
		wantStatus int
		wantHeader string
		wantBody   string
	}{
		{
			name:       "job not found",
			db:         &fakeDB{findErr: db.ErrJobNotFound},
			cache:      &fakeCache{},
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "cache hit",
			db:         &fakeDB{job: job},
			cache:      &fakeCache{getResp: &cache.LastHeartbeat{JobID: job.ID.String(), Status: "success"}},
			wantStatus: http.StatusOK,
			wantHeader: "HIT",
			wantBody:   `"status":"success"`,
		},
		{
			name:       "cache miss falls back to unknown",
			db:         &fakeDB{job: job},
			cache:      &fakeCache{getResp: nil},
			wantStatus: http.StatusOK,
			wantHeader: "MISS",
			wantBody:   `"status":"unknown"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			deps := Deps{DB: tc.db, Cache: tc.cache, Logger: testLogger()}
			rec := doRequest(deps, http.MethodGet, "/v1/heartbeat/nightly-etl", "")

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			if tc.wantHeader != "" && rec.Header().Get("X-Cache") != tc.wantHeader {
				t.Fatalf("X-Cache = %q, want %q", rec.Header().Get("X-Cache"), tc.wantHeader)
			}
			if tc.wantBody != "" && !strings.Contains(rec.Body.String(), tc.wantBody) {
				t.Fatalf("body %q does not contain %q", rec.Body.String(), tc.wantBody)
			}
		})
	}
}

func TestHealthz(t *testing.T) {
	tests := []struct {
		name       string
		db         *fakeDB
		cache      *fakeCache
		wantStatus int
	}{
		{
			name:       "all healthy",
			db:         &fakeDB{},
			cache:      &fakeCache{},
			wantStatus: http.StatusOK,
		},
		{
			name:       "db down",
			db:         &fakeDB{pingErr: errBoom},
			cache:      &fakeCache{},
			wantStatus: http.StatusServiceUnavailable,
		},
		{
			name:       "cache down",
			db:         &fakeDB{},
			cache:      &fakeCache{pingErr: errBoom},
			wantStatus: http.StatusServiceUnavailable,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			deps := Deps{DB: tc.db, Cache: tc.cache, Logger: testLogger()}
			rec := doRequest(deps, http.MethodGet, "/healthz", "")

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
		})
	}
}
