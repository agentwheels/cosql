// Package proposal persists write proposals on disk. Agents create them;
// humans apply them via `sudo cosql apply <id>`.
package proposal

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Status is the proposal lifecycle state.
type Status string

const (
	StatusPending  Status = "pending"
	StatusApplied  Status = "applied"
	StatusRejected Status = "rejected"
	StatusExpired  Status = "expired"

	// DefaultTTL is the age after which a pending proposal is considered expired.
	DefaultTTL = 7 * 24 * time.Hour
)

// Proposal is what gets written to disk as <id>.json.
type Proposal struct {
	ID        string  `json:"id"`
	DB        string  `json:"db"`
	Driver    string  `json:"driver"`
	SQL       string  `json:"sql"`
	Note      string  `json:"note,omitempty"`
	CreatedAt string  `json:"created_at"`
	CreatedBy string  `json:"created_by"`
	DryRun    *DryRun `json:"dry_run,omitempty"`
	Status    Status  `json:"status"`
	AppliedAt string  `json:"applied_at,omitempty"`
	AppliedBy string  `json:"applied_by,omitempty"`
	Result    *Result `json:"result,omitempty"`
}

type DryRun struct {
	Explain      string `json:"explain,omitempty"`
	AffectedEst  int64  `json:"affected_est,omitempty"`
}

type Result struct {
	AffectedRows int64  `json:"affected_rows"`
	DurationMS   int64  `json:"duration_ms"`
	Error        string `json:"error,omitempty"`
}

// Store is the filesystem-backed proposal store.
type Store struct{ Dir string }

// DefaultDir returns ~/.local/share/cosql/proposals (or $XDG_DATA_HOME).
func DefaultDir() (string, error) {
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, "cosql", "proposals"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", "cosql", "proposals"), nil
}

// Open returns a Store rooted at DefaultDir, creating the directory on demand.
func Open() (*Store, error) {
	d, err := DefaultDir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(d, 0o700); err != nil {
		return nil, err
	}
	return &Store{Dir: d}, nil
}

// New creates a new pending proposal and writes it to disk. Returns the id.
func (s *Store) New(p Proposal) (string, error) {
	id, err := randID()
	if err != nil {
		return "", err
	}
	p.ID = id
	p.CreatedAt = time.Now().Format(time.RFC3339)
	if p.CreatedBy == "" {
		p.CreatedBy = currentUser()
	}
	p.Status = StatusPending
	return id, s.save(&p)
}

// Get returns the proposal with the given id (exact match).
func (s *Store) Get(id string) (*Proposal, error) {
	if err := validID(id); err != nil {
		return nil, err
	}
	path := filepath.Join(s.Dir, id+".json")
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("proposal %s not found", id)
		}
		return nil, err
	}
	var p Proposal
	if err := json.Unmarshal(b, &p); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if p.Status == StatusPending && isExpired(p.CreatedAt) {
		p.Status = StatusExpired
	}
	return &p, nil
}

// Update writes the given proposal back to disk.
func (s *Store) Update(p *Proposal) error { return s.save(p) }

// List returns all proposals, optionally filtered by status.
func (s *Store) List(filter Status) ([]*Proposal, error) {
	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		return nil, err
	}
	var out []*Proposal
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ".json")
		p, err := s.Get(id)
		if err != nil {
			continue
		}
		if filter != "" && p.Status != filter {
			continue
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt > out[j].CreatedAt
	})
	return out, nil
}

func (s *Store) save(p *Proposal) error {
	path := filepath.Join(s.Dir, p.ID+".json")
	b, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o600)
}

func randID() (string, error) {
	var buf [4]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf[:]), nil
}

func validID(id string) error {
	if len(id) != 8 {
		return fmt.Errorf("invalid proposal id %q (expected 8 hex chars)", id)
	}
	if _, err := hex.DecodeString(id); err != nil {
		return fmt.Errorf("invalid proposal id %q: %w", id, err)
	}
	return nil
}

func currentUser() string {
	if u, err := user.Current(); err == nil {
		if sudoUser := os.Getenv("SUDO_USER"); sudoUser != "" {
			return sudoUser
		}
		return u.Username
	}
	return "unknown"
}

func isExpired(created string) bool {
	t, err := time.Parse(time.RFC3339, created)
	if err != nil {
		return false
	}
	return time.Since(t) > DefaultTTL
}
