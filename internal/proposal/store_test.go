package proposal

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func tempStore(t *testing.T) *Store {
	t.Helper()
	return &Store{Dir: t.TempDir()}
}

func TestNewWritesFile(t *testing.T) {
	s := tempStore(t)
	id, err := s.New(Proposal{DB: "x", Driver: "postgres", SQL: "UPDATE t SET x=1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(id) != 8 {
		t.Fatalf("id length: %d", len(id))
	}
	if _, err := hex.DecodeString(id); err != nil {
		t.Fatalf("id not hex: %v", err)
	}
	info, err := os.Stat(filepath.Join(s.Dir, id+".json"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("file mode: %o", info.Mode().Perm())
	}
	p, err := s.Get(id)
	if err != nil {
		t.Fatal(err)
	}
	if p.Status != StatusPending {
		t.Fatalf("status: %s", p.Status)
	}
	if p.CreatedAt == "" {
		t.Fatal("created_at missing")
	}
}

func TestGetExpiresOldPending(t *testing.T) {
	s := tempStore(t)
	id, _ := s.New(Proposal{DB: "x", Driver: "postgres", SQL: "X"})
	p, _ := s.Get(id)
	p.CreatedAt = time.Now().Add(-8 * 24 * time.Hour).Format(time.RFC3339)
	if err := s.Update(p); err != nil {
		t.Fatal(err)
	}
	p2, err := s.Get(id)
	if err != nil {
		t.Fatal(err)
	}
	if p2.Status != StatusExpired {
		t.Fatalf("want expired, got %s", p2.Status)
	}
}

func TestListFilter(t *testing.T) {
	s := tempStore(t)
	id1, _ := s.New(Proposal{DB: "x", Driver: "postgres", SQL: "A"})
	id2, _ := s.New(Proposal{DB: "y", Driver: "mysql", SQL: "B"})
	p2, _ := s.Get(id2)
	p2.Status = StatusApplied
	if err := s.Update(p2); err != nil {
		t.Fatal(err)
	}

	all, err := s.List("")
	if err != nil || len(all) != 2 {
		t.Fatalf("all: %d %v", len(all), err)
	}
	pending, _ := s.List(StatusPending)
	if len(pending) != 1 || pending[0].ID != id1 {
		t.Fatalf("pending: %+v", pending)
	}
	applied, _ := s.List(StatusApplied)
	if len(applied) != 1 || applied[0].ID != id2 {
		t.Fatalf("applied: %+v", applied)
	}
}

func TestGetInvalidID(t *testing.T) {
	s := tempStore(t)
	if _, err := s.Get("bad"); err == nil {
		t.Fatal("expected invalid id error (length)")
	}
	if _, err := s.Get("zzzzzzzz"); err == nil {
		t.Fatal("expected invalid id error (hex)")
	}
}

func TestUpdateRoundTrip(t *testing.T) {
	s := tempStore(t)
	id, _ := s.New(Proposal{DB: "x", Driver: "postgres", SQL: "UPDATE t SET x=1", Note: "why"})
	p, _ := s.Get(id)
	p.Status = StatusApplied
	p.Result = &Result{AffectedRows: 3, DurationMS: 42}
	if err := s.Update(p); err != nil {
		t.Fatal(err)
	}
	p2, _ := s.Get(id)
	if p2.Status != StatusApplied || p2.Result == nil || p2.Result.AffectedRows != 3 {
		t.Fatalf("round trip lost data: %+v", p2)
	}
}
