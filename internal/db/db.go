package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/neko233/kanban233/internal/config"
	"github.com/neko233/kanban233/internal/locale"
	"github.com/neko233/kanban233/internal/models"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

type Store struct {
	db        *sql.DB
	driver    string
	weekStart time.Weekday
}

func Open(cfg config.DatabaseConfig, authCfg config.AuthConfig, localeCfg config.LocaleConfig) (*Store, error) {
	driverName := cfg.Driver
	if cfg.Driver == "postgres" {
		driverName = "pgx"
	}
	if cfg.Driver == "sqlite" {
		if err := os.MkdirAll(filepath.Dir(cfg.DSN), 0o755); err != nil {
			return nil, fmt.Errorf("create data dir: %w", err)
		}
	}

	db, err := sql.Open(driverName, cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	store := &Store{
		db:        db,
		driver:    cfg.Driver,
		weekStart: locale.ParseWeekStart(localeCfg.WeekStart),
	}
	if err := store.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := store.SeedDefaults(ctxBackground(), authCfg); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("seed defaults: %w", err)
	}

	return store, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) DB() *sql.DB {
	return s.db
}

func (s *Store) migrate() error {
	tables := sqliteSchemaTables()
	if s.driver == "postgres" {
		tables = postgresSchemaTables()
	}
	if _, err := s.db.Exec(tables); err != nil {
		return fmt.Errorf("migrate tables: %w", err)
	}
	if err := s.migrateLegacy(ctxBackground()); err != nil {
		return fmt.Errorf("migrate legacy: %w", err)
	}
	indexes := sqliteSchemaIndexes()
	if s.driver == "postgres" {
		indexes = postgresSchemaIndexes()
	}
	if _, err := s.db.Exec(indexes); err != nil {
		return fmt.Errorf("migrate indexes: %w", err)
	}
	return nil
}

var ctxBackground = func() context.Context { return context.Background() }

func sqliteSchemaTables() string {
	return `
CREATE TABLE IF NOT EXISTS users (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	username TEXT NOT NULL UNIQUE,
	password_hash TEXT NOT NULL,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS project_groups (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	owner_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	name TEXT NOT NULL,
	description TEXT NOT NULL DEFAULT '',
	is_public INTEGER NOT NULL DEFAULT 0,
	join_mode TEXT NOT NULL DEFAULT 'free',
	export_key TEXT NOT NULL UNIQUE,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS boards (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	project_group_id INTEGER NOT NULL REFERENCES project_groups(id) ON DELETE CASCADE,
	owner_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	title TEXT NOT NULL,
	export_key TEXT NOT NULL UNIQUE,
	last_export_at DATETIME,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS columns (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	board_id INTEGER NOT NULL REFERENCES boards(id) ON DELETE CASCADE,
	title TEXT NOT NULL,
	position INTEGER NOT NULL DEFAULT 0,
	is_done INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS cards (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	column_id INTEGER NOT NULL REFERENCES columns(id) ON DELETE CASCADE,
	assignee_id INTEGER REFERENCES users(id),
	title TEXT NOT NULL,
	description TEXT NOT NULL DEFAULT '',
	position INTEGER NOT NULL DEFAULT 0,
	status TEXT NOT NULL DEFAULT 'active',
	completed_at DATETIME,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS project_group_members (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	group_id INTEGER NOT NULL REFERENCES project_groups(id) ON DELETE CASCADE,
	user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	status TEXT NOT NULL DEFAULT 'active',
	joined_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(group_id, user_id)
);

CREATE TABLE IF NOT EXISTS audit_logs (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	action TEXT NOT NULL,
	resource_type TEXT NOT NULL,
	resource_id INTEGER NOT NULL DEFAULT 0,
	detail TEXT NOT NULL DEFAULT '',
	ip TEXT NOT NULL DEFAULT '',
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
`
}

func sqliteSchemaIndexes() string {
	return `
CREATE INDEX IF NOT EXISTS idx_groups_owner ON project_groups(owner_id);
CREATE INDEX IF NOT EXISTS idx_groups_public ON project_groups(is_public);
CREATE INDEX IF NOT EXISTS idx_boards_group ON boards(project_group_id);
CREATE INDEX IF NOT EXISTS idx_boards_owner ON boards(owner_id);
CREATE INDEX IF NOT EXISTS idx_columns_board ON columns(board_id);
CREATE INDEX IF NOT EXISTS idx_cards_column ON cards(column_id);
CREATE INDEX IF NOT EXISTS idx_cards_status ON cards(status);
CREATE INDEX IF NOT EXISTS idx_members_group ON project_group_members(group_id);
CREATE INDEX IF NOT EXISTS idx_members_user ON project_group_members(user_id);
CREATE INDEX IF NOT EXISTS idx_audit_created ON audit_logs(created_at);
CREATE INDEX IF NOT EXISTS idx_audit_user ON audit_logs(user_id);
`
}

