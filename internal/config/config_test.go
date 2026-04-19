package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTOML(t *testing.T, body string, mode os.FileMode) string {
	t.Helper()
	d := t.TempDir()
	p := filepath.Join(d, "c.toml")
	if err := os.WriteFile(p, []byte(body), mode); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadRejectsLooseMode(t *testing.T) {
	p := writeTOML(t, `
[db.x]
driver = "postgres"
dsn    = "postgres://u:p@h/d"
`, 0o644)
	if _, err := Load(p); err == nil {
		t.Fatal("expected error for mode 0644")
	}
}

func TestLoadAndGet(t *testing.T) {
	p := writeTOML(t, `
default_db = "x"

[db.x]
driver = "postgres"
dsn    = "postgres://u:p@h/d"
notes  = "hi"

[db.y]
driver = "mysql"
dsn    = "u:p@tcp(h:3306)/d"
`, 0o600)
	cfg, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.DB) != 2 {
		t.Fatalf("want 2 DBs, got %d", len(cfg.DB))
	}
	if x, err := cfg.Get("x"); err != nil || x.Driver != "postgres" {
		t.Fatalf("get x: %v / %q", err, x.Driver)
	}
	if _, err := cfg.Get("z"); err == nil {
		t.Fatal("get z expected error")
	}
	if d, err := cfg.Get(""); err != nil || d.Driver != "postgres" {
		t.Fatalf("default: %v / %q", err, d.Driver)
	}
	als := cfg.Aliases()
	if len(als) != 2 || als[0] != "x" || als[1] != "y" {
		t.Fatalf("aliases: %v", als)
	}
}

func TestLoadRejectsUnknownDriver(t *testing.T) {
	p := writeTOML(t, `
[db.x]
driver = "oracle"
dsn    = "x"
`, 0o600)
	if _, err := Load(p); err == nil {
		t.Fatal("expected error for unknown driver")
	}
}

func TestLoadRejectsEmptyDSN(t *testing.T) {
	p := writeTOML(t, `
[db.x]
driver = "postgres"
dsn    = ""
`, 0o600)
	if _, err := Load(p); err == nil {
		t.Fatal("expected error for empty dsn")
	}
}

func TestLoadRejectsEmptyConfig(t *testing.T) {
	p := writeTOML(t, `default_db = "x"`, 0o600)
	if _, err := Load(p); err == nil {
		t.Fatal("expected error when no [db.*]")
	}
}
