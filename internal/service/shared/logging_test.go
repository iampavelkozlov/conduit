package shared

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
)

func TestServiceLoggerAndErrorAttrs(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	require.Equal(t, logger, ServiceLogger(logger))
	require.NotNil(t, ServiceLogger())

	articleID := uuid.MustParse("0199d9dd-2108-8000-8000-000000000001")
	logger.LogAttrs(t.Context(), slog.LevelError, "comment repo err", ErrorAttrs(errors.New("query failed"), slog.String("user_id", "user-1"), UUIDAttr("article_id", UUIDToPG(articleID)))...)

	require.JSONEq(t, `{
		"level":"ERROR",
		"msg":"comment repo err",
		"err":"query failed",
		"user_id":"user-1",
		"article_id":"0199d9dd-2108-8000-8000-000000000001"
	}`, stripLogTime(t, output.Bytes()))
}

func TestIsExpectedError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "api error", err: Validation("body", "invalid"), want: true},
		{name: "unauthorized", err: ErrUnauthorized, want: true},
		{name: "forbidden", err: ErrForbidden, want: true},
		{name: "not found", err: ErrNotFound, want: true},
		{name: "validation", err: ErrValidation, want: true},
		{name: "postgres no rows", err: pgx.ErrNoRows, want: true},
		{name: "unique conflict", err: &pgconn.PgError{Code: "23505"}, want: true},
		{name: "repository failure", err: errors.New("connection failed"), want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, IsExpectedError(tt.err))
		})
	}
}

func stripLogTime(t *testing.T, data []byte) string {
	t.Helper()
	var record map[string]any
	require.NoError(t, json.Unmarshal(data, &record))
	delete(record, "time")
	result, err := json.Marshal(record)
	require.NoError(t, err)
	return string(result)
}
