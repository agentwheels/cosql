package cmd

import (
	"context"
	"database/sql"
	"fmt"
	"os"

	"github.com/oriengy/dbops/internal/output"
)

// Explain implements `dbops explain <db> ...`.
func Explain(args []string) error {
	fs := newFlagSet("explain")
	var bf baseFlags
	bf.bind(fs)
	var sqlFlag, file string
	var analyze bool
	fs.StringVar(&sqlFlag, "sql", "", "inline SQL")
	fs.StringVar(&file, "f", "", "read SQL from file")
	fs.BoolVar(&analyze, "analyze", false, "run EXPLAIN ANALYZE (executes the query!)")
	if err := parseMixed(fs, args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return fmt.Errorf(`usage: dbops explain <db> --sql "..." | -f FILE|- | (stdin) [--analyze]`)
	}
	alias := fs.Arg(0)

	q, err := readSQL(sqlFlag, file)
	if err != nil {
		return err
	}

	db, drv, _, closeDB, err := openDB(bf.ConfigPath, alias)
	if err != nil {
		return err
	}
	defer closeDB()

	ctx := context.Background()
	// Even EXPLAIN ANALYZE stays inside a read-only tx so that the wrapped
	// INSERT/UPDATE/DELETE cannot persist. (EXPLAIN without ANALYZE is inert,
	// so the txn is just defense in depth.)
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return err
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx, drv.ExplainSQL(q, analyze))
	if err != nil {
		return err
	}
	defer rows.Close()

	return output.RenderRows(os.Stdout, rows, bf.JSON)
}
