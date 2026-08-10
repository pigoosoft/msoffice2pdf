package service

import (
	"strings"
	"time"

	"msoffice2pdf/internal/auth"
	"msoffice2pdf/internal/domain"
	"msoffice2pdf/internal/repo"
)

type UserService struct {
	Repo        *repo.UserRepo
	JWTSecret   string
	TokenExpire time.Duration
}

func (s *UserService) MaskToken(t string) string {
	if len(t) > 4 {
		return "****" + t[len(t)-4:]
	}
	return "****"
}

func (s *UserService) Login(uid, pwd string) (string, *domain.User, error) {
	user, err := s.Repo.FindByUID(uid)
	if err != nil {
		return "", nil, err
	}
	if user == nil {
		return "", nil, ErrUnauthorized
	}
	if user.Status == domain.StatusFrozen {
		return "", nil, ErrForbidden
	}
	if user.PwdHash != auth.MD5Hash(pwd) {
		return "", nil, ErrUnauthorized
	}
	jwt, err := auth.SignJWT(user.UID, user.Role, s.JWTSecret, s.TokenExpire)
	if err != nil {
		return "", nil, err
	}
	return jwt, user, nil
}

func (s *UserService) CreateUser(uid, pwd string, role int8) (string, *domain.User, error) {
	uid = strings.TrimSpace(uid)
	if uid == "" || pwd == "" {
		return "", nil, ErrInvalidInput
	}
	existing, err := s.Repo.FindByUID(uid)
	if err != nil {
		return "", nil, err
	}
	if existing != nil {
		return "", nil, ErrConflict
	}
	apiToken, err := auth.GenerateAPIToken()
	if err != nil {
		return "", nil, err
	}
	user := &domain.User{
		UID:     uid,
		PwdHash: auth.MD5Hash(pwd),
		Token:   apiToken,
		Role:    role,
		Status:  domain.StatusNormal,
	}
	if err := s.Repo.Create(user); err != nil {
		return "", nil, err
	}
	return apiToken, user, nil
}

// ChangePassword verifies oldPwd then sets a new password for uid.
func (s *UserService) ChangePassword(uid, oldPwd, newPwd string) error {
	user, err := s.Repo.FindByUID(uid)
	if err != nil {
		return err
	}
	if user == nil {
		return ErrNotFound
	}
	if user.PwdHash != auth.MD5Hash(oldPwd) {
		return ErrUnauthorized
	}
	if strings.TrimSpace(newPwd) == "" {
		return ErrInvalidInput
	}
	user.PwdHash = auth.MD5Hash(newPwd)
	return s.Repo.Update(user)
}

func (s *UserService) UpdateUser(uid string, pwd *string, role *int8) (*domain.User, error) {
	user, err := s.Repo.FindByUID(uid)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrNotFound
	}
	if pwd != nil {
		if strings.TrimSpace(*pwd) == "" {
			return nil, ErrInvalidInput
		}
		user.PwdHash = auth.MD5Hash(*pwd)
	}
	if role != nil {
		user.Role = *role
	}
	if err := s.Repo.Update(user); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *UserService) DeleteUser(uid string) error {
	user, err := s.Repo.FindByUID(uid)
	if err != nil {
		return err
	}
	if user == nil {
		return ErrNotFound
	}
	return s.Repo.DeleteByUID(uid)
}

func (s *UserService) SetFrozen(uid string, frozen bool) (*domain.User, error) {
	user, err := s.Repo.FindByUID(uid)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrNotFound
	}
	if frozen {
		user.Status = domain.StatusFrozen
	} else {
		user.Status = domain.StatusNormal
	}
	if err := s.Repo.Update(user); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *UserService) ResetAPIToken(uid string) (string, *domain.User, error) {
	user, err := s.Repo.FindByUID(uid)
	if err != nil {
		return "", nil, err
	}
	if user == nil {
		return "", nil, ErrNotFound
	}
	apiToken, err := auth.GenerateAPIToken()
	if err != nil {
		return "", nil, err
	}
	user.Token = apiToken
	if err := s.Repo.Update(user); err != nil {
		return "", nil, err
	}
	return apiToken, user, nil
}

func (s *UserService) GetByUID(uid string) (*domain.User, error) {
	user, err := s.Repo.FindByUID(uid)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrNotFound
	}
	return user, nil
}

func (s *UserService) List(page, pageSize int) ([]domain.User, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	offset := (page - 1) * pageSize
	return s.Repo.List(offset, pageSize)
}
