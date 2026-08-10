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
	"msoffice2pdf/internal/appruntime"
	"msoffice2pdf/internal/config"
	"msoffice2pdf/internal/version"
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

	rt := appruntime.New(cfg)
	if err := rt.Start(); err != nil {
		slog.Error("runtime start failed", "err", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()

	if err := rt.Stop(); err != nil {
		slog.Error("runtime stop failed", "err", err)
		os.Exit(1)
	}
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
