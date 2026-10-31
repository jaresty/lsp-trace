//go:build darwin || linux

package censuslineagepilot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"syscall"

	"lsp-trace/internal/publication"
)

// A separate execution-root journal accounts resource reservations, NOT source
// authority. Reopening it never restores a Census, source qualification or lease.
// All records are create-only and fsynced before the reserved work starts. A
// torn record is an accountable refusal, never permission to reset spending.
// Same-user malicious edits/rollback are outside this unauthenticated contract.
type PageLedgerPrivateV3 struct{ state *pageLedgerStatePrivateV3 }
type pageLedgerStatePrivateV3 struct {
	self                    *pageLedgerStatePrivateV3
	mu                      sync.Mutex
	root                    *os.Root
	observed                *publication.ObservedRoot
	path, application, last string
	allowance, spent        PageCostPrivateV3
	generation              uint64
	closed                  bool
}
type pageLedgerRecordPrivateV3 struct {
	Schema      string            `json:"schema"`
	Application string            `json:"application"`
	Generation  uint64            `json:"generation"`
	Previous    string            `json:"previous_sha256"`
	Allowance   PageCostPrivateV3 `json:"application_allowance"`
	Reserved    PageCostPrivateV3 `json:"reserved"`
	Cumulative  PageCostPrivateV3 `json:"cumulative"`
}

const pageLedgerSchemaPrivateV3 = "lsp-trace.resource-reservations.private.v3"

var pageLedgerOverheadPrivateV3 = PageCostPrivateV3{4 * producerPageHeaderPrivateV3, 4 * producerPageHeaderPrivateV3, producerPageHeaderPrivateV3, 32}

func (PageLedgerPrivateV3) MarshalJSON() ([]byte, error) { return nil, ErrInvalid }
func ledgerNamePrivateV3(generation uint64) string {
	return fmt.Sprintf("reservation-%020d.json", generation)
}

