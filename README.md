# httplog

> Structured HTTP request logging middleware for Go, built on the standard library `log/slog` package

[![Go Reference](https://pkg.go.dev/badge/github.com/golang-cz/httplog.svg)](https://pkg.go.dev/github.com/golang-cz/httplog)
[![Go Report Card](https://goreportcard.com/badge/github.com/golang-cz/httplog)](https://goreportcard.com/report/github.com/golang-cz/httplog)
[![MIT License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

`httplog` is a lightweight, high-performance HTTP request logging middleware for Go web applications. Built on Go 1.21+'s standard `log/slog` package, it provides structured logging with zero external dependencies.

## Features

- **🚀 High Performance**: Minimal overhead request/response capture
- **📋 Structured Logging**: Built on Go's standard `log/slog` package
- **🎯 Smart Log Levels**: Auto-assigns levels by status code (5xx = error, 4xx = warn)
- **🛡️ Panic Recovery**: Recovers panics with stack traces and HTTP 500 responses
- **🔍 Body Logging**: Conditional request/response body capture with content-type filtering
- **🎨 Developer Friendly**: Concise mode and cURL command generation
- **🔗 Router Agnostic**: Works with Chi, Gin, Echo, and standard `http.ServeMux`
- **📝 Custom Attributes**: Add log attributes from handlers and middlewares

## Example

See [_example/main.go](./_example/main.go). Try running it locally:
```sh
$ cd _example

# JSON logger
$ go run .

# pretty logger
$ ENV=localhost go run .
```

## Usage

`go get github.com/golang-cz/httplog@latest`

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
	"github.com/golang-cz/httplog"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil)).With(
		slog.String("app", "example-app"),
		slog.String("version", "v1.0.0-a1fa420"),
		slog.String("env", "production"),
	)

	r := chi.NewRouter()

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
		Concise: false,

		// RecoverPanics recovers from panics occurring in the underlying HTTP handlers
		// and middlewares. It returns HTTP 500 unless response status was already set.
		//
		// NOTE: Panics are logged as errors automatically, regardless of this setting.
		RecoverPanics: true,

		// Log selected request/response headers explicitly.
		LogRequestHeaders:  []string{"User-Agent", "Origin", "Referer"},
		LogResponseHeaders: []string{},

		// Log request/request body. Useful for debugging.
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
	return r.Header.Get("Debug") == "reveal-logs"
}
```

## License
[MIT license](./LICENSE)
