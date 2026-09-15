package shared

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewUUIDReturnsUUIDv8(t *testing.T) {
	id := NewUUID()

	require.True(t, id.Valid)
	require.Equal(t, byte(8), id.Bytes[6]>>4)
	require.Equal(t, byte(2), id.Bytes[8]>>6)
}

func TestNewUUIDValueReturnsUUIDv8(t *testing.T) {
	id := NewUUIDValue()

	require.Equal(t, byte(8), id[6]>>4)
	require.Equal(t, byte(2), id[8]>>6)
}
