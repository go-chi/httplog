package httplog

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
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

	handler := &DefaultHandler{
		Handler: logger.Handler(),
		level:   opts.Level,
		opts:    opts,
	}

	return &Logger{
		Logger: slog.New(handler),
		opts:   opts,
	}
}

type Logger struct {
	*slog.Logger
	opts *Options
}

type Options struct {
	Level slog.Level

	// Idea: Let users enable/disable request log per their own rules (e.g. force logs for admins).
	// EnableLog func(r *http.Request) bool
	//
	// Or should this be a context-aware function, e.g. httplog.EnableLog(ctx), which you can call in any handler/middleware?

	LogRequestHeaders  []string
	LogRequestStart    bool
	LogResponseHeaders []string
}

var defaultOptions = Options{
	Level:              slog.LevelInfo,
	LogRequestHeaders:  []string{"User-Agent", "Referer"},
	LogRequestStart:    false,
	LogResponseHeaders: []string{""},
}

func (l *Logger) Handle(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}

		logValue := &Log{
			Req: RequestLog{
				Scheme:     scheme,
				Method:     r.Method,
				Host:       r.Host,
				URL:        r.URL,
				Header:     r.Header,
				RemoteAddr: r.RemoteAddr,
				Proto:      r.Proto,
			},
		}

		ctx = context.WithValue(ctx, logCtxKey{}, logValue)

		if l.opts.LogRequestStart {
			l.InfoContext(ctx, "Request started")
		}

		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

		defer func() {
			logValue.Resp = ResponseLog{
				Header:   w.Header,
				Status:   ww.Status(),
				Bytes:    ww.BytesWritten(),
				Duration: time.Since(start),
			}

			l.InfoContext(ctx, "Request finished")
		}()

		next.ServeHTTP(ww, r.WithContext(ctx))
	})
}

type Log struct {
	Level slog.Level  // httplog.SetLevel(ctx, slog.DebugLevel)
	Attrs []slog.Attr // httplog.SetAttrs(ctx, slog.String("key", "value"))
	Req   RequestLog  // Set automatically in httplog.RequestLogger middleware at the start of request
	Resp  ResponseLog // Set automatically in httplog.RequestLogger middleware at the end of request
}

type RequestLog struct {
	Scheme     string
	Method     string
	Host       string
	URL        *url.URL
	Header     http.Header
	RemoteAddr string
	Proto      string
	Body       []byte
}

type ResponseLog struct {
	Header   func() http.Header
	Status   int
	Bytes    int
	Duration time.Duration
	Body     []byte
}
