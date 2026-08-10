package repo

import (
	"errors"
	"time"

	"gorm.io/gorm"

	"msoffice2pdf/internal/domain"
)

type UploadHistoryRepo struct {
	DB *gorm.DB
}

func (r *UploadHistoryRepo) Create(h *domain.UploadHistory) error {
	return r.DB.Create(h).Error
}

func (r *UploadHistoryRepo) ExistsByUploadID(uploadID int64) (bool, error) {
	var n int64
	err := r.DB.Model(&domain.UploadHistory{}).Where("upload_id = ?", uploadID).Limit(1).Count(&n).Error
	return n > 0, err
}

func (r *UploadHistoryRepo) ExistsByFileID(fileid string) (bool, error) {
	var n int64
	err := r.DB.Unscoped().Model(&domain.UploadHistory{}).Where("fileid = ?", fileid).Limit(1).Count(&n).Error
	return n > 0, err
}

func (r *UploadHistoryRepo) FindByFileID(fileid string) (*domain.UploadHistory, error) {
	var h domain.UploadHistory
	err := r.DB.Where("fileid = ?", fileid).First(&h).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &h, err
}

func (r *UploadHistoryRepo) FindByUploadID(uploadID int64) (*domain.UploadHistory, error) {
	var h domain.UploadHistory
	err := r.DB.Where("upload_id = ?", uploadID).Order("id desc").First(&h).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &h, err
}

func (r *UploadHistoryRepo) CountByUserID(userID int64) (int64, error) {
	var n int64
	err := r.DB.Model(&domain.UploadHistory{}).Where("user_id = ?", userID).Count(&n).Error
	return n, err
}

func (r *UploadHistoryRepo) CountByUserIDAndFinalStatus(userID int64, finalStatus string) (int64, error) {
	var n int64
	err := r.DB.Model(&domain.UploadHistory{}).
		Where("user_id = ? AND final_status = ?", userID, finalStatus).
		Count(&n).Error
	return n, err
}

func (r *UploadHistoryRepo) ListByUserID(userID int64, finalStatus *string, offset, limit int) ([]domain.UploadHistory, int64, error) {
	q := r.DB.Model(&domain.UploadHistory{}).Where("user_id = ?", userID)
	if finalStatus != nil && *finalStatus != "" {
		q = q.Where("final_status = ?", *finalStatus)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []domain.UploadHistory
	err := q.Order("id desc").Offset(offset).Limit(limit).Find(&list).Error
	return list, total, err
}

func (r *UploadHistoryRepo) ListAll(userID *int64, finalStatus *string, offset, limit int) ([]domain.UploadHistory, int64, error) {
	q := r.DB.Model(&domain.UploadHistory{})
	if userID != nil {
		q = q.Where("user_id = ?", *userID)
	}
	if finalStatus != nil && *finalStatus != "" {
		q = q.Where("final_status = ?", *finalStatus)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []domain.UploadHistory
	err := q.Order("id desc").Offset(offset).Limit(limit).Find(&list).Error
	return list, total, err
}

func (r *UploadHistoryRepo) ListForHistoryTTL(before time.Time, limit int) ([]domain.UploadHistory, error) {
	var list []domain.UploadHistory
	err := r.DB.Where("finished_at < ? AND moved_path != ''", before).
		Order("id asc").Limit(limit).Find(&list).Error
	return list, err
}

func (r *UploadHistoryRepo) ClearMovedPath(id int64) error {
	return r.DB.Model(&domain.UploadHistory{}).Where("id = ?", id).Update("moved_path", "").Error
}

func (r *UploadHistoryRepo) SoftDelete(id int64) error {
	return r.DB.Delete(&domain.UploadHistory{}, id).Error
}
