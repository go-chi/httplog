package httplog

import (
	"log/slog"
)

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
	LogRequestHeaders:          []string{"Content-Type", "User-Agent", "Referer", "Origin"},
	LogResponseHeaders:         []string{"Content-Type"},
	LogResponseBodyContentType: []string{"application/json", "text/plain"},
}
