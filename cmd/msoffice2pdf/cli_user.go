package main

import (
	"errors"
	"fmt"
	"strings"

	"msoffice2pdf/internal/config"
	"msoffice2pdf/internal/db"
	"msoffice2pdf/internal/domain"
	"msoffice2pdf/internal/repo"
	"msoffice2pdf/internal/service"

	"gorm.io/gorm"
)

func runUserCommand(configPath, subcmd string, args []string) error {
	_, _, _, args = parseGlobalConfig(args)
	flags := parseFlags(args)

	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	gdb, err := openDB(cfg)
	if err != nil {
		return err
	}

	userSvc := &service.UserService{
		Repo:        &repo.UserRepo{DB: gdb},
		JWTSecret:   cfg.Auth.JWTSecret,
		TokenExpire: cfg.Auth.TokenExpire,
	}

	switch subcmd {
	case "create-admin":
		return cliCreateUser(userSvc, flags, domain.RoleAdmin)
	case "create":
		return cliCreateUser(userSvc, flags, domain.RoleUser)
	case "update":
		return cliUpdateUser(userSvc, flags)
	case "reset-token":
		return cliResetToken(userSvc, flags)
	case "deactivate":
		return cliSetFrozen(userSvc, flags, true)
	case "activate":
		return cliSetFrozen(userSvc, flags, false)
	default:
		printUsage()
		return fmt.Errorf("unknown user subcommand %q", subcmd)
	}
}

func openDB(cfg *config.Config) (*gorm.DB, error) {
	gdb, err := db.Open(cfg.Database)
	if err != nil {
		return nil, fmt.Errorf("database init: %w", err)
	}
	return gdb, nil
}

func parseFlags(args []string) map[string]string {
	flags := make(map[string]string)
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "--") {
			continue
		}
		keyVal := strings.TrimPrefix(arg, "--")
		if idx := strings.Index(keyVal, "="); idx >= 0 {
			flags[keyVal[:idx]] = keyVal[idx+1:]
			continue
		}
		key := keyVal
		if i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
			flags[key] = args[i+1]
			i++
		} else {
			flags[key] = ""
		}
	}
	return flags
}

func cliCreateUser(svc *service.UserService, flags map[string]string, role int8) error {
	uid := strings.TrimSpace(flags["uid"])
	pwd := flags["pwd"]
	if uid == "" || pwd == "" {
		return fmt.Errorf("--uid and --pwd are required")
	}

	apiToken, user, err := svc.CreateUser(uid, pwd, role)
	if err != nil {
		return mapServiceError(err)
	}

	fmt.Printf("uid=%s\n", user.UID)
	fmt.Printf("role=%s\n", domain.RoleName(user.Role))
	fmt.Printf("api_token=%s\n", apiToken)
	return nil
}

func cliUpdateUser(svc *service.UserService, flags map[string]string) error {
	uid := strings.TrimSpace(flags["uid"])
	if uid == "" {
		return fmt.Errorf("--uid is required")
	}

	_, hasPwd := flags["pwd"]
	_, hasRole := flags["role"]
	if !hasPwd && !hasRole {
		return fmt.Errorf("--pwd and/or --role is required")
	}

	var pwd *string
	if hasPwd {
		p := flags["pwd"]
		pwd = &p
	}

	var role *int8
	if hasRole {
		r, err := parseCLIRole(flags["role"])
		if err != nil {
			return err
		}
		role = &r
	}

	user, err := svc.UpdateUser(uid, pwd, role)
	if err != nil {
		return mapServiceError(err)
	}

	fmt.Printf("uid=%s\n", user.UID)
	fmt.Printf("role=%s\n", domain.RoleName(user.Role))
	return nil
}

func parseCLIRole(s string) (int8, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "user":
		return domain.RoleUser, nil
	case "admin":
		return domain.RoleAdmin, nil
	default:
		return 0, fmt.Errorf("invalid --role %q (want admin or user)", s)
	}
}

func cliResetToken(svc *service.UserService, flags map[string]string) error {
	uid := strings.TrimSpace(flags["uid"])
	if uid == "" {
		return fmt.Errorf("--uid is required")
	}

	apiToken, _, err := svc.ResetAPIToken(uid)
	if err != nil {
		return mapServiceError(err)
	}

	fmt.Printf("api_token=%s\n", apiToken)
	return nil
}

func cliSetFrozen(svc *service.UserService, flags map[string]string, frozen bool) error {
	uid := strings.TrimSpace(flags["uid"])
	if uid == "" {
		return fmt.Errorf("--uid is required")
	}

	_, err := svc.SetFrozen(uid, frozen)
	if err != nil {
		return mapServiceError(err)
	}
	return nil
}

func mapServiceError(err error) error {
	switch {
	case errors.Is(err, service.ErrNotFound):
		return fmt.Errorf("user not found")
	case errors.Is(err, service.ErrConflict):
		return fmt.Errorf("user already exists")
	case errors.Is(err, service.ErrInvalidInput):
		return fmt.Errorf("invalid input")
	default:
		return err
	}
}
