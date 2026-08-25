package ofd

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"path"
	"strings"
)

// Minimal Encryptions.xml / Encryption.xml model for StdAES fixtures.
type encryptionXML struct {
	XMLName    xml.Name          `xml:"Encryptions"`
	Encryption []encryptionBlock `xml:"Encryption"`
}

type encryptionBlock struct {
	EncryptMethod string            `xml:"EncryptMethod,attr"`
	IV            string            `xml:"IV,attr"`
	Entries       []encryptEntryXML `xml:"EncryptEntry"`
	MethodElem    string            `xml:"EncryptMethod"`
	IVElem        string            `xml:"IV"`
}

type encryptEntryXML struct {
	Path string `xml:"Path,attr"`
}

func decryptIfNeeded(pkg *ofdPackage, password string) error {
	descName, data, err := findEncryptionDesc(pkg)
	if err != nil {
		return err
	}
	if descName == "" {
		return nil // unencrypted; ignore password
	}
	if password == "" {
		return ErrPasswordRequired
	}

	method, ivHex, entries, err := parseEncryptionXML(data)
	if err != nil {
		return ErrPasswordWrong
	}
	if !strings.EqualFold(method, "StdAES") {
		return ErrPasswordWrong
	}
	return decryptStdAES(pkg, password, ivHex, entries, descName)
}

func findEncryptionDesc(pkg *ofdPackage) (name string, data []byte, err error) {
	for key, f := range pkg.byName {
		base := path.Base(key)
		if base != "encryptions.xml" && base != "encryption.xml" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return "", nil, fmt.Errorf("%w: open %s: %v", ErrInvalidPackage, f.Name, err)
		}
		b, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			return "", nil, fmt.Errorf("%w: read %s: %v", ErrInvalidPackage, f.Name, err)
		}
		return f.Name, b, nil
	}
	return "", nil, nil
}

func parseEncryptionXML(data []byte) (method, ivHex string, entries []string, err error) {
	data = stripOFDNamespace(data)

	var root encryptionXML
	if err := xml.Unmarshal(data, &root); err == nil && len(root.Encryption) > 0 {
		return blockFields(root.Encryption[0])
	}

	var single struct {
		XMLName xml.Name `xml:"Encryption"`
		encryptionBlock
	}
	if err := xml.Unmarshal(data, &single); err != nil {
		return "", "", nil, err
	}
	return blockFields(single.encryptionBlock)
}

func blockFields(b encryptionBlock) (method, ivHex string, entries []string, err error) {
	method = firstNonEmpty(b.EncryptMethod, b.MethodElem)
	ivHex = firstNonEmpty(b.IV, b.IVElem)
	for _, e := range b.Entries {
		if e.Path != "" {
			entries = append(entries, e.Path)
		}
	}
	return method, ivHex, entries, nil
}

func stripOFDNamespace(data []byte) []byte {
	return []byte(strings.ReplaceAll(string(data), "ofd:", ""))
}

func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return strings.TrimSpace(a)
	}
	return strings.TrimSpace(b)
}

func decryptStdAES(pkg *ofdPackage, password, ivHex string, entries []string, descName string) error {
	iv, err := hex.DecodeString(ivHex)
	if err != nil || len(iv) == 0 {
		return ErrPasswordWrong
	}
	key := sha256.Sum256([]byte(password))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return ErrPasswordWrong
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return ErrPasswordWrong
	}
	if len(iv) != gcm.NonceSize() {
		return ErrPasswordWrong
	}
	if pkg.overlay == nil {
		pkg.overlay = make(map[string][]byte)
	}
	baseDir := path.Dir(strings.ReplaceAll(descName, "\\", "/"))
	for _, entry := range entries {
		resolved, cipherText, err := readEncryptEntry(pkg, baseDir, entry)
		if err != nil {
			return ErrPasswordWrong
		}
		plain, err := gcm.Open(nil, iv, cipherText, nil)
		if err != nil {
			return ErrPasswordWrong
		}
		pkg.overlay[resolved] = plain
	}
	return nil
}

// readEncryptEntry resolves Path like ofdPackage.resolve (relative to Encryptions.xml).
// Zip-root relative paths (Doc_0/Document.xml) are tried if the first lookup misses.
func readEncryptEntry(pkg *ofdPackage, baseDir, entry string) (string, []byte, error) {
	resolved := pkg.resolve(baseDir, entry)
	data, err := pkg.readAllFromZip(resolved)
	if err == nil {
		return resolved, data, nil
	}
	rootPath := pkg.resolve("", entry)
	if rootPath == resolved {
		return "", nil, err
	}
	data, err = pkg.readAllFromZip(rootPath)
	if err != nil {
		return "", nil, err
	}
	return rootPath, data, nil
}
