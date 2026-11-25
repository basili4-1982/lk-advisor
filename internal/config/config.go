// Package config reads service settings from the environment.
package config

import (
	"fmt"
	"os"
)

// Config holds runtime settings.
type Config struct {
	Addr string
}

// FromEnv builds Config from environment variables, applying defaults.
func FromEnv() (Config, error) {
	cfg := Config{Addr: env("ADDR", ":8080")}
	if cfg.Addr == "" {
		return Config{}, fmt.Errorf("ADDR must not be empty")
	}
	return cfg, nil
}

func env(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}
