package user

import (
	"context"
	"errors"
	"testing"

	"conduit/internal/gen/postgres"
	"conduit/internal/models"
	"conduit/internal/service/shared"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestGetCurrentUser(t *testing.T) {
	userID := uuid.New()
	repoErr := errors.New("repository error")
	tests := []struct {
		name    string
		auth    bool
		setup   func(*Mockrepository)
		wantErr error
	}{
		{name: "requires authentication", setup: func(*Mockrepository) {}, wantErr: shared.ErrUnauthorized},
		{name: "maps missing user", auth: true, setup: func(repo *Mockrepository) {
			repo.EXPECT().GetUserByID(gomock.Any(), gomock.Any()).Return(postgres.User{}, pgx.ErrNoRows)
		}, wantErr: shared.ErrNotFound},
		{name: "propagates repository error", auth: true, setup: func(repo *Mockrepository) {
			repo.EXPECT().GetUserByID(gomock.Any(), gomock.Any()).Return(postgres.User{}, repoErr)
		}, wantErr: repoErr},
		{name: "returns current user", auth: true, setup: func(repo *Mockrepository) {
			repo.EXPECT().GetUserByID(gomock.Any(), gomock.Any()).Return(databaseUser(userID), nil)
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := NewMockrepository(ctrl)
			tt.setup(repo)
			ctx := t.Context()
			if tt.auth {
				ctx = shared.WithAccessToken(shared.WithUserID(ctx, userID), "access-token")
			}

			response, err := New(repo, NewMockFollowService(ctrl), NewMockPasswordManager(ctrl)).GetCurrentUser(ctx)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				require.Nil(t, response)
				return
			}
			require.NoError(t, err)
			require.Equal(t, "access-token", response.User.Token)
			require.Equal(t, "user", response.User.Username)
		})
	}
}

func TestUpdateCurrentUser(t *testing.T) {
	userID := uuid.New()
	repoErr := errors.New("repository error")
	email := "new@example.com"
	password := "new-password"
	empty := ""
	tests := []struct {
		name    string
		auth    bool
		request models.UpdateUserRequest
		setup   func(*Mockrepository, *MockPasswordManager)
		wantErr error
	}{
		{name: "requires authentication", setup: func(*Mockrepository, *MockPasswordManager) {}, wantErr: shared.ErrUnauthorized},
		{name: "rejects empty email", auth: true, request: models.UpdateUserRequest{User: models.UpdateUser{EmailSet: true, Email: &empty}}, setup: func(*Mockrepository, *MockPasswordManager) {}, wantErr: shared.ErrValidation},
		{name: "rejects null username", auth: true, request: models.UpdateUserRequest{User: models.UpdateUser{UsernameSet: true}}, setup: func(*Mockrepository, *MockPasswordManager) {}, wantErr: shared.ErrValidation},
		{name: "rejects null password", auth: true, request: models.UpdateUserRequest{User: models.UpdateUser{PasswordSet: true}}, setup: func(*Mockrepository, *MockPasswordManager) {}, wantErr: shared.ErrValidation},
		{name: "rejects short password", auth: true, request: models.UpdateUserRequest{User: models.UpdateUser{PasswordSet: true, Password: &empty}}, setup: func(*Mockrepository, *MockPasswordManager) {}, wantErr: shared.ErrValidation},
		{name: "propagates hash error", auth: true, request: models.UpdateUserRequest{User: models.UpdateUser{PasswordSet: true, Password: &password}}, setup: func(_ *Mockrepository, passwords *MockPasswordManager) {
			passwords.EXPECT().Hash(password).Return("", repoErr)
		}, wantErr: repoErr},
		{name: "maps missing user", auth: true, request: models.UpdateUserRequest{User: models.UpdateUser{EmailSet: true, Email: &email}}, setup: func(repo *Mockrepository, _ *MockPasswordManager) {
			repo.EXPECT().UpdateUser(gomock.Any(), gomock.Any()).Return(postgres.User{}, pgx.ErrNoRows)
		}, wantErr: shared.ErrNotFound},
		{name: "propagates update error", auth: true, request: models.UpdateUserRequest{User: models.UpdateUser{EmailSet: true, Email: &email}}, setup: func(repo *Mockrepository, _ *MockPasswordManager) {
			repo.EXPECT().UpdateUser(gomock.Any(), gomock.Any()).Return(postgres.User{}, repoErr)
		}, wantErr: repoErr},
		{name: "updates supplied fields", auth: true, request: models.UpdateUserRequest{User: models.UpdateUser{EmailSet: true, Email: &email, PasswordSet: true, Password: &password}}, setup: func(repo *Mockrepository, passwords *MockPasswordManager) {
			passwords.EXPECT().Hash(password).Return("hashed-new", nil)
			repo.EXPECT().UpdateUser(gomock.Any(), gomock.Any()).DoAndReturn(func(_ any, params postgres.UpdateUserParams) (postgres.User, error) {
				require.True(t, params.SetEmail)
				require.True(t, params.SetPassword)
				require.Equal(t, "hashed-new", params.PasswordHash)
				user := databaseUser(userID)
				user.Email = email
				return user, nil
			})
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := NewMockrepository(ctrl)
			passwords := NewMockPasswordManager(ctrl)
			tt.setup(repo, passwords)
			ctx := t.Context()
			if tt.auth {
				ctx = shared.WithAccessToken(shared.WithUserID(ctx, userID), "access-token")
			}

			response, err := New(repo, NewMockFollowService(ctrl), passwords).UpdateCurrentUser(ctx, &tt.request)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				require.Nil(t, response)
				return
			}
			require.NoError(t, err)
			require.Equal(t, email, response.User.Email)
			require.Equal(t, "access-token", response.User.Token)
		})
	}
}

