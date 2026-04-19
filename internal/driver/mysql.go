package driver

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/go-sql-driver/mysql"
)

type mysqlDriver struct{ dsn string }

func (m *mysqlDriver) Open() (*sql.DB, error) { return sql.Open("mysql", m.dsn) }
func (m *mysqlDriver) Kind() string            { return "mysql" }

func (m *mysqlDriver) ExplainSQL(q string, analyze bool) string {
	if analyze {
		return "EXPLAIN ANALYZE " + q
	}
	return "EXPLAIN FORMAT=TREE " + q
}

func (m *mysqlDriver) ListObjects(ctx context.Context, db *sql.DB) ([]SchemaRow, error) {
	const q = `
SELECT table_schema, table_name, table_type, IFNULL(table_rows, 0)
FROM information_schema.tables
WHERE table_schema NOT IN ('information_schema','performance_schema','mysql','sys')
ORDER BY 1, 2`
	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SchemaRow
	for rows.Next() {
		var r SchemaRow
		var kind string
		var rc int64
		if err := rows.Scan(&r.Schema, &r.Name, &kind, &rc); err != nil {
			return nil, err
		}
		switch kind {
		case "BASE TABLE":
			r.Kind = "table"
		case "VIEW":
			r.Kind = "view"
		default:
			r.Kind = kind
		}
		r.Rows = &rc
		out = append(out, r)
	}
	return out, rows.Err()
}

func (m *mysqlDriver) DescribeTable(ctx context.Context, db *sql.DB, table string) (*TableInfo, error) {
	schema, name := splitSchema(table, "")
	if schema == "" {
		if err := db.QueryRowContext(ctx, "SELECT DATABASE()").Scan(&schema); err != nil {
			return nil, fmt.Errorf("resolve default schema: %w", err)
		}
		if schema == "" {
			return nil, fmt.Errorf("no default schema; use schema.table")
		}
	}
	info := &TableInfo{Schema: schema, Name: name}

	colQ := `
SELECT column_name, column_type, is_nullable = 'YES', IFNULL(column_default, '')
FROM information_schema.columns
WHERE table_schema = ? AND table_name = ?
ORDER BY ordinal_position`
	if err := queryCols(ctx, db, colQ, schema, name, info); err != nil {
		return nil, err
	}

	idxQ := `
SELECT index_name,
       GROUP_CONCAT(column_name ORDER BY seq_in_index),
       MAX(non_unique) = 0,
       index_name = 'PRIMARY'
FROM information_schema.statistics
WHERE table_schema = ? AND table_name = ?
GROUP BY index_name
ORDER BY index_name`
	if err := queryIdx(ctx, db, idxQ, schema, name, info); err != nil {
		return nil, err
	}

	fkQ := `
SELECT kcu.constraint_name,
       GROUP_CONCAT(kcu.column_name ORDER BY kcu.ordinal_position),
       CONCAT(kcu.referenced_table_schema, '.', kcu.referenced_table_name),
       GROUP_CONCAT(kcu.referenced_column_name ORDER BY kcu.ordinal_position)
FROM information_schema.key_column_usage kcu
WHERE kcu.table_schema = ?
  AND kcu.table_name   = ?
  AND kcu.referenced_table_name IS NOT NULL
GROUP BY kcu.constraint_name, kcu.referenced_table_schema, kcu.referenced_table_name
ORDER BY kcu.constraint_name`
	if err := queryFK(ctx, db, fkQ, schema, name, info); err != nil {
		return nil, err
	}

	if len(info.Columns) == 0 {
		return nil, fmt.Errorf("table %s.%s not found", schema, name)
	}
	return info, nil
}
