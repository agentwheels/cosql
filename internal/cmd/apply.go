package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/icebear/dbops/internal/config"
	"github.com/icebear/dbops/internal/driver"
	"github.com/icebear/dbops/internal/proposal"
)

// Apply implements `dbops apply <id>`. Requires euid == 0 (sudo).
func Apply(args []string) error {
	fs := newFlagSet("apply")
	var bf baseFlags
	bf.bind(fs)
	var yes bool
	fs.BoolVar(&yes, "yes", false, "skip the y/N prompt (still requires sudo)")
	if err := parseMixed(fs, args); err != nil {
		return err
	}
	if os.Geteuid() != 0 {
		return fmt.Errorf("apply must run with sudo: sudo dbops apply <id>")
	}
	if fs.NArg() < 1 {
		return fmt.Errorf("usage: sudo dbops apply <id>")
	}
	id := fs.Arg(0)

	if err := fixHomeForSudo(); err != nil {
		return err
	}

	store, err := proposal.Open()
	if err != nil {
		return err
	}
	p, err := store.Get(id)
	if err != nil {
		return err
	}
	if p.Status != proposal.StatusPending {
		return fmt.Errorf("proposal %s status is %s, not pending", id, p.Status)
	}

	cfg, err := config.Load(bf.ConfigPath)
	if err != nil {
		return err
	}
	dbc, err := cfg.Get(p.DB)
	if err != nil {
		return err
	}
	if dbc.Driver != p.Driver {
		return fmt.Errorf("proposal recorded driver=%s but config has %s", p.Driver, dbc.Driver)
	}

	printProposal(os.Stdout, p)

	if !yes {
		fmt.Fprintf(os.Stdout, "\napply to %s? [y/N] ", p.DB)
		r := bufio.NewReader(os.Stdin)
		line, _ := r.ReadString('\n')
		if strings.ToLower(strings.TrimSpace(line)) != "y" {
			fmt.Println("aborted.")
			return nil
		}
	}

	drv, err := driver.For(dbc)
	if err != nil {
		return err
	}
	db, err := drv.Open()
	if err != nil {
		return err
	}
	defer db.Close()

	ctx := context.Background()
	start := time.Now()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	res, err := tx.ExecContext(ctx, p.SQL)
	if err != nil {
		tx.Rollback()
		finalize(store, p, 0, time.Since(start), err)
		return fmt.Errorf("exec failed (rolled back): %w", err)
	}
	affected, _ := res.RowsAffected()

	if err := tx.Commit(); err != nil {
		finalize(store, p, 0, time.Since(start), err)
		return fmt.Errorf("commit failed: %w", err)
	}
	dur := time.Since(start)

	p.Status = proposal.StatusApplied
	p.AppliedAt = time.Now().Format(time.RFC3339)
	p.AppliedBy = appliedBy()
	p.Result = &proposal.Result{
		AffectedRows: affected,
		DurationMS:   dur.Milliseconds(),
	}
	if err := store.Update(p); err != nil {
		return fmt.Errorf("update proposal: %w", err)
	}
	chownBack(filepath.Join(store.Dir, p.ID+".json"))

	writeAudit(p, affected, dur, nil)
	fmt.Printf("\napplied. affected=%d, duration=%s\n", affected, dur)
	return nil
}

func finalize(store *proposal.Store, p *proposal.Proposal, affected int64, dur time.Duration, execErr error) {
	p.Result = &proposal.Result{
		AffectedRows: affected,
		DurationMS:   dur.Milliseconds(),
	}
	if execErr != nil {
		p.Result.Error = execErr.Error()
	}
	_ = store.Update(p)
	chownBack(filepath.Join(store.Dir, p.ID+".json"))
	writeAudit(p, affected, dur, execErr)
}

func appliedBy() string {
	if s := os.Getenv("SUDO_USER"); s != "" {
		return s + " (via sudo)"
	}
	return "root"
}

func chownBack(path string) {
	uid, gid, ok := sudoOwnership()
	if !ok {
		return
	}
	_ = os.Chown(path, uid, gid)
}

func auditPath() (string, error) {
	d, err := proposal.DefaultDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(d), "audit.log"), nil
}

func writeAudit(p *proposal.Proposal, affected int64, dur time.Duration, execErr error) {
	path, err := auditPath()
	if err != nil {
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()

	status := "ok"
	if execErr != nil {
		status = "error:" + strings.ReplaceAll(execErr.Error(), "\n", " ")
	}
	fmt.Fprintf(f, "%s APPLY id=%s db=%s by=%s affected=%d dur=%dms %s\n",
		time.Now().Format(time.RFC3339),
		p.ID, p.DB, appliedBy(),
		affected, dur.Milliseconds(),
		status,
	)
	chownBack(path)
}
