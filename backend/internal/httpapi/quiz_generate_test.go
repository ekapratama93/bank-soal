package httpapi

import (
	"strings"
	"testing"

	"banksoal/internal/store"
)

func TestBuildMaterialContextRenumbersMarkersAcrossMaterials(t *testing.T) {
	materials := []store.Material{
		{
			Title:   "Bab 1",
			Content: "Sebelum. [Gambar 1] Sesudah.",
			Images:  []store.MaterialImage{{URL: "https://x/a.png", Name: "a.png"}},
		},
		{
			Title:   "Bab 2",
			Content: "Diagram ini penting. [Gambar 1] Lanjutan.",
			Images:  []store.MaterialImage{{URL: "https://x/b.png", Name: "b.png"}},
		},
	}

	text, images := buildMaterialContext(materials)

	if len(images) != 2 || images[0].URL != "https://x/a.png" || images[1].URL != "https://x/b.png" {
		t.Fatalf("images = %+v", images)
	}
	// Bab 1's local "[Gambar 1]" stays index 1 (offset 0), but Bab 2's own
	// local "[Gambar 1]" must become global index 2 — otherwise the LLM
	// would see two different "[Gambar 1]" markers pointing at two
	// different attached images.
	if !strings.Contains(text, "Sebelum. [Gambar 1] Sesudah.") {
		t.Errorf("Bab 1 marker not preserved as index 1: %q", text)
	}
	if !strings.Contains(text, "Diagram ini penting. [Gambar 2] Lanjutan.") {
		t.Errorf("Bab 2 marker not renumbered to index 2: %q", text)
	}
}

func TestBuildMaterialContextDropsMarkersBeyondCap(t *testing.T) {
	var images []store.MaterialImage
	for i := 0; i < maxMaterialImagesForPrompt; i++ {
		images = append(images, store.MaterialImage{URL: "https://x/filler.png", Name: "filler.png"})
	}
	materials := []store.Material{
		{Title: "Bab 1", Content: "Isi materi.", Images: images},
		{
			Title:   "Bab 2",
			Content: "Gambar ini tidak terkirim. [Gambar 1] Lanjutan.",
			Images:  []store.MaterialImage{{URL: "https://x/overflow.png", Name: "overflow.png"}},
		},
	}

	text, gotImages := buildMaterialContext(materials)

	if len(gotImages) != maxMaterialImagesForPrompt {
		t.Fatalf("images = %d, want capped at %d", len(gotImages), maxMaterialImagesForPrompt)
	}
	if strings.Contains(text, "[Gambar") {
		t.Errorf("marker for an uncapped-out image should have been dropped: %q", text)
	}
	if !strings.Contains(text, "Gambar ini tidak terkirim.  Lanjutan.") &&
		!strings.Contains(text, "Gambar ini tidak terkirim. Lanjutan.") {
		t.Errorf("surrounding text should survive marker removal: %q", text)
	}
}
