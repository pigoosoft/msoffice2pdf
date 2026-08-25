package handlers

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"msoffice2pdf/internal/domain"
	"msoffice2pdf/internal/service"
)

type UploadHandler struct {
	Svc *service.UploadService
}

type uploadView struct {
	FileID      string     `json:"fileid"`
	RequestID   string     `json:"request_id"`
	Filename    string     `json:"filename"`
	Status      string     `json:"status"`
	FinalStatus string     `json:"final_status,omitempty"`
	ErrorCode   string     `json:"error_code,omitempty"`
	ErrorMsg    string     `json:"error_msg,omitempty"`
	RetryCount  int        `json:"retry_count,omitempty"`
	FileSize    int64      `json:"file_size"`
	FilePath    string     `json:"file_path"`
	ArchiveDir  string     `json:"archive_dir,omitempty"`
	UploadTime  time.Time  `json:"upload_time"`
	FinishedAt  *time.Time `json:"finished_at,omitempty"`
}

func toUploadView(u *domain.Upload) uploadView {
	return uploadView{
		FileID:     u.FileID,
		RequestID:  u.RequestID,
		Filename:   u.OriginalName,
		Status:     u.Status,
		RetryCount: u.RetryCount,
		FileSize:   u.FileSize,
		FilePath:   u.FilePath,
		UploadTime: u.CreatedAt,
	}
}

func toUploadDetailView(d *service.UploadDetail) uploadView {
	return uploadView{
		FileID:      d.FileID,
		RequestID:   d.RequestID,
		Filename:    d.Filename,
		Status:      d.Status,
		FinalStatus: d.FinalStatus,
		ErrorCode:   d.ErrorCode,
		ErrorMsg:    d.ErrorMsg,
		RetryCount:  d.RetryCount,
		FileSize:    d.FileSize,
		FilePath:    d.FilePath,
		ArchiveDir:  d.ArchiveDir,
		UploadTime:  d.UploadTime,
		FinishedAt:  d.FinishedAt,
	}
}

func currentUserFromContext(c *gin.Context) (*domain.User, bool) {
	v, ok := c.Get(currentUserContextKey)
	if !ok {
		return nil, false
	}
	u, ok := v.(*domain.User)
	return u, ok
}

func (h *UploadHandler) Upload(c *gin.Context) {
	user, ok := currentUserFromContext(c)
	if !ok {
		Fail(c, http.StatusUnauthorized, CodeUnauthorized, "unauthorized")
		return
	}
	fh, err := c.FormFile("file")
	if err != nil {
		Fail(c, http.StatusBadRequest, CodeBadRequest, "invalid input")
		return
	}
	f, err := fh.Open()
	if err != nil {
		Fail(c, http.StatusBadRequest, CodeBadRequest, "invalid input")
		return
	}
	defer f.Close()

	wm := strings.TrimSpace(c.PostForm("watermark"))
	reqID := strings.TrimSpace(c.GetHeader("X-Request-ID"))
	docPass := pickDocPassword(c.GetHeader("X-Doc-Password"), c.PostForm("password"))
	rec, err := h.Svc.Upload(user, fh.Filename, fh.Size, f, wm, reqID, docPass)
	if err != nil {
		if mapServiceError(c, err) {
			return
		}
		Fail(c, http.StatusInternalServerError, 50001, "upload failed")
		return
	}
	if rec.RequestID != "" {
		c.Header("X-Request-ID", rec.RequestID)
	}
	slog.InfoContext(c.Request.Context(), "upload accepted",
		"action", "upload",
		"request_id", rec.RequestID,
		"filename", rec.OriginalName,
		"fileid", rec.FileID,
		"size", rec.FileSize,
	)
	OK(c, gin.H{
		"fileid":      rec.FileID,
		"request_id":  rec.RequestID,
		"filename":    rec.OriginalName,
		"status":      rec.Status,
		"upload_time": rec.CreatedAt,
	})
}

func (h *UploadHandler) Get(c *gin.Context) {
	user, ok := currentUserFromContext(c)
	if !ok {
		Fail(c, http.StatusUnauthorized, CodeUnauthorized, "unauthorized")
		return
	}
	rec, err := h.Svc.GetDetail(user, c.Param("fileid"))
	if err != nil {
		if mapServiceError(c, err) {
			return
		}
		Fail(c, http.StatusInternalServerError, 50001, "get failed")
		return
	}
	OK(c, toUploadDetailView(rec))
}

func (h *UploadHandler) Download(c *gin.Context) {
	user, ok := currentUserFromContext(c)
	if !ok {
		Fail(c, http.StatusUnauthorized, CodeUnauthorized, "unauthorized")
		return
	}
	abs, name, err := h.Svc.DownloadPath(user, c.Param("fileid"))
	if err != nil {
		if mapServiceError(c, err) {
			return
		}
		Fail(c, http.StatusInternalServerError, 50001, "download failed")
		return
	}
	c.FileAttachment(abs, name)
}

func (h *UploadHandler) Delete(c *gin.Context) {
	user, ok := currentUserFromContext(c)
	if !ok {
		Fail(c, http.StatusUnauthorized, CodeUnauthorized, "unauthorized")
		return
	}
	if err := h.Svc.Delete(user, c.Param("fileid")); err != nil {
		if mapServiceError(c, err) {
			return
		}
		Fail(c, http.StatusInternalServerError, 50001, "delete failed")
		return
	}
	OK(c, gin.H{"ok": true})
}

func (h *UploadHandler) ListMine(c *gin.Context) {
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
	views := make([]uploadView, 0, len(items))
	for i := range items {
		views = append(views, toUploadView(&items[i]))
	}
	OK(c, gin.H{"items": views, "total": total, "page": page, "page_size": pageSize})
}

func (h *UploadHandler) ListAdmin(c *gin.Context) {
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
	views := make([]uploadView, 0, len(items))
	for i := range items {
		views = append(views, toUploadView(&items[i]))
	}
	OK(c, gin.H{"items": views, "total": total, "page": page, "page_size": pageSize})
}

type uploadLimitsView struct {
	AllowedExts []string `json:"allowed_exts"`
	MaxSize     int64    `json:"max_size"`
}

func (h *UploadHandler) Limits(c *gin.Context) {
	if _, ok := currentUserFromContext(c); !ok {
		Fail(c, http.StatusUnauthorized, CodeUnauthorized, "unauthorized")
		return
	}
	OK(c, uploadLimitsView{
		AllowedExts: h.Svc.UploadCfg.AllowedExtsForClient(),
		MaxSize:     h.Svc.UploadCfg.MaxSizeBytes,
	})
}
