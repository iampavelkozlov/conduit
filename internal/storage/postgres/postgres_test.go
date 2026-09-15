package postgres

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewRejectsInvalidDSN(t *testing.T) {
	pool, err := New(t.Context(), "://", slog.New(slog.DiscardHandler))
	require.ErrorContains(t, err, "pgxpool new")
	require.Nil(t, pool)
}
