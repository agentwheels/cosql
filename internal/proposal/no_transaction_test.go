package proposal

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLegacyProposalDefaultsToTransaction(t *testing.T) {
	s := &Store{Dir: t.TempDir()}
	const id = "12345678"
	legacy := `{"id":"12345678","db":"example","driver":"mysql","sql":"CREATE TABLE example (id BIGINT)","status":"pending"}`
	if err := os.WriteFile(filepath.Join(s.Dir, id+".json"), []byte(legacy), 0600); err != nil {
		t.Fatal(err)
	}
	p, err := s.Get(id)
	if err != nil {
		t.Fatal(err)
	}
	if p.NoTransaction {
		t.Fatal("legacy proposal must retain transaction protection")
	}
}

func TestLoadProposalWithNoTransactionMode(t *testing.T) {
	s := &Store{Dir: t.TempDir()}
	const id = "12345678"
	data := `{"id":"12345678","db":"example","driver":"mysql","sql":"CREATE TABLE example (id BIGINT)","status":"pending","no_transaction":true}`
	if err := os.WriteFile(filepath.Join(s.Dir, id+".json"), []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	p, err := s.Get(id)
	if err != nil {
		t.Fatal(err)
	}
	if !p.NoTransaction {
		t.Fatal("stored no_transaction setting must be honored")
	}
}

func TestProposalTransactionModeSurvivesStore(t *testing.T) {
	for _, noTransaction := range []bool{false, true} {
		name := "transaction"
		if noTransaction {
			name = "no transaction"
		}
		t.Run(name, func(t *testing.T) {
			s := &Store{Dir: t.TempDir()}
			id, err := s.New(Proposal{DB: "example", Driver: "mysql", SQL: "CREATE TABLE example (id BIGINT)", NoTransaction: noTransaction})
			if err != nil {
				t.Fatal(err)
			}
			p, err := s.Get(id)
			if err != nil {
				t.Fatal(err)
			}
			if p.NoTransaction != noTransaction {
				t.Fatalf("loaded NoTransaction = %v, want %v", p.NoTransaction, noTransaction)
			}
			p.Result = &Result{AffectedRows: 1}
			if err := s.Update(p); err != nil {
				t.Fatal(err)
			}
			p, err = s.Get(id)
			if err != nil {
				t.Fatal(err)
			}
			if p.NoTransaction != noTransaction {
				t.Fatalf("updated NoTransaction = %v, want %v", p.NoTransaction, noTransaction)
			}
		})
	}
}
