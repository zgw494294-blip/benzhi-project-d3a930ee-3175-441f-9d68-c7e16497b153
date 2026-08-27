package domain

import "fmt"

type ErrorCode string

const (
	ErrInvalidInput    ErrorCode = "invalid_input"
	ErrNotFound        ErrorCode = "not_found"
	ErrConflict        ErrorCode = "conflict"
	ErrInvalidState    ErrorCode = "invalid_state"
	ErrVersionMismatch ErrorCode = "version_mismatch"
	ErrIdempotency     ErrorCode = "idempotency_conflict"
)

type DomainError struct {
	Code    ErrorCode
	Message string
}

func (e *DomainError) Error() string  { return e.Message }
func invalid(message string) error    { return &DomainError{Code: ErrInvalidInput, Message: message} }
func notFound(message string) error   { return &DomainError{Code: ErrNotFound, Message: message} }
func conflict(message string) error   { return &DomainError{Code: ErrConflict, Message: message} }
func stateError(message string) error { return &DomainError{Code: ErrInvalidState, Message: message} }
func versionError(message string) error {
	return &DomainError{Code: ErrVersionMismatch, Message: message}
}
func idempotencyError(message string) error {
	return &DomainError{Code: ErrIdempotency, Message: message}
}
func wrap(code ErrorCode, format string, args ...any) error {
	return &DomainError{Code: code, Message: fmt.Sprintf(format, args...)}
}
