package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"msoffice2pdf/internal/domain"
	"msoffice2pdf/internal/service"
)

const currentUserContextKey = "currentUser"

type AuthHandler struct {
	Svc         *service.UserService
	TokenExpire int
}

type loginRequest struct {
	UID string `json:"uid"`
	Pwd string `json:"pwd"`
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, CodeBadRequest, "invalid input")
		return
	}

	jwt, user, err := h.Svc.Login(req.UID, req.Pwd)
	if mapServiceError(c, err) {
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, 50001, "internal error")
		return
	}

	maxAge := h.TokenExpire
	if maxAge <= 0 {
		maxAge = int(h.Svc.TokenExpire.Seconds())
	}
	c.SetCookie("access_token", jwt, maxAge, "/", "", false, true)

	OK(c, gin.H{
		"uid":   user.UID,
		"token": jwt,
		"role":  domain.RoleName(user.Role),
	})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	c.SetCookie("access_token", "", -1, "/", "", false, true)
	OK(c, nil)
}

func (h *AuthHandler) Verify(c *gin.Context) {
	v, ok := c.Get(currentUserContextKey)
	if !ok {
		Fail(c, http.StatusUnauthorized, CodeUnauthorized, "unauthorized")
		return
	}
	user, ok := v.(*domain.User)
	if !ok {
		Fail(c, http.StatusUnauthorized, CodeUnauthorized, "unauthorized")
		return
	}
	OK(c, gin.H{
		"uid":    user.UID,
		"role":   domain.RoleName(user.Role),
		"status": user.Status,
	})
}
