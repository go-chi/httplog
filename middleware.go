package httplog

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

func RequestLogger(logger *slog.Logger, o *Options) func(http.Handler) http.Handler {
	if o == nil {
		o = &defaultOptions
	}
	if len(o.LogBodyContentTypes) == 0 {
		o.LogBodyContentTypes = defaultOptions.LogBodyContentTypes
	}
	if o.LogBodyMaxLen == 0 {
		o.LogBodyMaxLen = defaultOptions.LogBodyMaxLen
	}
	f := o.Format

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), ctxKeyLogAttrs{}, &[]slog.Attr{})

			logReqBody := o.LogRequestBody != nil && o.LogRequestBody(r)
			logRespBody := o.LogResponseBody != nil && o.LogResponseBody(r)

			var reqBody bytes.Buffer
			if logReqBody || o.LogExtraAttrs != nil {
				r.Body = io.NopCloser(io.TeeReader(r.Body, &reqBody))
			}

			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			var respBody bytes.Buffer
			if o.LogResponseBody != nil && o.LogResponseBody(r) {
				ww.Tee(&respBody)
			}

			start := time.Now()

			defer func() {
				var logAttrs []slog.Attr

				if rec := recover(); rec != nil {
					// Return HTTP 500 if recover is enabled and no response status was set.
					if o.RecoverPanics && ww.Status() == 0 && r.Header.Get("Connection") != "Upgrade" {
						ww.WriteHeader(http.StatusInternalServerError)
					}

					if rec == http.ErrAbortHandler || !o.RecoverPanics {
						// Re-panic http.ErrAbortHandler unconditionally, and re-panic other errors if panic recovery is disabled.
						defer panic(rec)
					}

					logAttrs = appendAttrs(logAttrs, slog.String(f.Error, fmt.Sprintf("panic: %v", rec)))

					if rec != http.ErrAbortHandler {
						pc := make([]uintptr, 10)   // Capture up to 10 stack frames.
						n := runtime.Callers(3, pc) // Skip 3 frames (this middleware + runtime/panic.go).
						pc = pc[:n]

						// Process panic stack frames to print detailed information.
						frames := runtime.CallersFrames(pc)
						var stackValues []string
						for frame, more := frames.Next(); more; frame, more = frames.Next() {
							if !strings.Contains(frame.File, "runtime/panic.go") {
								stackValues = append(stackValues, fmt.Sprintf("%s:%d", frame.File, frame.Line))
							}
						}
						logAttrs = appendAttrs(logAttrs, slog.Any(f.ErrorStackTrace, stackValues))
					}
				}

				duration := time.Since(start)
				statusCode := ww.Status()

				var lvl slog.Level
				switch {
				case statusCode >= 500:
					lvl = slog.LevelError
				case statusCode == 429:
					lvl = slog.LevelInfo
				case statusCode >= 400:
					lvl = slog.LevelWarn
				case r.Method == "OPTIONS":
					lvl = slog.LevelDebug
				default:
					lvl = slog.LevelInfo
				}

				// Skip logging if the message level is below the logger's level or the minimum level specified in options
				if !logger.Enabled(ctx, lvl) || lvl < o.Level {
					return
				}

				logAttrs = appendAttrs(logAttrs,
					slog.String(f.Timestamp, start.Format(time.RFC3339)),
					slog.String(f.Level, lvl.String()),
					slog.String(f.Message, fmt.Sprintf("%s %s => HTTP %v (%v)", r.Method, r.URL, statusCode, duration)),
					slog.String(f.RequestURL, requestURL(r)),
					slog.String(f.RequestMethod, r.Method),
					slog.String(f.RequestPath, r.URL.Path),
					slog.String(f.RequestRemoteIP, r.RemoteAddr),
					slog.String(f.RequestHost, r.Host),
					slog.String(f.RequestScheme, scheme(r)),
					slog.String(f.RequestProto, r.Proto),
					slog.Any(f.RequestHeaders, slog.GroupValue(getHeaderAttrs(r.Header, o.LogRequestHeaders)...)),
					slog.Int64(f.RequestBytes, r.ContentLength),
					slog.String(f.RequestUserAgent, r.UserAgent()),
					slog.String(f.RequestReferer, r.Referer()),
					slog.Any(f.ResponseHeaders, slog.GroupValue(getHeaderAttrs(ww.Header(), o.LogResponseHeaders)...)),
					slog.Int(f.ResponseStatus, statusCode),
					slog.Float64(f.ResponseDuration, float64(duration.Milliseconds())),
					slog.Int(f.ResponseBytes, ww.BytesWritten()),
				)

				if logReqBody || o.LogExtraAttrs != nil {
					// Ensure the request body is fully read if the underlying HTTP handler didn't do so.
					n, _ := io.Copy(io.Discard, r.Body)
					if n > 0 {
						logAttrs = appendAttrs(logAttrs, slog.Any(f.RequestBytesUnread, n))
					}
				}
				if logReqBody {
					logAttrs = appendAttrs(logAttrs, slog.String(f.RequestBody, logBody(&reqBody, r.Header, o)))
				}
				if logRespBody {
					logAttrs = appendAttrs(logAttrs, slog.String(f.ResponseBody, logBody(&respBody, ww.Header(), o)))
				}
				if o.LogExtraAttrs != nil {
					logAttrs = appendAttrs(logAttrs, o.LogExtraAttrs(r, reqBody.String(), statusCode)...)
				}
				logAttrs = appendAttrs(logAttrs, getAttrs(ctx)...)

				// Group attributes into nested objects, e.g. for GCP structured logs.
				if f.GroupDelimiter != "" {
					logAttrs = groupAttrs(logAttrs, f.GroupDelimiter)
				}

				msg := fmt.Sprintf("%s %s => HTTP %v (%v)", r.Method, r.URL, statusCode, duration)
				logger.LogAttrs(ctx, lvl, msg, logAttrs...)
			}()

			next.ServeHTTP(ww, r.WithContext(ctx))
		})
	}
}

func appendAttrs(attrs []slog.Attr, newAttrs ...slog.Attr) []slog.Attr {
	for _, attr := range newAttrs {
		if attr.Key != "" {
			attrs = append(attrs, attr)
		}
	}
	return attrs
}

func groupAttrs(attrs []slog.Attr, delimiter string) []slog.Attr {
	var result []slog.Attr
	var nested = map[string][]slog.Attr{}

	for _, attr := range attrs {
		prefix, key, found := strings.Cut(attr.Key, delimiter)
		if !found {
			result = append(result, attr)
			continue
		}
		nested[prefix] = append(nested[prefix], slog.Attr{Key: key, Value: attr.Value})
	}

	for prefix, attrs := range nested {
		result = append(result, slog.Any(prefix, slog.GroupValue(attrs...)))
	}

	return result
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

func logBody(body *bytes.Buffer, header http.Header, o *Options) string {
	if body.Len() == 0 {
		return ""
	}
	contentType := header.Get("Content-Type")
	for _, whitelisted := range o.LogBodyContentTypes {
		if strings.HasPrefix(contentType, whitelisted) {
			if o.LogBodyMaxLen <= 0 || o.LogBodyMaxLen >= body.Len() {
				return body.String()
			}
			return body.String()[:o.LogBodyMaxLen] + "... [trimmed]"
		}
	}
	return fmt.Sprintf("[body redacted for Content-Type: %s]", contentType)
}
