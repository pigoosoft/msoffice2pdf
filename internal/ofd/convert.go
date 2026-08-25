package ofd

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	ofdconv "github.com/zc310/ofd/pkg/converter"
	// Previously rendered with github.com/xiaoqidun/ofdgo (NewReader / RenderToMultiPagePDF).
)

// Options configures OFD conversion. Password may be empty for unencrypted packages.
type Options struct {
	Password string
}

var docBodyRE = regexp.MustCompile(`(?s)<(?:[\w.]+:)?DocBody\b[^>]*>.*?</(?:[\w.]+:)?DocBody>`)

// Convert reads an OFD package at srcPath and writes a PDF to dstPath.
// Rendering uses github.com/zc310/ofd; github.com/xiaoqidun/ofdgo is not linked.
func Convert(ctx context.Context, srcPath, dstPath string, opts Options) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	pkg, err := openPackage(srcPath)
	if err != nil {
		return err
	}
	defer pkg.Close()
	if err := decryptIfNeeded(pkg, opts.Password); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	zipBytes, err := rebuildZip(pkg)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return err
	}
	tmp := dstPath + ".partial"
	if err := renderMaybeMultiDoc(ctx, pkg, zipBytes, tmp); err != nil {
		_ = os.Remove(tmp)
		return redactErr(err, opts.Password)
	}
	if err := ctx.Err(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, dstPath); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func renderMaybeMultiDoc(ctx context.Context, pkg *ofdPackage, zipBytes []byte, outPath string) error {
	ofdXML, err := pkg.readAll(pkg.ofdXML.Name)
	if err != nil {
		return err
	}
	bodies := docBodyRE.FindAll(ofdXML, -1)
	// converter.PDF (and ofdgo) only render the first DocBody; split+merge the rest.
	if len(bodies) <= 1 {
		return renderZipToPDF(ctx, zipBytes, outPath)
	}
	dir, err := os.MkdirTemp("", "ofd-docs-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	var parts []string
	for i := range bodies {
		if err := ctx.Err(); err != nil {
			return err
		}
		oneXML := spliceSingleDocBody(ofdXML, bodies, i)
		oneZip, err := replaceZipMember(zipBytes, pkg.ofdXML.Name, oneXML)
		if err != nil {
			return err
		}
		part := filepath.Join(dir, fmt.Sprintf("%d.pdf", i))
		if err := renderZipToPDF(ctx, oneZip, part); err != nil {
			return err
		}
		parts = append(parts, part)
	}
	return api.MergeCreateFile(parts, outPath, false, nil)
}

func spliceSingleDocBody(ofdXML []byte, bodies [][]byte, index int) []byte {
	locs := docBodyRE.FindAllIndex(ofdXML, -1)
	if len(locs) == 0 || index < 0 || index >= len(locs) {
		return ofdXML
	}
	start := locs[0][0]
	end := locs[len(locs)-1][1]
	var b bytes.Buffer
	b.Write(ofdXML[:start])
	b.Write(bodies[index])
	b.Write(ofdXML[end:])
	return b.Bytes()
}

func replaceZipMember(zipBytes []byte, name string, data []byte) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		return nil, err
	}
	want := normalizeZipPath(name)
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	replaced := false
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		var body []byte
		if normalizeZipPath(f.Name) == want {
			body = data
			replaced = true
		} else {
			rc, err := f.Open()
			if err != nil {
				_ = zw.Close()
				return nil, err
			}
			body, err = io.ReadAll(rc)
			_ = rc.Close()
			if err != nil {
				_ = zw.Close()
				return nil, err
			}
		}
		w, err := zw.Create(f.Name)
		if err != nil {
			_ = zw.Close()
			return nil, err
		}
		if _, err := w.Write(body); err != nil {
			_ = zw.Close()
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	if !replaced {
		return nil, fmt.Errorf("%w: missing member %s", ErrInvalidPackage, name)
	}
	return buf.Bytes(), nil
}

func renderZipToPDF(ctx context.Context, zipBytes []byte, outPath string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()
	// zc310/ofd converter.PDF accepts a file path or []byte ZIP.
	// ofdgo: ofdgo.NewReader + NewRenderer.RenderToMultiPagePDF
	if err := ofdconv.PDF(zipBytes, f); err != nil {
		if isNoPagesErr(err) {
			return ErrNoPages
		}
		return fmt.Errorf("%w: %v", ErrInvalidPackage, err)
	}
	return nil
}

func rebuildZip(pkg *ofdPackage) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, f := range pkg.zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		data, err := pkg.readAll(f.Name)
		if err != nil {
			_ = zw.Close()
			return nil, err
		}
		w, err := zw.Create(f.Name)
		if err != nil {
			_ = zw.Close()
			return nil, err
		}
		if _, err := w.Write(data); err != nil {
			_ = zw.Close()
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func isNoPagesErr(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	// ofdgo: "no pages" / "no docbody"; zc310/ofd: "没有文档" / "文档没有页面"
	return strings.Contains(s, "no pages") || strings.Contains(s, "no docbody") ||
		strings.Contains(s, "没有文档") || strings.Contains(s, "没有页面")
}

func redactErr(err error, password string) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrPasswordRequired) || errors.Is(err, ErrPasswordWrong) ||
		errors.Is(err, ErrInvalidPackage) || errors.Is(err, ErrNoPages) {
		return err
	}
	msg := err.Error()
	if password != "" {
		msg = strings.ReplaceAll(msg, password, "***")
	}
	return fmt.Errorf("%s", msg)
}
