package repo

import (
	"errors"
	"time"

	"gorm.io/gorm"

	"msoffice2pdf/internal/domain"
)

type PdfRepo struct {
	DB *gorm.DB
}

func (r *PdfRepo) Create(p *domain.Pdf) error {
	return r.DB.Create(p).Error
}

func (r *PdfRepo) FindByUploadID(uploadID int64) (*domain.Pdf, error) {
	var p domain.Pdf
	err := r.DB.Where("upload_id = ?", uploadID).Order("id desc").First(&p).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &p, err
}

func (r *PdfRepo) FindByFileID(fileID string) (*domain.Pdf, error) {
	var p domain.Pdf
	err := r.DB.Where("fileid = ?", fileID).First(&p).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &p, err
}

func (r *PdfRepo) FindByID(id int64) (*domain.Pdf, error) {
	var p domain.Pdf
	err := r.DB.First(&p, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &p, err
}

func (r *PdfRepo) Update(p *domain.Pdf) error {
	return r.DB.Save(p).Error
}

func (r *PdfRepo) ListByUserID(userID int64, status *string, offset, limit int) ([]domain.Pdf, int64, error) {
	q := r.DB.Model(&domain.Pdf{}).Where("user_id = ?", userID)
	if status != nil && *status != "" {
		q = q.Where("status = ?", *status)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var list []domain.Pdf
	err := q.Order("id desc").Offset(offset).Limit(limit).Find(&list).Error
	return list, total, err
}

func (r *PdfRepo) ListAll(userID *int64, status *string, offset, limit int) ([]domain.Pdf, int64, error) {
	q := r.DB.Model(&domain.Pdf{})
	if userID != nil {
		q = q.Where("user_id = ?", *userID)
	}
	if status != nil && *status != "" {
		q = q.Where("status = ?", *status)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var list []domain.Pdf
	err := q.Order("id desc").Offset(offset).Limit(limit).Find(&list).Error
	return list, total, err
}

// ListForPdfTTL: completed|failed, created_at < before, ORDER BY id ASC
func (r *PdfRepo) ListForPdfTTL(before time.Time, limit int) ([]domain.Pdf, error) {
	var list []domain.Pdf
	err := r.DB.Where("status IN ? AND created_at < ?", []string{
		domain.PdfStatusCompleted,
		domain.PdfStatusFailed,
	}, before).Order("id asc").Limit(limit).Find(&list).Error
	return list, err
}

func (r *PdfRepo) MarkExpired(id int64, expiredAt time.Time) error {
	return r.DB.Model(&domain.Pdf{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":     domain.PdfStatusExpired,
		"expired_at": expiredAt,
	}).Error
}
