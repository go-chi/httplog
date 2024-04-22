package httplog

import (
	"io"
	"os"
	"strings"
	"time"

	"log/slog"
)

var defaultOptions = Options{
	LogLevel:           slog.LevelInfo,
	LevelFieldName:     "level",
	JSON:               false,
	Concise:            true,
	Tags:               nil,
	RequestHeaders:     true,
	HideRequestHeaders: nil,
	QuietDownRoutes:    nil,
	QuietDownPeriod:    0,
	TimeFieldFormat:    time.RFC3339Nano,
	TimeFieldName:      "timestamp",
	MessageFieldName:   "message",
}

type Options struct {
	// LogLevel defines the minimum level of severity that app should log.
	// Must be one of:
	// slog.LevelDebug, slog.LevelInfo, slog.LevelWarn, slog.LevelError
	LogLevel slog.Level

	// LevelFieldName sets the field name for the log level or severity.
	// Some providers parse and search for different field names.
	LevelFieldName string

	// MessageFieldName sets the field name for the message.
	// Default is "msg".
	MessageFieldName string

	// JSON enables structured logging output in json. Make sure to enable this
	// in production mode so log aggregators can receive data in parsable format.
	//
	// In local development mode, its appropriate to set this value to false to
	// receive pretty output and stacktraces to stdout.
	JSON bool

	// Concise mode includes fewer log details during the request flow. For example
	// excluding details like request content length, user-agent and other details.
	// This is useful if during development your console is too noisy.
	Concise bool

	// Tags are additional fields included at the root level of all logs.
	// These can be useful for example the commit hash of a build, or an environment
	// name like prod/stg/dev
	Tags map[string]string

	// RequestHeaders enables logging of all request headers, however sensitive
	// headers like authorization, cookie and set-cookie are hidden.
	RequestHeaders bool

	// HideRequestHeaders are additional requests headers which are redacted from the logs
	HideRequestHeaders []string

	// ResponseHeaders enables logging of all response headers.
	ResponseHeaders bool

	// QuietDownRoutes are routes which are temporarily excluded from logging for a QuietDownPeriod after it occurs
	// for the first time
	// to cancel noise from logging for routes that are known to be noisy.
	QuietDownRoutes []string

	// QuietDownPeriod is the duration for which a route is excluded from logging after it occurs for the first time
	// if the route is in QuietDownRoutes
	QuietDownPeriod time.Duration

	// TimeFieldFormat defines the time format of the Time field, defaulting to "time.RFC3339Nano" see options at:
	// https://pkg.go.dev/time#pkg-constants
	TimeFieldFormat string

	// TimeFieldName sets the field name for the time field.
	// Some providers parse and search for different field names.
	TimeFieldName string

	// SourceFieldName sets the field name for the source field which logs
	// the location in the program source code where the logger was called.
	// If set to "" then it'll be disabled.
	SourceFieldName string

	// Writer is the log writer, default is os.Stdout
	Writer io.Writer

	// ReplaceAttrsOverride allows to add custom logic to replace attributes
	// in addition to the default logic set in this package.
	ReplaceAttrsOverride func(groups []string, a slog.Attr) slog.Attr
}

// Configure will set new options for the httplog instance and behaviour
// of underlying slog pkg and its global logger.
func (l *Logger) Configure(opts Options) {
	// if opts.LogLevel is not set
	// it would be 0 which is LevelInfo

	if opts.LevelFieldName == "" {
		opts.LevelFieldName = "level"
	}

	if opts.TimeFieldFormat == "" {
		opts.TimeFieldFormat = time.RFC3339Nano
	}

	if opts.TimeFieldName == "" {
		opts.TimeFieldName = "timestamp"
	}

	if len(opts.QuietDownRoutes) > 0 {
		if opts.QuietDownPeriod == 0 {
			opts.QuietDownPeriod = 5 * time.Minute
		}
	}

	// Pre-downcase all SkipHeaders
	for i, header := range opts.HideRequestHeaders {
		opts.HideRequestHeaders[i] = strings.ToLower(header)
	}

	l.Options = opts

	var addSource bool
	if opts.SourceFieldName != "" {
		addSource = true
	}

	replaceAttrs := func(groups []string, a slog.Attr) slog.Attr {
		switch a.Key {
		case slog.LevelKey:
			a.Key = opts.LevelFieldName
		case slog.TimeKey:
			a.Key = opts.TimeFieldName
			a.Value = slog.StringValue(a.Value.Time().Format(opts.TimeFieldFormat))
		case slog.MessageKey:
			if opts.MessageFieldName != "" {
				a.Key = opts.MessageFieldName
			}
		case slog.SourceKey:
			if opts.SourceFieldName != "" {
				a.Key = opts.SourceFieldName
			}
		}

		if opts.ReplaceAttrsOverride != nil {
			return opts.ReplaceAttrsOverride(groups, a)
		}
		return a
	}

	handlerOpts := &slog.HandlerOptions{
		Level:       opts.LogLevel,
		ReplaceAttr: replaceAttrs,
		AddSource:   addSource,
	}

	writer := opts.Writer
	if writer == nil {
		writer = os.Stdout
	}

	if !opts.JSON {
		l.Logger = slog.New(NewPrettyHandler(writer, handlerOpts))
	} else {
		l.Logger = slog.New(slog.NewJSONHandler(writer, handlerOpts))
	}
}

func LevelByName(name string) slog.Level {
	switch strings.ToUpper(name) {
	case "DEBUG":
		return slog.LevelDebug
	case "INFO":
		return slog.LevelInfo
	case "WARN":
		return slog.LevelWarn
	case "ERROR":
		return slog.LevelError
	default:
		return 0
	}
}
