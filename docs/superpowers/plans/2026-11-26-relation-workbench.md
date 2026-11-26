# Relation Workbench (v5) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Превратить `lk_relation_candidates` в реестр типизированных рёбер для каталога v5 и дать мерчандайзеру рабочую таблицу с фильтрами, доказательствами, очередью ревью и массовыми решениями.

**Architecture:** Одна таблица `relations` (переименованная из `lk_relation_candidates`) хранит подтверждённые и предложенные рёбра с провенансом, версией, TTL и областью действия. Отдельный `relation_events` — append-only аудит. Go-слой в `internal/catalog/relations*.go` отвечает за выборку, ревью, генерацию и метрики; HTTP-слой — тонкий; UI — новая вкладка в существующей странице `/admin`. Синтетический каталог (`products` + `product_relations`) не меняется и продолжает работать.

**Tech Stack:** Go 1.x, `net/http`, `database/sql` + `lib/pq`, PostgreSQL, vanilla JS внутри Go-шаблона, без сборки фронтенда.

**Spec:** `reports/Дизайн системы связей товаров.md`

## Global Constraints

- Интерфейс и документация — по-русски; идентификаторы кода — по-английски.
- Сырой датасет в репозиторий не попадает. Работаем только с уже импортированными `lk_items` (817 строк, 419 `product_group`).
- Оценка качества считается на всех 817 `item_id`; демо-выдача фильтруется по `lk_items.available`.
- Не менять `product_relations`, `products`, движок рекомендаций и существующие вкладки `/admin`. Их статусы остаются `confirmed|quarantine|llm_suggested|rejected`.
- Новый реестр использует статусы `candidate|active|rejected|stale|expired|superseded`.
- Все миграции обязаны быть идемпотентными: `store.Migrate` выполняет все файлы при каждом запуске.
- SQL-сортировка и имена колонок — только из белого списка; пользовательский ввод идёт параметрами.
- Тесты запускаются на `postgres://lk:lk@localhost:55432/lk?sslmode=disable` командой `go test -p 1 ./...`.
- Каждый task заканчивается зелёным `go build ./...` и коммитом.

## Review Focus

- Переименование `lk_relation_candidates` → `relations` ломает любой забытый SQL. После Task 1 `grep -rn lk_relation_candidates --include=*.go --include=*.py` должен возвращать пусто.
- Две словарные системы статусов: синтетика говорит `confirmed/quarantine`, реестр v5 — `active/candidate`. Смешение молча скрывает строки из выдачи.
- Массовое подтверждение без критерия превращается в «одобрить всё». Оно должно требовать `min_confidence > 0` или непустой список типов.
- Отклонение без причины делает память отказов бесполезной: генератор снова предложит ту же пару.
- Неэкранированная сортировка/фильтр — путь к SQL-инъекции; всё, что попадает в текст запроса, берётся из `switch`, а не из query-параметра.

---

### Task 1: Реестр как сущность (миграция + переименование)

**Files:**
- Create: `db/migrations/023_relation_registry.sql`
- Modify: `internal/catalog/graph.go:340,414`
- Modify: `internal/catalog/dataset.go:123,169,247,294,307`
- Modify: `internal/httpapi/graph_page.go:133`
- Modify: `internal/httpapi/dataset_agent.go:162`
- Modify: `analysis/import_dataset.py:93`
- Modify: `internal/catalog/graph_test.go:101`
- Test: `internal/store/migrate_test.go`

**Interfaces:**
- Consumes: существующая таблица `lk_relation_candidates` из `013_dataset_relations.sql`.
- Produces: таблица `relations` с колонками `id, from_item_id, to_item_id, type, confidence, status, reason, evidence, source, created_at, reviewed_by, reviewed_at, review_note, provenance_kind, owner_id, version, position, quantity, metadata, audience, industry, org_type, valid_from, valid_until, revalidate_after, supersedes, rejection_reason, rejected_until, created_by, approved_by, approved_at, updated_at`; таблица `relation_events`; расширенный `relation_types`.

- [ ] **Step 1: Написать падающий тест на схему**

Добавить в `internal/store/migrate_test.go`:

```go
func TestRelationRegistrySchema(t *testing.T) {
	dbc := store.OpenTestDB(t)
	ctx := context.Background()
	if err := store.Migrate(ctx, dbc); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	for _, name := range []string{"relations", "relation_events"} {
		if !store.ObjectExists(t, dbc, name) {
			t.Errorf("missing table %s", name)
		}
	}
	// Повторный migrate не должен воскрешать старое имя через 013.
	if err := store.Migrate(ctx, dbc); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	if store.ObjectExists(t, dbc, "lk_relation_candidates") {
		t.Error("old table lk_relation_candidates was recreated")
	}
	for _, col := range []string{"provenance_kind", "version", "valid_until",
		"revalidate_after", "rejection_reason", "audience", "org_type"} {
		var ok bool
		if err := dbc.QueryRow(`SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_name='relations' AND column_name=$1)`, col).Scan(&ok); err != nil {
			t.Fatal(err)
		}
		if !ok {
			t.Errorf("missing relations.%s", col)
		}
	}
	var transitive bool
	if err := dbc.QueryRow(`SELECT is_transitive FROM relation_types WHERE code='alternative'`).Scan(&transitive); err != nil {
		t.Fatal(err)
	}
	if transitive {
		t.Error("alternative must not be transitive")
	}
}
```

- [ ] **Step 2: Убедиться, что тест падает**

Run: `go test -p 1 ./internal/store -run TestRelationRegistrySchema -v`
Expected: FAIL — `missing table relations`.

- [ ] **Step 3: Создать миграцию**

`db/migrations/023_relation_registry.sql`:

