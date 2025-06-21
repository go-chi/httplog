package httplog

import (
	"log/slog"
	"strings"
	"time"
)

// Schema defines the mapping of semantic log fields to their corresponding
// field names in different logging systems and standards.
//
// This enables log output in different formats compatible with various logging
// platforms and standards (ECS, OTEL, GCP, etc.) by providing the schema.
type Schema struct {
	// Base attributes for core logging information.
	Timestamp       string // Timestamp of the log entry
	Level           string // Log level (e.g. INFO, WARNING, ERROR)
	Message         string // Primary log message
	ErrorMessage    string // Error message when an error occurs
	ErrorType       string // Low-cardinality error type (e.g. "ClientAborted", "ValidationError")
	ErrorStackTrace string // Stack trace for panic or error

	// Source code location attributes for tracking origin of log statements.
	SourceFile     string // Source file name where the log originated
	SourceLine     string // Line number in the source file
	SourceFunction string // Function name where the log originated

	// Request attributes for the incoming HTTP request.
	// NOTE: RequestQuery is intentionally not supported as it would likely leak sensitive data.
	RequestURL         string // Full request URL
	RequestMethod      string // HTTP method (e.g. GET, POST)
	RequestPath        string // URL path component
	RequestRemoteIP    string // Client IP address
	RequestHost        string // Host header value
	RequestScheme      string // URL scheme (http, https)
	RequestProto       string // HTTP protocol version (e.g. HTTP/1.1, HTTP/2)
	RequestHeaders     string // Selected request headers
	RequestBody        string // Request body content, if logged.
	RequestBytes       string // Size of request body in bytes
	RequestBytesUnread string // Unread bytes in request body
	RequestUserAgent   string // User-Agent header value
	RequestReferer     string // Referer header value

	// Response attributes for the HTTP response.
	ResponseHeaders  string // Selected response headers
	ResponseBody     string // Response body content, if logged.
	ResponseStatus   string // HTTP status code
	ResponseDuration string // Request processing duration
	ResponseBytes    string // Size of response body in bytes

	// GroupDelimiter is an optional delimiter for nested objects in some formats.
	// For example, GCP uses nested JSON objects like "httpRequest": {}.
	GroupDelimiter string
}

var (
	// SchemaECS represents the Elastic Common Schema (ECS) version 9.0.0.
	// This schema is widely used with Elasticsearch and the Elastic Stack.
	//
	// Reference: https://www.elastic.co/guide/en/ecs/current/ecs-http.html
	SchemaECS = &Schema{
		Timestamp:          "@timestamp",
		Level:              "log.level",
		Message:            "message",
		ErrorMessage:       "error.message",
		ErrorType:          "error.type",
		ErrorStackTrace:    "error.stack_trace",
		SourceFile:         "log.origin.file.name",
		SourceLine:         "log.origin.file.line",
		SourceFunction:     "log.origin.function",
		RequestURL:         "url.full",
		RequestMethod:      "http.request.method",
		RequestPath:        "url.path",
		RequestRemoteIP:    "client.ip",
		RequestHost:        "url.domain",
		RequestScheme:      "url.scheme",
		RequestProto:       "http.version",
		RequestHeaders:     "http.request.headers",
		RequestBody:        "http.request.body.content",
		RequestBytes:       "http.request.body.bytes",
		RequestBytesUnread: "http.request.body.unread.bytes",
		RequestUserAgent:   "user_agent.original",
		RequestReferer:     "http.request.referrer",
		ResponseHeaders:    "http.response.headers",
		ResponseBody:       "http.response.body.content",
		ResponseStatus:     "http.response.status_code",
		ResponseDuration:   "event.duration",
		ResponseBytes:      "http.response.body.bytes",
	}

	// SchemaOTEL represents OpenTelemetry (OTEL) semantic conventions version 1.34.0.
	// This schema follows OpenTelemetry standards for observability data.
	//
	// Reference: https://opentelemetry.io/docs/specs/semconv/http/http-metrics
	SchemaOTEL = &Schema{
		Timestamp:          "timestamp",
		Level:              "severity_text",
		Message:            "body",
		ErrorMessage:       "error.message",
		ErrorType:          "error.type",
		ErrorStackTrace:    "exception.stacktrace",
		SourceFile:         "code.filepath",
		SourceLine:         "code.lineno",
		SourceFunction:     "code.function",
		RequestURL:         "url.full",
		RequestMethod:      "http.request.method",
		RequestPath:        "url.path",
		RequestRemoteIP:    "client.address",
		RequestHost:        "server.address",
		RequestScheme:      "url.scheme",
		RequestProto:       "network.protocol.version",
		RequestHeaders:     "http.request.header",
		RequestBody:        "http.request.body.content",
		RequestBytes:       "http.request.body.size",
		RequestBytesUnread: "http.request.body.unread.size",
		RequestUserAgent:   "user_agent.original",
		RequestReferer:     "http.request.header.referer",
		ResponseHeaders:    "http.response.header",
		ResponseBody:       "http.response.body.content",
		ResponseStatus:     "http.response.status_code",
		ResponseDuration:   "http.server.request.duration",
		ResponseBytes:      "http.response.body.size",
	}

	// SchemaGCP represents Google Cloud Platform's structured logging format.
	// This schema is optimized for Google Cloud Logging service.
	//
	// References:
	//   - https://cloud.google.com/logging/docs/structured-logging
	//   - https://cloud.google.com/logging/docs/reference/v2/rest/v2/LogEntry#HttpRequest
	SchemaGCP = &Schema{
		Timestamp:          "timestamp",
		Level:              "severity",
		Message:            "message",
		ErrorMessage:       "error",
		ErrorType:          "error_type",
		ErrorStackTrace:    "stack_trace",
		SourceFile:         "logging.googleapis.com/sourceLocation:file",
		SourceLine:         "logging.googleapis.com/sourceLocation:line",
		SourceFunction:     "logging.googleapis.com/sourceLocation:function",
		RequestURL:         "httpRequest:requestUrl",
		RequestMethod:      "httpRequest:requestMethod",
		RequestPath:        "httpRequest:requestPath",
		RequestRemoteIP:    "httpRequest:remoteIp",
		RequestHost:        "httpRequest:host",
		RequestScheme:      "httpRequest:scheme",
		RequestProto:       "httpRequest:protocol",
		RequestHeaders:     "httpRequest:requestHeaders",
		RequestBody:        "httpRequest:requestBody",
		RequestBytes:       "httpRequest:requestSize",
		RequestBytesUnread: "httpRequest:requestUnreadSize",
		RequestUserAgent:   "httpRequest:userAgent",
		RequestReferer:     "httpRequest:referer",
		ResponseHeaders:    "httpRequest:responseHeaders",
		ResponseBody:       "httpRequest:responseBody",
		ResponseStatus:     "httpRequest:status",
		ResponseDuration:   "httpRequest:latency",
		ResponseBytes:      "httpRequest:responseSize",
		GroupDelimiter:     ":",
	}
)

