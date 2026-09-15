package comment

import (
	"errors"
	"strings"
	"testing"
	"time"

	"conduit/internal/gen/postgres"
	"conduit/internal/models"
	"conduit/internal/service/shared"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestGetArticleCommentsUsesBatchProfiles(t *testing.T) {
	authorID := uuid.New()
	now := pgtype.Timestamptz{Time: time.Now(), Valid: true}
	rows := []postgres.ListCommentsByArticleIDRow{{
		ID: commentUUIDFromUint64(7), AuthorID: shared.UUIDToPG(authorID),
		Body: "first", CreatedAt: now, UpdatedAt: now,
	}, {
		ID: commentUUIDFromUint64(8), AuthorID: shared.UUIDToPG(authorID),
		Body: "second", CreatedAt: now, UpdatedAt: now,
	}}
	repoErr := errors.New("repository error")
	tests := []struct {
		name    string
		setup   func(*Mockrepository, *MockUserService)
		wantErr error
		wantLen int
	}{
		{
			name: "loads duplicate authors in one batch",
			setup: func(repo *Mockrepository, users *MockUserService) {
				repo.EXPECT().GetArticleIDBySlug(gomock.Any(), "slug").Return(pgtype.UUID{}, nil)
				repo.EXPECT().ListCommentsByArticleID(gomock.Any(), gomock.Any()).Return(rows, nil)
				users.EXPECT().ProfilesByIDs(gomock.Any(), []uuid.UUID{authorID, authorID}).Return(map[uuid.UUID]models.Profile{authorID: {ID: authorID, Username: "author", Following: true}}, nil)
			},
			wantLen: 2,
		},
		{
			name: "propagates list error",
			setup: func(repo *Mockrepository, _ *MockUserService) {
				repo.EXPECT().GetArticleIDBySlug(gomock.Any(), "slug").Return(pgtype.UUID{}, nil)
				repo.EXPECT().ListCommentsByArticleID(gomock.Any(), gomock.Any()).Return(nil, repoErr)
			},
			wantErr: repoErr,
		},
		{
			name: "propagates batch profile error",
			setup: func(repo *Mockrepository, users *MockUserService) {
				repo.EXPECT().GetArticleIDBySlug(gomock.Any(), "slug").Return(pgtype.UUID{}, nil)
				repo.EXPECT().ListCommentsByArticleID(gomock.Any(), gomock.Any()).Return(rows, nil)
				users.EXPECT().ProfilesByIDs(gomock.Any(), gomock.Any()).Return(nil, repoErr)
			},
			wantErr: repoErr,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := NewMockrepository(ctrl)
			users := NewMockUserService(ctrl)
			tt.setup(repo, users)
			response, err := New(repo, users).GetArticleComments(t.Context(), "slug")
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				require.Nil(t, response)
				return
			}
			require.NoError(t, err)
			require.Len(t, response.Comments, tt.wantLen)
			require.Equal(t, "author", response.Comments[0].Author.Username)
			require.Equal(t, 7, response.Comments[0].ID)
			require.Equal(t, 8, response.Comments[1].ID)
		})
	}
}