```sql
-- Relation registry v2: the edge becomes an entity with provenance, scope and lifecycle.
DO $$
BEGIN
    IF to_regclass('lk_relation_candidates') IS NOT NULL
       AND to_regclass('relations') IS NULL THEN
        ALTER TABLE lk_relation_candidates RENAME TO relations;
        -- Освобождаем имя sequence: иначе повторный запуск 013 упадёт на bigserial.
        IF to_regclass('lk_relation_candidates_id_seq') IS NOT NULL
           AND to_regclass('relations_id_seq') IS NULL THEN
            ALTER SEQUENCE lk_relation_candidates_id_seq RENAME TO relations_id_seq;
        END IF;
    END IF;
    -- 013 выполняется при каждом старте и снова создаёт пустую старую таблицу.
    IF to_regclass('lk_relation_candidates') IS NOT NULL
       AND to_regclass('relations') IS NOT NULL THEN
        DROP TABLE lk_relation_candidates;
    END IF;
END $$;

ALTER TABLE relations
    ADD COLUMN IF NOT EXISTS provenance_kind  text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS owner_id         text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS version          integer NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS position         integer NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS quantity         numeric,
    ADD COLUMN IF NOT EXISTS metadata         jsonb NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN IF NOT EXISTS audience         text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS industry         text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS org_type         text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS valid_from       timestamptz NOT NULL DEFAULT now(),
    ADD COLUMN IF NOT EXISTS valid_until      timestamptz,
    ADD COLUMN IF NOT EXISTS revalidate_after timestamptz,
    ADD COLUMN IF NOT EXISTS supersedes       bigint,
    ADD COLUMN IF NOT EXISTS rejection_reason text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS rejected_until   timestamptz,
    ADD COLUMN IF NOT EXISTS created_by       text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS approved_by      text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS approved_at      timestamptz,
    ADD COLUMN IF NOT EXISTS updated_at       timestamptz NOT NULL DEFAULT now();

ALTER TABLE relations ALTER COLUMN status SET DEFAULT 'candidate';

UPDATE relations SET provenance_kind = source WHERE provenance_kind = '';
UPDATE relations SET status = 'candidate' WHERE status = 'quarantine';
UPDATE relations SET status = 'active'    WHERE status = 'confirmed';

CREATE INDEX IF NOT EXISTS relations_status_updated_idx
    ON relations (status, updated_at DESC);
CREATE INDEX IF NOT EXISTS relations_type_idx ON relations (type);
CREATE INDEX IF NOT EXISTS relations_expiry_idx
    ON relations (valid_until) WHERE valid_until IS NOT NULL;

ALTER TABLE relation_types
    ADD COLUMN IF NOT EXISTS cardinality       text NOT NULL DEFAULT 'N:M',
    ADD COLUMN IF NOT EXISTS is_transitive     boolean NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS default_ttl_days  integer NOT NULL DEFAULT 180,
    ADD COLUMN IF NOT EXISTS revalidate_on     text[] NOT NULL DEFAULT '{}',
    ADD COLUMN IF NOT EXISTS required_evidence text[] NOT NULL DEFAULT '{}',
    ADD COLUMN IF NOT EXISTS approval_role     text NOT NULL DEFAULT 'merchandiser';

UPDATE relation_types SET is_transitive = false
    WHERE code IN ('alternative', 'compatibility', 'substitute', 'same_task');
UPDATE relation_types SET default_ttl_days = 365 WHERE code IN ('accessory', 'bundle');
UPDATE relation_types SET default_ttl_days = 90  WHERE code = 'adjacent_task';
UPDATE relation_types SET required_evidence = ARRAY['spec', 'manual']
    WHERE code IN ('compatibility', 'bundle');
UPDATE relation_types SET approval_role = 'steward'
    WHERE code IN ('upgrade', 'bundle', 'compatibility');

INSERT INTO relation_types
    (code, title, description, examples, behavior, direction,
     default_confidence, status, builtin)
VALUES
    ('variant_of', 'Вариант',
     'Тот же продукт с другим сроком, количеством мест, регионом или исполнением. Структурная связь, не смысловая.',
     'Business Standard 12 мес → Business Standard 24 мес; 5 мест → 10 мест.',
     'info', 'forward', 0.99, 'active', true)
ON CONFLICT (code) DO NOTHING;

CREATE TABLE IF NOT EXISTS relation_events (
    id             bigserial PRIMARY KEY,
    relation_id    bigint NOT NULL,
    actor_kind     text NOT NULL DEFAULT 'human',
    actor_id       text NOT NULL DEFAULT '',
    action         text NOT NULL,
    payload_before jsonb,
    payload_after  jsonb,
    rubric_version text NOT NULL DEFAULT '',
    request_id     text NOT NULL DEFAULT '',
    created_at     timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS relation_events_relation_idx
    ON relation_events (relation_id, created_at DESC);
```

- [ ] **Step 4: Обновить все ссылки на старую таблицу и статусы v5**

- `internal/catalog/graph.go`: `FROM lk_relation_candidates` → `FROM relations`; `INSERT INTO lk_relation_candidates` → `INSERT INTO relations`; `status IN ('quarantine','confirmed')` → `status IN ('candidate','active')`; в `CreateManualDatasetRelation` допустимые статусы `candidate|active`.
- `internal/catalog/dataset.go`: заменить имя таблицы во всех пяти запросах; `'quarantine'` → `'candidate'` в `InsertDatasetCandidate`.
- `internal/httpapi/graph_page.go`: `<option value="quarantine">` → `<option value="candidate">`, `<option value="confirmed">` → `<option value="active">`.
- `internal/httpapi/dataset_relation.go`: default `req.Status` → `candidate`.
- `internal/httpapi/dataset_agent.go`: при подтверждении писать `status = "active"` вместо `"confirmed"`.
- `analysis/import_dataset.py`: `TRUNCATE lk_relation_candidates, ...` → `TRUNCATE relations, relation_events, ...`.
- `internal/catalog/graph_test.go`: имя таблицы в cleanup.

