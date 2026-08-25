package ofd

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestCryptUnencryptedIgnoresPassword(t *testing.T) {
	src := fixturePath(t, "plain.ofd")
	writeMinimalOFD(t, src, fixtureOpts{docBodies: 1})
	dst := filepath.Join(t.TempDir(), "out.pdf")

	if err := Convert(context.Background(), src, dst, Options{Password: "x"}); err != nil {
		t.Fatalf("Convert: %v", err)
	}
	assertValidPDF(t, dst, 1)
}

func TestCryptStdAESPasswordFlow(t *testing.T) {
	const password = "secret"
	src := fixturePath(t, "aes.ofd")
	writeEncryptedOFD(t, src, "StdAES", password)
	dstDir := t.TempDir()

	err := Convert(context.Background(), src, filepath.Join(dstDir, "empty.pdf"), Options{})
	if !errors.Is(err, ErrPasswordRequired) {
		t.Fatalf("empty password: got %v, want ErrPasswordRequired", err)
	}
	if err != nil && err.Error() != ErrPasswordRequired.Error() {
		t.Fatalf("Error()=%q, want sentinel %q", err.Error(), ErrPasswordRequired.Error())
	}

	err = Convert(context.Background(), src, filepath.Join(dstDir, "wrong.pdf"), Options{Password: "nope"})
	if !errors.Is(err, ErrPasswordWrong) {
		t.Fatalf("wrong password: got %v, want ErrPasswordWrong", err)
	}
	if err != nil && err.Error() != ErrPasswordWrong.Error() {
		t.Fatalf("Error()=%q, want sentinel %q", err.Error(), ErrPasswordWrong.Error())
	}

	dst := filepath.Join(dstDir, "ok.pdf")
	if err := Convert(context.Background(), src, dst, Options{Password: password}); err != nil {
		t.Fatalf("correct password: %v", err)
	}
	assertValidPDF(t, dst, 1)
}

func TestCryptSM4Unsupported(t *testing.T) {
	src := fixturePath(t, "sm4.ofd")
	writeEncryptedOFD(t, src, "SM4", "any")
	dstDir := t.TempDir()

	err := Convert(context.Background(), src, filepath.Join(dstDir, "empty.pdf"), Options{})
	if !errors.Is(err, ErrPasswordRequired) {
		t.Fatalf("empty password: got %v, want ErrPasswordRequired", err)
	}

	err = Convert(context.Background(), src, filepath.Join(dstDir, "pw.pdf"), Options{Password: "any"})
	if !errors.Is(err, ErrPasswordWrong) {
		t.Fatalf("any password: got %v, want ErrPasswordWrong", err)
	}
}

func TestCryptStdAESAbsoluteEncryptEntryPath(t *testing.T) {
	const password = "secret"
	src := fixturePath(t, "aes-abs.ofd")
	writeEncryptedOFD(t, src, "StdAES", password, encryptOFDOpts{
		entryPaths: []string{"/Doc_0/Document.xml", "/Doc_0/Pages/Page_0/Content.xml"},
	})
	dst := filepath.Join(t.TempDir(), "out.pdf")
	if err := Convert(context.Background(), src, dst, Options{Password: password}); err != nil {
		t.Fatalf("correct password with absolute EncryptEntry Path: %v", err)
	}
	assertValidPDF(t, dst, 1)
}

func TestCryptStdAESRelativeEncryptEntryPath(t *testing.T) {
	const password = "secret"
	src := fixturePath(t, "aes-rel.ofd")
	writeEncryptedOFD(t, src, "StdAES", password, encryptOFDOpts{
		entryPaths: []string{"Document.xml", "Pages/Page_0/Content.xml"},
	})
	dst := filepath.Join(t.TempDir(), "out.pdf")
	if err := Convert(context.Background(), src, dst, Options{Password: password}); err != nil {
		t.Fatalf("correct password with relative EncryptEntry Path: %v", err)
	}
	assertValidPDF(t, dst, 1)
}

func TestCryptUnreadableEncryptionsIsInvalidPackage(t *testing.T) {
	src := fixturePath(t, "bad-enc.ofd")
	writeOFDWithUnreadableEncryptions(t, src)
	dst := filepath.Join(t.TempDir(), "out.pdf")

	err := Convert(context.Background(), src, dst, Options{})
	if err == nil {
		t.Fatal("Convert succeeded; unreadable Encryptions.xml must not be treated as unencrypted")
	}
	if !errors.Is(err, ErrInvalidPackage) {
		t.Fatalf("got %v, want ErrInvalidPackage", err)
	}
	if _, statErr := os.Stat(dst); !os.IsNotExist(statErr) {
		t.Fatalf("dst should not exist, stat=%v", statErr)
	}
}

type encryptOFDOpts struct {
	entryPaths []string // EncryptEntry Path values; zip members stay Doc_0/...
}

