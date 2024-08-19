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
	// Level defines the verbosity of the requests logs:
	// slog.LevelDebug - log both request starts & responses (incl. OPTIONS)
	// slog.LevelInfo  - log responses (excl. OPTIONS)
	// slog.LevelWarn  - log only 4xx and 5xx responses (except for 429)
	// slog.LevelError - log only 5xx responses only
	Level slog.Level

	// Concise mode includes fewer log attributes details during the request flow. For example
	// excluding details like request content length, user-agent and other details.
	// This is useful if your console is too noisy during development.
	Consise bool

	// RequestHeaders is an explicit list of headers to be logged in request.headers attribute group.
	RequestHeaders []string

	// ResponseHeaders is an explicit list of headers to be logged in response.headers attribute group.
	ResponseHeaders []string
}

var defaultOptions = Options{
	Level:           slog.LevelInfo,
	Consise:         false,
	RequestHeaders:  []string{"User-Agent", "Referer"},
	ResponseHeaders: []string{""},
}

func (l *Logger) Handle(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}

		logValue := &Log{
			Level: l.opts.Level,
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
		l.DebugContext(ctx, "Request started")

		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

		defer func() {
			status := ww.Status()

			logValue.Resp = &ResponseLog{
				Header:   w.Header,
				Status:   status,
				Bytes:    ww.BytesWritten(),
				Duration: time.Since(start),
			}

			lvl := slog.LevelInfo
			if r.Method == "OPTIONS" {
				lvl = slog.LevelDebug
			}
			if status >= 500 {
				lvl = slog.LevelError
			} else if status == 429 {
				lvl = slog.LevelInfo
			} else if status >= 400 {
				lvl = slog.LevelWarn
			}

			l.LogAttrs(ctx, lvl, "Request finished")
		}()

		next.ServeHTTP(ww, r.WithContext(ctx))
	})
}

type Log struct {
	Level slog.Level   // Use httplog.SetLevel(ctx, slog.DebugLevel) to override level
	Attrs []slog.Attr  // Use httplog.SetAttrs(ctx, slog.String("key", "value")) to append
	Req   RequestLog   // Automatically set on request start by httplog.RequestLogger middleware
	Resp  *ResponseLog // Automatically set on request end by httplog.RequestLogger middleware
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
