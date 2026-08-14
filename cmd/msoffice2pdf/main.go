package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"

	"msoffice2pdf/internal/applog"
	"msoffice2pdf/internal/appruntime"
	"msoffice2pdf/internal/config"
	"msoffice2pdf/internal/consoleattach"
	"msoffice2pdf/internal/desktop"
	"msoffice2pdf/internal/singleinstance"
	"msoffice2pdf/internal/version"
)

func main() {
	configPath, noui, args := parseGlobalConfig(os.Args[1:])

	if len(args) >= 1 && isHelpCommand(args[0]) {
		consoleattach.EnsureCLI()
		printHelp(os.Stdout)
		return
	}

	if len(args) == 0 || args[0] == "serve" {
		runServe(configPath, noui)
		return
	}

	if len(args) >= 1 && isVersionCommand(args[0]) {
		consoleattach.EnsureCLI()
		printVersion()
		return
	}

	if len(args) >= 2 && args[0] == "user" {
		consoleattach.EnsureCLI()
		if err := runUserCommand(configPath, args[1], args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if len(args) >= 1 && args[0] == "convert-worker" {
		consoleattach.EnsureCLI()
		if err := runConvertWorker(args[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(2)
		}
		return
	}

	consoleattach.EnsureCLI()
	fmt.Fprintf(os.Stderr, "error: unknown command %q\n\n", args[0])
	printHelp(os.Stderr)
	os.Exit(1)
}

func isHelpCommand(name string) bool {
	switch name {
	case "help", "--help", "-h":
		return true
	default:
		return false
	}
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
	fmt.Printf("%s\n", version.AppName)
	fmt.Printf("Version:     %s\n", version.Version)
	fmt.Printf("Description: %s\n", version.Description)
	fmt.Printf("Copyright:   %s\n", version.Copyright)
}

func logStartupInfo() {
	slog.Info("msoffice2pdf starting",
		"app", version.AppName,
		"version", version.Version,
		"description", version.Description,
		"copyright", version.Copyright,
		"cli_help", "msoffice2pdf help",
	)
}

// exitError attaches a console on Windows GUI builds so the message is visible.
func exitError(format string, args ...any) {
	consoleattach.EnsureCLI()
	fmt.Fprintf(os.Stderr, format, args...)
	if !strings.HasSuffix(format, "\n") {
		fmt.Fprintln(os.Stderr)
	}
	os.Exit(1)
}

func runServe(configPath string, noui bool) {
	cfg, err := config.Load(configPath)
	if err != nil {
		exitError("error: load config failed: %v", err)
	}

	instLock, err := singleinstance.Acquire()
	if err != nil {
		exitError("error: %v", err)
	}
	defer func() { _ = instLock.Release() }()

	if err := singleinstance.CheckPortFree(cfg.Server.Port); err != nil {
		exitError("error: %v", err)
	}

	// --noui: skip desktop entirely; start HTTP + workers in console mode.
	if noui {
		consoleattach.EnsureCLI()
		runConsoleServe(cfg)
		return
	}

	wantUI := desktop.ShouldUseUI(false, runtime.GOOS, os.Getenv("DISPLAY"))
	if !wantUI {
		consoleattach.EnsureCLI()
		if runtime.GOOS == "linux" {
			fmt.Fprintln(os.Stderr, "no display; falling back to console mode")
		}
		runConsoleServe(cfg)
		return
	}

	ring := applog.NewRing(applog.RingCapDefault)
	logCloser, err := applog.Init(cfg.Log, ring.Handler(nil))
	if err != nil {
		exitError("error: init logger failed: %v", err)
	}
	defer func() { _ = logCloser.Close() }()

	logStartupInfo()

	rt := appruntime.New(cfg)
	consoleattach.DetachForUI()
	if err := desktop.Run(cfg, configPath, ring, rt); err != nil {
		consoleattach.EnsureCLI()
		slog.Error("desktop ui failed; falling back to console", "err", err)
		runRuntimeUntilSignal(rt)
	}
}

func runConsoleServe(cfg *config.Config) {
	logCloser, err := applog.Init(cfg.Log)
	if err != nil {
		exitError("error: init logger failed: %v", err)
	}
	defer func() { _ = logCloser.Close() }()

	logStartupInfo()

	rt := appruntime.New(cfg)
	runRuntimeUntilSignal(rt)
}

func runRuntimeUntilSignal(rt *appruntime.Runtime) {
	if err := rt.Start(); err != nil {
		consoleattach.EnsureCLI()
		slog.Error("runtime start failed", "err", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()

	if err := rt.Stop(); err != nil {
		consoleattach.EnsureCLI()
		slog.Error("runtime stop failed", "err", err)
		os.Exit(1)
	}
}

func parseGlobalConfig(args []string) (configPath string, noui bool, remaining []string) {
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
		if arg == "--noui" {
			noui = true
			continue
		}
		remaining = append(remaining, arg)
	}
	return configPath, noui, remaining
}

func printHelp(w *os.File) {
	fmt.Fprintf(w, `%s %s
%s
%s

Usage:
  msoffice2pdf [global flags] [command] [command flags]

Global flags:
  --config=PATH, --config PATH   Config file (default: config/config.yaml)
  --noui                         Skip desktop shell; start serve in console mode
  -h, --help, help               Show this help
  -v, --version, version         Show version and copyright

Commands:
  (none) / serve                 Start the service (desktop shell by default on Windows /
                                 Linux with DISPLAY; use --noui for console)
  help                           Show this help
  version                        Show version and copyright
  user create-admin              Create an admin user
      --uid=UID --pwd=PWD
  user create                    Create a normal user
      --uid=UID --pwd=PWD
  user update                    Update user password and/or role
      --uid=UID [--pwd=PWD] [--role=admin|user]
  user reset-token               Rotate API token for a user
      --uid=UID
  user deactivate                Freeze a user
      --uid=UID
  user activate                  Unfreeze a user
      --uid=UID
  convert-worker                 One-shot Office→PDF conversion worker (internal/COM helper)
      --src=PATH --dst=PATH [--excel-page-fit=fit_width|auto]

Examples:
  msoffice2pdf
  msoffice2pdf --noui --config config/config.yaml
  msoffice2pdf serve --noui
  msoffice2pdf version
  msoffice2pdf help
  msoffice2pdf user create-admin --uid=admin --pwd=secret
`, version.AppName, version.Version, version.Description, version.Copyright)
}

// printUsage keeps older call sites working (unknown user subcommand, etc.).
func printUsage() {
	printHelp(os.Stderr)
}
