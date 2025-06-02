package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/traceid"
	"github.com/golang-cz/devslog"
	"github.com/golang-cz/httplog"
)

func main() {
	prettyLogs := os.Getenv("ENV") == "localhost"

	logHandler := getLogHandler(prettyLogs)
	logHandler = traceid.LogHandler(logHandler) // Add "traceId" to all logs, if available in ctx.

	logger := slog.New(logHandler)

	if !prettyLogs {
		logger = logger.With(
			slog.String("app", "example-app"),
			slog.String("version", "v1.0.0-a1fa420"),
			slog.String("env", "production"),
		)
	}

	// Set as a default logger for both slog and log.
	slog.SetDefault(logger)
	slog.SetLogLoggerLevel(slog.LevelError)

	// Service
	r := chi.NewRouter()
	r.Use(middleware.Heartbeat("/ping"))

	// Propagate or create new TraceId header.
	r.Use(traceid.Middleware)

	// Request logger
	r.Use(httplog.RequestLogger(logger, &httplog.Options{
		// Level defines the verbosity of the request logs:
		// slog.LevelDebug - log both request starts & responses (incl. OPTIONS)
		// slog.LevelInfo  - log all responses (excl. OPTIONS)
		// slog.LevelWarn  - log 4xx and 5xx responses only (except for 429)
		// slog.LevelError - log 5xx responses only
		Level: slog.LevelInfo,

		// Concise mode causes fewer log attributes to be printed in request logs.
		// This is useful if your console is too noisy during development.
		Concise: prettyLogs,

		// RecoverPanics recovers from panics occurring in the underlying HTTP handlers
		// and middlewares. It returns HTTP 500 unless response status was already set.
		//
		// NOTE: Panics are logged as errors automatically, regardless of this setting.
		RecoverPanics: true,

		// Select request/response headers to be logged explicitly.
		LogRequestHeaders:  []string{"User-Agent", "Origin", "Referer"},
		LogResponseHeaders: []string{},

		// You can log request/request body. Useful for debugging.
		LogRequestBody:  prettyLogs,
		LogResponseBody: prettyLogs,

		LogBodyContentTypes: []string{"application/json"},
	}))

	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			// Set request log attribute from within middleware.
			httplog.SetAttrs(ctx, slog.String("user", "user1"))

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hello world \n"))
	})

	r.Get("/panic", func(w http.ResponseWriter, r *http.Request) {
		panic("oh no")
	})

	r.Get("/info", func(w http.ResponseWriter, r *http.Request) {
		logger.InfoContext(r.Context(), "info here")

		w.Header().Add("Content-Type", "text/plain")
		w.Write([]byte("info here \n"))
	})

	r.Get("/warn", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		logger.WarnContext(ctx, "warn here")

		w.WriteHeader(400)
		w.Write([]byte("warn here \n"))
	})

	r.Get("/err", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Log error explicitly.
		err := errors.New("err here")
		logger.ErrorContext(ctx, "msg here", slog.Any("error", err))

		// Logging with the global logger also works.
		slog.Default().With(slog.Group("ImpGroup", slog.String("account", "id"))).ErrorContext(ctx, "doesn't exist")
		slog.Default().ErrorContext(ctx, "oops, error occurred")

		w.WriteHeader(500)
		w.Write([]byte("oops, err \n"))
	})

	r.Post("/body", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		_ = ctx

		var payload struct {
			Data string `json:"data"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			w.WriteHeader(400)
			w.Write([]byte(fmt.Sprintf(`{"error": "%v"}`, err)))
			return
		}

		// Log request/response bodies for Admin requests.
		if r.Header.Get("Authorization") == "Bearer ADMIN-SECRET" {
			// logger.LogRequestBody(ctx)
			// logger.LogResponseBody(ctx)
		}

		payload.Data = strings.ToUpper(payload.Data)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(payload)
	})

	if !prettyLogs {
		fmt.Println("Enable pretty logs with:")
		fmt.Println("  ENV=localhost go run ./")
		fmt.Println()
	}

	fmt.Println("Try these commands from a new terminal window:")
	fmt.Println("  curl -v http://localhost:8000")
	fmt.Println("  curl -v http://localhost:8000/panic")
	fmt.Println("  curl -v http://localhost:8000/info")
	fmt.Println("  curl -v http://localhost:8000/warn")
	fmt.Println("  curl -v http://localhost:8000/err")
	fmt.Println(`  curl -v http://localhost:8000/body -X POST --json '{"data": "some data"}'`)
	fmt.Println(`  curl -v http://localhost:8000/body -X POST --json '{"data": "some data"}' -H "Authorization: Bearer ADMIN-SECRET"`)
	fmt.Println()

	if err := http.ListenAndServe("localhost:8000", r); err != http.ErrAbortHandler {
		log.Fatal(err)
	}
}

func getLogHandler(pretty bool) slog.Handler {
	if pretty {
		// Pretty logs for localhost development.
		return devslog.NewHandler(os.Stdout, &devslog.Options{
			SortKeys:           true,
			MaxErrorStackTrace: 5,
			MaxSlicePrintSize:  20,
		})
	}

	// JSON logs for production.
	return slog.NewJSONHandler(os.Stdout, nil)
}
