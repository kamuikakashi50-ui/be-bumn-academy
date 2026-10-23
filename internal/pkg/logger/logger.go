// Package logger menyiapkan logger terstruktur (log/slog) untuk aplikasi.
package logger

import (
	"log/slog"
	"os"
	"strings"
)

// Options mengatur pembuatan logger.
type Options struct {
	Level  string
	Pretty bool
}

// New membentuk logger slog sesuai konfigurasi.
func New(opts Options) *slog.Logger {
	level := parseLevel(opts.Level)

	var handler slog.Handler
	if opts.Pretty {
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	} else {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)
	return logger
}

func parseLevel(raw string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
