package db

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/neko233/kanban233/internal/config"
)

func TestMigrateLegacyCardsWithoutStatus(t *testing.T) {
	dir := t.TempDir()
	dsn := filepath.Join(dir, "legacy.db")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatal(err)
	}

	legacy := `
CREATE TABLE users (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	username TEXT NOT NULL UNIQUE,
	password_hash TEXT NOT NULL,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO users (username, password_hash) VALUES ('legacy', 'hash');

CREATE TABLE project_groups (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	owner_id INTEGER NOT NULL,
	name TEXT NOT NULL,
	description TEXT NOT NULL DEFAULT '',
	is_public INTEGER NOT NULL DEFAULT 0,
	join_mode TEXT NOT NULL DEFAULT 'free',
	export_key TEXT NOT NULL UNIQUE,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO project_groups (owner_id, name, export_key) VALUES (1, 'Legacy', 'legacy-key');

CREATE TABLE boards (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	project_group_id INTEGER NOT NULL,
	owner_id INTEGER NOT NULL,
	title TEXT NOT NULL,
	export_key TEXT NOT NULL UNIQUE,
	last_export_at DATETIME,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO boards (project_group_id, owner_id, title, export_key) VALUES (1, 1, 'Board', 'board-key');

CREATE TABLE columns (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	board_id INTEGER NOT NULL,
	title TEXT NOT NULL,
	position INTEGER NOT NULL DEFAULT 0,
	is_done INTEGER NOT NULL DEFAULT 0
);
INSERT INTO columns (board_id, title, position, is_done) VALUES (1, '待办', 0, 0), (1, '完成', 2, 1);

CREATE TABLE cards (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	column_id INTEGER NOT NULL,
	title TEXT NOT NULL,
	description TEXT NOT NULL DEFAULT '',
	position INTEGER NOT NULL DEFAULT 0,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO cards (column_id, title, description, position) VALUES (1, 'Old task', 'desc', 0);
`
	if _, err := db.Exec(legacy); err != nil {
		t.Fatalf("seed legacy schema: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	store, err := Open(config.DatabaseConfig{Driver: "sqlite", DSN: dsn}, config.AuthConfig{}, config.LocaleConfig{WeekStart: "monday"})
	if err != nil {
		t.Fatalf("open with migrate: %v", err)
	}
	defer store.Close()

	if !store.columnExists("cards", "status") {
		t.Fatal("expected status column after migrate")
	}

	var status string
	if err := store.db.QueryRow(`SELECT status FROM cards WHERE title = 'Old task'`).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "active" {
		t.Fatalf("expected active, got %q", status)
	}
}
