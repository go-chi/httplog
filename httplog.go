package httplog

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"golang.org/x/exp/slog"
)

func NewLogger(serviceName string, opts ...Options) *slog.Logger {
	if len(opts) > 0 {
		Configure(opts[0])
	} else {
		Configure(DefaultOptions)
	}
	logger := slog.With(slog.Attr{Key: "service", Value: slog.StringValue(strings.ToLower(serviceName))})
	// logger := log.With().Str("service", strings.ToLower(serviceName))
	if !DefaultOptions.Concise && len(DefaultOptions.Tags) > 0 {
		group := []slog.Attr{}
		for k, v := range DefaultOptions.Tags {
			group = append(group, slog.Attr{Key: k, Value: slog.StringValue(v)})
		}
		logger = logger.With(slog.Group("tags", group...))
	}
	return logger
}

// RequestLogger is an http middleware to log http requests and responses.
//
// NOTE: for simplicity, RequestLogger automatically makes use of the chi RequestID and
// Recoverer middleware.
func RequestLogger(logger *slog.Logger, skipPaths ...[]string) func(next http.Handler) http.Handler {
	return chi.Chain(
		middleware.RequestID,
		Handler(logger, skipPaths...),
		middleware.Recoverer,
	).Handler
}

func Handler(logger *slog.Logger, optSkipPaths ...[]string) func(next http.Handler) http.Handler {
	var f middleware.LogFormatter = &requestLogger{*logger}

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

			if rInCooldown(r) {
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
	Logger slog.Logger
}

func (l *requestLogger) NewLogEntry(r *http.Request) middleware.LogEntry {
	entry := &RequestLoggerEntry{}
	msg := fmt.Sprintf("Request: %s %s", r.Method, r.URL.Path)
	entry.Logger = *l.Logger.With(requestLogFields(r, true))
	if !DefaultOptions.Concise {
		entry.Logger = *l.Logger.With(requestLogFields(r, DefaultOptions.Concise))
		entry.Logger.Info(msg)
	}
	return entry
}

type RequestLoggerEntry struct {
	Logger slog.Logger
	msg    string
}

func (l *RequestLoggerEntry) Write(status, bytes int, header http.Header, elapsed time.Duration, extra interface{}) {
	msg := fmt.Sprintf("Response: %d %s", status, statusLabel(status))
	if l.msg != "" {
		msg = fmt.Sprintf("%s - %s", msg, l.msg)
	}

	responseLog := []slog.Attr{
		{Key: "status", Value: slog.IntValue(status)},
		{Key: "bytes", Value: slog.IntValue(bytes)},
		{Key: "elapsed", Value: slog.Float64Value(float64(elapsed.Nanoseconds()) / 1000000.0)}, // in milliseconds
	}

	if !DefaultOptions.Concise {
		// Include response header, as well for error status codes (>400) we include
		// the response body so we may inspect the log message sent back to the client.
		if status >= 400 {
			body, _ := extra.([]byte)
			responseLog = append(responseLog, slog.Attr{Key: "body", Value: slog.StringValue(string(body))})
		}
		if len(header) > 0 {
			responseLog = append(responseLog, slog.Group("header", headerLogField(header)...))
		}
	}
	l.Logger.With(slog.Group("httpResponse", responseLog...)).Log(statusLevel(status), msg)
}

func (l *RequestLoggerEntry) Panic(v interface{}, stack []byte) {
	stacktrace := "#"
	if DefaultOptions.JSON {
		stacktrace = string(stack)
	}
	l.Logger = *l.Logger.With(slog.Attr{Key: "stacktrace", Value: slog.StringValue(stacktrace)},
		slog.Attr{Key: "panic", Value: slog.StringValue(fmt.Sprintf("%+v", v))})
	// l.Logger = l.Logger.With().
	// 	Str("stacktrace", stacktrace).
	// 	Str("panic", fmt.Sprintf("%+v", v)).
	// 	Logger()

	l.msg = fmt.Sprintf("%+v", v)

	if !DefaultOptions.JSON {
		middleware.PrintPrettyStack(v)
	}
}

var coolDownMu sync.RWMutex
var coolDowns = map[string]time.Time{}

func rInCooldown(r *http.Request) bool {
	routePath := r.URL.EscapedPath()
	if routePath == "" {
		routePath = "/"
	}
	if !inArray(DefaultOptions.QuietDownRoutes, routePath) {
		return false
	}
	coolDownMu.RLock()
	coolDownTime, ok := coolDowns[routePath]
	coolDownMu.RUnlock()
	if ok {
		if time.Since(coolDownTime) < DefaultOptions.QuietDownPeriod {
			return true
		}
	}
	coolDownMu.Lock()
	defer coolDownMu.Unlock()
	coolDowns[routePath] = time.Now().Add(DefaultOptions.QuietDownPeriod)
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

func requestLogFields(r *http.Request, concise bool) slog.Attr {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	requestURL := fmt.Sprintf("%s://%s%s", scheme, r.Host, r.RequestURI)

	requestFields := []slog.Attr{
		{Key: "requestURL", Value: slog.StringValue(requestURL)},
		{Key: "requestMethod", Value: slog.StringValue(r.Method)},
		{Key: "requestPath", Value: slog.StringValue(r.URL.Path)},
		{Key: "remoteIP", Value: slog.StringValue(r.RemoteAddr)},
		{Key: "proto", Value: slog.StringValue(r.Proto)},
	}
	if reqID := middleware.GetReqID(r.Context()); reqID != "" {
		requestFields = append(requestFields, slog.Attr{Key: "requestID", Value: slog.StringValue(reqID)})
		// requestFields["requestID"] = reqID
	}

	if concise {
		return slog.Group("httpRequest", requestFields...)
	}

	// requestFields["scheme"] = scheme
	requestFields = append(requestFields, slog.Attr{Key: "scheme", Value: slog.StringValue(scheme)})
	if len(r.Header) > 0 {
		// requestFields["header"] = headerLogField(r.Header)
		requestFields = append(requestFields,
			slog.Attr{Key: "header",
				Value: slog.GroupValue(headerLogField(r.Header)...)})
	}

	return slog.Group("httpRequest", requestFields...)
}

func headerLogField(header http.Header) []slog.Attr {
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
			// headerField = fmt.Sprintf("[%s]", strings.Join(v, "], ["))
		}
		if k == "authorization" || k == "cookie" || k == "set-cookie" {
			headerField[len(headerField)] = slog.Attr{
				Key:   k,
				Value: slog.StringValue("***"),
			}
		}

		for _, skip := range DefaultOptions.SkipHeaders {
			if k == skip {
				headerField[len(headerField)] = slog.Attr{
					Key:   k,
					Value: slog.StringValue("***"),
				}
				break
			}
		}
	}
	return headerField
}

