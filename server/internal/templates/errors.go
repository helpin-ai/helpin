package templates

import (
	"errors"
	"fmt"
)

type ErrorKind string

const (
	ErrorKindValidation ErrorKind = "validation"
	ErrorKindNotFound   ErrorKind = "not_found"
	ErrorKindConflict   ErrorKind = "conflict"
	ErrorKindInternal   ErrorKind = "internal"
)

type Error struct {
	Kind    ErrorKind
	Message string
	Err     error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return e.Message
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return string(e.Kind)
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func ClassifyError(err error) (*Error, bool) {
	var templateErr *Error
	if errors.As(err, &templateErr) {
		return templateErr, true
	}
	return nil, false
}

func validationErrorf(format string, args ...any) error {
	return &Error{Kind: ErrorKindValidation, Message: fmt.Sprintf(format, args...)}
}

func notFoundErrorf(format string, args ...any) error {
	return &Error{Kind: ErrorKindNotFound, Message: fmt.Sprintf(format, args...)}
}

func internalErrorf(err error, message string) error {
	return &Error{Kind: ErrorKindInternal, Message: message, Err: err}
}
