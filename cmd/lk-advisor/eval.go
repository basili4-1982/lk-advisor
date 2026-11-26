package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/basili4-1982/lk-advisor/internal/config"
	"github.com/basili4-1982/lk-advisor/internal/eval"
)

func runEval(cfg config.Config) error {
	dbc, err := openDB(cfg)
	if err != nil {
		return err
	}
	defer dbc.Close()

	ctx := context.Background()
	cases, err := eval.LoadGroundTruth(ctx, dbc)
	if err != nil {
		return err
	}
	report, err := eval.Run(ctx, dbc, cases, time.Now())
	if err != nil {
		return err
	}

	out, _ := json.MarshalIndent(report, "", "  ")
	fmt.Println(string(out))
	if !report.Passed {
		return fmt.Errorf("eval thresholds are not met")
	}
	return nil
}
