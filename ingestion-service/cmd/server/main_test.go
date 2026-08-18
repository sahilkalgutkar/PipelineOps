package main

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
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
