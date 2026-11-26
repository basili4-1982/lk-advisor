package catalog_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/basili4-1982/lk-advisor/internal/catalog"
	"github.com/basili4-1982/lk-advisor/internal/store"
)

func TestReviewRegistryRelationLifecycle(t *testing.T) {
	dbc := store.OpenTestDB(t)
	ctx := context.Background()
	seedRegistry(t, dbc)
	defer cleanRegistry(t, dbc)

	id := registryID(t, dbc, "reg-a", "reg-c")
	rel, err := catalog.ReviewRegistryRelation(ctx, dbc, id, "confirm", "документация вендора", "ivanov", 1)
	if err != nil {
		t.Fatal(err)
	}
	if rel.Status != "active" || rel.Version != 2 || rel.ApprovedBy != "ivanov" {
		t.Fatalf("confirm = %+v", rel)
	}

	if _, err := catalog.ReviewRegistryRelation(ctx, dbc, id, "confirm", "", "ivanov", 1); !errors.Is(err, catalog.ErrVersionConflict) {
		t.Fatalf("stale version err = %v", err)
	}
	if _, err := catalog.ReviewRegistryRelation(ctx, dbc, id, "reject", "", "ivanov", 2); !errors.Is(err, catalog.ErrNoteRequired) {
		t.Fatalf("empty note err = %v", err)
	}
	rel, err = catalog.ReviewRegistryRelation(ctx, dbc, id, "reject", "не та задача", "ivanov", 2)
	if err != nil {
		t.Fatal(err)
	}
	if rel.Status != "rejected" || rel.RejectionReason != "не та задача" || rel.RejectedUntil == nil {
		t.Fatalf("reject = %+v", rel)
	}
}

// defer keeps the edge a candidate, records the note and still bumps the version.
func TestReviewRegistryRelationDefer(t *testing.T) {
	dbc := store.OpenTestDB(t)
	ctx := context.Background()
	seedRegistry(t, dbc)
	defer cleanRegistry(t, dbc)

	id := registryID(t, dbc, "reg-a", "reg-c")
	rel, err := catalog.ReviewRegistryRelation(ctx, dbc, id, "defer", "ждём данные по возвратам", "petrova", 1)
	if err != nil {
		t.Fatal(err)
	}
	if rel.Status != "candidate" || rel.Version != 2 || rel.ReviewNote != "ждём данные по возвратам" ||
		rel.ReviewedBy != "petrova" || rel.ApprovedBy != "" {
		t.Fatalf("defer = %+v", rel)
	}

	var events int
	if err := dbc.QueryRowContext(ctx, `
		SELECT count(*) FROM relation_events
		WHERE relation_id = $1 AND action = 'review' AND actor_kind = 'human'
		  AND actor_id = 'petrova'
		  AND payload_before IS NOT NULL AND payload_after IS NOT NULL`, id).Scan(&events); err != nil {
		t.Fatal(err)
	}
	if events != 1 {
		t.Fatalf("review events = %d, want 1", events)
	}
}

func TestBulkReviewRequiresCriterion(t *testing.T) {
	dbc := store.OpenTestDB(t)
	ctx := context.Background()
	seedRegistry(t, dbc)
	defer cleanRegistry(t, dbc)

	if _, err := catalog.BulkReviewRelations(ctx, dbc, catalog.RelationFilter{}, "confirm", "", "ivanov"); !errors.Is(err, catalog.ErrCriterionRequired) {
		t.Fatalf("bulk without criterion err = %v", err)
	}
	n, err := catalog.BulkReviewRelations(ctx, dbc,
		catalog.RelationFilter{Statuses: []string{"candidate"}, MinConfidence: 0.5},
		"confirm", "пакетное подтверждение", "ivanov")
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("bulk confirmed = %d", n)
	}

	// The bulk pass must leave the same audit trail as a single review.
	var events int
	if err := dbc.QueryRowContext(ctx, `
		SELECT count(*) FROM relation_events WHERE action = 'review'`).Scan(&events); err != nil {
		t.Fatal(err)
	}
	if events != 1 {
		t.Fatalf("bulk review events = %d, want 1", events)
	}
}

