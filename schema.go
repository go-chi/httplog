package httplog

import (
	"log/slog"
	"strings"
	"time"
)

const (
	ECSResponseDuration  = "event.duration"
	OTELResponseDuration = "http.server.request.duration"
	GCPResponseDuration  = "httpRequest:latency"

	GCPLevel = "severity"
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
	// Reference: https://www.elastic.co/docs/reference/ecs/ecs-field-reference
	SchemaECS = &Schema{
		Timestamp:          "@timestamp",                     // https://www.elastic.co/docs/reference/ecs/ecs-base#field-timestamp
		Level:              "log.level",                      // https://www.elastic.co/docs/reference/ecs/ecs-log#field-log-level
		Message:            "message",                        // https://www.elastic.co/docs/reference/ecs/ecs-base#field-message
		ErrorMessage:       "error.message",                  // https://www.elastic.co/docs/reference/ecs/ecs-error#field-error-message
		ErrorType:          "error.type",                     // https://www.elastic.co/docs/reference/ecs/ecs-error#field-error-type
		ErrorStackTrace:    "error.stack_trace",              // https://www.elastic.co/docs/reference/ecs/ecs-error#field-error-stack-trace
		SourceFile:         "log.origin.file.name",           // https://www.elastic.co/docs/reference/ecs/ecs-log#field-log-origin-file-name
		SourceLine:         "log.origin.file.line",           // https://www.elastic.co/docs/reference/ecs/ecs-log#field-log-origin-file-line
		SourceFunction:     "log.origin.function",            // https://www.elastic.co/docs/reference/ecs/ecs-log#field-log-origin-function
		RequestURL:         "url.full",                       // https://www.elastic.co/docs/reference/ecs/ecs-url#field-url-full
		RequestMethod:      "http.request.method",            // https://www.elastic.co/docs/reference/ecs/ecs-http#field-http-request-method
		RequestPath:        "url.path",                       // https://www.elastic.co/docs/reference/ecs/ecs-url#field-url-path
		RequestRemoteIP:    "client.ip",                      // https://www.elastic.co/docs/reference/ecs/ecs-client#field-client-ip
		RequestHost:        "url.domain",                     // https://www.elastic.co/docs/reference/ecs/ecs-url#field-url-domain
		RequestScheme:      "url.scheme",                     // https://www.elastic.co/docs/reference/ecs/ecs-url#field-url-scheme
		RequestProto:       "http.version",                   // https://www.elastic.co/docs/reference/ecs/ecs-http#field-http-version
		RequestHeaders:     "http.request.headers",           // Custom field; ECS 9.0.0 defines no header fields.
		RequestBody:        "http.request.body.content",      // https://www.elastic.co/docs/reference/ecs/ecs-http#field-http-request-body-content
		RequestBytes:       "http.request.body.bytes",        // https://www.elastic.co/docs/reference/ecs/ecs-http#field-http-request-body-bytes
		RequestBytesUnread: "http.request.body.unread.bytes", // Custom field; not part of ECS 9.0.0.
		RequestUserAgent:   "user_agent.original",            // https://www.elastic.co/docs/reference/ecs/ecs-user_agent#field-user-agent-original
		RequestReferer:     "http.request.referrer",          // https://www.elastic.co/docs/reference/ecs/ecs-http#field-http-request-referrer
		ResponseHeaders:    "http.response.headers",          // Custom field; ECS 9.0.0 defines no header fields.
		ResponseBody:       "http.response.body.content",     // https://www.elastic.co/docs/reference/ecs/ecs-http#field-http-response-body-content
		ResponseStatus:     "http.response.status_code",      // https://www.elastic.co/docs/reference/ecs/ecs-http#field-http-response-status-code
		ResponseDuration:   ECSResponseDuration,              // https://www.elastic.co/docs/reference/ecs/ecs-event#field-event-duration
		ResponseBytes:      "http.response.body.bytes",       // https://www.elastic.co/docs/reference/ecs/ecs-http#field-http-response-body-bytes
	}

	// SchemaOTEL represents OpenTelemetry (OTEL) semantic conventions version 1.34.0.
	// This schema follows OpenTelemetry standards for observability data.
	//
	// References:
	//   - https://github.com/open-telemetry/semantic-conventions/tree/v1.34.0/docs/registry/attributes
	//   - https://opentelemetry.io/docs/specs/otel/logs/data-model/ (timestamp, severity_text, body)
	SchemaOTEL = &Schema{
		Timestamp:          "timestamp",                     // Log record field: https://opentelemetry.io/docs/specs/otel/logs/data-model/#field-timestamp
		Level:              "severity_text",                 // Log record field: https://opentelemetry.io/docs/specs/otel/logs/data-model/#field-severitytext
		Message:            "body",                          // Log record field: https://opentelemetry.io/docs/specs/otel/logs/data-model/#field-body
		ErrorMessage:       "error.message",                 // https://github.com/open-telemetry/semantic-conventions/blob/v1.34.0/docs/registry/attributes/error.md
		ErrorType:          "error.type",                    // https://github.com/open-telemetry/semantic-conventions/blob/v1.34.0/docs/registry/attributes/error.md
		ErrorStackTrace:    "exception.stacktrace",          // https://github.com/open-telemetry/semantic-conventions/blob/v1.34.0/docs/registry/attributes/exception.md
		SourceFile:         "code.file.path",                // https://github.com/open-telemetry/semantic-conventions/blob/v1.34.0/docs/registry/attributes/code.md
		SourceLine:         "code.line.number",              // https://github.com/open-telemetry/semantic-conventions/blob/v1.34.0/docs/registry/attributes/code.md
		SourceFunction:     "code.function.name",            // https://github.com/open-telemetry/semantic-conventions/blob/v1.34.0/docs/registry/attributes/code.md
		RequestURL:         "url.full",                      // https://github.com/open-telemetry/semantic-conventions/blob/v1.34.0/docs/registry/attributes/url.md
		RequestMethod:      "http.request.method",           // https://github.com/open-telemetry/semantic-conventions/blob/v1.34.0/docs/registry/attributes/http.md
		RequestPath:        "url.path",                      // https://github.com/open-telemetry/semantic-conventions/blob/v1.34.0/docs/registry/attributes/url.md
		RequestRemoteIP:    "client.address",                // https://github.com/open-telemetry/semantic-conventions/blob/v1.34.0/docs/registry/attributes/client.md
		RequestHost:        "server.address",                // https://github.com/open-telemetry/semantic-conventions/blob/v1.34.0/docs/registry/attributes/server.md
		RequestScheme:      "url.scheme",                    // https://github.com/open-telemetry/semantic-conventions/blob/v1.34.0/docs/registry/attributes/url.md
		RequestProto:       "network.protocol.version",      // https://github.com/open-telemetry/semantic-conventions/blob/v1.34.0/docs/registry/attributes/network.md
		RequestHeaders:     "http.request.header",           // https://github.com/open-telemetry/semantic-conventions/blob/v1.34.0/docs/registry/attributes/http.md
		RequestBody:        "http.request.body.content",     // Custom field (ECS name); OTEL semconv defines no body content attribute.
		RequestBytes:       "http.request.body.size",        // https://github.com/open-telemetry/semantic-conventions/blob/v1.34.0/docs/registry/attributes/http.md
		RequestBytesUnread: "http.request.body.unread.size", // Custom field; not part of OTEL semconv.
		RequestUserAgent:   "user_agent.original",           // https://github.com/open-telemetry/semantic-conventions/blob/v1.34.0/docs/registry/attributes/user-agent.md
		RequestReferer:     "http.request.header.referer",   // Templated header attribute: https://github.com/open-telemetry/semantic-conventions/blob/v1.34.0/docs/registry/attributes/http.md
		ResponseHeaders:    "http.response.header",          // https://github.com/open-telemetry/semantic-conventions/blob/v1.34.0/docs/registry/attributes/http.md
		ResponseBody:       "http.response.body.content",    // Custom field (ECS name); OTEL semconv defines no body content attribute.
		ResponseStatus:     "http.response.status_code",     // https://github.com/open-telemetry/semantic-conventions/blob/v1.34.0/docs/registry/attributes/http.md
		ResponseDuration:   OTELResponseDuration,            // Metric name reused as a log attribute: https://github.com/open-telemetry/semantic-conventions/blob/v1.34.0/docs/http/http-metrics.md#metric-httpserverrequestduration
		ResponseBytes:      "http.response.body.size",       // https://github.com/open-telemetry/semantic-conventions/blob/v1.34.0/docs/registry/attributes/http.md
	}

	// SchemaGCP represents Google Cloud Platform's structured logging format.
	// This schema is optimized for Google Cloud Logging service.
	//
	// References:
	//   - https://cloud.google.com/logging/docs/structured-logging#special-payload-fields
	//   - https://cloud.google.com/logging/docs/reference/v2/rest/v2/LogEntry#HttpRequest
	//   - https://cloud.google.com/error-reporting/docs/formatting-error-messages
	//
	// Fields not defined by LogEntry#HttpRequest live at the top level of jsonPayload:
	// the Logging API rejects the whole entry on unknown httpRequest subfields.
	SchemaGCP = &Schema{
		Timestamp:          "time",                                           // https://cloud.google.com/logging/docs/structured-logging#special-payload-fields
		Level:              GCPLevel,                                         // https://cloud.google.com/logging/docs/reference/v2/rest/v2/LogEntry#LogSeverity
		Message:            "message",                                        // https://cloud.google.com/logging/docs/structured-logging#special-payload-fields
		ErrorMessage:       "error",                                          // Custom jsonPayload field; Error Reporting reads stack_trace/message instead.
		ErrorType:          "error_type",                                     // Custom jsonPayload field.
		ErrorStackTrace:    "stack_trace",                                    // https://cloud.google.com/error-reporting/docs/formatting-error-messages
		SourceFile:         "logging.googleapis.com/sourceLocation:file",     // https://cloud.google.com/logging/docs/structured-logging#special-payload-fields
		SourceLine:         "logging.googleapis.com/sourceLocation:line",     // https://cloud.google.com/logging/docs/structured-logging#special-payload-fields
		SourceFunction:     "logging.googleapis.com/sourceLocation:function", // https://cloud.google.com/logging/docs/structured-logging#special-payload-fields
		RequestURL:         "httpRequest:requestUrl",                         // https://cloud.google.com/logging/docs/reference/v2/rest/v2/LogEntry#HttpRequest
		RequestMethod:      "httpRequest:requestMethod",                      // https://cloud.google.com/logging/docs/reference/v2/rest/v2/LogEntry#HttpRequest
		RequestPath:        "requestPath",                                    // Custom jsonPayload field; not part of LogEntry#HttpRequest.
		RequestRemoteIP:    "httpRequest:remoteIp",                           // https://cloud.google.com/logging/docs/reference/v2/rest/v2/LogEntry#HttpRequest
		RequestHost:        "host",                                           // Custom jsonPayload field; not part of LogEntry#HttpRequest.
		RequestScheme:      "scheme",                                         // Custom jsonPayload field; not part of LogEntry#HttpRequest.
		RequestProto:       "httpRequest:protocol",                           // https://cloud.google.com/logging/docs/reference/v2/rest/v2/LogEntry#HttpRequest
		RequestHeaders:     "requestHeaders",                                 // Custom jsonPayload field; not part of LogEntry#HttpRequest.
		RequestBody:        "requestBody",                                    // Custom jsonPayload field; not part of LogEntry#HttpRequest.
		RequestBytes:       "httpRequest:requestSize",                        // https://cloud.google.com/logging/docs/reference/v2/rest/v2/LogEntry#HttpRequest
		RequestBytesUnread: "requestUnreadSize",                              // Custom jsonPayload field; not part of LogEntry#HttpRequest.
		RequestUserAgent:   "httpRequest:userAgent",                          // https://cloud.google.com/logging/docs/reference/v2/rest/v2/LogEntry#HttpRequest
		RequestReferer:     "httpRequest:referer",                            // https://cloud.google.com/logging/docs/reference/v2/rest/v2/LogEntry#HttpRequest
		ResponseHeaders:    "responseHeaders",                                // Custom jsonPayload field; not part of LogEntry#HttpRequest.
		ResponseBody:       "responseBody",                                   // Custom jsonPayload field; not part of LogEntry#HttpRequest.
		ResponseStatus:     "httpRequest:status",                             // https://cloud.google.com/logging/docs/reference/v2/rest/v2/LogEntry#HttpRequest
		ResponseDuration:   GCPResponseDuration,                              // https://cloud.google.com/logging/docs/reference/v2/rest/v2/LogEntry#HttpRequest.FIELDS.latency
		ResponseBytes:      "httpRequest:responseSize",                       // https://cloud.google.com/logging/docs/reference/v2/rest/v2/LogEntry#HttpRequest
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
		return slog.String(s.Timestamp, a.Value.Time().Format(time.RFC3339Nano))
	case slog.LevelKey:
		if s.Level == "" {
			return a
		}
		if s.Level == GCPLevel {
			if lvl, ok := a.Value.Any().(slog.Level); ok {
				return slog.String(s.Level, gcpLogSeverity(lvl))
			}
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

// gcpLogSeverity maps slog levels to GCP LogSeverity values, which have no "WARN".
//
// Reference: https://cloud.google.com/logging/docs/reference/v2/rest/v2/LogEntry#LogSeverity
func gcpLogSeverity(level slog.Level) string {
	switch {
	case level < slog.LevelInfo:
		return "DEBUG"
	case level < slog.LevelWarn:
		return "INFO"
	case level < slog.LevelError:
		return "WARNING"
	default:
		return "ERROR"
	}
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