func postgresSchemaTables() string {
	return `
CREATE TABLE IF NOT EXISTS users (
	id SERIAL PRIMARY KEY,
	username TEXT NOT NULL UNIQUE,
	password_hash TEXT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS project_groups (
	id SERIAL PRIMARY KEY,
	owner_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	name TEXT NOT NULL,
	description TEXT NOT NULL DEFAULT '',
	is_public BOOLEAN NOT NULL DEFAULT FALSE,
	join_mode TEXT NOT NULL DEFAULT 'free',
	export_key TEXT NOT NULL UNIQUE,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS boards (
	id SERIAL PRIMARY KEY,
	project_group_id INTEGER NOT NULL REFERENCES project_groups(id) ON DELETE CASCADE,
	owner_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	title TEXT NOT NULL,
	export_key TEXT NOT NULL UNIQUE,
	last_export_at TIMESTAMPTZ,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS columns (
	id SERIAL PRIMARY KEY,
	board_id INTEGER NOT NULL REFERENCES boards(id) ON DELETE CASCADE,
	title TEXT NOT NULL,
	position INTEGER NOT NULL DEFAULT 0,
	is_done BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE TABLE IF NOT EXISTS cards (
	id SERIAL PRIMARY KEY,
	column_id INTEGER NOT NULL REFERENCES columns(id) ON DELETE CASCADE,
	assignee_id INTEGER REFERENCES users(id),
	title TEXT NOT NULL,
	description TEXT NOT NULL DEFAULT '',
	position INTEGER NOT NULL DEFAULT 0,
	status TEXT NOT NULL DEFAULT 'active',
	completed_at TIMESTAMPTZ,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS project_group_members (
	id SERIAL PRIMARY KEY,
	group_id INTEGER NOT NULL REFERENCES project_groups(id) ON DELETE CASCADE,
	user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	status TEXT NOT NULL DEFAULT 'active',
	joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	UNIQUE(group_id, user_id)
);

CREATE TABLE IF NOT EXISTS audit_logs (
	id SERIAL PRIMARY KEY,
	user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	action TEXT NOT NULL,
	resource_type TEXT NOT NULL,
	resource_id INTEGER NOT NULL DEFAULT 0,
	detail TEXT NOT NULL DEFAULT '',
	ip TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
`
}

func postgresSchemaIndexes() string {
	return `
CREATE INDEX IF NOT EXISTS idx_groups_owner ON project_groups(owner_id);
CREATE INDEX IF NOT EXISTS idx_groups_public ON project_groups(is_public);
CREATE INDEX IF NOT EXISTS idx_boards_group ON boards(project_group_id);
CREATE INDEX IF NOT EXISTS idx_boards_owner ON boards(owner_id);
CREATE INDEX IF NOT EXISTS idx_columns_board ON columns(board_id);
CREATE INDEX IF NOT EXISTS idx_cards_column ON cards(column_id);
CREATE INDEX IF NOT EXISTS idx_cards_status ON cards(status);
CREATE INDEX IF NOT EXISTS idx_members_group ON project_group_members(group_id);
CREATE INDEX IF NOT EXISTS idx_members_user ON project_group_members(user_id);
CREATE INDEX IF NOT EXISTS idx_audit_created ON audit_logs(created_at);
CREATE INDEX IF NOT EXISTS idx_audit_user ON audit_logs(user_id);
`
}

