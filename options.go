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
	// You can override the level with a custom slog.Handler (e.g. on per-request basis).
	Level slog.Level

	// Schema defines the mapping of semantic log fields to their corresponding
	// field names in different logging systems and standards.
	//
	// This enables log output in different formats compatible with various logging
	// platforms and standards (ECS, OTEL, GCP, etc.) by providing the schema.
	//
	// httplog.SchemaECS (Elastic Common Schema)
	// httplog.SchemaOTEL (OpenTelemetry)
	// httplog.SchemaGCP (Google Cloud Platform)
	//
	// Append .Concise(true) to reduce log verbosity (e.g. for localhost development).
	Schema *Schema

	// RecoverPanics recovers from panics occurring in the underlying HTTP handlers
	// and middlewares and returns HTTP 500 unless response status was already set.
	//
	// NOTE: Panics are logged as errors automatically, regardless of this setting.
	RecoverPanics bool

	// Skip is an optional predicate function that determines whether to skip
	// recording logs for a given request.
	//
	// If nil, all requests are recorded.
	// If provided, requests where Skip returns true will not be recorded.
	Skip func(req *http.Request, respStatus int) bool

	// LogRequestHeaders is a list of headers to be logged as attributes.
	// If not provided, the default is ["Content-Type", "Origin"].
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
	// with LogRequestBody or LogResponseBody options.
	//
	// If not provided, the default is ["application/json", "application/xml", "text/plain", "text/csv", "application/x-www-form-urlencoded", ""].
	LogBodyContentTypes []string

	// LogBodyMaxLen defines the maximum length of the body to be logged.
	//
	// If not provided, the default is 1024 bytes. Set to -1 to log the full body.
	LogBodyMaxLen int

	// LogExtraAttrs is an optional function that lets you add extra attributes to the
	// request log.
	//
	// Example:
	//
	// // Log all requests with invalid payload as curl command.
	// func(req *http.Request, reqBody string, respStatus int) []slog.Attr {
	//     if respStatus == 400 || respStatus == 422 {
	// 	       req.Header.Del("Authorization")
	//         return []slog.Attr{slog.String("curl", httplog.CURL(req, reqBody))}
	// 	   }
	// 	   return nil
	// }
	//
	// WARNING: Be careful not to leak any sensitive information in the logs.
	LogExtraAttrs func(req *http.Request, reqBody string, respStatus int) []slog.Attr
}

var defaultOptions = Options{
	Level:               slog.LevelInfo,
	Schema:              SchemaECS,
	RecoverPanics:       true,
	LogRequestHeaders:   []string{"Content-Type", "Origin"},
	LogResponseHeaders:  []string{"Content-Type"},
	LogBodyContentTypes: []string{"application/json", "application/xml", "text/plain", "text/csv", "application/x-www-form-urlencoded", ""},
	LogBodyMaxLen:       1024,
}
