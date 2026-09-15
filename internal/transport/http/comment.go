package http

import (
	"context"

	api "conduit/internal/gen/http"
	"conduit/internal/models"
)

func (s Server) CreateArticleComment(ctx context.Context, request api.CreateArticleCommentRequestObject) (api.CreateArticleCommentResponseObject, error) {
	model := models.NewCommentRequest{Comment: models.NewComment{Body: request.Body.Comment.Body}}
	resp, err := s.svc.CreateArticleComment(ctx, request.Slug, model)
	if err != nil {
		return nil, err
	}
	return api.CreateArticleComment201JSONResponse{SingleCommentResponseJSONResponse: api.SingleCommentResponseJSONResponse{Comment: commentDTO(&resp.Comment)}}, nil
}

func (s Server) DeleteArticleComment(ctx context.Context, request api.DeleteArticleCommentRequestObject) (api.DeleteArticleCommentResponseObject, error) {
	if err := s.svc.DeleteArticleComment(ctx, request.Slug, request.Id); err != nil {
		return nil, err
	}
	return api.DeleteArticleComment204Response{}, nil
}

func (s Server) GetArticleComments(ctx context.Context, request api.GetArticleCommentsRequestObject) (api.GetArticleCommentsResponseObject, error) {
	resp, err := s.svc.GetArticleComments(ctx, request.Slug)
	if err != nil {
		return nil, err
	}
	comments := make([]api.Comment, len(resp.Comments))
	for i := range resp.Comments {
		comments[i] = commentDTO(&resp.Comments[i])
	}
	return api.GetArticleComments200JSONResponse{MultipleCommentsResponseJSONResponse: api.MultipleCommentsResponseJSONResponse{Comments: comments}}, nil
}