func (s *Store) migrateLegacy(ctx context.Context) error {
	if !s.columnExists("boards", "project_group_id") {
		_ = s.execIgnore(`ALTER TABLE boards ADD COLUMN project_group_id INTEGER`)
		_ = s.execIgnore(`ALTER TABLE boards ADD COLUMN export_key TEXT`)
		_ = s.execIgnore(`ALTER TABLE boards ADD COLUMN last_export_at DATETIME`)
		if err := s.backfillLegacyBoards(ctx); err != nil {
			return err
		}
	}
	if !s.columnExists("project_groups", "join_mode") {
		_ = s.execIgnore(`ALTER TABLE project_groups ADD COLUMN join_mode TEXT NOT NULL DEFAULT 'free'`)
	}
	if !s.columnExists("columns", "is_done") {
		_ = s.execIgnore(`ALTER TABLE columns ADD COLUMN is_done INTEGER NOT NULL DEFAULT 0`)
		_, _ = s.db.Exec(`UPDATE columns SET is_done = 1 WHERE title IN ('完成', 'Done', 'done')`)
	}
	if !s.columnExists("cards", "status") {
		_ = s.execIgnore(`ALTER TABLE cards ADD COLUMN status TEXT NOT NULL DEFAULT 'active'`)
	}
	if !s.columnExists("cards", "completed_at") {
		_ = s.execIgnore(`ALTER TABLE cards ADD COLUMN completed_at DATETIME`)
	}
	if s.columnExists("cards", "status") && s.columnExists("columns", "is_done") {
		_, _ = s.db.ExecContext(ctx, s.q(`
UPDATE cards SET status = ?, completed_at = COALESCE(completed_at, updated_at)
WHERE status = ? AND column_id IN (SELECT id FROM columns WHERE is_done = 1)`),
			models.CardStatusCompleted, models.CardStatusActive)
	}
	if !s.tableExists("project_group_members") {
		_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS project_group_members (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	group_id INTEGER NOT NULL,
	user_id INTEGER NOT NULL,
	status TEXT NOT NULL DEFAULT 'active',
	joined_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(group_id, user_id)
)`)
		if err != nil {
			return err
		}
	}
	if !s.columnExists("cards", "assignee_id") {
		_ = s.execIgnore(`ALTER TABLE cards ADD COLUMN assignee_id INTEGER REFERENCES users(id)`)
	}
	return nil
}

func (s *Store) tableExists(name string) bool {
	if s.driver == "postgres" {
		var n int
		err := s.db.QueryRow(`SELECT COUNT(*) FROM information_schema.tables WHERE table_name = $1`, name).Scan(&n)
		return err == nil && n > 0
	}
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, name).Scan(&n)
	return err == nil && n > 0
}

func (s *Store) columnExists(table, column string) bool {
	if s.driver == "postgres" {
		var n int
		err := s.db.QueryRow(`
SELECT COUNT(*) FROM information_schema.columns
WHERE table_name = $1 AND column_name = $2`, table, column).Scan(&n)
		return err == nil && n > 0
	}
	rows, err := s.db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return false
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return false
		}
		if name == column {
			return true
		}
	}
	return false
}

func (s *Store) execIgnore(query string) error {
	_, err := s.db.Exec(query)
	if err != nil && strings.Contains(strings.ToLower(err.Error()), "duplicate column") {
		return nil
	}
	return err
}

func (s *Store) backfillLegacyBoards(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT owner_id FROM boards WHERE project_group_id IS NULL OR project_group_id = 0`)
	if err != nil {
		return err
	}
	defer rows.Close()

	var ownerIDs []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return err
		}
		ownerIDs = append(ownerIDs, id)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, ownerID := range ownerIDs {
		group, err := s.EnsureDefaultGroup(ctx, ownerID)
		if err != nil {
			return err
		}
		if _, err := s.db.ExecContext(ctx, s.q(`
UPDATE boards SET project_group_id = ?, export_key = COALESCE(export_key, ?)
WHERE owner_id = ? AND (project_group_id IS NULL OR project_group_id = 0 OR export_key IS NULL OR export_key = '')`),
			group.ID, newExportKey(), ownerID); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) WithTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) q(query string) string {
	if s.driver != "postgres" {
		return query
	}
	n := 1
	var b strings.Builder
	for _, ch := range query {
		if ch == '?' {
			b.WriteString(fmt.Sprintf("$%d", n))
			n++
			continue
		}
		b.WriteRune(ch)
	}
	return b.String()
}

func (s *Store) insertID(ctx context.Context, res sql.Result) (int64, error) {
	id, err := res.LastInsertId()
	if err == nil && id > 0 {
		return id, nil
	}
	return 0, fmt.Errorf("last insert id unsupported")
}

var ErrNotFound = errors.New("not found")
var ErrConflict = errors.New("conflict")
var ErrForbidden = errors.New("forbidden")
var ErrImportStale = errors.New("import stale")

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique") || strings.Contains(msg, "duplicate")
}
