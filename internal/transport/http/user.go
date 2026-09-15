package http

import (
	"context"

	api "conduit/internal/gen/http"
	"conduit/internal/models"
)

func (s Server) CreateUser(ctx context.Context, request api.CreateUserRequestObject) (api.CreateUserResponseObject, error) {
	dto := request.Body.User
	model := models.NewUserRequest{User: models.NewUser{Email: dto.Email, Password: dto.Password, Username: dto.Username}}
	resp, err := s.svc.CreateUser(ctx, model)
	if err != nil {
		return nil, err
	}
	return api.CreateUser201JSONResponse{UserResponseJSONResponse: userResponse(&resp.User)}, nil
}

func (s Server) FollowUserByUsername(ctx context.Context, request api.FollowUserByUsernameRequestObject) (api.FollowUserByUsernameResponseObject, error) {
	resp, err := s.svc.FollowUserByUsername(ctx, request.Username)
	if err != nil {
		return nil, err
	}
	return api.FollowUserByUsername200JSONResponse{ProfileResponseJSONResponse: profileResponse(resp.Profile)}, nil
}

func (s Server) GetCurrentUser(ctx context.Context, _ api.GetCurrentUserRequestObject) (api.GetCurrentUserResponseObject, error) {
	resp, err := s.svc.GetCurrentUser(ctx)
	if err != nil {
		return nil, err
	}
	return api.GetCurrentUser200JSONResponse{UserResponseJSONResponse: userResponse(&resp.User)}, nil
}

func (s Server) GetProfileByUsername(ctx context.Context, request api.GetProfileByUsernameRequestObject) (api.GetProfileByUsernameResponseObject, error) {
	resp, err := s.svc.GetProfileByUsername(ctx, request.Username)
	if err != nil {
		return nil, err
	}
	return api.GetProfileByUsername200JSONResponse{ProfileResponseJSONResponse: profileResponse(resp.Profile)}, nil
}

func (s Server) UpdateCurrentUser(ctx context.Context, request api.UpdateCurrentUserRequestObject) (api.UpdateCurrentUserResponseObject, error) {
	dto := request.Body.User
	model := models.UpdateUserRequest{User: models.UpdateUser{
		Bio: dto.Bio, BioSet: updateFieldWasSet(ctx, "user", "bio"),
		Email: dto.Email, EmailSet: updateFieldWasSet(ctx, "user", "email"),
		Image: dto.Image, ImageSet: updateFieldWasSet(ctx, "user", "image"),
		Password: dto.Password, PasswordSet: updateFieldWasSet(ctx, "user", "password"),
		Username: dto.Username, UsernameSet: updateFieldWasSet(ctx, "user", "username"),
	}}
	resp, err := s.svc.UpdateCurrentUser(ctx, &model)
	if err != nil {
		return nil, err
	}
	return api.UpdateCurrentUser200JSONResponse{UserResponseJSONResponse: userResponse(&resp.User)}, nil
}

func (s Server) UnfollowUserByUsername(ctx context.Context, request api.UnfollowUserByUsernameRequestObject) (api.UnfollowUserByUsernameResponseObject, error) {
	resp, err := s.svc.UnfollowUserByUsername(ctx, request.Username)
	if err != nil {
		return nil, err
	}
	return api.UnfollowUserByUsername200JSONResponse{ProfileResponseJSONResponse: profileResponse(resp.Profile)}, nil
}
