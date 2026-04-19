package cmd

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/icebear/dbops/internal/driver"
	"github.com/icebear/dbops/internal/proposal"
)

// Propose implements `dbops propose <db> ...`.
func Propose(args []string) error {
	fs := newFlagSet("propose")
	var bf baseFlags
	bf.bind(fs)
	var sqlFlag, file, note string
	var noDryRun bool
	fs.StringVar(&sqlFlag, "sql", "", "inline SQL")
	fs.StringVar(&file, "f", "", "read SQL from file")
	fs.StringVar(&note, "note", "", "human-readable justification")
	fs.BoolVar(&noDryRun, "no-dry-run", false, "skip EXPLAIN dry-run (still validates config + driver)")
	if err := parseMixed(fs, args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return fmt.Errorf(`usage: dbops propose <db> --sql "..." | -f FILE|- | (stdin) [--note "..."]`)
	}
	alias := fs.Arg(0)

	q, err := readSQL(sqlFlag, file)
	if err != nil {
		return err
	}
	q = strings.TrimSpace(q)
	if q == "" {
		return fmt.Errorf("empty SQL")
	}

	db, drv, dbc, closeDB, err := openDB(bf.ConfigPath, alias)
	if err != nil {
		return err
	}
	defer closeDB()

	p := proposal.Proposal{
		DB:     alias,
		Driver: dbc.Driver,
		SQL:    q,
		Note:   note,
	}

	if !noDryRun {
		if dr, err := doDryRun(context.Background(), db, drv, q); err == nil {
			p.DryRun = dr
		} else {
			p.DryRun = &proposal.DryRun{Explain: "dry-run failed: " + err.Error()}
		}
	}

	store, err := proposal.Open()
	if err != nil {
		return err
	}
	id, err := store.New(p)
	if err != nil {
		return err
	}

	fmt.Printf("proposal %s created.\n", id)
	fmt.Printf("next: run `sudo dbops apply %s` in a terminal you control.\n", id)
	fmt.Printf("inspect: `dbops proposal show %s`\n", id)
	return nil
}

func doDryRun(ctx context.Context, db *sql.DB, drv driver.Driver, q string) (*proposal.DryRun, error) {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx, drv.ExplainSQL(q, false))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sb strings.Builder
	for rows.Next() {
		var line sql.NullString
		if err := rows.Scan(&line); err != nil {
			return nil, err
		}
		if line.Valid {
			sb.WriteString(line.String)
			sb.WriteByte('\n')
		}
	}
	return &proposal.DryRun{Explain: sb.String()}, rows.Err()
}
