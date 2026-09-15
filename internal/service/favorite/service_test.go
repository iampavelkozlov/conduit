package favorite

import (
	"errors"
	"testing"

	"conduit/internal/gen/postgres"
	"conduit/internal/service/shared"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestBatchQueries(t *testing.T) {
	articleID := uuid.New()
	userID := uuid.New()
	repoErr := errors.New("repository error")
	tests := []struct {
		name    string
		setup   func(*Mockrepository)
		run     func(*Service) error
		wantErr error
	}{
		{
			name: "maps favorite counts",
			setup: func(repo *Mockrepository) {
				repo.EXPECT().CountFavoritesByArticleIDs(gomock.Any(), gomock.Len(1)).Return([]postgres.CountFavoritesByArticleIDsRow{{ArticleID: shared.UUIDToPG(articleID), FavoritesCount: 3}}, nil)
			},
			run: func(service *Service) error {
				counts, err := service.CountsByArticleIDs(t.Context(), []uuid.UUID{articleID})
				require.Equal(t, 3, counts[articleID])
				return err
			},
		},
		{
			name: "maps viewer favorites",
			setup: func(repo *Mockrepository) {
				repo.EXPECT().ListFavoriteArticleIDsByUserIDAndArticleIDs(gomock.Any(), gomock.Any()).Return([]pgtype.UUID{shared.UUIDToPG(articleID)}, nil)
			},
			run: func(service *Service) error {
				ids, err := service.FavoritedArticleIDs(t.Context(), userID, []uuid.UUID{articleID})
				_, found := ids[articleID]
				require.True(t, found)
				return err
			},
		},
		{
			name: "propagates batch error",
			setup: func(repo *Mockrepository) {
				repo.EXPECT().CountFavoritesByArticleIDs(gomock.Any(), gomock.Any()).Return(nil, repoErr)
			},
			run: func(service *Service) error {
				_, err := service.CountsByArticleIDs(t.Context(), []uuid.UUID{articleID})
				return err
			},
			wantErr: repoErr,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := NewMockrepository(gomock.NewController(t))
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

func TestEmptyBatchesSkipRepository(t *testing.T) {
	service := New(NewMockrepository(gomock.NewController(t)))
	counts, err := service.CountsByArticleIDs(t.Context(), nil)
	require.NoError(t, err)
	require.Empty(t, counts)
	ids, err := service.FavoritedArticleIDs(t.Context(), uuid.New(), nil)
	require.NoError(t, err)
	require.Empty(t, ids)
}

func TestMutations(t *testing.T) {
	articleID := uuid.New()
	userID := uuid.New()
	repoErr := errors.New("repository error")
	tests := []struct {
		name    string
		remove  bool
		setup   func(*Mockrepository)
		wantErr error
	}{
		{name: "adds favorite", setup: func(repo *Mockrepository) {
			repo.EXPECT().FavoriteArticle(gomock.Any(), postgres.FavoriteArticleParams{ArticleID: shared.UUIDToPG(articleID), UserID: shared.UUIDToPG(userID)}).Return(nil)
		}},
		{name: "propagates add error", setup: func(repo *Mockrepository) {
			repo.EXPECT().FavoriteArticle(gomock.Any(), gomock.Any()).Return(repoErr)
		}, wantErr: repoErr},
		{name: "removes favorite", remove: true, setup: func(repo *Mockrepository) {
			repo.EXPECT().UnfavoriteArticle(gomock.Any(), postgres.UnfavoriteArticleParams{ArticleID: shared.UUIDToPG(articleID), UserID: shared.UUIDToPG(userID)}).Return(nil)
		}},
		{name: "propagates remove error", remove: true, setup: func(repo *Mockrepository) {
			repo.EXPECT().UnfavoriteArticle(gomock.Any(), gomock.Any()).Return(repoErr)
		}, wantErr: repoErr},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := NewMockrepository(gomock.NewController(t))
			tt.setup(repo)
			service := New(repo)
			var err error
			if tt.remove {
				err = service.Remove(t.Context(), articleID, userID)
			} else {
				err = service.Add(t.Context(), articleID, userID)
			}
			require.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func TestArticleIDsForUser(t *testing.T) {
	userID := uuid.New()
	articleID := uuid.New()
	repoErr := errors.New("repository error")
	tests := []struct {
		name    string
		ids     []pgtype.UUID
		repoErr error
		wantErr error
	}{
		{name: "returns ids", ids: []pgtype.UUID{shared.UUIDToPG(articleID)}},
		{name: "propagates repository error", repoErr: repoErr, wantErr: repoErr},
		{name: "rejects invalid UUID", ids: []pgtype.UUID{{}}, wantErr: shared.ErrInvalidUUID},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := NewMockrepository(gomock.NewController(t))
			repo.EXPECT().ListFavoriteArticleIDsByUserID(gomock.Any(), shared.UUIDToPG(userID)).Return(tt.ids, tt.repoErr)
			ids, err := New(repo).ArticleIDsForUser(t.Context(), userID)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				require.Nil(t, ids)
				return
			}
			require.NoError(t, err)
			require.Equal(t, []uuid.UUID{articleID}, ids)
		})
	}
}

func TestBatchMappingErrors(t *testing.T) {
	articleID := uuid.New()
	userID := uuid.New()
	repoErr := errors.New("repository error")
	tests := []struct {
		name    string
		setup   func(*Mockrepository)
		run     func(*Service) error
		wantErr error
	}{
		{name: "invalid count UUID", setup: func(repo *Mockrepository) {
			repo.EXPECT().CountFavoritesByArticleIDs(gomock.Any(), gomock.Any()).Return([]postgres.CountFavoritesByArticleIDsRow{{ArticleID: pgtype.UUID{}, FavoritesCount: 1}}, nil)
		}, run: func(service *Service) error {
			_, err := service.CountsByArticleIDs(t.Context(), []uuid.UUID{articleID})
			return err
		}, wantErr: shared.ErrInvalidUUID},
		{name: "favorite batch repository error", setup: func(repo *Mockrepository) {
			repo.EXPECT().ListFavoriteArticleIDsByUserIDAndArticleIDs(gomock.Any(), gomock.Any()).Return(nil, repoErr)
		}, run: func(service *Service) error {
			_, err := service.FavoritedArticleIDs(t.Context(), userID, []uuid.UUID{articleID})
			return err
		}, wantErr: repoErr},
		{name: "invalid favorite UUID", setup: func(repo *Mockrepository) {
			repo.EXPECT().ListFavoriteArticleIDsByUserIDAndArticleIDs(gomock.Any(), gomock.Any()).Return([]pgtype.UUID{{}}, nil)
		}, run: func(service *Service) error {
			_, err := service.FavoritedArticleIDs(t.Context(), userID, []uuid.UUID{articleID})
			return err
		}, wantErr: shared.ErrInvalidUUID},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := NewMockrepository(gomock.NewController(t))
			tt.setup(repo)
			require.ErrorIs(t, tt.run(New(repo)), tt.wantErr)
		})
	}
}