func TestCreateArticleComment(t *testing.T) {
	authorID := uuid.New()
	repoErr := errors.New("repository error")
	tests := []struct {
		name    string
		body    string
		auth    bool
		setup   func(*Mockrepository, *MockUserService)
		wantErr error
	}{
		{name: "rejects blank body", auth: true, setup: func(*Mockrepository, *MockUserService) {}, wantErr: shared.ErrValidation},
		{name: "requires authentication", body: "hello", setup: func(*Mockrepository, *MockUserService) {}, wantErr: shared.ErrUnauthorized},
		{name: "maps missing article", body: "hello", auth: true, setup: func(repo *Mockrepository, _ *MockUserService) {
			repo.EXPECT().GetArticleIDBySlug(gomock.Any(), "slug").Return(pgtype.UUID{}, pgx.ErrNoRows)
		}, wantErr: shared.ErrNotFound},
		{name: "propagates create error", body: "hello", auth: true, setup: func(repo *Mockrepository, _ *MockUserService) {
			repo.EXPECT().GetArticleIDBySlug(gomock.Any(), "slug").Return(pgtype.UUID{}, nil)
			repo.EXPECT().CreateComment(gomock.Any(), gomock.Any()).Return(postgres.CreateCommentRow{}, repoErr)
		}, wantErr: repoErr},
		{name: "creates comment with batch profile", body: "hello", auth: true, setup: func(repo *Mockrepository, users *MockUserService) {
			repo.EXPECT().GetArticleIDBySlug(gomock.Any(), "slug").Return(pgtype.UUID{}, nil)
			repo.EXPECT().CreateComment(gomock.Any(), gomock.Any()).Return(commentRow(authorID), nil)
			users.EXPECT().ProfilesByIDs(gomock.Any(), []uuid.UUID{authorID}).Return(map[uuid.UUID]models.Profile{authorID: {ID: authorID, Username: "author"}}, nil)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := NewMockrepository(ctrl)
			users := NewMockUserService(ctrl)
			tt.setup(repo, users)
			ctx := t.Context()
			if tt.auth {
				ctx = shared.WithUserID(ctx, authorID)
			}
			response, err := New(repo, users).CreateArticleComment(ctx, "slug", models.NewCommentRequest{Comment: models.NewComment{Body: tt.body}})
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, "author", response.Comment.Author.Username)
			require.Equal(t, 1, response.Comment.ID)
		})
	}
}

func commentRow(authorID uuid.UUID) postgres.CreateCommentRow {
	now := pgtype.Timestamptz{Time: time.Now(), Valid: true}
	return postgres.CreateCommentRow{ID: commentUUIDFromUint64(1), AuthorID: shared.UUIDToPG(authorID), Body: "hello", CreatedAt: now, UpdatedAt: now}
}

