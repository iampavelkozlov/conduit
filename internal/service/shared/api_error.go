package shared

import (
	"fmt"
	"net/http"
)

type APIError struct {
	Status  int
	Field   string
	Message string
}

func (e *APIError) Error() string { return fmt.Sprintf("%s: %s", e.Field, e.Message) }

func (e *APIError) Unwrap() error {
	switch e.Status {
	case http.StatusUnprocessableEntity:
		return ErrValidation
	case http.StatusNotFound:
		return ErrNotFound
	case http.StatusForbidden:
		return ErrForbidden
	default:
		return nil
	}
}

func Validation(field, message string) error {
	return &APIError{Status: http.StatusUnprocessableEntity, Field: field, Message: message}
}
func NotFound(resource string) error {
	return &APIError{Status: http.StatusNotFound, Field: resource, Message: "not found"}
}
func Forbidden(resource string) error {
	return &APIError{Status: http.StatusForbidden, Field: resource, Message: "forbidden"}
}
