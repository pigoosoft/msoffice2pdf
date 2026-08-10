package handlers

import (
	"time"

	"msoffice2pdf/internal/domain"
	"msoffice2pdf/internal/service"
)

type UserView struct {
	UID       string    `json:"uid"`
	Role      string    `json:"role"`
	Status    int8      `json:"status"`
	Token     string    `json:"token,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func toUserView(svc *service.UserService, u *domain.User, plainToken string, mask bool) UserView {
	token := plainToken
	if mask {
		token = svc.MaskToken(u.Token)
	}
	return UserView{
		UID:       u.UID,
		Role:      domain.RoleName(u.Role),
		Status:    u.Status,
		Token:     token,
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
	}
}
