package shared

import (
	"errors"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

// ServiceLogger returns the configured service logger or a silent logger for
// isolated unit tests that do not exercise logging behavior.
func ServiceLogger(loggers ...*slog.Logger) *slog.Logger {
	if len(loggers) > 0 && loggers[0] != nil {
		return loggers[0]
	}
	return slog.New(slog.DiscardHandler)
}

// ErrorAttrs prepends the standard err field to service-specific log fields.
func ErrorAttrs(err error, attrs ...slog.Attr) []slog.Attr {
	return append([]slog.Attr{slog.Any("err", err)}, attrs...)
}

// UUIDAttr renders a pgx UUID as a searchable canonical UUID string.
func UUIDAttr(name string, id pgtype.UUID) slog.Attr {
	return slog.String(name, uuid.UUID(id.Bytes).String())
}

// IsExpectedError identifies client and domain outcomes that should not be
// emitted as service error logs.
func IsExpectedError(err error) bool {
	if _, ok := errors.AsType[*APIError](err); ok {
		return true
	}
	if errors.Is(err, ErrUnauthorized) || errors.Is(err, ErrForbidden) || errors.Is(err, ErrNotFound) || errors.Is(err, ErrValidation) || errors.Is(err, pgx.ErrNoRows) {
		return true
	}
	pgErr, ok := errors.AsType[*pgconn.PgError](err)
	return ok && pgErr.Code == "23505"
}
