// Command server runs the Bank Soal API — a Go rewrite of the FastAPI
// backend, talking directly to Postgres (pgx) and to Supabase's Auth/
// Storage REST APIs (for the pieces that have no direct-Postgres
// equivalent).
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"banksoal/internal/config"
	"banksoal/internal/db"
	"banksoal/internal/httpapi"
	"banksoal/internal/llm"
	"banksoal/internal/store"
	"banksoal/internal/supabaseauth"
)

func main() {
	if err := run(); err != nil {
		slog.Error("server gagal dijalankan", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	st := store.New(pool)
	auth := supabaseauth.New(cfg.SupabaseURL, cfg.SupabaseServiceKey)
	llmClient := llm.NewClient(cfg.OpenRouterAPIKey, cfg.OpenRouterModel, cfg.OpenRouterImageModel, cfg.OpenRouterURL)
	handlers := httpapi.New(st, auth, llmClient, cfg.FrontendOrigin)
	router := httpapi.NewRouter(handlers)

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: router,
	}

	serveErr := make(chan error, 1)
	go func() {
		slog.Info("listening", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serveErr <- err
		}
	}()

	select {
	case err := <-serveErr:
		return err
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}
