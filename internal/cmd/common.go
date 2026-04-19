// Package cmd holds each `dbops <sub>` implementation.
package cmd

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/user"
	"strings"
	"time"

	"github.com/icebear/dbops/internal/config"
	"github.com/icebear/dbops/internal/driver"
)

// baseFlags are shared across subcommands.
type baseFlags struct {
	ConfigPath string
	JSON       bool
}

func (b *baseFlags) bind(fs *flag.FlagSet) {
	fs.StringVar(&b.ConfigPath, "config", "", "config file path (default ~/.config/dbops/config.toml)")
	fs.BoolVar(&b.JSON, "json", false, "emit JSON output")
}

func newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	return fs
}

// parseMixed calls fs.Parse but accepts flags and positional args in any
// order, matching git/docker/kubectl conventions. Go's stdlib flag stops
// parsing at the first non-flag token, which surprises users who write
// `dbops query <db> --sql "..."` (flags after the positional).
//
// We preserve the original ordering of each bucket and hand the re-sorted
// slice to fs.Parse so the rest of the code (fs.Args, fs.NArg, fs.Arg) keeps
// seeing what it expects.
func parseMixed(fs *flag.FlagSet, args []string) error {
	var flags, positional []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}
		if len(a) < 2 || a[0] != '-' {
			positional = append(positional, a)
			continue
		}
		name := strings.TrimLeft(a, "-")
		hasValue := false
		if eq := strings.IndexByte(name, '='); eq >= 0 {
			name = name[:eq]
			hasValue = true
		}
		f := fs.Lookup(name)
		if f == nil {
			// Unknown flag: pass through so fs.Parse surfaces a clear error.
			flags = append(flags, a)
			continue
		}
		if hasValue {
			flags = append(flags, a)
			continue
		}
		if bf, ok := f.Value.(interface{ IsBoolFlag() bool }); ok && bf.IsBoolFlag() {
			flags = append(flags, a)
			continue
		}
		// Non-bool flag: consume the following token as its value.
		if i+1 < len(args) {
			flags = append(flags, a, args[i+1])
			i++
			continue
		}
		flags = append(flags, a)
	}
	return fs.Parse(append(flags, positional...))
}

// openDB loads config, resolves alias, pings the connection. The caller must
// invoke the returned close fn.
func openDB(configPath, alias string) (*sql.DB, driver.Driver, config.Database, func(), error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, nil, config.Database{}, nil, err
	}
	dbc, err := cfg.Get(alias)
	if err != nil {
		return nil, nil, config.Database{}, nil, err
	}
	drv, err := driver.For(dbc)
	if err != nil {
		return nil, nil, dbc, nil, err
	}
	db, err := drv.Open()
	if err != nil {
		return nil, nil, dbc, nil, err
	}
	db.SetMaxOpenConns(4)
	db.SetConnMaxLifetime(5 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, nil, dbc, nil, fmt.Errorf("connect %s: %w", alias, err)
	}
	return db, drv, dbc, func() { db.Close() }, nil
}

// readSQL returns SQL from one of three sources, in precedence order:
//   1. --sql "..."                   (inline)
//   2. -f FILE   or   -f -           (file on disk, or "-" for stdin)
//   3. stdin, when it's piped in and neither flag is set (implicit)
// --sql and -f are mutually exclusive.
func readSQL(sqlFlag, fileFlag string) (string, error) {
	if sqlFlag != "" && fileFlag != "" {
		return "", fmt.Errorf("--sql and -f are mutually exclusive; pick one")
	}
	if sqlFlag != "" {
		return sqlFlag, nil
	}
	if fileFlag == "-" {
		return readStdin("-f -")
	}
	if fileFlag != "" {
		b, err := os.ReadFile(fileFlag)
		if err != nil {
			return "", fmt.Errorf("read -f %s: %w", fileFlag, err)
		}
		return string(b), nil
	}
	// Neither flag given. Only read stdin implicitly if it's piped — never
	// when it's a terminal, to avoid hanging on a forgotten flag.
	if !stdinIsTerminal() {
		s, err := readStdin("stdin")
		if err != nil {
			return "", err
		}
		if len(bytes.TrimSpace([]byte(s))) > 0 {
			return s, nil
		}
	}
	return "", fmt.Errorf("no SQL given: pass --sql \"...\", -f FILE, or pipe via stdin")
}

func readStdin(label string) (string, error) {
	b, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", label, err)
	}
	return string(b), nil
}

func stdinIsTerminal() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		// On error, treat as terminal — the safer default, since we won't
		// block trying to read a pipe that doesn't exist.
		return true
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// outputJSON writes v as indented JSON to stdout.
func outputJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// fixHomeForSudo rewrites $HOME to SUDO_USER's home when running under sudo.
// That way DefaultPath() and DefaultDir() resolve to the invoking user's
// config/data directories instead of root's.
func fixHomeForSudo() error {
	if os.Geteuid() != 0 {
		return nil
	}
	sudoUser := os.Getenv("SUDO_USER")
	if sudoUser == "" {
		return nil
	}
	u, err := user.Lookup(sudoUser)
	if err != nil {
		return fmt.Errorf("lookup SUDO_USER %q: %w", sudoUser, err)
	}
	return os.Setenv("HOME", u.HomeDir)
}

// sudoOwnership returns (uid, gid, ok) of the invoking user when running
// under sudo, so we can chown files back after writing them as root.
func sudoOwnership() (int, int, bool) {
	if os.Geteuid() != 0 {
		return 0, 0, false
	}
	sudoUID := os.Getenv("SUDO_UID")
	sudoGID := os.Getenv("SUDO_GID")
	if sudoUID == "" {
		return 0, 0, false
	}
	var uid, gid int
	_, _ = fmt.Sscanf(sudoUID, "%d", &uid)
	_, _ = fmt.Sscanf(sudoGID, "%d", &gid)
	if gid == 0 {
		gid = -1
	}
	return uid, gid, true
}
