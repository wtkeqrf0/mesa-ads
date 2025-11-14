package configs

import (
	"log/slog"
	"strings"
)

// Logger defines configuration options for the structured logger. The
// Level controls the minimum level emitted by the logger. Valid values
// include "debug", "info", "warn" and "error". Format determines the
// output encoding and may be "text" (default) or "json". An unknown
// format falls back to "text".
type Logger struct {
	Level  string `env:"LEVEL" envDefault:"info"`
	Format string `env:"FORMAT" envDefault:"text"`
}

// SlogLevel converts the textual level into a slog.Level. Unknown levels
// default to slog.LevelInfo.
func (c Logger) SlogLevel() slog.Level {
	switch strings.ToLower(c.Level) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error", "err":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// SlogFormat validates and normalises the requested log format. Supported
// formats are "text" and "json". Any other value returns "text".
func (c Logger) SlogFormat() string {
	switch strings.ToLower(c.Format) {
	case "json":
		return "json"
	default:
		return "text"
	}
}
