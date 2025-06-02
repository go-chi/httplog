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
	//
	// Use httplog.SetLevel(ctx, slog.DebugLevel) to override the level per-request.
	Level slog.Level

	// Concise mode causes fewer log attributes to be printed in request logs.
	// This is useful if your console is too noisy during development.
	Concise bool

	// RecoverPanics recovers from panics occurring in the underlying HTTP handlers
	// and middlewares and returns HTTP 500 unless response status was already set.
	//
	// NOTE: Panics are logged as errors automatically, regardless of this setting.
	RecoverPanics bool

	// LogRequestHeaders is an explicit list of headers to be logged as attributes.
	// If not provided, the default headers are User-Agent, Referer and Origin.
	LogRequestHeaders []string

	// LogRequestBody enables logging of request body into a response log attribute.
	//
	// Use httplog.LogRequestBody(ctx) to enable on per-request basis instead.
	LogRequestBody bool

	// LogRequestCURL enables logging of request body incl. all headers as a CURL command.
	//
	// Use httplog.LogRequestCURL(ctx) to enable on per-request basis instead.
	LogRequestCURL bool

	// LogResponseHeaders is an explicit list of headers to be logged as attributes.
	//
	// If not provided, there are no default headers.
	LogResponseHeaders []string

	// LogResponseBody enables logging of response body into a response log attribute.
	// The Content-Type of the response must match.
	//
	// Use httplog.LogResponseBody(ctx) to enable on per-request basis instead.
	LogResponseBody bool

	// LogResponseBodyContentType defines list of Content-Types for which LogResponseBody is enabled.
	//
	// If not provided, the default list is application/json and text/plain.
	LogResponseBodyContentType []string
}

var defaultOptions = Options{
	Level:                      slog.LevelInfo,
	RecoverPanics:              true,
	LogRequestHeaders:          []string{"User-Agent", "Referer", "Origin"},
	LogResponseHeaders:         []string{""},
	LogResponseBodyContentType: []string{"application/json", "text/plain"},
}

func (l *Logger) Handle(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		log := &log{
			Level:           l.opts.Level,
			Req:             r,
			LogRequestCURL:  l.opts.LogRequestCURL,
			LogRequestBody:  l.opts.LogRequestBody || l.opts.LogRequestCURL,
			LogResponseBody: l.opts.LogResponseBody,
		}

		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		log.WW = ww

		ctx = context.WithValue(ctx, logCtxKey{}, log)
		l.DebugContext(ctx, "Request started")

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

			log.Resp = &respLog{
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

type log struct {
	Level           slog.Level
	Attrs           []slog.Attr
	LogRequestCURL  bool
	LogRequestBody  bool
	LogResponseBody bool

	// Fields set by httplog.RequestLogger middleware:
	Req              *http.Request
	ReqBody          bytes.Buffer
	ReqBodyFullyRead bool
	WW               middleware.WrapResponseWriter
	Resp             *respLog
	RespBody         bytes.Buffer
	Panic            any
	PanicPC          []uintptr

	// Fields internal to httplog:
}

type respLog struct {
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
