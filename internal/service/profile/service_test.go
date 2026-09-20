package profile

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	profilepostgres "conduit/internal/gen/profile/postgres"
	"conduit/internal/service/shared"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
)

type fakeRepository struct {
	byID        profilepostgres.Profile
	byIDErr     error
	byUsername  profilepostgres.Profile
	usernameErr error
	created     profilepostgres.Profile
	createErr   error
	listRows    []profilepostgres.Profile
	listErr     error
	getCalls    int
	listIDs     []pgtype.UUID
	updated     profilepostgres.Profile
	updateErr   error
}

func (f *fakeRepository) GetProfileByID(context.Context, pgtype.UUID) (profilepostgres.Profile, error) {
	f.getCalls++
	return f.byID, f.byIDErr
}

func (f *fakeRepository) GetProfileByUsername(context.Context, string) (profilepostgres.Profile, error) {
	return f.byUsername, f.usernameErr
}

func (f *fakeRepository) CreateProfile(context.Context, profilepostgres.CreateProfileParams) (profilepostgres.Profile, error) {
	return f.created, f.createErr
}

func (f *fakeRepository) ListProfilesByIDs(_ context.Context, ids []pgtype.UUID) ([]profilepostgres.Profile, error) {
	f.listIDs = ids
	return f.listRows, f.listErr
}

func (f *fakeRepository) UpdateProfile(context.Context, profilepostgres.UpdateProfileParams) (profilepostgres.Profile, error) {
	return f.updated, f.updateErr
}

type fakeCache struct {
	profiles      map[uuid.UUID]Profile
	byUsername    map[string]Profile
	readErr       error
	setErr        error
	sets          []Profile
	deletedID     uuid.UUID
	deletedNames  []string
	invalidateErr error
}

func (f *fakeCache) GetByID(_ context.Context, id uuid.UUID) (Profile, bool, error) {
	if f.readErr != nil {
		return Profile{}, false, f.readErr
	}
	profile, ok := f.profiles[id]
	return profile, ok, nil
}

func (f *fakeCache) GetByUsername(_ context.Context, username string) (Profile, bool, error) {
	profile, ok := f.byUsername[username]
	return profile, ok, f.readErr
}

func (f *fakeCache) Set(_ context.Context, profile Profile) error {
	f.sets = append(f.sets, profile)
	return f.setErr
}

func (f *fakeCache) Delete(_ context.Context, id uuid.UUID, names ...string) error {
	f.deletedID, f.deletedNames = id, names
	return f.invalidateErr
}

func TestGetUsesCache(t *testing.T) {
	t.Parallel()
	id := uuid.New()
	want := Profile{UserID: id, Username: "alice"}
	repo := &fakeRepository{}
	service := New(repo, &fakeCache{profiles: map[uuid.UUID]Profile{id: want}}, slog.New(slog.DiscardHandler))

	got, err := service.Get(t.Context(), id)
	require.NoError(t, err)
	require.Equal(t, want, got)
	require.Zero(t, repo.getCalls)
}

func TestGetDegradesWhenCacheFails(t *testing.T) {
	t.Parallel()
	id := uuid.New()
	repo := &fakeRepository{byID: row(id, "alice")}
	cache := &fakeCache{readErr: errors.New("redis unavailable")}
	service := New(repo, cache, slog.New(slog.DiscardHandler))

	got, err := service.Get(t.Context(), id)
	require.NoError(t, err)
	require.Equal(t, "alice", got.Username)
	require.Equal(t, 1, repo.getCalls)
	require.Len(t, cache.sets, 1)
}

func TestBatchGetDeduplicatesAndLoadsOnlyMisses(t *testing.T) {
	t.Parallel()
	cachedID, missingID := uuid.New(), uuid.New()
	repo := &fakeRepository{listRows: []profilepostgres.Profile{row(missingID, "bob")}}
	cache := &fakeCache{profiles: map[uuid.UUID]Profile{cachedID: {UserID: cachedID, Username: "alice"}}}
	service := New(repo, cache, slog.New(slog.DiscardHandler))

	got, err := service.BatchGet(t.Context(), []uuid.UUID{cachedID, missingID, missingID})
	require.NoError(t, err)
	require.Len(t, got, 2)
	require.Equal(t, []pgtype.UUID{shared.UUIDToPG(missingID)}, repo.listIDs)
}

