httplog
=======

Small but powerful structured logging package for HTTP request logging built
on the Go 1.21+ stdlib `slog` package.

```
go get -u github.com/go-chi/httplog/v2
```

## Example

(see [_example/](./_example/main.go))

```go
package main

import (
  "log/slog"
  "net/http"
  "github.com/go-chi/chi/v5"
  "github.com/go-chi/chi/v5/middleware"
  "github.com/go-chi/httplog/v2"
)

func main() {
  // Logger
  logger := httplog.NewLogger("httplog-example", httplog.Options{
    // JSON:             true,
    LogLevel:         slog.LevelDebug,
    Concise:          true,
    RequestHeaders:   true,
    MessageFieldName: "message",
    // TimeFieldFormat: time.RFC850,
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
  r.Use(httplog.RequestLogger(logger))
  r.Use(middleware.Heartbeat("/ping"))

  r.Use(func(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
      ctx := r.Context()
      httplog.LogEntrySetField(ctx, "user", slog.StringValue("user1"))
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
    oplog.Error("msg here", "err", errors.New("err here"))
    w.WriteHeader(500)
    w.Write([]byte("oops, err"))
  })

  http.ListenAndServe("localhost:8000", r)
}

```

## License

MIT