func TestProfileMutations(t *testing.T) {
	viewerID := uuid.New()
	targetID := uuid.New()
	repoErr := errors.New("repository error")
	tests := []struct {
		name      string
		auth      bool
		unfollow  bool
		setup     func(*Mockrepository, *MockFollowService)
		wantErr   error
		following bool
	}{
		{name: "follow requires authentication", setup: func(*Mockrepository, *MockFollowService) {}, wantErr: shared.ErrUnauthorized},
		{name: "maps missing target", auth: true, setup: func(repo *Mockrepository, _ *MockFollowService) {
			repo.EXPECT().GetUserByUsername(gomock.Any(), "target").Return(postgres.User{}, pgx.ErrNoRows)
		}, wantErr: shared.ErrNotFound},
		{name: "propagates follow error", auth: true, setup: func(repo *Mockrepository, follows *MockFollowService) {
			repo.EXPECT().GetUserByUsername(gomock.Any(), "target").Return(databaseUserNamed(targetID, "target"), nil)
			follows.EXPECT().Follow(gomock.Any(), viewerID, targetID).Return(repoErr)
		}, wantErr: repoErr},
		{name: "rejects invalid target id", auth: true, setup: func(repo *Mockrepository, _ *MockFollowService) {
			repo.EXPECT().GetUserByUsername(gomock.Any(), "target").Return(postgres.User{Username: "target"}, nil)
		}, wantErr: shared.ErrInvalidUUID},
		{name: "follows target", auth: true, setup: func(repo *Mockrepository, follows *MockFollowService) {
			repo.EXPECT().GetUserByUsername(gomock.Any(), "target").Return(databaseUserNamed(targetID, "target"), nil)
			follows.EXPECT().Follow(gomock.Any(), viewerID, targetID).Return(nil)
		}, following: true},
		{name: "unfollows target", auth: true, unfollow: true, setup: func(repo *Mockrepository, follows *MockFollowService) {
			repo.EXPECT().GetUserByUsername(gomock.Any(), "target").Return(databaseUserNamed(targetID, "target"), nil)
			follows.EXPECT().Unfollow(gomock.Any(), viewerID, targetID).Return(nil)
		}},
		{name: "propagates unfollow error", auth: true, unfollow: true, setup: func(repo *Mockrepository, follows *MockFollowService) {
			repo.EXPECT().GetUserByUsername(gomock.Any(), "target").Return(databaseUserNamed(targetID, "target"), nil)
			follows.EXPECT().Unfollow(gomock.Any(), viewerID, targetID).Return(repoErr)
		}, wantErr: repoErr},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := NewMockrepository(ctrl)
			follows := NewMockFollowService(ctrl)
			tt.setup(repo, follows)
			ctx := t.Context()
			if tt.auth {
				ctx = shared.WithUserID(ctx, viewerID)
			}
			service := New(repo, follows, NewMockPasswordManager(ctrl))
			var (
				response *models.ProfileResponse
				err      error
			)
			if tt.unfollow {
				response, err = service.UnfollowUserByUsername(ctx, "target")
			} else {
				response, err = service.FollowUserByUsername(ctx, "target")
			}
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				require.Nil(t, response)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.following, response.Profile.Following)
		})
	}
}

