package fileextract

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ledongthuc/pdf"
)

// pdfRenderDPI balances legibility of diagrams/labels against image size —
// high enough to read a typical textbook diagram's labels, low enough that
// a handful of rendered pages stays well under upload/LLM payload limits.
const pdfRenderDPI = 150

// pdfRenderTimeout bounds a single pdftoppm invocation — a malformed or
// adversarial PDF (a user-uploaded, untrusted file) must not hang the
// upload request indefinitely.
const pdfRenderTimeout = 15 * time.Second

func extractPDF(data []byte) (string, []Image, error) {
	reader, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", nil, newError("Gagal membaca file PDF.")
	}
	total := reader.NumPage()

	// A full-page screenshot captures whatever a cropped embedded-image
	// extraction structurally can't: vector-drawn diagrams and scanned
	// pages that have no discrete image object at all, and it keeps a
	// diagram's caption/labels in the same frame automatically — no
	// separate position-marker heuristic needed, the marker below just
	// says "this page has a picture", not "this exact spot".
	renderCount := total
	if renderCount > maxExtractedImages {
		renderCount = maxExtractedImages
	}
	pageImages := renderPDFPages(data, renderCount)
	images := make([]Image, len(pageImages))
	markerIndexByPage := make(map[int]int, len(pageImages))
	for i, pi := range pageImages {
		images[i] = pi.Image
		markerIndexByPage[pi.pageNr] = i + 1
	}

	var pages []string
	for i := 1; i <= total; i++ {
		page := reader.Page(i)
		if page.V.IsNull() {
			continue
		}
		text, err := page.GetPlainText(nil)
		if err != nil {
			continue
		}
		text = strings.TrimSpace(text)
		if idx, ok := markerIndexByPage[i]; ok {
			text = strings.TrimSpace(fmt.Sprintf("%s [Gambar %d]", text, idx))
		}
		pages = append(pages, text)
	}
	return strings.TrimSpace(strings.Join(pages, "\n")), images, nil
}

// pdfPageImage pairs a rendered page image with its page number, so
// extractPDF can place its "[Gambar N]" marker in that page's text.
type pdfPageImage struct {
	Image
	pageNr int
}

// renderPDFPages rasterizes the first count pages of the PDF via pdftoppm
// (poppler-utils), one page at a time. A page that fails to render (a
// missing pdftoppm binary, a malformed page, a timeout) is just skipped —
// text extraction above already succeeded and is worth keeping even if
// rendering can't.
func renderPDFPages(data []byte, count int) []pdfPageImage {
	if count <= 0 {
		return nil
	}
	tmpDir, err := os.MkdirTemp("", "banksoal-pdf-*")
	if err != nil {
		slog.Warn("Gagal membuat direktori sementara untuk render PDF", "err", err)
		return nil
	}
	defer os.RemoveAll(tmpDir)

	inputPath := filepath.Join(tmpDir, "input.pdf")
	if err := os.WriteFile(inputPath, data, 0o600); err != nil {
		slog.Warn("Gagal menulis file PDF sementara", "err", err)
		return nil
	}

	var images []pdfPageImage
	for page := 1; page <= count; page++ {
		img, err := renderPDFPage(inputPath, tmpDir, page)
		if err != nil {
			slog.Warn("Gagal me-render halaman PDF ke gambar", "page", page, "err", err)
			continue
		}
		images = append(images, pdfPageImage{Image: *img, pageNr: page})
	}
	return images
}

func renderPDFPage(inputPath, tmpDir string, page int) (*Image, error) {
	outPrefix := filepath.Join(tmpDir, "page-"+strconv.Itoa(page))
	ctx, cancel := context.WithTimeout(context.Background(), pdfRenderTimeout)
	defer cancel()
	// -singlefile makes pdftoppm write exactly outPrefix+".png" instead of
	// a page-number-suffixed name whose width depends on the PDF's total
	// page count.
	cmd := exec.CommandContext(ctx, "pdftoppm",
		"-png", "-r", strconv.Itoa(pdfRenderDPI),
		"-f", strconv.Itoa(page), "-l", strconv.Itoa(page), "-singlefile",
		inputPath, outPrefix,
	)
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(outPrefix + ".png")
	if err != nil {
		return nil, err
	}
	return &Image{Name: fmt.Sprintf("halaman-%d.png", page), Data: data, ContentType: "image/png"}, nil
}
