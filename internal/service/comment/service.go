package comment

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"conduit/internal/models"
	"conduit/internal/service/shared"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type Service struct {
	repo     commentRepository
	profiles ProfileReader
	articles ArticleResolver
	logger   *slog.Logger
}

func New(repo repository, users UserService, loggers ...*slog.Logger) *Service {
	return NewWithResolver(repo, users, &postgresArticleResolver{repo: repo}, loggers...)
}

func NewWithResolver(repo commentRepository, profiles ProfileReader, articles ArticleResolver, loggers ...*slog.Logger) *Service {
	return &Service{repo: repo, profiles: profiles, articles: articles, logger: shared.ServiceLogger(loggers...)}
}
func (s *Service) CreateArticleComment(ctx context.Context, slug string, req models.NewCommentRequest) (*models.SingleCommentResponse, error) {
	if req.Comment.Body == "" {
		return nil, shared.Validation("body", "can't be blank")
	}
	authorID, err := shared.UserIDFromContext(ctx)
	if err != nil {
		return nil, shared.ErrUnauthorized
	}
	articleID, err := s.requireArticle(ctx, slug)
	if err != nil {
		return nil, err
	}
	row, err := s.Create(ctx, articleID, authorID, req.Comment.Body)
	if err != nil {
		return nil, err
	}
	return s.response(
		ctx,
		shared.UUIDToPG(row.ID),
		shared.UUIDToPG(row.AuthorID),
		row.Body,
		pgtype.Timestamptz{Time: row.CreatedAt, Valid: true},
		pgtype.Timestamptz{Time: row.UpdatedAt, Valid: true},
	)
}
func (s *Service) DeleteArticleComment(ctx context.Context, slug string, id int) error {
	authorID, err := shared.UserIDFromContext(ctx)
	if err != nil {
		return shared.ErrUnauthorized
	}
	articleID, err := s.requireArticle(ctx, slug)
	if err != nil {
		return err
	}
	return s.Delete(ctx, articleID, id, authorID)
}
func (s *Service) GetArticleComments(ctx context.Context, slug string) (*models.MultipleCommentsResponse, error) {
	articleID, err := s.requireArticle(ctx, slug)
	if err != nil {
		return nil, err
	}
	rows, err := s.List(ctx, articleID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return &models.MultipleCommentsResponse{Comments: []models.Comment{}}, nil
	}
	authorIDs := make([]uuid.UUID, len(rows))
	for i := range rows {
		authorIDs[i] = rows[i].AuthorID
	}
	profiles, err := s.profiles.ProfilesByIDs(ctx, deduplicateUUIDs(authorIDs))
	if err != nil {
		return nil, err
	}
	out := &models.MultipleCommentsResponse{Comments: make([]models.Comment, len(rows))}
	for i := range rows {
		profile, ok := profiles[authorIDs[i]]
		if !ok {
			err := fmt.Errorf("profile for comment author %s was not returned", authorIDs[i])
			s.logger.LogAttrs(ctx, slog.LevelError, "comment profile err", shared.ErrorAttrs(err, slog.String("article_id", articleID.String()), slog.String("author_id", authorIDs[i].String()), slog.String("comment_id", rows[i].ID.String()))...)
			return nil, err
		}
		out.Comments[i] = models.Comment{
			Author: profile, Body: rows[i].Body, CreatedAt: rows[i].CreatedAt,
			ID: rows[i].PublicID, UpdatedAt: rows[i].UpdatedAt,
		}
	}
	return out, nil
}
func (s *Service) requireArticle(ctx context.Context, slug string) (uuid.UUID, error) {
	articleID, err := s.articles.ResolveArticleID(ctx, slug)
	if err != nil {
		return uuid.Nil, err
	}
	return articleID, nil
}

func mapCommentNotFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return shared.NotFound("comment")
	}
	return err
}
func (s *Service) response(ctx context.Context, id pgtype.UUID, authorPG pgtype.UUID, body string, createdAt, updatedAt pgtype.Timestamptz) (*models.SingleCommentResponse, error) {
	commentID, err := commentIDFromUUID(id)
	if err != nil {
		s.logger.LogAttrs(ctx, slog.LevelError, "comment uuid err", shared.ErrorAttrs(err, shared.UUIDAttr("comment_id", id), shared.UUIDAttr("author_id", authorPG))...)
		return nil, err
	}
	authorID, err := shared.PGToUUID(authorPG)
	if err != nil {
		s.logger.LogAttrs(ctx, slog.LevelError, "comment uuid err", shared.ErrorAttrs(err, shared.UUIDAttr("comment_id", id), shared.UUIDAttr("author_id", authorPG))...)
		return nil, err
	}
	profiles, err := s.profiles.ProfilesByIDs(ctx, []uuid.UUID{authorID})
	if err != nil {
		return nil, err
	}
	author, ok := profiles[authorID]
	if !ok {
		err := fmt.Errorf("profile for comment author %s was not returned", authorID)
		s.logger.LogAttrs(ctx, slog.LevelError, "comment profile err", shared.ErrorAttrs(err, shared.UUIDAttr("comment_id", id), slog.String("author_id", authorID.String()))...)
		return nil, err
	}
	return &models.SingleCommentResponse{Comment: models.Comment{Author: author, Body: body, CreatedAt: createdAt.Time, ID: commentID, UpdatedAt: updatedAt.Time}}, nil
}
