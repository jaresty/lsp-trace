//go:build darwin || linux

package inventorysuccessor

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/discoveryfilter"
	"lsp-trace/internal/publication"
	"lsp-trace/internal/source"
)

var ErrPrivatePreparationEnumerationV1 = errors.New("private preparation enumeration refused")

// PrivatePreparationIntentV1 authorizes metadata observation, not a future
// content revision, Git assertion, native target, HOST call or execution grant.
type PrivatePreparationIntentV1 struct {
	Schema, SourceRoot, Attempt                  string
	Roots, Includes, Excludes, PrunedDirectories []string
	Limits                                       source.Limits
	Deadline                                     time.Time
}

func preparationIntentValidV1(i PrivatePreparationIntentV1) bool {
	if !hashV2(i.Attempt) || i.Schema != "private-preparation-intent.v1" || !filepath.IsAbs(i.SourceRoot) || filepath.Clean(i.SourceRoot) != i.SourceRoot || i.SourceRoot == string(filepath.Separator) || len(i.SourceRoot) > 4096 || !utf8.ValidString(i.SourceRoot) || strings.ContainsRune(i.SourceRoot, 0) || i.Deadline.IsZero() {
		return false
	}
	l := i.Limits
	if l.MaxEntries < 1 || l.MaxEntries > 1000000 || l.MaxAccepted < 1 || l.MaxAccepted > l.MaxEntries || l.MaxWork < 1 || l.MaxWork > 1<<26 || l.MaxPathBytes < 1 || l.MaxPathBytes > 4096 || l.MaxDepth < 1 || l.MaxDepth > 64 || len(i.Roots) == 0 {
		return false
	}
	size := len(i.SourceRoot) + 256
	for _, xs := range [][]string{i.Roots, i.Includes, i.Excludes, i.PrunedDirectories} {
		if len(xs) > 64 {
			return false
		}
		previous := ""
		for _, s := range xs {
			size += len(s) + 16
			if s == "" || len(s) > 4096 || !utf8.ValidString(s) || strings.ContainsAny(s, "\x00\r\n") || s <= previous || size > 16<<10 {
				return false
			}
			previous = s
		}
	}
	for _, s := range append(append([]string{}, i.Roots...), i.PrunedDirectories...) {
		if filepath.IsAbs(s) || filepath.Clean(s) != s || s == ".." || strings.HasPrefix(s, ".."+string(filepath.Separator)) {
			return false
		}
	}
	return discoveryfilter.ValidatePatterns(i.Includes, i.Excludes) == nil
}
func preparationIntentCopyV1(i PrivatePreparationIntentV1) PrivatePreparationIntentV1 {
	i.Roots = append([]string(nil), i.Roots...)
	i.Includes = append([]string(nil), i.Includes...)
	i.Excludes = append([]string(nil), i.Excludes...)
	i.PrunedDirectories = append([]string(nil), i.PrunedDirectories...)
	return i
}
func PrivatePreparationIntentIdentityV1(i PrivatePreparationIntentV1) string {
	if !preparationIntentValidV1(i) {
		return ""
	}
	i = preparationIntentCopyV1(i)
	i.Deadline = i.Deadline.UTC()
	raw, err := json.Marshal(i)
	if err != nil || len(raw) > 32<<10 {
		return ""
	}
	return hashLabelV2("private-preparation-intent.v1/" + string(raw))
}
func PrivatePreparationEnumerationScopeV1() string {
	return hashLabelV2("private-preparation-enumeration.v1/ENUMERATE")
}
func PrivatePreparationEnumerationRulesV1(i PrivatePreparationIntentV1) string {
	return hashLabelV2("private-preparation-enumeration.v1/" + PrivatePreparationIntentIdentityV1(i))
}

type PrivatePreparationEnumeratorV1 struct{ state *preparationEnumeratorV1 }
type preparationEnumeratorV1 struct {
	mu           sync.Mutex
	k            *kernelV2
	job          jobV2
	head         string
	verification verificationLimitsV2
	used         bool
}

