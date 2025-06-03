package httplog

type Format struct {
	// Base attributes.
	Timestamp       string
	Level           string
	Message         string
	Error           string
	ErrorStackTrace string

	// Request attributes.
	// NOTE: RequestQuery is not supported as it would likely leak sensitive data.
	RequestURL         string
	RequestMethod      string
	RequestPath        string
	RequestRemoteIP    string
	RequestHost        string
	RequestScheme      string
	RequestProto       string
	RequestHeaders     string
	RequestBody        string
	RequestBytes       string
	RequestBytesUnread string
	RequestUserAgent   string
	RequestReferer     string

	// Response attributes.
	ResponseHeaders  string
	ResponseBody     string
	ResponseStatus   string
	ResponseDuration string
	ResponseBytes    string

	// Optional delimiter denoting nested objects.
	GroupDelimiter string
}

var (
	// Elastic Common Schema (ECS) version 9.0.0.
	//
	// https://www.elastic.co/guide/en/ecs/current/ecs-http.html
	ECS = Format{
		Timestamp:          "@timestamp",
		Level:              "log.level",
		Message:            "message",
		Error:              "error.message",
		ErrorStackTrace:    "error.stack_trace",
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

	// OpenTelemetry (OTEL) semantic conventions 1.34.0
	//
	// https://opentelemetry.io/docs/specs/semconv/http/http-metrics
	OTEL = Format{
		Timestamp:          "timestamp",
		Level:              "severity_text",
		Message:            "body",
		Error:              "exception.message",
		ErrorStackTrace:    "exception.stacktrace",
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

	// Google Cloud Platform (GCP)
	//
	// https://cloud.google.com/logging/docs/structured-logging
	// https://cloud.google.com/logging/docs/reference/v2/rest/v2/LogEntry#HttpRequest
	GCP = Format{
		Timestamp:          "timestamp",
		Level:              "severity",
		Message:            "message",
		Error:              "error",
		ErrorStackTrace:    "stack_trace",
		RequestURL:         "httpRequest.requestUrl",
		RequestMethod:      "httpRequest.requestMethod",
		RequestPath:        "httpRequest.requestPath",
		RequestRemoteIP:    "httpRequest.remoteIp",
		RequestHost:        "httpRequest.host",
		RequestScheme:      "httpRequest.scheme",
		RequestProto:       "httpRequest.protocol",
		RequestHeaders:     "httpRequest.requestHeaders",
		RequestBody:        "httpRequest.requestBody",
		RequestBytes:       "httpRequest.requestSize",
		RequestBytesUnread: "httpRequest.requestUnreadSize",
		RequestUserAgent:   "httpRequest.userAgent",
		RequestReferer:     "httpRequest.referer",
		ResponseHeaders:    "httpRequest.responseHeaders",
		ResponseBody:       "httpRequest.responseBody",
		ResponseStatus:     "httpRequest.status",
		ResponseDuration:   "httpRequest.latency",
		ResponseBytes:      "httpRequest.responseSize",
		GroupDelimiter:     ".",
	}

	// Concise format causes fewer log attributes to be printed in request logs.
	// This is useful if your console is too noisy during development.
	Concise = Format{
		Error:              "error",
		ErrorStackTrace:    "stacktrace",
		RequestHeaders:     "request.headers",
		RequestBody:        "request.body",
		RequestBytesUnread: "request.bytesUnread",
		ResponseHeaders:    "response.headers",
		ResponseBody:       "response.body",
	}
)
