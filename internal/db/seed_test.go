package db

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/neko233/kanban233/internal/config"
)

func TestSeedDefaultRootUser(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(config.DatabaseConfig{
		Driver: "sqlite",
		DSN:    filepath.Join(dir, "seed.db"),
	}, config.AuthConfig{
		DefaultUser: config.DefaultUserConfig{Username: "root", Password: "root"},
	}, config.LocaleConfig{WeekStart: "monday"})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	user, err := store.GetUserByUsername(context.Background(), "root")
	if err != nil {
		t.Fatal(err)
	}
	if user.Username != "root" {
		t.Fatalf("expected root, got %s", user.Username)
	}
}

func TestSeedDemoContent(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(config.DatabaseConfig{
		Driver: "sqlite",
		DSN:    filepath.Join(dir, "demo.db"),
	}, config.AuthConfig{
		DefaultUser: config.DefaultUserConfig{Username: "root", Password: "root"},
	}, config.LocaleConfig{WeekStart: "monday"})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	ctx := context.Background()
	user, err := store.GetUserByUsername(ctx, "root")
	if err != nil {
		t.Fatal(err)
	}

	g1, err := store.projectGroupByExportKey(ctx, demoGroupTutorialKey)
	if err != nil {
		t.Fatalf("tutorial group: %v", err)
	}
	if g1.Name != "入门教学" || !g1.IsPublic {
		t.Fatalf("unexpected tutorial group: %+v", g1)
	}

	g2, err := store.projectGroupByExportKey(ctx, demoGroupAIKey)
	if err != nil {
		t.Fatalf("ai group: %v", err)
	}
	if g2.JoinMode != "apply" {
		t.Fatalf("expected apply join mode, got %s", g2.JoinMode)
	}

	boards, err := store.ListBoardsByGroup(ctx, g1.ID, user.ID)
	if err != nil || len(boards) != 1 {
		t.Fatalf("expected 1 board in tutorial group, got %d err=%v", len(boards), err)
	}
	if boards[0].Title != "入门教学" {
		t.Fatalf("unexpected board title: %s", boards[0].Title)
	}

	if err := store.SeedDemoContent(ctx, user.ID); err != nil {
		t.Fatal(err)
	}
	groups, err := store.ListProjectGroups(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	demoCount := 0
	for _, g := range groups {
		if g.Name == "入门教学" || g.Name == "AI 协作示例" {
			demoCount++
		}
	}
	if demoCount != 2 {
		t.Fatalf("expected 2 demo groups, counted %d in %d total groups", demoCount, len(groups))
	}
}
