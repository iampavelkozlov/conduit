package http

import (
	"context"

	api "conduit/internal/gen/http"
	"conduit/internal/models"
)

func (s Server) Login(ctx context.Context, request api.LoginRequestObject) (api.LoginResponseObject, error) {
	model := models.LoginUserRequest{User: models.LoginUser{Email: request.Body.User.Email, Password: request.Body.User.Password}}
	resp, err := s.svc.Login(ctx, model)
	if err != nil {
		return nil, err
	}
	return api.Login200JSONResponse{UserResponseJSONResponse: userResponse(&resp.User)}, nil
}
