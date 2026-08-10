package validate

import (
	"bytes"
	"fmt"
	"io"

	"msoffice2pdf/internal/config"
)

var (
	oleHeader = []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}
	zipHeader = []byte{0x50, 0x4B} // PK
)

func checkMagic(r io.Reader, filename string, uploadCfg config.UploadConfig) error {
	buf := make([]byte, 8)
	n, err := io.ReadFull(r, buf)
	if err != nil && err != io.ErrUnexpectedEOF {
		if err == io.EOF {
			return fmt.Errorf("%w: empty file", ErrMagic)
		}
		return err
	}
	buf = buf[:n]
	fam := uploadCfg.OfficeFamily(filename)
	switch fam {
	case "ole":
		if len(buf) < 8 || !bytes.Equal(buf[:8], oleHeader) {
			return fmt.Errorf("%w: expected OLE header", ErrMagic)
		}
	case "ooxml":
		if len(buf) < 2 || !bytes.Equal(buf[:2], zipHeader) {
			return fmt.Errorf("%w: expected ZIP/PK header", ErrMagic)
		}
	}
	return nil
}
