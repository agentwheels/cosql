package driver

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type postgres struct{ dsn string }

func (p *postgres) Open() (*sql.DB, error) { return sql.Open("pgx", p.dsn) }
func (p *postgres) Kind() string            { return "postgres" }

func (p *postgres) ExplainSQL(q string, analyze bool) string {
	if analyze {
		return "EXPLAIN (ANALYZE, BUFFERS, VERBOSE, FORMAT TEXT) " + q
	}
	return "EXPLAIN (VERBOSE, FORMAT TEXT) " + q
}

func (p *postgres) ListObjects(ctx context.Context, db *sql.DB) ([]SchemaRow, error) {
	const q = `
SELECT n.nspname                                  AS schema,
       c.relname                                  AS name,
       CASE c.relkind
         WHEN 'r' THEN 'table'
         WHEN 'p' THEN 'partitioned table'
         WHEN 'v' THEN 'view'
         WHEN 'm' THEN 'matview'
         WHEN 'f' THEN 'foreign table'
       END                                        AS kind,
       c.reltuples::bigint                        AS rows
FROM pg_class c
JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE c.relkind IN ('r','p','v','m','f')
  AND n.nspname NOT IN ('pg_catalog','information_schema')
  AND n.nspname NOT LIKE 'pg_toast%'
ORDER BY 1, 2`
	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SchemaRow
	for rows.Next() {
		var r SchemaRow
		var approx sql.NullInt64
		if err := rows.Scan(&r.Schema, &r.Name, &r.Kind, &approx); err != nil {
			return nil, err
		}
		if approx.Valid {
			v := approx.Int64
			r.Rows = &v
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (p *postgres) DescribeTable(ctx context.Context, db *sql.DB, table string) (*TableInfo, error) {
	schema, name := splitSchema(table, "public")
	info := &TableInfo{Schema: schema, Name: name}

	colQ := `
SELECT column_name,
       CASE
         WHEN data_type IN ('character varying','character')
              AND character_maximum_length IS NOT NULL
           THEN data_type || '(' || character_maximum_length || ')'
         WHEN data_type IN ('numeric','decimal') AND numeric_precision IS NOT NULL
           THEN data_type || '(' || numeric_precision || COALESCE(',' || numeric_scale, '') || ')'
         ELSE data_type
       END                              AS type,
       is_nullable = 'YES'               AS nullable,
       COALESCE(column_default, '')      AS dflt
FROM information_schema.columns
WHERE table_schema = $1 AND table_name = $2
ORDER BY ordinal_position`
	if err := queryCols(ctx, db, colQ, schema, name, info); err != nil {
		return nil, err
	}

	idxQ := `
SELECT i.relname,
       array_to_string(array_agg(a.attname ORDER BY k.n), ',') AS cols,
       ix.indisunique,
       ix.indisprimary
FROM pg_index ix
JOIN pg_class i       ON i.oid = ix.indexrelid
JOIN pg_class t       ON t.oid = ix.indrelid
JOIN pg_namespace n   ON n.oid = t.relnamespace
JOIN LATERAL unnest(ix.indkey) WITH ORDINALITY AS k(attnum, n) ON TRUE
JOIN pg_attribute a   ON a.attrelid = t.oid AND a.attnum = k.attnum
WHERE n.nspname = $1 AND t.relname = $2
GROUP BY i.relname, ix.indisunique, ix.indisprimary
ORDER BY i.relname`
	if err := queryIdx(ctx, db, idxQ, schema, name, info); err != nil {
		return nil, err
	}

	fkQ := `
SELECT con.conname,
       (SELECT array_to_string(array_agg(a.attname ORDER BY k.n), ',')
          FROM unnest(con.conkey) WITH ORDINALITY AS k(attnum, n)
          JOIN pg_attribute a ON a.attrelid = con.conrelid AND a.attnum = k.attnum) AS cols,
       nf.nspname || '.' || tf.relname AS ref_table,
       (SELECT array_to_string(array_agg(a.attname ORDER BY k.n), ',')
          FROM unnest(con.confkey) WITH ORDINALITY AS k(attnum, n)
          JOIN pg_attribute a ON a.attrelid = con.confrelid AND a.attnum = k.attnum) AS ref_cols
FROM pg_constraint con
JOIN pg_class t      ON t.oid = con.conrelid
JOIN pg_class tf     ON tf.oid = con.confrelid
JOIN pg_namespace n  ON n.oid = t.relnamespace
JOIN pg_namespace nf ON nf.oid = tf.relnamespace
WHERE con.contype = 'f' AND n.nspname = $1 AND t.relname = $2
ORDER BY con.conname`
	if err := queryFK(ctx, db, fkQ, schema, name, info); err != nil {
		return nil, err
	}

	if len(info.Columns) == 0 {
		return nil, fmt.Errorf("table %s.%s not found", schema, name)
	}
	return info, nil
}

// Helpers shared between postgres.go and mysql.go.

func queryCols(ctx context.Context, db *sql.DB, q, schema, name string, info *TableInfo) error {
	rows, err := db.QueryContext(ctx, q, schema, name)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var c ColumnInfo
		if err := rows.Scan(&c.Name, &c.Type, &c.Nullable, &c.Default); err != nil {
			return err
		}
		info.Columns = append(info.Columns, c)
	}
	return rows.Err()
}

func queryIdx(ctx context.Context, db *sql.DB, q, schema, name string, info *TableInfo) error {
	rows, err := db.QueryContext(ctx, q, schema, name)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var idx IndexInfo
		var cols string
		if err := rows.Scan(&idx.Name, &cols, &idx.Unique, &idx.Primary); err != nil {
			return err
		}
		idx.Columns = splitCSV(cols)
		info.Indexes = append(info.Indexes, idx)
	}
	return rows.Err()
}

func queryFK(ctx context.Context, db *sql.DB, q, schema, name string, info *TableInfo) error {
	rows, err := db.QueryContext(ctx, q, schema, name)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var fk FKInfo
		var cols, ref string
		if err := rows.Scan(&fk.Name, &cols, &fk.RefTable, &ref); err != nil {
			return err
		}
		fk.Columns = splitCSV(cols)
		fk.RefColumns = splitCSV(ref)
		info.ForeignKeys = append(info.ForeignKeys, fk)
	}
	return rows.Err()
}
