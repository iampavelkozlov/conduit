package article

import (
	"context"
	"errors"
	"math"
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

var errRepository = errors.New("repository failure")

type serviceMocks struct {
	repo         *Mockrepository
	transactions *Mocktransactions
	users        *MockUserService
	tags         *MockTagService
	articleTags  *MockArticleTagService
	favorites    *MockFavoriteService
	follows      *MockFollowService
}

func newServiceMocks(t *testing.T) (*Service, serviceMocks) {
	ctrl := gomock.NewController(t)
	mocks := serviceMocks{
		repo: NewMockrepository(ctrl), transactions: NewMocktransactions(ctrl), users: NewMockUserService(ctrl), tags: NewMockTagService(ctrl),
		articleTags: NewMockArticleTagService(ctrl), favorites: NewMockFavoriteService(ctrl), follows: NewMockFollowService(ctrl),
	}
	return New(mocks.repo, mocks.transactions, mocks.users, mocks.tags, mocks.articleTags, mocks.favorites, mocks.follows), mocks
}

func TestGetArticlesUsesBatchEnrichment(t *testing.T) {
	authorID := uuid.New()
	articleID := uuid.New()
	tagID := uuid.New()
	row := articleRow(articleID, authorID)
	tests := []struct {
		name    string
		setup   func(serviceMocks)
		wantErr error
		wantLen int
	}{
		{
			name: "returns count error",
			setup: func(m serviceMocks) {
				m.repo.EXPECT().CountArticles(gomock.Any(), gomock.Any()).Return(int64(0), errRepository)
			},
			wantErr: errRepository,
		},
		{
			name: "returns list error",
			setup: func(m serviceMocks) {
				m.repo.EXPECT().CountArticles(gomock.Any(), gomock.Any()).Return(int64(1), nil)
				m.repo.EXPECT().ListArticles(gomock.Any(), gomock.Any()).Return(nil, errRepository)
			},
			wantErr: errRepository,
		},
		{
			name: "returns batch profile error",
			setup: func(m serviceMocks) {
				m.repo.EXPECT().CountArticles(gomock.Any(), gomock.Any()).Return(int64(1), nil)
				m.repo.EXPECT().ListArticles(gomock.Any(), gomock.Any()).Return([]postgres.Article{row}, nil)
				m.users.EXPECT().ProfilesByIDs(gomock.Any(), []uuid.UUID{authorID}).Return(nil, errRepository)
			},
			wantErr: errRepository,
		},
		{
			name: "assembles article from bounded batches",
			setup: func(m serviceMocks) {
				m.repo.EXPECT().CountArticles(gomock.Any(), gomock.Any()).Return(int64(1), nil)
				m.repo.EXPECT().ListArticles(gomock.Any(), gomock.Any()).Return([]postgres.Article{row}, nil)
				expectEnrichment(m, articleID, authorID, tagID, false)
			},
			wantLen: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, mocks := newServiceMocks(t)
			tt.setup(mocks)
			response, err := service.GetArticles(t.Context(), models.GetArticlesParams{})
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				require.Nil(t, response)
				return
			}
			require.NoError(t, err)
			require.Len(t, response.Articles, tt.wantLen)
			require.Equal(t, []string{"go"}, response.Articles[0].TagList)
			require.Equal(t, 2, response.Articles[0].FavoritesCount)
		})
	}
}

func TestGetArticlesIntersectsResolvedFilters(t *testing.T) {
	authorID := uuid.New()
	tagID := uuid.New()
	firstID := uuid.New()
	sharedID := uuid.New()
	service, mocks := newServiceMocks(t)
	mocks.users.EXPECT().IDByUsername(gomock.Any(), "author").Return(authorID, nil)
	mocks.tags.EXPECT().IDByName(gomock.Any(), "go").Return(tagID, nil)
	mocks.articleTags.EXPECT().ArticleIDs(gomock.Any(), tagID).Return([]uuid.UUID{firstID, sharedID}, nil)
	mocks.users.EXPECT().IDByUsername(gomock.Any(), "fan").Return(uuid.New(), nil)
	mocks.favorites.EXPECT().ArticleIDsForUser(gomock.Any(), gomock.Any()).Return([]uuid.UUID{sharedID}, nil)
	mocks.repo.EXPECT().CountArticles(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, params postgres.CountArticlesParams) (int64, error) {
		require.True(t, params.FilterAuthorIds)
		require.True(t, params.FilterArticleIds)
		require.Equal(t, []pgtype.UUID{shared.UUIDToPG(sharedID)}, params.ArticleIds)
		return 0, nil
	})

	response, err := service.GetArticles(t.Context(), models.GetArticlesParams{
		Author: new("author"), Tag: new("go"), Favorited: new("fan"),
	})
	require.NoError(t, err)
	require.Empty(t, response.Articles)
}

