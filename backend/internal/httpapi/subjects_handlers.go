package httpapi

import (
	"net/http"
	"strings"

	"banksoal/internal/db"
)

type subjectRequest struct {
	Name string `json:"name"`
}

func (h *Handlers) handleListSubjects(w http.ResponseWriter, r *http.Request) {
	subjects, err := h.Store.ListSubjects(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Terjadi kesalahan pada server.")
		return
	}
	writeJSON(w, http.StatusOK, subjects)
}

func (h *Handlers) handleCreateSubject(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	var body subjectRequest
	if !decodeJSON(w, r, &body) {
		return
	}
	name := strings.TrimSpace(body.Name)
	if name == "" {
		writeError(w, http.StatusUnprocessableEntity, "Nama mata pelajaran kosong")
		return
	}
	existing, err := h.Store.GetSubjectByName(r.Context(), name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Terjadi kesalahan pada server.")
		return
	}
	if existing != nil {
		writeError(w, http.StatusConflict, "Mata pelajaran sudah ada")
		return
	}
	sub, err := h.Store.CreateSubject(r.Context(), name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Terjadi kesalahan pada server.")
		return
	}
	writeJSON(w, http.StatusCreated, sub)
}

func (h *Handlers) handleRenameSubject(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	id := r.PathValue("id")
	existing, err := h.Store.GetSubjectByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Terjadi kesalahan pada server.")
		return
	}
	if existing == nil {
		writeError(w, http.StatusNotFound, "Mata pelajaran tidak ditemukan")
		return
	}
	var body subjectRequest
	if !decodeJSON(w, r, &body) {
		return
	}
	name := strings.TrimSpace(body.Name)
	if name == "" {
		writeError(w, http.StatusUnprocessableEntity, "Nama mata pelajaran kosong")
		return
	}
	duplicate, err := h.Store.GetSubjectByName(r.Context(), name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Terjadi kesalahan pada server.")
		return
	}
	if duplicate != nil && duplicate.ID != id {
		writeError(w, http.StatusConflict, "Mata pelajaran sudah ada")
		return
	}
	sub, err := h.Store.RenameSubject(r.Context(), id, name)
	if err != nil {
		if db.IsUniqueViolation(err) {
			writeError(w, http.StatusConflict, "Mata pelajaran sudah ada")
			return
		}
		writeError(w, http.StatusInternalServerError, "Terjadi kesalahan pada server.")
		return
	}
	writeJSON(w, http.StatusOK, sub)
}

func (h *Handlers) handleDeleteSubject(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	id := r.PathValue("id")
	existing, err := h.Store.GetSubjectByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Terjadi kesalahan pada server.")
		return
	}
	if existing == nil {
		writeError(w, http.StatusNotFound, "Mata pelajaran tidak ditemukan")
		return
	}
	usedMaterials, err := h.Store.SubjectUsedByMaterials(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Terjadi kesalahan pada server.")
		return
	}
	if usedMaterials {
		writeError(w, http.StatusConflict, "Mata pelajaran masih dipakai materi. Hapus materinya dulu.")
		return
	}
	usedQuizzes, err := h.Store.SubjectUsedByQuizzes(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Terjadi kesalahan pada server.")
		return
	}
	if usedQuizzes {
		writeError(w, http.StatusConflict, "Mata pelajaran masih dipakai kuis. Reset pool dulu sebelum menghapus.")
		return
	}
	if err := h.Store.DeleteSubject(r.Context(), id); err != nil {
		if db.IsForeignKeyViolation(err) {
			writeError(w, http.StatusConflict, "Mata pelajaran masih dipakai materi atau kuis. Coba lagi.")
			return
		}
		writeError(w, http.StatusInternalServerError, "Terjadi kesalahan pada server.")
		return
	}
	writeNoContent(w)
}
