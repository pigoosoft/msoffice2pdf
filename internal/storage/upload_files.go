package storage

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

func EnsureUserDir(base, uid string) error {
	return os.MkdirAll(filepath.Join(base, uid), 0o755)
}

func SaveUpload(uploadDir, uid, fileid, originalName string, r io.Reader) (relPath, storedName string, size int64, err error) {
	originalName = filepath.Base(originalName)
	if originalName == "" || originalName == "." {
		return "", "", 0, fmt.Errorf("invalid filename")
	}

	if err := EnsureUserDir(uploadDir, uid); err != nil {
		return "", "", 0, err
	}

	storedName = fileid + "_" + originalName
	absPath := filepath.Join(uploadDir, uid, storedName)

	f, err := os.Create(absPath)
	if err != nil {
		return "", "", 0, err
	}
	defer f.Close()

	size, err = io.Copy(f, r)
	if err != nil {
		os.Remove(absPath)
		return "", "", 0, err
	}

	relPath = filepath.ToSlash(filepath.Join(uid, storedName))
	return relPath, storedName, size, nil
}

func AbsPath(baseDir, relPath string) string {
	return filepath.Join(baseDir, filepath.FromSlash(relPath))
}

func MoveToTrash(uploadDir, trashDir, uid, fileid, originalName, relPath string) (trashRel string, err error) {
	originalName = filepath.Base(originalName)
	if originalName == "" || originalName == "." {
		return "", fmt.Errorf("invalid filename")
	}

	datetime := time.Now().Format("20060102150405")
	trashName := fmt.Sprintf("%s_%s_%s_%s", uid, datetime, fileid, originalName)

	if err := EnsureUserDir(trashDir, uid); err != nil {
		return "", err
	}

	src := AbsPath(uploadDir, relPath)
	dst := filepath.Join(trashDir, uid, trashName)

	if err := os.Rename(src, dst); err != nil {
		if copyErr := copyFile(src, dst); copyErr != nil {
			return "", fmt.Errorf("rename: %w; copy: %v", err, copyErr)
		}
		if removeErr := os.Remove(src); removeErr != nil {
			return "", removeErr
		}
	}

	trashRel = filepath.ToSlash(filepath.Join(uid, trashName))
	return trashRel, nil
}

// MoveToExpired moves an upload file into expired/{uid}/{uid}_{datetime}_{fileid}_{originalName}.
// Returns slash-separated path relative to expiredDir.
// If the source file does not exist, returns an error wrapping os.ErrNotExist (caller may still finish DB).
func MoveToExpired(uploadDir, expiredDir, uid, fileid, originalName, relPath string) (expiredRel string, err error) {
	originalName = filepath.Base(originalName)
	if originalName == "" || originalName == "." {
		return "", fmt.Errorf("invalid filename")
	}

	datetime := time.Now().Format("20060102150405")
	expiredName := fmt.Sprintf("%s_%s_%s_%s", uid, datetime, fileid, originalName)

	if err := EnsureUserDir(expiredDir, uid); err != nil {
		return "", err
	}

	src := AbsPath(uploadDir, relPath)
	dst := filepath.Join(expiredDir, uid, expiredName)

	if _, statErr := os.Stat(src); statErr != nil {
		if os.IsNotExist(statErr) {
			return "", fmt.Errorf("%w: %s", os.ErrNotExist, src)
		}
		return "", statErr
	}

	if err := os.Rename(src, dst); err != nil {
		if copyErr := copyFile(src, dst); copyErr != nil {
			return "", fmt.Errorf("rename: %w; copy: %v", err, copyErr)
		}
		if removeErr := os.Remove(src); removeErr != nil {
			return "", removeErr
		}
	}

	expiredRel = filepath.ToSlash(filepath.Join(uid, expiredName))
	return expiredRel, nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		os.Remove(dst)
		return err
	}
	return out.Close()
}
