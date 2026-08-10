package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"msoffice2pdf/internal/applog"
	"msoffice2pdf/internal/config"
	"msoffice2pdf/internal/converter"
	"msoffice2pdf/internal/db"
	"msoffice2pdf/internal/queue"
	"msoffice2pdf/internal/repo"
	"msoffice2pdf/internal/server"
	"msoffice2pdf/internal/service"
	"msoffice2pdf/internal/storage"
	"msoffice2pdf/internal/version"
	"msoffice2pdf/internal/watermark"
)

func main() {
	configPath, args := parseGlobalConfig(os.Args[1:])

	if len(args) == 0 || args[0] == "serve" {
		runServe(configPath)
		return
	}

	if len(args) >= 1 && isVersionCommand(args[0]) {
		printVersion()
		return
	}

	if len(args) >= 2 && args[0] == "user" {
		if err := runUserCommand(configPath, args[1], args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if len(args) >= 1 && args[0] == "convert-worker" {
		if err := runConvertWorker(args[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(2)
		}
		return
	}

	printUsage()
	os.Exit(1)
}

func isVersionCommand(name string) bool {
	switch name {
	case "version", "--version", "-v":
		return true
	default:
		return false
	}
}

func printVersion() {
	fmt.Printf("%s\nversion %s\n", version.Copyright, version.Version)
}

func runServe(configPath string) {
	cfg, err := config.Load(configPath)
	if err != nil {
		slog.Error("load config failed", "err", err)
		os.Exit(1)
	}

	logCloser, err := applog.Init(cfg.Log)
	if err != nil {
		fmt.Fprintf(os.Stderr, "init logger failed: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = logCloser.Close() }()

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
		slog.Error("converter environment validation failed", "err", err, "engines", cfg.Converter.Engines)
		os.Exit(1)
	}
	converter.LogUnmappedAllowedExts(cfg.Upload.AllowedExts, cfg.Converter.ExtEngines)

	gdb, err := db.Open(cfg.Database)
	if err != nil {
		slog.Error("database init failed", "err", err)
		os.Exit(1)
	}

	if err := storage.EnsureDirs(cfg.Storage); err != nil {
		slog.Error("ensure storage dirs failed", "err", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

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

	defer func() {
		cleanup.Stop()
		q.Stop()
	}()

	srv := server.New(server.Deps{
		DB:          gdb,
		Cfg:         cfg,
		Queue:       q,
		Cleanup:     cleanup,
		HistoryRepo: historyRepo,
	})
	if err := srv.Run(ctx); err != nil {
		slog.Error("server stopped with error", "err", err)
		os.Exit(1)
	}
}

func extKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func parseGlobalConfig(args []string) (configPath string, remaining []string) {
	configPath = "config/config.yaml"
	remaining = make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "--config=") {
			configPath = strings.TrimPrefix(arg, "--config=")
			continue
		}
		if arg == "--config" && i+1 < len(args) {
			configPath = args[i+1]
			i++
			continue
		}
		remaining = append(remaining, arg)
	}
	return configPath, remaining
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `Usage:
  msoffice2pdf [--config=PATH]
  msoffice2pdf serve [--config=PATH]
  msoffice2pdf version
  msoffice2pdf user create-admin --uid=UID --pwd=PWD [--config=PATH]
  msoffice2pdf user create --uid=UID --pwd=PWD [--config=PATH]
  msoffice2pdf user update --uid=UID [--pwd=PWD] [--role=admin|user] [--config=PATH]
  msoffice2pdf user reset-token --uid=UID [--config=PATH]
  msoffice2pdf user deactivate --uid=UID [--config=PATH]
  msoffice2pdf user activate --uid=UID [--config=PATH]
  msoffice2pdf convert-worker --src=PATH --dst=PATH [--excel-page-fit=fit_width|auto]
`)
}
