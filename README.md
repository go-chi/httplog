# HTTP request logger

- HTTP middleware that logs single structured log per request/response
- Implements frontend (`slog.Logger`) and backend (`slog.Handler`) separately to improve critical performance of serving HTTP response
- The provided backend `slog.Handler` is extensible and/or replaceable
- Allows attaching log attributes to the single log line from within downstream HTTP handlers/middlewares

## Example

```go
	// Use JSON logger in production mode.
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil)).With(
		"service", "app-name",
		"version", "v24.8.5",
	)

	r := chi.NewRouter()

	r.Use(httplog.RequestLogger(logger, &httplog.Options{
		// Level define the verbosity of the requests logs.
		Level:         slog.LevelInfo,

        // Consise mode prints less information.
		Concise:       false,

        // Recover panics and respond with HTTP 500.
		RecoverPanics: true,

        // Log request/response headers explicitly.
		ReqHeaders:    []string{"User-Agent", "Origin", "Referer"},
		RespHeaders:   []string{},

		// Log request/request body. Useful for debugging.
		ReqBody:       false,
		RespBody:      false,
	}))

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		httplog.SetAttrs(ctx, slog.String("userId", "id"))

		w.Write([]byte("."))
	})
```

## TODO
- [x] Integrate panic recoverer
- [x] Debug request body
- [x] Debug response body
- [ ] Add example
- [ ] Add tests

## License
MIT
