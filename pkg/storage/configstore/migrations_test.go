package configstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/samber/do/v2"
	_ "modernc.org/sqlite"

	appconfig "agent-compose/pkg/config"
)

func TestNewConfigStoreHonorsCanceledStartupContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	root := t.TempDir()
	di := do.New()
	do.ProvideValue(di, ctx)
	do.ProvideValue(di, &appconfig.Config{
		DataRoot: root,
		DbAddr:   filepath.Join(root, "data.db"),
	})

	store, err := NewConfigStore(di)
	if store != nil {
		t.Fatal("NewConfigStore returned a store for canceled startup context")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("NewConfigStore error = %v, want context canceled", err)
	}
}

func TestConfigStoreMigrationBaseline(t *testing.T) {
	ctx := context.Background()
	db := newMemoryDB(t)
	store := FromDB(db)
	if err := store.InitSchema(ctx); err != nil {
		t.Fatalf("InitSchema: %v", err)
	}

	tables := []string{
		"schema_migrations", "global_env", "workspace_config", "agent_definition",
		"llm_provider", "llm_model", "llm_provider_model", "llm_facade_token",
		"capability_gateway", "volumes", "project_volumes", "loader", "loader_trigger",
		"loader_run", "loader_event", "loader_state", "loader_binding", "project",
		"project_revision", "project_agent", "project_scheduler", "project_run",
		"project_run_event", "event", "webhook_source", "event_delivery", "event_sandbox_link",
	}
	for _, table := range tables {
		var count int
		if err := db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&count); err != nil {
			t.Fatalf("query table %s: %v", table, err)
		}
		if count != 1 {
			t.Fatalf("table %s count = %d, want 1", table, count)
		}
	}
	var tableCount int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%'`).Scan(&tableCount); err != nil {
		t.Fatalf("count application tables: %v", err)
	}
	if tableCount != len(tables) {
		t.Fatalf("application table count = %d, want %d", tableCount, len(tables))
	}
	for _, index := range []string{
		"idx_agent_definition_deleted_enabled", "idx_agent_definition_managed_project", "idx_agent_definition_workspace",
		"idx_event_correlation", "idx_event_delivery_run", "idx_event_delivery_status", "idx_event_dispatch",
		"idx_event_dispatch_attempt", "idx_event_idempotency", "idx_event_parent", "idx_event_sandbox_link_run",
		"idx_event_sandbox_link_sandbox", "idx_event_topic_sequence", "idx_llm_facade_token_sandbox",
		"idx_loader_event_created", "idx_loader_managed_project", "idx_loader_run_started", "idx_loader_trigger_schedule",
		"idx_project_agent_id", "idx_project_agent_managed_agent", "idx_project_name", "idx_project_revision_hash",
		"idx_project_run_agent", "idx_project_run_event_sequence", "idx_project_run_project_status",
		"idx_project_run_sandbox", "idx_project_run_scheduler", "idx_project_scheduler_agent",
		"idx_project_scheduler_id", "idx_project_scheduler_managed_loader", "idx_project_short_id",
		"idx_project_source_path", "idx_project_volumes_volume", "idx_volumes_driver", "idx_volumes_project",
		"idx_webhook_source_enabled_topic",
	} {
		assertSQLiteIndexExists(t, db, index)
	}
	assertSQLiteIndexUnique(t, db, "idx_event_idempotency", true)
	var idempotencyIndexSQL string
	if err := db.QueryRowContext(ctx,
		`SELECT sql FROM sqlite_master WHERE type = 'index' AND name = 'idx_event_idempotency'`).Scan(&idempotencyIndexSQL); err != nil {
		t.Fatalf("query partial idempotency index: %v", err)
	}
	if !strings.Contains(idempotencyIndexSQL, "WHERE idempotency_key != ''") {
		t.Fatalf("idempotency index is not partial: %s", idempotencyIndexSQL)
	}

	available, err := loadMigrations(embeddedMigrations)
	if err != nil {
		t.Fatalf("loadMigrations: %v", err)
	}
	var version int64
	var name string
	var checksum string
	if err := db.QueryRowContext(ctx,
		`SELECT version, name, checksum FROM schema_migrations`).Scan(&version, &name, &checksum); err != nil {
		t.Fatalf("query migration history: %v", err)
	}
	if version != baselineMigrationVersion || name != available[0].name || checksum != available[0].checksum {
		t.Fatalf("migration history = (%d, %q, %q), want (%d, %q, %q)",
			version, name, checksum, baselineMigrationVersion, available[0].name, available[0].checksum)
	}

	if _, err := db.ExecContext(ctx,
		`INSERT INTO project_revision(project_id, revision, spec_hash, spec_json) VALUES('missing', 1, 'hash', '{}')`); err == nil {
		t.Fatal("foreign key enforcement is disabled")
	}
	if _, err := db.ExecContext(ctx,
		`INSERT INTO global_env(name, value, secret, updated_at) VALUES('PRESERVED', 'before', 0, 1234)`); err != nil {
		t.Fatalf("insert data before second initialization: %v", err)
	}
	var appliedAt int64
	if err := db.QueryRowContext(ctx, `SELECT applied_at FROM schema_migrations WHERE version = 1`).Scan(&appliedAt); err != nil {
		t.Fatalf("query initial migration time: %v", err)
	}
	if err := store.InitSchema(ctx); err != nil {
		t.Fatalf("second InitSchema: %v", err)
	}
	var historyCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations`).Scan(&historyCount); err != nil {
		t.Fatalf("count migration history: %v", err)
	}
	if historyCount != 1 {
		t.Fatalf("migration history count = %d, want 1", historyCount)
	}
	var value string
	var updatedAt int64
	if err := db.QueryRowContext(ctx,
		`SELECT value, updated_at FROM global_env WHERE name = 'PRESERVED'`).Scan(&value, &updatedAt); err != nil {
		t.Fatalf("query data after second initialization: %v", err)
	}
	if value != "before" || updatedAt != 1234 {
		t.Fatalf("data after second initialization = (%q, %d), want (%q, %d)", value, updatedAt, "before", 1234)
	}
	var reappliedAt int64
	if err := db.QueryRowContext(ctx, `SELECT applied_at FROM schema_migrations WHERE version = 1`).Scan(&reappliedAt); err != nil {
		t.Fatalf("query migration time after second initialization: %v", err)
	}
	if reappliedAt != appliedAt {
		t.Fatalf("migration applied_at changed from %d to %d", appliedAt, reappliedAt)
	}
}

func TestLoadMigrationsValidation(t *testing.T) {
	tests := []struct {
		name    string
		files   fstest.MapFS
		wantErr string
	}{
		{
			name:    "no SQL files",
			files:   fstest.MapFS{"migrations/README.md": {Data: []byte("docs")}},
			wantErr: "no embedded",
		},
		{
			name:    "invalid filename",
			files:   fstest.MapFS{"migrations/1_bad.sql": {Data: []byte("SELECT 1")}},
			wantErr: "invalid SQLite migration filename",
		},
		{
			name:    "empty SQL",
			files:   fstest.MapFS{"migrations/000001_empty.sql": {Data: []byte(" \n")}},
			wantErr: "is empty",
		},
		{
			name: "duplicate version",
			files: fstest.MapFS{
				"migrations/000001_one.sql": {Data: []byte("SELECT 1")},
				"migrations/000001_two.sql": {Data: []byte("SELECT 2")},
			},
			wantErr: "duplicate SQLite migration version",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := loadMigrations(test.files)
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("loadMigrations error = %v, want containing %q", err, test.wantErr)
			}
		})
	}

	items, err := loadMigrations(fstest.MapFS{
		"migrations/000002_second.sql": {Data: []byte("SELECT 2")},
		"migrations/000001_first.sql":  {Data: []byte("SELECT 1")},
	})
	if err != nil {
		t.Fatalf("load sorted migrations: %v", err)
	}
	if items[0].version != 1 || items[1].version != 2 {
		t.Fatalf("migration order = %d, %d", items[0].version, items[1].version)
	}
}

func TestValidateAppliedMigrations(t *testing.T) {
	available := []migration{
		{version: 1, name: "000001_first", checksum: "one"},
		{version: 3, name: "000003_third", checksum: "three"},
	}
	tests := []struct {
		name    string
		applied []appliedMigration
		wantErr string
	}{
		{name: "valid prefix", applied: []appliedMigration{{version: 1, name: "000001_first", checksum: "one"}}},
		{name: "newer database", applied: []appliedMigration{{version: 1}, {version: 3}, {version: 4}}, wantErr: "newer than this binary"},
		{name: "non-prefix", applied: []appliedMigration{{version: 3, name: "000003_third", checksum: "three"}}, wantErr: "not an embedded prefix"},
		{name: "renamed", applied: []appliedMigration{{version: 1, name: "renamed", checksum: "one"}}, wantErr: "name mismatch"},
		{name: "modified", applied: []appliedMigration{{version: 1, name: "000001_first", checksum: "changed"}}, wantErr: "checksum mismatch"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateAppliedMigrations(available, test.applied)
			if test.wantErr == "" && err != nil {
				t.Fatalf("validateAppliedMigrations: %v", err)
			}
			if test.wantErr != "" && (err == nil || !strings.Contains(err.Error(), test.wantErr)) {
				t.Fatalf("error = %v, want containing %q", err, test.wantErr)
			}
		})
	}
}

func TestMigrationFailureRollsBackPendingBatch(t *testing.T) {
	ctx := context.Background()
	db := newMemoryDB(t)
	available, err := loadMigrations(embeddedMigrations)
	if err != nil {
		t.Fatalf("load baseline migration: %v", err)
	}
	first := available[0]
	if err := applyMigrationSet(ctx, db, []migration{first}); err != nil {
		t.Fatalf("apply first migration: %v", err)
	}
	second := migration{
		version: first.version + 1, name: "000002_broken",
		statement: `CREATE TABLE pending_table(id INTEGER); THIS IS NOT SQL;`, checksum: "two",
	}
	if err := applyMigrationSet(ctx, db, []migration{first, second}); err == nil {
		t.Fatal("broken migration returned nil error")
	}

	var pendingCount int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'pending_table'`).Scan(&pendingCount); err != nil {
		t.Fatalf("query pending table: %v", err)
	}
	if pendingCount != 0 {
		t.Fatalf("pending table count = %d, want 0", pendingCount)
	}
	var historyCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations`).Scan(&historyCount); err != nil {
		t.Fatalf("count migration history: %v", err)
	}
	if historyCount != 1 {
		t.Fatalf("migration history count = %d, want 1", historyCount)
	}
	var name, checksum string
	if err := db.QueryRowContext(ctx,
		`SELECT name, checksum FROM schema_migrations WHERE version = 1`).Scan(&name, &checksum); err != nil {
		t.Fatalf("query prior migration after rollback: %v", err)
	}
	if name != first.name || checksum != first.checksum {
		t.Fatalf("prior migration after rollback = (%q, %q), want (%q, %q)", name, checksum, first.name, first.checksum)
	}
}

func TestMigrationHonorsCanceledContext(t *testing.T) {
	db := newMemoryDB(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := applyMigrationSet(ctx, db, []migration{{
		version: 1, name: "000001_first", statement: `CREATE TABLE canceled(id INTEGER)`, checksum: "one",
	}})
	if err == nil {
		t.Fatal("canceled migration returned nil error")
	}
}

func TestSQLiteDSNConfiguresProductionConnection(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data.db")
	db, err := sql.Open("sqlite", sqliteDSN(dbPath, 5*time.Second))
	if err != nil {
		t.Fatalf("open SQLite: %v", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	t.Cleanup(func() { _ = db.Close() })

	var journalMode string
	if err := db.QueryRow(`PRAGMA journal_mode`).Scan(&journalMode); err != nil {
		t.Fatalf("query journal mode: %v", err)
	}
	if journalMode != "wal" {
		t.Fatalf("journal mode = %q, want wal", journalMode)
	}
	var foreignKeys, busyTimeout int
	if err := db.QueryRow(`PRAGMA foreign_keys`).Scan(&foreignKeys); err != nil {
		t.Fatalf("query foreign keys: %v", err)
	}
	if err := db.QueryRow(`PRAGMA busy_timeout`).Scan(&busyTimeout); err != nil {
		t.Fatalf("query busy timeout: %v", err)
	}
	if foreignKeys != 1 || busyTimeout != 5000 {
		t.Fatalf("connection PRAGMAs = (foreign_keys=%d, busy_timeout=%d), want (1, 5000)", foreignKeys, busyTimeout)
	}
}

func TestConfigStoreMigratesEveryLegacyAddedColumn(t *testing.T) {
	ctx := context.Background()
	db := newMemoryDB(t)
	available, err := loadMigrations(embeddedMigrations)
	if err != nil {
		t.Fatalf("load baseline: %v", err)
	}
	if _, err := db.ExecContext(ctx, available[0].statement); err != nil {
		t.Fatalf("create unversioned baseline fixture: %v", err)
	}

	for _, index := range []string{
		"idx_agent_definition_managed_project", "idx_loader_managed_project",
		"idx_project_short_id", "idx_project_agent_id", "idx_project_scheduler_id",
		"idx_project_run_sandbox", "idx_event_dispatch_attempt", "idx_event_parent",
	} {
		if _, err := db.ExecContext(ctx, `DROP INDEX `+index); err != nil {
			t.Fatalf("drop fixture index %s: %v", index, err)
		}
	}

	legacyColumns := map[string][]string{
		"agent_definition":  {"capset_ids", "volumes_json", "skills", "managed_project_id", "managed_project_revision", "managed_agent_name"},
		"llm_provider":      {"use_generic_responses_text_parts"},
		"loader":            {"agent_id", "capset_ids", "volumes_json", "managed_project_id", "managed_project_revision", "managed_agent_name", "managed_scheduler_id"},
		"project":           {"short_id"},
		"project_agent":     {"id", "name", "short_id"},
		"project_scheduler": {"id", "short_id"},
		"project_run":       {"sandbox_id"},
		"event":             {"replay_of_event_id", "claim_id", "claim_until", "attempt_count", "next_attempt_at", "last_error", "dead_letter_at"},
		"webhook_source":    {"token_header"},
	}
	for table, columns := range legacyColumns {
		for _, column := range columns {
			if _, err := db.ExecContext(ctx, fmt.Sprintf(`ALTER TABLE %q DROP COLUMN %q`, table, column)); err != nil {
				t.Fatalf("drop fixture column %s.%s: %v", table, column, err)
			}
		}
	}

	if _, err := db.ExecContext(ctx,
		`INSERT INTO agent_definition(id, name, created_at, updated_at) VALUES('agent-1', 'preserved', 1, 2)`); err != nil {
		t.Fatalf("insert legacy agent: %v", err)
	}
	if _, err := db.ExecContext(ctx,
		`INSERT INTO loader(id, name, script) VALUES('loader-1', 'preserved', 'return 1')`); err != nil {
		t.Fatalf("insert legacy loader: %v", err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO project(id, name) VALUES('project-1', 'preserved')`); err != nil {
		t.Fatalf("insert legacy project: %v", err)
	}

	store := FromDB(db)
	if err := store.InitSchema(ctx); err != nil {
		t.Fatalf("migrate legacy columns: %v", err)
	}
	for table, columns := range legacyColumns {
		assertTableColumns(t, store, table, columns...)
	}
	for _, index := range []string{
		"idx_agent_definition_managed_project", "idx_loader_managed_project",
		"idx_project_short_id", "idx_project_agent_id", "idx_project_scheduler_id",
		"idx_project_run_sandbox", "idx_event_dispatch_attempt", "idx_event_parent",
	} {
		assertSQLiteIndexExists(t, db, index)
	}
	for table, id := range map[string]string{"agent_definition": "agent-1", "loader": "loader-1", "project": "project-1"} {
		var name string
		if err := db.QueryRowContext(ctx, `SELECT name FROM `+table+` WHERE id = ?`, id).Scan(&name); err != nil {
			t.Fatalf("query preserved %s: %v", table, err)
		}
		if name != "preserved" {
			t.Fatalf("preserved %s name = %q", table, name)
		}
	}
}

