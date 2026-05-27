package db

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/neko233/kanban233/internal/config"
	"github.com/neko233/kanban233/internal/models"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	store, err := Open(config.DatabaseConfig{
		Driver: "sqlite",
		DSN:    filepath.Join(dir, "test.db"),
	}, config.AuthConfig{
		DefaultUser: config.DefaultUserConfig{Username: "root", Password: "root"},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestUserBoardFlow(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	user, err := store.CreateUser(ctx, "alice", "hash")
	if err != nil {
		t.Fatal(err)
	}

	group, err := store.EnsureDefaultGroup(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}

	detail, err := store.CreateBoard(ctx, group.ID, user.ID, "Sprint 1")
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Columns) != 2 {
		t.Fatalf("expected 2 kanban columns (done column hidden), got %d", len(detail.Columns))
	}

	card, err := store.CreateCard(ctx, detail.Columns[0].ID, user.ID, "Task", "desc")
	if err != nil {
		t.Fatal(err)
	}

	moved, err := store.MoveCard(ctx, card.ID, user.ID, detail.Columns[1].ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if moved.ColumnID != detail.Columns[1].ID {
		t.Fatalf("expected column %d, got %d", detail.Columns[1].ID, moved.ColumnID)
	}
}

func TestExportImportBoard(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	user, err := store.CreateUser(ctx, "bob", "hash")
	if err != nil {
		t.Fatal(err)
	}
	group, err := store.EnsureDefaultGroup(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	detail, err := store.CreateBoard(ctx, group.ID, user.ID, "Import Test")
	if err != nil {
		t.Fatal(err)
	}

	exported, err := store.ExportBoard(ctx, detail.Board.ID, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if exported.Board.ExportKey == "" {
		t.Fatal("export key missing")
	}

	result, err := store.ImportBoard(ctx, user.ID, exported)
	if err != nil {
		t.Fatal(err)
	}
	if result.Action != "updated" {
		t.Fatalf("expected updated, got %s", result.Action)
	}

	stale := *exported
	stale.ExportedAt = exported.ExportedAt.Add(-time.Hour)
	_, err = store.ImportBoard(ctx, user.ID, &stale)
	if err == nil {
		t.Fatal("expected stale import error")
	}
}

func TestPublicGroupAccess(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	owner, _ := store.CreateUser(ctx, "owner", "hash")
	other, _ := store.CreateUser(ctx, "other", "hash")

	pub, err := store.CreateProjectGroup(ctx, owner.ID, "Public", "desc", true, models.JoinModeFree)
	if err != nil {
		t.Fatal(err)
	}
	detail, err := store.CreateBoard(ctx, pub.ID, owner.ID, "Shared")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := store.GetBoardDetail(ctx, detail.Board.ID, other.ID); err != nil {
		t.Fatalf("public board should be readable: %v", err)
	}
}

func TestExportAllPayload(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	user, _ := store.CreateUser(ctx, "carol", "hash")
	group, _ := store.EnsureDefaultGroup(ctx, user.ID)
	_, _ = store.CreateBoard(ctx, group.ID, user.ID, "B1")

	all, err := store.ExportAll(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if all.ExportType != "all_projects" {
		t.Fatalf("unexpected type %s", all.ExportType)
	}
	if len(all.Boards) != 1 {
		t.Fatalf("expected 1 board, got %d", len(all.Boards))
	}
	_ = models.ExportFormatVersion
}
