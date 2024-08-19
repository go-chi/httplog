package httplog

import (
	"context"
	"log/slog"
)

type logCtxKey struct{}

func (c *logCtxKey) String() string {
	return "httplog log context"
}

// SetAttrs sets additional attributes to the request log.
//
// NOTE: Not safe for concurrent access. Don't use outside of HTTP request goroutine.
func SetAttrs(ctx context.Context, attrs ...slog.Attr) {
	log, ok := ctx.Value(logCtxKey{}).(*Log)
	if !ok {
		// Panic to stress test the use of this function. Later, we can return error.
		panic("use of httplog.SetAttrs() outside of context set by httplog.RequestLogger")
	}
	log.Attrs = append(log.Attrs, attrs...)
}

// SetLevel overrides default request log level for this request. Useful for overriding
// log level in a middleware, eg. to force log a group of /admin/* endpoints or for privileged sessions.
//
// NOTE: Not safe for concurrent access. Don't use outside of HTTP request goroutine.
func SetLevel(ctx context.Context, level slog.Level) {
	log, ok := ctx.Value(logCtxKey{}).(*Log)
	if !ok {
		// Panic to stress test the use of this function. Later, we can return error.
		panic("use of httplog.SetLevel() outside of context set by httplog.RequestLogger")
	}
	log.Level = level
}
