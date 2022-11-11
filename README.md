httplog
=======

Small but powerful structured logging package for HTTP request logging in Go.

## Example

(see [_example/](./_example/main.go))

```go
package main

import (
  "net/http"
  "github.com/go-chi/chi/v5"
  "github.com/go-chi/chi/v5/middleware"
  "github.com/pcriv/httplogx"
)

func main() {
  logger := zerolog.New(os.Stdout).With().Timestamp().Logger()
  // Options
  options := httplogx.Options{
    // JSON: true,
    Concise: true,
    // SkipHeaders: []string{
    //  "proto",
    // "remoteIP",
    // },
  }


  r := chi.NewRouter()
  r.Use(httplogx.RequestLogger(logger, options))
  r.Use(middleware.Heartbeat("/ping"))

  r.Get("/", func(w http.ResponseWriter, r *http.Request) {
    w.Write([]byte("hello world"))
  })

  r.Get("/panic", func(w http.ResponseWriter, r *http.Request) {
    panic("oh no")
  })

  r.Get("/info", func(w http.ResponseWriter, r *http.Request) {
    oplog := httplogx.LogEntry(r.Context())
    w.Header().Add("Content-Type", "text/plain")
    oplog.Info().Msg("info here")
    w.Write([]byte("info here"))
  })

  r.Get("/warn", func(w http.ResponseWriter, r *http.Request) {
    oplog := httplogx.LogEntry(r.Context())
    oplog.Warn().Msg("warn here")
    w.WriteHeader(400)
    w.Write([]byte("warn here"))
  })

  r.Get("/err", func(w http.ResponseWriter, r *http.Request) {
    oplog := httplogx.LogEntry(r.Context())
    oplog.Error().Msg("err here")
    w.WriteHeader(500)
    w.Write([]byte("err here"))
  })

  http.ListenAndServe(":5555", r)
}
```

## License

MIT
