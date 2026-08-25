package converter

import (
	"errors"
	"os"
	"strings"
)

const DocPasswordEnv = "MSOFFICE2PDF_DOC_PASSWORD"

var (
	ErrPasswordRequired = errors.New("ERR_DOC_PASSWORD_REQUIRED")
	ErrPasswordWrong    = errors.New("ERR_DOC_PASSWORD_WRONG")
)

func IsPasswordError(err error) bool {
	return errors.Is(err, ErrPasswordRequired) || errors.Is(err, ErrPasswordWrong)
}

func PasswordFromEnv() string {
	return os.Getenv(DocPasswordEnv)
}

func PasswordEnv(base []string, password string) []string {
	out := make([]string, 0, len(base)+1)
	prefix := DocPasswordEnv + "="
	for _, e := range base {
		if strings.HasPrefix(e, prefix) {
			continue
		}
		out = append(out, e)
	}
	if password != "" {
		out = append(out, prefix+password)
	}
	return out
}

func ParseWorkerPasswordError(msg string) error {
	switch {
	case strings.Contains(msg, ErrPasswordRequired.Error()):
		return ErrPasswordRequired
	case strings.Contains(msg, ErrPasswordWrong.Error()):
		return ErrPasswordWrong
	default:
		return nil
	}
}

func officeOpenLooksLikePassword(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	if strings.Contains(s, "password") {
		return true
	}
	// Known Office OLE HRESULTs that surface without the word "password".
	if strings.Contains(s, "800a15a5") {
		return true
	}
	return false
}

// mapOfficeOpenError maps a failed Documents/Workbooks/Presentations Open to a
// password sentinel. looksLikePassword is the OLE hint; when it cannot be
// decided the same mapping still applies (empty → Required, else Wrong).
// Returns the raw sentinels so Error() stays ERR_DOC_PASSWORD_*.
func mapOfficeOpenError(openErr error, password string, looksLikePassword bool) error {
	if openErr == nil {
		return nil
	}
	_ = looksLikePassword
	if password == "" {
		return ErrPasswordRequired
	}
	return ErrPasswordWrong
}