func TestConfigStoreMigratesLegacyTimestampTables(t *testing.T) {
	ctx := context.Background()
	db := newMemoryDB(t)
	statements := []string{
		`CREATE TABLE global_env(name TEXT PRIMARY KEY, value TEXT NOT NULL, secret INTEGER NOT NULL, updated_at TEXT NOT NULL)`,
		`INSERT INTO global_env VALUES('A', 'one', 1, '2026-06-02T09:00:00Z')`,
		`CREATE TABLE workspace_config(id TEXT PRIMARY KEY, name TEXT NOT NULL, type TEXT NOT NULL, config_json TEXT NOT NULL, comment TEXT NOT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL)`,
		`INSERT INTO workspace_config VALUES('ws-1', 'Workspace', 'file', '{}', 'legacy', '2026-06-02T09:00:00Z', '2026-06-02T09:01:00Z')`,
	}
	for _, statement := range statements {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			t.Fatalf("prepare legacy timestamp fixture: %v", err)
		}
	}
	store := FromDB(db)
	if err := store.InitSchema(ctx); err != nil {
		t.Fatalf("migrate legacy timestamp tables: %v", err)
	}
	for table, columns := range map[string][]string{
		"global_env":       {"updated_at"},
		"workspace_config": {"created_at", "updated_at"},
	} {
		types, err := sqliteTableColumnTypes(ctx, db, table)
		if err != nil {
			t.Fatalf("inspect %s: %v", table, err)
		}
		for _, column := range columns {
			if !isIntegerColumnType(types[column]) {
				t.Fatalf("%s.%s type = %q, want integer", table, column, types[column])
			}
		}
	}
	if items, err := store.ListGlobalEnv(ctx); err != nil || len(items) != 1 || items[0].Name != "A" {
		t.Fatalf("migrated global env = %#v, err=%v", items, err)
	}
	if workspace, err := store.GetWorkspaceConfig(ctx, "ws-1"); err != nil || workspace.Name != "Workspace" {
		t.Fatalf("migrated workspace = %#v, err=%v", workspace, err)
	}
}

