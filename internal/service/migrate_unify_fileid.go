package service

import (
	"log/slog"

	"msoffice2pdf/internal/domain"
	"msoffice2pdf/internal/repo"
)

// MigrateUnifyFileIDOnce backfills pdf.fileid and pdflog.fileid to the unified
// upload-generated fileid. Idempotent.
func MigrateUnifyFileIDOnce(
	pdfRepo *repo.PdfRepo,
	pdfLogRepo *repo.PdfLogRepo,
	uploadRepo *repo.UploadRepo,
	historyRepo *repo.UploadHistoryRepo,
) {
	migratePdfFileIDs(pdfRepo, uploadRepo, historyRepo)
	migratePdfLogFileIDs(pdfLogRepo, pdfRepo)
}

func migratePdfFileIDs(pdfRepo *repo.PdfRepo, uploadRepo *repo.UploadRepo, historyRepo *repo.UploadHistoryRepo) {
	const batch = 500
	offset := 0
	for {
		list, _, err := pdfRepo.ListAll(nil, nil, offset, batch)
		if err != nil {
			slog.Error("migrate unify fileid: list pdf failed", "err", err)
			return
		}
		if len(list) == 0 {
			return
		}
		for i := range list {
			p := &list[i]
			want, resolveErr := resolveCanonicalFileID(uploadRepo, historyRepo, p.UploadID)
			if resolveErr != nil {
				slog.Error("migrate unify fileid: resolve failed", "pdf_id", p.ID, "upload_id", p.UploadID, "err", resolveErr)
				continue
			}
			if want == "" {
				slog.Error("migrate unify fileid: no upload/history fileid", "pdf_id", p.ID, "upload_id", p.UploadID)
				continue
			}
			if p.FileID == want {
				continue
			}
			// unique collision check
			existing, findErr := pdfRepo.FindByFileID(want)
			if findErr != nil {
				slog.Error("migrate unify fileid: find by fileid failed", "fileid", want, "err", findErr)
				continue
			}
			if existing != nil && existing.ID != p.ID {
				slog.Error("migrate unify fileid: unique conflict", "pdf_id", p.ID, "other_pdf_id", existing.ID, "fileid", want)
				continue
			}
			old := p.FileID
			p.FileID = want
			if err := pdfRepo.Update(p); err != nil {
				slog.Error("migrate unify fileid: update pdf failed", "pdf_id", p.ID, "err", err)
				p.FileID = old
				continue
			}
			slog.Info("migrate unify fileid: pdf updated", "pdf_id", p.ID, "fileid", want)
		}
		if len(list) < batch {
			return
		}
		offset += batch
	}
}

func resolveCanonicalFileID(uploadRepo *repo.UploadRepo, historyRepo *repo.UploadHistoryRepo, uploadID int64) (string, error) {
	u, err := uploadRepo.FindByID(uploadID)
	if err != nil {
		return "", err
	}
	if u != nil && u.FileID != "" {
		return u.FileID, nil
	}
	h, err := historyRepo.FindByUploadID(uploadID)
	if err != nil {
		return "", err
	}
	if h != nil {
		return h.FileID, nil
	}
	return "", nil
}

func migratePdfLogFileIDs(pdfLogRepo *repo.PdfLogRepo, pdfRepo *repo.PdfRepo) {
	db := pdfLogRepo.DB
	var logs []domain.PdfLog
	if err := db.Where("fileid = '' OR fileid IS NULL").Find(&logs).Error; err != nil {
		slog.Error("migrate unify fileid: list pdflog failed", "err", err)
		return
	}
	for i := range logs {
		l := &logs[i]
		p, err := pdfRepo.FindByID(l.PdfID)
		if err != nil {
			slog.Error("migrate unify fileid: load pdf for log failed", "pdflog_id", l.ID, "pdf_id", l.PdfID, "err", err)
			continue
		}
		if p == nil || p.FileID == "" {
			slog.Error("migrate unify fileid: pdflog missing pdf.fileid", "pdflog_id", l.ID, "pdf_id", l.PdfID)
			continue
		}
		if err := db.Model(l).Update("fileid", p.FileID).Error; err != nil {
			slog.Error("migrate unify fileid: update pdflog failed", "pdflog_id", l.ID, "err", err)
			continue
		}
	}
}