func TestUpdateInvalidatesIDAndBothUsernameKeys(t *testing.T) {
	t.Parallel()
	id := uuid.New()
	newUsername := "new-name"
	repo := &fakeRepository{byID: row(id, "old-name"), updated: row(id, newUsername)}
	cache := &fakeCache{}
	service := New(repo, cache, slog.New(slog.DiscardHandler))

	got, err := service.Update(t.Context(), id, Update{Username: &newUsername, BioSet: true})
	require.NoError(t, err)
	require.Equal(t, newUsername, got.Username)
	require.Equal(t, id, cache.deletedID)
	require.Equal(t, []string{"old-name", newUsername}, cache.deletedNames)
	require.Len(t, cache.sets, 1)
}

func TestCreateProfile(t *testing.T) {
	t.Parallel()
	id := uuid.New()
	t.Run("creates and caches", func(t *testing.T) {
		cache := &fakeCache{}
		service := New(&fakeRepository{created: row(id, "alice")}, cache, slog.New(slog.DiscardHandler))
		profile, created, err := service.Create(t.Context(), id, "alice")
		require.NoError(t, err)
		require.True(t, created)
		require.Equal(t, "alice", profile.Username)
		require.Len(t, cache.sets, 1)
	})
	t.Run("idempotent existing ID", func(t *testing.T) {
		service := New(&fakeRepository{createErr: pgx.ErrNoRows, byID: row(id, "alice")}, &fakeCache{}, slog.New(slog.DiscardHandler))
		_, created, err := service.Create(t.Context(), id, "alice")
		require.NoError(t, err)
		require.False(t, created)
	})
	t.Run("validates username", func(t *testing.T) {
		service := New(&fakeRepository{}, &fakeCache{}, slog.New(slog.DiscardHandler))
		_, _, err := service.Create(t.Context(), id, " ")
		require.ErrorIs(t, err, shared.ErrValidation)
	})
	t.Run("maps duplicate username", func(t *testing.T) {
		service := New(&fakeRepository{createErr: &pgconn.PgError{Code: "23505"}}, &fakeCache{}, slog.New(slog.DiscardHandler))
		_, _, err := service.Create(t.Context(), id, "alice")
		require.ErrorIs(t, err, shared.ErrValidation)
	})
}

func TestGetByUsernameCacheAndRepository(t *testing.T) {
	t.Parallel()
	id := uuid.New()
	cached := Profile{UserID: id, Username: "alice"}
	service := New(&fakeRepository{}, &fakeCache{byUsername: map[string]Profile{"alice": cached}}, slog.New(slog.DiscardHandler))
	got, err := service.GetByUsername(t.Context(), "alice")
	require.NoError(t, err)
	require.Equal(t, cached, got)

	cache := &fakeCache{readErr: errors.New("redis")}
	service = New(&fakeRepository{byUsername: row(id, "bob")}, cache, slog.New(slog.DiscardHandler))
	got, err = service.GetByUsername(t.Context(), "bob")
	require.NoError(t, err)
	require.Equal(t, "bob", got.Username)
	require.Len(t, cache.sets, 1)
}

func TestRepositoryAndConversionErrors(t *testing.T) {
	t.Parallel()
	id := uuid.New()
	repositoryErr := errors.New("database unavailable")
	t.Run("get missing", func(t *testing.T) {
		service := New(&fakeRepository{byIDErr: pgx.ErrNoRows}, &fakeCache{}, slog.New(slog.DiscardHandler))
		_, err := service.Get(t.Context(), id)
		require.ErrorIs(t, err, shared.ErrNotFound)
	})
	t.Run("username repository error", func(t *testing.T) {
		service := New(&fakeRepository{usernameErr: repositoryErr}, &fakeCache{}, slog.New(slog.DiscardHandler))
		_, err := service.GetByUsername(t.Context(), "alice")
		require.ErrorIs(t, err, repositoryErr)
	})
	t.Run("batch repository error", func(t *testing.T) {
		service := New(&fakeRepository{listErr: repositoryErr}, &fakeCache{}, slog.New(slog.DiscardHandler))
		_, err := service.BatchGet(t.Context(), []uuid.UUID{id})
		require.ErrorIs(t, err, repositoryErr)
	})
	t.Run("malformed stored UUID", func(t *testing.T) {
		service := New(&fakeRepository{byID: profilepostgres.Profile{Username: "alice"}}, &fakeCache{}, slog.New(slog.DiscardHandler))
		_, err := service.Get(t.Context(), id)
		require.Error(t, err)
	})
}

