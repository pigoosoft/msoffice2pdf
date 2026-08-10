package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"msoffice2pdf/internal/domain"
	"msoffice2pdf/internal/repo"
	"msoffice2pdf/internal/service"
)

type ProfileHandler struct {
	UserSvc     *service.UserService
	UploadRepo  *repo.UploadRepo
	HistoryRepo *repo.UploadHistoryRepo
}

type changePasswordRequest struct {
	OldPwd string `json:"old_pwd"`
	NewPwd string `json:"new_pwd"`
}

func (h *ProfileHandler) Get(c *gin.Context) {
	user, ok := currentUserFromContext(c)
	if !ok {
		Fail(c, http.StatusUnauthorized, CodeUnauthorized, "unauthorized")
		return
	}

	liveUploads, err := h.UploadRepo.CountByUserID(user.ID)
	if err != nil {
		Fail(c, http.StatusInternalServerError, 50001, "internal error")
		return
	}
	histUploads, err := h.HistoryRepo.CountByUserID(user.ID)
	if err != nil {
		Fail(c, http.StatusInternalServerError, 50001, "internal error")
		return
	}
	liveOK, err := h.UploadRepo.CountByUserIDAndStatus(user.ID, domain.UploadStatusCompleted)
	if err != nil {
		Fail(c, http.StatusInternalServerError, 50001, "internal error")
		return
	}
	histOK, err := h.HistoryRepo.CountByUserIDAndFinalStatus(user.ID, domain.UploadStatusCompleted)
	if err != nil {
		Fail(c, http.StatusInternalServerError, 50001, "internal error")
		return
	}

	OK(c, gin.H{
		"uid":                   user.UID,
		"role":                  domain.RoleName(user.Role),
		"status":                user.Status,
		"token":                 user.Token,
		"upload_count":          liveUploads + histUploads,
		"convert_success_count": liveOK + histOK,
		"created_at":            user.CreatedAt,
		"updated_at":            user.UpdatedAt,
	})
}

func (h *ProfileHandler) ChangePassword(c *gin.Context) {
	user, ok := currentUserFromContext(c)
	if !ok {
		Fail(c, http.StatusUnauthorized, CodeUnauthorized, "unauthorized")
		return
	}

	var req changePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, CodeBadRequest, "invalid input")
		return
	}
	if strings.TrimSpace(req.OldPwd) == "" || strings.TrimSpace(req.NewPwd) == "" {
		Fail(c, http.StatusBadRequest, CodeBadRequest, "invalid input")
		return
	}

	if err := h.UserSvc.ChangePassword(user.UID, req.OldPwd, req.NewPwd); mapServiceError(c, err) {
		return
	} else if err != nil {
		Fail(c, http.StatusInternalServerError, 50001, "internal error")
		return
	}
	OK(c, nil)
}

func (h *ProfileHandler) ResetToken(c *gin.Context) {
	user, ok := currentUserFromContext(c)
	if !ok {
		Fail(c, http.StatusUnauthorized, CodeUnauthorized, "unauthorized")
		return
	}

	apiToken, updated, err := h.UserSvc.ResetAPIToken(user.UID)
	if mapServiceError(c, err) {
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, 50001, "internal error")
		return
	}

	OK(c, gin.H{
		"uid":    updated.UID,
		"role":   domain.RoleName(updated.Role),
		"status": updated.Status,
		"token":  apiToken,
	})
}
