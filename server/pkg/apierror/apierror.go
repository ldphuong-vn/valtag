package apierror

import (
	"encoding/json"
	"net/http"
)

type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type errorResponse struct {
	Error APIError `json:"error"`
}

func Write(w http.ResponseWriter, statusCode int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	// Nothing left to do if the client has already gone or the response
	// fails to encode — the status code above is already sent.
	_ = json.NewEncoder(w).Encode(errorResponse{
		Error: APIError{Code: code, Message: message},
	})
}

// Common helpers
func BadRequest(w http.ResponseWriter, message string) {
	Write(w, http.StatusBadRequest, "bad_request", message)
}

func Unauthorized(w http.ResponseWriter, message string) {
	Write(w, http.StatusUnauthorized, "unauthorized", message)
}

func Forbidden(w http.ResponseWriter, message string) {
	Write(w, http.StatusForbidden, "forbidden", message)
}

func NotFound(w http.ResponseWriter, message string) {
	Write(w, http.StatusNotFound, "not_found", message)
}

func Conflict(w http.ResponseWriter, message string) {
	Write(w, http.StatusConflict, "conflict", message)
}

func TooManyRequests(w http.ResponseWriter, message string) {
	Write(w, http.StatusTooManyRequests, "rate_limit_exceeded", message)
}

func InternalError(w http.ResponseWriter, message string) {
	Write(w, http.StatusInternalServerError, "internal_error", message)
}
