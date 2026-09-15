package models

import "github.com/google/uuid"

type User struct {
	Bio          string
	Email        string
	Image        string
	RefreshToken string
	Token        string
	Username     string
}

type UserResponse struct {
	User User
}

type UpdateUser struct {
	Bio         *string
	BioSet      bool
	Email       *string
	EmailSet    bool
	Image       *string
	ImageSet    bool
	Password    *string
	PasswordSet bool
	Username    *string
	UsernameSet bool
}

type UpdateUserRequest struct {
	User UpdateUser
}

type Profile struct {
	Bio       string
	Following bool
	ID        uuid.UUID
	Image     string
	Username  string
}

type ProfileResponse struct {
	Profile Profile
}
