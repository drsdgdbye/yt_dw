package logger

import (
	"log/slog"
	"os"
	"strings"
)

// Init настраивает глобальный slog с указанным уровнем и форматом (text/json).
func Init(level, format string) {
	var lvl slog.Level
	if err := lvl.UnmarshalText([]byte(level)); err != nil {
		lvl = slog.LevelInfo
	}

	var handler slog.Handler
	switch strings.ToLower(format) {
	case "json":
		handler = slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: lvl})
	default:
		handler = slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: lvl})
	}

	slog.SetDefault(slog.New(handler))
}
