package http

import (
	"context"

	api "conduit/internal/gen/http"
)

func (s Server) GetTags(ctx context.Context, _ api.GetTagsRequestObject) (api.GetTagsResponseObject, error) {
	resp, err := s.svc.GetTags(ctx)
	if err != nil {
		return nil, err
	}
	return api.GetTags200JSONResponse{TagsResponseJSONResponse: api.TagsResponseJSONResponse{Tags: resp.Tags}}, nil
}
