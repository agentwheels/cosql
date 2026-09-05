package cmd

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/agentwheels/cosql/internal/driver"
	"github.com/agentwheels/cosql/internal/proposal"
)

// Propose implements `cosql propose <db> ...`.
func Propose(args []string) error {
	fs := newFlagSet("propose")
	var bf baseFlags
	bf.bind(fs)
	var sqlFlag, file, note string
	var noDryRun, noTransaction bool
	fs.StringVar(&sqlFlag, "sql", "", "inline SQL")
	fs.StringVar(&file, "f", "", "read SQL from file")
	fs.StringVar(&note, "note", "", "human-readable justification")
	fs.BoolVar(&noDryRun, "no-dry-run", false, "skip EXPLAIN dry-run (still validates config + driver)")
	fs.BoolVar(&noTransaction, "no-transaction", false, "apply without a transaction; skips EXPLAIN and cannot roll back")
	if err := parseMixed(fs, args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return fmt.Errorf(`usage: cosql propose <db> --sql "..." | -f FILE|- | (stdin) [--note "..."]`)
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
		DB:            alias,
		Driver:        dbc.Driver,
		SQL:           q,
		Note:          note,
		NoTransaction: noTransaction,
	}

	if noTransaction || !noDryRun {
		p.DryRun = proposalDryRun(context.Background(), db, drv, q, noTransaction)
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
	if noTransaction {
		fmt.Println("mode: no transaction; SQL has not been validated and apply cannot roll back")
	}
	fmt.Printf("next: run `sudo cosql apply %s` in a terminal you control.\n", id)
	fmt.Printf("inspect: `cosql proposal show %s`\n", id)
	return nil
}

func proposalDryRun(ctx context.Context, db *sql.DB, drv driver.Driver, q string, noTransaction bool) *proposal.DryRun {
	if noTransaction {
		// 无只读事务保护时不发送任意 SQL，避免试跑产生真实变更。
		return &proposal.DryRun{Explain: "dry-run skipped: no-transaction mode; SQL has not been validated"}
	}
	dr, err := doDryRun(ctx, db, drv, q)
	if err != nil {
		return &proposal.DryRun{Explain: "dry-run failed: " + err.Error()}
	}
	return dr
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
