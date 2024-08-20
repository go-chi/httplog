# HTTP request logger

- Middleware which prints single `slog.Info()` line per request/response
- Allows attaching log attributes from downstream HTTP handlers / middlewares (this is otherwise not possible, as the middleware doesn't see context values set by later downstream)
- Allows debugging request/response body
- Allows enabling/disabling request logs per request using context helpers

## TODO
- [x] Integrate panic Recoverer
- [x] Debug request body
- [x] Debug response body
- [ ] Robust example

## Example
See ./example/
