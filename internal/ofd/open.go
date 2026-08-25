package ofd

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"path"
	"strings"
)

type ofdPackage struct {
	zr      *zip.ReadCloser
	ofdXML  *zip.File
	byName  map[string]*zip.File // lower-case slash paths
	overlay map[string][]byte    // decrypted member bytes (lower-case paths)
}

func openPackage(srcPath string) (*ofdPackage, error) {
	zr, err := zip.OpenReader(srcPath)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidPackage, err)
	}
	pkg := &ofdPackage{
		zr:     zr,
		byName: make(map[string]*zip.File, len(zr.File)),
	}
	for _, f := range zr.File {
		key := normalizeZipPath(f.Name)
		pkg.byName[key] = f
		base := path.Base(key)
		if base == "ofd.xml" {
			pkg.ofdXML = f
		}
	}
	if pkg.ofdXML == nil {
		_ = zr.Close()
		return nil, fmt.Errorf("%w: missing OFD.xml", ErrInvalidPackage)
	}
	return pkg, nil
}

func (p *ofdPackage) Close() error {
	if p == nil || p.zr == nil {
		return nil
	}
	return p.zr.Close()
}

func (p *ofdPackage) open(rel string) (io.ReadCloser, error) {
	key := normalizeZipPath(rel)
	if p.overlay != nil {
		if b, ok := p.overlay[key]; ok {
			return io.NopCloser(bytes.NewReader(b)), nil
		}
	}
	f, ok := p.byName[key]
	if !ok {
		return nil, fmt.Errorf("%w: missing member %s", ErrInvalidPackage, rel)
	}
	rc, err := f.Open()
	if err != nil {
		return nil, fmt.Errorf("%w: open %s: %v", ErrInvalidPackage, rel, err)
	}
	return rc, nil
}

func (p *ofdPackage) readAll(rel string) ([]byte, error) {
	key := normalizeZipPath(rel)
	if p.overlay != nil {
		if b, ok := p.overlay[key]; ok {
			out := make([]byte, len(b))
			copy(out, b)
			return out, nil
		}
	}
	return p.readAllFromZip(rel)
}

// readAllFromZip reads a member from the zip only (ignores overlay).
func (p *ofdPackage) readAllFromZip(rel string) ([]byte, error) {
	key := normalizeZipPath(rel)
	f, ok := p.byName[key]
	if !ok {
		return nil, fmt.Errorf("%w: missing member %s", ErrInvalidPackage, rel)
	}
	rc, err := f.Open()
	if err != nil {
		return nil, fmt.Errorf("%w: open %s: %v", ErrInvalidPackage, rel, err)
	}
	defer rc.Close()
	b, err := io.ReadAll(rc)
	if err != nil {
		return nil, fmt.Errorf("%w: read %s: %v", ErrInvalidPackage, rel, err)
	}
	return b, nil
}

func (p *ofdPackage) resolve(baseDir, loc string) string {
	loc = strings.TrimSpace(loc)
	loc = strings.ReplaceAll(loc, "\\", "/")
	if loc == "" {
		return ""
	}
	if path.IsAbs(loc) || strings.HasPrefix(loc, "/") {
		return normalizeZipPath(strings.TrimPrefix(loc, "/"))
	}
	return normalizeZipPath(path.Join(baseDir, loc))
}

func normalizeZipPath(p string) string {
	p = strings.ReplaceAll(p, "\\", "/")
	p = strings.TrimPrefix(p, "./")
	p = path.Clean(p)
	if p == "." {
		return ""
	}
	return strings.ToLower(p)
}
