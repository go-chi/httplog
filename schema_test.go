package httplog

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"
)

func captureLog(t *testing.T, schema *Schema, method string, handler http.HandlerFunc) map[string]any {
	t.Helper()

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level:       slog.LevelDebug,
		ReplaceAttr: schema.ReplaceAttr,
	}))

	mw := RequestLogger(logger, &Options{
		Level:              slog.LevelDebug,
		Schema:             schema,
		RecoverPanics:      true,
		LogRequestHeaders:  []string{"Content-Type"},
		LogResponseHeaders: []string{"Content-Type"},
	})

	req := httptest.NewRequest(method, "/api/users", nil)
	req.Host = "example.com"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "test-agent")
	req.Header.Set("Referer", "https://example.com/home")

	mw(handler).ServeHTTP(httptest.NewRecorder(), req)

	if buf.Len() == 0 {
		t.Fatal("no log entry emitted")
	}
	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("invalid JSON log entry: %v\n%s", err, buf.String())
	}
	return entry
}

func okHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	w.Write([]byte(`{}`))
}

func statusHandler(status int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
	}
}

func TestSchemaECSFieldNames(t *testing.T) {
	entry := captureLog(t, SchemaECS, "GET", okHandler)

	for _, key := range []string{
		"@timestamp", "log.level", "message",
		"url.full", "http.request.method", "url.path", "client.ip", "url.domain",
		"url.scheme", "http.version", "http.request.headers", "http.request.body.bytes",
		"user_agent.original", "http.request.referrer",
		"http.response.headers", "http.response.status_code", "event.duration", "http.response.body.bytes",
	} {
		if _, ok := entry[key]; !ok {
			t.Errorf("missing ECS field %q in %v", key, entry)
		}
	}

	if ts, _ := entry["@timestamp"].(string); ts == "" {
		t.Errorf("@timestamp is not a string: %v", entry["@timestamp"])
	} else if _, err := time.Parse(time.RFC3339Nano, ts); err != nil {
		t.Errorf("@timestamp is not RFC3339Nano: %v", err)
	}

	// ECS event.duration is nanoseconds as a JSON number.
	if _, ok := entry["event.duration"].(float64); !ok {
		t.Errorf("event.duration is not a number: %v", entry["event.duration"])
	}
}

func TestSchemaOTELFieldNames(t *testing.T) {
	entry := captureLog(t, SchemaOTEL, "GET", okHandler)

	for _, key := range []string{
		"timestamp", "severity_text", "body",
		"url.full", "http.request.method", "url.path", "client.address", "server.address",
		"url.scheme", "network.protocol.version", "http.request.header", "http.request.body.size",
		"user_agent.original", "http.request.header.referer",
		"http.response.header", "http.response.status_code", "http.server.request.duration", "http.response.body.size",
	} {
		if _, ok := entry[key]; !ok {
			t.Errorf("missing OTEL field %q in %v", key, entry)
		}
	}
}

func TestSchemaOTELSourceLocation(t *testing.T) {
	src := &slog.Source{File: "/app/main.go", Line: 42, Function: "main.main"}
	a := SchemaOTEL.ReplaceAttr(nil, slog.Any(slog.SourceKey, src))

	got := map[string]bool{}
	for _, attr := range a.Value.Group() {
		got[attr.Key] = true
	}
	// Semconv v1.31.0+ names; code.filepath, code.lineno and code.function are deprecated.
	for _, key := range []string{"code.file.path", "code.line.number", "code.function.name"} {
		if !got[key] {
			t.Errorf("missing OTEL source attribute %q in %v", key, got)
		}
	}
}

