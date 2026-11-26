// Package config reads service settings from the environment.
package config

import (
	"fmt"
	"os"
)

// Config holds runtime settings.
type Config struct {
	Addr        string
	DatabaseURL string
	LLMBaseURL  string
	LLMAPIKey   string
	LLMModel    string
}

// FromEnv builds Config from environment variables, applying defaults.
func FromEnv() (Config, error) {
	cfg := Config{
		Addr:        env("ADDR", ":8080"),
		DatabaseURL: env("DATABASE_URL", "postgres://lk:lk@localhost:55432/lk?sslmode=disable"),
		LLMBaseURL:  env("LLM_BASE_URL", "https://api.deepseek.com/v1"),
		LLMAPIKey:   firstNonEmpty(os.Getenv("LLM_API_KEY"), os.Getenv("DEEPSEEK_API_KEY")),
		LLMModel:    env("LLM_MODEL", "deepseek-flash"),
	}
	if cfg.Addr == "" {
		return Config{}, fmt.Errorf("ADDR must not be empty")
	}
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL must not be empty")
	}
	return cfg, nil
}

func env(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
