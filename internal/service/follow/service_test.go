package follow

import (
	"errors"
	"testing"

	"conduit/internal/service/shared"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestFollow(t *testing.T) {
	followerID := uuid.New()
	followeeID := uuid.New()
	tests := []struct {
		name               string
		follower, followee uuid.UUID
		setup              func(*Mockrepository)
		wantErr            bool
	}{
		{name: "creates relation", follower: followerID, followee: followeeID, setup: func(repo *Mockrepository) { repo.EXPECT().FollowUser(gomock.Any(), gomock.Any()).Return(nil) }},
		{name: "rejects self follow", follower: followerID, followee: followerID, setup: func(*Mockrepository) {}, wantErr: true},
		{name: "propagates repository error", follower: followerID, followee: followeeID, setup: func(repo *Mockrepository) {
			repo.EXPECT().FollowUser(gomock.Any(), gomock.Any()).Return(errors.New("db error"))
		}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := NewMockrepository(ctrl)
			tt.setup(repo)
			err := New(repo).Follow(t.Context(), tt.follower, tt.followee)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestFollowOperations(t *testing.T) {
	followerID := uuid.New()
	followeeID := uuid.New()
	repoErr := errors.New("repository error")
	tests := []struct {
		name    string
		setup   func(*Mockrepository)
		run     func(*Service) error
		wantErr error
	}{
		{name: "unfollows", setup: func(repo *Mockrepository) { repo.EXPECT().UnfollowUser(gomock.Any(), gomock.Any()).Return(nil) }, run: func(service *Service) error { return service.Unfollow(t.Context(), followerID, followeeID) }},
		{name: "propagates unfollow error", setup: func(repo *Mockrepository) { repo.EXPECT().UnfollowUser(gomock.Any(), gomock.Any()).Return(repoErr) }, run: func(service *Service) error { return service.Unfollow(t.Context(), followerID, followeeID) }, wantErr: repoErr},
		{name: "checks following", setup: func(repo *Mockrepository) { repo.EXPECT().IsFollowing(gomock.Any(), gomock.Any()).Return(true, nil) }, run: func(service *Service) error {
			following, err := service.IsFollowing(t.Context(), followerID, followeeID)
			require.True(t, following)
			return err
		}},
		{name: "propagates following error", setup: func(repo *Mockrepository) {
			repo.EXPECT().IsFollowing(gomock.Any(), gomock.Any()).Return(false, repoErr)
		}, run: func(service *Service) error {
			_, err := service.IsFollowing(t.Context(), followerID, followeeID)
			return err
		}, wantErr: repoErr},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := NewMockrepository(ctrl)
			tt.setup(repo)
			err := tt.run(New(repo))
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestFolloweeIDs(t *testing.T) {
	followeeID := uuid.New()
	repoErr := errors.New("repository error")
	tests := []struct {
		name    string
		ids     []pgtype.UUID
		repoErr error
		want    []uuid.UUID
		wantErr error
	}{
		{name: "returns ids", ids: []pgtype.UUID{shared.UUIDToPG(followeeID)}, want: []uuid.UUID{followeeID}},
		{name: "propagates repository error", repoErr: repoErr, wantErr: repoErr},
		{name: "rejects invalid database uuid", ids: []pgtype.UUID{{Valid: false}}, wantErr: shared.ErrInvalidUUID},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := NewMockrepository(ctrl)
			repo.EXPECT().ListFolloweeIDsByFollowerID(gomock.Any(), gomock.Any()).Return(tt.ids, tt.repoErr)
			ids, err := New(repo).FolloweeIDs(t.Context(), uuid.New())
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				require.Nil(t, ids)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, ids)
		})
	}
}

func TestFollowingIDs(t *testing.T) {
	followerID := uuid.New()
	followeeID := uuid.New()
	repoErr := errors.New("repository error")
	tests := []struct {
		name    string
		ids     []pgtype.UUID
		repoErr error
		wantErr error
	}{
		{name: "returns followed candidates", ids: []pgtype.UUID{shared.UUIDToPG(followeeID)}},
		{name: "propagates repository error", repoErr: repoErr, wantErr: repoErr},
		{name: "rejects invalid uuid", ids: []pgtype.UUID{{Valid: false}}, wantErr: shared.ErrInvalidUUID},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := NewMockrepository(gomock.NewController(t))
			repo.EXPECT().ListFollowingIDs(gomock.Any(), gomock.Any()).Return(tt.ids, tt.repoErr)
			result, err := New(repo).FollowingIDs(t.Context(), followerID, []uuid.UUID{followeeID})
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			_, found := result[followeeID]
			require.True(t, found)
		})
	}
}
