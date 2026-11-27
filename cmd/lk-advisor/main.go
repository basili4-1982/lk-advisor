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
	"strconv"
	"strings"
	"syscall"
	"time"

	_ "github.com/lib/pq"

	"github.com/basili4-1982/lk-advisor/internal/catalog"
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
	case "recompute":
		if err := runRecompute(logger, cfg); err != nil {
			logger.Error("recompute", "err", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "usage: %s [serve|migrate|seed|eval|recompute]\n", os.Args[0])
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

func runRecompute(logger *slog.Logger, cfg config.Config) error {
	dbc, err := openDB(cfg)
	if err != nil {
		return err
	}
	defer dbc.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	top := 10
	if raw := os.Getenv("RECOMPUTE_TOP"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 && v <= 36 {
			top = v
		}
	}
	res, err := catalog.RebuildClientRecommendations(ctx, dbc, top)
	if err != nil {
		return err
	}
	logger.Info("recommendations recomputed", "clients", res.Clients, "rows", res.Rows, "took", res.Took.String())
	return nil
}

func startRecomputeScheduler(ctx context.Context, logger *slog.Logger, dbc *sql.DB) {
	raw := os.Getenv("RECOMPUTE_INTERVAL")
	if raw == "" {
		raw = "24h"
	}
	if raw == "0" || strings.EqualFold(raw, "off") {
		return
	}
	interval, err := time.ParseDuration(raw)
	if err != nil || interval <= 0 {
		logger.Warn("recompute scheduler disabled", "interval", raw, "err", err)
		return
	}
	run := func() {
		runCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
		defer cancel()
		res, err := catalog.RebuildClientRecommendations(runCtx, dbc, 10)
		if err != nil {
			logger.Error("recompute", "err", err)
			return
		}
		logger.Info("recommendations recomputed", "clients", res.Clients, "rows", res.Rows, "took", res.Took.String())
	}
	if os.Getenv("RECOMPUTE_ON_START") == "1" {
		go run()
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				run()
			case <-ctx.Done():
				return
			}
		}
	}()
	logger.Info("recompute scheduler started", "interval", interval.String())
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

	bgCtx, bgCancel := context.WithCancel(context.Background())
	defer bgCancel()
	startRecomputeScheduler(bgCtx, logger, dbc)

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