- [ ] **Step 5: Прогнать тесты схемы и графа, проверить остатки**

Run: `go test -p 1 ./internal/store ./internal/catalog -run 'TestMigrate|TestRelationRegistrySchema|TestDemoGraph' -v`
Expected: PASS.

Run: `grep -rn "lk_relation_candidates" --include=*.go --include=*.py .`
Expected: пусто (кроме `docs/` и `research_notes/`, если там есть упоминания).

- [ ] **Step 6: Коммит**

```bash
git add db/migrations/023_relation_registry.sql internal/catalog/graph.go internal/catalog/dataset.go internal/httpapi/graph_page.go internal/httpapi/dataset_relation.go analysis/import_dataset.py internal/catalog/graph_test.go internal/store/migrate_test.go
git commit -m "feat: relation registry schema on v5 catalog"
```

---

### Task 2: Чтение реестра и метрики

**Files:**
- Create: `internal/catalog/relations.go`
- Test: `internal/catalog/relations_test.go`

**Interfaces:**
- Consumes: `relations`, `relation_types`, `lk_items` (колонки `item_id, title, category, product_group, available`).
- Produces:
  - `type RelationFilter struct { Query string; Types, Statuses, Provenance []string; Family, Audience, OrgType, OwnerID string; MinConfidence, MaxConfidence float64; ExpiringDays, Limit, Offset int; Sort string }`
  - `type RegistryRelation struct { ID int64; FromID, ToID, Type, Status, ProvenanceKind, Reason string; Confidence float64; Evidence []RelationEvidence; Audience, Industry, OrgType, OwnerID string; Version, Position int; ValidUntil, RevalidateAfter, RejectedUntil *time.Time; RejectionReason, ReviewNote, ReviewedBy, ApprovedBy string; ApprovedAt *time.Time; FromTitle, ToTitle, FromFamily, ToFamily, FromCategory, ToCategory string; UpdatedAt time.Time }`
  - `type RelationEvidence struct { Kind, Summary string; Details map[string]any }`
  - `type RegistryPage struct { Items []RegistryRelation; Total, Limit, Offset int }`
  - `type RegistryMetrics struct { Total, Active, Candidates, Rejected, Stale, Expiring30, ItemsWithActive, ItemsTotal int; CoveragePct, HubTop1Pct, StaleShare float64; ByType, ByProvenance map[string]int; Families []string }`
  - `func ListRegistryRelations(ctx context.Context, dbc *sql.DB, f RelationFilter) (RegistryPage, error)`
  - `func RegistryMetrics(ctx context.Context, dbc *sql.DB) (RegistryMetrics, error)`

- [ ] **Step 1: Написать падающий тест выборки**

`internal/catalog/relations_test.go` (пакет `catalog_test`). Хелперы:

- `seedRegistry(t, dbc)` очищает `relation_events` и `relations` (тестовая БД изолирована; другие тесты создают свои рёбра сами), затем вставляет `lk_items` `reg-a`/`reg-b` с `product_group='REG-G1'`, `reg-c`/`reg-d` с `product_group='REG-G2'` и три ребра: `active accessory a→b manual 0.90`, `candidate adjacent_task a→c co_purchase 0.60`, `rejected upgrade c→d llm 0.40` с `rejection_reason='не та задача'` и `rejected_until=now()+90 days`. `lk_item_privacy` — это view, вставлять в неё нельзя; реестр v5 работает без неё.
- `registryID(t, dbc, from, to)` возвращает `id` ребра.
- `cleanRegistry(t, dbc)` удаляет `relations` и `lk_items` с префиксом `reg-`.

```go
func TestListRegistryRelationsFilters(t *testing.T) {
	dbc := store.OpenTestDB(t)
	ctx := context.Background()
	seedRegistry(t, dbc)
	defer cleanRegistry(t, dbc)

	page, err := catalog.ListRegistryRelations(ctx, dbc, catalog.RelationFilter{
		Statuses: []string{"candidate"}, Limit: 50,
	})
	if err != nil { t.Fatal(err) }
	if page.Total != 1 || page.Items[0].FromID != "reg-a" {
		t.Fatalf("candidate filter = %+v", page.Items)
	}

	page, err = catalog.ListRegistryRelations(ctx, dbc, catalog.RelationFilter{
		Query: "reg-b", Limit: 50,
	})
	if err != nil { t.Fatal(err) }
	if page.Total != 1 { t.Fatalf("q=reg-b total = %d", page.Total) }

	page, err = catalog.ListRegistryRelations(ctx, dbc, catalog.RelationFilter{
		Family: "REG-G1", Limit: 50,
	})
	if err != nil { t.Fatal(err) }
	if page.Total != 2 { t.Fatalf("family total = %d", page.Total) }

	page, err = catalog.ListRegistryRelations(ctx, dbc, catalog.RelationFilter{
		MinConfidence: 0.7, Limit: 50, Sort: "confidence_desc",
	})
	if err != nil { t.Fatal(err) }
	if page.Total != 1 || page.Items[0].ToID != "reg-b" {
		t.Fatalf("confidence filter = %+v", page.Items)
	}

	// Неизвестная сортировка не должна попадать в SQL и ломать запрос.
	page, err = catalog.ListRegistryRelations(ctx, dbc, catalog.RelationFilter{
		Sort: "confidence_desc; DROP TABLE relations", Limit: 50,
	})
	if err != nil { t.Fatal(err) }
	if page.Total != 3 { t.Fatalf("injected sort total = %d", page.Total) }
}
```

- [ ] **Step 2: Убедиться, что тест падает**

Run: `go test -p 1 ./internal/catalog -run TestListRegistryRelationsFilters -v`
Expected: FAIL — `undefined: catalog.ListRegistryRelations`.

- [ ] **Step 3: Реализовать выборку**

