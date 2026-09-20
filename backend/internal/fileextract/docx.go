package fileextract

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

// docx is a zip archive of XML parts — no need for an external docx
// library, just read word/document.xml directly. Go's encoding/xml
// matches struct tags without a namespace prefix against an element's
// local name regardless of its actual XML namespace, so plain tags like
// `xml:"p"` match Word's namespaced <w:p> elements.

// wdBlip is the <a:blip r:embed="rId4"/> deep inside an inline/floating
// image's DrawingML — its embed attribute is a relationship ID that
// word/_rels/document.xml.rels resolves to the actual media file.
type wdBlip struct {
	Embed string `xml:"embed,attr"`
}

type wdPic struct {
	Blip wdBlip `xml:"blipFill>blip"`
}

type wdGraphicData struct {
	Pic wdPic `xml:"pic"`
}

// wdAnchorOrInline is the common shape of both <wp:inline> (in-line images)
// and <wp:anchor> (floating images) — both wrap the same graphic/pic/blip
// chain, just with different positioning metadata we don't care about.
type wdAnchorOrInline struct {
	GraphicData wdGraphicData `xml:"graphic>graphicData"`
}

type wdDrawing struct {
	Inline wdAnchorOrInline `xml:"inline"`
	Anchor wdAnchorOrInline `xml:"anchor"`
}

type wdRun struct {
	Text     []string    `xml:"t"`
	Drawings []wdDrawing `xml:"drawing"`
}

// embedIDs returns the r:embed relationship IDs of every image this run
// draws, in document order.
func (r wdRun) embedIDs() []string {
	var ids []string
	for _, d := range r.Drawings {
		if id := d.Inline.GraphicData.Pic.Blip.Embed; id != "" {
			ids = append(ids, id)
		}
		if id := d.Anchor.GraphicData.Pic.Blip.Embed; id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}

type wdParagraph struct {
	Runs []wdRun `xml:"r"`
}

func (p wdParagraph) text() string {
	parts := make([]string, len(p.Runs))
	for i, r := range p.Runs {
		parts[i] = strings.Join(r.Text, "")
	}
	return strings.Join(parts, "")
}

func (p wdParagraph) embedIDs() []string {
	var ids []string
	for _, r := range p.Runs {
		ids = append(ids, r.embedIDs()...)
	}
	return ids
}

type wdCell struct {
	Paragraphs []wdParagraph `xml:"p"`
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

// wdRelationships is word/_rels/document.xml.rels — maps the r:embed IDs
// referenced from document.xml to the actual media zip entries.
type wdRelationships struct {
	Relationships []struct {
		ID     string `xml:"Id,attr"`
		Target string `xml:"Target,attr"`
	} `xml:"Relationship"`
}

func readZipEntry(zr *zip.Reader, name string) []byte {
	for _, f := range zr.File {
		if f.Name != name {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil
		}
		defer rc.Close()
		data, err := io.ReadAll(rc)
		if err != nil {
			return nil
		}
		return data
	}
	return nil
}

func parseDocxRels(zr *zip.Reader) map[string]string {
	rels := map[string]string{}
	data := readZipEntry(zr, "word/_rels/document.xml.rels")
	if data == nil {
		return rels
	}
	var parsed wdRelationships
	if xml.Unmarshal(data, &parsed) != nil {
		return rels
	}
	for _, r := range parsed.Relationships {
		rels[r.ID] = r.Target
	}
	return rels
}

// docxImageExtractor turns a paragraph's embed IDs into "[Gambar N]"
// position markers, extracting/deduplicating the referenced image the
// first time it's seen — N is 1-based and matches images[N-1], so the
// marker left in the extracted text stays a true position anchor for
// whichever image ends up attached to the LLM prompt at generation time.
type docxImageExtractor struct {
	zr      *zip.Reader
	rels    map[string]string
	indexOf map[string]int // media zip path -> 1-based image index, for de-duplication
	images  []Image
}

func (e *docxImageExtractor) markersFor(embedIDs []string) string {
	var marker strings.Builder
	for _, rid := range embedIDs {
		if idx := e.resolve(rid); idx > 0 {
			fmt.Fprintf(&marker, " [Gambar %d]", idx)
		}
	}
	return marker.String()
}

func (e *docxImageExtractor) resolve(rid string) int {
	target, ok := e.rels[rid]
	if !ok {
		return 0
	}
	path := "word/" + strings.TrimPrefix(target, "/")
	if idx, ok := e.indexOf[path]; ok {
		return idx
	}
	if len(e.images) >= maxExtractedImages {
		return 0
	}
	contentType, ok := imageContentTypeByExt[extOf(path)]
	if !ok {
		return 0
	}
	data := readZipEntry(e.zr, path)
	if len(data) < minExtractedImageBytes {
		return 0
	}
	e.images = append(e.images, Image{Name: path[strings.LastIndex(path, "/")+1:], Data: data, ContentType: contentType})
	idx := len(e.images)
	e.indexOf[path] = idx
	return idx
}

func extractDOCX(data []byte) (string, []Image, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", nil, newError("Gagal membaca file DOCX.")
	}
	docXML := readZipEntry(zr, "word/document.xml")
	if docXML == nil {
		return "", nil, newError("Gagal membaca file DOCX.")
	}

	var doc wdDocument
	if err := xml.Unmarshal(docXML, &doc); err != nil {
		return "", nil, newError("Gagal membaca file DOCX.")
	}

	extractor := &docxImageExtractor{zr: zr, rels: parseDocxRels(zr), indexOf: map[string]int{}}

	var lines []string
	for _, p := range doc.Body.Paragraphs {
		line := strings.TrimSpace(p.text() + extractor.markersFor(p.embedIDs()))
		if line != "" {
			lines = append(lines, line)
		}
	}
	for _, table := range doc.Body.Tables {
		for _, row := range table.Rows {
			cells := make([]string, len(row.Cells))
			for i, c := range row.Cells {
				var cellLines []string
				for _, p := range c.Paragraphs {
					if line := strings.TrimSpace(p.text() + extractor.markersFor(p.embedIDs())); line != "" {
						cellLines = append(cellLines, line)
					}
				}
				cells[i] = strings.Join(cellLines, "\n")
			}
			lines = append(lines, strings.Join(cells, "\t"))
		}
	}
	return strings.TrimSpace(strings.Join(lines, "\n")), extractor.images, nil
}
