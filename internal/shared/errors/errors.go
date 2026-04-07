package errors

import "net/http"

type APIError struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
	Status  int            `json:"-"`
}

func (e APIError) Error() string {
	return e.Message
}

func BadRequest(message string) APIError {
	return APIError{Code: "bad_request", Message: message, Status: http.StatusBadRequest}
}

func NotFound(message string) APIError {
	return APIError{Code: "not_found", Message: message, Status: http.StatusNotFound}
}

func Conflict(message string) APIError {
	return APIError{Code: "conflict", Message: message, Status: http.StatusConflict}
}

func Internal(message string) APIError {
	return APIError{Code: "internal", Message: message, Status: http.StatusInternalServerError}
}
