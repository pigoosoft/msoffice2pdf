package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Envelope struct {
	Code    int         `json:"code"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

func JSON(c *gin.Context, httpStatus int, code int, message string, data interface{}) {
	c.JSON(httpStatus, Envelope{Code: code, Message: message, Data: data})
}

func OK(c *gin.Context, data interface{}) {
	JSON(c, http.StatusOK, 0, "", data)
}

func Fail(c *gin.Context, httpStatus, code int, message string) {
	JSON(c, httpStatus, code, message, nil)
}
