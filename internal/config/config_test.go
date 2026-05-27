package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "server.yaml")
	if err := os.WriteFile(path, []byte(`
server:
  addr: ":9090"
auth:
  registration_open: false
database:
  driver: sqlite
  dsn: "./test.db"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.Addr != ":9090" {
		t.Fatalf("addr = %q", cfg.Server.Addr)
	}
	if cfg.Auth.RegistrationOpen {
		t.Fatal("expected registration closed")
	}
}

func TestLoadInvalidDriver(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "server.yaml")
	if err := os.WriteFile(path, []byte(`
database:
  driver: mysql
  dsn: "x"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Load(path); err == nil {
		t.Fatal("expected error for unsupported driver")
	}
}
