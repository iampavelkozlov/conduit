package comment

import (
	"encoding/binary"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
)

func TestCommentUUIDRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		id   int
	}{
		{name: "minimum", id: 1},
		{name: "regular", id: 42},
		{name: "crosses low sixty bits", id: 1 << 60},
		{name: "maximum", id: int(commentIDMask)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded, err := commentUUIDFromID(tt.id)
			require.NoError(t, err)
			require.Equal(t, commentUUIDPrefix, binary.BigEndian.Uint64(encoded.Bytes[:8]))
			require.Equal(t, commentUUIDVariant, binary.BigEndian.Uint64(encoded.Bytes[8:])&^commentIDMask)

			decoded, err := commentIDFromUUID(encoded)
			require.NoError(t, err)
			require.Equal(t, tt.id, decoded)
		})
	}
}

func TestCommentUUIDRejectsInvalidValues(t *testing.T) {
	foreignUUID := uuid.New()
	wrongVariant := commentUUIDFromUint64(1)
	binary.BigEndian.PutUint64(wrongVariant.Bytes[8:], 1)
	zeroPayload := commentUUIDFromUint64(0)

	tests := []struct {
		name string
		id   pgtype.UUID
	}{
		{name: "null database value", id: pgtype.UUID{}},
		{name: "foreign UUID namespace", id: pgtype.UUID{Bytes: foreignUUID, Valid: true}},
		{name: "wrong UUID variant", id: wrongVariant},
		{name: "zero payload", id: zeroPayload},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := commentIDFromUUID(tt.id)
			require.ErrorIs(t, err, errInvalidCommentID)
		})
	}
}

func TestCommentUUIDFromIDRejectsOutOfRangeValues(t *testing.T) {
	for _, id := range []int{-1, 0} {
		_, err := commentUUIDFromID(id)
		require.ErrorIs(t, err, errInvalidCommentID)
	}
}

func TestNewCommentUUID(t *testing.T) {
	id := newCommentUUID()
	commentID, err := commentIDFromUUID(id)
	require.NoError(t, err)
	require.Positive(t, commentID)
}