func TestGetProfile(t *testing.T) {
	viewerID := uuid.New()
	targetID := uuid.New()
	repoErr := errors.New("repository error")
	tests := []struct {
		name    string
		auth    bool
		setup   func(*Mockrepository, *MockFollowService)
		wantErr error
		follow  bool
	}{
		{name: "maps missing profile", setup: func(repo *Mockrepository, _ *MockFollowService) {
			repo.EXPECT().GetUserByUsername(gomock.Any(), "target").Return(postgres.User{}, pgx.ErrNoRows)
		}, wantErr: shared.ErrNotFound},
		{name: "propagates profile lookup error", setup: func(repo *Mockrepository, _ *MockFollowService) {
			repo.EXPECT().GetUserByUsername(gomock.Any(), "target").Return(postgres.User{}, repoErr)
		}, wantErr: repoErr},
		{name: "rejects invalid profile id", setup: func(repo *Mockrepository, _ *MockFollowService) {
			repo.EXPECT().GetUserByUsername(gomock.Any(), "target").Return(postgres.User{Username: "target"}, nil)
		}, wantErr: shared.ErrInvalidUUID},
		{name: "public profile skips follow lookup", setup: func(repo *Mockrepository, _ *MockFollowService) {
			repo.EXPECT().GetUserByUsername(gomock.Any(), "target").Return(databaseUserNamed(targetID, "target"), nil)
		}},
		{name: "authenticated profile", auth: true, setup: func(repo *Mockrepository, follows *MockFollowService) {
			repo.EXPECT().GetUserByUsername(gomock.Any(), "target").Return(databaseUserNamed(targetID, "target"), nil)
			follows.EXPECT().IsFollowing(gomock.Any(), viewerID, targetID).Return(true, nil)
		}, follow: true},
		{name: "propagates follow lookup error", auth: true, setup: func(repo *Mockrepository, follows *MockFollowService) {
			repo.EXPECT().GetUserByUsername(gomock.Any(), "target").Return(databaseUserNamed(targetID, "target"), nil)
			follows.EXPECT().IsFollowing(gomock.Any(), viewerID, targetID).Return(false, repoErr)
		}, wantErr: repoErr},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := NewMockrepository(ctrl)
			follows := NewMockFollowService(ctrl)
			tt.setup(repo, follows)
			ctx := t.Context()
			if tt.auth {
				ctx = shared.WithUserID(ctx, viewerID)
			}

			response, err := New(repo, follows, NewMockPasswordManager(ctrl)).GetProfileByUsername(ctx, "target")
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				require.Nil(t, response)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.follow, response.Profile.Following)
		})
	}
}

func TestProfilesByIDsUsesOneDeduplicatedBatch(t *testing.T) {
	viewerID := uuid.New()
	authorID := uuid.New()
	ctrl := gomock.NewController(t)
	repo := NewMockrepository(ctrl)
	follows := NewMockFollowService(ctrl)
	repo.EXPECT().ListProfilesByIDs(gomock.Any(), []pgtype.UUID{shared.UUIDToPG(authorID)}).Return([]postgres.ListProfilesByIDsRow{{ID: shared.UUIDToPG(authorID), Username: "author"}}, nil)
	follows.EXPECT().FollowingIDs(gomock.Any(), viewerID, []uuid.UUID{authorID}).Return(map[uuid.UUID]struct{}{authorID: {}}, nil)

	profiles, err := New(repo, follows, NewMockPasswordManager(ctrl)).ProfilesByIDs(
		shared.WithUserID(t.Context(), viewerID),
		[]uuid.UUID{authorID, authorID},
	)

	require.NoError(t, err)
	require.Len(t, profiles, 1)
	require.True(t, profiles[authorID].Following)
	require.Equal(t, "author", profiles[authorID].Username)
}

func TestIDByUsername(t *testing.T) {
	userID := uuid.New()
	repoErr := errors.New("repository error")
	tests := []struct {
		name    string
		id      pgtype.UUID
		repoErr error
		wantErr error
	}{
		{name: "returns id", id: shared.UUIDToPG(userID)},
		{name: "maps missing user", repoErr: pgx.ErrNoRows, wantErr: shared.ErrNotFound},
		{name: "propagates repository error", repoErr: repoErr, wantErr: repoErr},
		{name: "rejects invalid UUID", wantErr: shared.ErrInvalidUUID},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := NewMockrepository(gomock.NewController(t))
			repo.EXPECT().GetUserIDByUsername(gomock.Any(), "user").Return(tt.id, tt.repoErr)
			id, err := New(repo, NewMockFollowService(gomock.NewController(t)), NewMockPasswordManager(gomock.NewController(t))).IDByUsername(t.Context(), "user")
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, userID, id)
		})
	}
}

