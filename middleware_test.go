package httplog

import (
	"bytes"
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