func statusLevel(status int) slog.Level {
	switch {
	case status <= 0:
		return slog.LevelWarn
	case status < 400: // for codes in 100s, 200s, 300s
		return slog.LevelInfo
	case status >= 400 && status < 500:
		return slog.LevelWarn
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

// Helper methods used by the application to get the request-scoped
// logger entry and set additional fields between handlers.
//
// This is a useful pattern to use to set state on the entry as it
// passes through the handler chain, which at any point can be logged
// with a call to .Print(), .Info(), etc.

func LogEntry(ctx context.Context) slog.Logger {
	entry, ok := ctx.Value(middleware.LogEntryCtxKey).(*RequestLoggerEntry)
	if !ok || entry == nil {
		handlerOpts := &slog.HandlerOptions{
			AddSource: true,
			// LevelError+1 will be higher than all levels
			// hence logs would be skipped
			Level: slog.LevelError + 1,
			// ReplaceAttr: func(attr slog.Attr) slog.Attr ,
		}
		return *slog.New(handlerOpts.NewTextHandler(os.Stdout))
	} else {
		return entry.Logger
	}
}

func LogEntrySetField(ctx context.Context, key, value string) {
	if entry, ok := ctx.Value(middleware.LogEntryCtxKey).(*RequestLoggerEntry); ok {
		entry.Logger = *entry.Logger.With(slog.Attr{Key: key, Value: slog.StringValue(value)})
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
		entry.Logger = *entry.Logger.With(attrs)
	}
}
