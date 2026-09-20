package fileextract

import (
	"archive/zip"
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"golang.org/x/text/encoding/charmap"
)

func TestExtractTXTUTF8(t *testing.T) {
	text, _, err := Extract("materi.txt", []byte("Perkalian dasar.\n2 x 3 = 6"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(text, "Perkalian dasar.") {
		t.Errorf("text = %q", text)
	}
}

func TestExtractTXTNonUTF8Fallback(t *testing.T) {
	data := append([]byte("Jaring-jaring kubus "), 0xe9) // latin-1 'é', not valid UTF-8
	text, _, err := Extract("materi.txt", data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(text, "Jaring-jaring") {
		t.Errorf("text = %q", text)
	}
}

func TestExtractTXTUTF8Arabic(t *testing.T) {
	text, _, err := Extract("materi.txt", []byte("Hadis: إنَّمَا الأَعْمَالُ بِالنِّيَّاتِ"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(text, "إنَّمَا") {
		t.Errorf("text = %q", text)
	}
}

func TestExtractTXTUTF8BOMArabic(t *testing.T) {
	data := append([]byte{0xEF, 0xBB, 0xBF}, []byte("بسم الله")...)
	text, _, err := Extract("materi.txt", data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(text, "بسم") {
		t.Errorf("text = %q", text)
	}
}

func TestExtractTXTCP1256Arabic(t *testing.T) {
	data, err := charmap.Windows1256.NewEncoder().Bytes([]byte("بسم الله"))
	if err != nil {
		t.Fatalf("failed to build cp1256 fixture: %v", err)
	}
	text, _, err := Extract("materi.txt", data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if text != "بسم الله" {
		t.Errorf("text = %q, want %q", text, "بسم الله")
	}
}

func TestExtractUnsupportedExtension(t *testing.T) {
	_, _, err := Extract("materi.rtf", []byte("isi"))
	if err == nil || !strings.Contains(err.Error(), ".pdf, .docx, atau .txt") {
		t.Fatalf("err = %v", err)
	}
}

func TestExtractEmptyText(t *testing.T) {
	_, _, err := Extract("materi.txt", []byte("   \n  "))
	if err == nil || !strings.Contains(err.Error(), "kosong") {
		t.Fatalf("err = %v", err)
	}
}

func TestExtractSizeLimit(t *testing.T) {
	_, _, err := Extract("materi.txt", bytes.Repeat([]byte("x"), 2*1024*1024+1))
	if err == nil || !strings.Contains(err.Error(), "2 MB") {
		t.Fatalf("err = %v", err)
	}
}

func TestExtractDOCXCorrupt(t *testing.T) {
	_, _, err := Extract("materi.docx", []byte("bukan file docx"))
	if err == nil || !strings.Contains(err.Error(), "DOCX") {
		t.Fatalf("err = %v", err)
	}
}

func TestExtractDOCXEmpty(t *testing.T) {
	_, _, err := Extract("materi.docx", buildDOCX(t, nil, nil))
	if err == nil || !strings.Contains(err.Error(), "kosong") {
		t.Fatalf("err = %v", err)
	}
}

func TestExtractDOCXParagraphsAndTable(t *testing.T) {
	paragraphs := []string{"Bab 1: Perkalian", ""}
	table := [][]string{{"2 x 3", "6"}, {"3 x 4", "12"}}
	text, _, err := Extract("materi.docx", buildDOCX(t, paragraphs, table))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(text, "Bab 1: Perkalian") {
		t.Errorf("text missing paragraph: %q", text)
	}
	if !strings.Contains(text, "2 x 3\t6") {
		t.Errorf("text missing tab-joined table row: %q", text)
	}
}

// TestExtractPDFTextAndPageRender exercises text extraction unconditionally,
// and the pdftoppm page-render path when the binary happens to be installed
// (mirrors this package's other environment-dependent test, the Docker
// integration tests in internal/store) — either way, it checks that a
// missing renderer degrades gracefully instead of losing the text or
// leaving a dangling "[Gambar N]" marker with no image behind it.
func TestExtractPDFTextAndPageRender(t *testing.T) {
	text, images, err := Extract("materi.pdf", buildMinimalPDF(t, "Hello PDF materi"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(text, "Hello PDF materi") {
		t.Fatalf("text = %q, missing extracted content", text)
	}

	if _, err := exec.LookPath("pdftoppm"); err != nil {
		if len(images) != 0 || strings.Contains(text, "[Gambar") {
			t.Errorf("expected no rendered images/marker without pdftoppm: images=%+v text=%q", images, text)
		}
		t.Skip("pdftoppm not installed — page-render path not exercised")
	}
	if len(images) != 1 || images[0].ContentType != "image/png" {
		t.Fatalf("images = %+v", images)
	}
	if !strings.Contains(text, "[Gambar 1]") {
		t.Errorf("text missing page-render marker: %q", text)
	}
}

// buildMinimalPDF assembles a minimal, spec-valid single-page PDF (correct
// xref byte offsets computed as it's written) with the given text drawn on
// the page, so the test exercises the real PDF parsing path.
func buildMinimalPDF(t *testing.T, text string) []byte {
	t.Helper()
	var buf bytes.Buffer
	var offsets []int // offsets[id] = byte offset of object id (offsets[0] unused)
	writeObj := func(id int, body string) {
		for len(offsets) <= id {
			offsets = append(offsets, 0)
		}
		offsets[id] = buf.Len()
		fmt.Fprintf(&buf, "%d 0 obj\n%s\nendobj\n", id, body)
	}

	buf.WriteString("%PDF-1.4\n")
	writeObj(1, "<< /Type /Catalog /Pages 2 0 R >>")
	writeObj(2, "<< /Type /Pages /Kids [3 0 R] /Count 1 >>")
	writeObj(3, "<< /Type /Page /Parent 2 0 R /Resources << /Font << /F1 4 0 R >> >> /MediaBox [0 0 200 200] /Contents 5 0 R >>")
	writeObj(4, "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>")
	stream := fmt.Sprintf("BT /F1 24 Tf 10 100 Td (%s) Tj ET", text)
	writeObj(5, fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(stream), stream))

	const totalObjs = 5
	xrefOffset := buf.Len()
	fmt.Fprintf(&buf, "xref\n0 %d\n0000000000 65535 f \n", totalObjs+1)
	for id := 1; id <= totalObjs; id++ {
		fmt.Fprintf(&buf, "%010d 00000 n \n", offsets[id])
	}
	fmt.Fprintf(&buf, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF", totalObjs+1, xrefOffset)
	return buf.Bytes()
}

func TestExtractDOCXInlineImageMarker(t *testing.T) {
	imageData := bytes.Repeat([]byte{0xFF}, minExtractedImageBytes+1) // above the size filter
	docx := buildDOCXWithInlineImage(t, "Sebelum gambar", "image1.png", imageData, "Setelah gambar")

	text, images, err := Extract("materi.docx", docx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(images) != 1 || images[0].Name != "image1.png" || images[0].ContentType != "image/png" {
		t.Fatalf("images = %+v", images)
	}

	before := strings.Index(text, "Sebelum gambar")
	marker := strings.Index(text, "[Gambar 1]")
	after := strings.Index(text, "Setelah gambar")
	if before < 0 || marker < 0 || after < 0 || !(before < marker && marker < after) {
		t.Fatalf("marker not positioned between surrounding paragraphs: %q", text)
	}
}

func TestExtractDOCXImageBelowSizeFilterHasNoMarker(t *testing.T) {
	tiny := bytes.Repeat([]byte{0x00}, 10)
	docx := buildDOCXWithInlineImage(t, "Sebelum gambar", "image1.png", tiny, "Setelah gambar")

	text, images, err := Extract("materi.docx", docx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(images) != 0 {
		t.Fatalf("images = %+v, want none (below size filter)", images)
	}
	if strings.Contains(text, "[Gambar") {
		t.Errorf("text should have no leftover marker: %q", text)
	}
}

// buildDOCXWithInlineImage builds a real docx with a paragraph before, one
// paragraph containing an inline DrawingML image (referencing the given
// image via a real word/_rels/document.xml.rels relationship, the same way
// Word links a <w:drawing> to its media file), and a paragraph after — so
// tests exercise the actual position-anchoring path, not just "some file
// exists under word/media/".
func buildDOCXWithInlineImage(t *testing.T, before, imageName string, imageData []byte, after string) []byte {
	t.Helper()
	documentXML := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document
  xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"
  xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"
  xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing"
  xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
  xmlns:pic="http://schemas.openxmlformats.org/drawingml/2006/picture">
<w:body>
<w:p><w:r><w:t>` + before + `</w:t></w:r></w:p>
<w:p><w:r><w:drawing><wp:inline><a:graphic><a:graphicData><pic:pic><pic:blipFill><a:blip r:embed="rId1"/></pic:blipFill></pic:pic></a:graphicData></a:graphic></wp:inline></w:drawing></w:r></w:p>
<w:p><w:r><w:t>` + after + `</w:t></w:r></w:p>
</w:body>
</w:document>`

	relsXML := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="media/` + imageName + `"/>
</Relationships>`

	buf := &bytes.Buffer{}
	zw := zip.NewWriter(buf)
	for name, data := range map[string][]byte{
		"word/document.xml":            []byte(documentXML),
		"word/_rels/document.xml.rels": []byte(relsXML),
		"word/media/" + imageName:      imageData,
	} {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// buildDOCX assembles a minimal, real docx zip (word/document.xml with the
// WordprocessingML namespace) so the test exercises the actual zip+XML
// parsing path, not a mocked one.
func buildDOCX(t *testing.T, paragraphs []string, table [][]string) []byte {
	t.Helper()
	var body strings.Builder
	for _, p := range paragraphs {
		body.WriteString("<w:p>")
		if p != "" {
			body.WriteString("<w:r><w:t>" + p + "</w:t></w:r>")
		}
		body.WriteString("</w:p>")
	}
	if len(table) > 0 {
		body.WriteString("<w:tbl>")
		for _, row := range table {
			body.WriteString("<w:tr>")
			for _, cell := range row {
				body.WriteString("<w:tc><w:p><w:r><w:t>" + cell + "</w:t></w:r></w:p></w:tc>")
			}
			body.WriteString("</w:tr>")
		}
		body.WriteString("</w:tbl>")
	}

	documentXML := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
<w:body>` + body.String() + `</w:body>
</w:document>`

	buf := &bytes.Buffer{}
	zw := zip.NewWriter(buf)
	f, err := zw.Create("word/document.xml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write([]byte(documentXML)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
