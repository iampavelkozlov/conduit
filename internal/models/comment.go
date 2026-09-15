package models

import "time"

type Comment struct {
	Author    Profile
	Body      string
	CreatedAt time.Time
	ID        int
	UpdatedAt time.Time
}

type NewComment struct {
	Body string
}

type NewCommentRequest struct {
	Comment NewComment
}

type SingleCommentResponse struct {
	Comment Comment
}

type MultipleCommentsResponse struct {
	Comments []Comment
}
