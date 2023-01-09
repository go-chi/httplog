package httplog

import (
	"os"
	"strings"
	"time"

	"golang.org/x/exp/slog"
)

var DefaultOptions = Options{
	LogLevel:        slog.LevelInfo,
	LevelFieldName:  "level",
	JSON:            false,
	Concise:         false,
	Tags:            nil,
	SkipHeaders:     nil,
	TimeFieldFormat: time.RFC3339Nano,
	TimeFieldName:   "timestamp",
}

type Options struct {
	// LogLevel defines the minimum level of severity that app should log.
	// Must be one of:
	// slog.LevelDebug, slog.LevelInfo, slog.LevelWarn, slog.LevelError
	LogLevel slog.Level

	// LevelFieldName sets the field name for the log level or severity.
	// Some providers parse and search for different field names.
	LevelFieldName string

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

	// SkipHeaders are additional headers which are redacted from the logs
	SkipHeaders []string

	// TimeFieldFormat defines the time format of the Time field, defaulting to "time.RFC3339Nano" see options at:
	// https://pkg.go.dev/time#pkg-constants
	TimeFieldFormat string

	// TimeFieldName sets the field name for the time field.
	// Some providers parse and search for different field names.
	TimeFieldName string
}

// Configure will set new global/default options for the httplog and behaviour
// of underlying zerolog pkg and its global logger.
func Configure(opts Options) {
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

	// Pre-downcase all SkipHeaders
	for i, header := range opts.SkipHeaders {
		opts.SkipHeaders[i] = strings.ToLower(header)
	}

	DefaultOptions = opts

	replaceAttrs := func(_ []string, a slog.Attr) slog.Attr {
		switch a.Key {
		case slog.LevelKey:
			a.Key = opts.LevelFieldName
		case slog.TimeKey:
			a.Key = opts.TimeFieldName
			a.Value = slog.StringValue(a.Value.Time().Format(opts.TimeFieldFormat))
		}
		return a
	}

	handlerOpts := &slog.HandlerOptions{
		AddSource:   true,
		Level:       opts.LogLevel,
		ReplaceAttr: replaceAttrs,
	}

	if !opts.JSON {
		slog.SetDefault(slog.New(NewPrettyHandler(os.Stderr, handlerOpts)))
	} else {
		slog.SetDefault(slog.New(handlerOpts.NewJSONHandler(os.Stderr)))
	}
}