// Open only reopens a caller-authorized READY chain. It never initializes one.
func OpenPrivatePreparationEnumeratorV1(ctx context.Context, root *publication.Root, raw []byte, expected string, v PrivateManagedInputVerificationV2) (PrivatePreparationEnumeratorV1, error) {
	var j jobV2
	if ctx == nil || ctx.Err() != nil || root == nil || !hashV2(expected) || parseV2(raw, &j) != nil || j.Scope != PrivatePreparationEnumerationScopeV1() {
		return PrivatePreparationEnumeratorV1{}, ErrPrivatePreparationEnumerationV1
	}
	k := &kernelV2{root: root}
	limits := verificationLimitsV2{v.Records, v.Bytes, v.Work}
	h, e := k.reopen(ctx, j, limits)
	if e != nil || h.Identity != expected || !readyV2(h) {
		return PrivatePreparationEnumeratorV1{}, ErrPrivatePreparationEnumerationV1
	}
	return PrivatePreparationEnumeratorV1{&preparationEnumeratorV1{k: k, job: j, head: expected, verification: limits}}, nil
}

// Result is data-only; copying it cannot restore an owner or operation.
type PrivatePreparationEnumerationResultV1 struct {
	state *preparationEnumerationResultV1
}
type preparationEnumerationResultV1 struct {
	intent, reservation, completion string
	files                           []censusacquisition.SourceFile
}
type PrivatePreparationEnumerationFactsV1 struct {
	Intent, ReservationHead, CompletionHead, Assurance, Completeness string
	Authority                                                        int
	Accepted                                                         bool
}

func (r PrivatePreparationEnumerationResultV1) Available() bool { return r.state != nil }
func (r PrivatePreparationEnumerationResultV1) Files() []censusacquisition.SourceFile {
	if r.state == nil {
		return nil
	}
	return append([]censusacquisition.SourceFile(nil), r.state.files...)
}
func (r PrivatePreparationEnumerationResultV1) Facts() PrivatePreparationEnumerationFactsV1 {
	if r.state == nil {
		return PrivatePreparationEnumerationFactsV1{}
	}
	return PrivatePreparationEnumerationFactsV1{Intent: r.state.intent, ReservationHead: r.state.reservation, CompletionHead: r.state.completion, Assurance: assuranceV2, Completeness: "UNKNOWN"}
}

// PrivatePreparationEnumerationQuoteV1 is a logical upper reservation, not a
// permit. Filesystem walking remains cooperative/context-bounded by source.
// No provider, content read/hash, Git, or native consumer is called here.
func PrivatePreparationEnumerationQuoteV1(i PrivatePreparationIntentV1, v PrivateManagedInputVerificationV2, now time.Time) ([14]uint64, bool) {
	if !preparationIntentValidV1(i) || !now.Before(i.Deadline) || i.Deadline.Sub(now) > 15*time.Second || v.Records < 1 || v.Records > 128 || v.Bytes < 1 || v.Bytes > 4<<20 || v.Work < 1 || v.Work > 128 {
		return [14]uint64{}, false
	}
	n, p, w := uint64(i.Limits.MaxEntries), uint64(len(i.SourceRoot)+i.Limits.MaxPathBytes), uint64(i.Limits.MaxWork)
	// Includes URI escaping, JSON escaping, multiple detached representations,
	// callback/path bookkeeping, sorting, and bounded kernel verification passes.
	// Path and URI can each incur JSON HTML escaping (six bytes per
	// original byte); allow sixteen combined, plus fixed per-record fields.
	data := uint64(4096) + n*(16*p+512)
	logical := 2*data + 64<<10
	work := 64*(w+n+64)*(p+512) + 32*data + 8*v.Work
	if logical > 64<<20 || work > 1<<32 {
		return [14]uint64{}, false
	}
	q := countersV2{LogicalBytes: logical, ReadHashBytes: 8*v.Bytes + 16*data, WriteBytes: 4*v.Bytes + 2*data, Records: 8*v.Records + n + 64, ParserBytes: 8*v.Bytes + 4*data, Work: work, ActiveNS: uint64(i.Deadline.Sub(now)), Calls: 1, OutputBytes: data}
	return q.vector(), true
}

