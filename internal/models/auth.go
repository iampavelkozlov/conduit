package models

type LoginUser struct {
	Email    string
	Password string
}

type LoginUserRequest struct {
	User LoginUser
}

type NewUser struct {
	Email    string
	Password string
	Username string
}

type NewUserRequest struct {
	User NewUser
}