// ReplaceAttr returns transforms standard slog attribute names to the schema format.
func (s *Schema) ReplaceAttr(groups []string, a slog.Attr) slog.Attr {
	if len(groups) > 0 {
		return a
	}

	switch a.Key {
	case slog.TimeKey:
		if s.Timestamp == "" {
			return a
		}
		return slog.String(s.Timestamp, a.Value.Time().Format(time.RFC3339))
	case slog.LevelKey:
		if s.Level == "" {
			return a
		}
		return slog.String(s.Level, a.Value.String())
	case slog.MessageKey:
		if s.Message == "" {
			return a
		}
		return slog.String(s.Message, a.Value.String())
	case slog.SourceKey:
		source, ok := a.Value.Any().(*slog.Source)
		if !ok {
			return a
		}

		if s.SourceFile == "" {
			// Ignore httplog.RequestLogger middleware source.
			if strings.Contains(source.File, "/go-chi/httplog/") {
				return slog.Attr{}
			}
			return a
		}

		if s.GroupDelimiter == "" {
			return slog.Group("", slog.String(s.SourceFile, source.File), slog.Int(s.SourceLine, source.Line), slog.String(s.SourceFunction, source.Function))
		}

		grp, file, _ := strings.Cut(s.SourceFile, s.GroupDelimiter)
		_, line, _ := strings.Cut(s.SourceLine, s.GroupDelimiter)
		_, fn, _ := strings.Cut(s.SourceFunction, s.GroupDelimiter)
		return slog.Group(grp, slog.String(file, source.File), slog.Int(line, source.Line), slog.String(fn, source.Function))

	case ErrorKey:
		if s.GroupDelimiter == "" {
			return slog.Attr{Key: s.ErrorMessage, Value: a.Value}
		}

		grp, errMsg, found := strings.Cut(s.ErrorMessage, s.GroupDelimiter)
		if !found {
			return slog.Attr{Key: s.ErrorMessage, Value: a.Value}
		}

		return slog.Group(grp, slog.Attr{Key: errMsg, Value: a.Value})
	}

	return a
}

// Concise returns a simplified schema with essential fields only.
// If concise is true, it reduces log verbosity.
//
// This is useful for localhost development to reduce log verbosity.
func (s *Schema) Concise(concise bool) *Schema {
	if !concise {
		return s
	}

	return &Schema{
		ErrorMessage:       s.ErrorMessage,
		ErrorStackTrace:    s.ErrorStackTrace,
		RequestHeaders:     s.RequestHeaders,
		RequestBody:        s.RequestBody,
		RequestBytesUnread: s.RequestBytesUnread,
		ResponseHeaders:    s.ResponseHeaders,
		ResponseBody:       s.ResponseBody,
		GroupDelimiter:     s.GroupDelimiter,
	}
}
