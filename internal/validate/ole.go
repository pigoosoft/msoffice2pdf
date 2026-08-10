package validate

import (
	"bytes"
	"fmt"
	"io"
	"unicode/utf16"
)

const oleScanLimit = 2 << 20 // 2 MiB

func checkOLEStreams(r io.ReadSeeker, required []string) error {
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return err
	}
	limited := io.LimitReader(r, oleScanLimit)
	data, err := io.ReadAll(limited)
	if err != nil {
		return err
	}
	if len(data) < 8 || !bytes.Equal(data[:8], oleHeader) {
		return fmt.Errorf("%w: bad OLE header", ErrStructure)
	}
	for _, need := range required {
		if !containsUTF16LE(data, need) {
			return fmt.Errorf("%w: missing OLE stream %q", ErrStructure, need)
		}
	}
	return nil
}

func containsUTF16LE(data []byte, s string) bool {
	u := utf16.Encode([]rune(s))
	pat := make([]byte, len(u)*2)
	for i, v := range u {
		pat[i*2] = byte(v)
		pat[i*2+1] = byte(v >> 8)
	}
	return bytes.Contains(data, pat)
}
