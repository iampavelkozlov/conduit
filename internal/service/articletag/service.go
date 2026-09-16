package articletag

import (
	"context"
	"log/slog"

	"conduit/internal/service/shared"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// Service owns read operations for the article_tags relation table.
type Service struct {
	repo   repository
	logger *slog.Logger
}

func New(repo repository, loggers ...*slog.Logger) *Service {
	return &Service{repo: repo, logger: shared.ServiceLogger(loggers...)}
}

func (s *Service) ArticleIDs(ctx context.Context, tagID uuid.UUID) ([]uuid.UUID, error) {
	ids, err := s.repo.ListArticleIDsByTagID(ctx, shared.UUIDToPG(tagID))
	if err != nil {
		s.logger.LogAttrs(ctx, slog.LevelError, "article tag repo err", shared.ErrorAttrs(err, slog.String("tag_id", tagID.String()))...)
		return nil, err
	}
	parsed, err := parseUUIDs(ids)
	if err != nil {
		s.logger.LogAttrs(ctx, slog.LevelError, "article tag uuid err", shared.ErrorAttrs(err, slog.String("tag_id", tagID.String()))...)
	}
	return parsed, err
}

func (s *Service) TagIDsByArticleIDs(ctx context.Context, articleIDs []uuid.UUID) (map[uuid.UUID][]uuid.UUID, error) {
	result := make(map[uuid.UUID][]uuid.UUID, len(articleIDs))
	if len(articleIDs) == 0 {
		return result, nil
	}
	rows, err := s.repo.ListArticleTagRelationsByArticleIDs(ctx, toPGUUIDs(articleIDs))
	if err != nil {
		s.logger.LogAttrs(ctx, slog.LevelError, "article tag repo err", shared.ErrorAttrs(err, slog.Any("article_ids", articleIDs))...)
		return nil, err
	}
	for _, row := range rows {
		articleID, err := shared.PGToUUID(row.ArticleID)
		if err != nil {
			s.logger.LogAttrs(ctx, slog.LevelError, "article tag uuid err", shared.ErrorAttrs(err, shared.UUIDAttr("article_id", row.ArticleID), shared.UUIDAttr("tag_id", row.TagID))...)
			return nil, err
		}
		tagID, err := shared.PGToUUID(row.TagID)
		if err != nil {
			s.logger.LogAttrs(ctx, slog.LevelError, "article tag uuid err", shared.ErrorAttrs(err, slog.String("article_id", articleID.String()), shared.UUIDAttr("tag_id", row.TagID))...)
			return nil, err
		}
		result[articleID] = append(result[articleID], tagID)
	}
	return result, nil
}

func parseUUIDs(values []pgtype.UUID) ([]uuid.UUID, error) {
	ids := make([]uuid.UUID, 0, len(values))
	for _, value := range values {
		id, err := shared.PGToUUID(value)
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func toPGUUIDs(ids []uuid.UUID) []pgtype.UUID {
	values := make([]pgtype.UUID, len(ids))
	for i := range ids {
		values[i] = shared.UUIDToPG(ids[i])
	}
	return values
}
