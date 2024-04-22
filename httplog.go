package httplog

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type Logger struct {
	*slog.Logger
	Options Options
}

func NewLogger(serviceName string, options ...Options) *Logger {
	logger := &Logger{}
	if len(options) > 0 {
		logger.Configure(options[0])
	} else {
		logger.Configure(defaultOptions)
	}

	slogger := logger.Logger.With(slog.Attr{Key: "service", Value: slog.StringValue(serviceName)})

	if !logger.Options.Concise && len(logger.Options.Tags) > 0 {
		group := []any{}
		for k, v := range logger.Options.Tags {
			group = append(group, slog.Attr{Key: k, Value: slog.StringValue(v)})
		}
		slogger = slogger.With(slog.Group("tags", group...))
	}

	logger.Logger = slogger
	return logger
}

// RequestLogger is an http middleware to log http requests and responses.
//
// NOTE: for simplicity, RequestLogger automatically makes use of the chi RequestID and
// Recoverer middleware.
func RequestLogger(logger *Logger, skipPaths ...[]string) func(next http.Handler) http.Handler {
	return chi.Chain(
		middleware.RequestID,
		Handler(logger, skipPaths...),
		middleware.Recoverer,
	).Handler
}

func Handler(logger *Logger, optSkipPaths ...[]string) func(next http.Handler) http.Handler {
	var f middleware.LogFormatter = &requestLogger{logger.Logger, logger.Options}

	skipPaths := map[string]struct{}{}
	if len(optSkipPaths) > 0 {
		for _, path := range optSkipPaths[0] {
			skipPaths[path] = struct{}{}
		}
	}

	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			// Skip the logger if the path is in the skip list
			if len(skipPaths) > 0 {
				_, skip := skipPaths[r.URL.Path]
				if skip {
					next.ServeHTTP(w, r)
					return
				}
			}

			if rInCooldown(r, &logger.Options) {
				next.ServeHTTP(w, r)
				return
			}

			entry := f.NewLogEntry(r)
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			buf := newLimitBuffer(512)
			ww.Tee(buf)

			t1 := time.Now()
			defer func() {
				var respBody []byte
				if ww.Status() >= 400 {
					respBody, _ = io.ReadAll(buf)
				}
				entry.Write(ww.Status(), ww.BytesWritten(), ww.Header(), time.Since(t1), respBody)
			}()

			next.ServeHTTP(ww, middleware.WithLogEntry(r, entry))
		}
		return http.HandlerFunc(fn)
	}
}

type requestLogger struct {
	Logger  *slog.Logger
	Options Options
}

func (l *requestLogger) NewLogEntry(r *http.Request) middleware.LogEntry {
	entry := &RequestLoggerEntry{l.Logger, l.Options, ""}
	msg := fmt.Sprintf("Request: %s %s", r.Method, r.URL.Path)

	if l.Options.RequestHeaders {
		entry.Logger = l.Logger.With(requestLogFields(r, l.Options, true))
	} else {
		entry.Logger = l.Logger.With(requestLogFields(r, l.Options, false))
	}

	if !l.Options.Concise {
		entry.Logger.Info(msg)
	}
	return entry
}

type RequestLoggerEntry struct {
	Logger  *slog.Logger
	Options Options
	msg     string
}

func (l *RequestLoggerEntry) Write(status, bytes int, header http.Header, elapsed time.Duration, extra interface{}) {
	msg := fmt.Sprintf("Response: %d %s", status, statusLabel(status))
	if l.msg != "" {
		msg = fmt.Sprintf("%s - %s", msg, l.msg)
	}

	responseLog := []any{
		slog.Attr{Key: "status", Value: slog.IntValue(status)},
		slog.Attr{Key: "bytes", Value: slog.IntValue(bytes)},
		slog.Attr{Key: "elapsed", Value: slog.Float64Value(float64(elapsed.Nanoseconds()) / 1000000.0)}, // in milliseconds
	}

	if !l.Options.Concise {
		// Include response header, as well for error status codes (>400) we include
		// the response body so we may inspect the log message sent back to the client.
		if status >= 400 {
			body, _ := extra.([]byte)
			responseLog = append(responseLog, slog.Attr{Key: "body", Value: slog.StringValue(string(body))})
		}
		if l.Options.ResponseHeaders && len(header) > 0 {
			responseLog = append(responseLog, slog.Group("header", attrsToAnys(headerLogField(header, l.Options))...))
		}
	}

	l.Logger.With(slog.Group("httpResponse", responseLog...)).Log(context.Background(), statusLevel(status), msg)
}

func (l *RequestLoggerEntry) Panic(v interface{}, stack []byte) {
	stacktrace := "#"
	if l.Options.JSON {
		stacktrace = string(stack)
	}
	l.Logger = l.Logger.With(
		slog.Attr{
			Key:   "stacktrace",
			Value: slog.StringValue(stacktrace)},
		slog.Attr{
			Key:   "panic",
			Value: slog.StringValue(fmt.Sprintf("%+v", v)),
		})

	l.msg = fmt.Sprintf("%+v", v)

	if !l.Options.JSON {
		middleware.PrintPrettyStack(v)
	}
}

var coolDownMu sync.RWMutex
var coolDowns = map[string]time.Time{}

