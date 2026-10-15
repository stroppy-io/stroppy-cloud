// Package errs is the domain error vocabulary: every failure a service
// reports carries a stable Code the API maps to an RFC 9457 Problem and the
// UI branches on. Wrap with fmt.Errorf("%w") freely; the code survives.
package errs

import (
	"errors"
	"fmt"
	"net/http"
)

// Code is the stable machine-readable problem code.
type Code string

// Codes. Add here, never invent strings inline.
const (
	CodeUnauthenticated Code = "unauthenticated"
	CodeForbidden       Code = "forbidden"
	CodeNotFound        Code = "not_found"
	CodeConflict        Code = "conflict"
	CodeInvalid         Code = "invalid"
	CodeValidation      Code = "validation_failed"
	CodeLimit           Code = "limit_exceeded"
	CodeUnavailable     Code = "unavailable"
	CodeInternal        Code = "internal"
)

// Error is a coded domain error.
type Error struct {
	Code   Code
	Detail string
	// Reason is the stable machine key of WHY it failed (`tenant_slug_taken`),
	// finer than Code: the UI shows its own text for it, filled from Params.
	// Detail stays the English explanation for logs and API clients.
	Reason string
	Params map[string]string
	// Validation is the schemapb validation result for CodeValidation,
	// serialized by the API; nil otherwise.
	Validation any
	cause      error
}

func (e *Error) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Detail, e.cause)
	}
	if e.Detail == "" {
		return string(e.Code)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Detail)
}

// Because names the reason and its parameters (key, value pairs). It is
// called on a freshly built error, before anyone else sees it.
func (e *Error) Because(reason string, kv ...string) *Error {
	e.Reason = reason
	if len(kv) > 1 {
		e.Params = make(map[string]string, len(kv)/2)
		for i := 0; i+1 < len(kv); i += 2 {
			e.Params[kv[i]] = kv[i+1]
		}
	}
	return e
}

// ReasonOf extracts the reason and its parameters of err ("" for none).
func ReasonOf(err error) (reason string, params map[string]string) {
	var e *Error
	if errors.As(err, &e) {
		return e.Reason, e.Params
	}
	return "", nil
}

// Unwrap exposes the cause.
func (e *Error) Unwrap() error { return e.cause }

// New builds a coded error.
func New(code Code, detail string) *Error { return &Error{Code: code, Detail: detail} }

// Newf builds a coded error with a formatted detail.
func Newf(code Code, format string, args ...any) *Error {
	return &Error{Code: code, Detail: fmt.Sprintf(format, args...)}
}

// Wrap attaches a cause.
func Wrap(code Code, detail string, cause error) *Error {
	return &Error{Code: code, Detail: detail, cause: cause}
}

// Unauthenticated, NotFound and friends are the frequent shapes.
func Unauthenticated(detail string) *Error { return New(CodeUnauthenticated, detail) }
func Forbidden(detail string) *Error       { return New(CodeForbidden, detail) }
func NotFound(what string) *Error {
	return Newf(CodeNotFound, "%s not found", what).Because(reasonKey(what) + "_not_found")
}
func Conflict(detail string) *Error { return New(CodeConflict, detail) }
func Invalid(detail string) *Error  { return New(CodeInvalid, detail) }

// AsValidation reports a CodeValidation error and returns it.
func AsValidation(err error) (*Error, bool) {
	var e *Error
	if errors.As(err, &e) && e.Code == CodeValidation {
		return e, true
	}
	return nil, false
}

// CodeOf extracts the code of err; CodeInternal for foreign errors.
func CodeOf(err error) Code {
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	return CodeInternal
}

// Status maps a code to its HTTP status.
func Status(code Code) int {
	switch code {
	case CodeUnauthenticated:
		return http.StatusUnauthorized
	case CodeForbidden:
		return http.StatusForbidden
	case CodeNotFound:
		return http.StatusNotFound
	case CodeConflict:
		return http.StatusConflict
	case CodeInvalid, CodeValidation:
		return http.StatusUnprocessableEntity
	case CodeLimit:
		return http.StatusUnprocessableEntity
	case CodeUnavailable:
		return http.StatusServiceUnavailable
	case CodeInternal:
		return http.StatusInternalServerError
	default:
		return http.StatusInternalServerError
	}
}

// reasonKey turns an entity name ("suite run") into a reason key part.
func reasonKey(s string) string {
	b := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z', c >= '0' && c <= '9':
			b = append(b, c)
		case c >= 'A' && c <= 'Z':
			b = append(b, c+'a'-'A')
		case len(b) > 0 && b[len(b)-1] != '_':
			b = append(b, '_')
		}
	}
	for len(b) > 0 && b[len(b)-1] == '_' {
		b = b[:len(b)-1]
	}
	return string(b)
}
