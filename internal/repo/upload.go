package repo

import (
	"errors"
	"time"

	"gorm.io/gorm"

	"msoffice2pdf/internal/domain"
)

type UploadRepo struct {
	DB *gorm.DB
}

func (r *UploadRepo) Create(u *domain.Upload) error {
	return r.DB.Create(u).Error
}

func (r *UploadRepo) FindByFileID(fileid string) (*domain.Upload, error) {
	var u domain.Upload
	err := r.DB.Where("fileid = ?", fileid).First(&u).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &u, err
}

func (r *UploadRepo) CountByUserID(userID int64) (int64, error) {
	var n int64
	err := r.DB.Model(&domain.Upload{}).Where("user_id = ?", userID).Count(&n).Error
	return n, err
}

func (r *UploadRepo) CountByUserIDAndStatus(userID int64, status string) (int64, error) {
	var n int64
	err := r.DB.Model(&domain.Upload{}).Where("user_id = ? AND status = ?", userID, status).Count(&n).Error
	return n, err
}

func (r *UploadRepo) ListByUserID(userID int64, status *string, offset, limit int) ([]domain.Upload, int64, error) {
	q := r.DB.Model(&domain.Upload{}).Where("user_id = ?", userID)
	if status != nil && *status != "" {
		q = q.Where("status = ?", *status)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var list []domain.Upload
	err := q.Order("id desc").Offset(offset).Limit(limit).Find(&list).Error
	return list, total, err
}

func (r *UploadRepo) ListAll(userID *int64, status *string, offset, limit int) ([]domain.Upload, int64, error) {
	q := r.DB.Model(&domain.Upload{})
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

	var list []domain.Upload
	err := q.Order("id desc").Offset(offset).Limit(limit).Find(&list).Error
	return list, total, err
}

func (r *UploadRepo) UpdateStatus(id int64, status string, errorMsg string) error {
	updates := map[string]interface{}{"status": status, "error_msg": errorMsg}
	return r.DB.Model(&domain.Upload{}).Where("id = ?", id).Updates(updates).Error
}

func (r *UploadRepo) UpdateStatusOnly(id int64, status string) error {
	return r.DB.Model(&domain.Upload{}).Where("id = ?", id).Update("status", status).Error
}

// RecordFailure sets status=failed, stores errorMsg, bumps retry_count by 1, sets last_failed_at=now.
// Returns the new retry_count value.
func (r *UploadRepo) RecordFailure(id int64, errorMsg string) (newRetryCount int, err error) {
	now := time.Now()
	err = r.DB.Transaction(func(tx *gorm.DB) error {
		var u domain.Upload
		if err := tx.First(&u, id).Error; err != nil {
			return err
		}
		u.Status = domain.UploadStatusFailed
		u.ErrorMsg = errorMsg
		u.RetryCount++
		u.LastFailedAt = &now
		if err := tx.Model(&u).Updates(map[string]interface{}{
			"status":         u.Status,
			"error_msg":      u.ErrorMsg,
			"retry_count":    u.RetryCount,
			"last_failed_at": u.LastFailedAt,
		}).Error; err != nil {
			return err
		}
		newRetryCount = u.RetryCount
		return nil
	})
	return newRetryCount, err
}

// UpdateStatusIf sets status only when current status is in fromStatuses.
// Returns rows affected (0 if already advanced past those states).
func (r *UploadRepo) UpdateStatusIf(id int64, fromStatuses []string, status string, errorMsg string) (int64, error) {
	updates := map[string]interface{}{"status": status, "error_msg": errorMsg}
	res := r.DB.Model(&domain.Upload{}).Where("id = ? AND status IN ?", id, fromStatuses).Updates(updates)
	return res.RowsAffected, res.Error
}

// ListForRequeue: pending|queued|converting, not soft-deleted, ORDER BY id ASC
func (r *UploadRepo) ListForRequeue(limit int) ([]domain.Upload, error) {
	var list []domain.Upload
	err := r.DB.Where("status IN ?", []string{
		domain.UploadStatusPending,
		domain.UploadStatusQueued,
		domain.UploadStatusConverting,
	}).Order("id asc").Limit(limit).Find(&list).Error
	return list, err
}

// ListForRetry: status=failed, retry_count <= maxRetry, last_failed_at + interval elapsed, not soft-deleted.
func (r *UploadRepo) ListForRetry(maxRetry int, olderThan time.Time, limit int) ([]domain.Upload, error) {
	var list []domain.Upload
	err := r.DB.Where(
		"status = ? AND retry_count <= ? AND last_failed_at IS NOT NULL AND last_failed_at <= ?",
		domain.UploadStatusFailed, maxRetry, olderThan,
	).Order("id asc").Limit(limit).Find(&list).Error
	return list, err
}

func (r *UploadRepo) FindByID(id int64) (*domain.Upload, error) {
	var u domain.Upload
	err := r.DB.First(&u, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &u, err
}

func (r *UploadRepo) HardDelete(u *domain.Upload) error {
	return r.DB.Unscoped().Delete(u).Error
}

func (r *UploadRepo) ListTerminalForMigrate(limit int) ([]domain.Upload, error) {
	var list []domain.Upload
	err := r.DB.Unscoped().
		Where("status IN ?", []string{domain.UploadStatusCompleted, domain.UploadStatusDeleted}).
		Order("id asc").Limit(limit).Find(&list).Error
	return list, err
}

func (r *UploadRepo) ListRetryExceededForMigrate(maxRetry int, limit int) ([]domain.Upload, error) {
	var list []domain.Upload
	err := r.DB.Unscoped().
		Where("status = ? AND last_failed_at IS NOT NULL AND retry_count > ?", domain.UploadStatusFailed, maxRetry).
		Order("id asc").Limit(limit).Find(&list).Error
	return list, err
}
