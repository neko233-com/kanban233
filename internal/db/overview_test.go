package db

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/neko233/kanban233/internal/config"
)

func TestGetResearchOverview(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(config.DatabaseConfig{
		Driver: "sqlite",
		DSN:    filepath.Join(dir, "overview.db"),
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
	overview, err := store.GetResearchOverview(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if overview.Summary.ProjectGroups < 2 {
		t.Fatalf("expected demo groups, got %+v", overview.Summary)
	}
	if len(overview.Groups) == 0 {
		t.Fatal("expected groups in overview")
	}
}