func (e PrivatePreparationEnumeratorV1) Enumerate(ctx context.Context, i PrivatePreparationIntentV1, attempt string) (PrivatePreparationEnumerationResultV1, error) {
	if e.state == nil {
		return PrivatePreparationEnumerationResultV1{}, ErrPrivatePreparationEnumerationV1
	}
	if !preparationIntentValidV1(i) {
		return PrivatePreparationEnumerationResultV1{}, ErrPrivatePreparationEnumerationV1
	}
	i = preparationIntentCopyV1(i)
	now := time.Now()
	if ctx == nil || ctx.Err() != nil || !hashV2(attempt) || attempt != i.Attempt || !now.Before(i.Deadline) || i.Deadline.Sub(now) > 15*time.Second || PrivatePreparationIntentIdentityV1(i) == "" || e.state.job.SourceAuthorization != PrivatePreparationIntentIdentityV1(i) || e.state.job.ResourceRules != PrivatePreparationEnumerationRulesV1(i) {
		return PrivatePreparationEnumerationResultV1{}, ErrPrivatePreparationEnumerationV1
	}
	s := e.state
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.used {
		return PrivatePreparationEnumerationResultV1{}, ErrPrivatePreparationEnumerationV1
	}
	q, ok := PrivatePreparationEnumerationQuoteV1(i, PrivateManagedInputVerificationV2{s.verification.Records, s.verification.Bytes, s.verification.Work}, now)
	if !ok {
		return PrivatePreparationEnumerationResultV1{}, ErrPrivatePreparationEnumerationV1
	}
	current, err := s.k.reopen(ctx, s.job, s.verification)
	if err != nil || current.Identity != s.head || !readyV2(current) || current.Windows != 0 || current.Charged != (countersV2{}) {
		return PrivatePreparationEnumerationResultV1{}, ErrPrivatePreparationEnumerationV1
	}
	// Burn this handle before an ambiguous reservation, including committed+error.
	s.used = true
	reserved, err := s.k.reserve(ctx, s.job, s.head, attempt, countersFromV2(q), i.Deadline.UnixNano(), now.UnixNano(), s.verification)
	if err != nil {
		return PrivatePreparationEnumerationResultV1{}, ErrPrivatePreparationEnumerationV1
	}
	confirmed, err := s.k.reopen(ctx, s.job, s.verification)
	if err != nil || confirmed.Identity != reserved.Identity || confirmed.Disposition != "PENDING" || confirmed.Attempt != attempt || confirmed.Charged != reserved.Charged {
		return PrivatePreparationEnumerationResultV1{}, ErrPrivatePreparationEnumerationV1
	}
	funded, cancel := context.WithDeadline(ctx, i.Deadline)
	defer cancel()
	files, err := censusacquisition.EnumerateWorkspaceContext(funded, i.SourceRoot, i.Roots, source.LanguageID, censusacquisition.Filters{Includes: i.Includes, Excludes: i.Excludes}, i.Limits, i.PrunedDirectories...)
	if err == nil {
		if len(files) > i.Limits.MaxEntries {
			err = ErrPrivatePreparationEnumerationV1
		}
		for _, f := range files {
			if !utf8.ValidString(f.Path) || !utf8.ValidString(f.URI) || len(f.Path) > i.Limits.MaxPathBytes {
				err = ErrPrivatePreparationEnumerationV1
				break
			}
		}
	}
	if err == nil {
		raw, encodeErr := json.Marshal(files)
		if encodeErr != nil || uint64(len(raw)) > q[13] {
			err = ErrPrivatePreparationEnumerationV1
		}
	}
	if funded.Err() != nil {
		err = ErrPrivatePreparationEnumerationV1
	}
	disposition := "OBSERVED"
	if err != nil {
		disposition = "FAILED"
	}
	completed, completeErr := s.k.observe(funded, s.job, reserved.Identity, attempt, disposition, time.Now().UnixNano(), s.verification)
	if completeErr != nil {
		return PrivatePreparationEnumerationResultV1{}, ErrPrivatePreparationEnumerationV1
	}
	readback, readErr := s.k.reopen(funded, s.job, s.verification)
	if readErr != nil || readback.Identity != completed.Identity || readback.Disposition != disposition || readback.Charged != reserved.Charged || readback.Attempt != attempt || err != nil || funded.Err() != nil {
		return PrivatePreparationEnumerationResultV1{}, ErrPrivatePreparationEnumerationV1
	}
	return PrivatePreparationEnumerationResultV1{&preparationEnumerationResultV1{intent: PrivatePreparationIntentIdentityV1(i), reservation: reserved.Identity, completion: completed.Identity, files: append([]censusacquisition.SourceFile(nil), files...)}}, nil
}
