package tag

import (
	"errors"
	"testing"

	"conduit/internal/gen/postgres"
	"conduit/internal/service/shared"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestNamesByIDs(t *testing.T) {
	tagID := uuid.New()
	repoErr := errors.New("repository error")
	tests := []struct {
		name    string
		tags    []postgres.ListTagsByIDsRow
		repoErr error
		wantErr error
	}{
		{name: "maps names by id", tags: []postgres.ListTagsByIDsRow{{ID: shared.UUIDToPG(tagID), Name: "go"}}},
		{name: "propagates repository error", repoErr: repoErr, wantErr: repoErr},
		{name: "rejects invalid id", tags: []postgres.ListTagsByIDsRow{{Name: "go"}}, wantErr: shared.ErrInvalidUUID},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := NewMockrepository(gomock.NewController(t))
			repo.EXPECT().ListTagsByIDs(gomock.Any(), gomock.Any()).Return(tt.tags, tt.repoErr)
			names, err := New(repo).NamesByIDs(t.Context(), []uuid.UUID{tagID})
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, "go", names[tagID])
		})
	}
}

func TestGetTags(t *testing.T) {
	repoErr := errors.New("repository error")
	tests := []struct {
		name    string
		tags    []string
		repoErr error
		wantErr error
	}{
		{name: "returns tags", tags: []string{"go", "testing"}},
		{name: "propagates repository error", repoErr: repoErr, wantErr: repoErr},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := NewMockrepository(gomock.NewController(t))
			repo.EXPECT().ListTags(gomock.Any()).Return(tt.tags, tt.repoErr)
			response, err := New(repo).GetTags(t.Context())
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				require.Nil(t, response)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.tags, response.Tags)
		})
	}
}

func TestIDByName(t *testing.T) {
	tagID := uuid.New()
	repoErr := errors.New("repository error")
	tests := []struct {
		name    string
		id      pgtype.UUID
		repoErr error
		wantErr error
	}{
		{name: "returns id", id: shared.UUIDToPG(tagID)},
		{name: "maps missing tag", repoErr: pgx.ErrNoRows, wantErr: shared.ErrNotFound},
		{name: "propagates repository error", repoErr: repoErr, wantErr: repoErr},
		{name: "rejects invalid UUID", wantErr: shared.ErrInvalidUUID},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := NewMockrepository(gomock.NewController(t))
			repo.EXPECT().GetTagIDByName(gomock.Any(), "go").Return(tt.id, tt.repoErr)
			id, err := New(repo).IDByName(t.Context(), "go")
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tagID, id)
		})
	}
}
