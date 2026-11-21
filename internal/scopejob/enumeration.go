package scopejob

import (
	"context"
	"errors"
)

// Enumeration is a process-local metadata cursor, not a job or durable owner.
// Its caller must reserve work before OpenEnumeration and every Batch. It has
// no publication, ledger, job ID, restart, directory offset or resume operation.
type Enumeration struct{ cursor *cursor }

// EnumerationBatch is also the replay input used by the inventory owner. Source
// metadata is not source-content identity and grants no acquisition authority.
type EnumerationBatch struct {
	Records  []Record `json:"records"`
	Frontier []string `json:"frontier"`
	Done     bool     `json:"done"`
	Reason   string   `json:"reason"`
}

func OpenEnumeration(c Config) (*Enumeration, error) {
	cur, err := newCursor(c)
	if err != nil {
		return nil, err
	}
	return &Enumeration{cursor: cur}, nil
}

// HasGoRoot checks only paid, bounded root metadata; it reads no file contents.
func (e *Enumeration) HasGoRoot() bool {
	for _, name := range []string{"go.mod", "go.work"} {
		info, err := e.cursor.base.Lstat(name)
		if err == nil && info.Mode().IsRegular() {
			return true
		}
	}
	return false
}
func (e *Enumeration) Close() error {
	if e == nil || e.cursor == nil {
		return nil
	}
	return e.cursor.close()
}
func (e *Enumeration) Batch(ctx context.Context, steps uint64) EnumerationBatch {
	s := EnumerationBatch{Frontier: e.cursor.frontier()}
	if steps == 0 || steps > BatchSteps {
		s.Reason = "TRAVERSAL_UNAVAILABLE"
		return s
	}
	for i := uint64(0); i < steps; i++ {
		if ctx.Err() != nil {
			s.Reason = "INTERRUPTED"
			break
		}
		r, err := e.cursor.step()
		if r != nil {
			s.Records = append(s.Records, *r)
		}
		if err != nil {
			s.Reason = "TRAVERSAL_UNAVAILABLE"
			if errors.Is(err, ErrAssociationConflict) {
				s.Reason = "FILE_ASSOCIATION_CONFLICT"
			}
			if errors.Is(err, errEntries) {
				s.Reason = "ENTRY_LIMIT"
			}
			if errors.Is(err, errDirectories) {
				s.Reason = "DIRECTORY_LIMIT"
			}
			break
		}
		s.Frontier = e.cursor.frontier()
		if ctx.Err() != nil {
			s.Reason = "INTERRUPTED"
			break
		}
		s.Done = e.cursor.done()
		if s.Done {
			break
		}
	}
	return s
}

// EnumerationReplay uses exactly the legacy reader's policy and DFS transition.
// Its state is bounded by roots+depth, never by the number of scope records.
type EnumerationReplay struct {
	progress progress
	counts   Counts
}

func (p *EnumerationReplay) Apply(ctx context.Context, c Config, s EnumerationBatch, steps uint64) error {
	if ctx == nil || ctx.Err() != nil || steps == 0 || steps > BatchSteps || uint64(len(s.Records)) > steps || !validFrontier(c, s.Frontier) || !validSegmentReason(s.Reason) || s.Reason == "FILE_ASSOCIATION_CONFLICT" && c.Selection == nil || s.Done && (len(s.Frontier) != 0 || s.Reason != "") || !s.Done && len(s.Frontier) == 0 && s.Reason == "" {
		return ErrIntegrity
	}
	// One immutable filter preparation per replay batch; never trust a cached
	// selector across calls that can supply a different Config.
	var selection regularSelector
	for _, r := range s.Records {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if !validRecordSelected(c, r, &selection) {
			return ErrIntegrity
		}
		p.counts.add(r)
	}
	if p.counts.Entries > c.Budget.Entries || p.counts.Directories > c.Budget.Directories || !p.progress.segment(c, segment{Records: s.Records, Frontier: s.Frontier, Done: s.Done, Reason: s.Reason}, steps) {
		return ErrIntegrity
	}
	return ctx.Err()
}
func (p *EnumerationReplay) Counts() Counts { return p.counts }

// EnumerationWork retains the shared policy-v2 upper bound, including its
// historical per-step filter preparation allowance. Reusing prepared selection
// does not reprice old policies. The caller conserves its own cumulative ledger.
func EnumerationWork(c Config, steps uint64) (uint64, bool) {
	coefficient := uint64(8 + 2*c.Budget.Depth + len(c.Roots) + len(c.Prunes) + 2*(len(c.Includes)+len(c.Excludes)))
	if c.Selection != nil {
		coefficient += c.Selection.work(c.Budget.PathBytes)
	}
	if coefficient == 0 || steps > c.Budget.Work/coefficient {
		return 0, false
	}
	return steps * coefficient, true
}
func frameAvailable(c Config, n int) bool { return n < c.Budget.Depth }
