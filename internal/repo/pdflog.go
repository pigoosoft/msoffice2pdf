package repo

import (
	"strings"

	"gorm.io/gorm"

	"msoffice2pdf/internal/domain"
)

type PdfLogRepo struct {
	DB *gorm.DB
}

func (r *PdfLogRepo) Create(l *domain.PdfLog) error {
	return r.DB.Create(l).Error
}

type PdfLogRow struct {
	domain.PdfLog
	UID string `gorm:"column:uid"`
}

const pdflogListSelect = "pdflog.*, u.uid AS uid"

func (r *PdfLogRepo) userJoin() string {
	switch r.DB.Dialector.Name() {
	case "postgres":
		return `JOIN "user" AS u ON u.id = pdf.user_id`
	default:
		return "JOIN `user` AS u ON u.id = pdf.user_id"
	}
}

func (r *PdfLogRepo) pdflogListQuery() *gorm.DB {
	return r.DB.Table("pdflog").
		Joins("JOIN pdf ON pdf.id = pdflog.pdf_id").
		Joins(r.userJoin())
}

func applyFileIDFilter(q *gorm.DB, fileID *string) *gorm.DB {
	if fileID == nil || strings.TrimSpace(*fileID) == "" {
		return q
	}
	return q.Where("pdflog.fileid = ?", strings.TrimSpace(*fileID))
}

func (r *PdfLogRepo) ListByUserID(userID int64, fileID *string, offset, limit int) ([]PdfLogRow, int64, error) {
	q := applyFileIDFilter(r.pdflogListQuery().Where("pdf.user_id = ?", userID), fileID)

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var list []PdfLogRow
	err := q.Select(pdflogListSelect).
		Order("pdflog.id desc").
		Offset(offset).
		Limit(limit).
		Find(&list).Error
	return list, total, err
}

func (r *PdfLogRepo) ListAll(userID *int64, fileID *string, offset, limit int) ([]PdfLogRow, int64, error) {
	q := applyFileIDFilter(r.pdflogListQuery(), fileID)
	if userID != nil {
		q = q.Where("pdf.user_id = ?", *userID)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var list []PdfLogRow
	err := q.Select(pdflogListSelect).
		Order("pdflog.id desc").
		Offset(offset).
		Limit(limit).
		Find(&list).Error
	return list, total, err
}
