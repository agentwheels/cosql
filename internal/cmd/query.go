package cmd

import (
	"context"
	"database/sql"
	"fmt"
	"os"

	"github.com/oriengy/dbops/internal/output"
)

// Query implements `dbops query <db> ...`.
func Query(args []string) error {
	fs := newFlagSet("query")
	var bf baseFlags
	bf.bind(fs)
	var sqlFlag, file string
	fs.StringVar(&sqlFlag, "sql", "", "inline SQL")
	fs.StringVar(&file, "f", "", "read SQL from file")
	if err := parseMixed(fs, args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return fmt.Errorf(`usage: dbops query <db> --sql "..." | -f FILE|- | (stdin)`)
	}
	alias := fs.Arg(0)

	q, err := readSQL(sqlFlag, file)
	if err != nil {
		return err
	}

	db, _, _, closeDB, err := openDB(bf.ConfigPath, alias)
	if err != nil {
		return err
	}
	defer closeDB()

	ctx := context.Background()
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return err
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx, q)
	if err != nil {
		return err
	}
	defer rows.Close()

	return output.RenderRows(os.Stdout, rows, bf.JSON)
}
