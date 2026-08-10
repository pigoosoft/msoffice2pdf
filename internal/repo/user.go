package repo

import (
	"errors"

	"gorm.io/gorm"

	"msoffice2pdf/internal/domain"
)

type UserRepo struct {
	DB *gorm.DB
}

func (r *UserRepo) Create(u *domain.User) error {
	return r.DB.Create(u).Error
}

func (r *UserRepo) FindByUID(uid string) (*domain.User, error) {
	var u domain.User
	err := r.DB.Where("uid = ?", uid).First(&u).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &u, err
}

func (r *UserRepo) FindByID(id int64) (*domain.User, error) {
	var u domain.User
	err := r.DB.First(&u, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &u, err
}

func (r *UserRepo) Update(u *domain.User) error {
	return r.DB.Save(u).Error
}

func (r *UserRepo) DeleteByUID(uid string) error {
	return r.DB.Where("uid = ?", uid).Delete(&domain.User{}).Error
}

func (r *UserRepo) List(offset, limit int) ([]domain.User, int64, error) {
	var total int64
	if err := r.DB.Model(&domain.User{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []domain.User
	err := r.DB.Order("id asc").Offset(offset).Limit(limit).Find(&list).Error
	return list, total, err
}
