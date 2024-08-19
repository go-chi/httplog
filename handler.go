package httplog

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
)

type DefaultHandler struct {
	slog.Handler
	level slog.Level
	opts  *Options
	attrs []slog.Attr
}

func (h *DefaultHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= h.level
}

func (h *DefaultHandler) Handle(ctx context.Context, r slog.Record) error {
	log, ok := ctx.Value(logCtxKey{}).(*Log)
	if !ok {
		panic("fuuu.. did you use httplog.DefaultHandler without http.RequestLogger middleware")
		return h.Handler.Handle(ctx, r)
	}

	req := slog.GroupValue(
		slog.String("url", fmt.Sprintf("%s://%s%s", log.Req.Scheme, log.Req.Host, log.Req.URL)),
		slog.String("method", log.Req.Method),
		slog.String("path", log.Req.URL.Path),
		slog.String("remoteIp", log.Req.RemoteAddr),
		slog.String("proto", log.Req.Proto),
		slog.Any("headers", slog.GroupValue(getHeaderAttrs(log.Req.Header, h.opts.LogRequestHeaders)...)),
	)

	r.AddAttrs(slog.Any("request", req))

	resp := slog.GroupValue(
		slog.Any("headers", slog.GroupValue(getHeaderAttrs(log.Resp.Header(), h.opts.LogResponseHeaders)...)),
		slog.Int("status", log.Resp.Status),
		slog.Any("duration", log.Resp.Duration),
	)

	r.AddAttrs(slog.Any("response", resp))

	r.AddAttrs(h.attrs...)
	r.AddAttrs(log.Attrs...)

	return h.Handler.Handle(ctx, r)
}

func (c *DefaultHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	clone := c.clone()
	clone.attrs = append(clone.attrs, attrs...)
	return clone
}

func (c *DefaultHandler) clone() *DefaultHandler {
	clone := *c
	return &clone
}

func getHeaderAttrs(header http.Header, headers []string) []slog.Attr {
	attrs := make([]slog.Attr, 0, len(headers))
	for _, h := range headers {
		vals := header.Values(h)
		if len(vals) == 1 {
			attrs = append(attrs, slog.String(h, vals[0]))
		} else if len(vals) > 1 {
			attrs = append(attrs, slog.Any(h, vals))
		}
	}
	return attrs
}