func TestGetArticlesMissingFilterValueReturnsEmpty(t *testing.T) {
	service, mocks := newServiceMocks(t)
	mocks.users.EXPECT().IDByUsername(gomock.Any(), "missing").Return(uuid.Nil, shared.ErrNotFound)
	response, err := service.GetArticles(t.Context(), models.GetArticlesParams{Author: new("missing")})
	require.NoError(t, err)
	require.Empty(t, response.Articles)
}

func TestGetArticlesFeed(t *testing.T) {
	viewerID := uuid.New()
	tests := []struct {
		name    string
		ctx     context.Context
		setup   func(serviceMocks)
		wantErr error
	}{
		{name: "requires authentication", ctx: t.Context(), setup: func(serviceMocks) {}, wantErr: shared.ErrUnauthorized},
		{name: "skips article queries for empty feed", ctx: shared.WithUserID(t.Context(), viewerID), setup: func(m serviceMocks) {
			m.follows.EXPECT().FolloweeIDs(gomock.Any(), viewerID).Return([]uuid.UUID{}, nil)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, mocks := newServiceMocks(t)
			tt.setup(mocks)
			response, err := service.GetArticlesFeed(tt.ctx, models.GetArticlesFeedParams{})
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Empty(t, response.Articles)
		})
	}
}

func TestCreateArticleBatchesAndDeduplicatesTags(t *testing.T) {
	userID := uuid.New()
	articleID := uuid.New()
	tagID := uuid.New()
	service, mocks := newServiceMocks(t)
	expectTransaction(mocks.transactions, mocks.repo)
	mocks.repo.EXPECT().CreateArticle(gomock.Any(), gomock.Any()).Return(shared.UUIDToPG(articleID), nil)
	mocks.repo.EXPECT().UpsertTags(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, params postgres.UpsertTagsParams) ([]postgres.Tag, error) {
		require.Equal(t, []string{"go"}, params.Names)
		return []postgres.Tag{{ID: shared.UUIDToPG(tagID), Name: "go"}}, nil
	})
	mocks.repo.EXPECT().AttachTagsToArticle(gomock.Any(), postgres.AttachTagsToArticleParams{ArticleID: shared.UUIDToPG(articleID), TagIds: []pgtype.UUID{shared.UUIDToPG(tagID)}}).Return(nil)
	mocks.repo.EXPECT().GetArticleBySlug(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, slug string) (postgres.Article, error) {
		row := articleRow(articleID, userID)
		row.Slug = slug
		return row, nil
	})
	expectEnrichment(mocks, articleID, userID, tagID, true)

	tags := []string{"go", "go"}
	response, err := service.CreateArticle(shared.WithUserID(t.Context(), userID), models.NewArticleRequest{Article: models.NewArticle{
		Title: "Title", Description: "description", Body: "body", TagList: &tags,
	}})
	require.NoError(t, err)
	require.Contains(t, response.Article.Slug, "title-")
}

func TestGetArticleMapsMissingRow(t *testing.T) {
	service, mocks := newServiceMocks(t)
	mocks.repo.EXPECT().GetArticleBySlug(gomock.Any(), "missing").Return(postgres.Article{}, pgx.ErrNoRows)
	response, err := service.GetArticle(t.Context(), "missing")
	require.ErrorIs(t, err, shared.ErrNotFound)
	require.Nil(t, response)
}

