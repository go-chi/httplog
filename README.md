# httplog

> Structured HTTP request logging middleware for Go, built on the standard library `log/slog` package

[![Go Reference](https://pkg.go.dev/badge/github.com/go-chi/httplog/v3.svg)](https://pkg.go.dev/github.com/go-chi/httplog/v3)
[![Go Report Card](https://goreportcard.com/badge/github.com/go-chi/httplog)](https://goreportcard.com/report/github.com/go-chi/httplog)
[![MIT License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

`httplog` is a lightweight, high-performance HTTP request logging middleware for Go web applications. Built on Go 1.21+'s standard `log/slog` package, it provides structured logging with zero external dependencies.

## Features

- **🚀 High Performance**: Minimal overhead
- **📋 Structured Logging**: Built on Go's standard `log/slog` package
- **🎯 Smart Log Levels**: Auto-assigns levels by status code (5xx = error, 4xx = warn)
- **📊 Schema Support**: Compatible with ECS, OTEL, and GCP logging formats
- **🛡️ Panic Recovery**: Recovers panics with stack traces and HTTP 500 responses
- **🔍 Body Logging**: Conditional request/response body capture with content-type filtering
- **📝 Custom Attributes**: Add log attributes from handlers and middlewares
- **🎨 Developer Friendly**: Concise mode and `curl` command generation
- **🔗 Router Agnostic**: Works with [Chi](https://github.com/go-chi/chi), Gin, Echo, and standard `http.ServeMux`

## Usage

`go get github.com/go-chi/httplog/v3@latest`

```go
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
	"github.com/go-chi/httplog/v3"
)

func main() {
	logFormat := httplog.SchemaECS.Concise(isLocalhost)

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		ReplaceAttr: logFormat.ReplaceAttr,
	})).With(
		slog.String("app", "example-app"),
		slog.String("version", "v1.0.0-a1fa420"),
		slog.String("env", "production"),
	)

	r := chi.NewRouter()

	// Request logger
	r.Use(httplog.RequestLogger(logger, &httplog.Options{
		// Level defines the verbosity of the request logs:
		// slog.LevelDebug - log all responses (incl. OPTIONS)
		// slog.LevelInfo  - log responses (excl. OPTIONS)
		// slog.LevelWarn  - log 4xx and 5xx responses only (except for 429)
		// slog.LevelError - log 5xx responses only
		Level: slog.LevelInfo,

		// Set log output to Elastic Common Schema (ECS) format.
		Schema: httplog.SchemaECS,

		// RecoverPanics recovers from panics occurring in the underlying HTTP handlers
		// and middlewares. It returns HTTP 500 unless response status was already set.
		//
		// NOTE: Panics are logged as errors automatically, regardless of this setting.
		RecoverPanics: true,

		// Optionally, filter out some request logs.
		Skip: func(req *http.Request, respStatus int) bool {
			return respStatus == 404 || respStatus == 405
		},

		// Optionally, log selected request/response headers explicitly.
		LogRequestHeaders:  []string{"Origin"},
		LogResponseHeaders: []string{},

		// Optionally, enable logging of request/response body based on custom conditions.
		// Useful for debugging payload issues in development.
		LogRequestBody:  isDebugHeaderSet,
		LogResponseBody: isDebugHeaderSet,
	}))

	// Set request log attribute from within middleware.
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			httplog.SetAttrs(ctx, slog.String("user", "user1"))

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hello world \n"))
	})

	http.ListenAndServe("localhost:8000", r)
}

func isDebugHeaderSet(r *http.Request) bool {
	return r.Header.Get("Debug") == "reveal-body-logs"
}
```

## Example

See [_example/main.go](./_example/main.go) and try it locally:
```sh
$ cd _example

# JSON logger (production)
$ go run .

# Pretty logger (localhost)
$ ENV=localhost go run .
```

## License
[MIT license](./LICENSE)
