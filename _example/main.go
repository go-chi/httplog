package main

import (
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/traceid"
	"github.com/golang-cz/devslog"
	"github.com/golang-cz/httplog"
)

func main() {
	// JSON logger for production.
	// For localhost development, you can replace it with slog.NewTextHandler()
	// or with a pretty logger like github.com/golang-cz/devslog.
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil)).With(
		slog.String("app", "example-app"),
		slog.String("version", "v1.0.0-a1fa420"),
		slog.String("env", "production"),
	)

	isLocalhost := os.Getenv("ENV") == "localhost"
	if isLocalhost {
		// Pretty logger for localhost development.
		logger = slog.New(devslog.NewHandler(os.Stdout, &devslog.Options{
			SortKeys:           true,
			MaxErrorStackTrace: 5,
			MaxSlicePrintSize:  20,
		}))
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
		// Level defines the verbosity of the requests logs:
		// slog.LevelDebug - log both request starts & responses (incl. OPTIONS)
		// slog.LevelInfo  - log responses (excl. OPTIONS)
		// slog.LevelWarn  - log only 4xx and 5xx responses (except for 429)
		// slog.LevelError - log only 5xx responses only
		Level: slog.LevelInfo,

		// Concise mode causes fewer log attributes to be printed in request logs.
		// This is useful if your console is too noisy during development.
		Concise: isLocalhost,

		// RecoverPanics recovers from panics occurring in the underlying HTTP handlers
		// and middlewares. It returns HTTP 500 unless response status was already set.
		//
		// NOTE: The request logger automatically logs all panics, regardless of this setting.
		RecoverPanics: true,

		// Select request/response headers to be logged explicitly.
		ReqHeaders:  []string{"User-Agent", "Origin", "Referer", traceid.Header},
		RespHeaders: []string{traceid.Header},

		// You can log request/request body. Useful for debugging.
		ReqBody:  false,
		RespBody: false,
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
		httplog.InfoContext(r.Context(), "info here")

		w.Header().Add("Content-Type", "text/plain")
		w.Write([]byte("info here \n"))
	})

	r.Get("/warn", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		httplog.WarnContext(ctx, "warn here")

		w.WriteHeader(400)
		w.Write([]byte("warn here \n"))
	})

	r.Get("/err", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Log error explicitly.
		err := errors.New("err here")
		httplog.ErrorContext(ctx, "msg here", slog.Any("error", err))

		// Logging with the global logger also works.
		slog.Default().With(slog.Group("ImpGroup", slog.String("account", "id"))).ErrorContext(ctx, "doesn't exist")
		slog.Default().ErrorContext(ctx, "oops, error occurred")

		w.WriteHeader(500)
		w.Write([]byte("oops, err \n"))
	})

	fmt.Println("Enable pretty logs with:")
	fmt.Println("  ENV=localhost go run ./")
	fmt.Println()
	fmt.Println("Try these commands from a new terminal window:")
	fmt.Println("  curl -v http://localhost:8000")
	fmt.Println("  curl -v http://localhost:8000/panic")
	fmt.Println("  curl -v http://localhost:8000/info")
	fmt.Println("  curl -v http://localhost:8000/warn")
	fmt.Println("  curl -v http://localhost:8000/err")
	fmt.Println()

	if err := http.ListenAndServe("localhost:8000", r); err != http.ErrAbortHandler {
		log.Fatal(err)
	}
}