func TestEnrichFailureStages(t *testing.T) {
	articleID := uuid.New()
	authorID := uuid.New()
	tagID := uuid.New()
	validRow := articleRow(articleID, authorID)

	tests := []struct {
		name        string
		ctx         func(*testing.T) context.Context
		rows        []postgres.Article
		setup       func(serviceMocks)
		wantErr     error
		wantMessage string
	}{
		{
			name:  "empty input avoids dependencies",
			ctx:   func(t *testing.T) context.Context { return t.Context() },
			rows:  nil,
			setup: func(serviceMocks) {},
		},
		{
			name:  "rejects invalid article id",
			ctx:   func(t *testing.T) context.Context { return t.Context() },
			rows:  []postgres.Article{{AuthorID: shared.UUIDToPG(authorID)}},
			setup: func(serviceMocks) {}, wantErr: shared.ErrInvalidUUID,
		},
		{
			name:  "rejects invalid author id",
			ctx:   func(t *testing.T) context.Context { return t.Context() },
			rows:  []postgres.Article{{ID: shared.UUIDToPG(articleID)}},
			setup: func(serviceMocks) {}, wantErr: shared.ErrInvalidUUID,
		},
		{
			name: "propagates article tag batch error",
			ctx:  func(t *testing.T) context.Context { return t.Context() },
			rows: []postgres.Article{validRow},
			setup: func(m serviceMocks) {
				m.users.EXPECT().ProfilesByIDs(gomock.Any(), []uuid.UUID{authorID}).Return(map[uuid.UUID]models.Profile{authorID: {}}, nil)
				m.articleTags.EXPECT().TagIDsByArticleIDs(gomock.Any(), []uuid.UUID{articleID}).Return(nil, errRepository)
			},
			wantErr: errRepository,
		},
		{
			name: "propagates tag name batch error",
			ctx:  func(t *testing.T) context.Context { return t.Context() },
			rows: []postgres.Article{validRow},
			setup: func(m serviceMocks) {
				m.users.EXPECT().ProfilesByIDs(gomock.Any(), []uuid.UUID{authorID}).Return(map[uuid.UUID]models.Profile{authorID: {}}, nil)
				m.articleTags.EXPECT().TagIDsByArticleIDs(gomock.Any(), []uuid.UUID{articleID}).Return(map[uuid.UUID][]uuid.UUID{articleID: {tagID}}, nil)
				m.tags.EXPECT().NamesByIDs(gomock.Any(), []uuid.UUID{tagID}).Return(nil, errRepository)
			},
			wantErr: errRepository,
		},
		{
			name: "propagates favorite count batch error",
			ctx:  func(t *testing.T) context.Context { return t.Context() },
			rows: []postgres.Article{validRow},
			setup: func(m serviceMocks) {
				m.users.EXPECT().ProfilesByIDs(gomock.Any(), []uuid.UUID{authorID}).Return(map[uuid.UUID]models.Profile{authorID: {}}, nil)
				m.articleTags.EXPECT().TagIDsByArticleIDs(gomock.Any(), []uuid.UUID{articleID}).Return(map[uuid.UUID][]uuid.UUID{}, nil)
				m.tags.EXPECT().NamesByIDs(gomock.Any(), []uuid.UUID{}).Return(map[uuid.UUID]string{}, nil)
				m.favorites.EXPECT().CountsByArticleIDs(gomock.Any(), []uuid.UUID{articleID}).Return(nil, errRepository)
			},
			wantErr: errRepository,
		},
		{
			name: "propagates viewer favorites batch error",
			ctx:  func(t *testing.T) context.Context { return shared.WithUserID(t.Context(), uuid.New()) },
			rows: []postgres.Article{validRow},
			setup: func(m serviceMocks) {
				m.users.EXPECT().ProfilesByIDs(gomock.Any(), []uuid.UUID{authorID}).Return(map[uuid.UUID]models.Profile{authorID: {}}, nil)
				m.articleTags.EXPECT().TagIDsByArticleIDs(gomock.Any(), []uuid.UUID{articleID}).Return(map[uuid.UUID][]uuid.UUID{}, nil)
				m.tags.EXPECT().NamesByIDs(gomock.Any(), []uuid.UUID{}).Return(map[uuid.UUID]string{}, nil)
				m.favorites.EXPECT().CountsByArticleIDs(gomock.Any(), []uuid.UUID{articleID}).Return(map[uuid.UUID]int{}, nil)
				m.favorites.EXPECT().FavoritedArticleIDs(gomock.Any(), gomock.Any(), []uuid.UUID{articleID}).Return(nil, errRepository)
			},
			wantErr: errRepository,
		},
		{
			name: "rejects missing author profile",
			ctx:  func(t *testing.T) context.Context { return t.Context() },
			rows: []postgres.Article{validRow},
			setup: func(m serviceMocks) {
				m.users.EXPECT().ProfilesByIDs(gomock.Any(), []uuid.UUID{authorID}).Return(map[uuid.UUID]models.Profile{}, nil)
				m.articleTags.EXPECT().TagIDsByArticleIDs(gomock.Any(), []uuid.UUID{articleID}).Return(map[uuid.UUID][]uuid.UUID{}, nil)
				m.tags.EXPECT().NamesByIDs(gomock.Any(), []uuid.UUID{}).Return(map[uuid.UUID]string{}, nil)
				m.favorites.EXPECT().CountsByArticleIDs(gomock.Any(), []uuid.UUID{articleID}).Return(map[uuid.UUID]int{}, nil)
			},
			wantMessage: "profile for article author",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, mocks := newServiceMocks(t)
			tt.setup(mocks)
			articles, err := service.enrich(tt.ctx(t), tt.rows)
			switch {
			case tt.wantErr != nil:
				require.ErrorIs(t, err, tt.wantErr)
				require.Nil(t, articles)
			case tt.wantMessage != "":
				require.ErrorContains(t, err, tt.wantMessage)
				require.Nil(t, articles)
			default:
				require.NoError(t, err)
				require.Empty(t, articles)
			}
		})
	}
}

