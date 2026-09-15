package article

import (
	"context"

	"conduit/internal/gen/postgres"
	"conduit/internal/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

//go:generate go run go.uber.org/mock/mockgen -source=interfaces.go -destination=mock_article.go -package=article
type repository interface {
	WithinTx(context.Context, func(repository) error) error
	CreateArticle(context.Context, postgres.CreateArticleParams) (pgtype.UUID, error)
	GetArticleBySlug(context.Context, string) (postgres.Article, error)
	GetArticleIDBySlug(context.Context, string) (pgtype.UUID, error)
	ListArticles(context.Context, postgres.ListArticlesParams) ([]postgres.Article, error)
	CountArticles(context.Context, postgres.CountArticlesParams) (int64, error)
	UpdateArticle(context.Context, postgres.UpdateArticleParams) (pgtype.UUID, error)
	DeleteArticleBySlugAndAuthorID(context.Context, postgres.DeleteArticleBySlugAndAuthorIDParams) (int64, error)
	UpsertTags(context.Context, postgres.UpsertTagsParams) ([]postgres.Tag, error)
	AttachTagsToArticle(context.Context, postgres.AttachTagsToArticleParams) error
	DeleteArticleTags(context.Context, pgtype.UUID) error
}

type UserService interface {
	IDByUsername(context.Context, string) (uuid.UUID, error)
	ProfilesByIDs(context.Context, []uuid.UUID) (map[uuid.UUID]models.Profile, error)
}

type TagService interface {
	IDByName(context.Context, string) (uuid.UUID, error)
	NamesByIDs(context.Context, []uuid.UUID) (map[uuid.UUID]string, error)
}

type ArticleTagService interface {
	ArticleIDs(context.Context, uuid.UUID) ([]uuid.UUID, error)
	TagIDsByArticleIDs(context.Context, []uuid.UUID) (map[uuid.UUID][]uuid.UUID, error)
}

type FavoriteService interface {
	Add(context.Context, uuid.UUID, uuid.UUID) error
	Remove(context.Context, uuid.UUID, uuid.UUID) error
	ArticleIDsForUser(context.Context, uuid.UUID) ([]uuid.UUID, error)
	CountsByArticleIDs(context.Context, []uuid.UUID) (map[uuid.UUID]int, error)
	FavoritedArticleIDs(context.Context, uuid.UUID, []uuid.UUID) (map[uuid.UUID]struct{}, error)
}

type FollowService interface {
	FolloweeIDs(context.Context, uuid.UUID) ([]uuid.UUID, error)
}