В `relations.go`:

- `scanRegistryRelation(rows)` читает все колонки из `Interfaces`, `evidence` разбирает через `decodeEvidence([]byte) ([]RelationEvidence, error)`; legacy-формат `{"source","value"}` маппится в `Kind=source, Summary=value`.
- `buildRelationWhere(f RelationFilter) (string, []any)` собирает `WHERE` только из параметров; `q` → `(f.title ILIKE $n OR t.title ILIKE $n OR r.from_item_id ILIKE $n OR r.to_item_id ILIKE $n)`; `types/statuses/provenance` → `= ANY($n)`; `family` → `f.product_group=$n` (семейство источника); `audience/org_type` → `(r.audience='' OR r.audience=$n)`; `expiring` → `r.valid_until IS NOT NULL AND r.valid_until <= now() + ($n || ' days')::interval`.
- `orderBy` — `switch` по четырём значениям: `confidence_desc` (default), `updated_desc`, `expiry_asc`, `confidence_asc`. Любое другое значение → default.
- Запрос: `LEFT JOIN lk_items f ON f.item_id=r.from_item_id`, `LEFT JOIN lk_items t ON t.item_id=r.to_item_id`, `count(*) OVER() AS total`, `LIMIT $n OFFSET $n`.

- [ ] **Step 4: Написать падающий тест метрик**

```go
func TestRegistryMetrics(t *testing.T) {
	dbc := store.OpenTestDB(t)
	ctx := context.Background()
	seedRegistry(t, dbc)
	defer cleanRegistry(t, dbc)

	m, err := catalog.RegistryMetrics(ctx, dbc)
	if err != nil { t.Fatal(err) }
	if m.Active != 1 || m.Candidates != 1 || m.Rejected != 1 {
		t.Fatalf("status metrics = %+v", m)
	}
	if m.ItemsWithActive < 2 || m.ItemsTotal < 3 {
		t.Fatalf("coverage metrics = %+v", m)
	}
	if m.ByType["accessory"] != 1 || m.ByProvenance["co_purchase"] != 1 {
		t.Fatalf("group metrics = %+v", m)
	}
}
```

- [ ] **Step 5: Убедиться, что тест падает и реализовать метрики**

Run: `go test -p 1 ./internal/catalog -run TestRegistryMetrics -v`
Expected: FAIL — `undefined: catalog.RegistryMetrics`.

Реализация: один запрос с `count(*) FILTER (WHERE status='active')` и т.д.; `ItemsTotal` — `SELECT count(*) FROM lk_items`; `ItemsWithActive` — `count(DISTINCT from_item_id) UNION to_item_id` по active; `CoveragePct = ItemsWithActive/ItemsTotal*100`; `StaleShare = stale/(active+stale)*100`; `Expiring30` — active с `valid_until <= now()+interval '30 days'`. `HubTop1Pct` — CTE по степеням целей active-рёбер, доля рёбер, идущих в цели с рангом ≤ 1% (минимум одна). `Families` — `SELECT DISTINCT product_group FROM lk_items WHERE product_group<>'' ORDER BY 1 LIMIT 100`.

- [ ] **Step 6: Прогнать и закоммитить**

Run: `go test -p 1 ./internal/catalog -run 'TestListRegistry|TestRegistryMetrics' -v`
Expected: PASS.

```bash
git add internal/catalog/relations.go internal/catalog/relations_test.go
git commit -m "feat: registry relations query and metrics"
```

---

### Task 3: Ревью, память отказов и TTL

**Files:**
- Create: `internal/catalog/relations_review.go`
- Test: `internal/catalog/relations_review_test.go`

**Interfaces:**
- Consumes: `relations`, `relation_events`; `ListRegistryRelations`, `RelationFilter`.
- Produces:
  - `var ErrRelationNotFound = errors.New("relation not found")`
  - `var ErrVersionConflict = errors.New("relation version conflict")`
  - `var ErrNoteRequired = errors.New("review note required")`
  - `var ErrCriterionRequired = errors.New("bulk review requires a criterion")`
  - `func ReviewRegistryRelation(ctx context.Context, dbc *sql.DB, id int64, decision, note, actor string, expectedVersion int) (RegistryRelation, error)`
  - `func BulkReviewRelations(ctx context.Context, dbc *sql.DB, f RelationFilter, decision, note, actor string) (int, error)`
  - `func SweepRelations(ctx context.Context, dbc *sql.DB, now time.Time) (stale, expired int, err error)`

- [ ] **Step 1: Написать падающие тесты ревью**

```go
func TestReviewRegistryRelationLifecycle(t *testing.T) {
	dbc := store.OpenTestDB(t)
	ctx := context.Background()
	seedRegistry(t, dbc)
	defer cleanRegistry(t, dbc)

	id := registryID(t, dbc, "reg-a", "reg-c")
	rel, err := catalog.ReviewRegistryRelation(ctx, dbc, id, "confirm", "документация вендора", "ivanov", 1)
	if err != nil { t.Fatal(err) }
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
	if err != nil { t.Fatal(err) }
	if rel.Status != "rejected" || rel.RejectionReason != "не та задача" || rel.RejectedUntil == nil {
		t.Fatalf("reject = %+v", rel)
	}
}
```

- [ ] **Step 2: Убедиться, что тест падает**

Run: `go test -p 1 ./internal/catalog -run TestReviewRegistryRelationLifecycle -v`
Expected: FAIL — `undefined: catalog.ReviewRegistryRelation`.

- [ ] **Step 3: Реализовать ревью**

