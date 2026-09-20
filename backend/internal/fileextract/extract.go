// Package fileextract pulls plain text (and any embedded images) out of
// uploaded material files (.txt/.pdf/.docx), for use as AI quiz-generation
// context.
package fileextract

import (
	"fmt"
	"strings"
)

const MaxFileBytes = 2 * 1024 * 1024 // 2 MB

// maxExtractedImages caps how many images come out of a single material
// file — for docx, how many distinct inline images; for pdf, how many
// pages get rendered. Enough for a handful of diagrams/photos without
// letting a large document balloon storage/LLM cost.
const maxExtractedImages = 10

// minExtractedImageBytes filters out tiny decorative docx images (bullets,
// separator lines, brand icons) that clutter document media folders but
// carry no quiz-relevant content. Not applied to PDF page renders, which
// are never that small.
const minExtractedImageBytes = 3 * 1024

// Image is one image extracted from an uploaded material file — an
// inline image from a docx's word/media/*, or a full-page render of a PDF
// page (see pdf.go: a screenshot of the whole page keeps a diagram and its
// caption/labels together and catches vector-drawn diagrams that have no
// discrete embeddable image object).
type Image struct {
	Name        string
	Data        []byte
	ContentType string
}

var imageContentTypeByExt = map[string]string{
	"png":  "image/png",
	"jpg":  "image/jpeg",
	"jpeg": "image/jpeg",
	"gif":  "image/gif",
	"bmp":  "image/bmp",
	"tif":  "image/tiff",
	"tiff": "image/tiff",
	"webp": "image/webp",
}

func extOf(name string) string {
	idx := strings.LastIndex(name, ".")
	if idx < 0 {
		return ""
	}
	return strings.ToLower(name[idx+1:])
}

type Error struct{ msg string }

func (e *Error) Error() string { return e.msg }

func newError(msg string) error { return &Error{msg: msg} }

// Extract dispatches by file extension and returns the extracted text and
// any embedded images, or an error if the format is unsupported, too
// large, or unreadable.
func Extract(filename string, data []byte) (string, []Image, error) {
	if len(data) > MaxFileBytes {
		return "", nil, newError(fmt.Sprintf("Ukuran file maksimal %d MB.", MaxFileBytes/(1024*1024)))
	}
	name := strings.ToLower(filename)
	var (
		text   string
		images []Image
		err    error
	)
	switch {
	case strings.HasSuffix(name, ".txt"):
		text, err = extractTXT(data)
	case strings.HasSuffix(name, ".pdf"):
		text, images, err = extractPDF(data)
	case strings.HasSuffix(name, ".docx"):
		text, images, err = extractDOCX(data)
	default:
		return "", nil, newError("Format file harus .pdf, .docx, atau .txt")
	}
	if err != nil {
		return "", nil, err
	}
	if strings.TrimSpace(text) == "" {
		return "", nil, newError("Isi file kosong atau tidak terbaca. PDF hasil scan tidak didukung.")
	}
	return text, images, nil
}
