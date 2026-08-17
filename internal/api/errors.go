package api

import (
	"errors"
	"fmt"
)

type ErrorCode string

const (
	CodeNotAuthenticated ErrorCode = "not_authenticated"
	CodePermissionDenied ErrorCode = "permission_denied"
	CodeInvalidInput     ErrorCode = "invalid_input"
	CodeNetwork          ErrorCode = "network_error"
	CodeUpstream         ErrorCode = "upstream_error"
	CodeNotFound         ErrorCode = "not_found"
	CodeRateLimited      ErrorCode = "rate_limited"
	CodeInternal         ErrorCode = "internal_error"
)

type Error struct {
	Code      ErrorCode
	Action    string
	Message   string
	HTTPStatus int
	APIStatus int
	Err       error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Action == "" {
		return e.Message
	}
	if e.Message == "" {
		return e.Action
	}
	return fmt.Sprintf("%s: %s", e.Action, e.Message)
}

func (e *Error) Unwrap() error { return e.Err }

func NewError(code ErrorCode, action, message string) *Error {
	return &Error{Code: code, Action: action, Message: message}
}

func CodeOf(err error) ErrorCode {
	var apiErr *Error
	if errors.As(err, &apiErr) && apiErr.Code != "" {
		return apiErr.Code
	}
	return CodeInternal
}

func IsNetwork(err error) bool {
	var apiErr *Error
	return errors.As(err, &apiErr) && apiErr.Code == CodeNetwork
}

func IsAuth(err error) bool {
	var apiErr *Error
	return errors.As(err, &apiErr) && apiErr.Code == CodeNotAuthenticated
}