`ReviewRegistryRelation` в транзакции: `SELECT ... FOR UPDATE`; проверить `version == expectedVersion` (иначе `ErrVersionConflict`); `confirm` → `status='active', approved_by=$actor, approved_at=now(), rejection_reason='', rejected_until=NULL`; `reject` → требовать `note != ""` (`ErrNoteRequired`), `status='rejected', rejection_reason=note, rejected_until=now()+interval '90 days'`; `defer` → `status='candidate'`; во всех случаях `version=version+1, updated_at=now(), reviewed_by=actor, reviewed_at=now(), review_note=note`. Записать `relation_events` с `action='review'`, `payload_before`/`payload_after` и `actor_kind='human'`.

- [ ] **Step 4: Написать падающие тесты bulk и sweep**

```go
func TestBulkReviewRequiresCriterion(t *testing.T) {
	dbc := store.OpenTestDB(t)
	ctx := context.Background()
	seedRegistry(t, dbc)
	defer cleanRegistry(t, dbc)

	if _, err := catalog.BulkReviewRelations(ctx, dbc, catalog.RelationFilter{}, "confirm", "", "ivanov"); err == nil {
		t.Fatal("bulk without criterion must fail")
	}
	n, err := catalog.BulkReviewRelations(ctx, dbc,
		catalog.RelationFilter{Statuses: []string{"candidate"}, MinConfidence: 0.5},
		"confirm", "пакетное подтверждение", "ivanov")
	if err != nil { t.Fatal(err) }
	if n != 1 { t.Fatalf("bulk confirmed = %d", n) }
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
	if err != nil { t.Fatal(err) }
	if stale != 1 || expired != 1 {
		t.Fatalf("sweep stale=%d expired=%d", stale, expired)
	}
}
```

- [ ] **Step 5: Реализовать bulk и sweep, прогнать тесты**

`BulkReviewRelations`: отклонять, если `len(f.Types)==0 && f.MinConfidence<=0` (ошибка `ErrCriterionRequired`); для reject требовать note; выбирать `id` с `LIMIT 500`, вызывать `ReviewRegistryRelation` по одному и считать успешные.

`SweepRelations`: `UPDATE relations SET status='stale' WHERE status IN ('candidate','active') AND revalidate_after IS NOT NULL AND revalidate_after < $1`; затем `UPDATE relations SET status='expired' WHERE status='active' AND valid_until IS NOT NULL AND valid_until < $1`; вернуть `RowsAffected`; записать одно агрегатное событие `action='sweep'`.

Run: `go test -p 1 ./internal/catalog -run 'TestReviewRegistry|TestBulkReview|TestSweepRelations' -v`
Expected: PASS.

- [ ] **Step 6: Коммит**

```bash
git add internal/catalog/relations_review.go internal/catalog/relations_review_test.go
git commit -m "feat: relation review lifecycle, reject memory and TTL sweep"
```

---

### Task 4: Генерация кандидатов

**Files:**
- Create: `internal/catalog/relations_generate.go`
- Test: `internal/catalog/relations_generate_test.go`

**Interfaces:**
- Consumes: `co_purchase_pairs`, `lk_items`, `relations`.
- Produces:
  - `type GenerateOptions struct { MinCoPurchase, Limit int; IncludeVariants bool; OnlyItems []string }`
  - `type GenerateResult struct { CoPurchase, Variant, SkippedRejected, SkippedHub int }`
  - `func GenerateRelations(ctx context.Context, dbc *sql.DB, opts GenerateOptions) (GenerateResult, error)`

- [ ] **Step 1: Написать падающий тест вариантов**

```go
func TestGenerateVariantRelations(t *testing.T) {
	dbc := store.OpenTestDB(t)
	ctx := context.Background()
	seedRegistry(t, dbc)
	defer cleanRegistry(t, dbc)

	res, err := catalog.GenerateRelations(ctx, dbc, catalog.GenerateOptions{
		IncludeVariants: true, OnlyItems: []string{"reg-a", "reg-b", "reg-c", "reg-d"},
	})
	if err != nil { t.Fatal(err) }
	if res.Variant != 2 {
		t.Fatalf("variants = %d, want 2", res.Variant)
	}
	var active int
	dbc.QueryRowContext(ctx, `SELECT count(*) FROM relations
		WHERE type='variant_of' AND status='active' AND provenance_kind='attribute'`).Scan(&active)
	if active != 2 { t.Fatalf("active variant_of = %d", active) }
}
```

- [ ] **Step 2: Убедиться, что тест падает**

Run: `go test -p 1 ./internal/catalog -run TestGenerateVariantRelations -v`
Expected: FAIL — `undefined: catalog.GenerateRelations`.

- [ ] **Step 3: Реализовать генерацию вариантов**

Для каждого `product_group`, где больше одного элемента из `OnlyItems` (или все, если пусто): выбрать представителя `min(item_id)` и вставить `variant_of` от остальных к представителю. `status='active'`, `provenance_kind='attribute'`, `source='attribute'`, `confidence=0.99`, `reason='общий product_group'`, `evidence=[{"kind":"attribute","summary":"product_group=G..."}]`, `valid_from=now()`. `ON CONFLICT DO NOTHING`.

- [ ] **Step 4: Написать падающий тест co-purchase и памяти отказов**

