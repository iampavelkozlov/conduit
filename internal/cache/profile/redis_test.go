package profilecache

import (
	"testing"
	"time"

	profiledomain "conduit/internal/service/profile"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestRedisSetGetAndDelete(t *testing.T) {
	t.Parallel()
	server := miniredis.RunT(t)
	cache := New(server.Addr(), "", 0, 10*time.Minute)
	t.Cleanup(func() { require.NoError(t, cache.Close()) })
	id := uuid.New()
	bio, image := "bio", "image"
	want := profiledomain.Profile{UserID: id, Username: "alice", Bio: &bio, Image: &image}

	require.NoError(t, cache.Set(t.Context(), want))
	byID, found, err := cache.GetByID(t.Context(), id)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, want, byID)
	byUsername, found, err := cache.GetByUsername(t.Context(), "alice")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, want, byUsername)
	require.Greater(t, server.TTL(idKey(id)), time.Duration(0))

	require.NoError(t, cache.Delete(t.Context(), id, "alice", "alice"))
	_, found, err = cache.GetByID(t.Context(), id)
	require.NoError(t, err)
	require.False(t, found)
	_, found, err = cache.GetByUsername(t.Context(), "alice")
	require.NoError(t, err)
	require.False(t, found)
}

func TestRedisCacheMiss(t *testing.T) {
	t.Parallel()
	server := miniredis.RunT(t)
	cache := New(server.Addr(), "", 0, time.Minute)
	t.Cleanup(func() { require.NoError(t, cache.Close()) })

	profile, found, err := cache.GetByID(t.Context(), uuid.New())
	require.NoError(t, err)
	require.False(t, found)
	require.Equal(t, profiledomain.Profile{}, profile)
}

func TestRedisRejectsMalformedPayload(t *testing.T) {
	t.Parallel()
	server := miniredis.RunT(t)
	cache := New(server.Addr(), "", 0, time.Minute)
	t.Cleanup(func() { require.NoError(t, cache.Close()) })
	id := uuid.New()
	require.NoError(t, server.Set(idKey(id), "not-json"))

	_, _, err := cache.GetByID(t.Context(), id)
	require.ErrorContains(t, err, "unmarshal profile cache value")
}

func TestRedisReportsUnavailableClient(t *testing.T) {
	t.Parallel()
	server := miniredis.RunT(t)
	cache := New(server.Addr(), "", 0, time.Minute)
	require.NoError(t, cache.Close())

	_, _, err := cache.GetByID(t.Context(), uuid.New())
	require.ErrorContains(t, err, "get profile cache value")
	require.Error(t, cache.Set(t.Context(), profiledomain.Profile{UserID: uuid.New(), Username: "alice"}))
	require.Error(t, cache.Delete(t.Context(), uuid.New(), "alice"))
}
