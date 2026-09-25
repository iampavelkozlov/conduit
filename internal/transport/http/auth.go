package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	api "conduit/internal/gen/http"
	"conduit/internal/models"
	"conduit/internal/service/auth"

	"github.com/jackc/pgx/v5"
)

const (
	refreshTokenCookieName = "conduit_refresh"
	refreshCookiePath      = "/internal/auth/refresh"
)

type loginResponse struct {
	api.Login200JSONResponse
	refreshToken string
}

func (r *loginResponse) VisitLoginResponse(w http.ResponseWriter) error {
	setRefreshTokenCookie(w, r.refreshToken)
	return r.Login200JSONResponse.VisitLoginResponse(w)
}

type createUserResponse struct {
	api.CreateUser201JSONResponse
	refreshToken string
}

func (r *createUserResponse) VisitCreateUserResponse(w http.ResponseWriter) error {
	setRefreshTokenCookie(w, r.refreshToken)
	return r.CreateUser201JSONResponse.VisitCreateUserResponse(w)
}

func (s Server) Login(ctx context.Context, request api.LoginRequestObject) (api.LoginResponseObject, error) {
	model := models.LoginUserRequest{User: models.LoginUser{Email: request.Body.User.Email, Password: request.Body.User.Password}}
	resp, err := s.svc.Login(ctx, model)
	if err != nil {
		return nil, err
	}
	return &loginResponse{
		Login200JSONResponse: api.Login200JSONResponse{UserResponseJSONResponse: userResponse(&resp.User)},
		refreshToken:         resp.User.RefreshToken,
	}, nil
}

// NewRefreshHandler returns the private frontend refresh endpoint. It deliberately
// lives outside the canonical OpenAPI router so the RealWorld contract remains
// unchanged.
func NewRefreshHandler(svc applicationService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		accessToken, ok := requestToken(r.Header.Get("Authorization"))
		if !ok {
			writeErrorResponse(w, http.StatusUnauthorized, "token", "invalid")
			return
		}
		refreshCookie, err := r.Cookie(refreshTokenCookieName)
		if err != nil || refreshCookie.Value == "" {
			writeErrorResponse(w, http.StatusUnauthorized, "token", "invalid")
			return
		}

		resp, err := svc.RefreshToken(r.Context(), accessToken, refreshCookie.Value)
		if err != nil {
			if errors.Is(err, auth.ErrInvalidToken) || errors.Is(err, auth.ErrExpiredToken) || errors.Is(err, pgx.ErrNoRows) {
				clearRefreshTokenCookie(w)
				writeErrorResponse(w, http.StatusUnauthorized, "token", "invalid")
				return
			}
			writeErrorResponse(w, http.StatusInternalServerError, "body", "internal server error")
			return
		}

		setRefreshTokenCookie(w, resp.User.RefreshToken)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(userResponse(&resp.User))
	})
}

func requestToken(header string) (string, bool) {
	for _, prefix := range []string{"Bearer ", "Token "} {
		if token, ok := strings.CutPrefix(header, prefix); ok {
			return token, token != ""
		}
	}
	return "", false
}

func setRefreshTokenCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name: refreshTokenCookieName, Value: token, Path: refreshCookiePath, HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode,
	})
}

func clearRefreshTokenCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: refreshTokenCookieName, Path: refreshCookiePath, HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode, MaxAge: -1,
	})
}
