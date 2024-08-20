package httplog

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"runtime"
	"strings"
)

type Handler struct {
	slog.Handler
	opts  *Options
	attrs []slog.Attr
}

func NewHandler(handler slog.Handler, opts *Options) *Handler {
	return &Handler{
		Handler: handler,
		opts:    opts,
	}
}

func (h *Handler) Enabled(ctx context.Context, level slog.Level) bool {
	if log, ok := ctx.Value(logCtxKey{}).(*Log); ok {
		return level >= log.Level
	}

	return level >= h.opts.Level
}

func (h *Handler) Handle(ctx context.Context, r slog.Record) error {
	log, ok := ctx.Value(logCtxKey{}).(*Log)
	if !ok {
		// Panic to stress test the use of this handler. Later, we can return error.
		panic("use of httplog.DefaultHandler outside of context set by http.RequestLogger middleware")
	}

	if h.opts.Concise {
		r.AddAttrs(slog.Any("requestHeaders", slog.GroupValue(getHeaderAttrs(log.Req.Header, h.opts.ReqHeaders)...)))
		if h.opts.ReqBody && log.Resp != nil {
			r.AddAttrs(slog.String("requestBody", log.Req.Body.String()))
			if !log.Req.BodyFullyRead {
				r.AddAttrs(slog.Bool("requestBodyFullyRead", false))
			}
		}

		if log.Resp != nil {
			r.Message = fmt.Sprintf("HTTP %v (%v): %s %s", log.Resp.Status, log.Resp.Duration, log.Req.Method, log.Req.URL)
			r.AddAttrs(slog.Any("responseHeaders", slog.GroupValue(getHeaderAttrs(log.Resp.Header(), h.opts.RespHeaders)...)))
			if h.opts.RespBody {
				r.AddAttrs(slog.String("responseBody", log.Resp.Body.String()))
			}
		} else {
			r.Message = fmt.Sprintf("%s %s://%s%s", log.Req.Method, log.Req.Scheme, log.Req.Host, log.Req.URL)
		}
	} else {
		reqAttrs := []slog.Attr{
			slog.String("url", fmt.Sprintf("%s://%s%s", log.Req.Scheme, log.Req.Host, log.Req.URL)),
			slog.String("method", log.Req.Method),
			slog.String("path", log.Req.URL.Path),
			slog.String("remoteIp", log.Req.RemoteAddr),
			slog.String("proto", log.Req.Proto),
			slog.Any("headers", slog.GroupValue(getHeaderAttrs(log.Req.Header, h.opts.ReqHeaders)...)),
		}
		if h.opts.ReqBody && log.Resp != nil {
			reqAttrs = append(reqAttrs, slog.String("body", log.Req.Body.String()))
			if !log.Req.BodyFullyRead {
				reqAttrs = append(reqAttrs, slog.Bool("bodyFullyRead", false))
			}
		}
		r.AddAttrs(slog.Any("request", slog.GroupValue(reqAttrs...)))

		if log.Resp != nil {
			respAttrs := []slog.Attr{
				slog.Any("headers", slog.GroupValue(getHeaderAttrs(log.Resp.Header(), h.opts.RespHeaders)...)),
				slog.Int("status", log.Resp.Status),
				slog.Int("bytes", log.Resp.Bytes),
				slog.Float64("duration", float64(log.Resp.Duration.Nanoseconds()/1000000.0)), // in milliseconds
			}
			if h.opts.RespBody {
				respAttrs = append(respAttrs, slog.String("body", log.Resp.Body.String()))
			}
			r.AddAttrs(slog.Any("response", slog.GroupValue(respAttrs...)))
		}
	}

	if log.Panic != nil {
		// Process panic stack frames to print detailed information.
		frames := runtime.CallersFrames(log.PanicPC)
		var stackValues []string
		for {
			frame, more := frames.Next()
			if !strings.Contains(frame.File, "runtime/panic.go") {
				stackValues = append(stackValues, fmt.Sprintf("%s:%d", frame.File, frame.Line))
			}
			if !more {
				break
			}
		}
		r.AddAttrs(
			slog.Any("panic", log.Panic),
			slog.Any("panicStack", stackValues),
		)
	}

	r.AddAttrs(h.attrs...)
	r.AddAttrs(log.Attrs...)

	return h.Handler.Handle(ctx, r)
}

func (c *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	clone := c.clone()
	clone.attrs = append(clone.attrs, attrs...)
	return clone
}

func (c *Handler) clone() *Handler {
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
