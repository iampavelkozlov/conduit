package shared

import (
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrNotFound     = errors.New("not found")
	ErrForbidden    = errors.New("forbidden")
	ErrUnauthorized = errors.New("unauthorized")
	ErrValidation   = errors.New("validation failed")
	ErrInvalidUUID  = errors.New("invalid uuid")
)

func NewUUID() pgtype.UUID {
	return UUIDToPG(NewUUIDValue())
}

func NewUUIDValue() uuid.UUID {
	id := uuid.New()
	id[6] = id[6]&0x0f | 0x80
	id[8] = id[8]&0x3f | 0x80
	return id
}

func UUIDToPG(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: true}
}

func PGToUUID(id pgtype.UUID) (uuid.UUID, error) {
	if !id.Valid {
		return uuid.UUID{}, ErrInvalidUUID
	}
	return uuid.FromBytes(id.Bytes[:])
}

func TextFromPtr(p *string) pgtype.Text {
	if p == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *p, Valid: true}
}

func StringFromPtr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
