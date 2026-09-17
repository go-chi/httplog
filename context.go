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

type logData struct {
	mu    sync.RWMutex
	attrs []slog.Attr
}

// SetAttrs sets the attributes on the request log.
func SetAttrs(ctx context.Context, attrs ...slog.Attr) {
	if ptr, ok := ctx.Value(ctxKeyLogAttrs{}).(*logData); ok && ptr != nil {
		ptr.mu.Lock()
		defer ptr.mu.Unlock()
		ptr.attrs = append(ptr.attrs, attrs...)
	}
}

func lockData(ctx context.Context) {
	if ptr, ok := ctx.Value(ctxKeyLogAttrs{}).(*logData); ok && ptr != nil {
		ptr.mu.RLock()
	}
}

func unlockData(ctx context.Context) {
	if ptr, ok := ctx.Value(ctxKeyLogAttrs{}).(*logData); ok && ptr != nil {
		ptr.mu.RUnlock()
	}
}

func getAttrs(ctx context.Context) []slog.Attr {
	if ptr, ok := ctx.Value(ctxKeyLogAttrs{}).(*logData); ok && ptr != nil {
		return ptr.attrs
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
