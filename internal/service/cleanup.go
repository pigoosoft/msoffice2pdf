package service

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"sync"
	"time"

	"gorm.io/gorm"

	"msoffice2pdf/internal/config"
	"msoffice2pdf/internal/domain"
	"msoffice2pdf/internal/repo"
	"msoffice2pdf/internal/storage"
)

const cleanupBatchSize = 100

type CleanupService struct {
	DB          *gorm.DB
	Cfg         config.CleanupConfig
	Storage     config.StorageConfig
	UploadRepo  *repo.UploadRepo
	HistoryRepo *repo.UploadHistoryRepo
	PdfRepo     *repo.PdfRepo
	PdfLogRepo  *repo.PdfLogRepo
	UserRepo    *repo.UserRepo

	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	stopOnce sync.Once
}

func (s *CleanupService) Start() {
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.wg.Add(1)
	go s.loop()
}

func (s *CleanupService) Stop() {
	s.stopOnce.Do(func() {
		if s.cancel != nil {
			s.cancel()
		}
		s.wg.Wait()
	})
}

func (s *CleanupService) loop() {
	defer s.wg.Done()
	s.runOnce()
	t := time.NewTicker(s.Cfg.Interval)
	defer t.Stop()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-t.C:
			s.runOnce()
		}
	}
}

func (s *CleanupService) runOnce() {
	s.expireHistory()
	s.expirePdfs()
}

func (s *CleanupService) expireHistory() {
	if !s.Cfg.HistoryTTLEnabled {
		return
	}
	before := time.Now().Add(-s.Cfg.HistoryTTL)
	list, err := s.HistoryRepo.ListForHistoryTTL(before, cleanupBatchSize)
	if err != nil {
		slog.Error("cleanup history list failed", "err", err)
		return
	}
	for i := range list {
		s.expireOneHistory(&list[i])
	}
}

func (s *CleanupService) expireOneHistory(h *domain.UploadHistory) {
	root := s.Storage.ExpiredDir
	if h.ArchiveDir == domain.ArchiveDirTrash {
		root = s.Storage.TrashDir
	}
	if h.MovedPath != "" {
		abs := storage.AbsPath(root, h.MovedPath)
		if _, err := os.Stat(abs); err != nil {
			if os.IsNotExist(err) {
				slog.Warn("cleanup history: file missing", "history_id", h.ID, "path", abs)
			} else {
				slog.Error("cleanup history: stat failed", "history_id", h.ID, "err", err)
				return
			}
		} else if err := os.Remove(abs); err != nil {
			slog.Error("cleanup history: remove failed", "history_id", h.ID, "err", err)
			return
		}
	}

	if s.Cfg.HistoryTTLDeleteRow {
		if err := s.HistoryRepo.SoftDelete(h.ID); err != nil {
			slog.Error("cleanup history: soft-delete failed", "history_id", h.ID, "err", err)
		}
		return
	}
	if err := s.HistoryRepo.ClearMovedPath(h.ID); err != nil {
		slog.Error("cleanup history: clear moved_path failed", "history_id", h.ID, "err", err)
	}
}

// ArchiveUpload moves the Office file to expired/ or trash/, inserts upload_history, hard-deletes upload.
// dest must be domain.ArchiveDirExpired or domain.ArchiveDirTrash.
// On missing source: WARN, moved_path="", still write history and hard-delete.
func (s *CleanupService) ArchiveUpload(u *domain.Upload, finalStatus, errorCode, errorMsg, dest string) {
	user, err := s.UserRepo.FindByID(u.UserID)
	if err != nil || user == nil {
		slog.Error("archive upload: user missing", "upload_id", u.ID, "user_id", u.UserID, "err", err)
		return
	}

	movedPath := ""
	var moveErr error
	switch dest {
	case domain.ArchiveDirExpired:
		movedPath, moveErr = storage.MoveToExpired(
			s.Storage.UploadDir, s.Storage.ExpiredDir,
			user.UID, u.FileID, u.OriginalName, u.FilePath,
		)
	case domain.ArchiveDirTrash:
		movedPath, moveErr = storage.MoveToTrash(
			s.Storage.UploadDir, s.Storage.TrashDir,
			user.UID, u.FileID, u.OriginalName, u.FilePath,
		)
	default:
		slog.Error("archive upload: bad dest", "upload_id", u.ID, "dest", dest)
		return
	}
	if moveErr != nil {
		if errors.Is(moveErr, os.ErrNotExist) {
			slog.Warn("archive upload: source missing, finishing DB", "upload_id", u.ID, "fileid", u.FileID, "err", moveErr)
			movedPath = ""
		} else {
			slog.Error("archive upload: move failed", "upload_id", u.ID, "fileid", u.FileID, "err", moveErr)
			return
		}
	}

	now := time.Now()
	err = s.DB.Transaction(func(tx *gorm.DB) error {
		h := &domain.UploadHistory{
			UploadID:      u.ID,
			FileID:        u.FileID,
			UserID:        u.UserID,
			OriginalName:  u.OriginalName,
			StoredName:    u.StoredName,
			FileSize:      u.FileSize,
			FinalStatus:   finalStatus,
			ErrorCode:     errorCode,
			ErrorMsg:      errorMsg,
			RetryCount:    u.RetryCount,
			RequestID:     u.RequestID,
			WatermarkText: u.WatermarkText,
			ArchiveDir:    dest,
			MovedPath:     movedPath,
			UploadedAt:    u.CreatedAt,
			FinishedAt:    now,
		}
		if err := tx.Create(h).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Delete(u).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		slog.Error("archive upload: db failed", "upload_id", u.ID, "fileid", u.FileID, "err", err)
	}
}

func (s *CleanupService) expirePdfs() {
	before := time.Now().Add(-s.Cfg.PdfTTL)
	list, err := s.PdfRepo.ListForPdfTTL(before, cleanupBatchSize)
	if err != nil {
		slog.Error("cleanup pdf list failed", "err", err)
		return
	}
	for i := range list {
		s.expireOnePdf(&list[i])
	}
}

func (s *CleanupService) expireOnePdf(p *domain.Pdf) {
	abs := storage.AbsPath(s.Storage.OutputDir, p.FilePath)
	if _, err := os.Stat(abs); err != nil {
		if os.IsNotExist(err) {
			slog.Warn("cleanup pdf: file missing, finishing DB", "pdf_id", p.ID, "path", abs)
		} else {
			slog.Error("cleanup pdf: stat failed", "pdf_id", p.ID, "err", err)
			return
		}
	} else if err := os.Remove(abs); err != nil {
		slog.Error("cleanup pdf: remove failed", "pdf_id", p.ID, "err", err)
		return
	}

	now := time.Now()
	if err := s.PdfRepo.MarkExpired(p.ID, now); err != nil {
		slog.Error("cleanup pdf: mark expired failed", "pdf_id", p.ID, "err", err)
		return
	}
	log := &domain.PdfLog{
		PdfID:  p.ID,
		FileID: p.FileID,
		Action: domain.PdfLogActionExpire,
		Detail: "ttl expired",
	}
	if err := s.PdfLogRepo.Create(log); err != nil {
		slog.Error("cleanup pdf: pdflog failed", "pdf_id", p.ID, "err", err)
	}
}
