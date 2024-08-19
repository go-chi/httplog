# HTTP request logger

- Middleware which prints single `slog.Info()` line per request/response
- Allows attaching log attributes from downstream HTTP handlers / middlewares (this is otherwise not possible, as the middleware doesn't see context values set by later downstream)
- Allows debugging request/response body
- Allows enabling/disabling request logs and debugging per your own rules