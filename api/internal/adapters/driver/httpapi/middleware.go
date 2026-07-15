package httpapi

import (
	"mime"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/huynhanx03/datadock/internal/core/apperror"
	"github.com/huynhanx03/go-common/pkg/cid"
	commonlogger "github.com/huynhanx03/go-common/pkg/logger"
)

func CorrelationID(logger *zap.Logger) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		requestID := strings.TrimSpace(ctx.GetHeader(cid.Header))
		if requestID == "" || len(requestID) > 128 {
			requestID = cid.New()
		}
		requestContext := cid.WithContext(ctx.Request.Context(), requestID)
		requestContext = commonlogger.WithContext(requestContext, logger.With(zap.String("cid", requestID)))
		ctx.Request = ctx.Request.WithContext(requestContext)
		ctx.Header(cid.Header, requestID)
		ctx.Next()
	}
}

func Recovery() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		defer func() {
			if recovered := recover(); recovered != nil {
				commonlogger.FromContext(ctx.Request.Context()).Error("request panic", zap.Any("panic", recovered), zap.String("route", ctx.FullPath()))
				ctx.Abort()
				WriteError(ctx, apperror.NewInternal("An unexpected error occurred", nil))
			}
		}()
		ctx.Next()
	}
}

func BodyLimit(maximumBytes int64) gin.HandlerFunc {
	if maximumBytes < 1 {
		maximumBytes = 10 * 1024 * 1024
	}
	return func(ctx *gin.Context) {
		if !requestMayHaveJSONBody(ctx.Request.Method) || ctx.Request.ContentLength == 0 {
			ctx.Next()
			return
		}
		if ctx.Request.ContentLength > maximumBytes {
			ctx.Abort()
			WriteError(ctx, apperror.NewRequestTooLarge("Request body is too large", nil))
			return
		}
		mediaType, _, err := mime.ParseMediaType(ctx.GetHeader("Content-Type"))
		if err != nil || !jsonMediaType(mediaType) {
			ctx.Abort()
			WriteError(ctx, apperror.NewUnsupportedMediaType("Content-Type must be application/json", err))
			return
		}
		ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, maximumBytes)
		ctx.Next()
	}
}

func CORS(origins []string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(origins))
	allowAll := false
	for _, origin := range origins {
		origin = strings.TrimSpace(origin)
		if origin == "*" {
			allowAll = true
		} else if origin != "" {
			allowed[origin] = struct{}{}
		}
	}
	return func(ctx *gin.Context) {
		origin := ctx.GetHeader("Origin")
		_, accepted := allowed[origin]
		if allowAll && origin != "" {
			ctx.Header("Access-Control-Allow-Origin", "*")
		} else if accepted {
			ctx.Header("Access-Control-Allow-Origin", origin)
			ctx.Header("Vary", "Origin")
		}
		if allowAll || accepted {
			ctx.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			ctx.Header("Access-Control-Allow-Headers", "Content-Type, X-Correlation-ID")
			ctx.Header("Access-Control-Expose-Headers", "X-Correlation-ID")
		}
		if ctx.Request.Method == http.MethodOptions {
			if origin != "" && !allowAll && !accepted {
				ctx.AbortWithStatus(http.StatusForbidden)
				return
			}
			ctx.AbortWithStatus(http.StatusNoContent)
			return
		}
		ctx.Next()
	}
}

func requestMayHaveJSONBody(method string) bool {
	return method == http.MethodPost || method == http.MethodPut || method == http.MethodPatch || method == http.MethodDelete
}

func jsonMediaType(value string) bool {
	return value == "application/json" || strings.HasPrefix(value, "application/") && strings.HasSuffix(value, "+json")
}

func RequestLogger(logger *zap.Logger) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		startedAt := time.Now()
		defer func() {
			requestLogger := commonlogger.FromContext(ctx.Request.Context())
			route := ctx.FullPath()
			if route == "" {
				route = "unmatched"
			}
			requestLogger.Info("http request",
				zap.String("method", ctx.Request.Method),
				zap.String("route", route),
				zap.Int("status", ctx.Writer.Status()),
				zap.Int("bytes", ctx.Writer.Size()),
				zap.Duration("duration", time.Since(startedAt)),
				zap.String("client_ip", ctx.ClientIP()),
			)
		}()
		ctx.Next()
	}
}
