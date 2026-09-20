package fileextract

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"strings"
)

// docx is a zip archive of XML parts — no need for an external docx
// library, just read word/document.xml directly. Go's encoding/xml
// matches struct tags without a namespace prefix against an element's
// local name regardless of its actual XML namespace, so plain tags like
// `xml:"p"` match Word's namespaced <w:p> elements.
type wdParagraph struct {
	Runs []string `xml:"r>t"`
}

func (p wdParagraph) text() string { return strings.Join(p.Runs, "") }

type wdCell struct {
	Paragraphs []wdParagraph `xml:"p"`
}

func (c wdCell) text() string {
	lines := make([]string, len(c.Paragraphs))
	for i, p := range c.Paragraphs {
		lines[i] = p.text()
	}
	return strings.Join(lines, "\n")
}

type wdRow struct {
	Cells []wdCell `xml:"tc"`
}

type wdTable struct {
	Rows []wdRow `xml:"tr"`
}

type wdBody struct {
	Paragraphs []wdParagraph `xml:"p"`
	Tables     []wdTable     `xml:"tbl"`
}

type wdDocument struct {
	Body wdBody `xml:"body"`
}

func extractDOCX(data []byte) (string, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", newError("Gagal membaca file DOCX.")
	}
	var docXML []byte
	for _, f := range zr.File {
		if f.Name == "word/document.xml" {
			rc, err := f.Open()
			if err != nil {
				return "", newError("Gagal membaca file DOCX.")
			}
			docXML, err = io.ReadAll(rc)
			rc.Close()
			if err != nil {
				return "", newError("Gagal membaca file DOCX.")
			}
			break
		}
	}
	if docXML == nil {
		return "", newError("Gagal membaca file DOCX.")
	}

	var doc wdDocument
	if err := xml.Unmarshal(docXML, &doc); err != nil {
		return "", newError("Gagal membaca file DOCX.")
	}

	var lines []string
	for _, p := range doc.Body.Paragraphs {
		if t := strings.TrimSpace(p.text()); t != "" {
			lines = append(lines, p.text())
		}
	}
	for _, table := range doc.Body.Tables {
		for _, row := range table.Rows {
			cells := make([]string, len(row.Cells))
			for i, c := range row.Cells {
				cells[i] = strings.TrimSpace(c.text())
			}
			lines = append(lines, strings.Join(cells, "\t"))
		}
	}
	return strings.TrimSpace(strings.Join(lines, "\n")), nil
}
