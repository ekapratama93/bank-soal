package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"banksoal/internal/db"
	"banksoal/internal/store"
)

var validQtypes = map[string]bool{"pilihan_ganda": true, "benar_salah": true, "isian": true, "deskripsi": true}

func validQtypesJoined() string {
	return "pilihan_ganda, benar_salah, isian, deskripsi"
}

func validateTipeSoal(tipeSoal map[string]int) (map[string]int, error) {
	if tipeSoal == nil {
		return nil, nil
	}
	total := 0
	for tipe, jumlah := range tipeSoal {
		if !validQtypes[tipe] {
			return nil, fmt.Errorf("Tipe soal tidak valid: %s. Pilihan: %s", tipe, validQtypesJoined())
		}
		if jumlah < 0 || jumlah > 50 {
			return nil, fmt.Errorf("Jumlah soal '%s' harus 0-50", tipe)
		}
		total += jumlah
	}
	if total < 5 || total > 50 {
		return nil, fmt.Errorf("Total jumlah soal dari komposisi tipe soal harus 5-50")
	}
	out := map[string]int{}
	for t, j := range tipeSoal {
		if j > 0 {
			out[t] = j
		}
	}
	return out, nil
}

func validatePoinPerTipe(poin map[string]int) (map[string]int, error) {
	if poin == nil {
		return nil, nil
	}
	out := map[string]int{}
	for tipe, nilaiPoin := range poin {
		if !validQtypes[tipe] {
			return nil, fmt.Errorf("Tipe soal tidak valid: %s. Pilihan: %s", tipe, validQtypesJoined())
		}
		if nilaiPoin < 1 || nilaiPoin > 100 {
			return nil, fmt.Errorf("Poin untuk '%s' harus 1-100", tipe)
		}
		out[tipe] = nilaiPoin
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

func (h *Handlers) handleListExamTypes(w http.ResponseWriter, r *http.Request) {
	types, err := h.Store.ListExamTypes(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Terjadi kesalahan pada server.")
		return
	}
	writeJSON(w, http.StatusOK, types)
}

type examTypeRequest struct {
	Name        string         `json:"name"`
	JumlahSoal  *int           `json:"jumlah_soal"`
	DurasiMenit *int           `json:"durasi_menit"`
	TipeSoal    map[string]int `json:"tipe_soal"`
	PoinPerTipe map[string]int `json:"poin_per_tipe"`
}

func (h *Handlers) handleCreateExamType(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	var body examTypeRequest
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.JumlahSoal != nil && (*body.JumlahSoal < 5 || *body.JumlahSoal > 50) {
		writeError(w, http.StatusUnprocessableEntity, "Jumlah soal harus 5-50")
		return
	}
	if body.DurasiMenit != nil && (*body.DurasiMenit < 10 || *body.DurasiMenit > 180) {
		writeError(w, http.StatusUnprocessableEntity, "Durasi harus 10-180 menit")
		return
	}
	name := strings.TrimSpace(body.Name)
	if name == "" {
		writeError(w, http.StatusUnprocessableEntity, "Nama tipe ujian kosong")
		return
	}
	existing, err := h.Store.GetExamTypeByName(r.Context(), name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Terjadi kesalahan pada server.")
		return
	}
	if existing != nil {
		writeError(w, http.StatusConflict, "Tipe ujian sudah ada")
		return
	}
	tipeSoal, err := validateTipeSoal(body.TipeSoal)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	poinPerTipe, err := validatePoinPerTipe(body.PoinPerTipe)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	et, err := h.Store.CreateExamType(r.Context(), store.ExamTypeInput{
		Name: name, JumlahSoal: body.JumlahSoal, DurasiMenit: body.DurasiMenit,
		TipeSoal: tipeSoal, PoinPerTipe: poinPerTipe,
	})
	if err != nil {
		if db.IsUniqueViolation(err) {
			writeError(w, http.StatusConflict, "Tipe ujian sudah ada")
			return
		}
		writeError(w, http.StatusInternalServerError, "Terjadi kesalahan pada server.")
		return
	}
	writeJSON(w, http.StatusCreated, et)
}

func (h *Handlers) handleUpdateExamType(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	id := r.PathValue("id")
	existing, err := h.Store.GetExamType(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Terjadi kesalahan pada server.")
		return
	}
	if existing == nil {
		writeError(w, http.StatusNotFound, "Tipe ujian tidak ditemukan")
		return
	}

	bodyBytes, err := readAndRestoreBody(r)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "Body permintaan tidak valid")
		return
	}
	var fieldsSent map[string]json.RawMessage
	if err := json.Unmarshal(bodyBytes, &fieldsSent); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "Body permintaan tidak valid")
		return
	}
	var body examTypeRequest
	if err := json.Unmarshal(bodyBytes, &body); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "Body permintaan tidak valid")
		return
	}

	var upd store.ExamTypeUpdate
	if _, sent := fieldsSent["name"]; sent {
		name := strings.TrimSpace(body.Name)
		if name == "" {
			writeError(w, http.StatusUnprocessableEntity, "Nama tipe ujian kosong")
			return
		}
		duplicate, err := h.Store.GetExamTypeByName(r.Context(), name)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Terjadi kesalahan pada server.")
			return
		}
		if duplicate != nil && duplicate.ID != id {
			writeError(w, http.StatusConflict, "Tipe ujian sudah ada")
			return
		}
		upd.Name = &name
	}
	if _, sent := fieldsSent["jumlah_soal"]; sent {
		if body.JumlahSoal != nil && (*body.JumlahSoal < 5 || *body.JumlahSoal > 50) {
			writeError(w, http.StatusUnprocessableEntity, "Jumlah soal harus 5-50")
			return
		}
		v := body.JumlahSoal
		upd.JumlahSoal = &v
	}
	if _, sent := fieldsSent["durasi_menit"]; sent {
		if body.DurasiMenit != nil && (*body.DurasiMenit < 10 || *body.DurasiMenit > 180) {
			writeError(w, http.StatusUnprocessableEntity, "Durasi harus 10-180")
			return
		}
		v := body.DurasiMenit
		upd.DurasiMenit = &v
	}
	if _, sent := fieldsSent["tipe_soal"]; sent {
		tipeSoal, err := validateTipeSoal(body.TipeSoal)
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		upd.TipeSoal = &tipeSoal
	}
	if _, sent := fieldsSent["poin_per_tipe"]; sent {
		poinPerTipe, err := validatePoinPerTipe(body.PoinPerTipe)
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		upd.PoinPerTipe = &poinPerTipe
	}

	et, err := h.Store.UpdateExamType(r.Context(), id, upd)
	if err != nil {
		if db.IsUniqueViolation(err) {
			writeError(w, http.StatusConflict, "Tipe ujian sudah ada")
			return
		}
		writeError(w, http.StatusInternalServerError, "Terjadi kesalahan pada server.")
		return
	}
	writeJSON(w, http.StatusOK, et)
}

