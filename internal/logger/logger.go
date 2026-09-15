package logger

import (
	"log/slog"
	"os"
	"strings"

	"conduit/internal/config"
)

func New(cfg config.LoggerConfig) (*slog.Logger, error) {
	level, err := parseLevel(cfg.Level)
	if err != nil {
		return nil, err
	}

	opts := &slog.HandlerOptions{Level: level}

	var handler slog.Handler
	switch strings.ToLower(cfg.Format) {
	case "json":
		handler = slog.NewJSONHandler(os.Stdout, opts)
	case "text", "":
		handler = slog.NewTextHandler(os.Stdout, opts)
	default:
		return nil, ErrUnknownFormat(cfg.Format)
	}

	return slog.New(handler), nil
}

func parseLevel(v string) (slog.Level, error) {
	switch strings.ToLower(v) {
	case "debug":
		return slog.LevelDebug, nil
	case "info", "":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, ErrUnknownLevel(v)
	}
}

type ErrUnknownLevel string

func (e ErrUnknownLevel) Error() string {
	return "unknown log level: " + string(e)
}

type ErrUnknownFormat string

func (e ErrUnknownFormat) Error() string {
	return "unknown log format: " + string(e)
}