func expectTransaction(transactions *Mocktransactions, repo *Mockrepository) {
	transactions.EXPECT().WithTx(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, operation func(postgres.Querier) error) error {
		return operation(articleQuerierAdapter{repository: repo})
	})
}

type articleQuerierAdapter struct {
	postgres.Querier
	repository repository
}

//nolint:gocritic // The generated postgres.Querier contract passes these parameters by value.
func (a articleQuerierAdapter) CreateArticle(ctx context.Context, params postgres.CreateArticleParams) (pgtype.UUID, error) {
	return a.repository.CreateArticle(ctx, params)
}

func (a articleQuerierAdapter) GetArticleBySlug(ctx context.Context, slug string) (postgres.Article, error) {
	return a.repository.GetArticleBySlug(ctx, slug)
}

func (a articleQuerierAdapter) GetArticleIDBySlug(ctx context.Context, slug string) (pgtype.UUID, error) {
	return a.repository.GetArticleIDBySlug(ctx, slug)
}

func (a articleQuerierAdapter) ListArticles(ctx context.Context, params postgres.ListArticlesParams) ([]postgres.Article, error) {
	return a.repository.ListArticles(ctx, params)
}

func (a articleQuerierAdapter) CountArticles(ctx context.Context, params postgres.CountArticlesParams) (int64, error) {
	return a.repository.CountArticles(ctx, params)
}

func (a articleQuerierAdapter) UpdateArticle(ctx context.Context, params postgres.UpdateArticleParams) (pgtype.UUID, error) {
	return a.repository.UpdateArticle(ctx, params)
}

func (a articleQuerierAdapter) DeleteArticleBySlugAndAuthorID(ctx context.Context, params postgres.DeleteArticleBySlugAndAuthorIDParams) (int64, error) {
	return a.repository.DeleteArticleBySlugAndAuthorID(ctx, params)
}

func (a articleQuerierAdapter) UpsertTags(ctx context.Context, params postgres.UpsertTagsParams) ([]postgres.Tag, error) {
	return a.repository.UpsertTags(ctx, params)
}

func (a articleQuerierAdapter) AttachTagsToArticle(ctx context.Context, params postgres.AttachTagsToArticleParams) error {
	return a.repository.AttachTagsToArticle(ctx, params)
}

func (a articleQuerierAdapter) DeleteArticleTags(ctx context.Context, articleID pgtype.UUID) error {
	return a.repository.DeleteArticleTags(ctx, articleID)
}

func expectEnrichment(m serviceMocks, articleID, authorID, tagID uuid.UUID, authenticated bool) {
	m.users.EXPECT().ProfilesByIDs(gomock.Any(), []uuid.UUID{authorID}).Return(map[uuid.UUID]models.Profile{authorID: {ID: authorID, Username: "author"}}, nil)
	m.articleTags.EXPECT().TagIDsByArticleIDs(gomock.Any(), []uuid.UUID{articleID}).Return(map[uuid.UUID][]uuid.UUID{articleID: {tagID}}, nil)
	m.tags.EXPECT().NamesByIDs(gomock.Any(), []uuid.UUID{tagID}).Return(map[uuid.UUID]string{tagID: "go"}, nil)
	m.favorites.EXPECT().CountsByArticleIDs(gomock.Any(), []uuid.UUID{articleID}).Return(map[uuid.UUID]int{articleID: 2}, nil)
	if authenticated {
		m.favorites.EXPECT().FavoritedArticleIDs(gomock.Any(), gomock.Any(), []uuid.UUID{articleID}).Return(map[uuid.UUID]struct{}{articleID: {}}, nil)
	}
}