```go
func TestGenerateCoPurchaseSkipsRejected(t *testing.T) {
	dbc := store.OpenTestDB(t)
	ctx := context.Background()
	seedRegistry(t, dbc)
	defer cleanRegistry(t, dbc)

	_, err := dbc.ExecContext(ctx, `INSERT INTO co_purchase_pairs (item_a,item_b,count,b2b_count,b2c_count)
		VALUES ('reg-a','reg-b',5,5,0) ON CONFLICT DO NOTHING`)
	if err != nil { t.Fatal(err) }

	res, err := catalog.GenerateRelations(ctx, dbc, catalog.GenerateOptions{
		MinCoPurchase: 3, Limit: 50, OnlyItems: []string{"reg-a", "reg-b", "reg-c"},
	})
	if err != nil { t.Fatal(err) }
	if res.CoPurchase != 1 { t.Fatalf("co-purchase created = %d", res.CoPurchase) }

	// Отклоняем и проверяем, что повторный прогон не воскрешает пару.
	var id int64
	dbc.QueryRowContext(ctx, `SELECT id FROM relations
		WHERE from_item_id='reg-a' AND to_item_id='reg-b' AND type='adjacent_task'`).Scan(&id)
	if _, err := catalog.ReviewRegistryRelation(ctx, dbc, id, "reject", "не связаны", "ivanov", 1); err != nil {
		t.Fatal(err)
	}
	res, err = catalog.GenerateRelations(ctx, dbc, catalog.GenerateOptions{
		MinCoPurchase: 3, Limit: 50, OnlyItems: []string{"reg-a", "reg-b", "reg-c"},
	})
	if err != nil { t.Fatal(err) }
	if res.CoPurchase != 0 || res.SkippedRejected != 1 {
		t.Fatalf("second run = %+v", res)
	}
}
```

- [ ] **Step 5: Реализовать co-purchase генерацию**

- `OnlyItems` → `AND p.item_a = ANY($n) AND p.item_b = ANY($n)`.
- Порог: `p.b2b_count >= 3`. Приватность здесь не фильтруется: реестр внутренний, а публичная витрина по-прежнему отсекает скрытые товары.
- Выбрать пары одним запросом: `p.item_a, p.item_b, p.count, p.b2b_count` + `target_degree` (степень `item_b` по всем `co_purchase_pairs`) + `hub_threshold` (`percentile_cont(0.99)` по степеням). Пройти циклом в Go: пары с `target_degree > hub_threshold` пропускать как `SkippedHub`; пары с активным отказом — как `SkippedRejected`; остальные вставлять по одной.
- Память отказов: `NOT EXISTS (SELECT 1 FROM relations r WHERE r.from_item_id=p.item_a AND r.to_item_id=p.item_b AND r.type='adjacent_task' AND r.status='rejected' AND (r.rejected_until IS NULL OR r.rejected_until > now()))`; иначе `SkippedRejected`.
- Вставка: `type='adjacent_task'` (предварительный тип, `metadata='{"typing":"pending"}'`; LLM-типизация — отдельный план), `status='candidate'`, `provenance_kind='co_purchase'`, `source='co_purchase'`, `confidence=LEAST(0.90, 0.55 + count*0.03)`, `reason='куплено вместе N раз'`, evidence `[{"kind":"co_purchase","summary":"N заказов, B2B M","details":{"orders":N,"b2b":M}}]`.

- [ ] **Step 6: Прогнать и закоммитить**

Run: `go test -p 1 ./internal/catalog -run 'TestGenerate' -v`
Expected: PASS.

```bash
git add internal/catalog/relations_generate.go internal/catalog/relations_generate_test.go
git commit -m "feat: co-purchase and variant candidate generation"
```

---

### Task 5: HTTP API реестра

**Files:**
- Create: `internal/httpapi/registry_api.go`
- Modify: `internal/httpapi/router.go:64-77`
- Test: `internal/httpapi/registry_test.go`

**Interfaces:**
- Consumes: `catalog.ListRegistryRelations`, `catalog.RegistryMetrics`, `catalog.ReviewRegistryRelation`, `catalog.BulkReviewRelations`, `catalog.GenerateRelations`, `catalog.Err*`.
- Produces: маршруты и JSON:
  - `GET /v1/admin/registry/relations`
  - `POST /v1/admin/registry/relations/{id}/review` → `{"decision":"confirm|reject|defer","note":"...","expected_version":N}`
  - `POST /v1/admin/registry/relations/bulk-review` → `{"filter":{...},"decision":"confirm|reject","note":"..."}`
  - `POST /v1/admin/registry/generate` → `{"min_co_purchase":3,"limit":500,"include_variants":true}`
  - `GET /v1/admin/registry/metrics`

- [ ] **Step 1: Написать падающий HTTP-тест**

Файл `internal/httpapi/registry_test.go` (импорты `encoding/json`, `fmt`, `net/http`, `testing`).

```go
func TestRegistryAPIFlow(t *testing.T) {
	h, dbc := setupAPI(t)
	ctx := context.Background()
	clean := func() {
		dbc.ExecContext(ctx, `DELETE FROM relations WHERE from_item_id LIKE 'reg-%'`)
		dbc.ExecContext(ctx, `DELETE FROM lk_items WHERE item_id LIKE 'reg-%'`)
	}
	clean(); defer clean()
	for _, id := range []string{"reg-a", "reg-b"} {
		dbc.ExecContext(ctx, `INSERT INTO lk_items (item_id,title,category,product_group)
			VALUES ($1,$1,'security','REG-G1')`, id)
	}
	dbc.ExecContext(ctx, `INSERT INTO relations
		(from_item_id,to_item_id,type,confidence,status,provenance_kind,source)
		VALUES ('reg-a','reg-b','accessory',0.8,'candidate','manual','manual')`)

	rec := doJSON(t, h, http.MethodGet, "/v1/admin/registry/relations?status=candidate&q=reg-a", "")
	if rec.Code != http.StatusOK { t.Fatalf("list: %d %s", rec.Code, rec.Body.String()) }

	var page struct{ Total int `json:"total"` }
	json.Unmarshal(rec.Body.Bytes(), &page)
	if page.Total != 1 { t.Fatalf("total = %d", page.Total) }

	var id int64
	dbc.QueryRow(`SELECT id FROM relations WHERE from_item_id='reg-a'`).Scan(&id)
	rec = doJSON(t, h, http.MethodPost,
		fmt.Sprintf("/v1/admin/registry/relations/%d/review", id),
		`{"decision":"reject","note":"","expected_version":1}`)
	if rec.Code != http.StatusBadRequest { t.Fatalf("empty note = %d", rec.Code) }

	rec = doJSON(t, h, http.MethodPost,
		fmt.Sprintf("/v1/admin/registry/relations/%d/review", id),
		`{"decision":"confirm","note":"ok","expected_version":1}`)
	if rec.Code != http.StatusOK { t.Fatalf("confirm = %d %s", rec.Code, rec.Body.String()) }

	rec = doJSON(t, h, http.MethodPost,
		fmt.Sprintf("/v1/admin/registry/relations/%d/review", id),
		`{"decision":"confirm","note":"ok","expected_version":1}`)
	if rec.Code != http.StatusConflict { t.Fatalf("stale = %d", rec.Code) }
}
```

