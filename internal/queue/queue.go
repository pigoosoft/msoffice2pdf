package queue

import (
	"context"
	"log/slog"
	"strings"
	"sync"

	"msoffice2pdf/internal/config"
	"msoffice2pdf/internal/converter"
	"msoffice2pdf/internal/domain"
	"msoffice2pdf/internal/repo"
	"msoffice2pdf/internal/watermark"
)

// UploadArchiver moves a terminal upload to expired/ or trash/ and writes upload_history.
// Implemented by *service.CleanupService; kept as interface to avoid queue↔service import cycle.
type UploadArchiver interface {
	ArchiveUpload(u *domain.Upload, finalStatus, errorCode, errorMsg, dest string)
}

// Queue owns the conversion channel, workers, requeue loop, and retry loop.
type Queue struct {
	jobs         chan Task
	cfg          config.ConverterConfig
	WatermarkCfg config.WatermarkConfig
	UploadRepo   *repo.UploadRepo
	PdfRepo      *repo.PdfRepo
	PdfLogRepo   *repo.PdfLogRepo
	UserRepo     *repo.UserRepo
	Storage      config.StorageConfig
	Converter    converter.Converter
	Watermarker  watermark.Watermarker
	Cleanup      UploadArchiver

	mu        sync.Mutex
	inflight  map[int64]struct{}
	closed    bool
	workerWG  sync.WaitGroup
	requeueWG sync.WaitGroup
	retryWG   sync.WaitGroup
	cancel    context.CancelFunc
	ctx       context.Context
	stopOnce  sync.Once
}

func New(
	cfg config.ConverterConfig,
	wmCfg config.WatermarkConfig,
	uploadRepo *repo.UploadRepo,
	pdfRepo *repo.PdfRepo,
	pdfLogRepo *repo.PdfLogRepo,
	userRepo *repo.UserRepo,
	storage config.StorageConfig,
	conv converter.Converter,
	wm watermark.Watermarker,
	cleanup UploadArchiver,
) *Queue {
	return &Queue{
		jobs:         make(chan Task, cfg.QueueSize),
		cfg:          cfg,
		WatermarkCfg: wmCfg,
		UploadRepo:   uploadRepo,
		PdfRepo:      pdfRepo,
		PdfLogRepo:   pdfLogRepo,
		UserRepo:     userRepo,
		Storage:      storage,
		Converter:    conv,
		Watermarker:  wm,
		Cleanup:      cleanup,
		inflight:     make(map[int64]struct{}),
	}
}

// TryEnqueue returns true if the task is accepted into the channel (or already in flight).
func (q *Queue) TryEnqueue(t Task) bool {
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		return false
	}
	if _, ok := q.inflight[t.UploadID]; ok {
		q.mu.Unlock()
		return true
	}
	q.mu.Unlock()

	select {
	case q.jobs <- t:
		q.mu.Lock()
		if !q.closed {
			q.inflight[t.UploadID] = struct{}{}
		}
		q.mu.Unlock()
		return true
	default:
		return false
	}
}

func (q *Queue) doneInflight(uploadID int64) {
	q.mu.Lock()
	delete(q.inflight, uploadID)
	q.mu.Unlock()
}

func (q *Queue) Start() {
	q.ctx, q.cancel = context.WithCancel(context.Background())
	// Clear leftovers from prior crashes before workers touch Office COM.
	converter.CleanupOrphansAtStart(q.cfg.Engines)
	primary := strings.TrimSpace(q.WatermarkCfg.Text)
	slog.Info("converter queue starting",
		"workers", q.cfg.WorkerCount,
		"watermark_primary", primary != "",
		"watermark_primary_runes", len([]rune(primary)),
		"retry_count", q.cfg.RetryCount,
		"retry_interval", q.cfg.RetryInterval,
	)
	for i := 0; i < q.cfg.WorkerCount; i++ {
		q.workerWG.Add(1)
		go q.workerLoop(i)
	}
	q.requeueWG.Add(1)
	go q.requeueLoop()
	q.retryWG.Add(1)
	go q.retryLoop()
}

// Stop cancels requeue/retry, closes the job channel, and waits for workers.
func (q *Queue) Stop() {
	q.stopOnce.Do(func() {
		if q.cancel != nil {
			q.cancel()
		}
		q.requeueWG.Wait()
		q.retryWG.Wait()
		q.mu.Lock()
		q.closed = true
		q.mu.Unlock()
		close(q.jobs)
		q.workerWG.Wait()
	})
}
