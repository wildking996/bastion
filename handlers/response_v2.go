package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type ResponseV2 struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

const (
	CodeOK             = "OK"
	CodeInvalidRequest = "INVALID_REQUEST"
	CodeNotFound       = "NOT_FOUND"
	CodeConflict       = "CONFLICT"
	CodeResourceBusy   = "RESOURCE_BUSY"
	CodeBadGateway     = "BAD_GATEWAY"
	CodeInternal       = "INTERNAL_ERROR"
)

func respondV2(c *gin.Context, code, message string, data any) {
	c.JSON(http.StatusOK, ResponseV2{Code: code, Message: message, Data: data})
}

func okV2(c *gin.Context, data any) {
	respondV2(c, CodeOK, "OK", data)
}

func errV2(c *gin.Context, code, message string, detail any) {
	// Keep the envelope stable: put free-form details into `data.detail`.
	payload := gin.H{}
	if detail != nil {
		payload["detail"] = detail
	}
	respondV2(c, code, message, payload)
}
