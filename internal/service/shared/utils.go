package shared

import (
	"github.com/google/uuid"
	"github.com/gosimple/slug"
)

func GenerateSlug(title string) string {
	base := slug.Make(title)
	if base == "" {
		return uuid.NewString()
	}
	return base + "-" + uuid.NewString()[:8]
}
