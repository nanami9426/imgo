package utils

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

type Error struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Param   string `json:"param,omitempty"`
	Code    string `json:"code,omitempty"`
	Details string `json:"details,omitempty"`
}

type Response struct {
	Success bool   `json:"success"`
	Data    any    `json:"data,omitempty"`
	Error   *Error `json:"error,omitempty"`
}

func Success(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Response{
		Success: true,
		Data:    data,
	})
}

func SuccessMessage(c *gin.Context, message string) {
	Success(c, gin.H{"message": message})
}

func Fail(c *gin.Context, httpStatus int, code StatCode, message string, err error) {
	c.JSON(httpStatus, Response{
		Success: false,
		Error:   newError(code, message, err),
	})
}

func Abort(c *gin.Context, httpStatus int, code StatCode, message string, err error) {
	c.AbortWithStatusJSON(httpStatus, Response{
		Success: false,
		Error:   newError(code, message, err),
	})
}

func newError(code StatCode, message string, err error) *Error {
	if strings.TrimSpace(message) == "" {
		message = StatText(code)
	}
	apiErr := &Error{
		Message: message,
		Type:    errorType(code),
		Code:    strconv.Itoa(int(code)),
	}
	if err != nil {
		apiErr.Details = err.Error()
	}
	return apiErr
}

func errorType(code StatCode) string {
	switch code {
	case StatInvalidParam, StatNotFound, StatConflict:
		return "invalid_request_error"
	case StatUnauthorized:
		return "authentication_error"
	case StatForbidden:
		return "permission_error"
	case StatTooManyRequests:
		return "rate_limit_error"
	case StatInternalError, StatDatabaseError:
		return "server_error"
	default:
		return "unknown_error"
	}
}
