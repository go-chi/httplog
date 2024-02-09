package httplog

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// Test that ensures the Logger options match the options of a LogEntry
func Test_requestLogger_NewLogEntry_Options(t *testing.T) {
	opts := Options{
		Concise: true,
		JSON:    true,
	}
	logger := &Logger{
		Logger:  slog.Default(),
		Options: opts,
	}

	r := chi.NewMux()
	r.Use(Handler(logger))

	var entryFromRequest middleware.LogEntry
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		entryFromRequest = middleware.GetLogEntry(r)
		w.Write([]byte("OK"))
	})

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	rEntry, ok := entryFromRequest.(*RequestLoggerEntry)
	if !ok {
		t.Errorf("expected log entry to be a RequestLoggerEntry")
	}
	if !reflect.DeepEqual(opts, rEntry.Options) {
		t.Errorf("entry = %v, want %v", entryFromRequest, opts)
	}
}
