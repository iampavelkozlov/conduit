package comment

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"conduit/internal/gen/postgres"
	"conduit/internal/service/shared"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// Record is the comments service's database-owned representation. Article and
// author identifiers are references to remote aggregates, not foreign keys.
type Record struct {
	ID        uuid.UUID
	PublicID  int
	ArticleID uuid.UUID
	AuthorID  uuid.UUID
	Body      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (s *Service) Create(ctx context.Context, articleID, authorID uuid.UUID, body string) (Record, error) {
	if body == "" {
		return Record{}, shared.Validation("body", "can't be blank")
	}
	row, err := s.repo.CreateComment(ctx, postgres.CreateCommentParams{
		ID: newCommentUUID(), ArticleID: shared.UUIDToPG(articleID), AuthorID: shared.UUIDToPG(authorID), Body: body,
	})
	if err != nil {
		s.logger.LogAttrs(ctx, slog.LevelError, "comment repo err", shared.ErrorAttrs(
			err, slog.String("article_id", articleID.String()), slog.String("author_id", authorID.String()),
		)...)
		return Record{}, err
	}
	return recordFromValues(articleID, row.ID, row.AuthorID, row.Body, row.CreatedAt.Time, row.UpdatedAt.Time)
}

func (s *Service) List(ctx context.Context, articleID uuid.UUID) ([]Record, error) {
	rows, err := s.repo.ListCommentsByArticleID(ctx, shared.UUIDToPG(articleID))
	if err != nil {
		s.logger.LogAttrs(ctx, slog.LevelError, "comment repo err", shared.ErrorAttrs(err, slog.String("article_id", articleID.String()))...)
		return nil, err
	}
	records := make([]Record, len(rows))
	for i := range rows {
		records[i], err = recordFromValues(articleID, rows[i].ID, rows[i].AuthorID, rows[i].Body, rows[i].CreatedAt.Time, rows[i].UpdatedAt.Time)
		if err != nil {
			return nil, err
		}
	}
	return records, nil
}

func (s *Service) Get(ctx context.Context, articleID uuid.UUID, publicID int) (Record, error) {
	records, err := s.List(ctx, articleID)
	if err != nil {
		return Record{}, err
	}
	for _, record := range records {
		if record.PublicID == publicID {
			return record, nil
		}
	}
	return Record{}, shared.NotFound("comment")
}

func (s *Service) Delete(ctx context.Context, articleID uuid.UUID, publicID int, requesterID uuid.UUID) error {
	commentID, err := commentUUIDFromID(publicID)
	if err != nil {
		return shared.NotFound("comment")
	}
	articlePG := shared.UUIDToPG(articleID)
	commentAuthor, err := s.repo.GetCommentAuthorIDByIDAndArticleID(ctx, postgres.GetCommentAuthorIDByIDAndArticleIDParams{
		ID: commentID, ArticleID: articlePG,
	})
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			s.logger.LogAttrs(ctx, slog.LevelError, "comment repo err", shared.ErrorAttrs(
				err, slog.String("article_id", articleID.String()), shared.UUIDAttr("comment_id", commentID), slog.String("author_id", requesterID.String()),
			)...)
		}
		return mapCommentNotFound(err)
	}
	commentAuthorID, err := shared.PGToUUID(commentAuthor)
	if err != nil {
		s.logger.LogAttrs(ctx, slog.LevelError, "comment uuid err", shared.ErrorAttrs(err, shared.UUIDAttr("author_id", commentAuthor), shared.UUIDAttr("comment_id", commentID))...)
		return err
	}
	if commentAuthorID != requesterID {
		return shared.Forbidden("comment")
	}
	n, err := s.repo.DeleteCommentByIDAndArticleIDAndAuthorID(ctx, postgres.DeleteCommentByIDAndArticleIDAndAuthorIDParams{
		ID: commentID, ArticleID: articlePG, AuthorID: shared.UUIDToPG(requesterID),
	})
	if err != nil {
		s.logger.LogAttrs(ctx, slog.LevelError, "comment repo err", shared.ErrorAttrs(
			err, slog.String("article_id", articleID.String()), shared.UUIDAttr("comment_id", commentID), slog.String("author_id", requesterID.String()),
		)...)
		return err
	}
	if n == 0 {
		return shared.NotFound("comment")
	}
	return nil
}

func recordFromValues(articleID uuid.UUID, id, authorIDPG pgtype.UUID, body string, createdAt, updatedAt time.Time) (Record, error) {
	idValue, err := shared.PGToUUID(id)
	if err != nil {
		return Record{}, err
	}
	authorID, err := shared.PGToUUID(authorIDPG)
	if err != nil {
		return Record{}, err
	}
	publicID, err := commentIDFromUUID(id)
	if err != nil {
		return Record{}, err
	}
	return Record{
		ID: idValue, PublicID: publicID, ArticleID: articleID, AuthorID: authorID,
		Body: body, CreatedAt: createdAt, UpdatedAt: updatedAt,
	}, nil
}

func deduplicateUUIDs(ids []uuid.UUID) []uuid.UUID {
	result := make([]uuid.UUID, 0, len(ids))
	seen := make(map[uuid.UUID]struct{}, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}

type postgresArticleResolver struct {
	repo interface {
		GetArticleIDBySlug(context.Context, string) (pgtype.UUID, error)
	}
}

func (r *postgresArticleResolver) ResolveArticleID(ctx context.Context, slug string) (uuid.UUID, error) {
	id, err := r.repo.GetArticleIDBySlug(ctx, slug)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, shared.NotFound("article")
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("resolve article ID: %w", err)
	}
	if !id.Valid {
		// Legacy monolith tests historically use a zero pgtype.UUID as an
		// arbitrary article identifier. A database row can never contain it.
		return uuid.Nil, nil
	}
	return shared.PGToUUID(id)
}
