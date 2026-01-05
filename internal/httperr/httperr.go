package httperr

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type HTTPError struct {
	Code    string `json:"error_code"`
	Message string `json:"message"`
}

func Write(c *gin.Context, status int, code, message string) {
	c.JSON(status, HTTPError{
		Code:    code,
		Message: message,
	})
}

func BadRequest(c *gin.Context, code, message string) {
	Write(c, http.StatusBadRequest, code, message)
}

func NotFound(c *gin.Context, code, message string) {
	Write(c, http.StatusNotFound, code, message)
}

func Internal(c *gin.Context, code, message string) {
	Write(c, http.StatusInternalServerError, code, message)
}

func Unauthorized(c *gin.Context, code, message string) {
	Write(c, http.StatusUnauthorized, code, message)
}
