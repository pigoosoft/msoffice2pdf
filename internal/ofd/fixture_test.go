package ofd

import (
	"archive/zip"
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

const ofdNS = `xmlns:ofd="http://www.ofdspec.org/2016"`

func tinyPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("png encode: %v", err)
	}
	return buf.Bytes()
}

type fixtureOpts struct {
	docBodies int
	noPages   bool
}

func writeMinimalOFD(t *testing.T, path string, opts fixtureOpts) {
	t.Helper()
	if opts.docBodies <= 0 {
		opts.docBodies = 1
	}
	pngBytes := tinyPNG(t)

	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create ofd: %v", err)
	}
	defer f.Close()
	zw := zip.NewWriter(f)

	var ofdBuf bytes.Buffer
	ofdBuf.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	ofdBuf.WriteString(fmt.Sprintf(`<ofd:OFD %s Version="1.0" DocType="OFD">`, ofdNS))
	for i := 0; i < opts.docBodies; i++ {
		ofdBuf.WriteString(`<ofd:DocBody>`)
		ofdBuf.WriteString(`<ofd:DocInfo><ofd:DocID>doc`)
		ofdBuf.WriteString(fmt.Sprintf("%d", i))
		ofdBuf.WriteString(`</ofd:DocID></ofd:DocInfo>`)
		ofdBuf.WriteString(fmt.Sprintf(`<ofd:DocRoot>Doc_%d/Document.xml</ofd:DocRoot>`, i))
		ofdBuf.WriteString(`</ofd:DocBody>`)
	}
	ofdBuf.WriteString(`</ofd:OFD>`)
	mustZipWrite(t, zw, "OFD.xml", ofdBuf.Bytes())

	for i := 0; i < opts.docBodies; i++ {
		prefix := fmt.Sprintf("Doc_%d", i)
		docXML := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<ofd:Document %s>
  <ofd:CommonData>
    <ofd:PageArea>
      <ofd:PhysicalBox>0 0 210 297</ofd:PhysicalBox>
    </ofd:PageArea>
    <ofd:PublicRes>PublicRes.xml</ofd:PublicRes>
  </ofd:CommonData>
  <ofd:Pages>`, ofdNS)
		if !opts.noPages {
			docXML += fmt.Sprintf(`
    <ofd:Page ID="1" BaseLoc="Pages/Page_0/Content.xml"/>`)
		}
		docXML += `
  </ofd:Pages>
</ofd:Document>`
		mustZipWrite(t, zw, prefix+"/Document.xml", []byte(docXML))

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
		mustZipWrite(t, zw, prefix+"/PublicRes.xml", []byte(resXML))
		mustZipWrite(t, zw, prefix+"/Res/image.png", pngBytes)

		if !opts.noPages {
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
      <ofd:PathObject ID="11" Boundary="0 0 210 297" Stroke="true" Fill="false" LineWidth="0.5">
        <ofd:AbbreviatedData>M 10 10 L 20 10</ofd:AbbreviatedData>
      </ofd:PathObject>
      <ofd:ImageObject ID="12" Boundary="30 30 20 20" ResourceID="2"/>
    </ofd:Layer>
  </ofd:Content>
</ofd:Page>`, ofdNS)
			mustZipWrite(t, zw, prefix+"/Pages/Page_0/Content.xml", []byte(contentXML))
		}
	}

	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
}

func mustZipWrite(t *testing.T, zw *zip.Writer, name string, data []byte) {
	t.Helper()
	w, err := zw.Create(name)
	if err != nil {
		t.Fatalf("zip create %s: %v", name, err)
	}
	if _, err := w.Write(data); err != nil {
		t.Fatalf("zip write %s: %v", name, err)
	}
}

func writeEmptyZip(t *testing.T, path string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	zw := zip.NewWriter(f)
	if err := zw.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("file close: %v", err)
	}
}

func fixturePath(t *testing.T, name string) string {
	t.Helper()
	return filepath.Join(t.TempDir(), name)
}
