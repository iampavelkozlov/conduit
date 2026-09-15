package articletag

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

func TestTagIDsByArticleIDs(t *testing.T) {
	articleID := uuid.New()
	tagID := uuid.New()
	repoErr := errors.New("repository error")
	tests := []struct {
		name    string
		rows    []postgres.ListArticleTagRelationsByArticleIDsRow
		repoErr error
		wantErr error
	}{
		{name: "groups relations", rows: []postgres.ListArticleTagRelationsByArticleIDsRow{{ArticleID: shared.UUIDToPG(articleID), TagID: shared.UUIDToPG(tagID)}}},
		{name: "propagates repository error", repoErr: repoErr, wantErr: repoErr},
		{name: "rejects invalid ids", rows: []postgres.ListArticleTagRelationsByArticleIDsRow{{}}, wantErr: shared.ErrInvalidUUID},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := NewMockrepository(gomock.NewController(t))
			repo.EXPECT().ListArticleTagRelationsByArticleIDs(gomock.Any(), gomock.Any()).Return(tt.rows, tt.repoErr)
			result, err := New(repo).TagIDsByArticleIDs(t.Context(), []uuid.UUID{articleID})
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, []uuid.UUID{tagID}, result[articleID])
		})
	}
}

func TestTagIDsEmptyBatch(t *testing.T) {
	result, err := New(NewMockrepository(gomock.NewController(t))).TagIDsByArticleIDs(t.Context(), nil)
	require.NoError(t, err)
	require.Empty(t, result)
}

func TestArticleIDs(t *testing.T) {
	tagID := uuid.New()
	articleID := uuid.New()
	repoErr := errors.New("repository error")
	tests := []struct {
		name    string
		ids     []pgtype.UUID
		repoErr error
		wantErr error
	}{
		{name: "returns article ids", ids: []pgtype.UUID{shared.UUIDToPG(articleID)}},
		{name: "propagates repository error", repoErr: repoErr, wantErr: repoErr},
		{name: "rejects invalid UUID", ids: []pgtype.UUID{{}}, wantErr: shared.ErrInvalidUUID},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := NewMockrepository(gomock.NewController(t))
			repo.EXPECT().ListArticleIDsByTagID(gomock.Any(), shared.UUIDToPG(tagID)).Return(tt.ids, tt.repoErr)
			ids, err := New(repo).ArticleIDs(t.Context(), tagID)
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

func TestTagIDsRejectsInvalidTagUUID(t *testing.T) {
	articleID := uuid.New()
	repo := NewMockrepository(gomock.NewController(t))
	repo.EXPECT().ListArticleTagRelationsByArticleIDs(gomock.Any(), gomock.Any()).Return([]postgres.ListArticleTagRelationsByArticleIDsRow{{ArticleID: shared.UUIDToPG(articleID)}}, nil)

	result, err := New(repo).TagIDsByArticleIDs(t.Context(), []uuid.UUID{articleID})
	require.ErrorIs(t, err, shared.ErrInvalidUUID)
	require.Nil(t, result)
}
