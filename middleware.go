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

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), ctxKeyLogAttrs{}, &[]slog.Attr{})

			var reqBody bytes.Buffer
			if o.LogRequestBody {
				r.Body = io.NopCloser(io.TeeReader(r.Body, &reqBody))
			}

			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			var respBody bytes.Buffer
			if o.LogResponseBody {
				ww.Tee(&respBody)
			}

			start := time.Now()

			defer func() {
				logAttrs := make([]slog.Attr, 0)

				if rec := recover(); rec != nil {
					// Return HTTP 500 if recover is enabled and no response status was set.
					if o.RecoverPanics && ww.Status() == 0 && r.Header.Get("Connection") != "Upgrade" {
						ww.WriteHeader(http.StatusInternalServerError)
					}

					if rec == http.ErrAbortHandler || !o.RecoverPanics {
						// Re-panic http.ErrAbortHandler unconditionally, and re-panic other errors if panic recovery is disabled.
						defer panic(rec)
					}

					logAttrs = append(logAttrs, slog.Any("panic", rec))

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
						logAttrs = append(logAttrs, slog.Any("panicStack", stackValues))
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

				// Request attributes
				reqAttrs := []slog.Attr{
					slog.Any("headers", slog.GroupValue(getHeaderAttrs(r.Header, o.LogRequestHeaders)...)),
				}
				if o.LogRequestBody {
					// Ensure the request body is fully read if the underlying HTTP handler didn't do so.
					n, _ := io.Copy(io.Discard, r.Body)
					if n > 0 {
						reqAttrs = append(reqAttrs, slog.Any("request.bytes.unread", n))
					}
				}
				if o.LogRequestCURL {
					reqAttrs = append(reqAttrs, slog.String("curl", curl(r, reqBody.String())))
				}
				if o.LogRequestBody {
					reqAttrs = append(reqAttrs, slog.String("body", logBody(&reqBody, r.Header, o)))
				}
				if !o.Concise {
					reqAttrs = append(reqAttrs,
						slog.String("url", requestURL(r)),
						slog.String("method", r.Method),
						slog.String("path", r.URL.Path),
						slog.String("remoteIp", r.RemoteAddr),
						slog.String("proto", r.Proto),
					)
				}

				// Response attributes
				respAttrs := []slog.Attr{
					slog.Any("headers", slog.GroupValue(getHeaderAttrs(ww.Header(), o.LogResponseHeaders)...)),
				}
				if o.LogResponseBody {
					respAttrs = append(respAttrs, slog.String("body", logBody(&respBody, ww.Header(), o)))
				}

				if !o.Concise {
					respAttrs = append(respAttrs,
						slog.Int("status", statusCode),
						slog.Float64("duration", float64(duration.Milliseconds())),
						slog.Int("bytes", ww.BytesWritten()),
					)
				}

				logAttrs = append(logAttrs, slog.Any("request", slog.GroupValue(reqAttrs...)))
				logAttrs = append(logAttrs, slog.Any("response", slog.GroupValue(respAttrs...)))
				logAttrs = append(logAttrs, getAttrs(ctx)...)

				msg := fmt.Sprintf("%s %s => HTTP %v (%v)", r.Method, r.URL, statusCode, duration)
				logger.LogAttrs(ctx, lvl, msg, logAttrs...)
			}()

			next.ServeHTTP(ww, r.WithContext(ctx))
		})
	}
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
	for _, contentType := range o.LogBodyContentTypes {
		if strings.HasPrefix(header.Get("Content-Type"), contentType) {
			if o.LogBodyMaxLen <= 0 || o.LogBodyMaxLen >= body.Len() {
				return body.String()
			}
			return body.String()[:o.LogBodyMaxLen] + "... [trimmed]"
		}
	}
	return "[redacted]"
}
