// Command lk-advisor starts the recommendation service.
package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/lib/pq"

	"github.com/basili4-1982/lk-advisor/internal/config"
	"github.com/basili4-1982/lk-advisor/internal/httpapi"
	"github.com/basili4-1982/lk-advisor/internal/llm"
	"github.com/basili4-1982/lk-advisor/internal/store"
)

func main() {
	cmd := "serve"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := config.FromEnv()
	if err != nil {
		logger.Error("config", "err", err)
		os.Exit(1)
	}

	switch cmd {
	case "serve":
		serve(logger, cfg)
	case "migrate":
		if err := migrate(cfg); err != nil {
			logger.Error("migrate", "err", err)
			os.Exit(1)
		}
		logger.Info("migrations applied")
	case "seed":
		if err := runSeed(cfg); err != nil {
			logger.Error("seed", "err", err)
			os.Exit(1)
		}
	case "eval":
		if err := runEval(cfg); err != nil {
			logger.Error("eval", "err", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "usage: %s [serve|migrate|seed|eval]\n", os.Args[0])
		os.Exit(2)
	}
}

func openDB(cfg config.Config) (*sql.DB, error) {
	dbc, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := dbc.PingContext(ctx); err != nil {
		_ = dbc.Close()
		return nil, fmt.Errorf("ping db: %w", err)
	}
	return dbc, nil
}

func migrate(cfg config.Config) error {
	dbc, err := openDB(cfg)
	if err != nil {
		return err
	}
	defer dbc.Close()
	return store.Migrate(context.Background(), dbc)
}

func serve(logger *slog.Logger, cfg config.Config) {
	dbc, err := openDB(cfg)
	if err != nil {
		logger.Error("db", "err", err)
		os.Exit(1)
	}
	defer dbc.Close()

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           httpapi.NewWithLLM(logger, dbc, time.Now, llm.New(cfg.LLMBaseURL, cfg.LLMAPIKey, cfg.LLMModel)),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("listening", "addr", cfg.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("serve", "err", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("shutdown", "err", err)
	}
}
