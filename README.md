# httplog - HTTP Request Logger

A small but powerful structured logging package for HTTP request logging, built on the Go 1.21+ standard library `slog` package.

## Features

- **Efficient Logging**: Separates frontend (`slog.Logger`) and backend (`slog.Handler`) to optimize performance in serving HTTP responses quickly.
- **Extensible Backend**: The provided backend `slog.Handler` is both extensible and replaceable, allowing customization to suit your logging needs.
- **Flexible Attribute Attachment**: Supports attaching additional log attributes from within downstream HTTP handlers or middlewares, ensuring comprehensive and contextual logging.
- **Debug Request and Response Bodies**: Provides options to log request and response bodies for debugging and analysis.
- **Panic Recovery**: Optionally recovers from panics in underlying HTTP handlers or middlewares, logging the error and ensuring a consistent response status code.

## Example

See [_example/main.go](./_example/main.go). Try running it locally:
```sh
$ ENV=production go run github.com/golang-cz/httplog/_example

$ ENV=localhost go run github.com/golang-cz/httplog/_example
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
		// NOTE: The request logger logs all panics automatically, regardless of this setting.
		RecoverPanics: true,

		// Select request/response headers to be logged explicitly.
		LogRequestHeaders:  []string{"User-Agent", "Origin", "Referer"},
		LogResponseHeaders: []string{},

		// You can log request/request body. Useful for debugging.
		LogRequestBody:  false,
		LogResponseBody: false,
	}))

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Set attribute on the request log.
		httplog.SetAttrs(ctx, slog.String("userId", "id"))

		w.Write([]byte("."))
	})

	http.ListenAndServe("localhost:8000", r)
}
```

## TODO
- [x] Integrate panic recoverer
- [x] Debug request body
- [x] Debug response body
- [x] Add example
- [ ] Add tests

## License
[MIT license](./LICENSE)
