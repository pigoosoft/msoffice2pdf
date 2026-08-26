package repo

import (
	"time"

	"gorm.io/gorm"

	"msoffice2pdf/internal/domain"
)

type PressureSampleRepo struct {
	DB *gorm.DB
}

func (r *PressureSampleRepo) Insert(s *domain.PressureSample) error {
	return r.DB.Create(s).Error
}

func (r *PressureSampleRepo) ListSince(since time.Time) ([]domain.PressureSample, error) {
	var list []domain.PressureSample
	err := r.DB.Where("sampled_at >= ?", since).Order("sampled_at asc").Find(&list).Error
	return list, err
}

func (r *PressureSampleRepo) DeleteOlderThan(before time.Time) (int64, error) {
	res := r.DB.Where("sampled_at < ?", before).Delete(&domain.PressureSample{})
	return res.RowsAffected, res.Error
}
