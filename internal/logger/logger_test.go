package logger

import (
	"log/slog"
	"testing"

	"conduit/internal/config"

	"github.com/stretchr/testify/require"
)

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input   string
		want    slog.Level
		wantErr bool
	}{
		{input: "", want: slog.LevelInfo},
		{input: "DEBUG", want: slog.LevelDebug},
		{input: "info", want: slog.LevelInfo},
		{input: "warn", want: slog.LevelWarn},
		{input: "warning", want: slog.LevelWarn},
		{input: "error", want: slog.LevelError},
		{input: "trace", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			level, err := parseLevel(tt.input)
			if tt.wantErr {
				var target ErrUnknownLevel
				require.ErrorAs(t, err, &target)
				require.Contains(t, err.Error(), tt.input)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, level)
		})
	}
}

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		cfg     config.LoggerConfig
		wantErr error
	}{
		{name: "text", cfg: config.LoggerConfig{Level: "info", Format: "text"}},
		{name: "default text", cfg: config.LoggerConfig{}},
		{name: "json", cfg: config.LoggerConfig{Level: "debug", Format: "JSON"}},
		{name: "invalid level", cfg: config.LoggerConfig{Level: "trace"}, wantErr: ErrUnknownLevel("trace")},
		{name: "invalid format", cfg: config.LoggerConfig{Level: "info", Format: "xml"}, wantErr: ErrUnknownFormat("xml")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, err := New(tt.cfg)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				require.EqualError(t, err, tt.wantErr.Error())
				require.Nil(t, value)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, value)
		})
	}
}
