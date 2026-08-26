package queue

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"strings"

	"msoffice2pdf/internal/applog"
	"msoffice2pdf/internal/converter"
	"msoffice2pdf/internal/domain"
	"msoffice2pdf/internal/storage"
	"msoffice2pdf/internal/watermark"
)

func (q *Queue) workerLoop(id int) {
	defer q.workerWG.Done()
	for t := range q.jobs {
		held := q.slots.acquire()
		q.process(t)
		if held {
			q.slots.release()
		}
		q.doneInflight(t.UploadID)
	}
	slog.Debug("converter worker stopped", "worker", id)
}

func convertLogCtx(uid string) context.Context {
	return applog.ContextWithUID(context.Background(), uid)
}

func convertLogArgs(t Task, upload *domain.Upload, extra ...any) []any {
	filename := ""
	requestID := ""
	if upload != nil {
		filename = upload.OriginalName
		requestID = upload.RequestID
	}
	args := []any{
		"action", "convert",
		"uid", t.UID,
		"request_id", requestID,
		"filename", filename,
		"fileid", t.FileID,
		"upload_id", t.UploadID,
	}
	return append(args, extra...)
}

func (q *Queue) engineForUpload(upload *domain.Upload) string {
	if upload == nil {
		return ""
	}
	if eng, ok := q.cfg.EngineForFilename(upload.OriginalName); ok {
		return eng
	}
	return ""
}

func generatePdfLogDetail(engine string) string {
	engine = strings.TrimSpace(engine)
	if engine == "" {
		return "ok"
	}
	return "ok; engine=" + engine
}

// failConvert marks pdf failed (if non-nil), records upload failure, archives if over retry limit.
func (q *Queue) failConvert(uploadID int64, pdf *domain.Pdf, msg string) {
	if pdf != nil {
		pdf.Status = domain.PdfStatusFailed
		_ = q.PdfRepo.Update(pdf)
	}
	n, err := q.UploadRepo.RecordFailure(uploadID, msg)
	if err != nil {
		slog.Error("record failure failed", "upload_id", uploadID, "err", err)
		return
	}
	if n <= q.cfg.RetryCount {
		slog.Warn("convert failed; will retry", "upload_id", uploadID, "retry_count", n, "max_extra", q.cfg.RetryCount, "err", msg)
		return
	}
	u, ferr := q.UploadRepo.FindByID(uploadID)
	if ferr != nil || u == nil {
		slog.Error("archive after retry: upload missing", "upload_id", uploadID, "err", ferr)
		return
	}
	if q.Cleanup == nil {
		slog.Error("archive after retry: cleanup not wired", "upload_id", uploadID)
		return
	}
	slog.Warn("retry limit exceeded; archiving upload", "upload_id", uploadID, "retry_count", n, "fileid", u.FileID)
	q.Cleanup.ArchiveUpload(u, domain.UploadStatusFailed, domain.ErrRetryLimitExceeded, msg, domain.ArchiveDirExpired)
	q.clearPassword(uploadID)
}

// failPassword marks pdf failed, records failure, and archives immediately (no retry).
func (q *Queue) failPassword(uploadID int64, pdf *domain.Pdf, code, msg string) {
	if pdf != nil {
		pdf.Status = domain.PdfStatusFailed
		_ = q.PdfRepo.Update(pdf)
	}
	if _, err := q.UploadRepo.RecordFailure(uploadID, msg); err != nil {
		slog.Error("record failure failed", "upload_id", uploadID, "err", err)
	}
	u, ferr := q.UploadRepo.FindByID(uploadID)
	if ferr != nil || u == nil {
		slog.Error("archive after password error: upload missing", "upload_id", uploadID, "err", ferr)
		q.clearPassword(uploadID)
		return
	}
	if q.Cleanup == nil {
		slog.Error("archive after password error: cleanup not wired", "upload_id", uploadID)
		q.clearPassword(uploadID)
		return
	}
	slog.Warn("password error; archiving upload", "upload_id", uploadID, "fileid", u.FileID, "err", msg)
	q.Cleanup.ArchiveUpload(u, domain.UploadStatusFailed, code, msg, domain.ArchiveDirExpired)
	q.clearPassword(uploadID)
}

