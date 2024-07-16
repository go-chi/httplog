package main

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httplog/v2"
)

func main() {
	// Logger
	logger := httplog.NewLogger("httplog-example", httplog.Options{
		LogLevel: slog.LevelDebug,
		// JSON:             true,
		Concise: true,
		// RequestHeaders:   true,
		// ResponseHeaders:  true,
		MessageFieldName: "message",
		LevelFieldName:   "severity",
		TimeFieldFormat:  time.RFC3339,
		Tags: map[string]string{
			"version": "v1.0-81aa4244d9fc8076a",
			"env":     "dev",
		},
		QuietDownRoutes: []string{
			"/",
			"/ping",
		},
		QuietDownPeriod: 10 * time.Second,
		// SourceFieldName: "source",
	})

	// Service
	r := chi.NewRouter()
	r.Use(httplog.RequestLogger(logger, []string{"/ping"}))
	r.Use(middleware.Heartbeat("/ping"))

	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			// Set a single log field
			httplog.LogEntrySetField(ctx, "user", slog.StringValue("user1"))

			// Set multiple fields
			fields := map[string]any{
				"remote": "example.com",
				"action": "update",
			}
			httplog.LogEntrySetFields(ctx, fields)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hello world"))
	})

	r.Get("/panic", func(w http.ResponseWriter, r *http.Request) {
		panic("oh no")
	})

	r.Get("/info", func(w http.ResponseWriter, r *http.Request) {
		oplog := httplog.LogEntry(r.Context())
		w.Header().Add("Content-Type", "text/plain")
		oplog.Info("info here")
		w.Write([]byte("info here"))
	})

	r.Get("/warn", func(w http.ResponseWriter, r *http.Request) {
		oplog := httplog.LogEntry(r.Context())
		oplog.Warn("warn here")
		w.WriteHeader(400)
		w.Write([]byte("warn here"))
	})

	r.Get("/err", func(w http.ResponseWriter, r *http.Request) {
		oplog := httplog.LogEntry(r.Context())

		// two varianets of syntax to specify "err" attr.
		err := errors.New("err here")
		// oplog.Error("msg here", "err", err)
		oplog.Error("msg here", httplog.ErrAttr(err))

		// logging with the global logger also works
		slog.Default().With(slog.Group("ImpGroup", slog.String("account", "id"))).Error("doesn't exist")
		slog.Default().Error("oops, err occured")
		w.WriteHeader(500)
		w.Write([]byte("oops, err"))
	})

	http.ListenAndServe("localhost:8000", r)
}
