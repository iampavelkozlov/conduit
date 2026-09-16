package comment

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"conduit/internal/gen/postgres"
	"conduit/internal/models"
	"conduit/internal/service/shared"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type Service struct {
	repo   repository
	users  UserService
	logger *slog.Logger
}

func New(repo repository, users UserService, loggers ...*slog.Logger) *Service {
	return &Service{repo: repo, users: users, logger: shared.ServiceLogger(loggers...)}
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
	row, err := s.repo.CreateComment(ctx, postgres.CreateCommentParams{ID: newCommentUUID(), ArticleID: articleID, AuthorID: shared.UUIDToPG(authorID), Body: req.Comment.Body})
	if err != nil {
		s.logger.LogAttrs(ctx, slog.LevelError, "comment repo err", shared.ErrorAttrs(err, shared.UUIDAttr("article_id", articleID), slog.String("author_id", authorID.String()))...)
		return nil, err
	}
	return s.response(ctx, row.ID, row.AuthorID, row.Body, row.CreatedAt, row.UpdatedAt)
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
	commentID, err := commentUUIDFromID(id)
	if err != nil {
		return shared.NotFound("comment")
	}
	commentAuthor, err := s.repo.GetCommentAuthorIDByIDAndArticleID(ctx, postgres.GetCommentAuthorIDByIDAndArticleIDParams{
		ID: commentID, ArticleID: articleID,
	})
	if err != nil {
		mapped := mapCommentNotFound(err)
		if !errors.Is(err, pgx.ErrNoRows) {
			s.logger.LogAttrs(ctx, slog.LevelError, "comment repo err", shared.ErrorAttrs(err, shared.UUIDAttr("article_id", articleID), shared.UUIDAttr("comment_id", commentID), slog.String("author_id", authorID.String()))...)
		}
		return mapped
	}
	commentAuthorID, err := shared.PGToUUID(commentAuthor)
	if err != nil {
		s.logger.LogAttrs(ctx, slog.LevelError, "comment uuid err", shared.ErrorAttrs(err, shared.UUIDAttr("author_id", commentAuthor), shared.UUIDAttr("comment_id", commentID))...)
		return err
	}
	if commentAuthorID != authorID {
		return shared.Forbidden("comment")
	}
	n, err := s.repo.DeleteCommentByIDAndArticleIDAndAuthorID(ctx, postgres.DeleteCommentByIDAndArticleIDAndAuthorIDParams{ID: commentID, ArticleID: articleID, AuthorID: shared.UUIDToPG(authorID)})
	if err != nil {
		s.logger.LogAttrs(ctx, slog.LevelError, "comment repo err", shared.ErrorAttrs(err, shared.UUIDAttr("article_id", articleID), shared.UUIDAttr("comment_id", commentID), slog.String("author_id", authorID.String()))...)
		return err
	}
	if n == 0 {
		return shared.NotFound("comment")
	}
	return nil
}
func (s *Service) GetArticleComments(ctx context.Context, slug string) (*models.MultipleCommentsResponse, error) {
	articleID, err := s.requireArticle(ctx, slug)
	if err != nil {
		return nil, err
	}
	rows, err := s.repo.ListCommentsByArticleID(ctx, articleID)
	if err != nil {
		s.logger.LogAttrs(ctx, slog.LevelError, "comment repo err", shared.ErrorAttrs(err, shared.UUIDAttr("article_id", articleID))...)
		return nil, err
	}
	authorIDs := make([]uuid.UUID, len(rows))
	for i := range rows {
		authorID, parseErr := shared.PGToUUID(rows[i].AuthorID)
		if parseErr != nil {
			s.logger.LogAttrs(ctx, slog.LevelError, "comment uuid err", shared.ErrorAttrs(parseErr, shared.UUIDAttr("article_id", articleID), shared.UUIDAttr("author_id", rows[i].AuthorID), shared.UUIDAttr("comment_id", rows[i].ID))...)
			return nil, parseErr
		}
		authorIDs[i] = authorID
	}
	profiles, err := s.users.ProfilesByIDs(ctx, authorIDs)
	if err != nil {
		return nil, err
	}
	out := &models.MultipleCommentsResponse{Comments: make([]models.Comment, len(rows))}
	for i := range rows {
		profile, ok := profiles[authorIDs[i]]
		if !ok {
			err := fmt.Errorf("profile for comment author %s was not returned", authorIDs[i])
			s.logger.LogAttrs(ctx, slog.LevelError, "comment profile err", shared.ErrorAttrs(err, shared.UUIDAttr("article_id", articleID), slog.String("author_id", authorIDs[i].String()), shared.UUIDAttr("comment_id", rows[i].ID))...)
			return nil, err
		}
		commentID, parseErr := commentIDFromUUID(rows[i].ID)
		if parseErr != nil {
			s.logger.LogAttrs(ctx, slog.LevelError, "comment uuid err", shared.ErrorAttrs(parseErr, shared.UUIDAttr("article_id", articleID), slog.String("author_id", authorIDs[i].String()), shared.UUIDAttr("comment_id", rows[i].ID))...)
			return nil, parseErr
		}
		out.Comments[i] = models.Comment{
			Author: profile, Body: rows[i].Body, CreatedAt: rows[i].CreatedAt.Time,
			ID: commentID, UpdatedAt: rows[i].UpdatedAt.Time,
		}
	}
	return out, nil
}
func (s *Service) requireArticle(ctx context.Context, slug string) (pgtype.UUID, error) {
	articleID, err := s.repo.GetArticleIDBySlug(ctx, slug)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return pgtype.UUID{}, shared.NotFound("article")
		}
		s.logger.LogAttrs(ctx, slog.LevelError, "comment repo err", shared.ErrorAttrs(err)...)
		return pgtype.UUID{}, err
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
	profiles, err := s.users.ProfilesByIDs(ctx, []uuid.UUID{authorID})
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
