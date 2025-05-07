package apperrors

import "errors"

// Standard application errors
var (
	// ErrNotFound is returned when a requested resource is not found.
	ErrNotFound = errors.New("resource not found")

	// ErrInvalidInput is returned when the input provided by the client is invalid.
	ErrInvalidInput = errors.New("invalid input provided")

	// ErrUnauthorized is returned when a request lacks valid authentication credentials.
	ErrUnauthorized = errors.New("unauthorized access")

	// ErrForbidden is returned when a server understands the request but refuses to authorize it.
	ErrForbidden = errors.New("forbidden access")

	// ErrExternalServiceFailure is returned when an interaction with an external service fails.
	ErrExternalServiceFailure = errors.New("external service interaction failed")

	// ErrTimeout is returned when an operation times out.
	ErrTimeout = errors.New("operation timed out")

	// ErrInternal is returned for unexpected internal system errors.
	ErrInternal = errors.New("internal system error")

	// ErrConflict is returned when a request conflicts with current state of the target resource.
	ErrConflict = errors.New("request conflicts with current state")
)
