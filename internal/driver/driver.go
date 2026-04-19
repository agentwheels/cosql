// Package driver abstracts the engine-specific bits of dbops: connection
// opening, schema introspection, and EXPLAIN prefix. Query/exec themselves go
// through database/sql directly.
package driver

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/oriengy/dbops/internal/config"
)

// Driver is implemented per engine. Callers Open once, pass the *sql.DB to the
// other methods so connections can be reused.
type Driver interface {
	Open() (*sql.DB, error)
	Kind() string // "postgres" or "mysql"
	ListObjects(ctx context.Context, db *sql.DB) ([]SchemaRow, error)
	DescribeTable(ctx context.Context, db *sql.DB, table string) (*TableInfo, error)
	// ExplainSQL wraps a user query into an EXPLAIN that returns rows.
	ExplainSQL(sql string, analyze bool) string
}

// SchemaRow is one row of `schema` output.
type SchemaRow struct {
	Schema string
	Name   string
	Kind   string
	Rows   *int64 // approximate, may be nil
}

// TableInfo is the full description of a single table.
type TableInfo struct {
	Schema      string
	Name        string
	Columns     []ColumnInfo
	Indexes     []IndexInfo
	ForeignKeys []FKInfo
}

type ColumnInfo struct {
	Name     string
	Type     string
	Nullable bool
	Default  string
}

type IndexInfo struct {
	Name    string
	Columns []string
	Unique  bool
	Primary bool
}

type FKInfo struct {
	Name       string
	Columns    []string
	RefTable   string
	RefColumns []string
}

// For returns a Driver for the given db config.
func For(db config.Database) (Driver, error) {
	switch db.Driver {
	case "postgres":
		return &postgres{dsn: db.DSN}, nil
	case "mysql":
		return &mysqlDriver{dsn: db.DSN}, nil
	default:
		return nil, fmt.Errorf("unsupported driver: %s", db.Driver)
	}
}

// splitSchema splits "schema.table" into parts. If no dot, returns
// (defaultSchema, table).
func splitSchema(table, defaultSchema string) (schema, name string) {
	if i := strings.Index(table, "."); i > 0 {
		return table[:i], table[i+1:]
	}
	return defaultSchema, table
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, ",")
}
