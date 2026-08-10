package appruntime

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"gorm.io/gorm"

	"msoffice2pdf/internal/config"
	"msoffice2pdf/internal/converter"
	"msoffice2pdf/internal/db"
	"msoffice2pdf/internal/queue"
	"msoffice2pdf/internal/repo"
	"msoffice2pdf/internal/server"
	"msoffice2pdf/internal/service"
	"msoffice2pdf/internal/storage"
	"msoffice2pdf/internal/watermark"
)

// Status is the lifecycle state of a Runtime.
type Status int

const (
	Stopped Status = iota
	Starting
	Running
	Stopping
	Failed
)

func (s Status) String() string {
	switch s {
	case Stopped:
		return "Stopped"
	case Starting:
		return "Starting"
	case Running:
		return "Running"
	case Stopping:
		return "Stopping"
	case Failed:
		return "Failed"
	default:
		return fmt.Sprintf("Status(%d)", int(s))
	}
}

// Runtime owns the restartable service stack (DB, queue, cleanup, HTTP).
// Each successful Start allocates new Queue/CleanupService/Server instances;
// those types use sync.Once on Stop and must not be restarted in place.
type Runtime struct {
	cfg *config.Config

	mu      sync.Mutex
	status  Status
	gdb     *gorm.DB
	queue   *queue.Queue
	cleanup *service.CleanupService
	srv     *server.Server
}

// New returns a Runtime in Stopped state. cfg must already be loaded.
func New(cfg *config.Config) *Runtime {
	return &Runtime{
		cfg:    cfg,
		status: Stopped,
	}
}

// Status returns the current lifecycle status.
func (rt *Runtime) Status() Status {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	return rt.status
}

// Start brings up converter validation, DB, storage, queue, cleanup, and HTTP.
// Idempotent no-op if already Running or Starting. On failure sets Failed.
func (rt *Runtime) Start() error {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	if rt.status == Running || rt.status == Starting {
		return nil
	}

	rt.status = Starting
	if err := rt.startLocked(); err != nil {
		rt.cleanupPartialLocked()
		rt.status = Failed
		return err
	}
	rt.status = Running
	return nil
}

// Stop shuts down HTTP, cleanup, queue, and closes the SQL DB.
// Safe if already Stopped.
func (rt *Runtime) Stop() error {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	if rt.status == Stopped {
		return nil
	}

	rt.status = Stopping
	err := rt.stopLocked()
	rt.status = Stopped
	return err
}

func (rt *Runtime) startLocked() error {
	cfg := rt.cfg

	convOpts := converter.Options{
		ExcelPageFit:          cfg.Converter.ExcelPageFit,
		ComMode:               cfg.Converter.ComMode,
		TempSandbox:           cfg.Converter.TempSandboxEnabled(),
		Engines:               cfg.Converter.Engines,
		ExtEngines:            cfg.Converter.ExtEngines,
		ExtAppKinds:           converter.ExtAppKindsFromUpload(cfg.Upload.AppKind, extKeys(cfg.Converter.ExtEngines)),
		OpenOfficeCommand:     cfg.Converter.OpenOffice.Command,
		OpenOfficeUserProfile: cfg.Converter.OpenOffice.UserProfile,
	}
	if err := converter.ValidateEnvironment(convOpts); err != nil {
		return fmt.Errorf("converter environment validation failed: %w", err)
	}
	converter.LogUnmappedAllowedExts(cfg.Upload.AllowedExts, cfg.Converter.ExtEngines)

	gdb, err := db.Open(cfg.Database)
	if err != nil {
		return fmt.Errorf("database init failed: %w", err)
	}
	rt.gdb = gdb

	if err := storage.EnsureDirs(cfg.Storage); err != nil {
		return fmt.Errorf("ensure storage dirs failed: %w", err)
	}

	userRepo := &repo.UserRepo{DB: gdb}
	uploadRepo := &repo.UploadRepo{DB: gdb}
	pdfRepo := &repo.PdfRepo{DB: gdb}
	pdfLogRepo := &repo.PdfLogRepo{DB: gdb}
	historyRepo := &repo.UploadHistoryRepo{DB: gdb}
	expiredRepo := &repo.ExpiredUploadRepo{DB: gdb}

	cleanup := &service.CleanupService{
		DB:          gdb,
		Cfg:         cfg.Cleanup,
		Storage:     cfg.Storage,
		UploadRepo:  uploadRepo,
		HistoryRepo: historyRepo,
		PdfRepo:     pdfRepo,
		PdfLogRepo:  pdfLogRepo,
		UserRepo:    userRepo,
	}

	service.MigrateUploadHistoryOnce(cleanup, expiredRepo, cfg.Converter.RetryCount)
	service.MigrateUnifyFileIDOnce(pdfRepo, pdfLogRepo, uploadRepo, historyRepo)

	q := queue.New(
		cfg.Converter,
		cfg.Watermark,
		uploadRepo, pdfRepo, pdfLogRepo, userRepo,
		cfg.Storage,
		converter.New(convOpts),
		watermark.Service{},
		cleanup,
	)
	q.Start()
	cleanup.Start()
	rt.queue = q
	rt.cleanup = cleanup

	srv := server.New(server.Deps{
		DB:          gdb,
		Cfg:         cfg,
		Queue:       q,
		Cleanup:     cleanup,
		HistoryRepo: historyRepo,
	})
	rt.srv = srv
	errCh := srv.ListenAndServeBackground()
	go rt.watchListenErrors(errCh)

	return nil
}

func (rt *Runtime) watchListenErrors(errCh <-chan error) {
	err, ok := <-errCh
	if !ok || err == nil {
		return
	}
	slog.Error("http server failed", "err", err)

	rt.mu.Lock()
	defer rt.mu.Unlock()
	if rt.status != Running {
		return
	}
	_ = rt.stopLocked()
	rt.status = Failed
}

func (rt *Runtime) stopLocked() error {
	var firstErr error

	if rt.srv != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := rt.srv.Shutdown(shutdownCtx); err != nil && firstErr == nil {
			firstErr = err
		}
		cancel()
		rt.srv = nil
	}

	if rt.cleanup != nil {
		rt.cleanup.Stop()
		rt.cleanup = nil
	}
	if rt.queue != nil {
		rt.queue.Stop()
		rt.queue = nil
	}

	if rt.gdb != nil {
		sqlDB, err := rt.gdb.DB()
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
		} else if err := sqlDB.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		rt.gdb = nil
	}

	return firstErr
}

func (rt *Runtime) cleanupPartialLocked() {
	_ = rt.stopLocked()
}

func extKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