func TestConfigStoreMigratesLegacyLoaderTimestampsOnce(t *testing.T) {
	ctx := context.Background()
	db := newMemoryDB(t)
	available, err := loadMigrations(embeddedMigrations)
	if err != nil {
		t.Fatalf("load baseline: %v", err)
	}
	if _, err := db.ExecContext(ctx, available[0].statement); err != nil {
		t.Fatalf("create unversioned baseline fixture: %v", err)
	}
	const seconds int64 = 1_720_000_000
	statements := []string{
		`INSERT INTO loader(id, name, script) VALUES('loader-1', 'legacy', 'return 1')`,
		`INSERT INTO loader_trigger(loader_id, trigger_id, kind, next_fire_at, last_fired_at)
			VALUES('loader-1', 'trigger-1', 'interval', 1720000000, 1720000001)`,
		`INSERT INTO loader_run(loader_id, run_id, started_at, completed_at)
			VALUES('loader-1', 'run-1', 1720000002, 1720000003)`,
		`INSERT INTO loader_event(loader_id, event_id, type, created_at)
			VALUES('loader-1', 'event-1', 'legacy', 1720000004)`,
	}
	for _, statement := range statements {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			t.Fatalf("prepare legacy loader timestamp fixture: %v", err)
		}
	}

	store := FromDB(db)
	for pass := 1; pass <= 2; pass++ {
		if err := store.InitSchema(ctx); err != nil {
			t.Fatalf("InitSchema pass %d: %v", pass, err)
		}
	}
	var nextFireAt, lastFiredAt, startedAt, completedAt, eventCreatedAt int64
	if err := db.QueryRowContext(ctx,
		`SELECT next_fire_at, last_fired_at FROM loader_trigger WHERE loader_id = 'loader-1' AND trigger_id = 'trigger-1'`).
		Scan(&nextFireAt, &lastFiredAt); err != nil {
		t.Fatalf("query migrated loader trigger timestamps: %v", err)
	}
	if err := db.QueryRowContext(ctx,
		`SELECT started_at, completed_at FROM loader_run WHERE loader_id = 'loader-1' AND run_id = 'run-1'`).
		Scan(&startedAt, &completedAt); err != nil {
		t.Fatalf("query migrated loader run timestamps: %v", err)
	}
	if err := db.QueryRowContext(ctx,
		`SELECT created_at FROM loader_event WHERE loader_id = 'loader-1' AND event_id = 'event-1'`).
		Scan(&eventCreatedAt); err != nil {
		t.Fatalf("query migrated loader event timestamp: %v", err)
	}
	want := []int64{seconds * 1000, (seconds + 1) * 1000, (seconds + 2) * 1000, (seconds + 3) * 1000, (seconds + 4) * 1000}
	got := []int64{nextFireAt, lastFiredAt, startedAt, completedAt, eventCreatedAt}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("migrated loader timestamp %d = %d, want %d", index, got[index], want[index])
		}
	}
}

func TestApplyMigrationsRequiresBaselineVersion(t *testing.T) {
	db := newMemoryDB(t)
	err := applyMigrations(context.Background(), db, fstest.MapFS{
		"migrations/000002_second.sql": {Data: []byte("SELECT 1")},
	})
	if err == nil || !strings.Contains(err.Error(), "must be version 000001") {
		t.Fatalf("applyMigrations error = %v", err)
	}
}

func TestMigrationFSReadError(t *testing.T) {
	_, err := loadMigrations(failingMigrationFS{})
	if err == nil || !strings.Contains(err.Error(), "read embedded SQLite migrations") {
		t.Fatalf("loadMigrations error = %v", err)
	}
}

type failingMigrationFS struct{}

func (failingMigrationFS) Open(string) (fs.File, error) {
	return nil, fmt.Errorf("read failed")
}
