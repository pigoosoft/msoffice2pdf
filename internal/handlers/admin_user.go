package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"msoffice2pdf/internal/domain"
	"msoffice2pdf/internal/service"
)

type AdminUserHandler struct {
	Svc *service.UserService
}

type createUserRequest struct {
	UID  string `json:"uid"`
	Pwd  string `json:"pwd"`
	Role string `json:"role"`
}

type updateUserRequest struct {
	Pwd  *string `json:"pwd"`
	Role *string `json:"role"`
}

type freezeUserRequest struct {
	Frozen bool `json:"frozen"`
}

func parseRole(s string) (int8, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "user":
		return domain.RoleUser, nil
	case "admin":
		return domain.RoleAdmin, nil
	default:
		return 0, service.ErrInvalidInput
	}
}

func (h *AdminUserHandler) Create(c *gin.Context) {
	var req createUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, CodeBadRequest, "invalid input")
		return
	}

	role, err := parseRole(req.Role)
	if mapServiceError(c, err) {
		return
	}

	apiToken, user, err := h.Svc.CreateUser(req.UID, req.Pwd, role)
	if mapServiceError(c, err) {
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, 50001, "internal error")
		return
	}

	OK(c, toUserView(h.Svc, user, apiToken, false))
}

func (h *AdminUserHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	users, total, err := h.Svc.List(page, pageSize)
	if err != nil {
		Fail(c, http.StatusInternalServerError, 50001, "internal error")
		return
	}

	items := make([]UserView, 0, len(users))
	for i := range users {
		items = append(items, toUserView(h.Svc, &users[i], "", true))
	}

	OK(c, gin.H{
		"items":     items,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

func (h *AdminUserHandler) Get(c *gin.Context) {
	uid := c.Param("uid")
	user, err := h.Svc.GetByUID(uid)
	if mapServiceError(c, err) {
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, 50001, "internal error")
		return
	}

	OK(c, toUserView(h.Svc, user, "", true))
}

func (h *AdminUserHandler) Update(c *gin.Context) {
	uid := c.Param("uid")
	var req updateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, CodeBadRequest, "invalid input")
		return
	}

	var role *int8
	if req.Role != nil {
		r, err := parseRole(*req.Role)
		if mapServiceError(c, err) {
			return
		}
		role = &r
	}

	user, err := h.Svc.UpdateUser(uid, req.Pwd, role)
	if mapServiceError(c, err) {
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, 50001, "internal error")
		return
	}

	OK(c, toUserView(h.Svc, user, "", true))
}

func (h *AdminUserHandler) Delete(c *gin.Context) {
	uid := c.Param("uid")
	if err := h.Svc.DeleteUser(uid); mapServiceError(c, err) {
		return
	} else if err != nil {
		Fail(c, http.StatusInternalServerError, 50001, "internal error")
		return
	}
	OK(c, nil)
}

func (h *AdminUserHandler) Freeze(c *gin.Context) {
	uid := c.Param("uid")
	var req freezeUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, CodeBadRequest, "invalid input")
		return
	}

	user, err := h.Svc.SetFrozen(uid, req.Frozen)
	if mapServiceError(c, err) {
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, 50001, "internal error")
		return
	}

	OK(c, toUserView(h.Svc, user, "", true))
}

func (h *AdminUserHandler) ResetToken(c *gin.Context) {
	uid := c.Param("uid")
	apiToken, user, err := h.Svc.ResetAPIToken(uid)
	if mapServiceError(c, err) {
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, 50001, "internal error")
		return
	}

	OK(c, toUserView(h.Svc, user, apiToken, false))
}
