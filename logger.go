package httplog

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"runtime"
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

	// Concise mode causes fewer log attributes to be printed in request logs.
	// This is useful if your console is too noisy during development.
	Concise bool

	// RecoverPanics recovers from panics caused in the underlying HTTP handlers
	// and middlewares. It returns HTTP 500 unless response status was already set.
	//
	// NOTE: The request logger automatically logs all panics, regardless of this setting.
	RecoverPanics bool

	// ReqHeaders is an explicit list of headers to be logged as attributes.
	ReqHeaders []string

	// ReqBody enables logging of request body into a response log attribute.
	ReqBody bool

	// RespHeaders is an explicit list of headers to be logged as attributes.
	RespHeaders []string

	// RespBody enables logging of response body into a response log attribute.
	RespBody bool
}

var defaultOptions = Options{
	Level:         slog.LevelInfo,
	Concise:       false,
	RecoverPanics: true,
	ReqHeaders:    []string{"User-Agent", "Referer", "Origin"},
	RespHeaders:   []string{""},
}

func (l *Logger) Handle(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}

		log := &Log{
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

		ctx = context.WithValue(ctx, logCtxKey{}, log)
		l.DebugContext(ctx, "Request started")

		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

		var reqBody bytes.Buffer
		if l.opts.ReqBody {
			r.Body = io.NopCloser(io.TeeReader(r.Body, &reqBody))
		}

		var respBody bytes.Buffer
		if l.opts.RespBody {
			ww.Tee(&respBody)
		}

		defer func() {
			if rec := recover(); rec != nil {
				if rec != http.ErrAbortHandler {
					pc := make([]uintptr, 10)   // Capture up to 10 stack frames.
					n := runtime.Callers(3, pc) // Skip 3 frames (this middleware + runtime/panic.go).

					log.Panic = rec
					log.PanicPC = pc[:n]
				}

				// Return HTTP 500 if recover is enabled and no response status was set.
				if l.opts.RecoverPanics && ww.Status() == 0 && r.Header.Get("Connection") != "Upgrade" {
					ww.WriteHeader(http.StatusInternalServerError)
				}

				if rec == http.ErrAbortHandler || !l.opts.RecoverPanics {
					// Always re-panic http.ErrAbortHandler. Re-panic everything unless recover is enabled.
					defer panic(rec)
				}
			}

			status := ww.Status()

			log.Resp = &ResponseLog{
				Header:   w.Header,
				Status:   status,
				Bytes:    ww.BytesWritten(),
				Duration: time.Since(start),
				Body:     respBody,
			}

			if l.opts.ReqBody {
				// Make sure to read full request body if the underlying handler didn't do so.
				n, _ := io.Copy(io.Discard, r.Body)
				if n == 0 {
					log.Req.BodyFullyRead = true
				}
				log.Req.Body = reqBody
			}

			if l.opts.RespBody {
				log.Resp.Body = respBody
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
	Level   slog.Level   // Use httplog.SetLevel(ctx, slog.DebugLevel) to override level
	Attrs   []slog.Attr  // Use httplog.SetAttrs(ctx, slog.String("key", "value")) to append
	Req     RequestLog   // Automatically set on request start by httplog.RequestLogger middleware
	Resp    *ResponseLog // Automatically set on request end by httplog.RequestLogger middleware
	Panic   any
	PanicPC []uintptr
}

type RequestLog struct {
	Scheme        string
	Method        string
	Host          string
	URL           *url.URL
	Header        http.Header
	RemoteAddr    string
	Proto         string
	Body          bytes.Buffer
	BodyFullyRead bool
}

type ResponseLog struct {
	Header   func() http.Header
	Status   int
	Bytes    int
	Duration time.Duration
	Body     bytes.Buffer
}

// DebugContext calls [Logger.DebugContext] on the default logger.
func DebugContext(ctx context.Context, msg string, args ...any) {
	slog.Default().DebugContext(ctx, msg, args...)
}

// InfoContext calls [Logger.InfoContext] on the default logger.
func InfoContext(ctx context.Context, msg string, args ...any) {
	slog.Default().InfoContext(ctx, msg, args...)
}

// WarnContext calls [Logger.WarnContext] on the default logger.
func WarnContext(ctx context.Context, msg string, args ...any) {
	slog.Default().WarnContext(ctx, msg, args...)
}

// ErrorContext calls [Logger.ErrorContext] on the default logger.
func ErrorContext(ctx context.Context, msg string, args ...any) {
	slog.Default().ErrorContext(ctx, msg, args...)
}

// LogAttrs calls [Logger.LogAttrs] on the default logger.
func LogAttrs(ctx context.Context, level slog.Level, msg string, attrs ...slog.Attr) {
	slog.Default().LogAttrs(ctx, level, msg, attrs...)
}
