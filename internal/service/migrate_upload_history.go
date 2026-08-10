package service

import (
	"log/slog"

	"msoffice2pdf/internal/domain"
	"msoffice2pdf/internal/repo"
)

// MigrateUploadHistoryOnce copies expired_upload + leftover terminal uploads into upload_history.
// Idempotent: skip when history already has upload_id or fileid.
func MigrateUploadHistoryOnce(
	cleanup *CleanupService,
	expiredRepo *repo.ExpiredUploadRepo,
	maxRetry int,
) {
	migrateExpiredRows(cleanup, expiredRepo)
	migrateTerminalUploads(cleanup, maxRetry)
}

func migrateExpiredRows(cleanup *CleanupService, expiredRepo *repo.ExpiredUploadRepo) {
	const batch = 100
	offset := 0
	for {
		list, err := expiredRepo.ListAll(offset, batch)
		if err != nil {
			slog.Error("migrate expired_upload list failed", "err", err)
			return
		}
		if len(list) == 0 {
			return
		}
		for _, eu := range list {
			ok, err := cleanup.HistoryRepo.ExistsByUploadID(eu.UploadID)
			if err != nil {
				slog.Error("migrate exists check failed", "upload_id", eu.UploadID, "err", err)
				continue
			}
			if ok {
				continue
			}
			ok, err = cleanup.HistoryRepo.ExistsByFileID(eu.FileID)
			if err != nil || ok {
				continue
			}
			final := domain.UploadStatusCompleted
			if eu.ErrorCode == domain.ErrRetryLimitExceeded {
				final = domain.UploadStatusFailed
			}
			uploadedAt := eu.ExpiredAt
			h := &domain.UploadHistory{
				UploadID:     eu.UploadID,
				FileID:       eu.FileID,
				UserID:       eu.UserID,
				OriginalName: eu.OriginalName,
				FinalStatus:  final,
				ErrorCode:    eu.ErrorCode,
				ErrorMsg:     eu.ErrorMsg,
				ArchiveDir:   domain.ArchiveDirExpired,
				MovedPath:    eu.MovedPath,
				UploadedAt:   uploadedAt,
				FinishedAt:   eu.ExpiredAt,
			}
			if err := cleanup.HistoryRepo.Create(h); err != nil {
				slog.Error("migrate expired_upload insert failed", "upload_id", eu.UploadID, "err", err)
			}
		}
		if len(list) < batch {
			return
		}
		offset += batch
	}
}

func migrateTerminalUploads(cleanup *CleanupService, maxRetry int) {
	const batch = 100
	for {
		list, err := cleanup.UploadRepo.ListTerminalForMigrate(batch)
		if err != nil {
			slog.Error("migrate terminal uploads list failed", "err", err)
			return
		}
		if len(list) == 0 {
			break
		}
		for i := range list {
			u := &list[i]
			ok, _ := cleanup.HistoryRepo.ExistsByUploadID(u.ID)
			if ok {
				_ = cleanup.UploadRepo.HardDelete(u)
				continue
			}
			dest := domain.ArchiveDirExpired
			final := u.Status
			if u.Status == domain.UploadStatusDeleted {
				dest = domain.ArchiveDirTrash
				final = domain.UploadStatusDeleted
			} else {
				final = domain.UploadStatusCompleted
			}
			cleanup.ArchiveUpload(u, final, "", "", dest)
		}
		if len(list) < batch {
			break
		}
	}

	for {
		list, err := cleanup.UploadRepo.ListRetryExceededForMigrate(maxRetry, batch)
		if err != nil {
			slog.Error("migrate retry-exceeded list failed", "err", err)
			return
		}
		if len(list) == 0 {
			return
		}
		for i := range list {
			u := &list[i]
			ok, _ := cleanup.HistoryRepo.ExistsByUploadID(u.ID)
			if ok {
				_ = cleanup.UploadRepo.HardDelete(u)
				continue
			}
			msg := u.ErrorMsg
			cleanup.ArchiveUpload(u, domain.UploadStatusFailed, domain.ErrRetryLimitExceeded, msg, domain.ArchiveDirExpired)
		}
		if len(list) < batch {
			return
		}
	}
}
