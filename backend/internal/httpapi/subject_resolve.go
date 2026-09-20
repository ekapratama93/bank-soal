package httpapi

import (
	"context"
	"net/http"

	"banksoal/internal/store"
)

// resolveSubject accepts either a subject_id (new) or a subject name
// (legacy-compatible) and returns the matching subjects row. Mirrors the
// Python backend's resolve_subject() in routers/materials.py.
func (h *Handlers) resolveSubject(ctx context.Context, subjectID, subjectName string) (*store.Subject, string, int) {
	if subjectID != "" {
		sub, err := h.Store.GetSubjectByID(ctx, subjectID)
		if err != nil {
			return nil, "Terjadi kesalahan pada server.", http.StatusInternalServerError
		}
		if sub == nil {
			return nil, "Mata pelajaran tidak valid", http.StatusUnprocessableEntity
		}
		return sub, "", 0
	}
	if subjectName != "" {
		sub, err := h.Store.GetSubjectByName(ctx, subjectName)
		if err != nil {
			return nil, "Terjadi kesalahan pada server.", http.StatusInternalServerError
		}
		if sub == nil {
			return nil, "Mata pelajaran tidak valid", http.StatusUnprocessableEntity
		}
		return sub, "", 0
	}
	return nil, "subject atau subject_id wajib diisi", http.StatusUnprocessableEntity
}

// validateExamType checks that an exam_type_id refers to a real row.
func (h *Handlers) validateExamType(ctx context.Context, examTypeID string) (string, int) {
	et, err := h.Store.GetExamType(ctx, examTypeID)
	if err != nil {
		return "Terjadi kesalahan pada server.", http.StatusInternalServerError
	}
	if et == nil {
		return "Tipe ujian tidak valid", http.StatusUnprocessableEntity
	}
	return "", 0
}

func displaySubjectName(subjectID *string, names map[string]string) string {
	if subjectID == nil {
		return ""
	}
	return names[*subjectID]
}