func (h *Handlers) handleDeleteExamType(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	id := r.PathValue("id")
	existing, err := h.Store.GetExamType(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Terjadi kesalahan pada server.")
		return
	}
	if existing == nil {
		writeError(w, http.StatusNotFound, "Tipe ujian tidak ditemukan")
		return
	}
	usedMaterials, err := h.Store.ExamTypeUsedByMaterials(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Terjadi kesalahan pada server.")
		return
	}
	if usedMaterials {
		writeError(w, http.StatusConflict, "Tipe ujian masih dipakai materi. Pindahkan atau hapus materinya dulu.")
		return
	}
	usedQuizzes, err := h.Store.ExamTypeUsedByQuizzes(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Terjadi kesalahan pada server.")
		return
	}
	if usedQuizzes {
		writeError(w, http.StatusConflict, "Tipe ujian masih dipakai kuis. Reset pool dulu sebelum menghapus.")
		return
	}
	if err := h.Store.DeleteExamType(r.Context(), id); err != nil {
		if db.IsForeignKeyViolation(err) {
			writeError(w, http.StatusConflict, "Tipe ujian masih dipakai materi atau kuis. Coba lagi.")
			return
		}
		writeError(w, http.StatusInternalServerError, "Terjadi kesalahan pada server.")
		return
	}
	writeNoContent(w)
}

// parseIntQuery parses an optional integer query parameter, returning nil
// if absent, or an error message if present-but-invalid.
func parseIntQuery(r *http.Request, key string) (*int, bool) {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return nil, true
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return nil, false
	}
	return &v, true
}
