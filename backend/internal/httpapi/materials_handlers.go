package httpapi

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"strings"

	"banksoal/internal/fileextract"
	"banksoal/internal/store"
)

func (h *Handlers) decorateMaterial(ctx context.Context, m *store.Material) error {
	if m.SubjectID == nil {
		return nil
	}
	names, err := h.Store.SubjectNamesMap(ctx)
	if err != nil {
		return err
	}
	m.Subject = names[*m.SubjectID]
	return nil
}

func (h *Handlers) handleListMaterials(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	ctx := r.Context()
	var filter store.MaterialFilter
	if subjectID := r.URL.Query().Get("subject_id"); subjectID != "" {
		filter.SubjectID = &subjectID
	} else if subjectName := r.URL.Query().Get("subject"); subjectName != "" {
		sub, errMsg, status := h.resolveSubject(ctx, "", subjectName)
		if errMsg != "" {
			writeError(w, status, errMsg)
			return
		}
		filter.SubjectID = &sub.ID
	}
	if grade, ok := parseIntQuery(r, "grade"); ok {
		filter.Grade = grade
	} else {
		writeError(w, http.StatusUnprocessableEntity, "Parameter grade tidak valid")
		return
	}

	materials, err := h.Store.ListMaterials(ctx, filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Terjadi kesalahan pada server.")
		return
	}
	names, err := h.Store.SubjectNamesMap(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Terjadi kesalahan pada server.")
		return
	}
	for i := range materials {
		materials[i].Subject = displaySubjectName(materials[i].SubjectID, names)
	}
	writeJSON(w, http.StatusOK, materials)
}

type materialRequest struct {
	SubjectID  string `json:"subject_id"`
	Subject    string `json:"subject"`
	Grade      int    `json:"grade"`
	ExamTypeID string `json:"exam_type_id"`
	Title      string `json:"title"`
	Content    string `json:"content"`
}

func (h *Handlers) handleCreateMaterial(w http.ResponseWriter, r *http.Request) {
	admin, ok := h.requireAdmin(w, r)
	if !ok {
		return
	}
	var body materialRequest
	if !decodeJSON(w, r, &body) {
		return
	}
	ctx := r.Context()
	sub, errMsg, status := h.resolveSubject(ctx, body.SubjectID, body.Subject)
	if errMsg != "" {
		writeError(w, status, errMsg)
		return
	}
	if errMsg, status := h.validateExamType(ctx, body.ExamTypeID); errMsg != "" {
		writeError(w, status, errMsg)
		return
	}
	material, err := h.Store.CreateMaterial(ctx, store.MaterialInput{
		SubjectID: sub.ID, Grade: body.Grade, ExamTypeID: body.ExamTypeID,
		Title: body.Title, Content: body.Content, CreatedBy: admin.Email,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Terjadi kesalahan pada server.")
		return
	}
	if _, err := h.Store.DeletePoolForCombo(ctx, sub.ID, body.Grade, body.ExamTypeID); err != nil {
		writeError(w, http.StatusInternalServerError, "Terjadi kesalahan pada server.")
		return
	}
	material.Subject = sub.Name
	writeJSON(w, http.StatusCreated, material)
}

const maxUploadMemory = 4 * 1024 * 1024 // form parts buffered in memory beyond this spill to temp files

func (h *Handlers) handleUploadMaterial(w http.ResponseWriter, r *http.Request) {
	admin, ok := h.requireAdmin(w, r)
	if !ok {
		return
	}
	if err := r.ParseMultipartForm(maxUploadMemory); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "Form permintaan tidak valid")
		return
	}
	ctx := r.Context()

	gradeStr := r.FormValue("grade")
	grade, gradeOK := parseGrade(gradeStr)
	if !gradeOK {
		writeError(w, http.StatusUnprocessableEntity, "Kelas tidak valid")
		return
	}
	examTypeID := r.FormValue("exam_type_id")
	subjectID := r.FormValue("subject_id")
	subjectName := r.FormValue("subject")
	title := r.FormValue("title")
	content := r.FormValue("content")

	sub, errMsg, status := h.resolveSubject(ctx, subjectID, subjectName)
	if errMsg != "" {
		writeError(w, status, errMsg)
		return
	}
	if errMsg, status := h.validateExamType(ctx, examTypeID); errMsg != "" {
		writeError(w, status, errMsg)
		return
	}

	var parts []string
	var fileName *string
	if file, header, err := r.FormFile("file"); err == nil {
		defer file.Close()
		data := make([]byte, fileextract.MaxFileBytes+1)
		n, _ := io.ReadFull(file, data)
		data = data[:n]
		if len(data) > fileextract.MaxFileBytes {
			writeError(w, http.StatusUnprocessableEntity, "Ukuran file melebihi 2 MB.")
			return
		}
		text, err := fileextract.Extract(header.Filename, data)
		if err != nil {
			if strings.TrimSpace(content) == "" {
				writeError(w, http.StatusUnprocessableEntity, err.Error())
				return
			}
		} else {
			parts = append(parts, text)
			name := header.Filename
			fileName = &name
		}
	}
	if strings.TrimSpace(content) != "" {
		parts = append(parts, strings.TrimSpace(content))
	}
	if len(parts) == 0 {
		writeError(w, http.StatusUnprocessableEntity, "Isi materi kosong. Unggah file atau tulis isi materi.")
		return
	}
	fullContent := strings.Join(parts, "\n\n")

	finalTitle := strings.TrimSpace(title)
	if finalTitle == "" {
		base := "materi"
		if fileName != nil {
			base = *fileName
			if idx := strings.LastIndex(base, "."); idx > 0 {
				base = base[:idx]
			}
		}
		finalTitle = base
	}

	material, err := h.Store.CreateMaterial(ctx, store.MaterialInput{
		SubjectID: sub.ID, Grade: grade, ExamTypeID: examTypeID,
		Title: finalTitle, Content: fullContent, FileName: fileName, CreatedBy: admin.Email,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Terjadi kesalahan pada server.")
		return
	}
	if _, err := h.Store.DeletePoolForCombo(ctx, sub.ID, grade, examTypeID); err != nil {
		writeError(w, http.StatusInternalServerError, "Terjadi kesalahan pada server.")
		return
	}
	material.Subject = sub.Name
	writeJSON(w, http.StatusCreated, material)
}

