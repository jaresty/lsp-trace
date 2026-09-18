// Package retainedlifecycle records exact retained-object lifecycle outcomes and
// manages explicit Git reachability leases. It does not resolve source bytes,
// infer causes from resolver errors, or provide deletion/collection operations.
package retainedlifecycle

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type State string

const (
	StateUnknown          State = "UNKNOWN"
	StateMissing          State = "MISSING"
	StateCorrupt          State = "CORRUPT"
	StateWithheld         State = "WITHHELD"
	StateCollected        State = "COLLECTED"
	StateShallowHistory   State = "SHALLOW_HISTORY"
	StateRewrittenHistory State = "REWRITTEN_HISTORY"
	StateGCCollected      State = "GC_COLLECTED"
	StateDeleted          State = "DELETED"
	// StateLimit is deliberately outside the lifecycle vocabulary. It exists so
	// callers can prove that budget exhaustion cannot be recorded as lifecycle.
	StateLimit State = "LIMIT"
)

type Key struct {
	ManifestDigest string
	EntryID        string
}

type Classifier interface {
	State(Key) State
}

type Ledger struct {
	mu     sync.RWMutex
	states map[Key]State
}

func NewLedger() *Ledger { return &Ledger{states: make(map[Key]State)} }

func (l *Ledger) Record(key Key, state State) error {
	if l == nil || !canonicalDigest(key.ManifestDigest) || !canonicalDigest(key.EntryID) || !terminal(state) {
		return errors.New("exact lifecycle key and terminal state required")
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.states[key] = state
	return nil
}

func (l *Ledger) State(key Key) State {
	if l == nil {
		return StateUnknown
	}
	l.mu.RLock()
	defer l.mu.RUnlock()
	if state, ok := l.states[key]; ok {
		return state
	}
	return StateUnknown
}

func terminal(state State) bool {
	switch state {
	case StateMissing, StateCorrupt, StateWithheld, StateCollected, StateShallowHistory, StateRewrittenHistory, StateGCCollected, StateDeleted:
		return true
	default:
		return false
	}
}

func canonicalDigest(value string) bool {
	if len(value) != 71 || !strings.HasPrefix(value, "sha256:") || strings.ToLower(value) != value {
		return false
	}
	_, err := hex.DecodeString(value[7:])
	return err == nil
}

type Lease struct {
	id      string
	ref     string
	commit  string
	expires time.Time
}

type GitLeaseManager struct {
	mu     sync.Mutex
	root   string
	now    func() time.Time
	leases map[string]Lease
}

func NewGitLeaseManager(root string, now func() time.Time) (*GitLeaseManager, error) {
	if !filepath.IsAbs(root) || now == nil {
		return nil, errors.New("absolute Git root and clock required")
	}
	clean := filepath.Clean(root)
	info, err := os.Stat(clean)
	if err != nil || !info.IsDir() {
		return nil, errors.New("Git lease root must be an existing directory")
	}
	manager := &GitLeaseManager{root: clean, now: now, leases: make(map[string]Lease)}
	if err := manager.git("rev-parse", "--git-dir"); err != nil {
		return nil, errors.New("Git lease root must be a repository")
	}
	return manager, nil
}

func (m *GitLeaseManager) Acquire(id, commit string, expires time.Time) (Lease, error) {
	if m == nil || id == "" || !validOID(commit) || !expires.After(m.now()) {
		return Lease{}, errors.New("exact future Git lease required")
	}
	sum := sha256.Sum256([]byte(id))
	ref := "refs/lsp-trace/leases/" + hex.EncodeToString(sum[:])
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.leases[id]; exists {
		return Lease{}, errors.New("Git lease already exists")
	}
	if err := m.git("cat-file", "-e", commit+"^{commit}"); err != nil {
		return Lease{}, errors.New("leased Git commit unavailable")
	}
	if err := m.git("update-ref", ref, commit, strings.Repeat("0", len(commit))); err != nil {
		return Lease{}, errors.New("Git lease publication failed")
	}
	lease := Lease{id: id, ref: ref, commit: commit, expires: expires.UTC()}
	m.leases[id] = lease
	return lease, nil
}

func (m *GitLeaseManager) Release(lease Lease) error {
	if m == nil || lease.id == "" || lease.ref == "" || !validOID(lease.commit) {
		return errors.New("exact Git lease required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	current, ok := m.leases[lease.id]
	if !ok || current != lease {
		return errors.New("Git lease identity mismatch")
	}
	if err := m.git("update-ref", "-d", lease.ref, lease.commit); err != nil {
		return errors.New("Git lease release failed")
	}
	delete(m.leases, lease.id)
	return nil
}

func (m *GitLeaseManager) SweepExpired() (int, error) {
	if m == nil {
		return 0, errors.New("Git lease manager required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	removed := 0
	for id, lease := range m.leases {
		if now.Before(lease.expires) {
			continue
		}
		if err := m.git("update-ref", "-d", lease.ref, lease.commit); err != nil {
			return removed, errors.New("expired Git lease release failed")
		}
		delete(m.leases, id)
		removed++
	}
	return removed, nil
}

func (m *GitLeaseManager) git(args ...string) error {
	cmd := exec.Command("git", append([]string{"-C", m.root}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_OPTIONAL_LOCKS=0", "GIT_TERMINAL_PROMPT=0")
	return cmd.Run()
}

func validOID(value string) bool {
	if len(value) != 40 && len(value) != 64 || strings.ToLower(value) != value {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
