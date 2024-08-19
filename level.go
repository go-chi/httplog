package httplog

import (
	"log/slog"
	"strings"
)

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
