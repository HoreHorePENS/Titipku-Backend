package apperror

import (
	"errors"
	"fmt"
)

type Kind uint8

const (
	KindInternal Kind = iota
	KindInvalid
	KindUnauthorized
	KindForbidden
	KindNotFound
	KindConflict
	KindUnavailable
	KindMethodNotAllowed
	KindTooManyRequests
)

const (
	CodeValidation       = "VALIDATION_ERROR"
	CodeInternal         = "INTERNAL_ERROR"
	CodeUnauthorized     = "UNAUTHORIZED"
	CodeForbidden        = "FORBIDDEN"
	CodeNotFound         = "NOT_FOUND"
	CodeConflict         = "CONFLICT"
	CodeUnavailable      = "UNAVAILABLE"
	CodeMethodNotAllowed = "METHOD_NOT_ALLOWED"
	CodeRateLimited      = "RATE_LIMITED"
)

type Error struct {
	Kind    Kind
	Code    string
	Message string
	Fields  map[string]string

	cause error
}

func (e *Error) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %v", e.Code, e.cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *Error) Unwrap() error {
	return e.cause
}

func New(kind Kind, code, message string) *Error {
	return &Error{Kind: kind, Code: code, Message: message}
}

func (e *Error) WithCause(err error) *Error {
	clone := *e
	clone.cause = err
	return &clone
}

func (e *Error) WithFields(fields map[string]string) *Error {
	clone := *e
	clone.Fields = fields
	return &clone
}

func Invalid(code, message string) *Error {
	return New(KindInvalid, code, message)
}

func Validation(fields map[string]string) *Error {
	return &Error{
		Kind:    KindInvalid,
		Code:    CodeValidation,
		Message: "request validation failed",
		Fields:  fields,
	}
}

func Unauthorized(code, message string) *Error {
	return New(KindUnauthorized, code, message)
}

func Forbidden(code, message string) *Error {
	return New(KindForbidden, code, message)
}

func NotFound(code, message string) *Error {
	return New(KindNotFound, code, message)
}

func Conflict(code, message string) *Error {
	return New(KindConflict, code, message)
}

func Unavailable(code, message string) *Error {
	return New(KindUnavailable, code, message)
}

func MethodNotAllowed() *Error {
	return New(KindMethodNotAllowed, CodeMethodNotAllowed, "method not allowed for this route")
}

func Internal(code, message string) *Error {
	return New(KindInternal, code, message)
}

func From(err error) *Error {
	if err == nil {
		return nil
	}

	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr
	}

	return Internal(CodeInternal, "internal server error").WithCause(err)
}
