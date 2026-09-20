package fileextract

import (
	"bytes"
	"unicode/utf8"

	"golang.org/x/text/encoding/charmap"
)

var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// extractTXT mirrors the Python backend's encoding fallback list:
// utf-8-sig, then utf-8, then cp1256 (Windows Arabic — some material files
// with Arabic quotes were saved from older Windows text editors in this
// encoding).
func extractTXT(data []byte) (string, error) {
	if bytes.HasPrefix(data, utf8BOM) {
		stripped := data[len(utf8BOM):]
		if utf8.Valid(stripped) {
			return string(stripped), nil
		}
	}
	if utf8.Valid(data) {
		return string(data), nil
	}
	decoded, err := charmap.Windows1256.NewDecoder().Bytes(data)
	if err == nil {
		return string(decoded), nil
	}
	return "", newError("File teks tidak dapat dibaca.")
}
