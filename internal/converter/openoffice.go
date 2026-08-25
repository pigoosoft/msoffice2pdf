package converter

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	openOfficeVersionTimeout = 15 * time.Second
	openOfficeMaxStderr      = 8 * 1024
)

type openOfficeEngine struct {
	command     string
	userProfile string
}

func (e *openOfficeEngine) Name() string { return EngineOpenOffice }

func (e *openOfficeEngine) ProcessImages() []string { return openOfficeProcessImages() }

func (e *openOfficeEngine) Validate() error {
	resolved, err := resolveOpenOfficeCommand(e.command)
	if err != nil {
		return err
	}
	if strings.TrimSpace(e.userProfile) == "" {
		return fmt.Errorf("engine openoffice: user_profile is empty")
	}
	if err := os.MkdirAll(e.userProfile, 0o755); err != nil {
		return fmt.Errorf("engine openoffice: create user_profile: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), openOfficeVersionTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, resolved, "--version")
	var stderr bytes.Buffer
	cmd.Stdout = io.Discard
	cmd.Stderr = &limitedBuffer{buf: &stderr, limit: openOfficeMaxStderr}
	if err := cmd.Run(); err != nil {
		snippet := strings.TrimSpace(stderr.String())
		if ctx.Err() != nil {
			return fmt.Errorf("engine openoffice: version check: %w", ctx.Err())
		}
		if snippet != "" {
			return fmt.Errorf("engine openoffice: version check: %w: %s", err, snippet)
		}
		return fmt.Errorf("engine openoffice: version check: %w", err)
	}
	return nil
}

func (e *openOfficeEngine) Convert(ctx context.Context, srcPath, dstPath, password string) error {
	resolved, err := resolveOpenOfficeCommand(e.command)
	if err != nil {
		return err
	}

	srcPath, err = filepath.Abs(srcPath)
	if err != nil {
		return fmt.Errorf("engine openoffice: abs src: %w", err)
	}
	dstPath, err = filepath.Abs(dstPath)
	if err != nil {
		return fmt.Errorf("engine openoffice: abs dst: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return fmt.Errorf("engine openoffice: create dst dir: %w", err)
	}
	_ = os.Remove(dstPath)

	taskDir := filepath.Join(e.userProfile, uuid.NewString())
	if err := os.MkdirAll(taskDir, 0o755); err != nil {
		return fmt.Errorf("engine openoffice: create task profile: %w", err)
	}
	defer func() { _ = os.RemoveAll(taskDir) }()

	taskDir, err = filepath.Abs(taskDir)
	if err != nil {
		return fmt.Errorf("engine openoffice: abs task profile: %w", err)
	}

	outdir := filepath.Join(taskDir, "out")
	if err := os.MkdirAll(outdir, 0o755); err != nil {
		return fmt.Errorf("engine openoffice: create outdir: %w", err)
	}

	uri := userInstallationURI(taskDir)
	args := []string{
		"--headless", "--nologo", "--nofirststartwizard", "--norestore",
		"-env:UserInstallation=" + uri,
	}
	if password != "" {
		args = append(args, "--password="+password)
	}
	args = append(args, "--convert-to", "pdf", "--outdir", outdir, srcPath)
	cmd := exec.Command(resolved, args...)
	var stderr bytes.Buffer
	cmd.Stdout = io.Discard
	cmd.Stderr = &limitedBuffer{buf: &stderr, limit: openOfficeMaxStderr}
	if err := runOpenOfficeCmd(ctx, cmd); err != nil {
		return mapOpenOfficeConvertError(err, password, ctx.Err())
	}

	pdf, err := findConvertedPDF(outdir, srcPath)
	if err != nil {
		if strings.Contains(err.Error(), "no pdf produced") {
			return mapOpenOfficeConvertError(err, password, nil)
		}
		return err
	}
	if err := moveFile(pdf, dstPath); err != nil {
		return fmt.Errorf("engine openoffice: move pdf: %w", err)
	}
	return nil
}

// mapOpenOfficeConvertError maps a failed soffice convert (process started and
// failed) to password sentinels. Headless stderr often has no "password" word,
// so password vs corruption cannot be distinguished: empty password →
// ErrPasswordRequired, non-empty → ErrPasswordWrong. ctx timeout/cancel is
// returned as-is (wrapped), never as a password sentinel.
func mapOpenOfficeConvertError(err error, password string, ctxErr error) error {
	if err == nil {
		return nil
	}
	if ctxErr != nil {
		return fmt.Errorf("engine openoffice: %w", ctxErr)
	}
	return mapOfficeOpenError(err, password, true)
}

func resolveOpenOfficeCommand(command string) (string, error) {
	command = strings.TrimSpace(command)
	if command == "" {
		return "", fmt.Errorf("engine openoffice: command is empty")
	}
	if filepath.IsAbs(command) || strings.ContainsAny(command, `/\`) {
		fi, err := os.Stat(command)
		if err != nil {
			return "", fmt.Errorf("engine openoffice: command %q: %w", command, err)
		}
		if fi.IsDir() {
			return "", fmt.Errorf("engine openoffice: command %q is a directory", command)
		}
		return command, nil
	}
	resolved, err := exec.LookPath(command)
	if err != nil {
		return "", fmt.Errorf("engine openoffice: look up %q: %w", command, err)
	}
	return resolved, nil
}

// userInstallationURI builds a file:/// URI for LibreOffice/OpenOffice -env:UserInstallation.
// Windows example: C:\a\b → file:///C:/a/b
func userInstallationURI(absDir string) string {
	p := filepath.ToSlash(absDir)
	if strings.HasPrefix(p, "/") {
		return "file://" + p
	}
	return "file:///" + p
}

// findConvertedPDF locates the PDF produced by soffice in outdir.
// Prefers {stem}.pdf from srcPath; otherwise accepts exactly one *.pdf.
func findConvertedPDF(outdir, srcPath string) (string, error) {
	base := filepath.Base(srcPath)
	stem := strings.TrimSuffix(base, filepath.Ext(base))
	preferred := filepath.Join(outdir, stem+".pdf")
	if fi, err := os.Stat(preferred); err == nil && !fi.IsDir() {
		return preferred, nil
	}

	matches, err := filepath.Glob(filepath.Join(outdir, "*.pdf"))
	if err != nil {
		return "", fmt.Errorf("engine openoffice: list pdf: %w", err)
	}
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return "", fmt.Errorf("engine openoffice: no pdf produced in %s", outdir)
	default:
		return "", fmt.Errorf("engine openoffice: multiple pdfs in %s", outdir)
	}
}

func moveFile(src, dst string) error {
	err := os.Rename(src, dst)
	if err == nil {
		return nil
	}
	if copyErr := copyFileOpenOffice(src, dst); copyErr != nil {
		return fmt.Errorf("rename: %w; copy: %w", err, copyErr)
	}
	if removeErr := os.Remove(src); removeErr != nil {
		return removeErr
	}
	return nil
}

func copyFileOpenOffice(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}

	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		_ = os.Remove(dst)
		return copyErr
	}
	if closeErr != nil {
		_ = os.Remove(dst)
		return closeErr
	}
	return nil
}

type limitedBuffer struct {
	buf   *bytes.Buffer
	limit int
}

func redactPasswordSnippet(snippet, password string) string {
	snippet = strings.TrimSpace(snippet)
	if password == "" || snippet == "" {
		return snippet
	}
	return strings.ReplaceAll(snippet, password, "")
}

func (l *limitedBuffer) Write(p []byte) (int, error) {
	remain := l.limit - l.buf.Len()
	if remain <= 0 {
		return len(p), nil
	}
	if len(p) > remain {
		_, _ = l.buf.Write(p[:remain])
		return len(p), nil
	}
	return l.buf.Write(p)
}
