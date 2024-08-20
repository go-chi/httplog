# HTTP request logger

- HTTP middleware that logs single structured log per request/response
- Implements frontend (`slog.Logger`) and backend (`slog.Handler`) separately to improve critical performance of serving HTTP response
- The provided backend `slog.Handler` is extensible and/or replaceable
- Allows attaching log attributes to the single log line from within downstream HTTP handlers/middlewares

## Example

See [_example/main.go](./_example/main.go) and try running it locally:
```sh
$ go run github.com/golang-cz/httplog/_example

$ ENV=localhost go run github.com/golang-cz/httplog/_example
```

## Usage

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

	// Logger
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
MIT
