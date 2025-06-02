package httplog

import (
	"bytes"
	"context"
	"fmt"
	"io"
	loga "log"
	"log/slog"
	"net/http"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

func RequestLogger(logger *slog.Logger, o *Options) func(http.Handler) http.Handler {
	if o == nil {
		o = &defaultOptions
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), ctxKeyLogAttrs{}, &[]slog.Attr{})

			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			var resBody bytes.Buffer
			if o.LogResponseBody {
				ww.Tee(&resBody)
			}

			var reqBody bytes.Buffer
			if o.LogRequestBody {
				bodyBytes, err := io.ReadAll(r.Body)
				if err == nil {
					reqBody.Write(bodyBytes) // buffer for logging
				}

				r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes)) // restore body
			}

			start := time.Now()

			defer func() {
				logAttrs := make([]slog.Attr, 0)

				if rec := recover(); rec != nil {
					var p []uintptr
					if rec != http.ErrAbortHandler {
						pc := make([]uintptr, 10)   // Capture up to 10 stack frames.
						n := runtime.Callers(3, pc) // Skip 3 frames (this middleware + runtime/panic.go).

						p = pc[:n]
					}

					// Return HTTP 500 if recover is enabled and no response status was set.
					if o.RecoverPanics && ww.Status() == 0 && r.Header.Get("Connection") != "Upgrade" {
						ww.WriteHeader(http.StatusInternalServerError)
					}

					if rec == http.ErrAbortHandler || !o.RecoverPanics {
						// Always re-panic http.ErrAbortHandler. Re-panic everything unless recover is enabled.
						defer panic(rec)
					}

					// Process panic stack frames to print detailed information.
					frames := runtime.CallersFrames(p)
					var stackValues []string
					for frame, more := frames.Next(); more; frame, more = frames.Next() {
						if !strings.Contains(frame.File, "runtime/panic.go") {
							stackValues = append(stackValues, fmt.Sprintf("%s:%d", frame.File, frame.Line))
						}
					}

					logAttrs = append(logAttrs,
						slog.Any("panic", loga.Panic),
						slog.Any("panicStack", stackValues),
					)
				}

				duration := time.Since(start)
				statusCode := ww.Status()

				var lvl slog.Level
				switch {
				case r.Method == "OPTIONS":
					lvl = slog.LevelDebug
				case statusCode >= 500:
					lvl = slog.LevelError
				case statusCode == 429:
					lvl = slog.LevelInfo
				case statusCode >= 400:
					lvl = slog.LevelWarn
				default:
					lvl = slog.LevelInfo
				}

				// Stop processign, when the message wouldn't be logged, or when the the level os lover, the level in options
				if !logger.Enabled(ctx, lvl) || lvl < o.Level {
					return
				}

				if o.LogRequestBody {
					// Make sure to read full request body if the underlying handler didn't do so.
					_, _ = io.Copy(io.Discard, r.Body)
				}

				// Request attributes
				reqAttrs := []slog.Attr{
					slog.Any("headers", slog.GroupValue(getHeaderAttrs(r.Header, o.LogRequestHeaders)...)),
				}
				if !o.Concise {
					reqAttrs = append(reqAttrs,
						slog.String("url", fullURL(r)),
						slog.String("method", r.Method),
						slog.String("path", r.URL.Path),
						slog.String("remoteIp", r.RemoteAddr),
						slog.String("proto", r.Proto),
					)
				}

				if o.LogRequestCURL {
					reqAttrs = append(reqAttrs, slog.String("curl", curl(r, reqBody.String())))
				}

				if o.LogRequestBody {
					reqAttrs = append(reqAttrs, slog.String("body", reqBody.String()))
				}

				// Response attributes
				resAttrs := []slog.Attr{
					slog.Any("headers", slog.GroupValue(getHeaderAttrs(ww.Header(), o.LogResponseHeaders)...)),
				}
				if !o.Concise {
					resAttrs = append(resAttrs,
						slog.Int("status", ww.Status()),
						slog.Float64("duration", float64(duration.Milliseconds())),
						slog.Int("bytes", ww.BytesWritten()),
					)
				}

				if o.LogResponseBody {
					ww.Tee(&resBody)

					if slices.Contains(o.LogResponseBodyContentType, ww.Header().Get("content-type")) {
						resAttrs = append(resAttrs, slog.String("body", resBody.String()))
					}
				}

				if !o.Concise || ww.Status() >= 400 || o.Level < slog.LevelInfo {
					logAttrs = append(logAttrs, slog.Any("request", slog.GroupValue(reqAttrs...)))
					logAttrs = append(logAttrs, slog.Any("response", slog.GroupValue(resAttrs...)))
					logAttrs = append(logAttrs, getAttrs(ctx)...)
				}

				msg := fmt.Sprintf("%s %s => HTTP %v (%v)", r.Method, r.URL, ww.Status(), duration)
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
