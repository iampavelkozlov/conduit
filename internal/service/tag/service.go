package tag

import (
	"context"
	"errors"
	"log/slog"

	"conduit/internal/models"
	"conduit/internal/service/shared"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// Service owns the tags table. Relations to articles live in articletag.
type Service struct {
	repo   repository
	logger *slog.Logger
}

func New(repo repository, loggers ...*slog.Logger) *Service {
	return &Service{repo: repo, logger: shared.ServiceLogger(loggers...)}
}
func (s *Service) GetTags(ctx context.Context) (*models.TagsResponse, error) {
	tags, err := s.repo.ListTags(ctx)
	if err != nil {
		s.logger.LogAttrs(ctx, slog.LevelError, "tag repo err", shared.ErrorAttrs(err)...)
		return nil, err
	}
	return &models.TagsResponse{Tags: tags}, nil
}
func (s *Service) IDByName(ctx context.Context, name string) (uuid.UUID, error) {
	id, err := s.repo.GetTagIDByName(ctx, name)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.UUID{}, shared.ErrNotFound
		}
		s.logger.LogAttrs(ctx, slog.LevelError, "tag repo err", shared.ErrorAttrs(err)...)
		return uuid.UUID{}, err
	}
	parsed, err := shared.PGToUUID(id)
	if err != nil {
		s.logger.LogAttrs(ctx, slog.LevelError, "tag uuid err", shared.ErrorAttrs(err, shared.UUIDAttr("tag_id", id))...)
	}
	return parsed, err
}
func (s *Service) NamesByIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	names := make(map[uuid.UUID]string, len(ids))
	if len(ids) == 0 {
		return names, nil
	}
	pgIDs := make([]pgtype.UUID, 0, len(ids))
	for _, id := range ids {
		pgIDs = append(pgIDs, shared.UUIDToPG(id))
	}
	tags, err := s.repo.ListTagsByIDs(ctx, pgIDs)
	if err != nil {
		s.logger.LogAttrs(ctx, slog.LevelError, "tag repo err", shared.ErrorAttrs(err, slog.Any("tag_ids", ids))...)
		return nil, err
	}
	for _, item := range tags {
		id, err := shared.PGToUUID(item.ID)
		if err != nil {
			s.logger.LogAttrs(ctx, slog.LevelError, "tag uuid err", shared.ErrorAttrs(err, shared.UUIDAttr("tag_id", item.ID))...)
			return nil, err
		}
		names[id] = item.Name
	}
	return names, nil
}
