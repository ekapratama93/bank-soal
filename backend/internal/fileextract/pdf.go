package fileextract

import (
	"bytes"
	"strings"

	"github.com/ledongthuc/pdf"
)

func extractPDF(data []byte) (string, error) {
	reader, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", newError("Gagal membaca file PDF.")
	}
	var pages []string
	total := reader.NumPage()
	for i := 1; i <= total; i++ {
		page := reader.Page(i)
		if page.V.IsNull() {
			continue
		}
		text, err := page.GetPlainText(nil)
		if err != nil {
			continue
		}
		pages = append(pages, text)
	}
	return strings.TrimSpace(strings.Join(pages, "\n")), nil
}
