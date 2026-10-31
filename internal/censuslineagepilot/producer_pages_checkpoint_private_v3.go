//go:build darwin || linux

package censuslineagepilot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"syscall"
)

type pageAttemptPrivateV3 struct {
	Schema           string              `json:"schema"`
	Binding          string              `json:"binding_sha256"`
	Generation       uint64              `json:"generation"`
	PreviousRevision string              `json:"previous_revision_sha256"`
	Cursor           PageCursorPrivateV3 `json:"starting_cursor"`
	LedgerGeneration uint64              `json:"ledger_generation"`
	LedgerSHA256     string              `json:"ledger_sha256"`
}
type pageCheckpointPrivateV3 struct {
	Schema     string `json:"schema"`
	Binding    string `json:"binding_sha256"`
	Generation uint64 `json:"generation"`
	Revision   string `json:"revision_sha256"`
}

func pageJournalNamePrivateV3(kind, binding string, g uint64) string {
	return fmt.Sprintf("%s-%s-%020d.json", kind, binding[7:], g)
}

var pageJournalCostPrivateV3 = PageCostPrivateV3{LogicalBytes: 8 * producerPageHeaderPrivateV3, ReadHashBytes: 8 * producerPageHeaderPrivateV3, WriteBytes: 2 * producerPageHeaderPrivateV3, Work: 64}

func (s *pageLedgerStatePrivateV3) createNamed(name string, raw []byte) (err error) {
	if len(raw) > producerPageHeaderPrivateV3 {
		return ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrInvalid
	}
	f, e := s.root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		return e
	}
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
func (s *pageLedgerStatePrivateV3) readNamed(name string, v any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrInvalid
	}
	f, e := s.root.OpenFile(name, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if e != nil {
		return e
	}
	info, e := f.Stat()
	if e != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 || info.Size() > producerPageHeaderPrivateV3 {
		f.Close()
		return ErrInvalid
	}
	raw, e := io.ReadAll(io.LimitReader(f, producerPageHeaderPrivateV3+1))
	e = errors.Join(e, f.Close())
	if e != nil {
		return e
	}
	return producerPacketStrictPrivateV2(raw, v)
}
func (s *producerPagesStatePrivateV3) claimAttempt(g uint64, previous string) error {
	lg, tip, _ := s.ledger.Tip()
	a := pageAttemptPrivateV3{ProducerPagesSchemaPrivateV3, s.binding, g, previous, s.cursor, lg, tip}
	raw, _ := json.Marshal(a)
	return s.ledger.state.createNamed(pageJournalNamePrivateV3("attempt", s.binding, g), raw)
}
func (s *producerPagesStatePrivateV3) checkpoint(id string) error {
	c := pageCheckpointPrivateV3{ProducerPagesSchemaPrivateV3, s.binding, s.generation, id}
	raw, _ := json.Marshal(c)
	return s.ledger.state.createNamed(pageJournalNamePrivateV3("checkpoint", s.binding, s.generation), raw)
}

// ResumeOriginalCheckpoint admits NO decoded producer owner. First construct a
// fresh paged owner from the original opaque f/o/c capabilities and same ledger.
// Then this operation freshly reads and verifies the checkpoint's paired pages
// against those ORIGINAL bytes. It does not acquire, call a Manager, reuse a
// consumed guard, or assert that serialized history is authentic. A new process
// without original opaque owners cannot use this API; that remains an explicit
// unsupported boundary pending a separately reviewed checkpoint contract.
//
// A started generation is permanently spent, even if there was no completion
// checkpoint. On crash, only the PREVIOUS admitted revision is resumed. Unknown
// uncheckpointed bytes remain retained but are not silently added to coverage.
func (p ProducerPagesPrivateV3) ResumeOriginalCheckpoint(ctx context.Context, generation uint64, exchange PageCostPrivateV3) (PageRevisionPrivateV3, error) {
	if p.state == nil {
		return PageRevisionPrivateV3{}, ErrInvalid
	}
	s := p.state
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.valid() || s.generation != 0 || ctx == nil || ctx.Err() != nil || generation == 0 || generation == ^uint64(0) {
		return PageRevisionPrivateV3{}, ErrInvalid
	}
	cost, _ := pageJournalCostPrivateV3.plus(PageCostPrivateV3{LogicalBytes: 2 * producerPageHeaderPrivateV3, ReadHashBytes: 8 * producerPageHeaderPrivateV3, Work: 32})
	if e := s.reserve(cost, &exchange); e != nil {
		return PageRevisionPrivateV3{}, e
	}
	ledger := s.ledger.state
	// No scan or mutable head: the host supplies the last started generation and
	// immediate-next existence refuses stale checkpoints and competing resumptions.
	if _, e := ledger.root.Lstat(pageJournalNamePrivateV3("attempt", s.binding, generation+1)); !os.IsNotExist(e) {
		return PageRevisionPrivateV3{}, ErrInvalid
	}
	var a pageAttemptPrivateV3
	if e := ledger.readNamed(pageJournalNamePrivateV3("attempt", s.binding, generation), &a); e != nil {
		return PageRevisionPrivateV3{}, e
	}
	if a.Schema != ProducerPagesSchemaPrivateV3 || a.Binding != s.binding || a.Generation != generation {
		return PageRevisionPrivateV3{}, ErrInvalid
	}
	var checkpoint pageCheckpointPrivateV3
	e := ledger.readNamed(pageJournalNamePrivateV3("checkpoint", s.binding, generation), &checkpoint)
	id := a.PreviousRevision
	if e == nil {
		if checkpoint.Schema != ProducerPagesSchemaPrivateV3 || checkpoint.Binding != s.binding || checkpoint.Generation != generation || !validPageDigestPrivateV3(checkpoint.Revision) {
			return PageRevisionPrivateV3{}, ErrInvalid
		}
		id = checkpoint.Revision
	} else if !os.IsNotExist(e) {
		return PageRevisionPrivateV3{}, e
	}
	if id == "" {
		if a.Cursor != (PageCursorPrivateV3{}) {
			return PageRevisionPrivateV3{}, ErrInvalid
		}
		s.generation = generation
		return s.revision("FAILED", ""), nil
	}
	if !validPageDigestPrivateV3(id) {
		return PageRevisionPrivateV3{}, ErrInvalid
	}
	var revision PageRevisionPrivateV3
	var raw []byte
	for _, root := range s.roots.state.outputs {
		b, e := s.ops.read(ctx, root, id, producerPageHeaderPrivateV3)
		if e != nil {
			return revision, e
		}
		if Digest(b) != id || raw != nil && string(raw) != string(b) {
			return revision, ErrInvalid
		}
		raw = b
	}
	if e := producerPacketStrictPrivateV2(raw, &revision); e != nil {
		return revision, e
	}
	if checkpoint.Revision != "" && revision.Generation != generation || checkpoint.Revision == "" && (revision.Generation >= generation || revision.Cursor != a.Cursor) {
		return revision, ErrInvalid
	}
	if e := s.verify(ctx, revision, &exchange, nil, nil); e != nil {
		return revision, e
	}
	s.cursor = revision.Cursor
	s.lastRevision = id
	s.generation = generation
	return revision, nil
}
