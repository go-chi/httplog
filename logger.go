package reqslog

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

func RequestLogger(logger *slog.Logger, opts *Options) func(next http.Handler) http.Handler {
	return NewRequestLogger(logger, opts).Handle
}

func NewRequestLogger(logger *slog.Logger, opts *Options) *Logger {
	if opts == nil {
		opts = &defaultOptions
	}

	return &Logger{
		logger: logger,
		opts:   *opts,
	}
}

type Logger struct {
	logger *slog.Logger
	opts   Options
}

type Options struct {
	LogRequestHeaders  []string
	LogRequestStart    bool
	LogResponseHeaders []string
}

var defaultOptions = Options{
	LogRequestHeaders:  []string{"User-Agent", "Referer"},
	LogRequestStart:    false,
	LogResponseHeaders: []string{""},
}

func (l *Logger) Handle(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		ctx := r.Context()
		// ctx = context.WithValue(loggerAttrs, ..)

		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}

		req := slog.GroupValue(
			slog.String("url", fmt.Sprintf("%s://%s%s", scheme, r.Host, r.RequestURI)),
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.String("remoteIp", r.RemoteAddr),
			slog.String("proto", r.Proto),
			slog.Any("headers", slog.GroupValue(getHeaderAttrs(r.Header, l.opts.LogRequestHeaders)...)),
		)

		if l.opts.LogRequestStart {
			l.logger.With(
				"request", req,
			).InfoContext(ctx, "Request")
		}

		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

		defer func() {
			resp := slog.GroupValue(
				slog.Any("headers", slog.GroupValue(getHeaderAttrs(w.Header(), l.opts.LogResponseHeaders)...)),
				slog.Int("status", ww.Status()),
				slog.Any("duration", time.Since(start)),
			)

			l.logger.With(
				"request", req,
				"response", resp,
			).InfoContext(ctx, "Response")
		}()

		next.ServeHTTP(ww, r.WithContext(ctx))
	})
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
