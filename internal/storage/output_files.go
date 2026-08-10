package storage

import (
	"os"
	"path/filepath"
	"strings"
)

// OutputPDFName returns {fileid}.pdf (COM-safe ASCII path on disk).
func OutputPDFName(fileid string) string {
	return fileid + ".pdf"
}

// DownloadPDFName returns {originalStem}.pdf for Content-Disposition / API display.
func DownloadPDFName(originalName string) string {
	base := filepath.Base(strings.TrimSpace(originalName))
	if base == "" || base == "." {
		return "document.pdf"
	}
	stem := strings.TrimSuffix(base, filepath.Ext(base))
	if stem == "" {
		stem = "document"
	}
	return stem + ".pdf"
}

// RelOutputPDF returns slash-separated path relative to outputDir: {uid}/{fileid}.pdf
func RelOutputPDF(uid, fileid string) string {
	return filepath.ToSlash(filepath.Join(uid, OutputPDFName(fileid)))
}

func AbsOutputPDF(outputDir, uid, fileid string) string {
	return AbsPath(outputDir, RelOutputPDF(uid, fileid))
}

func EnsureUserOutputDir(outputDir, uid string) error {
	return EnsureUserDir(outputDir, uid)
}

func RemoveIfExists(path string) error {
	err := os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
