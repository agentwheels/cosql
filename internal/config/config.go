// Package config loads and validates the cosql TOML config file.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/BurntSushi/toml"
)

// Config is the top-level TOML config.
type Config struct {
	DefaultDB string              `toml:"default_db"`
	DB        map[string]Database `toml:"db"`
}

// Database describes a single db alias.
type Database struct {
	Driver string `toml:"driver"` // "postgres" or "mysql"
	DSN    string `toml:"dsn"`
	Notes  string `toml:"notes"`
}

// DefaultPath returns ~/.config/cosql/config.toml.
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "cosql", "config.toml"), nil
}

// Load reads and validates the config at path. An empty path resolves to
// DefaultPath. The file must have mode 0600 (no group/other bits).
func Load(path string) (*Config, error) {
	if path == "" {
		p, err := DefaultPath()
		if err != nil {
			return nil, err
		}
		path = p
	}

	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("config not found: %s (copy examples/config.toml or run `make install-example-config`)", path)
		}
		return nil, err
	}
	if info.Mode().Perm()&0077 != 0 {
		return nil, fmt.Errorf("config %s has permissions %o; must be 0600. Run: chmod 600 %s", path, info.Mode().Perm(), path)
	}

	var c Config
	if _, err := toml.DecodeFile(path, &c); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if len(c.DB) == 0 {
		return nil, fmt.Errorf("no [db.*] tables defined in %s", path)
	}
	for name, db := range c.DB {
		if db.Driver != "postgres" && db.Driver != "mysql" {
			return nil, fmt.Errorf("db %q: driver must be postgres or mysql, got %q", name, db.Driver)
		}
		if db.DSN == "" {
			return nil, fmt.Errorf("db %q: dsn is empty", name)
		}
	}
	return &c, nil
}

// Get returns the database for an alias. If alias is empty, DefaultDB is used.
func (c *Config) Get(alias string) (Database, error) {
	if alias == "" {
		alias = c.DefaultDB
	}
	if alias == "" {
		return Database{}, fmt.Errorf("no db alias given and default_db is not set")
	}
	db, ok := c.DB[alias]
	if !ok {
		return Database{}, fmt.Errorf("no such db alias: %q (configured: %v)", alias, c.Aliases())
	}
	return db, nil
}

// Aliases returns a sorted list of configured db aliases.
func (c *Config) Aliases() []string {
	out := make([]string, 0, len(c.DB))
	for k := range c.DB {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