func articleRow(articleID, authorID uuid.UUID) postgres.Article {
	now := pgtype.Timestamptz{Time: time.Now(), Valid: true}
	return postgres.Article{
		ID: shared.UUIDToPG(articleID), AuthorID: shared.UUIDToPG(authorID), Slug: "title",
		Title: "title", Description: "description", Body: "body", CreatedAt: now, UpdatedAt: now,
	}
}

func TestMapArticleNotFound(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want error
	}{{name: "maps missing row", err: pgx.ErrNoRows, want: shared.ErrNotFound}, {name: "preserves repository error", err: errRepository, want: errRepository}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.ErrorIs(t, mapArticleNotFound(tt.err), tt.want)
		})
	}
}

func TestDeleteArticle(t *testing.T) {
	ownerID := uuid.New()
	otherID := uuid.New()
	articleID := uuid.New()
	tests := []struct {
		name    string
		auth    bool
		setup   func(serviceMocks)
		wantErr error
	}{
		{name: "requires authentication", setup: func(serviceMocks) {}, wantErr: shared.ErrUnauthorized},
		{name: "maps missing article", auth: true, setup: func(m serviceMocks) {
			m.repo.EXPECT().GetArticleBySlug(gomock.Any(), "slug").Return(postgres.Article{}, pgx.ErrNoRows)
		}, wantErr: shared.ErrNotFound},
		{name: "propagates owner lookup error", auth: true, setup: func(m serviceMocks) {
			m.repo.EXPECT().GetArticleBySlug(gomock.Any(), "slug").Return(postgres.Article{}, errRepository)
		}, wantErr: errRepository},
		{name: "rejects invalid author UUID", auth: true, setup: func(m serviceMocks) {
			row := articleRow(articleID, ownerID)
			row.AuthorID = pgtype.UUID{}
			m.repo.EXPECT().GetArticleBySlug(gomock.Any(), "slug").Return(row, nil)
		}, wantErr: shared.ErrInvalidUUID},
		{name: "forbids non-owner", auth: true, setup: func(m serviceMocks) {
			m.repo.EXPECT().GetArticleBySlug(gomock.Any(), "slug").Return(articleRow(articleID, otherID), nil)
		}, wantErr: shared.ErrForbidden},
		{name: "propagates delete error", auth: true, setup: func(m serviceMocks) {
			row := articleRow(articleID, ownerID)
			m.repo.EXPECT().GetArticleBySlug(gomock.Any(), "slug").Return(row, nil)
			m.repo.EXPECT().DeleteArticleBySlugAndAuthorID(gomock.Any(), postgres.DeleteArticleBySlugAndAuthorIDParams{Slug: "slug", AuthorID: row.AuthorID}).Return(int64(0), errRepository)
		}, wantErr: errRepository},
		{name: "maps concurrent deletion", auth: true, setup: func(m serviceMocks) {
			row := articleRow(articleID, ownerID)
			m.repo.EXPECT().GetArticleBySlug(gomock.Any(), "slug").Return(row, nil)
			m.repo.EXPECT().DeleteArticleBySlugAndAuthorID(gomock.Any(), gomock.Any()).Return(int64(0), nil)
		}, wantErr: shared.ErrNotFound},
		{name: "deletes owned article", auth: true, setup: func(m serviceMocks) {
			row := articleRow(articleID, ownerID)
			m.repo.EXPECT().GetArticleBySlug(gomock.Any(), "slug").Return(row, nil)
			m.repo.EXPECT().DeleteArticleBySlugAndAuthorID(gomock.Any(), gomock.Any()).Return(int64(1), nil)
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, mocks := newServiceMocks(t)
			tt.setup(mocks)
			ctx := t.Context()
			if tt.auth {
				ctx = shared.WithUserID(ctx, ownerID)
			}
			err := service.DeleteArticle(ctx, "slug")
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestChangeFavorite(t *testing.T) {
	viewerID := uuid.New()
	articleID := uuid.New()
	authorID := uuid.New()
	tagID := uuid.New()
	tests := []struct {
		name    string
		auth    bool
		remove  bool
		setup   func(serviceMocks)
		wantErr error
	}{
		{name: "requires authentication", setup: func(serviceMocks) {}, wantErr: shared.ErrUnauthorized},
		{name: "maps missing article", auth: true, setup: func(m serviceMocks) {
			m.repo.EXPECT().GetArticleIDBySlug(gomock.Any(), "slug").Return(pgtype.UUID{}, pgx.ErrNoRows)
		}, wantErr: shared.ErrNotFound},
		{name: "propagates lookup error", auth: true, setup: func(m serviceMocks) {
			m.repo.EXPECT().GetArticleIDBySlug(gomock.Any(), "slug").Return(pgtype.UUID{}, errRepository)
		}, wantErr: errRepository},
		{name: "rejects invalid article UUID", auth: true, setup: func(m serviceMocks) {
			m.repo.EXPECT().GetArticleIDBySlug(gomock.Any(), "slug").Return(pgtype.UUID{}, nil)
		}, wantErr: shared.ErrInvalidUUID},
		{name: "propagates favorite error", auth: true, setup: func(m serviceMocks) {
			m.repo.EXPECT().GetArticleIDBySlug(gomock.Any(), "slug").Return(shared.UUIDToPG(articleID), nil)
			m.favorites.EXPECT().Add(gomock.Any(), articleID, viewerID).Return(errRepository)
		}, wantErr: errRepository},
		{name: "removes and returns article", auth: true, remove: true, setup: func(m serviceMocks) {
			m.repo.EXPECT().GetArticleIDBySlug(gomock.Any(), "slug").Return(shared.UUIDToPG(articleID), nil)
			m.favorites.EXPECT().Remove(gomock.Any(), articleID, viewerID).Return(nil)
			m.repo.EXPECT().GetArticleBySlug(gomock.Any(), "slug").Return(articleRow(articleID, authorID), nil)
			expectEnrichment(m, articleID, authorID, tagID, true)
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, mocks := newServiceMocks(t)
			tt.setup(mocks)
			ctx := t.Context()
			if tt.auth {
				ctx = shared.WithUserID(ctx, viewerID)
			}
			var (
				response *models.SingleArticleResponse
				err      error
			)
			if tt.remove {
				response, err = service.DeleteArticleFavorite(ctx, "slug")
			} else {
				response, err = service.CreateArticleFavorite(ctx, "slug")
			}
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				require.Nil(t, response)
				return
			}
			require.NoError(t, err)
			require.Equal(t, "title", response.Article.Title)
		})
	}
}

func TestUpdateArticle(t *testing.T) {
	ownerID := uuid.New()
	articleID := uuid.New()
	tagID := uuid.New()
	newTitle := "updated"
	empty := ""
	tags := []string{"go"}
	tests := []struct {
		name    string
		auth    bool
		request models.UpdateArticleRequest
		setup   func(serviceMocks)
		wantErr error
	}{
		{name: "rejects empty title", request: models.UpdateArticleRequest{Article: models.UpdateArticle{TitleSet: true, Title: &empty}}, setup: func(serviceMocks) {}, wantErr: shared.ErrValidation},
		{name: "rejects null tag list", request: models.UpdateArticleRequest{Article: models.UpdateArticle{TagListSet: true}}, setup: func(serviceMocks) {}, wantErr: shared.ErrValidation},
		{name: "requires authentication", setup: func(serviceMocks) {}, wantErr: shared.ErrUnauthorized},
		{name: "propagates transaction error", auth: true, setup: func(m serviceMocks) {
			m.transactions.EXPECT().WithTx(gomock.Any(), gomock.Any()).Return(errRepository)
		}, wantErr: errRepository},
		{name: "propagates owner lookup error", auth: true, setup: func(m serviceMocks) {
			expectTransaction(m.transactions, m.repo)
			m.repo.EXPECT().GetArticleBySlug(gomock.Any(), "slug").Return(postgres.Article{}, errRepository)
		}, wantErr: errRepository},
		{name: "propagates update error", auth: true, setup: func(m serviceMocks) {
			expectTransaction(m.transactions, m.repo)
			current := articleRow(articleID, ownerID)
			m.repo.EXPECT().GetArticleBySlug(gomock.Any(), "slug").Return(current, nil)
			m.repo.EXPECT().UpdateArticle(gomock.Any(), gomock.Any()).Return(pgtype.UUID{}, errRepository)
		}, wantErr: errRepository},
		{name: "propagates tag deletion error", auth: true, request: models.UpdateArticleRequest{Article: models.UpdateArticle{TagListSet: true, TagList: &tags}}, setup: func(m serviceMocks) {
			expectTransaction(m.transactions, m.repo)
			current := articleRow(articleID, ownerID)
			m.repo.EXPECT().GetArticleBySlug(gomock.Any(), "slug").Return(current, nil)
			m.repo.EXPECT().UpdateArticle(gomock.Any(), gomock.Any()).Return(shared.UUIDToPG(articleID), nil)
			m.repo.EXPECT().DeleteArticleTags(gomock.Any(), shared.UUIDToPG(articleID)).Return(errRepository)
		}, wantErr: errRepository},
		{name: "updates fields without replacing tags", auth: true, request: models.UpdateArticleRequest{Article: models.UpdateArticle{TitleSet: true, Title: &newTitle}}, setup: func(m serviceMocks) {
			expectTransaction(m.transactions, m.repo)
			current := articleRow(articleID, ownerID)
			m.repo.EXPECT().GetArticleBySlug(gomock.Any(), "slug").Return(current, nil)
			m.repo.EXPECT().UpdateArticle(gomock.Any(), postgres.UpdateArticleParams{Slug: "slug", Title: newTitle, Description: current.Description, Body: current.Body}).Return(shared.UUIDToPG(articleID), nil)
			updated := current
			updated.Title = newTitle
			m.repo.EXPECT().GetArticleBySlug(gomock.Any(), "slug").Return(updated, nil)
			expectEnrichment(m, articleID, ownerID, tagID, true)
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, mocks := newServiceMocks(t)
			tt.setup(mocks)
			ctx := t.Context()
			if tt.auth {
				ctx = shared.WithUserID(ctx, ownerID)
			}
			response, err := service.UpdateArticle(ctx, "slug", tt.request)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				require.Nil(t, response)
				return
			}
			require.NoError(t, err)
			require.Equal(t, newTitle, response.Article.Title)
		})
	}
}

func TestCreateArticleErrors(t *testing.T) {
	userID := uuid.New()
	articleID := uuid.New()
	tagID := uuid.New()
	tags := []string{"go"}
	request := models.NewArticleRequest{Article: models.NewArticle{Title: "title", Description: "description", Body: "body", TagList: &tags}}
	tests := []struct {
		name    string
		auth    bool
		setup   func(serviceMocks)
		wantErr error
	}{
		{name: "requires authentication", setup: func(serviceMocks) {}, wantErr: shared.ErrUnauthorized},
		{name: "propagates transaction error", auth: true, setup: func(m serviceMocks) {
			m.transactions.EXPECT().WithTx(gomock.Any(), gomock.Any()).Return(errRepository)
		}, wantErr: errRepository},
		{name: "propagates create error", auth: true, setup: func(m serviceMocks) {
			expectTransaction(m.transactions, m.repo)
			m.repo.EXPECT().CreateArticle(gomock.Any(), gomock.Any()).Return(pgtype.UUID{}, errRepository)
		}, wantErr: errRepository},
		{name: "propagates tag upsert error", auth: true, setup: func(m serviceMocks) {
			expectTransaction(m.transactions, m.repo)
			m.repo.EXPECT().CreateArticle(gomock.Any(), gomock.Any()).Return(shared.UUIDToPG(articleID), nil)
			m.repo.EXPECT().UpsertTags(gomock.Any(), gomock.Any()).Return(nil, errRepository)
		}, wantErr: errRepository},
		{name: "propagates tag attach error", auth: true, setup: func(m serviceMocks) {
			expectTransaction(m.transactions, m.repo)
			m.repo.EXPECT().CreateArticle(gomock.Any(), gomock.Any()).Return(shared.UUIDToPG(articleID), nil)
			m.repo.EXPECT().UpsertTags(gomock.Any(), gomock.Any()).Return([]postgres.Tag{{ID: shared.UUIDToPG(tagID)}}, nil)
			m.repo.EXPECT().AttachTagsToArticle(gomock.Any(), gomock.Any()).Return(errRepository)
		}, wantErr: errRepository},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, mocks := newServiceMocks(t)
			tt.setup(mocks)
			ctx := t.Context()
			if tt.auth {
				ctx = shared.WithUserID(ctx, userID)
			}
			response, err := service.CreateArticle(ctx, request)
			require.ErrorIs(t, err, tt.wantErr)
			require.Nil(t, response)
		})
	}
}

