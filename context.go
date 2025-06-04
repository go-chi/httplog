package httplog

import (
	"context"
	"log/slog"
)

type ctxKeyLogAttrs struct{}

func (c *ctxKeyLogAttrs) String() string {
	return "httplog attrs context"
}

func SetAttrs(ctx context.Context, attrs ...slog.Attr) {
	if ptr, ok := ctx.Value(ctxKeyLogAttrs{}).(*[]slog.Attr); ok && ptr != nil {
		*ptr = append(*ptr, attrs...)
	}
}

func getAttrs(ctx context.Context) []slog.Attr {
	if ptr, ok := ctx.Value(ctxKeyLogAttrs{}).(*[]slog.Attr); ok && ptr != nil {
		return *ptr
	}

	return nil
}
