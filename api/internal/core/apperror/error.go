package apperror

import "errors"

const (
	CodeValidation            = "validation_error"
	CodeNotFound              = "not_found"
	CodeConflict              = "conflict"
	CodeConnectionFailed      = "connection_failed"
	CodeConnectionRequired    = "connection_required"
	CodeConnectionBusy        = "connection_busy"
	CodeQueryCancelled        = "query_cancelled"
	CodeQueryTimeout          = "query_timeout"
	CodeReadonlyViolation     = "readonly_violation"
	CodePermissionDenied      = "permission_denied"
	CodeUnsupportedCapability = "unsupported_capability"
	CodeTransactionExpired    = "transaction_expired"
	CodeTransportFailed       = "transport_failed"
	CodeRequestTooLarge       = "request_too_large"
	CodeUnsupportedMediaType  = "unsupported_media_type"
	CodeMethodNotAllowed      = "method_not_allowed"
	CodeServiceUnavailable    = "service_unavailable"
	CodeInternal              = "internal_error"
)

type Error struct {
	Code      string
	Message   string
	Cause     error
	Temporary bool
	Details   map[string]any
}

func (err *Error) Error() string {
	if err == nil {
		return ""
	}
	if err.Message != "" {
		return err.Message
	}
	return err.Code
}

func (err *Error) Unwrap() error {
	if err == nil {
		return nil
	}
	return err.Cause
}

func New(code, message string, cause error, temporary bool) *Error {
	return &Error{Code: code, Message: message, Cause: cause, Temporary: temporary}
}

func NewValidation(message string, cause error) *Error {
	return New(CodeValidation, message, cause, false)
}

func NewNotFound(message string, cause error) *Error {
	return New(CodeNotFound, message, cause, false)
}

func NewConflict(message string, cause error) *Error {
	return New(CodeConflict, message, cause, false)
}

func NewConnection(message string, cause error) *Error {
	return New(CodeConnectionFailed, message, cause, true)
}

func NewConnectionRequired(message string, cause error) *Error {
	return New(CodeConnectionRequired, message, cause, false)
}

func NewConnectionBusy(message string, cause error) *Error {
	return New(CodeConnectionBusy, message, cause, true)
}

func NewTimeout(message string, cause error) *Error {
	return New(CodeQueryTimeout, message, cause, true)
}

func NewCancellation(message string, cause error) *Error {
	return New(CodeQueryCancelled, message, cause, false)
}

func NewReadonly(message string, cause error) *Error {
	return New(CodeReadonlyViolation, message, cause, false)
}

func NewPermission(message string, cause error) *Error {
	return New(CodePermissionDenied, message, cause, false)
}

func NewUnsupported(message string, cause error) *Error {
	return New(CodeUnsupportedCapability, message, cause, false)
}

func NewTransactionExpired(message string, cause error) *Error {
	return New(CodeTransactionExpired, message, cause, false)
}

func NewTransport(message string, cause error) *Error {
	return New(CodeTransportFailed, message, cause, true)
}

func NewRequestTooLarge(message string, cause error) *Error {
	return New(CodeRequestTooLarge, message, cause, false)
}

func NewUnsupportedMediaType(message string, cause error) *Error {
	return New(CodeUnsupportedMediaType, message, cause, false)
}

func NewMethodNotAllowed(message string, cause error) *Error {
	return New(CodeMethodNotAllowed, message, cause, false)
}

func NewServiceUnavailable(message string, cause error) *Error {
	return New(CodeServiceUnavailable, message, cause, true)
}

func NewInternal(message string, cause error) *Error {
	return New(CodeInternal, message, cause, false)
}

func WithDetails(err error, details map[string]any) error {
	var applicationError *Error
	if !errors.As(err, &applicationError) {
		return err
	}
	clone := *applicationError
	clone.Details = make(map[string]any, len(details))
	for key, value := range details {
		clone.Details[key] = value
	}
	return &clone
}