func TestCreateArticleRejectsInvalidRequest(t *testing.T) {
	service, _ := newServiceMocks(t)
	response, err := service.CreateArticle(t.Context(), models.NewArticleRequest{})
	require.ErrorIs(t, err, shared.ErrValidation)
	require.Nil(t, response)
}

func TestArticleValidationAndPagination(t *testing.T) {
	valid := models.NewArticle{Title: "title", Description: "description", Body: "body"}
	createTests := []struct {
		name    string
		article models.NewArticle
		field   string
	}{
		{name: "valid", article: valid},
		{name: "missing title", article: models.NewArticle{Description: "description", Body: "body"}, field: "title"},
		{name: "missing description", article: models.NewArticle{Title: "title", Body: "body"}, field: "description"},
		{name: "missing body", article: models.NewArticle{Title: "title", Description: "description"}, field: "body"},
	}
	for _, tt := range createTests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCreate(tt.article)
			if tt.field == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, shared.ErrValidation)
			require.Contains(t, err.Error(), tt.field)
		})
	}

	negative := -1
	tooLarge := 101
	limit := 25
	require.Equal(t, int32(20), pageLimit(nil))
	require.Equal(t, int32(20), pageLimit(&negative))
	require.Equal(t, int32(20), pageLimit(&tooLarge))
	require.Equal(t, int32(25), pageLimit(&limit))
	require.Equal(t, int32(0), pageOffset(nil))
	require.Equal(t, int32(0), pageOffset(&negative))
	require.Equal(t, int32(25), pageOffset(&limit))
	huge := int(math.MaxInt32) + 1
	require.Equal(t, int32(math.MaxInt32), pageOffset(&huge))
	require.Equal(t, "fallback", valueOr(nil, "fallback"))
	require.Equal(t, "value", valueOr(new("value"), "fallback"))
}