func TestProfilesByIDsErrors(t *testing.T) {
	authorID := uuid.New()
	repoErr := errors.New("repository error")
	tests := []struct {
		name    string
		setup   func(*Mockrepository, *MockFollowService)
		ctx     context.Context
		wantErr error
	}{
		{
			name: "rejects invalid returned UUID",
			ctx:  t.Context(),
			setup: func(repo *Mockrepository, _ *MockFollowService) {
				repo.EXPECT().ListProfilesByIDs(gomock.Any(), gomock.Any()).Return([]postgres.ListProfilesByIDsRow{{}}, nil)
			},
			wantErr: shared.ErrInvalidUUID,
		},
		{
			name: "propagates user batch error",
			ctx:  t.Context(),
			setup: func(repo *Mockrepository, _ *MockFollowService) {
				repo.EXPECT().ListProfilesByIDs(gomock.Any(), gomock.Any()).Return(nil, repoErr)
			},
			wantErr: repoErr,
		},
		{
			name: "propagates following batch error",
			ctx:  shared.WithUserID(t.Context(), uuid.New()),
			setup: func(repo *Mockrepository, follows *MockFollowService) {
				repo.EXPECT().ListProfilesByIDs(gomock.Any(), gomock.Any()).Return([]postgres.ListProfilesByIDsRow{{ID: shared.UUIDToPG(authorID), Username: "author"}}, nil)
				follows.EXPECT().FollowingIDs(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, repoErr)
			},
			wantErr: repoErr,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := NewMockrepository(ctrl)
			follows := NewMockFollowService(ctrl)
			tt.setup(repo, follows)
			profiles, err := New(repo, follows, NewMockPasswordManager(ctrl)).ProfilesByIDs(tt.ctx, []uuid.UUID{authorID})
			require.ErrorIs(t, err, tt.wantErr)
			require.Nil(t, profiles)
		})
	}
}

func TestProfilesByIDsEmptyInput(t *testing.T) {
	ctrl := gomock.NewController(t)
	profiles, err := New(NewMockrepository(ctrl), NewMockFollowService(ctrl), NewMockPasswordManager(ctrl)).ProfilesByIDs(t.Context(), nil)
	require.NoError(t, err)
	require.Empty(t, profiles)
}

func TestUnfollowValidationErrors(t *testing.T) {
	viewerID := uuid.New()
	repoErr := errors.New("repository error")
	tests := []struct {
		name    string
		auth    bool
		user    postgres.User
		repoErr error
		wantErr error
	}{
		{name: "requires authentication", wantErr: shared.ErrUnauthorized},
		{name: "maps missing target", auth: true, repoErr: pgx.ErrNoRows, wantErr: shared.ErrNotFound},
		{name: "propagates target lookup error", auth: true, repoErr: repoErr, wantErr: repoErr},
		{name: "rejects invalid target UUID", auth: true, user: postgres.User{Username: "target"}, wantErr: shared.ErrInvalidUUID},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := NewMockrepository(ctrl)
			if tt.auth {
				repo.EXPECT().GetUserByUsername(gomock.Any(), "target").Return(tt.user, tt.repoErr)
			}
			ctx := t.Context()
			if tt.auth {
				ctx = shared.WithUserID(ctx, viewerID)
			}
			response, err := New(repo, NewMockFollowService(ctrl), NewMockPasswordManager(ctrl)).UnfollowUserByUsername(ctx, "target")
			require.ErrorIs(t, err, tt.wantErr)
			require.Nil(t, response)
		})
	}
}

func TestToProfileRejectsInvalidUUID(t *testing.T) {
	profile, err := toProfile(&postgres.User{}, false)
	require.ErrorIs(t, err, shared.ErrInvalidUUID)
	require.Empty(t, profile)
}

func databaseUser(id uuid.UUID) postgres.User {
	return databaseUserNamed(id, "user")
}

func databaseUserNamed(id uuid.UUID, username string) postgres.User {
	return postgres.User{ID: pgtype.UUID{Bytes: id, Valid: true}, Email: "user@example.com", Username: username}
}