func TestSweepRelations(t *testing.T) {
	dbc := store.OpenTestDB(t)
	ctx := context.Background()
	seedRegistry(t, dbc)
	defer cleanRegistry(t, dbc)

	// reg-a -> reg-b: active, просрочено по valid_until
	dbc.ExecContext(ctx, `UPDATE relations SET valid_until=now()-interval '1 day'
		WHERE from_item_id='reg-a' AND to_item_id='reg-b'`)
	// reg-a -> reg-c: candidate, просрочено по revalidate_after
	dbc.ExecContext(ctx, `UPDATE relations SET revalidate_after=now()-interval '1 day'
		WHERE from_item_id='reg-a' AND to_item_id='reg-c'`)

	stale, expired, err := catalog.SweepRelations(ctx, dbc, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if stale != 1 || expired != 1 {
		t.Fatalf("sweep stale=%d expired=%d", stale, expired)
	}

	// The sweep leaves one aggregate audit event.
	var events int
	if err := dbc.QueryRowContext(ctx, `
		SELECT count(*) FROM relation_events WHERE action = 'sweep'`).Scan(&events); err != nil {
		t.Fatal(err)
	}
	if events != 1 {
		t.Fatalf("sweep events = %d, want 1", events)
	}
}

func TestReviewRegistryRejectRequiresNonBlankReason(t *testing.T) {
	dbc := store.OpenTestDB(t)
	ctx := context.Background()
	seedRegistry(t, dbc)
	defer cleanRegistry(t, dbc)

	id := registryID(t, dbc, "reg-a", "reg-c")
	if _, err := catalog.ReviewRegistryRelation(ctx, dbc, id, "reject", "   \n", "ivanov", 1); !errors.Is(err, catalog.ErrNoteRequired) {
		t.Fatalf("blank note err = %v, want ErrNoteRequired", err)
	}
}

func TestConfirmClearsRejectionMemory(t *testing.T) {
	dbc := store.OpenTestDB(t)
	ctx := context.Background()
	seedRegistry(t, dbc)
	defer cleanRegistry(t, dbc)

	id := registryID(t, dbc, "reg-a", "reg-c")
	if _, err := catalog.ReviewRegistryRelation(ctx, dbc, id, "reject", "не та задача", "ivanov", 1); err != nil {
		t.Fatal(err)
	}
	rel, err := catalog.ReviewRegistryRelation(ctx, dbc, id, "confirm", "передумали", "ivanov", 2)
	if err != nil {
		t.Fatal(err)
	}
	if rel.Status != "active" || rel.RejectionReason != "" || rel.RejectedUntil != nil {
		t.Fatalf("confirm after reject = %+v", rel)
	}
}

func TestBulkReviewDefaultsToCandidates(t *testing.T) {
	dbc := store.OpenTestDB(t)
	ctx := context.Background()
	seedRegistry(t, dbc)
	defer cleanRegistry(t, dbc)

	// seed has active a→b (0.9), candidate a→c (0.6), rejected c→d (0.4).
	n, err := catalog.BulkReviewRelations(ctx, dbc,
		catalog.RelationFilter{MinConfidence: 0.3}, "confirm", "пакет", "ivanov")
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("bulk confirmed = %d, want 1 (only the candidate)", n)
	}
	var rejected int
	dbc.QueryRowContext(ctx, `SELECT count(*) FROM relations
		WHERE from_item_id='reg-c' AND to_item_id='reg-d' AND status='rejected'`).Scan(&rejected)
	if rejected != 1 {
		t.Fatal("bulk resurrected a rejected edge")
	}
}

func TestSweepBumpsVersion(t *testing.T) {
	dbc := store.OpenTestDB(t)
	ctx := context.Background()
	seedRegistry(t, dbc)
	defer cleanRegistry(t, dbc)

	dbc.ExecContext(ctx, `UPDATE relations SET valid_until=now()-interval '1 day'
		WHERE from_item_id='reg-a' AND to_item_id='reg-b'`)
	before := registryVersion(t, dbc, "reg-a", "reg-b")
	if _, _, err := catalog.SweepRelations(ctx, dbc, time.Now()); err != nil {
		t.Fatal(err)
	}
	after := registryVersion(t, dbc, "reg-a", "reg-b")
	if after != before+1 {
		t.Fatalf("sweep version = %d, want %d", after, before+1)
	}
}

func TestReviewAuditPayloadsAreComparable(t *testing.T) {
	dbc := store.OpenTestDB(t)
	ctx := context.Background()
	seedRegistry(t, dbc)
	defer cleanRegistry(t, dbc)

	id := registryID(t, dbc, "reg-a", "reg-c")
	if _, err := catalog.ReviewRegistryRelation(ctx, dbc, id, "confirm", "ок", "ivanov", 1); err != nil {
		t.Fatal(err)
	}
	var before, after []byte
	if err := dbc.QueryRowContext(ctx, `SELECT payload_before, payload_after
		FROM relation_events WHERE relation_id=$1 ORDER BY id DESC LIMIT 1`, id).
		Scan(&before, &after); err != nil {
		t.Fatal(err)
	}
	var beforeMap, afterMap map[string]any
	if err := json.Unmarshal(before, &beforeMap); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(after, &afterMap); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"from_title", "to_title", "status", "version"} {
		if _, ok := beforeMap[key]; !ok {
			t.Fatalf("payload_before missing %q (shapes differ)", key)
		}
		if _, ok := afterMap[key]; !ok {
			t.Fatalf("payload_after missing %q", key)
		}
	}
}

func TestReviewRegistryNotFound(t *testing.T) {
	dbc := store.OpenTestDB(t)
	ctx := context.Background()
	seedRegistry(t, dbc)
	defer cleanRegistry(t, dbc)

	if _, err := catalog.ReviewRegistryRelation(ctx, dbc, 999999999, "confirm", "", "ivanov", 1); !errors.Is(err, catalog.ErrRelationNotFound) {
		t.Fatalf("not found err = %v", err)
	}
}

func registryVersion(t *testing.T, dbc *sql.DB, from, to string) int {
	t.Helper()
	var v int
	if err := dbc.QueryRow(`SELECT version FROM relations WHERE from_item_id=$1 AND to_item_id=$2`, from, to).Scan(&v); err != nil {
		t.Fatal(err)
	}
	return v
}
