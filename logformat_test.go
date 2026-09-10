package httplog

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func serveAndDecode(t *testing.T, o *Options) map[string]any {
	t.Helper()

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	req := httptest.NewRequest("GET", "/api/users", nil)
	RequestLogger(logger, o)(http.HandlerFunc(okHandler)).ServeHTTP(httptest.NewRecorder(), req)

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("invalid JSON log entry: %v\n%s", err, buf.String())
	}
	return entry
}

func TestLogFormat(t *testing.T) {
	var gotArgs *LogFormatArgs

	entry := serveAndDecode(t, &Options{
		Schema: SchemaECS,
		LogFormat: func(r *http.Request, args *LogFormatArgs) string {
			gotArgs = args
			return "request processed"
		},
	})

	if entry["msg"] != "request processed" {
		t.Errorf("msg = %v, want %q", entry["msg"], "request processed")
	}
	if gotArgs == nil {
		t.Fatal("LogFormat was not called")
	}
	if gotArgs.StatusCode != 200 {
		t.Errorf("StatusCode = %v, want 200", gotArgs.StatusCode)
	}
	if gotArgs.Duration <= 0 {
		t.Errorf("Duration = %v, want > 0", gotArgs.Duration)
	}
	if len(gotArgs.Attrs) == 0 {
		t.Error("Attrs is empty")
	}
}

func TestLogFormatDefault(t *testing.T) {
	entry := serveAndDecode(t, &Options{Schema: SchemaECS})

	msg, _ := entry["msg"].(string)
	if !strings.HasPrefix(msg, "GET /api/users => HTTP 200 (") {
		t.Errorf("unexpected default msg: %q", msg)
	}
}