- [ ] **Step 2: Убедиться, что тест падает**

Run: `go test -p 1 ./internal/httpapi -run TestRegistryAPIFlow -v`
Expected: FAIL — 404 на `/v1/admin/registry/relations`.

- [ ] **Step 3: Реализовать обработчики**

`registry_api.go`: разбор query-параметров в `catalog.RelationFilter` (`q,type,status,source,family,audience,org_type,min_conf,max_conf,expiring,owner,sort,limit,offset`; повторяющиеся `type` и `status` читать через `r.URL.Query()["type"]`). Ошибки: `ErrNoteRequired` → 400, `ErrVersionConflict` → 409, `ErrRelationNotFound` → 404, `ErrCriterionRequired` → 400. Все изменения логировать через `catalog.LogAdminChange`. Зарегистрировать маршруты в `router.go` рядом с dataset-маршрутами:

```go
mux.HandleFunc("GET /v1/admin/registry/relations", h.adminRegistryList)
mux.HandleFunc("POST /v1/admin/registry/relations/bulk-review", h.adminRegistryBulkReview)
mux.HandleFunc("POST /v1/admin/registry/relations/{id}/review", h.adminRegistryReview)
mux.HandleFunc("POST /v1/admin/registry/generate", h.adminRegistryGenerate)
mux.HandleFunc("GET /v1/admin/registry/metrics", h.adminRegistryMetrics)
```

- [ ] **Step 4: Прогнать тесты**

Run: `go test -p 1 ./internal/httpapi -run TestRegistryAPIFlow -v`
Expected: PASS.

- [ ] **Step 5: Коммит**

```bash
git add internal/httpapi/registry_api.go internal/httpapi/router.go internal/httpapi/registry_test.go
git commit -m "feat: registry HTTP API"
```

---

### Task 6: Workbench UI

**Files:**
- Modify: `internal/httpapi/admin.go` (вкладка, разметка, JS)
- Test: `internal/httpapi/admin_test.go`

**Interfaces:**
- Consumes: API из Task 5; существующие CSS-классы `.toolbar`, `.tablewrap`, `.pill`, `.hint`, `.stat`.
- Produces: вкладка `data-tab="registry"`, контейнеры `#tab-registry`, `#registry-metrics`, `#registry-body`, `#registry-detail`, `#registry-note`, `#registry-generate`; JS `loadRegistry()`, `renderRegistry()`, `registryReview(id, decision)`, `registryBulk(decision)`, `registryGenerate()`.

- [ ] **Step 1: Написать падающий тест страницы**

В `internal/httpapi/admin_test.go` (добавить импорт `strings`):

```go
func TestAdminPageHasRegistryWorkbench(t *testing.T) {
	h, _ := setupAPI(t)
	rec := doJSON(t, h, http.MethodGet, "/admin", "")
	body := rec.Body.String()
	for _, needle := range []string{
		`data-tab="registry"`, `id="tab-registry"`, `id="registry-metrics"`,
		`id="registry-detail"`, `id="registry-generate"`, `function loadRegistry`,
	} {
		if !strings.Contains(body, needle) {
			t.Errorf("admin page missing %s", needle)
		}
	}
}
```

- [ ] **Step 2: Убедиться, что тест падает**

Run: `go test -p 1 ./internal/httpapi -run TestAdminPageHasRegistryWorkbench -v`
Expected: FAIL — `missing data-tab="registry"`.

- [ ] **Step 3: Добавить вкладку и разметку**

- Кнопка после `data-tab="relations"`: `<button class="tab" data-tab="registry">Каталог v5</button>`.
- Блок `#tab-registry` вставить перед `<div id="tab-validation"`:

```html
<div id="tab-registry" class="hidden">
  <div class="stats" id="registry-metrics" style="margin-bottom:12px"></div>
  <div class="toolbar">
    <input id="registry-search" placeholder="Поиск по товару или id" style="max-width:220px">
    <select id="registry-type"><option value="">Все типы</option></select>
    <select id="registry-status">
      <option value="">Все статусы</option>
      <option value="candidate">Кандидаты</option>
      <option value="active">Активные</option>
      <option value="rejected">Отклонённые</option>
      <option value="stale">Устаревшие</option>
    </select>
    <select id="registry-source">
      <option value="">Все источники</option>
      <option value="manual">Ручные</option>
      <option value="co_purchase">Co-purchase</option>
      <option value="llm">LLM</option>
      <option value="attribute">Атрибуты</option>
    </select>
    <select id="registry-family"><option value="">Все семейства</option></select>
    <button class="secondary small" data-view="all">Все</button>
    <button class="secondary small" data-view="disputed">Спорные</button>
    <button class="secondary small" data-view="expiring">Истекают</button>
    <button class="secondary small" data-view="structural">Структурные</button>
    <button id="registry-generate" class="small">Сгенерировать</button>
    <span class="hint" id="registry-count"></span>
  </div>
  <div class="tablewrap"><table>
    <thead><tr>
      <th><input type="checkbox" id="registry-all" style="width:auto"></th>
      <th>Источник</th><th>Цель</th><th>Тип</th><th>Статус</th>
      <th>Провенанс</th><th>Уверенность</th><th>Доказательство</th>
      <th>Семейство</th><th>Действует до</th><th>Версия</th>
    </tr></thead>
    <tbody id="registry-body"></tbody>
  </table></div>
  <div class="toolbar" style="margin-top:10px">
    <input id="registry-note" placeholder="Причина решения (обязательна для отказа)" style="max-width:360px">
    <button id="registry-confirm" class="small">Подтвердить выбранные</button>
    <button id="registry-reject" class="secondary small">Отклонить</button>
    <button id="registry-defer" class="secondary small">Отложить</button>
  </div>
  <div id="registry-detail" class="formbox" style="margin-top:12px"></div>
</div>
```

