package httplog

import (
	"context"
	"log/slog"
	"sync"
)

const (
	ErrorKey = "error"
)

type ctxKeyLogAttrs struct{}

func (c *ctxKeyLogAttrs) String() string {
	return "httplog attrs context"
}

// logAttrs holds request-scoped log attributes. SetAttrs may be called
// from multiple goroutines on the same request (e.g. GraphQL subscriptions
// or fan-out handlers), so the slice is mutex-protected.
type logAttrs struct {
	mu    sync.Mutex
	attrs []slog.Attr
}

func newLogAttrs() *logAttrs {
	return &logAttrs{}
}

// SetAttrs sets the attributes on the request log.
// It is safe for concurrent use on the same request context.
func SetAttrs(ctx context.Context, attrs ...slog.Attr) {
	if bag, ok := ctx.Value(ctxKeyLogAttrs{}).(*logAttrs); ok && bag != nil {
		bag.mu.Lock()
		bag.attrs = append(bag.attrs, attrs...)
		bag.mu.Unlock()
	}
}

func getAttrs(ctx context.Context) []slog.Attr {
	if bag, ok := ctx.Value(ctxKeyLogAttrs{}).(*logAttrs); ok && bag != nil {
		bag.mu.Lock()
		defer bag.mu.Unlock()
		out := make([]slog.Attr, len(bag.attrs))
		copy(out, bag.attrs)
		return out
	}

	return nil
}

// SetError sets the error attribute on the request log.
func SetError(ctx context.Context, err error) error {
	if err != nil {
		SetAttrs(ctx, slog.Any(ErrorKey, err))
	}

	return err
}
