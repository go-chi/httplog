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
	if log, ok := ctx.Value(logCtxKey{}).(*log); ok {
		return level >= log.Level
	}

	return level >= h.opts.Level
}

func (h *Handler) Handle(ctx context.Context, rec slog.Record) error {
	log, ok := ctx.Value(logCtxKey{}).(*log)
	if !ok {
		// Panic to stress test the use of this handler. Later, we can return error.
		panic("use of httplog.DefaultHandler outside of context set by http.RequestLogger middleware")
	}

	if h.opts.LogRequestCURL {
		rec.AddAttrs(slog.String("curl", log.curl()))
	}

	if h.opts.Concise {
		reqAttrs := []slog.Attr{}
		respAttrs := []slog.Attr{}

		reqAttrs = append(reqAttrs, slog.Any("headers", slog.GroupValue(getHeaderAttrs(log.Req.Header, h.opts.LogRequestHeaders)...)))
		if log.LogRequestBody && log.Resp != nil {
			reqAttrs = append(reqAttrs, slog.String("body", log.ReqBody.String()))
			if !log.ReqBodyFullyRead {
				reqAttrs = append(reqAttrs, slog.Bool("bodyFullyRead", false))
			}
		}

		if log.Resp != nil {
			rec.Message = fmt.Sprintf("%s %s => HTTP %v (%v)", log.Req.Method, log.Req.URL, log.WW.Status(), log.Resp.Duration)
			respAttrs = append(respAttrs, slog.Any("headers", slog.GroupValue(getHeaderAttrs(log.Resp.Header(), h.opts.LogResponseHeaders)...)))
			if log.LogResponseBody {
				respAttrs = append(respAttrs, slog.String("body", log.RespBody.String()))
			}
		} else {
			rec.Message = fmt.Sprintf("%s %s://%s%s", log.Req.Method, log.scheme(), log.Req.Host, log.Req.URL)
		}

		if log.WW.Status() >= 400 {
			rec.AddAttrs(slog.Any("request", slog.GroupValue(reqAttrs...)))
			rec.AddAttrs(slog.Any("response", slog.GroupValue(respAttrs...)))
		}

	} else {
		reqAttrs := []slog.Attr{
			slog.String("url", fmt.Sprintf("%s://%s%s", log.scheme(), log.Req.Host, log.Req.URL)),
			slog.String("method", log.Req.Method),
			slog.String("path", log.Req.URL.Path),
			slog.String("remoteIp", log.Req.RemoteAddr),
			slog.String("proto", log.Req.Proto),
			slog.Any("headers", slog.GroupValue(getHeaderAttrs(log.Req.Header, h.opts.LogRequestHeaders)...)),
		}
		if log.LogRequestBody && log.Resp != nil {
			reqAttrs = append(reqAttrs, slog.String("body", log.ReqBody.String()))
			if !log.ReqBodyFullyRead {
				reqAttrs = append(reqAttrs, slog.Bool("bodyFullyRead", false))
			}
		}
		rec.AddAttrs(slog.Any("request", slog.GroupValue(reqAttrs...)))

		if log.Resp != nil {
			respAttrs := []slog.Attr{
				slog.Any("headers", slog.GroupValue(getHeaderAttrs(log.Resp.Header(), h.opts.LogResponseHeaders)...)),
				slog.Int("status", log.WW.Status()),
				slog.Int("bytes", log.WW.BytesWritten()),
				slog.Float64("duration", float64(log.Resp.Duration.Nanoseconds()/1000000.0)), // in milliseconds
			}
			if log.LogResponseBody {
				respAttrs = append(respAttrs, slog.String("body", log.RespBody.String()))
			}
			rec.AddAttrs(slog.Any("response", slog.GroupValue(respAttrs...)))
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
		rec.AddAttrs(
			slog.Any("panic", log.Panic),
			slog.Any("panicStack", stackValues),
		)
	}

	rec.AddAttrs(h.attrs...)
	rec.AddAttrs(log.Attrs...)

	return h.Handler.Handle(ctx, rec)
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