// writeEncryptedOFD builds a minimal OFD with Doc_0/Encryptions.xml.
// For StdAES, Document.xml and Content.xml are AES-256-GCM ciphertext (key=sha256(password)).
// For other methods, Encryptions.xml declares the method and entries stay plaintext (unsupported path).
func writeEncryptedOFD(t *testing.T, path, method, password string, opts ...encryptOFDOpts) {
	t.Helper()
	pngBytes := tinyPNG(t)
	entries := []string{
		"Doc_0/Document.xml",
		"Doc_0/Pages/Page_0/Content.xml",
	}
	if len(opts) > 0 && len(opts[0].entryPaths) > 0 {
		entries = opts[0].entryPaths
	}

	docXML := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<ofd:Document %s>
  <ofd:CommonData>
    <ofd:PageArea>
      <ofd:PhysicalBox>0 0 210 297</ofd:PhysicalBox>
    </ofd:PageArea>
    <ofd:PublicRes>PublicRes.xml</ofd:PublicRes>
  </ofd:CommonData>
  <ofd:Pages>
    <ofd:Page ID="1" BaseLoc="Pages/Page_0/Content.xml"/>
  </ofd:Pages>
</ofd:Document>`, ofdNS)

	contentXML := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<ofd:Page %s>
  <ofd:Area>
    <ofd:PhysicalBox>0 0 210 297</ofd:PhysicalBox>
  </ofd:Area>
  <ofd:Content>
    <ofd:Layer ID="1">
      <ofd:TextObject ID="10" Boundary="10 10 50 20" Font="1" Size="12">
        <ofd:TextCode X="0" Y="12">Hello</ofd:TextCode>
      </ofd:TextObject>
    </ofd:Layer>
  </ofd:Content>
</ofd:Page>`, ofdNS)

	resXML := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<ofd:Res %s BaseLoc="Res">
  <ofd:Fonts>
    <ofd:Font ID="1" FontName="Dummy"/>
  </ofd:Fonts>
  <ofd:MultiMedias>
    <ofd:MultiMedia ID="2" Type="Image">
      <ofd:MediaFile>image.png</ofd:MediaFile>
    </ofd:MultiMedia>
  </ofd:MultiMedias>
</ofd:Res>`, ofdNS)

	ofdXML := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<ofd:OFD %s Version="1.0" DocType="OFD">
  <ofd:DocBody>
    <ofd:DocInfo><ofd:DocID>doc0</ofd:DocID></ofd:DocInfo>
    <ofd:DocRoot>Doc_0/Document.xml</ofd:DocRoot>
  </ofd:DocBody>
</ofd:OFD>`, ofdNS)

	docBytes := []byte(docXML)
	contentBytes := []byte(contentXML)
	ivHex := ""

	if method == "StdAES" {
		iv := make([]byte, 12)
		if _, err := rand.Read(iv); err != nil {
			t.Fatalf("rand IV: %v", err)
		}
		ivHex = hex.EncodeToString(iv)
		key := sha256.Sum256([]byte(password))
		block, err := aes.NewCipher(key[:])
		if err != nil {
			t.Fatalf("aes: %v", err)
		}
		gcm, err := cipher.NewGCM(block)
		if err != nil {
			t.Fatalf("gcm: %v", err)
		}
		docBytes = gcm.Seal(nil, iv, docBytes, nil)
		contentBytes = gcm.Seal(nil, iv, contentBytes, nil)
	}

	encXML := bytes.Buffer{}
	encXML.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	encXML.WriteString(fmt.Sprintf(`<ofd:Encryptions %s>`, ofdNS))
	if ivHex != "" {
		encXML.WriteString(fmt.Sprintf(`<ofd:Encryption EncryptMethod=%q IV=%q>`, method, ivHex))
	} else {
		encXML.WriteString(fmt.Sprintf(`<ofd:Encryption EncryptMethod=%q>`, method))
	}
	for _, e := range entries {
		encXML.WriteString(fmt.Sprintf(`<ofd:EncryptEntry Path=%q/>`, e))
	}
	encXML.WriteString(`</ofd:Encryption></ofd:Encryptions>`)

	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	mustZipWrite(t, zw, "OFD.xml", []byte(ofdXML))
	mustZipWrite(t, zw, "Doc_0/Encryptions.xml", encXML.Bytes())
	mustZipWrite(t, zw, "Doc_0/Document.xml", docBytes)
	mustZipWrite(t, zw, "Doc_0/PublicRes.xml", []byte(resXML))
	mustZipWrite(t, zw, "Doc_0/Res/image.png", pngBytes)
	mustZipWrite(t, zw, "Doc_0/Pages/Page_0/Content.xml", contentBytes)
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
}

// writeOFDWithUnreadableEncryptions writes a convertible plaintext OFD whose
// Encryptions.xml member cannot be opened (unsupported zip compression).
func writeOFDWithUnreadableEncryptions(t *testing.T, dest string) {
	t.Helper()
	writeMinimalOFD(t, dest, fixtureOpts{docBodies: 1})

	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("zip reader: %v", err)
	}

	f, err := os.Create(dest)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	const badMethod uint16 = 99
	zw.RegisterCompressor(badMethod, func(out io.Writer) (io.WriteCloser, error) {
		return nopWriteCloser{Writer: out}, nil
	})
	for _, zf := range zr.File {
		rc, err := zf.Open()
		if err != nil {
			t.Fatalf("open %s: %v", zf.Name, err)
		}
		body, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatalf("read %s: %v", zf.Name, err)
		}
		mustZipWrite(t, zw, zf.Name, body)
	}
	h := &zip.FileHeader{Name: "Doc_0/Encryptions.xml", Method: badMethod}
	w, err := zw.CreateHeader(h)
	if err != nil {
		t.Fatalf("create Encryptions.xml: %v", err)
	}
	if _, err := w.Write([]byte(`<?xml version="1.0"?><Encryptions/>`)); err != nil {
		t.Fatalf("write Encryptions.xml: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
}

type nopWriteCloser struct{ io.Writer }

func (nopWriteCloser) Close() error { return nil }