func (q *Queue) process(t Task) {
	if _, err := q.UploadRepo.UpdateStatusIf(t.UploadID,
		[]string{domain.UploadStatusPending, domain.UploadStatusQueued, domain.UploadStatusConverting},
		domain.UploadStatusConverting, ""); err != nil {
		slog.Error("update converting status failed", "upload_id", t.UploadID, "err", err)
	}

	upload, err := q.UploadRepo.FindByID(t.UploadID)
	if err != nil || upload == nil {
		slog.Error("upload not found for convert", "upload_id", t.UploadID, "err", err)
		return
	}

	uid := t.UID
	if uid == "" {
		user, uerr := q.UserRepo.FindByID(t.UserID)
		if uerr != nil || user == nil {
			q.failConvert(t.UploadID, nil, "user not found")
			return
		}
		uid = user.UID
		t.UID = uid
		if t.DstPath == "" {
			t.DstPath = storage.AbsOutputPDF(q.Storage.OutputDir, uid, upload.FileID)
		}
	}

	logCtx := convertLogCtx(uid)
	engine := q.engineForUpload(upload)

	filename := storage.DownloadPDFName(upload.OriginalName)
	rel := storage.RelOutputPDF(uid, upload.FileID)
	t.DstPath = storage.AbsOutputPDF(q.Storage.OutputDir, uid, upload.FileID)

	existing, err := q.PdfRepo.FindByUploadID(t.UploadID)
	if err != nil {
		q.failConvert(t.UploadID, nil, err.Error())
		return
	}

	var pdf *domain.Pdf
	if existing != nil {
		if existing.FilePath != "" && existing.FilePath != rel {
			_ = storage.RemoveIfExists(storage.AbsPath(q.Storage.OutputDir, existing.FilePath))
		}
		pdf = existing
		pdf.Status = domain.PdfStatusGenerating
		pdf.FilePath = rel
		pdf.Filename = filename
		if err := q.PdfRepo.Update(pdf); err != nil {
			q.failConvert(t.UploadID, nil, err.Error())
			return
		}
	} else {
		pdf = &domain.Pdf{
			FileID:   t.FileID,
			UploadID: t.UploadID,
			UserID:   t.UserID,
			Filename: filename,
			FilePath: rel,
			Status:   domain.PdfStatusGenerating,
		}
		if err := q.PdfRepo.Create(pdf); err != nil {
			q.failConvert(t.UploadID, nil, err.Error())
			return
		}
	}

	if err := storage.EnsureUserOutputDir(q.Storage.OutputDir, uid); err != nil {
		q.failConvert(t.UploadID, pdf, err.Error())
		return
	}
	_ = storage.RemoveIfExists(t.DstPath)

	ctx, cancel := context.WithTimeout(context.Background(), q.cfg.OfficeTimeout)
	defer cancel()

	err = q.Converter.Convert(ctx, t.SrcPath, t.DstPath, t.DocPassword)
	if err != nil {
		_ = storage.RemoveIfExists(t.DstPath)
		if converter.IsPasswordError(err) {
			code := domain.ErrDocPasswordRequired
			if errors.Is(err, converter.ErrPasswordWrong) {
				code = domain.ErrDocPasswordWrong
			}
			q.failPassword(t.UploadID, pdf, code, code)
			slog.WarnContext(logCtx, "convert password error", convertLogArgs(t, upload, "engine", engine, "err", code)...)
			return
		}
		msg := err.Error()
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			msg = "timeout"
		}
		q.failConvert(t.UploadID, pdf, msg)
		slog.WarnContext(logCtx, "convert failed", convertLogArgs(t, upload, "engine", engine, "err", err)...)
		return
	}

	warnCode := ""
	primary := strings.TrimSpace(q.WatermarkCfg.Text)
	secondary := strings.TrimSpace(upload.WatermarkText)
	if q.Watermarker == nil || !watermark.Need(primary, secondary) {
		slog.DebugContext(logCtx, "watermark skipped", convertLogArgs(t, upload, "has_primary", primary != "", "has_secondary", secondary != "")...)
	} else {
		wmErr := q.Watermarker.Apply(ctx, t.DstPath, watermark.Options{
			Primary:      primary,
			Secondary:    secondary,
			Angle:        q.WatermarkCfg.Angle,
			Density:      q.WatermarkCfg.Density,
			DensityCount: q.WatermarkCfg.DensityCount,
			Opacity:      q.WatermarkCfg.Opacity,
			Color:        q.WatermarkCfg.Color,
			FontSize:     q.WatermarkCfg.FontSize,
			FontPath:     q.WatermarkCfg.FontPath,
		})
		if wmErr != nil {
			if errors.Is(wmErr, context.DeadlineExceeded) || errors.Is(wmErr, context.Canceled) {
				_ = storage.RemoveIfExists(t.DstPath)
				q.failConvert(t.UploadID, pdf, "timeout")
				slog.WarnContext(logCtx, "watermark timeout", convertLogArgs(t, upload, "err", wmErr)...)
				return
			}
			slog.WarnContext(logCtx, "watermark failed", convertLogArgs(t, upload, "err", wmErr)...)
			warnCode = domain.PdfWarnWatermark
		} else {
			slog.InfoContext(logCtx, "watermark applied", convertLogArgs(t, upload)...)
		}
	}

	fi, statErr := os.Stat(t.DstPath)
	if statErr != nil {
		q.failConvert(t.UploadID, pdf, statErr.Error())
		return
	}

	pdf.Status = domain.PdfStatusCompleted
	pdf.FileSize = fi.Size()
	pdf.WarnCode = warnCode
	if err := q.PdfRepo.Update(pdf); err != nil {
		q.failConvert(t.UploadID, pdf, err.Error())
		return
	}
	_ = q.PdfLogRepo.Create(&domain.PdfLog{
		PdfID:  pdf.ID,
		FileID: pdf.FileID,
		Action: domain.PdfLogActionGenerate,
		Detail: generatePdfLogDetail(engine),
	})
	u, ferr := q.UploadRepo.FindByID(t.UploadID)
	if ferr != nil || u == nil {
		slog.ErrorContext(logCtx, "archive after success: upload missing", convertLogArgs(t, upload, "engine", engine, "err", ferr)...)
		return
	}
	if q.Cleanup == nil {
		slog.ErrorContext(logCtx, "archive after success: cleanup not wired", convertLogArgs(t, upload, "engine", engine)...)
		return
	}
	q.Cleanup.ArchiveUpload(u, domain.UploadStatusCompleted, "", "", domain.ArchiveDirExpired)
	q.clearPassword(t.UploadID)
	slog.InfoContext(logCtx, "convert completed; archived", convertLogArgs(t, upload, "engine", engine)...)
}
