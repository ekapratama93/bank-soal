package httpapi

import "net/http"

// NewRouter builds the full route table. Go 1.22+'s ServeMux resolves the
// most specific registered pattern for a given request regardless of
// registration order (a literal segment always beats a {wildcard} one at
// the same position), so static routes like /api/quiz/available and
// dynamic ones like /api/quiz/{quiz_id} can be registered in any order —
// unlike the FastAPI version, where route declaration order mattered.
func NewRouter(h *Handlers) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	mux.HandleFunc("POST /api/auth/login", h.handleLogin)

	mux.HandleFunc("GET /api/subjects", h.handleListSubjects)
	mux.HandleFunc("POST /api/subjects", h.handleCreateSubject)
	mux.HandleFunc("PATCH /api/subjects/{id}", h.handleRenameSubject)
	mux.HandleFunc("DELETE /api/subjects/{id}", h.handleDeleteSubject)

	mux.HandleFunc("GET /api/exam-types", h.handleListExamTypes)
	mux.HandleFunc("POST /api/exam-types", h.handleCreateExamType)
	mux.HandleFunc("PATCH /api/exam-types/{id}", h.handleUpdateExamType)
	mux.HandleFunc("DELETE /api/exam-types/{id}", h.handleDeleteExamType)

	mux.HandleFunc("GET /api/materials", h.handleListMaterials)
	mux.HandleFunc("POST /api/materials", h.handleCreateMaterial)
	mux.HandleFunc("POST /api/materials/upload", h.handleUploadMaterial)
	mux.HandleFunc("PATCH /api/materials/{id}", h.handleUpdateMaterial)
	mux.HandleFunc("DELETE /api/materials/{id}", h.handleDeleteMaterial)

	mux.HandleFunc("POST /api/quiz/generate", h.handleGenerateQuiz)
	mux.HandleFunc("POST /api/quiz/request", h.handleRequestQuiz)
	mux.HandleFunc("GET /api/quiz/available", h.handleAvailable)
	mux.HandleFunc("GET /api/quiz/attempts", h.handleListAttempts)
	mux.HandleFunc("GET /api/quiz/admin/list", h.handleAdminListQuizzes)
	mux.HandleFunc("GET /api/quiz/admin/quizzes/{quiz_id}", h.handleAdminGetQuiz)
	mux.HandleFunc("PATCH /api/quiz/admin/quizzes/{quiz_id}", h.handleAdminUpdateQuiz)
	mux.HandleFunc("DELETE /api/quiz/admin/quizzes/{quiz_id}", h.handleAdminDeleteQuiz)
	mux.HandleFunc("POST /api/quiz/admin/quizzes/bulk-delete", h.handleAdminBulkDeleteQuizzes)
	mux.HandleFunc("POST /api/quiz/pool/reset", h.handlePoolReset)
	mux.HandleFunc("GET /api/quiz/{quiz_id}", h.handleGetQuiz)
	mux.HandleFunc("POST /api/quiz/{quiz_id}/submit", h.handleSubmitQuiz)

	return withRecovery(withCORS(h.FrontendOrigin, mux))
}
