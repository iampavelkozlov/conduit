package auth

import (
	"errors"
	"testing"

	"conduit/internal/gen/postgres"
	"conduit/internal/service/shared"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestValidateRefreshTokenDelegates(t *testing.T) {
	tokens := NewMockTokenManagerIface(gomock.NewController(t))
	want := &TokenClaims{UserID: uuid.NewString()}
	tokens.EXPECT().ValidateRefreshToken("refresh").Return(want, nil)
	claims, err := NewWithDeps(nil, nil, nil, tokens, nil).ValidateRefreshToken("refresh")
	require.NoError(t, err)
	require.Same(t, want, claims)
}

func TestGetAccount(t *testing.T) {
	userID := uuid.New()
	t.Run("success", func(t *testing.T) {
		repo := NewMockUserRepository(gomock.NewController(t))
		repo.EXPECT().GetUserByID(gomock.Any(), uuidToPGType(userID)).Return(postgres.User{ID: uuidToPGType(userID), Email: "user@example.com"}, nil)
		account, err := NewWithDeps(repo, nil, nil, nil, nil).GetAccount(t.Context(), userID)
		require.NoError(t, err)
		require.Equal(t, &Account{UserID: userID, Email: "user@example.com"}, account)
	})
	t.Run("not found", func(t *testing.T) {
		repo := NewMockUserRepository(gomock.NewController(t))
		repo.EXPECT().GetUserByID(gomock.Any(), uuidToPGType(userID)).Return(postgres.User{}, pgx.ErrNoRows)
		account, err := NewWithDeps(repo, nil, nil, nil, nil).GetAccount(t.Context(), userID)
		require.Nil(t, account)
		require.ErrorIs(t, err, shared.ErrNotFound)
	})
	t.Run("repository failure", func(t *testing.T) {
		repo := NewMockUserRepository(gomock.NewController(t))
		wantErr := errors.New("database offline")
		repo.EXPECT().GetUserByID(gomock.Any(), uuidToPGType(userID)).Return(postgres.User{}, wantErr)
		account, err := NewWithDeps(repo, nil, nil, nil, nil).GetAccount(t.Context(), userID)
		require.Nil(t, account)
		require.ErrorIs(t, err, wantErr)
	})
	t.Run("invalid persisted id", func(t *testing.T) {
		repo := NewMockUserRepository(gomock.NewController(t))
		repo.EXPECT().GetUserByID(gomock.Any(), uuidToPGType(userID)).Return(postgres.User{Email: "user@example.com"}, nil)
		account, err := NewWithDeps(repo, nil, nil, nil, nil).GetAccount(t.Context(), userID)
		require.Nil(t, account)
		require.ErrorContains(t, err, "parse user id")
	})
}

func TestUpdateCredentials(t *testing.T) {
	userID := uuid.New()
	t.Run("success", func(t *testing.T) {
		repo := NewMockUserRepository(gomock.NewController(t))
		passwords := NewMockPasswordManagerIface(gomock.NewController(t))
		email, password := "new@example.com", "password"
		passwords.EXPECT().Hash(password).Return("hash", nil)
		repo.EXPECT().UpdateUser(gomock.Any(), postgres.UpdateUserParams{
			SetEmail: true, Email: email, SetPassword: true, PasswordHash: "hash", ID: uuidToPGType(userID),
		}).Return(postgres.User{ID: uuidToPGType(userID), Email: email}, nil)
		account, err := NewWithDeps(repo, nil, nil, nil, passwords).UpdateCredentials(t.Context(), userID, &email, &password)
		require.NoError(t, err)
		require.Equal(t, email, account.Email)
	})
	t.Run("validates supplied fields", func(t *testing.T) {
		empty := ""
		service := NewWithDeps(NewMockUserRepository(gomock.NewController(t)), nil, nil, nil, NewMockPasswordManagerIface(gomock.NewController(t)))
		_, err := service.UpdateCredentials(t.Context(), userID, &empty, nil)
		require.ErrorIs(t, err, shared.ErrValidation)
		_, err = service.UpdateCredentials(t.Context(), userID, nil, &empty)
		require.ErrorIs(t, err, shared.ErrValidation)
		short := "short7c"
		_, err = service.UpdateCredentials(t.Context(), userID, nil, &short)
		require.ErrorIs(t, err, shared.ErrValidation)
	})
	t.Run("hash failure", func(t *testing.T) {
		passwords := NewMockPasswordManagerIface(gomock.NewController(t))
		password := "password"
		passwords.EXPECT().Hash(password).Return("", errors.New("hash failed"))
		_, err := NewWithDeps(NewMockUserRepository(gomock.NewController(t)), nil, nil, nil, passwords).UpdateCredentials(t.Context(), userID, nil, &password)
		require.ErrorContains(t, err, "hash password")
	})
	t.Run("not found", func(t *testing.T) {
		repo := NewMockUserRepository(gomock.NewController(t))
		repo.EXPECT().UpdateUser(gomock.Any(), gomock.Any()).Return(postgres.User{}, pgx.ErrNoRows)
		_, err := NewWithDeps(repo, nil, nil, nil, NewMockPasswordManagerIface(gomock.NewController(t))).UpdateCredentials(t.Context(), userID, nil, nil)
		require.ErrorIs(t, err, shared.ErrNotFound)
	})
	t.Run("repository failure", func(t *testing.T) {
		repo := NewMockUserRepository(gomock.NewController(t))
		wantErr := errors.New("database offline")
		repo.EXPECT().UpdateUser(gomock.Any(), gomock.Any()).Return(postgres.User{}, wantErr)
		_, err := NewWithDeps(repo, nil, nil, nil, NewMockPasswordManagerIface(gomock.NewController(t))).UpdateCredentials(t.Context(), userID, nil, nil)
		require.ErrorIs(t, err, wantErr)
	})
}

func TestDeleteAccount(t *testing.T) {
	userID := uuid.New()
	tests := []struct {
		name    string
		rows    int64
		repoErr error
		wantErr error
	}{
		{name: "success", rows: 1},
		{name: "not found", wantErr: shared.ErrNotFound},
		{name: "repository failure", repoErr: errors.New("database offline")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := NewMockUserRepository(gomock.NewController(t))
			repo.EXPECT().DeleteUser(gomock.Any(), uuidToPGType(userID)).Return(tt.rows, tt.repoErr)
			err := NewWithDeps(repo, nil, nil, nil, nil).DeleteAccount(t.Context(), userID)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			} else {
				require.ErrorIs(t, err, tt.repoErr)
			}
		})
	}
}