func TestDeleteArticleComment(t *testing.T) {
	authorID := uuid.New()
	otherAuthorID := uuid.New()
	articleID := shared.NewUUID()
	commentID := commentUUIDFromUint64(7)
	repoErr := errors.New("repository error")
	tests := []struct {
		name    string
		auth    bool
		id      int
		setup   func(*Mockrepository)
		wantErr error
	}{
		{name: "requires authentication", id: 7, setup: func(*Mockrepository) {}, wantErr: shared.ErrUnauthorized},
		{name: "maps missing article", auth: true, id: 7, setup: func(repo *Mockrepository) {
			repo.EXPECT().GetArticleIDBySlug(gomock.Any(), "slug").Return(pgtype.UUID{}, pgx.ErrNoRows)
		}, wantErr: shared.ErrNotFound},
		{name: "rejects invalid public id", auth: true, setup: func(repo *Mockrepository) {
			repo.EXPECT().GetArticleIDBySlug(gomock.Any(), "slug").Return(articleID, nil)
		}, wantErr: shared.ErrNotFound},
		{name: "maps missing comment", auth: true, id: 7, setup: func(repo *Mockrepository) {
			repo.EXPECT().GetArticleIDBySlug(gomock.Any(), "slug").Return(articleID, nil)
			repo.EXPECT().GetCommentAuthorIDByIDAndArticleID(gomock.Any(), postgres.GetCommentAuthorIDByIDAndArticleIDParams{ID: commentID, ArticleID: articleID}).Return(pgtype.UUID{}, pgx.ErrNoRows)
		}, wantErr: shared.ErrNotFound},
		{name: "propagates author lookup error", auth: true, id: 7, setup: func(repo *Mockrepository) {
			repo.EXPECT().GetArticleIDBySlug(gomock.Any(), "slug").Return(articleID, nil)
			repo.EXPECT().GetCommentAuthorIDByIDAndArticleID(gomock.Any(), gomock.Any()).Return(pgtype.UUID{}, repoErr)
		}, wantErr: repoErr},
		{name: "rejects invalid author UUID", auth: true, id: 7, setup: func(repo *Mockrepository) {
			repo.EXPECT().GetArticleIDBySlug(gomock.Any(), "slug").Return(articleID, nil)
			repo.EXPECT().GetCommentAuthorIDByIDAndArticleID(gomock.Any(), gomock.Any()).Return(pgtype.UUID{}, nil)
		}, wantErr: shared.ErrInvalidUUID},
		{name: "forbids another author", auth: true, id: 7, setup: func(repo *Mockrepository) {
			repo.EXPECT().GetArticleIDBySlug(gomock.Any(), "slug").Return(articleID, nil)
			repo.EXPECT().GetCommentAuthorIDByIDAndArticleID(gomock.Any(), gomock.Any()).Return(shared.UUIDToPG(otherAuthorID), nil)
		}, wantErr: shared.ErrForbidden},
		{name: "propagates delete error", auth: true, id: 7, setup: func(repo *Mockrepository) {
			repo.EXPECT().GetArticleIDBySlug(gomock.Any(), "slug").Return(articleID, nil)
			repo.EXPECT().GetCommentAuthorIDByIDAndArticleID(gomock.Any(), gomock.Any()).Return(shared.UUIDToPG(authorID), nil)
			repo.EXPECT().DeleteCommentByIDAndArticleIDAndAuthorID(gomock.Any(), gomock.Any()).Return(int64(0), repoErr)
		}, wantErr: repoErr},
		{name: "maps concurrent deletion", auth: true, id: 7, setup: func(repo *Mockrepository) {
			repo.EXPECT().GetArticleIDBySlug(gomock.Any(), "slug").Return(articleID, nil)
			repo.EXPECT().GetCommentAuthorIDByIDAndArticleID(gomock.Any(), gomock.Any()).Return(shared.UUIDToPG(authorID), nil)
			repo.EXPECT().DeleteCommentByIDAndArticleIDAndAuthorID(gomock.Any(), gomock.Any()).Return(int64(0), nil)
		}, wantErr: shared.ErrNotFound},
		{name: "deletes owned comment", auth: true, id: 7, setup: func(repo *Mockrepository) {
			repo.EXPECT().GetArticleIDBySlug(gomock.Any(), "slug").Return(articleID, nil)
			repo.EXPECT().GetCommentAuthorIDByIDAndArticleID(gomock.Any(), gomock.Any()).Return(shared.UUIDToPG(authorID), nil)
			repo.EXPECT().DeleteCommentByIDAndArticleIDAndAuthorID(gomock.Any(), postgres.DeleteCommentByIDAndArticleIDAndAuthorIDParams{ID: commentID, ArticleID: articleID, AuthorID: shared.UUIDToPG(authorID)}).Return(int64(1), nil)
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := NewMockrepository(gomock.NewController(t))
			tt.setup(repo)
			ctx := t.Context()
			if tt.auth {
				ctx = shared.WithUserID(ctx, authorID)
			}
			err := New(repo, NewMockUserService(gomock.NewController(t))).DeleteArticleComment(ctx, "slug", tt.id)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestGetArticleCommentsMappingErrors(t *testing.T) {
	authorID := uuid.New()
	repoErr := errors.New("repository error")
	validRow := postgres.ListCommentsByArticleIDRow{ID: commentUUIDFromUint64(1), AuthorID: shared.UUIDToPG(authorID)}
	tests := []struct {
		name    string
		setup   func(*Mockrepository, *MockUserService)
		wantErr error
	}{
		{name: "maps missing article", setup: func(repo *Mockrepository, _ *MockUserService) {
			repo.EXPECT().GetArticleIDBySlug(gomock.Any(), "slug").Return(pgtype.UUID{}, pgx.ErrNoRows)
		}, wantErr: shared.ErrNotFound},
		{name: "propagates article lookup error", setup: func(repo *Mockrepository, _ *MockUserService) {
			repo.EXPECT().GetArticleIDBySlug(gomock.Any(), "slug").Return(pgtype.UUID{}, repoErr)
		}, wantErr: repoErr},
		{name: "rejects invalid author UUID", setup: func(repo *Mockrepository, _ *MockUserService) {
			repo.EXPECT().GetArticleIDBySlug(gomock.Any(), "slug").Return(shared.NewUUID(), nil)
			row := validRow
			row.AuthorID = pgtype.UUID{}
			repo.EXPECT().ListCommentsByArticleID(gomock.Any(), gomock.Any()).Return([]postgres.ListCommentsByArticleIDRow{row}, nil)
		}, wantErr: shared.ErrInvalidUUID},
		{name: "rejects missing author profile", setup: func(repo *Mockrepository, users *MockUserService) {
			repo.EXPECT().GetArticleIDBySlug(gomock.Any(), "slug").Return(shared.NewUUID(), nil)
			repo.EXPECT().ListCommentsByArticleID(gomock.Any(), gomock.Any()).Return([]postgres.ListCommentsByArticleIDRow{validRow}, nil)
			users.EXPECT().ProfilesByIDs(gomock.Any(), gomock.Any()).Return(map[uuid.UUID]models.Profile{}, nil)
		}, wantErr: errors.New("missing profile")},
		{name: "rejects invalid comment UUID", setup: func(repo *Mockrepository, users *MockUserService) {
			repo.EXPECT().GetArticleIDBySlug(gomock.Any(), "slug").Return(shared.NewUUID(), nil)
			row := validRow
			row.ID = shared.NewUUID()
			repo.EXPECT().ListCommentsByArticleID(gomock.Any(), gomock.Any()).Return([]postgres.ListCommentsByArticleIDRow{row}, nil)
			users.EXPECT().ProfilesByIDs(gomock.Any(), gomock.Any()).Return(map[uuid.UUID]models.Profile{authorID: {}}, nil)
		}, wantErr: errInvalidCommentID},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := NewMockrepository(ctrl)
			users := NewMockUserService(ctrl)
			tt.setup(repo, users)
			response, err := New(repo, users).GetArticleComments(t.Context(), "slug")
			require.Error(t, err)
			if errors.Is(tt.wantErr, shared.ErrNotFound) || errors.Is(tt.wantErr, repoErr) || errors.Is(tt.wantErr, shared.ErrInvalidUUID) || errors.Is(tt.wantErr, errInvalidCommentID) {
				require.ErrorIs(t, err, tt.wantErr)
			}
			require.Nil(t, response)
		})
	}
}

func TestCommentResponseErrors(t *testing.T) {
	authorID := uuid.New()
	now := pgtype.Timestamptz{Time: time.Now(), Valid: true}
	repoErr := errors.New("profile error")
	tests := []struct {
		name     string
		id       pgtype.UUID
		authorID pgtype.UUID
		setup    func(*MockUserService)
		wantErr  error
	}{
		{name: "invalid comment id", id: shared.NewUUID(), authorID: shared.UUIDToPG(authorID), setup: func(*MockUserService) {}, wantErr: errInvalidCommentID},
		{name: "invalid author id", id: commentUUIDFromUint64(1), setup: func(*MockUserService) {}, wantErr: shared.ErrInvalidUUID},
		{name: "profile lookup error", id: commentUUIDFromUint64(1), authorID: shared.UUIDToPG(authorID), setup: func(users *MockUserService) {
			users.EXPECT().ProfilesByIDs(gomock.Any(), []uuid.UUID{authorID}).Return(nil, repoErr)
		}, wantErr: repoErr},
		{name: "missing profile", id: commentUUIDFromUint64(1), authorID: shared.UUIDToPG(authorID), setup: func(users *MockUserService) {
			users.EXPECT().ProfilesByIDs(gomock.Any(), []uuid.UUID{authorID}).Return(map[uuid.UUID]models.Profile{}, nil)
		}, wantErr: errors.New("missing profile")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			users := NewMockUserService(gomock.NewController(t))
			tt.setup(users)
			response, err := New(nil, users).response(t.Context(), tt.id, tt.authorID, "body", now, now)
			require.Error(t, err)
			if !strings.Contains(tt.wantErr.Error(), "missing profile") {
				require.ErrorIs(t, err, tt.wantErr)
			}
			require.Nil(t, response)
		})
	}
}
