package cmd

import (
	"context"
	"database/sql"
	sqldriver "database/sql/driver"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/agentwheels/cosql/internal/proposal"
)

type transactionRecorder struct {
	events                       []string
	beginErr, execErr, commitErr error
}

func (r *transactionRecorder) Connect(context.Context) (sqldriver.Conn, error) {
	return &recordingConn{r}, nil
}
func (r *transactionRecorder) Driver() sqldriver.Driver { return recordingDriver{r} }

type recordingDriver struct{ r *transactionRecorder }

func (d recordingDriver) Open(string) (sqldriver.Conn, error) { return &recordingConn{d.r}, nil }

type recordingConn struct{ r *transactionRecorder }

func (c *recordingConn) Prepare(string) (sqldriver.Stmt, error) {
	return nil, errors.New("unexpected Prepare")
}
func (c *recordingConn) Close() error { return nil }
func (c *recordingConn) Begin() (sqldriver.Tx, error) {
	return c.BeginTx(context.Background(), sqldriver.TxOptions{})
}
func (c *recordingConn) BeginTx(_ context.Context, opts sqldriver.TxOptions) (sqldriver.Tx, error) {
	event := "begin"
	if opts.ReadOnly {
		event = "begin read-only"
	}
	c.r.events = append(c.r.events, event)
	if c.r.beginErr != nil {
		return nil, c.r.beginErr
	}
	return &recordingTx{c.r}, nil
}
func (c *recordingConn) ExecContext(_ context.Context, q string, _ []sqldriver.NamedValue) (sqldriver.Result, error) {
	c.r.events = append(c.r.events, "exec: "+q)
	if c.r.execErr != nil {
		return nil, c.r.execErr
	}
	return sqldriver.RowsAffected(1), nil
}
func (c *recordingConn) QueryContext(_ context.Context, q string, _ []sqldriver.NamedValue) (sqldriver.Rows, error) {
	c.r.events = append(c.r.events, "query: "+q)
	return nil, errors.New("unexpected query")
}

type recordingTx struct{ r *transactionRecorder }

func (tx *recordingTx) Commit() error {
	tx.r.events = append(tx.r.events, "commit")
	return tx.r.commitErr
}
func (tx *recordingTx) Rollback() error {
	tx.r.events = append(tx.r.events, "rollback")
	return nil
}

func recordedDB(t *testing.T, r *transactionRecorder) *sql.DB {
	t.Helper()
	db := sql.OpenDB(r)
	t.Cleanup(func() { db.Close() })
	return db
}

func TestExecuteProposalTransactionMode(t *testing.T) {
	const statement = "CREATE TABLE example (id BIGINT PRIMARY KEY)"
	execFailure := errors.New("server rejected statement")
	commitFailure := errors.New("connection lost during commit")
	beginFailure := errors.New("transactions unsupported")
	tests := []struct {
		name                         string
		noTransaction                bool
		beginErr, execErr, commitErr error
		wantEvents                   []string
		wantErr                      error
		wantMessage                  string
	}{
		{name: "no transaction server", noTransaction: true, beginErr: beginFailure, wantEvents: []string{"exec: " + statement}},
		{name: "default commits", wantEvents: []string{"begin", "exec: " + statement, "commit"}},
		{name: "default failure rolls back", execErr: execFailure, wantEvents: []string{"begin", "exec: " + statement, "rollback"}, wantErr: execFailure, wantMessage: "rolled back"},
		{name: "no transaction failure warns of partial application", noTransaction: true, execErr: execFailure, wantEvents: []string{"exec: " + statement}, wantErr: execFailure, wantMessage: "may have been applied"},
		{name: "commit failure is reported", commitErr: commitFailure, wantEvents: []string{"begin", "exec: " + statement, "commit"}, wantErr: commitFailure, wantMessage: "commit failed"},
		{name: "default never silently bypasses rejected transaction", beginErr: beginFailure, wantEvents: []string{"begin"}, wantErr: beginFailure},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &transactionRecorder{beginErr: tt.beginErr, execErr: tt.execErr, commitErr: tt.commitErr}
			res, err := executeProposal(context.Background(), recordedDB(t, r), &proposal.Proposal{SQL: statement, NoTransaction: tt.noTransaction})
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr != nil {
				if !strings.Contains(err.Error(), tt.wantMessage) {
					t.Errorf("error %q lacks %q", err, tt.wantMessage)
				}
				if tt.noTransaction && strings.Contains(err.Error(), "rolled back") {
					t.Errorf("nontransactional error claims rollback: %v", err)
				}
			} else {
				if res == nil {
					t.Fatal("successful execution returned no result")
				}
				rows, err := res.RowsAffected()
				if err != nil || rows != 1 {
					t.Errorf("RowsAffected = %d, %v; want 1, nil", rows, err)
				}
			}
			if !reflect.DeepEqual(r.events, tt.wantEvents) {
				t.Errorf("database events = %v, want %v", r.events, tt.wantEvents)
			}
		})
	}
}

func TestProposalDryRunNoTransactionDoesNotTouchDatabase(t *testing.T) {
	r := &transactionRecorder{beginErr: errors.New("transactions unsupported")}
	dr := proposalDryRun(context.Background(), recordedDB(t, r), nil, "CREATE TABLE example (id BIGINT)", true)
	if len(r.events) != 0 {
		t.Fatalf("dry-run touched database: %v", r.events)
	}
	if dr == nil || !strings.Contains(strings.ToLower(dr.Explain), "skip") {
		t.Fatalf("dry-run must explain it was skipped: %+v", dr)
	}
}

func TestProposalDryRunRequiresReadOnlyTransaction(t *testing.T) {
	r := &transactionRecorder{beginErr: errors.New("transactions unsupported")}
	dr := proposalDryRun(context.Background(), recordedDB(t, r), nil, "CREATE TABLE example (id BIGINT)", false)
	if !reflect.DeepEqual(r.events, []string{"begin read-only"}) {
		t.Fatalf("dry-run bypassed read-only protection: %v", r.events)
	}
	if dr == nil || !strings.Contains(dr.Explain, "transactions unsupported") {
		t.Fatalf("dry-run must report transaction failure: %+v", dr)
	}
}

func TestReadCommandsRejectNoTransaction(t *testing.T) {
	for name, run := range map[string]func([]string) error{"query": Query, "explain": Explain} {
		t.Run(name, func(t *testing.T) {
			err := run([]string{"example", "--sql", "SELECT 1", "--no-transaction"})
			if err == nil || !strings.Contains(err.Error(), "flag provided but not defined: -no-transaction") {
				t.Fatalf("%s must reject transaction bypass: %v", name, err)
			}
		})
	}
}

func TestApplyStillRequiresRoot(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("cannot check non-root rejection as root")
	}
	err := Apply([]string{"12345678", "--yes"})
	if err == nil || !strings.Contains(err.Error(), "must run with sudo") {
		t.Fatalf("Apply error = %v, want sudo requirement", err)
	}
}
