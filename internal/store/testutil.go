package store

import (
	"context"
	"database/sql"
	"os"
	"testing"

	_ "github.com/lib/pq"
)

// TestDSN returns the test database URL.
func TestDSN() string {
	if v := os.Getenv("TEST_DATABASE_URL"); v != "" {
		return v
	}
	return "postgres://lk:lk@localhost:55432/lk?sslmode=disable"
}

// OpenTestDB opens the test database and fails the test if it is unreachable.
func OpenTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dbc, err := sql.Open("postgres", TestDSN())
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := dbc.PingContext(context.Background()); err != nil {
		t.Fatalf("ping test db: %v", err)
	}
	t.Cleanup(func() { _ = dbc.Close() })
	return dbc
}

// Truncate clears all business tables.
func Truncate(t *testing.T, dbc *sql.DB) {
	t.Helper()
	_, err := dbc.Exec(`TRUNCATE clients, products, client_environment, product_relations,
		purchases, licenses, orders, documents, events, rules, promotions,
		ground_truth, agent_sessions, profile_refresh_queue, demo_audit, agent_calls,
		recommendation_requests, recommendation_events RESTART IDENTITY CASCADE`)
	if err != nil {
		t.Fatalf("truncate: %v", err)
	}
}

// ObjectExists reports whether a table, view or materialized view exists.
func ObjectExists(t *testing.T, dbc *sql.DB, name string) bool {
	t.Helper()
	var ok bool
	if err := dbc.QueryRow(`SELECT to_regclass($1) IS NOT NULL`, name).Scan(&ok); err != nil {
		t.Fatalf("to_regclass %s: %v", name, err)
	}
	return ok
}

// Count returns the number of rows in a table.
func Count(t *testing.T, dbc *sql.DB, table string) int {
	t.Helper()
	var n int
	if err := dbc.QueryRow(`SELECT count(*) FROM ` + table).Scan(&n); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return n
}
