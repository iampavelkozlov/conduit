package comment

import (
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	commentUUIDPrefix  = uint64(0x636f6d6d656e8000)
	commentUUIDVariant = uint64(0x8000000000000000)
	commentIDMask      = uint64(0x3fffffffffffffff)
)

var errInvalidCommentID = errors.New("invalid comment ID")

func newCommentUUID() pgtype.UUID {
	randomUUID := uuid.New()
	commentID := binary.BigEndian.Uint64(randomUUID[8:]) & commentIDMask
	if commentID == 0 {
		commentID = 1
	}

	return commentUUIDFromUint64(commentID)
}

func commentUUIDFromID(commentID int) (pgtype.UUID, error) {
	if commentID <= 0 || uint64(commentID) > commentIDMask {
		return pgtype.UUID{}, errInvalidCommentID
	}

	return commentUUIDFromUint64(uint64(commentID)), nil
}

func commentIDFromUUID(id pgtype.UUID) (int, error) {
	if !id.Valid || binary.BigEndian.Uint64(id.Bytes[:8]) != commentUUIDPrefix {
		return 0, errInvalidCommentID
	}

	encoded := binary.BigEndian.Uint64(id.Bytes[8:])
	if encoded&^commentIDMask != commentUUIDVariant {
		return 0, errInvalidCommentID
	}

	commentID := encoded & commentIDMask
	if commentID == 0 || uint64(int(commentID)) != commentID {
		return 0, fmt.Errorf("%w: value does not fit into int", errInvalidCommentID)
	}

	return int(commentID), nil
}

func commentUUIDFromUint64(commentID uint64) pgtype.UUID {
	var id uuid.UUID
	binary.BigEndian.PutUint64(id[:8], commentUUIDPrefix)
	binary.BigEndian.PutUint64(id[8:], commentUUIDVariant|commentID)
	return pgtype.UUID{Bytes: id, Valid: true}
}
