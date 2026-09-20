// Package fileextract pulls plain text out of uploaded material files
// (.txt/.pdf/.docx), for use as AI quiz-generation context.
package fileextract

import (
	"fmt"
	"strings"
)

const MaxFileBytes = 2 * 1024 * 1024 // 2 MB

type Error struct{ msg string }

func (e *Error) Error() string { return e.msg }

func newError(msg string) error { return &Error{msg: msg} }

// Extract dispatches by file extension and returns the extracted text, or
// an error if the format is unsupported, too large, or unreadable.
func Extract(filename string, data []byte) (string, error) {
	if len(data) > MaxFileBytes {
		return "", newError(fmt.Sprintf("Ukuran file maksimal %d MB.", MaxFileBytes/(1024*1024)))
	}
	name := strings.ToLower(filename)
	var (
		text string
		err  error
	)
	switch {
	case strings.HasSuffix(name, ".txt"):
		text, err = extractTXT(data)
	case strings.HasSuffix(name, ".pdf"):
		text, err = extractPDF(data)
	case strings.HasSuffix(name, ".docx"):
		text, err = extractDOCX(data)
	default:
		return "", newError("Format file harus .pdf, .docx, atau .txt")
	}
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(text) == "" {
		return "", newError("Isi file kosong atau tidak terbaca. PDF hasil scan tidak didukung.")
	}
	return text, nil
}
