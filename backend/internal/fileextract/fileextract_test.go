package fileextract

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"

	"golang.org/x/text/encoding/charmap"
)

func TestExtractTXTUTF8(t *testing.T) {
	text, err := Extract("materi.txt", []byte("Perkalian dasar.\n2 x 3 = 6"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(text, "Perkalian dasar.") {
		t.Errorf("text = %q", text)
	}
}

func TestExtractTXTNonUTF8Fallback(t *testing.T) {
	data := append([]byte("Jaring-jaring kubus "), 0xe9) // latin-1 'é', not valid UTF-8
	text, err := Extract("materi.txt", data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(text, "Jaring-jaring") {
		t.Errorf("text = %q", text)
	}
}

func TestExtractTXTUTF8Arabic(t *testing.T) {
	text, err := Extract("materi.txt", []byte("Hadis: إنَّمَا الأَعْمَالُ بِالنِّيَّاتِ"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(text, "إنَّمَا") {
		t.Errorf("text = %q", text)
	}
}

func TestExtractTXTUTF8BOMArabic(t *testing.T) {
	data := append([]byte{0xEF, 0xBB, 0xBF}, []byte("بسم الله")...)
	text, err := Extract("materi.txt", data)
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
	text, err := Extract("materi.txt", data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if text != "بسم الله" {
		t.Errorf("text = %q, want %q", text, "بسم الله")
	}
}

func TestExtractUnsupportedExtension(t *testing.T) {
	_, err := Extract("materi.rtf", []byte("isi"))
	if err == nil || !strings.Contains(err.Error(), ".pdf, .docx, atau .txt") {
		t.Fatalf("err = %v", err)
	}
}

func TestExtractEmptyText(t *testing.T) {
	_, err := Extract("materi.txt", []byte("   \n  "))
	if err == nil || !strings.Contains(err.Error(), "kosong") {
		t.Fatalf("err = %v", err)
	}
}

func TestExtractSizeLimit(t *testing.T) {
	_, err := Extract("materi.txt", bytes.Repeat([]byte("x"), 2*1024*1024+1))
	if err == nil || !strings.Contains(err.Error(), "2 MB") {
		t.Fatalf("err = %v", err)
	}
}

func TestExtractDOCXCorrupt(t *testing.T) {
	_, err := Extract("materi.docx", []byte("bukan file docx"))
	if err == nil || !strings.Contains(err.Error(), "DOCX") {
		t.Fatalf("err = %v", err)
	}
}

func TestExtractDOCXEmpty(t *testing.T) {
	_, err := Extract("materi.docx", buildDOCX(t, nil, nil))
	if err == nil || !strings.Contains(err.Error(), "kosong") {
		t.Fatalf("err = %v", err)
	}
}

func TestExtractDOCXParagraphsAndTable(t *testing.T) {
	paragraphs := []string{"Bab 1: Perkalian", ""}
	table := [][]string{{"2 x 3", "6"}, {"3 x 4", "12"}}
	text, err := Extract("materi.docx", buildDOCX(t, paragraphs, table))
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
