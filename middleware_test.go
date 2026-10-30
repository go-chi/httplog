package httplog

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBodyPredicatesCalledOnce(t *testing.T) {
	var reqCalls, respCalls int

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	mw := RequestLogger(logger, &Options{
		Schema:          SchemaECS,
		LogRequestBody:  func(*http.Request) bool { reqCalls++; return true },
		LogResponseBody: func(*http.Request) bool { respCalls++; return true },
	})

	req := httptest.NewRequest("POST", "/api/users", strings.NewReader(`{"a":1}`))
	req.Header.Set("Content-Type", "application/json")
	mw(http.HandlerFunc(okHandler)).ServeHTTP(httptest.NewRecorder(), req)

	if reqCalls != 1 {
		t.Errorf("LogRequestBody called %d times, want 1", reqCalls)
	}
	if respCalls != 1 {
		t.Errorf("LogResponseBody called %d times, want 1", respCalls)
	}

	// The predicate's single result must still drive the response body capture.
	if !strings.Contains(buf.String(), `"http.response.body.content":"{}"`) {
		t.Errorf("response body not logged: %s", buf.String())
	}
}

func TestClientAbortedWithoutResponse(t *testing.T) {
	tests := []struct {
		name    string
		handler http.HandlerFunc
		status  float64
	}{
		{"no response written", func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }, 499},
		{"response written", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(503) }, 503},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&buf, nil))
			mw := RequestLogger(logger, &Options{Schema: SchemaECS})

			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			req := httptest.NewRequest("GET", "/api/users", nil).WithContext(ctx)
			mw(tt.handler).ServeHTTP(httptest.NewRecorder(), req)

			var entry map[string]any
			if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
				t.Fatalf("invalid log entry %q: %v", buf.String(), err)
			}
			if entry["http.response.status_code"] != tt.status {
				t.Errorf("http.response.status_code = %v, want %v", entry["http.response.status_code"], tt.status)
			}
			if entry["error.type"] != "ClientAborted" {
				t.Errorf("error.type = %v, want ClientAborted", entry["error.type"])
			}
		})
	}
}
