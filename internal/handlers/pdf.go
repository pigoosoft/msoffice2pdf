package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"msoffice2pdf/internal/domain"
	"msoffice2pdf/internal/service"
)

type PdfHandler struct {
	Svc *service.PdfService
}

type pdfView struct {
	FileID   string `json:"fileid"`
	UploadID int64  `json:"upload_id"`
	Filename string `json:"filename"`
	FilePath string `json:"file_path"`
	FileSize int64  `json:"file_size"`
	Status   string `json:"status"`
	WarnCode string `json:"warn_code"`
}

func toPdfView(p *domain.Pdf) pdfView {
	return pdfView{
		FileID:   p.FileID,
		UploadID: p.UploadID,
		Filename: p.Filename,
		FilePath: p.FilePath,
		FileSize: p.FileSize,
		Status:   p.Status,
		WarnCode: p.WarnCode,
	}
}

func (h *PdfHandler) Status(c *gin.Context) {
	user, ok := currentUserFromContext(c)
	if !ok {
		Fail(c, http.StatusUnauthorized, CodeUnauthorized, "unauthorized")
		return
	}
	data, err := h.Svc.Status(user, c.Param("fileid"))
	if err != nil {
		if mapServiceError(c, err) {
			return
		}
		Fail(c, http.StatusInternalServerError, 50001, "status failed")
		return
	}
	OK(c, data)
}

func (h *PdfHandler) Download(c *gin.Context) {
	user, ok := currentUserFromContext(c)
	if !ok {
		Fail(c, http.StatusUnauthorized, CodeUnauthorized, "unauthorized")
		return
	}
	abs, name, err := h.Svc.Download(user, c.Param("fileid"), c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		if mapServiceError(c, err) {
			return
		}
		Fail(c, http.StatusInternalServerError, 50001, "download failed")
		return
	}
	c.FileAttachment(abs, name)
}

func (h *PdfHandler) ListMine(c *gin.Context) {
	user, ok := currentUserFromContext(c)
	if !ok {
		Fail(c, http.StatusUnauthorized, CodeUnauthorized, "unauthorized")
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	var status *string
	if s := c.Query("status"); s != "" {
		status = &s
	}
	items, total, err := h.Svc.ListMine(user, page, pageSize, status)
	if err != nil {
		if mapServiceError(c, err) {
			return
		}
		Fail(c, http.StatusInternalServerError, 50001, "list failed")
		return
	}
	views := make([]pdfView, 0, len(items))
	for i := range items {
		views = append(views, toPdfView(&items[i]))
	}
	OK(c, gin.H{"items": views, "total": total, "page": page, "page_size": pageSize})
}

func (h *PdfHandler) ListAdmin(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	var status *string
	if s := c.Query("status"); s != "" {
		status = &s
	}
	var uidFilter *string
	if u := c.Query("uid"); u != "" {
		uidFilter = &u
	}
	items, total, err := h.Svc.ListAdmin(page, pageSize, uidFilter, status)
	if err != nil {
		if mapServiceError(c, err) {
			return
		}
		Fail(c, http.StatusInternalServerError, 50001, "list failed")
		return
	}
	views := make([]pdfView, 0, len(items))
	for i := range items {
		views = append(views, toPdfView(&items[i]))
	}
	OK(c, gin.H{"items": views, "total": total, "page": page, "page_size": pageSize})
}
