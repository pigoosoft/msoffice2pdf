package queue

import (
	"log/slog"
	"time"

	"msoffice2pdf/internal/domain"
	"msoffice2pdf/internal/storage"
)

func (q *Queue) retryLoop() {
	defer q.retryWG.Done()
	q.retryOnce()
	t := time.NewTicker(q.cfg.RetryInterval)
	defer t.Stop()
	for {
		select {
		case <-q.ctx.Done():
			return
		case <-t.C:
			q.retryOnce()
		}
	}
}

// isPasswordFailCode reports whether error_msg / error_code is a password failure
// that must never be re-enqueued (even if ArchiveUpload failed and the row remains).
func isPasswordFailCode(code string) bool {
	return code == domain.ErrDocPasswordRequired || code == domain.ErrDocPasswordWrong
}

func (q *Queue) retryOnce() {
	olderThan := time.Now().Add(-q.cfg.RetryInterval)
	list, err := q.UploadRepo.ListForRetry(q.cfg.RetryCount, olderThan, q.cfg.QueueSize)
	if err != nil {
		slog.Error("retry list failed", "err", err)
		return
	}
	for _, u := range list {
		if isPasswordFailCode(u.ErrorMsg) {
			slog.Warn("retry skip: password error", "upload_id", u.ID, "err", u.ErrorMsg)
			continue
		}
		user, err := q.UserRepo.FindByID(u.UserID)
		if err != nil || user == nil {
			slog.Warn("retry skip: user missing", "upload_id", u.ID, "user_id", u.UserID)
			continue
		}
		src := storage.AbsPath(q.Storage.UploadDir, u.FilePath)
		dst := storage.AbsOutputPDF(q.Storage.OutputDir, user.UID, u.FileID)
		task := Task{
			UploadID:    u.ID,
			FileID:      u.FileID,
			UserID:      u.UserID,
			UID:         user.UID,
			SrcPath:     src,
			DstPath:     dst,
			DocPassword: q.passwordFor(u.ID),
		}
		if q.TryEnqueue(task) {
			// Only promote failed→queued; never overwrite converting if Worker already claimed it.
			if u.Status == domain.UploadStatusFailed {
				_ = q.UploadRepo.UpdateStatusOnly(u.ID, domain.UploadStatusQueued)
			}
		} else {
			break
		}
	}
}