func parseGrade(s string) (int, bool) {
	v, err := strconv.Atoi(s)
	if err != nil || v < 1 || v > 12 {
		return 0, false
	}
	return v, true
}

type materialUpdateRequest struct {
	SubjectID  *string `json:"subject_id"`
	Subject    *string `json:"subject"`
	Grade      *int    `json:"grade"`
	ExamTypeID *string `json:"exam_type_id"`
	Title      *string `json:"title"`
	Content    *string `json:"content"`
}

func (h *Handlers) handleUpdateMaterial(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	ctx := r.Context()
	id := r.PathValue("id")
	current, err := h.Store.GetMaterial(ctx, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Terjadi kesalahan pada server.")
		return
	}
	if current == nil {
		writeError(w, http.StatusNotFound, "Materi tidak ditemukan")
		return
	}
	var body materialUpdateRequest
	if !decodeJSON(w, r, &body) {
		return
	}

	var upd store.MaterialUpdate
	newSubjectID := current.SubjectID
	if body.SubjectID != nil || body.Subject != nil {
		subID, subName := "", ""
		if body.SubjectID != nil {
			subID = *body.SubjectID
		}
		if body.Subject != nil {
			subName = *body.Subject
		}
		sub, errMsg, status := h.resolveSubject(ctx, subID, subName)
		if errMsg != "" {
			writeError(w, status, errMsg)
			return
		}
		if current.SubjectID == nil || sub.ID != *current.SubjectID {
			upd.SubjectID = &sub.ID
		}
		newSubjectID = &sub.ID
	}
	newGrade := current.Grade
	if body.Grade != nil && *body.Grade != current.Grade {
		upd.Grade = body.Grade
		newGrade = *body.Grade
	}
	newExamTypeID := current.ExamTypeID
	if body.ExamTypeID != nil && *body.ExamTypeID != current.ExamTypeID {
		if errMsg, status := h.validateExamType(ctx, *body.ExamTypeID); errMsg != "" {
			writeError(w, status, errMsg)
			return
		}
		upd.ExamTypeID = body.ExamTypeID
		newExamTypeID = *body.ExamTypeID
	}
	if body.Title != nil {
		title := strings.TrimSpace(*body.Title)
		if title == "" {
			writeError(w, http.StatusUnprocessableEntity, "Judul materi kosong")
			return
		}
		upd.Title = &title
	}
	if body.Content != nil {
		if strings.TrimSpace(*body.Content) == "" {
			writeError(w, http.StatusUnprocessableEntity, "Isi materi kosong")
			return
		}
		upd.Content = body.Content
	}

	changed := upd.SubjectID != nil || upd.Grade != nil || upd.ExamTypeID != nil || upd.Title != nil || upd.Content != nil
	if changed {
		if err := h.Store.UpdateMaterial(ctx, id, upd); err != nil {
			writeError(w, http.StatusInternalServerError, "Terjadi kesalahan pada server.")
			return
		}
		if newSubjectID != nil {
			if _, err := h.Store.DeletePoolForCombo(ctx, *newSubjectID, newGrade, newExamTypeID); err != nil {
				writeError(w, http.StatusInternalServerError, "Terjadi kesalahan pada server.")
				return
			}
		}
		if (upd.SubjectID != nil || upd.Grade != nil || upd.ExamTypeID != nil) && current.SubjectID != nil {
			// Combo moved — also clear the OLD combo's pool.
			if _, err := h.Store.DeletePoolForCombo(ctx, *current.SubjectID, current.Grade, current.ExamTypeID); err != nil {
				writeError(w, http.StatusInternalServerError, "Terjadi kesalahan pada server.")
				return
			}
		}
	}

	updated, err := h.Store.GetMaterialWithExamType(ctx, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Terjadi kesalahan pada server.")
		return
	}
	if updated == nil {
		writeJSON(w, http.StatusOK, nil)
		return
	}
	if err := h.decorateMaterial(ctx, updated); err != nil {
		writeError(w, http.StatusInternalServerError, "Terjadi kesalahan pada server.")
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (h *Handlers) handleDeleteMaterial(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	ctx := r.Context()
	id := r.PathValue("id")
	current, err := h.Store.GetMaterial(ctx, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Terjadi kesalahan pada server.")
		return
	}
	if current == nil {
		writeError(w, http.StatusNotFound, "Materi tidak ditemukan")
		return
	}
	if err := h.Store.DeleteMaterial(ctx, id); err != nil {
		writeError(w, http.StatusInternalServerError, "Terjadi kesalahan pada server.")
		return
	}
	if current.SubjectID != nil {
		if _, err := h.Store.DeletePoolForCombo(ctx, *current.SubjectID, current.Grade, current.ExamTypeID); err != nil {
			writeError(w, http.StatusInternalServerError, "Terjadi kesalahan pada server.")
			return
		}
	}
	writeNoContent(w)
}
