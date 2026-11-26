package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/basili4-1982/lk-advisor/internal/config"
	"github.com/basili4-1982/lk-advisor/internal/profile"
	"github.com/basili4-1982/lk-advisor/internal/store"
)

func runSeed(cfg config.Config) error {
	fs := flag.NewFlagSet("seed", flag.ExitOnError)
	seed := fs.Int64("seed", 42, "deterministic seed")
	_ = fs.Parse(os.Args[2:])

	dbc, err := openDB(cfg)
	if err != nil {
		return err
	}
	defer dbc.Close()

	ctx := context.Background()
	if err := store.Migrate(ctx, dbc); err != nil {
		return err
	}
	if err := store.Clear(ctx, dbc); err != nil {
		return err
	}
	ds, err := store.Generate(*seed)
	if err != nil {
		return err
	}
	if err := store.Load(ctx, dbc, ds); err != nil {
		return err
	}
	if err := profile.RefreshAll(ctx, dbc); err != nil {
		return err
	}

	fmt.Printf("seeded: clients=%d products=%d promotions=%d cases=%d\n",
		len(ds.Clients), len(ds.Products), len(ds.Promotions), len(ds.GroundTruth))
	return nil
}
