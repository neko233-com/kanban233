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
	})
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