func TestUpdateValidationAndFailures(t *testing.T) {
	t.Parallel()
	id := uuid.New()
	repositoryErr := errors.New("database unavailable")
	empty := " "
	service := New(&fakeRepository{}, &fakeCache{}, slog.New(slog.DiscardHandler))
	_, err := service.Update(t.Context(), id, Update{Username: &empty})
	require.ErrorIs(t, err, shared.ErrValidation)

	service = New(&fakeRepository{byIDErr: pgx.ErrNoRows}, &fakeCache{}, slog.New(slog.DiscardHandler))
	_, err = service.Update(t.Context(), id, Update{})
	require.ErrorIs(t, err, shared.ErrNotFound)

	service = New(&fakeRepository{byID: row(id, "alice"), updateErr: repositoryErr}, &fakeCache{}, slog.New(slog.DiscardHandler))
	_, err = service.Update(t.Context(), id, Update{})
	require.ErrorIs(t, err, repositoryErr)

	cache := &fakeCache{invalidateErr: errors.New("redis"), setErr: errors.New("redis")}
	service = New(&fakeRepository{byID: row(id, "alice"), updated: row(id, "alice")}, cache, slog.New(slog.DiscardHandler))
	_, err = service.Update(t.Context(), id, Update{})
	require.NoError(t, err)
}

func TestNullableTextHelpers(t *testing.T) {
	t.Parallel()
	require.Nil(t, textPointer(pgtype.Text{}))
	value := "value"
	require.Equal(t, value, *textPointer(pgtype.Text{String: value, Valid: true}))
	require.Equal(t, pgtype.Text{}, textValue(nil))
	require.Equal(t, pgtype.Text{String: value, Valid: true}, textValue(&value))
	require.Empty(t, stringValue(nil))
	require.Equal(t, value, stringValue(&value))
}

func TestAdditionalRepositoryAndConversionFailures(t *testing.T) {
	t.Parallel()
	id := uuid.New()
	repositoryErr := errors.New("database unavailable")
	t.Run("create idempotent lookup fails", func(t *testing.T) {
		service := New(&fakeRepository{createErr: pgx.ErrNoRows, byIDErr: repositoryErr}, &fakeCache{}, slog.New(slog.DiscardHandler))
		_, _, err := service.Create(t.Context(), id, "alice")
		require.ErrorIs(t, err, repositoryErr)
	})
	t.Run("create returns malformed row", func(t *testing.T) {
		service := New(&fakeRepository{created: profilepostgres.Profile{Username: "alice"}}, &fakeCache{}, slog.New(slog.DiscardHandler))
		_, _, err := service.Create(t.Context(), id, "alice")
		require.Error(t, err)
	})
	t.Run("username missing", func(t *testing.T) {
		service := New(&fakeRepository{usernameErr: pgx.ErrNoRows}, &fakeCache{}, slog.New(slog.DiscardHandler))
		_, err := service.GetByUsername(t.Context(), "alice")
		require.ErrorIs(t, err, shared.ErrNotFound)
	})
	t.Run("username returns malformed row", func(t *testing.T) {
		service := New(&fakeRepository{byUsername: profilepostgres.Profile{Username: "alice"}}, &fakeCache{}, slog.New(slog.DiscardHandler))
		_, err := service.GetByUsername(t.Context(), "alice")
		require.Error(t, err)
	})
	t.Run("batch malformed row", func(t *testing.T) {
		service := New(&fakeRepository{listRows: []profilepostgres.Profile{{Username: "alice"}}}, &fakeCache{}, slog.New(slog.DiscardHandler))
		_, err := service.BatchGet(t.Context(), []uuid.UUID{id})
		require.Error(t, err)
	})
	t.Run("update returns malformed row", func(t *testing.T) {
		service := New(&fakeRepository{byID: row(id, "alice"), updated: profilepostgres.Profile{Username: "alice"}}, &fakeCache{}, slog.New(slog.DiscardHandler))
		_, err := service.Update(t.Context(), id, Update{})
		require.Error(t, err)
	})
}

func row(id uuid.UUID, username string) profilepostgres.Profile {
	return profilepostgres.Profile{UserID: shared.UUIDToPG(id), Username: username}
}
