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
	"msoffice2pdf/internal/metrics"
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
//
// Concurrency:
//   - startStopMu serializes Start/Stop so only one transition runs at a time
//     (Stop during Starting waits for Start to finish, then stops).
//   - mu protects status and resource fields; Start/Stop release mu during I/O
//     so Status() can observe Starting and Stopping.
type Runtime struct {
	cfg *config.Config

	startStopMu sync.Mutex // serializes Start / Stop bodies
	mu          sync.Mutex // status + resource fields
	status      Status
	gdb         *gorm.DB
	queue       *queue.Queue
	cleanup     *service.CleanupService
	sampler     *metrics.Sampler
	srv         *server.Server
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
// Concurrent Start calls are serialized; a second Start that arrives while
// Starting waits, then no-ops if the first succeeded (Running).
func (rt *Runtime) Start() error {
	rt.startStopMu.Lock()
	defer rt.startStopMu.Unlock()

	rt.mu.Lock()
	if rt.status == Running || rt.status == Starting {
		rt.mu.Unlock()
		return nil
	}
	rt.status = Starting
	rt.mu.Unlock()

	if err := rt.startWork(); err != nil {
		rt.mu.Lock()
		srv, cleanup, sampler, q, gdb := rt.takeResourcesLocked()
		rt.mu.Unlock()
		_ = stopResources(srv, cleanup, sampler, q, gdb)

		rt.mu.Lock()
		rt.status = Failed
		rt.mu.Unlock()
		return err
	}

	rt.mu.Lock()
	rt.status = Running
	rt.mu.Unlock()
	return nil
}

// Stop shuts down HTTP, cleanup, queue, and closes the SQL DB.
// Safe if already Stopped. If Start is in progress, waits for it to finish
// (via startStopMu), then stops the resulting Running/Failed runtime.
func (rt *Runtime) Stop() error {
	rt.startStopMu.Lock()
	defer rt.startStopMu.Unlock()

	rt.mu.Lock()
	if rt.status == Stopped {
		rt.mu.Unlock()
		return nil
	}
	rt.status = Stopping
	srv, cleanup, sampler, q, gdb := rt.takeResourcesLocked()
	rt.mu.Unlock()

	err := stopResources(srv, cleanup, sampler, q, gdb)

	rt.mu.Lock()
	rt.status = Stopped
	rt.mu.Unlock()
	return err
}

func (rt *Runtime) startWork() error {
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
	rt.mu.Lock()
	rt.gdb = gdb
	rt.mu.Unlock()

	if err := storage.EnsureDirs(cfg.Storage); err != nil {
		return fmt.Errorf("ensure storage dirs failed: %w", err)
	}

	userRepo := &repo.UserRepo{DB: gdb}
	uploadRepo := &repo.UploadRepo{DB: gdb}
	pdfRepo := &repo.PdfRepo{DB: gdb}
	pdfLogRepo := &repo.PdfLogRepo{DB: gdb}
	historyRepo := &repo.UploadHistoryRepo{DB: gdb}
	expiredRepo := &repo.ExpiredUploadRepo{DB: gdb}
	sampleRepo := &repo.PressureSampleRepo{DB: gdb}

	cleanup := &service.CleanupService{
		DB:          gdb,
		Cfg:         cfg.Cleanup,
		Storage:     cfg.Storage,
		UploadRepo:  uploadRepo,
		HistoryRepo: historyRepo,
		PdfRepo:     pdfRepo,
		PdfLogRepo:  pdfLogRepo,
		UserRepo:    userRepo,
		SampleRepo:  sampleRepo,
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
	q.SetWatchDirs(cfg.Storage.UploadDir, cfg.Storage.OutputDir, cfg.Storage.TrashDir, cfg.Storage.ExpiredDir, cfg.Log.FileDir)
	q.Start()
	cleanup.Start()

	sampler := &metrics.Sampler{
		Interval: cfg.Cleanup.MetricsInterval,
		Queue:    q,
		Uploads:  uploadRepo,
		Samples:  sampleRepo,
	}
	sampler.Start()

	rt.mu.Lock()
	rt.queue = q
	rt.cleanup = cleanup
	rt.sampler = sampler
	rt.mu.Unlock()

	srv := server.New(server.Deps{
		DB:          gdb,
		Cfg:         cfg,
		Queue:       q,
		Cleanup:     cleanup,
		HistoryRepo: historyRepo,
		SampleRepo:  sampleRepo,
	})
	rt.mu.Lock()
	rt.srv = srv
	rt.mu.Unlock()
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

	rt.startStopMu.Lock()
	defer rt.startStopMu.Unlock()

	rt.mu.Lock()
	if rt.status != Running {
		rt.mu.Unlock()
		return
	}
	rt.status = Stopping
	srv, cleanup, sampler, q, gdb := rt.takeResourcesLocked()
	rt.mu.Unlock()

	_ = stopResources(srv, cleanup, sampler, q, gdb)

	rt.mu.Lock()
	rt.status = Failed
	rt.mu.Unlock()
}

// takeResourcesLocked clears and returns owned resources. Caller must hold mu.
func (rt *Runtime) takeResourcesLocked() (srv *server.Server, cleanup *service.CleanupService, sampler *metrics.Sampler, q *queue.Queue, gdb *gorm.DB) {
	srv = rt.srv
	cleanup = rt.cleanup
	sampler = rt.sampler
	q = rt.queue
	gdb = rt.gdb
	rt.srv = nil
	rt.cleanup = nil
	rt.sampler = nil
	rt.queue = nil
	rt.gdb = nil
	return srv, cleanup, sampler, q, gdb
}

func stopResources(srv *server.Server, cleanup *service.CleanupService, sampler *metrics.Sampler, q *queue.Queue, gdb *gorm.DB) error {
	var firstErr error

	if srv != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := srv.Shutdown(shutdownCtx); err != nil && firstErr == nil {
			firstErr = err
		}
		cancel()
	}

	if sampler != nil {
		sampler.Stop()
	}
	if cleanup != nil {
		cleanup.Stop()
	}
	if q != nil {
		q.Stop()
	}

	if gdb != nil {
		sqlDB, err := gdb.DB()
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
		} else if err := sqlDB.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	return firstErr
}

func extKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