- [ ] **Step 4: Реализовать JS**

`loadRegistry()` собирает query из полей, вызывает `GET /v1/admin/registry/relations` и `GET /v1/admin/registry/metrics`, заполняет `#registry-metrics` (всего, активных, кандидатов, покрытие, stale, хабы), `#registry-type` (из `state.relation_types`, уже загруженного в `load()`), `#registry-family` (из `metrics.families`) и `#registry-body`. Строка: чекбокс `data-id`, `from_title → to_title`, тип, статус как `.pill`, провенанс, `Math.round(confidence*100)+'%'`, evidence chips (`kind: summary`), семейство, `valid_until`, `v{version}`. Клик по строке (кроме чекбокса) открывает `#registry-detail`: карточки источника и цели, чипы доказательств, заметка, кнопки `Подтвердить/Отклонить/Отложить` с текущей версией.

`registryReview(id, decision)` и `registryBulk(decision)` вызывают API; для `reject` требуют непустой `#registry-note` и показывают `toast('Нужна причина отказа', true)`; успех → тост и `loadRegistry()`. `registryGenerate()` вызывает `POST /v1/admin/registry/generate` с `{"min_co_purchase":3,"limit":500,"include_variants":true}` и показывает `co_purchase/variant/skipped_rejected`.

Пресеты `data-view`: `all` — очистить фильтры; `disputed` — `status=candidate&min_conf=0.5&max_conf=0.75`; `expiring` — `expiring=30`; `structural` — `type=variant_of`.

В обработчик переключения вкладок (рядом с `if (b.dataset.tab === 'dataset') loadDataset();`) добавить `if (b.dataset.tab === 'registry') loadRegistry();`.

- [ ] **Step 5: Проверить синтаксис JS и тест**

```bash
python3 - <<'PY'
s=open('internal/httpapi/admin.go',encoding='utf-8').read()
open('/tmp/admin_workbench.js','w',encoding='utf-8').write(s[s.rindex('<script>')+8:s.rindex('</script>')])
PY
node --check /tmp/admin_workbench.js
go test -p 1 ./internal/httpapi -run 'TestAdminPage' -v
```

Expected: JS без ошибок; тест PASS.

- [ ] **Step 6: Коммит**

```bash
git add internal/httpapi/admin.go internal/httpapi/admin_test.go
git commit -m "feat: relation workbench tab for v5 catalog"
```

---

### Task 7: Документация, интеграция и смоук

**Files:**
- Create: `docs/relation-workbench.md`
- Modify: `README.md` (строка про новый документ)
- Test: ручной смоук + полный прогон

**Interfaces:**
- Consumes: всё предыдущее.
- Produces: описание реестра, API и статусов; зелёный полный прогон.

- [ ] **Step 1: Написать `docs/relation-workbench.md`**

Разделы: зачем реестр; схема `relations`; таблица статусов `candidate|active|rejected|stale|expired|superseded`; провенанс; TTL и `revalidate_after`; API; четыре пресета Workbench; память отказов; что осталось за рамками (2D-граф, LLM-типизация, релизы-срезы).

- [ ] **Step 2: Полный прогон**

```bash
DATABASE_URL='postgres://lk:lk@localhost:55432/lk?sslmode=disable' go run ./cmd/lk-advisor migrate
go build ./...
go vet ./...
go test -count=1 -p 1 ./...
```

Expected: сборка и все тесты PASS.

- [ ] **Step 3: Смоук на живом сервере**

```bash
fuser -k 8080/tcp 2>/dev/null || true
DEMO_ALLOW_REAL_DATA=1 ADDR=:8080 DATABASE_URL='postgres://lk:lk@localhost:55432/lk?sslmode=disable' go run ./cmd/lk-advisor serve
```

В другом терминале:

```bash
curl -s 'localhost:8080/v1/admin/registry/metrics'
curl -s 'localhost:8080/v1/admin/registry/relations?limit=3'
curl -s -X POST localhost:8080/v1/admin/registry/generate -d '{"min_co_purchase":3,"limit":500,"include_variants":true}'
curl -s 'localhost:8080/v1/admin/registry/metrics'
```

Expected: метрики возвращаются; после генерации `active` вырос на число `variant_of` (ожидаемо около 398), кандидатов — на число пар co-purchase с `b2b_count>=3`.

- [ ] **Step 4: Коммит**

```bash
git add docs/relation-workbench.md README.md
git commit -m "docs: relation workbench and registry API"
```

---

## Вне рамок этого плана

- Перевод демо-графа с 3D на 2D и пять сценариев-кадров — отдельный план (`docs/superpowers/plans/*-graph-demo.md`).
- LLM-типизация новых пар и оценка precision по типам — отдельный план.
- Полноценный split view с групповыми кандидатами и preview выдачи: в этом плане только таблица + карточка выбранного ребра.
- Rule engine для `compatibility` и автопубликация по атрибутам, кроме структурного `variant_of`.
- Релизы-срезы (draft→release), impact analysis и права по категориям — production-этап.
- Пересчёт variant/new-product share остаётся в `analysis/baseline_official.py` и `analysis/evaluate.py`; Workbench показывает покрытие и состав рёбер, а не подменяет офлайн-метрику.
