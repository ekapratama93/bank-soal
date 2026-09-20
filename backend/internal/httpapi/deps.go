package httpapi

import (
	"banksoal/internal/llm"
	"banksoal/internal/store"
	"banksoal/internal/supabaseauth"
)

// Handlers holds every dependency the HTTP handlers need — the Go
// equivalent of the Python routers each importing get_supabase() etc.
type Handlers struct {
	Store          *store.Store
	Auth           *supabaseauth.Client
	LLM            *llm.Client
	FrontendOrigin string
	LoginLimiter   *LoginLimiter
}

func New(st *store.Store, auth *supabaseauth.Client, llmClient *llm.Client, frontendOrigin string) *Handlers {
	return &Handlers{
		Store:          st,
		Auth:           auth,
		LLM:            llmClient,
		FrontendOrigin: frontendOrigin,
		LoginLimiter:   NewLoginLimiter(),
	}
}
