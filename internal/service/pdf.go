package service

import (
	"os"
	"strings"
	"time"

	"msoffice2pdf/internal/config"
	"msoffice2pdf/internal/domain"
	"msoffice2pdf/internal/repo"
	"msoffice2pdf/internal/storage"
)

type PdfService struct {
	UploadRepo  *repo.UploadRepo
	HistoryRepo *repo.UploadHistoryRepo
	PdfRepo     *repo.PdfRepo
	PdfLogRepo  *repo.PdfLogRepo
	UserRepo    *repo.UserRepo
	Storage     config.StorageConfig
}

func (s *PdfService) canAccessUpload(viewer *domain.User, u *domain.Upload) error {
	if viewer.Role == domain.RoleAdmin {
		return nil
	}
	if viewer.ID != u.UserID {
		return ErrForbidden
	}
	return nil
}

func (s *PdfService) canAccessHistory(viewer *domain.User, h *domain.UploadHistory) error {
	if viewer.Role == domain.RoleAdmin {
		return nil
	}
	if viewer.ID != h.UserID {
		return ErrForbidden
	}
	return nil
}

// Status returns conversion status keyed by the unified fileid.
func (s *PdfService) Status(viewer *domain.User, fileID string) (map[string]interface{}, error) {
	u, err := s.UploadRepo.FindByFileID(fileID)
	if err != nil {
		return nil, err
	}
	if u != nil {
		if err := s.canAccessUpload(viewer, u); err != nil {
			return nil, err
		}
		out := map[string]interface{}{
			"fileid":      u.FileID,
			"request_id":  u.RequestID,
			"status":      u.Status,
			"error_msg":   u.ErrorMsg,
			"retry_count": u.RetryCount,
			"created_at":  u.CreatedAt,
		}
		pdf, err := s.PdfRepo.FindByUploadID(u.ID)
		if err != nil {
			return nil, err
		}
		fillPdfFields(out, pdf, u.Status)
		return out, nil
	}

	h, err := s.HistoryRepo.FindByFileID(fileID)
	if err != nil {
		return nil, err
	}
	if h == nil {
		return nil, ErrNotFound
	}
	if err := s.canAccessHistory(viewer, h); err != nil {
		return nil, err
	}
	out := map[string]interface{}{
		"fileid":       h.FileID,
		"request_id":   h.RequestID,
		"status":       h.FinalStatus,
		"final_status": h.FinalStatus,
		"error_code":   h.ErrorCode,
		"error_msg":    h.ErrorMsg,
		"retry_count":  h.RetryCount,
		"created_at":   h.UploadedAt,
		"finished_at":  h.FinishedAt,
	}
	pdf, err := s.PdfRepo.FindByUploadID(h.UploadID)
	if err != nil {
		return nil, err
	}
	fillPdfFields(out, pdf, h.FinalStatus)
	return out, nil
}

func fillPdfFields(out map[string]interface{}, pdf *domain.Pdf, status string) {
	if pdf != nil {
		out["pdf_filename"] = pdf.Filename
		out["warn_code"] = pdf.WarnCode
		if status == domain.UploadStatusCompleted || status == domain.UploadStatusFailed {
			out["completed_at"] = pdf.UpdatedAt
		}
	} else {
		out["pdf_filename"] = ""
		out["warn_code"] = ""
	}
}

func (s *PdfService) Download(viewer *domain.User, fileID, ip, ua string) (absPath, filename string, err error) {
	var uploadID int64

	u, err := s.UploadRepo.FindByFileID(fileID)
	if err != nil {
		return "", "", err
	}
	if u != nil {
		if err := s.canAccessUpload(viewer, u); err != nil {
			return "", "", err
		}
		uploadID = u.ID
	} else {
		h, herr := s.HistoryRepo.FindByFileID(fileID)
		if herr != nil {
			return "", "", herr
		}
		if h == nil {
			return "", "", ErrNotFound
		}
		if err := s.canAccessHistory(viewer, h); err != nil {
			return "", "", err
		}
		uploadID = h.UploadID
	}

	pdf, err := s.PdfRepo.FindByUploadID(uploadID)
	if err != nil {
		return "", "", err
	}
	if pdf == nil || pdf.Status != domain.PdfStatusCompleted {
		return "", "", ErrConflict
	}

	abs := storage.AbsPath(s.Storage.OutputDir, pdf.FilePath)
	if _, statErr := os.Stat(abs); statErr != nil {
		return "", "", ErrNotFound
	}

	_ = s.PdfLogRepo.Create(&domain.PdfLog{
		PdfID:     pdf.ID,
		FileID:    pdf.FileID,
		Action:    domain.PdfLogActionDownload,
		Detail:    "download",
		IPAddress: ip,
		UserAgent: ua,
		CreatedAt: time.Now(),
	})
	return abs, pdf.Filename, nil
}

func (s *PdfService) ListMine(user *domain.User, page, pageSize int, status *string) ([]domain.Pdf, int64, error) {
	page, pageSize = normalizePage(page, pageSize)
	return s.PdfRepo.ListByUserID(user.ID, status, (page-1)*pageSize, pageSize)
}

func (s *PdfService) ListAdmin(page, pageSize int, uidFilter *string, status *string) ([]domain.Pdf, int64, error) {
	page, pageSize = normalizePage(page, pageSize)
	var userID *int64
	if uidFilter != nil && strings.TrimSpace(*uidFilter) != "" {
		u, err := s.UserRepo.FindByUID(strings.TrimSpace(*uidFilter))
		if err != nil {
			return nil, 0, err
		}
		if u == nil {
			return nil, 0, ErrNotFound
		}
		id := u.ID
		userID = &id
	}
	return s.PdfRepo.ListAll(userID, status, (page-1)*pageSize, pageSize)
}

func (s *PdfService) ListPdfLogsMine(user *domain.User, page, pageSize int, fileID *string) ([]repo.PdfLogRow, int64, error) {
	page, pageSize = normalizePage(page, pageSize)
	return s.PdfLogRepo.ListByUserID(user.ID, fileID, (page-1)*pageSize, pageSize)
}

func (s *PdfService) ListPdfLogsAdmin(page, pageSize int, uidFilter *string, fileID *string) ([]repo.PdfLogRow, int64, error) {
	page, pageSize = normalizePage(page, pageSize)
	var userID *int64
	if uidFilter != nil && strings.TrimSpace(*uidFilter) != "" {
		u, err := s.UserRepo.FindByUID(strings.TrimSpace(*uidFilter))
		if err != nil {
			return nil, 0, err
		}
		if u == nil {
			return nil, 0, ErrNotFound
		}
		id := u.ID
		userID = &id
	}
	return s.PdfLogRepo.ListAll(userID, fileID, (page-1)*pageSize, pageSize)
}
