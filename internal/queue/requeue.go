package queue

import (
	"log/slog"
	"time"

	"msoffice2pdf/internal/converter"
	"msoffice2pdf/internal/domain"
	"msoffice2pdf/internal/storage"
)

func (q *Queue) requeueLoop() {
	defer q.requeueWG.Done()
	q.requeueOnce()
	q.orphanSweepOnce()
	t := time.NewTicker(q.cfg.RequeueInterval)
	defer t.Stop()
	for {
		select {
		case <-q.ctx.Done():
			return
		case <-t.C:
			q.requeueOnce()
			q.orphanSweepOnce()
		}
	}
}

func (q *Queue) orphanSweepOnce() {
	alive, killed := converter.SweepConvertWorkers(q.cfg.OfficeTimeout + 2*time.Minute)
	if killed > 0 || alive == 0 {
		// OpenOffice runs inside queue workers (no convert-worker), so alive==0 is normal;
		// never kill soffice mid-conversion. Startup CleanupOrphansAtStart still uses full engines.
		converter.CleanupOrphanOfficeProcesses(converter.COMEngines(q.cfg.Engines))
	}
	converter.SweepTempSandboxes(q.cfg.OfficeTimeout + 2*time.Minute)
}

func (q *Queue) requeueOnce() {
	list, err := q.UploadRepo.ListForRequeue(q.cfg.QueueSize)
	if err != nil {
		slog.Error("requeue list failed", "err", err)
		return
	}
	for _, u := range list {
		user, err := q.UserRepo.FindByID(u.UserID)
		if err != nil || user == nil {
			slog.Warn("requeue skip: user missing", "upload_id", u.ID, "user_id", u.UserID)
			continue
		}
		src := storage.AbsPath(q.Storage.UploadDir, u.FilePath)
		dst := storage.AbsOutputPDF(q.Storage.OutputDir, user.UID, u.FileID)
		task := Task{
			UploadID: u.ID,
			FileID:   u.FileID,
			UserID:   u.UserID,
			UID:      user.UID,
			SrcPath:  src,
			DstPath:  dst,
		}
		if q.TryEnqueue(task) {
			// Only promote pending→queued. Never overwrite converting (Worker may already own it;
			// TryEnqueue returns true for in-flight ids without re-adding).
			if u.Status == domain.UploadStatusPending {
				_ = q.UploadRepo.UpdateStatus(u.ID, domain.UploadStatusQueued, "")
			}
		} else {
			break
		}
	}
}