func TestSchemaGCPFieldNames(t *testing.T) {
	entry := captureLog(t, SchemaGCP, "GET", okHandler)

	for _, key := range []string{
		"time", "severity", "message", "httpRequest",
		"requestPath", "host", "scheme", "requestHeaders", "responseHeaders",
	} {
		if _, ok := entry[key]; !ok {
			t.Errorf("missing GCP field %q in %v", key, entry)
		}
	}

	// Cloud Logging's "timestamp" special field must be an object; the string form is "time".
	if _, ok := entry["timestamp"]; ok {
		t.Errorf(`unexpected "timestamp" field: string timestamps belong in "time"`)
	}
	if ts, _ := entry["time"].(string); ts == "" {
		t.Errorf("time is not a string: %v", entry["time"])
	} else if _, err := time.Parse(time.RFC3339Nano, ts); err != nil {
		t.Errorf("time is not RFC3339Nano: %v", err)
	}

	// The Logging API rejects entries with unknown httpRequest subfields, so the
	// nested object must stay within LogEntry#HttpRequest's official field set.
	officialHTTPRequestFields := map[string]bool{
		"requestMethod": true, "requestUrl": true, "requestSize": true, "status": true,
		"responseSize": true, "userAgent": true, "remoteIp": true, "serverIp": true,
		"referer": true, "latency": true, "cacheLookup": true, "cacheHit": true,
		"cacheValidatedWithOriginServer": true, "cacheFillBytes": true, "protocol": true,
	}
	httpRequest, ok := entry["httpRequest"].(map[string]any)
	if !ok {
		t.Fatalf("httpRequest is not an object: %v", entry["httpRequest"])
	}
	for key := range httpRequest {
		if !officialHTTPRequestFields[key] {
			t.Errorf("unofficial field %q in httpRequest", key)
		}
	}
	for _, key := range []string{
		"requestUrl", "requestMethod", "remoteIp", "protocol", "requestSize",
		"userAgent", "referer", "status", "latency", "responseSize",
	} {
		if _, ok := httpRequest[key]; !ok {
			t.Errorf("missing httpRequest field %q in %v", key, httpRequest)
		}
	}

	if latency, _ := httpRequest["latency"].(string); !regexp.MustCompile(`^\d+(\.\d+)?s$`).MatchString(latency) {
		t.Errorf("latency %q does not match the GCP duration format", latency)
	}
}

func TestSchemaGCPSourceLocation(t *testing.T) {
	src := &slog.Source{File: "/app/main.go", Line: 42, Function: "main.main"}
	a := SchemaGCP.ReplaceAttr(nil, slog.Any(slog.SourceKey, src))

	if a.Key != "logging.googleapis.com/sourceLocation" {
		t.Errorf("unexpected source location group %q", a.Key)
	}
	got := map[string]bool{}
	for _, attr := range a.Value.Group() {
		got[attr.Key] = true
	}
	for _, key := range []string{"file", "line", "function"} {
		if !got[key] {
			t.Errorf("missing sourceLocation attribute %q in %v", key, got)
		}
	}
}

func TestSchemaGCPSeverity(t *testing.T) {
	tests := []struct {
		method   string
		handler  http.HandlerFunc
		severity string
	}{
		{"GET", okHandler, "INFO"},
		{"GET", statusHandler(400), "WARNING"},
		{"GET", statusHandler(429), "INFO"},
		{"GET", statusHandler(500), "ERROR"},
		{"OPTIONS", okHandler, "DEBUG"},
	}

	for _, tt := range tests {
		entry := captureLog(t, SchemaGCP, tt.method, tt.handler)
		if entry["severity"] != tt.severity {
			t.Errorf("%s %v: severity = %v, want %q", tt.method, tt.handler, entry["severity"], tt.severity)
		}
	}
}

func TestPanicStackTrace(t *testing.T) {
	panicHandler := func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	}

	tests := []struct {
		name          string
		schema        *Schema
		stackTraceKey string
		errorKey      string
	}{
		{"ECS", SchemaECS, "error.stack_trace", "error.message"},
		{"OTEL", SchemaOTEL, "exception.stacktrace", "error.message"},
		{"GCP", SchemaGCP, "stack_trace", "error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := captureLog(t, tt.schema, "GET", panicHandler)

			if entry[tt.errorKey] != "panic: boom" {
				t.Errorf("%s = %v, want %q", tt.errorKey, entry[tt.errorKey], "panic: boom")
			}

			// Go panic format: parseable by GCP Error Reporting, single string for ECS/OTEL.
			stackTrace, ok := entry[tt.stackTraceKey].(string)
			if !ok {
				t.Fatalf("%s is not a string: %v", tt.stackTraceKey, entry[tt.stackTraceKey])
			}
			if !strings.HasPrefix(stackTrace, "panic: boom\n\ngoroutine ") {
				t.Errorf("%s does not start with the Go panic format:\n%s", tt.stackTraceKey, stackTrace)
			}
			if !strings.Contains(stackTrace, "schema_test.go") {
				t.Errorf("%s does not contain the panic origin:\n%s", tt.stackTraceKey, stackTrace)
			}
		})
	}
}
