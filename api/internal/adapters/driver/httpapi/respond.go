package httpapi

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/huynhanx03/datadock/internal/core/apperror"
	"github.com/huynhanx03/go-common/pkg/cid"
)

type errorEnvelope struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	RequestID string         `json:"requestId,omitempty"`
	Details   map[string]any `json:"details,omitempty"`
}

func Respond(ctx *gin.Context, data any, err error) {
	if err != nil {
		WriteError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"data": data})
}

func RespondCreated(ctx *gin.Context, data any, err error) {
	if err != nil {
		WriteError(ctx, err)
		return
	}
	ctx.JSON(http.StatusCreated, gin.H{"data": data})
}

func WriteError(ctx *gin.Context, err error) {
	status, code, message := mapError(err)
	ctx.JSON(status, errorEnvelope{Error: errorBody{Code: code, Message: message, RequestID: cid.FromContext(ctx.Request.Context()), Details: errorDetails(err)}})
}

func errorDetails(err error) map[string]any {
	var applicationError *apperror.Error
	if !errors.As(err, &applicationError) || !knownErrorCode(applicationError.Code) || len(applicationError.Details) == 0 {
		return nil
	}
	details := make(map[string]any, len(applicationError.Details))
	for key, value := range applicationError.Details {
		details[key] = value
	}
	return details
}

func BadRequest(ctx *gin.Context) {
	WriteError(ctx, apperror.NewValidation("Invalid request body", nil))
}

func mapError(err error) (int, string, string) {
	var appErr *apperror.Error
	if !errors.As(err, &appErr) {
		return http.StatusInternalServerError, apperror.CodeInternal, "Internal server error"
	}
	status, known := errorStatus(appErr.Code)
	if !known {
		return http.StatusInternalServerError, apperror.CodeInternal, "Internal server error"
	}
	message := appErr.Message
	if message == "" {
		message = "Request failed"
	}
	return status, appErr.Code, message
}

func knownErrorCode(code string) bool {
	_, known := errorStatus(code)
	return known
}

func errorStatus(code string) (int, bool) {
	switch code {
	case apperror.CodeValidation:
		return http.StatusBadRequest, true
	case apperror.CodeNotFound:
		return http.StatusNotFound, true
	case apperror.CodeConflict, apperror.CodeConnectionRequired, apperror.CodeConnectionBusy, apperror.CodeQueryCancelled:
		return http.StatusConflict, true
	case apperror.CodeReadonlyViolation, apperror.CodePermissionDenied:
		return http.StatusForbidden, true
	case apperror.CodeUnsupportedCapability:
		return http.StatusUnprocessableEntity, true
	case apperror.CodeTransactionExpired:
		return http.StatusGone, true
	case apperror.CodeRequestTooLarge:
		return http.StatusRequestEntityTooLarge, true
	case apperror.CodeUnsupportedMediaType:
		return http.StatusUnsupportedMediaType, true
	case apperror.CodeMethodNotAllowed:
		return http.StatusMethodNotAllowed, true
	case apperror.CodeServiceUnavailable:
		return http.StatusServiceUnavailable, true
	case apperror.CodeQueryTimeout:
		return http.StatusGatewayTimeout, true
	case apperror.CodeConnectionFailed, apperror.CodeTransportFailed:
		return http.StatusBadGateway, true
	case apperror.CodeInternal:
		return http.StatusInternalServerError, true
	default:
		return 0, false
	}
}
