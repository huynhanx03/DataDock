package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/huynhanx03/datadock/internal/core/apperror"
)

func BindJSON(ctx *gin.Context, destination any) bool {
	return bindJSON(ctx, destination, false)
}

func BindOptionalJSON(ctx *gin.Context, destination any) bool {
	return bindJSON(ctx, destination, true)
}

func bindJSON(ctx *gin.Context, destination any, optional bool) bool {
	if ctx.Request.Body == nil {
		if optional {
			return true
		}
		BadRequest(ctx)
		return false
	}
	decoder := json.NewDecoder(ctx.Request.Body)
	decoder.UseNumber()
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		if optional && errors.Is(err, io.EOF) {
			return true
		}
		writeBindError(ctx, err)
		return false
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			err = errors.New("multiple JSON values")
		}
		writeBindError(ctx, err)
		return false
	}
	if err := binding.Validator.ValidateStruct(destination); err != nil {
		WriteError(ctx, apperror.NewValidation("Invalid request body", err))
		return false
	}
	return true
}

func writeBindError(ctx *gin.Context, err error) {
	var maximumBytesError *http.MaxBytesError
	if errors.As(err, &maximumBytesError) {
		WriteError(ctx, apperror.NewRequestTooLarge("Request body is too large", err))
		return
	}
	WriteError(ctx, apperror.NewValidation("Invalid JSON request body", err))
}
