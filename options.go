package httplog

import (
	"log/slog"
	"net/http"
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

	// LogRequestHeaders is a list of headers to be logged as attributes.
	// If not provided, the default is ["Content-Type", "User-Agent", "Referer", "Origin"].
	//
	// WARNING: Do not leak any request headers with sensitive information.
	LogRequestHeaders []string

	// LogRequestBody is an optional predicate function that controls logging of request body.
	//
	// If the function returns true, the request body will be logged.
	// If false, no request body will be logged.
	//
	// WARNING: Do not leak any request bodies with sensitive information.
	LogRequestBody func(req *http.Request) bool

	// LogResponseHeaders controls a list of headers to be logged as attributes.
	//
	// If not provided, there are no default headers.
	LogResponseHeaders []string

	// LogRequestBody is an optional predicate function that controls logging of request body.
	//
	// If the function returns true, the request body will be logged.
	// If false, no request body will be logged.
	//
	// WARNING: Do not leak any response bodies with sensitive information.
	LogResponseBody func(req *http.Request) bool

	// LogBodyContentTypes defines a list of body Content-Types that are safe to be logged
	// with LogRequestCURL, LogRequestBody or LogResponseBody options.
	//
	// If not provided, the default is ["application/json", "application/xml", "text/plain", "text/csv"].
	LogBodyContentTypes []string

	// LogBodyMaxLen defines the maximum length of the body to be logged.
	//
	// If not provided, the default is 1024 bytes. Set to -1 to log the full body.
	LogBodyMaxLen int

	// LogRequestAsCURL is an optional predicate function that controls logging of requests
	// as curl commands. Useful for localhost debugging.
	//
	// If the predicate returns true, the request will be logged in "curl" attribute
	// including all headers and body. If false, no curl logging will occur.
	//
	// WARNING: This will likely leak sensitive information in the logs. Do not use in production.
	LogRequestAsCURL func(req *http.Request) bool
}

var defaultOptions = Options{
	Level:               slog.LevelInfo,
	RecoverPanics:       true,
	LogRequestHeaders:   []string{"Content-Type", "User-Agent", "Referer", "Origin"},
	LogResponseHeaders:  []string{"Content-Type"},
	LogBodyContentTypes: []string{"application/json", "application/xml", "text/plain", "text/csv"},
	LogBodyMaxLen:       1024,
}
