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
		panic("httplog.SetAttrs() used outside of context with httplog.RequestLogger")
	}
	log.Attrs = append(log.Attrs, attrs...)
}

// SetLevel overrides default log level. Useful for overriding log level in a middleware,
// eg. log a group of /admin/* endpoints or for privileged sessions.
//
// NOTE: Not safe for concurrent access. Don't use outside of HTTP request goroutine.
func SetLevel(ctx context.Context, level slog.Level) {
	log, ok := ctx.Value(logCtxKey{}).(*Log)
	if !ok {
		panic("httplog.SetAttrs() used outside of context with httplog.RequestLogger")
	}
	log.Level = level
}
