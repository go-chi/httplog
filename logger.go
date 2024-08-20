package httplog

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
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

	handler := NewHandler(logger.Handler(), opts)

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
	// Level defines the verbosity of the request logs:
	// slog.LevelDebug - log both request starts & responses (incl. OPTIONS)
	// slog.LevelInfo  - log responses (excl. OPTIONS)
	// slog.LevelWarn  - log 4xx and 5xx responses only (except for 429)
	// slog.LevelError - log 5xx responses only
	Level slog.Level

	// Concise mode causes fewer log attributes to be printed in request logs.
	// This is useful if your console is too noisy during development.
	Concise bool

	// RecoverPanics recovers from panics occurring in the underlying HTTP handlers
	// and middlewares. It returns HTTP 500 unless response status was already set.
	//
	// NOTE: The request logger logs all panics automatically, regardless of this setting.
	RecoverPanics bool

	// LogRequestHeaders is an explicit list of headers to be logged as attributes.
	LogRequestHeaders []string

	// LogRequestBody enables logging of request body into a response log attribute.
	LogRequestBody bool

	// LogResponseHeaders is an explicit list of headers to be logged as attributes.
	LogResponseHeaders []string

	// LogResponseBody enables logging of response body into a response log attribute.
	LogResponseBody bool
}

var defaultOptions = Options{
	Level:              slog.LevelInfo,
	Concise:            false,
	RecoverPanics:      true,
	LogRequestHeaders:  []string{"User-Agent", "Referer", "Origin"},
	LogResponseHeaders: []string{""},
}

func (l *Logger) Handle(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		log := &Log{
			Level:           l.opts.Level,
			Req:             r,
			LogRequestBody:  l.opts.LogRequestBody,
			LogResponseBody: l.opts.LogResponseBody,
		}

		ctx = context.WithValue(ctx, logCtxKey{}, log)
		l.DebugContext(ctx, "Request started")

		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		log.WW = ww

		if log.LogRequestBody {
			r.Body = io.NopCloser(io.TeeReader(r.Body, &log.ReqBody))
		}
		if log.LogResponseBody {
			ww.Tee(&log.RespBody)
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
				Duration: time.Since(start),
			}

			if log.LogRequestBody {
				// Make sure to read full request body if the underlying handler didn't do so.
				n, _ := io.Copy(io.Discard, r.Body)
				if n == 0 {
					log.ReqBodyFullyRead = true
				}
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
	Level           slog.Level  // Use httplog.SetLevel(ctx, slog.DebugLevel) to override level
	Attrs           []slog.Attr // Use httplog.SetAttrs(ctx, slog.String("key", "value")) to append
	LogRequestBody  bool        // Use httplog.LogRequestBody(ctx) to force-enable
	LogResponseBody bool        // Use httplog.LogResponseBody(ctx) to force-enable

	// Fields automatically set by httplog.RequestLogger middleware:
	Req              *http.Request
	ReqBody          bytes.Buffer
	ReqBodyFullyRead bool
	WW               middleware.WrapResponseWriter
	Resp             *ResponseLog
	RespBody         bytes.Buffer
	Panic            any
	PanicPC          []uintptr
}

type ResponseLog struct {
	Header   func() http.Header
	Duration time.Duration
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