func TestGetArticlesFilterErrors(t *testing.T) {
	author := "author"
	tag := "go"
	fan := "fan"
	tagID := uuid.New()
	userID := uuid.New()
	tests := []struct {
		name   string
		params models.GetArticlesParams
		setup  func(serviceMocks)
	}{
		{name: "author lookup", params: models.GetArticlesParams{Author: &author}, setup: func(m serviceMocks) {
			m.users.EXPECT().IDByUsername(gomock.Any(), author).Return(uuid.Nil, errRepository)
		}},
		{name: "tag lookup", params: models.GetArticlesParams{Tag: &tag}, setup: func(m serviceMocks) {
			m.tags.EXPECT().IDByName(gomock.Any(), tag).Return(uuid.Nil, errRepository)
		}},
		{name: "article tag lookup", params: models.GetArticlesParams{Tag: &tag}, setup: func(m serviceMocks) {
			m.tags.EXPECT().IDByName(gomock.Any(), tag).Return(tagID, nil)
			m.articleTags.EXPECT().ArticleIDs(gomock.Any(), tagID).Return(nil, errRepository)
		}},
		{name: "favorited user lookup", params: models.GetArticlesParams{Favorited: &fan}, setup: func(m serviceMocks) {
			m.users.EXPECT().IDByUsername(gomock.Any(), fan).Return(uuid.Nil, errRepository)
		}},
		{name: "favorite lookup", params: models.GetArticlesParams{Favorited: &fan}, setup: func(m serviceMocks) {
			m.users.EXPECT().IDByUsername(gomock.Any(), fan).Return(userID, nil)
			m.favorites.EXPECT().ArticleIDsForUser(gomock.Any(), userID).Return(nil, errRepository)
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, mocks := newServiceMocks(t)
			tt.setup(mocks)
			response, err := service.GetArticles(t.Context(), tt.params)
			require.ErrorIs(t, err, errRepository)
			require.Nil(t, response)
		})
	}
}

func TestGetArticlesFeedDependencyError(t *testing.T) {
	viewerID := uuid.New()
	service, mocks := newServiceMocks(t)
	mocks.follows.EXPECT().FolloweeIDs(gomock.Any(), viewerID).Return(nil, errRepository)

	response, err := service.GetArticlesFeed(shared.WithUserID(t.Context(), viewerID), models.GetArticlesFeedParams{})
	require.ErrorIs(t, err, errRepository)
	require.Nil(t, response)
}