// CreatePageLedgerPrivateV3 requires an empty, separate 0700 execution directory.
// There is no default application allowance and no automatic allowance extension.
func CreatePageLedgerPrivateV3(ctx context.Context, path, application string, allowance PageCostPrivateV3) (PageLedgerPrivateV3, error) {
	l, e := openPageLedgerRootPrivateV3(ctx, path, application, allowance)
	if e != nil {
		return PageLedgerPrivateV3{}, e
	}
	s := l.state
	dir, e := s.root.Open(".")
	if e != nil {
		l.Close()
		return PageLedgerPrivateV3{}, e
	}
	entries, readErr := dir.ReadDir(1)
	closeErr := dir.Close()
	if len(entries) != 0 || readErr != io.EOF || closeErr != nil {
		l.Close()
		return PageLedgerPrivateV3{}, ErrInvalid
	}
	r := pageLedgerRecordPrivateV3{Schema: pageLedgerSchemaPrivateV3, Application: application, Allowance: allowance, Reserved: pageLedgerOverheadPrivateV3, Cumulative: pageLedgerOverheadPrivateV3}
	if !r.Cumulative.fits(allowance) {
		l.Close()
		return PageLedgerPrivateV3{}, ErrPagesPausedPrivateV3
	}
	raw, _ := json.Marshal(r)
	if e = s.createRecord(0, raw); e != nil {
		l.Close()
		return PageLedgerPrivateV3{}, e
	}
	s.spent = r.Cumulative
	s.last = Digest(raw)
	return l, nil
}
func openPageLedgerRootPrivateV3(ctx context.Context, path, application string, allowance PageCostPrivateV3) (PageLedgerPrivateV3, error) {
	if ctx == nil || ctx.Err() != nil || !filepath.IsAbs(path) || filepath.Clean(path) != path || !validPageDigestPrivateV3(application) {
		return PageLedgerPrivateV3{}, ErrInvalid
	}
	real, e := filepath.EvalSymlinks(path)
	if e != nil || real != path {
		return PageLedgerPrivateV3{}, ErrInvalid
	}
	root, e := os.OpenRoot(path)
	if e != nil {
		return PageLedgerPrivateV3{}, e
	}
	info, e := root.Stat(".")
	if e != nil || !info.IsDir() || info.Mode().Perm() != 0700 {
		root.Close()
		return PageLedgerPrivateV3{}, ErrInvalid
	}
	observed, e := publication.OpenObservedRoot(ctx, path)
	if e != nil {
		root.Close()
		return PageLedgerPrivateV3{}, e
	}
	s := &pageLedgerStatePrivateV3{root: root, observed: observed, path: path, application: application, allowance: allowance}
	s.self = s
	return PageLedgerPrivateV3{s}, nil
}
func (s *pageLedgerStatePrivateV3) createRecord(g uint64, raw []byte) (err error) {
	if len(raw) > producerPageHeaderPrivateV3 {
		return ErrInvalid
	}
	f, e := s.root.OpenFile(ledgerNamePrivateV3(g), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		return e
	}
	// A crash/error after create burns the generation. Never unlink or retry it.
	defer func() { err = errors.Join(err, f.Close()) }()
	if _, err = f.Write(raw); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	dir, e := s.root.Open(".")
	if e != nil {
		return e
	}
	return errors.Join(dir.Sync(), dir.Close())
}
func readLedgerRecordPrivateV3(root *os.Root, g uint64) (pageLedgerRecordPrivateV3, []byte, error) {
	var r pageLedgerRecordPrivateV3
	f, e := root.OpenFile(ledgerNamePrivateV3(g), os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if e != nil {
		return r, nil, e
	}
	info, e := f.Stat()
	if e != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 || info.Size() > producerPageHeaderPrivateV3 {
		f.Close()
		return r, nil, ErrInvalid
	}
	raw, e := io.ReadAll(io.LimitReader(f, producerPageHeaderPrivateV3+1))
	e = errors.Join(e, f.Close())
	if e != nil {
		return r, nil, e
	}
	if e = producerPacketStrictPrivateV2(raw, &r); e != nil {
		return r, nil, e
	}
	return r, raw, nil
}

// OpenPageLedgerPrivateV3 uses an explicit tip, never an unbounded directory scan.
// The supplied digest binds the caller's previously recorded tip. Immediate-next
// existence detects stale/spent tips (including crash-created incomplete files).
// This is same-user durable accounting, not authentication or rollback defense.
func OpenPageLedgerPrivateV3(ctx context.Context, path, application string, allowance PageCostPrivateV3, generation uint64, tip string) (PageLedgerPrivateV3, error) {
	l, e := openPageLedgerRootPrivateV3(ctx, path, application, allowance)
	if e != nil {
		return PageLedgerPrivateV3{}, e
	}
	s := l.state
	r, raw, e := readLedgerRecordPrivateV3(s.root, generation)
	if e != nil || Digest(raw) != tip || r.Schema != pageLedgerSchemaPrivateV3 || r.Application != application || r.Allowance != allowance || r.Generation != generation || !r.Cumulative.fits(allowance) || !r.Reserved.fits(r.Cumulative) {
		l.Close()
		return PageLedgerPrivateV3{}, ErrInvalid
	}
	if _, e = s.root.Lstat(ledgerNamePrivateV3(generation + 1)); !os.IsNotExist(e) {
		l.Close()
		return PageLedgerPrivateV3{}, ErrInvalid
	}
	s.generation = generation
	s.spent = r.Cumulative
	s.last = tip
	// Account reopening before using the returned ledger. It creates a fresh
	// reservation generation; an old tip cannot reopen it a second time.
	left := allowance
	if e = s.reserve(pageLedgerOverheadPrivateV3, &left); e != nil {
		l.Close()
		return PageLedgerPrivateV3{}, e
	}
	return l, nil
}
func (s *pageLedgerStatePrivateV3) reserve(cost PageCostPrivateV3, left *PageCostPrivateV3) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.self != s || s.closed || s.generation == ^uint64(0) {
		return ErrInvalid
	}
	charged, e := cost.plus(pageLedgerOverheadPrivateV3)
	if e != nil {
		return e
	}
	total, e := s.spent.plus(charged)
	if e != nil {
		return e
	}
	if !total.fits(s.allowance) || !charged.fits(*left) {
		return ErrPagesPausedPrivateV3
	}
	// Advisory file lock plus O_EXCL serialize independent reopened handles. The
	// latest generation check prevents a competing handle from resetting spend.
	lock, e := s.root.OpenFile(ledgerNamePrivateV3(s.generation), os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if e != nil {
		return e
	}
	defer lock.Close()
	if e = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); e != nil {
		return e
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	r := pageLedgerRecordPrivateV3{pageLedgerSchemaPrivateV3, s.application, s.generation + 1, s.last, s.allowance, charged, total}
	raw, _ := json.Marshal(r)
	if e = s.createRecord(s.generation+1, raw); e != nil {
		return e
	}
	s.generation++
	s.spent = total
	s.last = Digest(raw)
	*left = left.minus(charged)
	return nil
}
func (l PageLedgerPrivateV3) Tip() (uint64, string, PageCostPrivateV3) {
	if l.state == nil {
		return 0, "", PageCostPrivateV3{}
	}
	s := l.state
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.generation, s.last, s.spent
}
func (l PageLedgerPrivateV3) Close() error {
	if l.state == nil {
		return ErrInvalid
	}
	s := l.state
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrInvalid
	}
	s.closed = true
	return errors.Join(s.observed.Close(), s.root.Close())
}
