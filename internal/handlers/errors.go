package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"msoffice2pdf/internal/service"
)

const (
	CodeBadRequest         = 40001
	CodeFileMagic          = 40002
	CodeFileStructure      = 40003
	CodeExtEngineUnmapped  = 40004
	CodeUnauthorized       = 40101
	CodeForbidden          = 40301
	CodeNotFound           = 40401
	CodeConflict           = 40901
	CodeInternal           = 50001
)

func mapServiceError(c *gin.Context, err error) bool {
	switch {
	case errors.Is(err, service.ErrUnauthorized):
		Fail(c, http.StatusUnauthorized, CodeUnauthorized, "unauthorized")
	case errors.Is(err, service.ErrForbidden):
		Fail(c, http.StatusForbidden, CodeForbidden, "forbidden")
	case errors.Is(err, service.ErrNotFound):
		Fail(c, http.StatusNotFound, CodeNotFound, "not found")
	case errors.Is(err, service.ErrConflict):
		Fail(c, http.StatusConflict, CodeConflict, "conflict")
	case errors.Is(err, service.ErrFileMagic):
		Fail(c, http.StatusBadRequest, CodeFileMagic, "ERR_FILE_MAGIC")
	case errors.Is(err, service.ErrFileStructure):
		Fail(c, http.StatusBadRequest, CodeFileStructure, "ERR_FILE_STRUCTURE")
	case errors.Is(err, service.ErrExtEngineUnmapped):
		Fail(c, http.StatusBadRequest, CodeExtEngineUnmapped, "ERR_EXT_ENGINE_UNMAPPED")
	case errors.Is(err, service.ErrInvalidInput):
		Fail(c, http.StatusBadRequest, CodeBadRequest, "invalid input")
	default:
		return false
	}
	return true
}
