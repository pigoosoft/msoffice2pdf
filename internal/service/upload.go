package service

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"msoffice2pdf/internal/config"
	"msoffice2pdf/internal/domain"
	"msoffice2pdf/internal/queue"
	"msoffice2pdf/internal/repo"
	"msoffice2pdf/internal/storage"
	"msoffice2pdf/internal/validate"
)

// Enqueuer is satisfied by *queue.Queue.
type Enqueuer interface {
	TryEnqueue(t queue.Task) bool
}

// UploadArchiver is implemented by CleanupService.
type UploadArchiver interface {
	ArchiveUpload(u *domain.Upload, finalStatus, errorCode, errorMsg, dest string)
}

type UploadService struct {
	Repo         *repo.UploadRepo
	HistoryRepo  *repo.UploadHistoryRepo
	UserRepo     *repo.UserRepo
	UploadCfg    config.UploadConfig
	ConverterCfg config.ConverterConfig
	Storage      config.StorageConfig
	Queue        Enqueuer
	Archiver     UploadArchiver
}

const maxRequestIDLen = 128

func (s *UploadService) Upload(user *domain.User, filename string, declaredSize int64, r io.Reader, watermarkText, requestID string) (*domain.Upload, error) {
	filename = filepath.Base(filename)
	if filename == "" || filename == "." {
		return nil, ErrInvalidInput
	}
	if !s.UploadCfg.ExtAllowed(filename) {
		return nil, ErrInvalidInput
	}
	if _, ok := s.ConverterCfg.EngineForFilename(filename); !ok {
		return nil, ErrExtEngineUnmapped
	}
	if declaredSize > 0 && declaredSize > s.UploadCfg.MaxSizeBytes {
		return nil, ErrInvalidInput
	}
	watermarkText = strings.TrimSpace(watermarkText)
	if len([]rune(watermarkText)) > 255 {
		return nil, ErrInvalidInput
	}
	requestID = strings.TrimSpace(requestID)
	if len(requestID) > maxRequestIDLen {
		return nil, ErrInvalidInput
	}

	tmp, err := os.CreateTemp("", "msoffice2pdf-upload-*")
	if err != nil {
		return nil, err
	}
	tmpPath := tmp.Name()
	defer func() {
		tmp.Close()
		os.Remove(tmpPath)
	}()

	limited := io.LimitReader(r, s.UploadCfg.MaxSizeBytes+1)
	size, err := io.Copy(tmp, limited)
	if err != nil {
		return nil, err
	}
	if size > s.UploadCfg.MaxSizeBytes {
		return nil, ErrInvalidInput
	}
	if err := tmp.Close(); err != nil {
		return nil, err
	}

	if err := validate.File(tmpPath, filename, s.UploadCfg); err != nil {
		if errors.Is(err, validate.ErrMagic) {
			return nil, ErrFileMagic
		}
		if errors.Is(err, validate.ErrStructure) {
			return nil, ErrFileStructure
		}
		return nil, err
	}

	f, err := os.Open(tmpPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	fileid := strings.ReplaceAll(uuid.NewString(), "-", "")
	relPath, storedName, size2, err := storage.SaveUpload(s.Storage.UploadDir, user.UID, fileid, filename, f)
	if err != nil {
		return nil, err
	}
	size = size2
	if size > s.UploadCfg.MaxSizeBytes {
		_ = os.Remove(storage.AbsPath(s.Storage.UploadDir, relPath))
		return nil, ErrInvalidInput
	}

	rec := &domain.Upload{
		FileID:        fileid,
		UserID:        user.ID,
		OriginalName:  filename,
		StoredName:    storedName,
		FilePath:      relPath,
		FileSize:      size,
		WatermarkText: watermarkText,
		RequestID:     requestID,
		Status:        domain.UploadStatusPending,
	}
	if err := s.Repo.Create(rec); err != nil {
		_ = os.Remove(storage.AbsPath(s.Storage.UploadDir, relPath))
		return nil, err
	}

	if s.Queue != nil {
		src := storage.AbsPath(s.Storage.UploadDir, rec.FilePath)
		dst := storage.AbsOutputPDF(s.Storage.OutputDir, user.UID, rec.FileID)
		if s.Queue.TryEnqueue(queue.Task{
			UploadID: rec.ID,
			FileID:   rec.FileID,
			UserID:   rec.UserID,
			UID:      user.UID,
			SrcPath:  src,
			DstPath:  dst,
		}) {
			// Only pending→queued. Do not overwrite converting/completed if Worker already advanced.
			n, err := s.Repo.UpdateStatusIf(rec.ID, []string{domain.UploadStatusPending}, domain.UploadStatusQueued, "")
			if err != nil {
				return nil, err
			}
			if n == 0 {
				if fresh, ferr := s.Repo.FindByID(rec.ID); ferr == nil && fresh != nil {
					*rec = *fresh
				}
			} else {
				rec.Status = domain.UploadStatusQueued
			}
		}
	}
	return rec, nil
}

func (s *UploadService) canAccess(viewer *domain.User, u *domain.Upload) error {
	if viewer.Role == domain.RoleAdmin {
		return nil
	}
	if viewer.ID != u.UserID {
		return ErrForbidden
	}
	return nil
}

func (s *UploadService) canAccessHistory(viewer *domain.User, h *domain.UploadHistory) error {
	if viewer.Role == domain.RoleAdmin {
		return nil
	}
	if viewer.ID != h.UserID {
		return ErrForbidden
	}
	return nil
}

// UploadDetail is the API snapshot for GET /api/upload/:fileid (live queue or history).
type UploadDetail struct {
	FileID      string
	RequestID   string
	Filename    string
	Status      string
	FinalStatus string
	ErrorCode   string
	ErrorMsg    string
	RetryCount  int
	FileSize    int64
	FilePath    string // empty when FromHistory
	ArchiveDir  string
	UploadTime  time.Time
	FinishedAt  *time.Time
	FromHistory bool
}

// Get returns a live upload row only (Delete / Office download).
func (s *UploadService) Get(viewer *domain.User, fileid string) (*domain.Upload, error) {
	u, err := s.Repo.FindByFileID(fileid)
	if err != nil {
		return nil, err
	}
	if u == nil {
		return nil, ErrNotFound
	}
	if err := s.canAccess(viewer, u); err != nil {
		return nil, err
	}
	return u, nil
}

// GetDetail returns live upload or upload_history terminal snapshot.
func (s *UploadService) GetDetail(viewer *domain.User, fileid string) (*UploadDetail, error) {
	u, err := s.Repo.FindByFileID(fileid)
	if err != nil {
		return nil, err
	}
	if u != nil {
		if err := s.canAccess(viewer, u); err != nil {
			return nil, err
		}
		return &UploadDetail{
			FileID:     u.FileID,
			RequestID:  u.RequestID,
			Filename:   u.OriginalName,
			Status:     u.Status,
			ErrorMsg:   u.ErrorMsg,
			RetryCount: u.RetryCount,
			FileSize:   u.FileSize,
			FilePath:   u.FilePath,
			UploadTime: u.CreatedAt,
		}, nil
	}

	if s.HistoryRepo == nil {
		return nil, ErrNotFound
	}
	h, err := s.HistoryRepo.FindByFileID(fileid)
	if err != nil {
		return nil, err
	}
	if h == nil {
		return nil, ErrNotFound
	}
	if err := s.canAccessHistory(viewer, h); err != nil {
		return nil, err
	}
	finished := h.FinishedAt
	return &UploadDetail{
		FileID:      h.FileID,
		RequestID:   h.RequestID,
		Filename:    h.OriginalName,
		Status:      h.FinalStatus,
		FinalStatus: h.FinalStatus,
		ErrorCode:   h.ErrorCode,
		ErrorMsg:    h.ErrorMsg,
		RetryCount:  h.RetryCount,
		FileSize:    h.FileSize,
		FilePath:    "",
		ArchiveDir:  h.ArchiveDir,
		UploadTime:  h.UploadedAt,
		FinishedAt:  &finished,
		FromHistory: true,
	}, nil
}

func (s *UploadService) DownloadPath(viewer *domain.User, fileid string) (absPath, originalName string, err error) {
	u, err := s.Get(viewer, fileid)
	if err != nil {
		return "", "", err
	}
	abs := storage.AbsPath(s.Storage.UploadDir, u.FilePath)
	if _, statErr := os.Stat(abs); statErr != nil {
		return "", "", fmt.Errorf("file missing on disk: %w", ErrNotFound)
	}
	return abs, u.OriginalName, nil
}

func (s *UploadService) Delete(viewer *domain.User, fileid string) error {
	u, err := s.Get(viewer, fileid)
	if err != nil {
		return err
	}
	if s.Archiver == nil {
		return fmt.Errorf("archiver not configured")
	}
	s.Archiver.ArchiveUpload(u, domain.UploadStatusDeleted, "", "", domain.ArchiveDirTrash)
	return nil
}

func (s *UploadService) ListMine(user *domain.User, page, pageSize int, status *string) ([]domain.Upload, int64, error) {
	page, pageSize = normalizePage(page, pageSize)
	return s.Repo.ListByUserID(user.ID, status, (page-1)*pageSize, pageSize)
}

func (s *UploadService) ListHistoryMine(user *domain.User, page, pageSize int, finalStatus *string) ([]domain.UploadHistory, int64, error) {
	page, pageSize = normalizePage(page, pageSize)
	return s.HistoryRepo.ListByUserID(user.ID, finalStatus, (page-1)*pageSize, pageSize)
}

func (s *UploadService) ListHistoryAdmin(page, pageSize int, uidFilter *string, finalStatus *string) ([]domain.UploadHistory, int64, error) {
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
	return s.HistoryRepo.ListAll(userID, finalStatus, (page-1)*pageSize, pageSize)
}

func (s *UploadService) ListAdmin(page, pageSize int, uidFilter *string, status *string) ([]domain.Upload, int64, error) {
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
	return s.Repo.ListAll(userID, status, (page-1)*pageSize, pageSize)
}

func normalizePage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}