func rInCooldown(r *http.Request, options *Options) bool {
	routePath := r.URL.EscapedPath()
	if routePath == "" {
		routePath = "/"
	}
	if !inArray(options.QuietDownRoutes, routePath) {
		return false
	}
	coolDownMu.RLock()
	coolDownTime, ok := coolDowns[routePath]
	coolDownMu.RUnlock()
	if ok {
		if time.Since(coolDownTime) < options.QuietDownPeriod {
			return true
		}
	}
	coolDownMu.Lock()
	defer coolDownMu.Unlock()
	coolDowns[routePath] = time.Now().Add(options.QuietDownPeriod)
	return false
}

func inArray(arr []string, val string) bool {
	for _, v := range arr {
		if v == val {
			return true
		}
	}
	return false
}

func requestLogFields(r *http.Request, options Options, requestHeaders bool) slog.Attr {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	requestURL := fmt.Sprintf("%s://%s%s", scheme, r.Host, r.RequestURI)

	requestFields := []any{
		slog.Attr{Key: "url", Value: slog.StringValue(requestURL)},
		slog.Attr{Key: "method", Value: slog.StringValue(r.Method)},
		slog.Attr{Key: "path", Value: slog.StringValue(r.URL.Path)},
		slog.Attr{Key: "remoteIP", Value: slog.StringValue(r.RemoteAddr)},
		slog.Attr{Key: "proto", Value: slog.StringValue(r.Proto)},
	}
	if reqID := middleware.GetReqID(r.Context()); reqID != "" {
		requestFields = append(requestFields, slog.Attr{Key: "requestID", Value: slog.StringValue(reqID)})
	}

	if !options.RequestHeaders {
		return slog.Group("httpRequest", requestFields...)
	}

	// include request headers
	requestFields = append(requestFields, slog.Attr{Key: "scheme", Value: slog.StringValue(scheme)})
	if len(r.Header) > 0 {
		requestFields = append(requestFields,
			slog.Attr{
				Key:   "header",
				Value: slog.GroupValue(headerLogField(r.Header, options)...),
			})
	}

	return slog.Group("httpRequest", requestFields...)
}

func headerLogField(header http.Header, options Options) []slog.Attr {
	headerField := []slog.Attr{}
	for k, v := range header {
		k = strings.ToLower(k)
		switch {
		case len(v) == 0:
			continue
		case len(v) == 1:
			headerField = append(headerField, slog.Attr{Key: k, Value: slog.StringValue(v[0])})
		default:
			headerField = append(headerField, slog.Attr{Key: k,
				Value: slog.StringValue(fmt.Sprintf("[%s]", strings.Join(v, "], [")))})
		}
		if k == "authorization" || k == "cookie" || k == "set-cookie" {
			headerField[len(headerField)-1] = slog.Attr{
				Key:   k,
				Value: slog.StringValue("***"),
			}
		}

		for _, skip := range options.HideRequestHeaders {
			if k == skip {
				headerField[len(headerField)-1] = slog.Attr{
					Key:   k,
					Value: slog.StringValue("***"),
				}
				break
			}
		}
	}
	return headerField
}

func attrsToAnys(attr []slog.Attr) []any {
	attrs := make([]any, len(attr))
	for i, a := range attr {
		attrs[i] = a
	}
	return attrs
}

func statusLevel(status int) slog.Level {
	switch {
	case status <= 0:
		return slog.LevelWarn
	case status < 400: // for codes in 100s, 200s, 300s
		return slog.LevelInfo
	case status >= 400 && status < 500:
		// switching to info level to be less noisy
		return slog.LevelInfo
	case status >= 500:
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func statusLabel(status int) string {
	switch {
	case status >= 100 && status < 300:
		return "OK"
	case status >= 300 && status < 400:
		return "Redirect"
	case status >= 400 && status < 500:
		return "Client Error"
	case status >= 500:
		return "Server Error"
	default:
		return "Unknown"
	}
}

func ErrAttr(err error) slog.Attr {
	return slog.Any("err", err)
}

// Helper methods used by the application to get the request-scoped
// logger entry and set additional fields between handlers.
//
// This is a useful pattern to use to set state on the entry as it
// passes through the handler chain, which at any point can be logged
// with a call to .Print(), .Info(), etc.

func LogEntry(ctx context.Context) *slog.Logger {
	entry, ok := ctx.Value(middleware.LogEntryCtxKey).(*RequestLoggerEntry)
	if !ok || entry == nil {
		handlerOpts := &slog.HandlerOptions{
			AddSource: true,
			// LevelError+1 will be higher than all levels
			// hence logs would be skipped
			Level: slog.LevelError + 1,
			// ReplaceAttr: func(attr slog.Attr) slog.Attr ,
		}
		return slog.New(slog.NewTextHandler(os.Stdout, handlerOpts))
	} else {
		return entry.Logger
	}
}

func LogEntrySetField(ctx context.Context, key string, value slog.Value) {
	if entry, ok := ctx.Value(middleware.LogEntryCtxKey).(*RequestLoggerEntry); ok {
		entry.Logger = entry.Logger.With(slog.Attr{Key: key, Value: value})
	}
}

func LogEntrySetFields(ctx context.Context, fields map[string]interface{}) {
	if entry, ok := ctx.Value(middleware.LogEntryCtxKey).(*RequestLoggerEntry); ok {
		attrs := make([]slog.Attr, len(fields))
		i := 0
		for k, v := range fields {
			attrs[i] = slog.Attr{Key: k, Value: slog.AnyValue(v)}
			i++
		}
		entry.Logger = entry.Logger.With(attrs)
	}
}
