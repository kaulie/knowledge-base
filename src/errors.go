package kb

import (
	"errors"
	"fmt"
)

// ErrorCode is the machine-readable reason a call failed. Autonomy branches
// on the code (retry / change query / need_input), never on the message.
type ErrorCode string

const (
	ErrNotFound          ErrorCode = "NOT_FOUND"
	ErrInvalidQuery      ErrorCode = "INVALID_QUERY"
	ErrSourceUnavailable ErrorCode = "SOURCE_UNAVAILABLE"
	ErrParseFailed       ErrorCode = "PARSE_FAILED"
	ErrIndexFailed       ErrorCode = "INDEX_FAILED"
	ErrForbidden         ErrorCode = "FORBIDDEN" // reserved: permission control is not enforced in V1
)

type Error struct {
	Code    ErrorCode
	Message string
	Err     error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Err != nil {
		return string(e.Code) + ": " + e.Message + ": " + e.Err.Error()
	}
	return string(e.Code) + ": " + e.Message
}

func (e *Error) Unwrap() error { return e.Err }

func Errorf(code ErrorCode, format string, args ...any) *Error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...)}
}

func Wrap(code ErrorCode, err error, format string, args ...any) *Error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...), Err: err}
}

func CodeOf(err error) ErrorCode {
	var typed *Error
	if errors.As(err, &typed) {
		return typed.Code
	}
	return ""
}

func IsCode(err error, code ErrorCode) bool { return CodeOf(err) == code }
