// Package config loads server configuration from environment variables
// (optionally via a .env file), mirroring the Python backend's
// pydantic-settings Settings class.
package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

type Config struct {
	OpenRouterAPIKey     string
	OpenRouterModel      string
	OpenRouterImageModel string
	OpenRouterURL        string
	OpenRouterImagesURL  string
	SupabaseURL          string
	SupabaseServiceKey   string
	DatabaseURL          string
	FrontendOrigin       string
	Port                 string
}

// Load reads a .env file (if present, without overriding already-set
// real environment variables) and builds a Config, applying the same
// fail-fast validation as the Python backend: a non-empty URL-shaped
// setting must actually look like a URL.
func Load() (*Config, error) {
	loadDotEnv(".env")

	cfg := &Config{
		OpenRouterAPIKey:     os.Getenv("OPENROUTER_API_KEY"),
		OpenRouterModel:      getEnvDefault("OPENROUTER_MODEL", "z-ai/glm-4.5-air:free"),
		OpenRouterImageModel: getEnvDefault("OPENROUTER_IMAGE_MODEL", "google/gemini-2.5-flash-image-preview:free"),
		OpenRouterURL:        getEnvDefault("OPENROUTER_URL", "https://openrouter.ai/api/v1/chat/completions"),
		OpenRouterImagesURL:  getEnvDefault("OPENROUTER_IMAGES_URL", "https://openrouter.ai/api/v1/images"),
		SupabaseURL:          os.Getenv("SUPABASE_URL"),
		SupabaseServiceKey:   os.Getenv("SUPABASE_SERVICE_KEY"),
		DatabaseURL:          os.Getenv("DATABASE_URL"),
		FrontendOrigin:       getEnvDefault("FRONTEND_ORIGIN", "http://localhost:5173"),
		Port:                 getEnvDefault("PORT", "8000"),
	}

	if err := mustBeHTTPURLIfSet("SUPABASE_URL", cfg.SupabaseURL); err != nil {
		return nil, err
	}
	if err := mustBeHTTPURLIfSet("OPENROUTER_URL", cfg.OpenRouterURL); err != nil {
		return nil, err
	}
	if err := mustBeHTTPURLIfSet("OPENROUTER_IMAGES_URL", cfg.OpenRouterImagesURL); err != nil {
		return nil, err
	}
	return cfg, nil
}

func mustBeHTTPURLIfSet(name, v string) error {
	if v == "" {
		return nil
	}
	if !strings.HasPrefix(v, "http://") && !strings.HasPrefix(v, "https://") {
		return fmt.Errorf("%s harus berupa URL http(s):// yang valid, dapat: %q", name, v)
	}
	return nil
}

func getEnvDefault(key, def string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return def
}

// loadDotEnv is a tiny .env parser (KEY=VALUE per line, '#' comments,
// optional surrounding quotes) — real environment variables always win.
func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if len(value) >= 2 {
			if (value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'') {
				value = value[1 : len(value)-1]
			}
		}
		if _, exists := os.LookupEnv(key); !exists {
			os.Setenv(key, value)
		}
	}
}
