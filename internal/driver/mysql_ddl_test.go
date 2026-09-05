package driver

import (
	"context"
	"database/sql"
	sqldriver "database/sql/driver"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"
)

type ddlConnector struct {
	tableParts                []string
	ddl                       string
	queryErr                  error
	rowErr                    error
	connects, begins, queries int
}

func (c *ddlConnector) Connect(context.Context) (sqldriver.Conn, error) {
	c.connects++
	return &ddlConn{c}, nil
}
func (c *ddlConnector) Driver() sqldriver.Driver { return ddlSQLDriver{c} }

type ddlSQLDriver struct{ c *ddlConnector }

func (d ddlSQLDriver) Open(string) (sqldriver.Conn, error) { return d.c.Connect(context.Background()) }

type ddlConn struct{ c *ddlConnector }

func (c *ddlConn) Prepare(string) (sqldriver.Stmt, error) {
	return nil, errors.New("unexpected prepare")
}
func (c *ddlConn) Close() error { return nil }
func (c *ddlConn) Begin() (sqldriver.Tx, error) {
	c.c.begins++
	return nil, errors.New("transactions unsupported")
}
func (c *ddlConn) QueryContext(_ context.Context, query string, args []sqldriver.NamedValue) (sqldriver.Rows, error) {
	c.c.queries++
	if c.c.queryErr != nil {
		return nil, c.c.queryErr
	}
	parts, err := parseDDLReference(query)
	if err != nil {
		return nil, err
	}
	if len(args) != 0 || !reflect.DeepEqual(parts, c.c.tableParts) {
		return nil, fmt.Errorf("table not found: %q", parts)
	}
	return &ddlRows{ddl: c.c.ddl, err: c.c.rowErr}, nil
}

// Parse the server's quoted identifier syntax so an injected second statement
// fails, while semicolons inside identifiers remain valid table names.
func parseDDLReference(query string) ([]string, error) {
	const prefix = "SHOW CREATE TABLE "
	if !strings.HasPrefix(query, prefix) {
		return nil, errors.New("unexpected database operation")
	}
	rest := strings.TrimPrefix(query, prefix)
	var parts []string
	for {
		if !strings.HasPrefix(rest, "`") {
			return nil, errors.New("expected quoted identifier")
		}
		rest = rest[1:]
		var name strings.Builder
		closed := false
		for len(rest) > 0 {
			if rest[0] != '`' {
				name.WriteByte(rest[0])
				rest = rest[1:]
				continue
			}
			if strings.HasPrefix(rest, "``") {
				name.WriteByte('`')
				rest = rest[2:]
				continue
			}
			rest = rest[1:]
			closed = true
			break
		}
		if !closed {
			return nil, errors.New("unterminated identifier")
		}
		parts = append(parts, name.String())
		if rest == "" {
			return parts, nil
		}
		if !strings.HasPrefix(rest, ".") {
			return nil, errors.New("statement escaped identifier")
		}
		rest = rest[1:]
	}
}

type ddlRows struct {
	ddl  string
	err  error
	done bool
}

func (r *ddlRows) Columns() []string { return []string{"Table", "Create Table"} }
func (r *ddlRows) Close() error      { return nil }
func (r *ddlRows) Next(values []sqldriver.Value) error {
	if r.err != nil {
		return r.err
	}
	if r.done {
		return io.EOF
	}
	r.done = true
	values[0], values[1] = "events", []byte(r.ddl)
	return nil
}

func ddlDB(t *testing.T, c *ddlConnector) *sql.DB {
	t.Helper()
	db := sql.OpenDB(c)
	t.Cleanup(func() { db.Close() })
	return db
}

func TestMySQLShowCreateTable(t *testing.T) {
	const ddl = "CREATE TABLE events (id BIGINT, PRIMARY KEY(id)) WITH (TTL = '2592000')"
	for _, tt := range []struct {
		name, table string
		parts       []string
	}{
		{"default database", "events", []string{"events"}},
		{"qualified", "growth.events", []string{"growth", "events"}},
		{"table escape", "events`; DROP TABLE events; --", []string{"events`; DROP TABLE events; --"}},
		{"schema escape", "growth`; DROP TABLE events; --.event`s;", []string{"growth`; DROP TABLE events; --", "event`s;"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			c := &ddlConnector{tableParts: tt.parts, ddl: ddl}
			got, err := (&mysqlDriver{}).ShowCreateTable(context.Background(), ddlDB(t, c), tt.table)
			if err != nil || got != ddl {
				t.Fatalf("ShowCreateTable = %q, %v; want intact primary key and TTL DDL", got, err)
			}
			if c.begins != 0 || c.queries != 1 {
				t.Fatalf("database operations: %d begins, %d queries", c.begins, c.queries)
			}
		})
	}
}

func TestMySQLShowCreateTableErrors(t *testing.T) {
	wantErr := errors.New("server failed")
	for _, stage := range []string{"query", "rows"} {
		t.Run(stage, func(t *testing.T) {
			c := &ddlConnector{tableParts: []string{"events"}}
			if stage == "query" {
				c.queryErr = wantErr
			} else {
				c.rowErr = wantErr
			}
			got, err := (&mysqlDriver{}).ShowCreateTable(context.Background(), ddlDB(t, c), "events")
			if !errors.Is(err, wantErr) || got != "" {
				t.Fatalf("ShowCreateTable = %q, %v; want propagated error", got, err)
			}
		})
	}
}

func TestPostgresShowCreateTableUnsupported(t *testing.T) {
	c := &ddlConnector{}
	got, err := (&postgres{}).ShowCreateTable(context.Background(), ddlDB(t, c), "events")
	if got != "" || err == nil || !strings.Contains(strings.ToLower(err.Error()), "unsupported") {
		t.Fatalf("ShowCreateTable = %q, %v; want unsupported error", got, err)
	}
	if c.connects != 0 {
		t.Fatalf("unsupported operation opened %d connections", c.connects)
	}
}
