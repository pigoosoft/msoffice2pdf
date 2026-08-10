package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"msoffice2pdf/internal/domain"
	"msoffice2pdf/internal/repo"
	"msoffice2pdf/internal/service"
)

type HistoryHandler struct {
	UploadSvc *service.UploadService
	PdfSvc    *service.PdfService
}

type uploadHistoryView struct {
	FileID      string    `json:"fileid"`
	RequestID   string    `json:"request_id"`
	Filename    string    `json:"filename"`
	FinalStatus string    `json:"final_status"`
	Status      string    `json:"status"` // same as final_status for older clients
	ErrorCode   string    `json:"error_code"`
	ErrorMsg    string    `json:"error_msg"`
	RetryCount  int       `json:"retry_count"`
	FileSize    int64     `json:"file_size"`
	ArchiveDir  string    `json:"archive_dir"`
	UploadTime  time.Time `json:"upload_time"`
	FinishedAt  time.Time `json:"finished_at"`
}

func toUploadHistoryView(h *domain.UploadHistory) uploadHistoryView {
	return uploadHistoryView{
		FileID:      h.FileID,
		RequestID:   h.RequestID,
		Filename:    h.OriginalName,
		FinalStatus: h.FinalStatus,
		Status:      h.FinalStatus,
		ErrorCode:   h.ErrorCode,
		ErrorMsg:    h.ErrorMsg,
		RetryCount:  h.RetryCount,
		FileSize:    h.FileSize,
		ArchiveDir:  h.ArchiveDir,
		UploadTime:  h.UploadedAt,
		FinishedAt:  h.FinishedAt,
	}
}

type pdfLogView struct {
	ID        int64     `json:"id"`
	PdfID     int64     `json:"pdf_id"`
	FileID    string    `json:"fileid"`
	Action    string    `json:"action"`
	Detail    string    `json:"detail"`
	IPAddress string    `json:"ip_address"`
	UserAgent string    `json:"user_agent"`
	CreatedAt time.Time `json:"created_at"`
	UID       string    `json:"uid"`
}

func toPdfLogView(r repo.PdfLogRow) pdfLogView {
	return pdfLogView{
		ID:        r.ID,
		PdfID:     r.PdfID,
		FileID:    r.FileID,
		Action:    r.Action,
		Detail:    r.Detail,
		IPAddress: r.IPAddress,
		UserAgent: r.UserAgent,
		CreatedAt: r.CreatedAt,
		UID:       r.UID,
	}
}

func historyFinalStatusFilter(c *gin.Context) *string {
	if s := c.Query("final_status"); s != "" {
		return &s
	}
	if s := c.Query("status"); s != "" {
		return &s
	}
	return nil
}

func (h *HistoryHandler) UploadsMine(c *gin.Context) {
	user, ok := currentUserFromContext(c)
	if !ok {
		Fail(c, http.StatusUnauthorized, CodeUnauthorized, "unauthorized")
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	status := historyFinalStatusFilter(c)
	items, total, err := h.UploadSvc.ListHistoryMine(user, page, pageSize, status)
	if err != nil {
		if mapServiceError(c, err) {
			return
		}
		Fail(c, http.StatusInternalServerError, 50001, "list failed")
		return
	}
	views := make([]uploadHistoryView, 0, len(items))
	for i := range items {
		views = append(views, toUploadHistoryView(&items[i]))
	}
	OK(c, gin.H{"items": views, "total": total, "page": page, "page_size": pageSize})
}

func (h *HistoryHandler) UploadsAdmin(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	status := historyFinalStatusFilter(c)
	var uidFilter *string
	if u := c.Query("uid"); u != "" {
		uidFilter = &u
	}
	items, total, err := h.UploadSvc.ListHistoryAdmin(page, pageSize, uidFilter, status)
	if err != nil {
		if mapServiceError(c, err) {
			return
		}
		Fail(c, http.StatusInternalServerError, 50001, "list failed")
		return
	}
	views := make([]uploadHistoryView, 0, len(items))
	for i := range items {
		views = append(views, toUploadHistoryView(&items[i]))
	}
	OK(c, gin.H{"items": views, "total": total, "page": page, "page_size": pageSize})
}

func (h *HistoryHandler) PdfLogsMine(c *gin.Context) {
	user, ok := currentUserFromContext(c)
	if !ok {
		Fail(c, http.StatusUnauthorized, CodeUnauthorized, "unauthorized")
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	var fileIDFilter *string
	if f := strings.TrimSpace(c.Query("fileid")); f != "" {
		fileIDFilter = &f
	}
	items, total, err := h.PdfSvc.ListPdfLogsMine(user, page, pageSize, fileIDFilter)
	if err != nil {
		if mapServiceError(c, err) {
			return
		}
		Fail(c, http.StatusInternalServerError, 50001, "list failed")
		return
	}
	views := make([]pdfLogView, 0, len(items))
	for _, row := range items {
		views = append(views, toPdfLogView(row))
	}
	OK(c, gin.H{"items": views, "total": total, "page": page, "page_size": pageSize})
}

func (h *HistoryHandler) PdfLogsAdmin(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	var uidFilter *string
	if u := c.Query("uid"); u != "" {
		uidFilter = &u
	}
	var fileIDFilter *string
	if f := strings.TrimSpace(c.Query("fileid")); f != "" {
		fileIDFilter = &f
	}
	items, total, err := h.PdfSvc.ListPdfLogsAdmin(page, pageSize, uidFilter, fileIDFilter)
	if err != nil {
		if mapServiceError(c, err) {
			return
		}
		Fail(c, http.StatusInternalServerError, 50001, "list failed")
		return
	}
	views := make([]pdfLogView, 0, len(items))
	for _, row := range items {
		views = append(views, toPdfLogView(row))
	}
	OK(c, gin.H{"items": views, "total": total, "page": page, "page_size": pageSize})
}
