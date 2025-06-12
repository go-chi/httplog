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
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httplog/v3"
	"github.com/go-chi/traceid"
	"github.com/golang-cz/devslog"
)

func main() {
	isLocalhost := os.Getenv("ENV") == "localhost"

	logFormat := httplog.SchemaECS.Concise(isLocalhost)

	logger := slog.New(logHandler(isLocalhost, &slog.HandlerOptions{
		AddSource:   !isLocalhost,
		ReplaceAttr: logFormat.ReplaceAttr,
	}))

	if !isLocalhost {
		logger = logger.With(
			slog.String("app", "example-app"),
			slog.String("version", "v1.0.0-a1fa420"),
			slog.String("env", "production"),
		)
	}

	// Set as a default logger for both slog and log.
	slog.SetDefault(logger)
	slog.SetLogLoggerLevel(slog.LevelError)

	r := chi.NewRouter()
	r.Use(middleware.Heartbeat("/ping"))

	// Propagate or create new TraceId header.
	r.Use(traceid.Middleware)

	// Request logger.
	r.Use(httplog.RequestLogger(logger, &httplog.Options{
		// Level defines the verbosity of the request logs:
		// slog.LevelDebug - log all responses (incl. OPTIONS)
		// slog.LevelInfo  - log all responses (excl. OPTIONS)
		// slog.LevelWarn  - log 4xx and 5xx responses only (except for 429)
		// slog.LevelError - log 5xx responses only
		Level: slog.LevelInfo,

		// Log attributes using given schema/format.
		Schema: logFormat,

		// RecoverPanics recovers from panics occurring in the underlying HTTP handlers
		// and middlewares. It returns HTTP 500 unless response status was already set.
		//
		// NOTE: Panics are logged as errors automatically, regardless of this setting.
		RecoverPanics: true,

		// Filter out some request logs.
		Skip: func(req *http.Request, respStatus int) bool {
			return respStatus == 404 || respStatus == 405
		},

		// Select request/response headers to be logged explicitly.
		LogRequestHeaders:  []string{"Origin"},
		LogResponseHeaders: []string{},

		// You can log request/request body conditionally. Useful for debugging.
		LogRequestBody:  isDebugHeaderSet,
		LogResponseBody: isDebugHeaderSet,

		// Log all requests with invalid payload as curl command.
		LogExtraAttrs: func(req *http.Request, reqBody string, respStatus int) []slog.Attr {
			if respStatus == 400 || respStatus == 422 {
				req.Header.Del("Authorization")
				return []slog.Attr{slog.String("curl", httplog.CURL(req, reqBody))}
			}
			return nil
		},
	}))

	// Set request log attribute from within middleware.
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			httplog.SetAttrs(ctx, slog.String("user", "user1"))

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})

	r.Get("/slow", func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(5 * time.Second):
			w.Write([]byte("slow operation completed \n"))

		case <-r.Context().Done():
			// client disconnected
		}
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

		w.Header().Add("Content-Type", "text/plain")
		w.WriteHeader(200)
		w.Write([]byte("warn here \n"))
	})

	r.Get("/err", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Log error explicitly.
		err := errors.New("err here")
		logger.ErrorContext(ctx, "msg here", slog.String("error", err.Error()))

		// Logging with the global logger also works.
		slog.Default().With(slog.Group("group", slog.String("account", "id"))).ErrorContext(ctx, "doesn't exist")
		slog.Default().ErrorContext(ctx, "oops, error occurred")

		// Or, set the error attribute on the request log.
		httplog.SetError(ctx, err)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(500)
		w.Write([]byte(fmt.Sprintf(`{"error": "%v"}`, err)))
	})

	r.Post("/string/to/upper", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		w.Header().Set("Content-Type", "application/json")

		var payload struct {
			Data string `json:"data"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			err = fmt.Errorf("invalid json: %w", err)

			w.WriteHeader(400)
			w.Write([]byte(fmt.Sprintf(`{"error": %q}`, httplog.SetError(ctx, err))))
			return
		}
		if payload.Data == "" {
			err := errors.New("data field is required")

			w.WriteHeader(422)
			w.Write([]byte(fmt.Sprintf(`{"error": %q}`, httplog.SetError(ctx, err))))
			return
		}

		payload.Data = strings.ToUpper(payload.Data)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(payload)
	})

	if !isLocalhost {
		fmt.Println("Enable pretty logs with:")
		fmt.Println("  ENV=localhost go run ./")
		fmt.Println()
	}

	fmt.Println("Try these commands from a new terminal window:")
	fmt.Println("  curl -v http://localhost:8000/info")
	fmt.Println("  curl -v http://localhost:8000/warn")
	fmt.Println("  curl -v http://localhost:8000/err")
	fmt.Println("  curl -v http://localhost:8000/panic")
	fmt.Println("  curl -v http://localhost:8000/slow")
	fmt.Println(`  curl -v http://localhost:8000/string/to/upper -X POST --json '{"data": "valid payload"}'`)
	fmt.Println(`  curl -v http://localhost:8000/string/to/upper -X POST --json '{"data": "valid payload"}' -H "Debug: reveal-body-logs"`)
	fmt.Println(`  curl -v http://localhost:8000/string/to/upper -X POST --json '{"xx": "invalid payload"}'`)
	fmt.Println()

	if err := http.ListenAndServe("localhost:8000", r); err != http.ErrAbortHandler {
		log.Fatal(err)
	}
}

func logHandler(isLocalhost bool, handlerOpts *slog.HandlerOptions) slog.Handler {
	if isLocalhost {
		// Pretty logs for localhost development.
		return devslog.NewHandler(os.Stdout, &devslog.Options{
			SortKeys:           true,
			MaxErrorStackTrace: 5,
			MaxSlicePrintSize:  20,
			HandlerOptions:     handlerOpts,
		})
	}

	// JSON logs for production with "traceId".
	return traceid.LogHandler(
		slog.NewJSONHandler(os.Stdout, handlerOpts),
	)
}

func isDebugHeaderSet(r *http.Request) bool {
	return r.Header.Get("Debug") == "reveal-body-logs"
}
