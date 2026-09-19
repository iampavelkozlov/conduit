package models

import (
	"time"

	"github.com/google/uuid"
)

type Article struct {
	Author         Profile
	Body           string
	CreatedAt      time.Time
	Description    string
	Favorited      bool
	FavoritesCount int
	ID             uuid.UUID
	Slug           string
	TagList        []string
	Title          string
	UpdatedAt      time.Time
}

type NewArticle struct {
	Body        string
	Description string
	TagList     *[]string
	Title       string
}

type NewArticleRequest struct {
	Article NewArticle
}

type UpdateArticle struct {
	Body           *string
	BodySet        bool
	Description    *string
	DescriptionSet bool
	TagList        *[]string
	TagListSet     bool
	Title          *string
	TitleSet       bool
}

type UpdateArticleRequest struct {
	Article UpdateArticle
}

type SingleArticleResponse struct {
	Article Article
}

type MultipleArticlesResponse struct {
	Articles      []Article
	ArticlesCount int
}

type GetArticlesParams struct {
	Tag       *string
	Author    *string
	Favorited *string
	Limit     *int
	Offset    *int
}

type GetArticlesFeedParams struct {
	Limit  *int
	Offset *int
}
