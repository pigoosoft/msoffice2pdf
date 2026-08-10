package repo

import (
	"gorm.io/gorm"

	"msoffice2pdf/internal/domain"
)

type ExpiredUploadRepo struct {
	DB *gorm.DB
}

func (r *ExpiredUploadRepo) Create(e *domain.ExpiredUpload) error {
	return r.DB.Create(e).Error
}

func (r *ExpiredUploadRepo) ListAll(offset, limit int) ([]domain.ExpiredUpload, error) {
	var list []domain.ExpiredUpload
	err := r.DB.Order("id asc").Offset(offset).Limit(limit).Find(&list).Error
	return list, err
}
