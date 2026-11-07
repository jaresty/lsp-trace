package censuscontinuation

import (
	"encoding/json"
	"errors"
	"reflect"
	"unicode/utf8"

	"lsp-trace/internal/programc"
	rg "lsp-trace/internal/referencegrouping"
)

const PrivateAllModeVersion = "lsp-trace.private.allmode-continuation.v1"
const PrivateAllModeReady = "READY"
const PrivateAllModeDeclaredMissing = "DECLARED_MISSING_UNRESOLVED"
const privateAllModeMaxSlots = 64

// The schedule is an owner declaration, never an attestation of RPC execution.
// Ordinal and seed namespace retain repeated seeds without inventing run hashes.
type PrivateAllModeSlot struct {
	Ordinal                    int
	Mode, RunID, SeedNamespace string
	Seed                       uint64
}

type PrivateAllModeEntry struct {
	Slot                                PrivateAllModeSlot
	Disposition, Lineage, MissingReason string
	Input                               rg.AdmittedInput
	Policy                              rg.GroupingPolicy
	Case                                programc.GroupingEvaluationCasePrivate
	Result                              programc.GroupingEvaluationResultPrivate
}

type privateAllModeEntryCommitment struct {
	Slot                                PrivateAllModeSlot
	Disposition, Lineage, MissingReason string
	Binding                             programc.GroupingPairBindingPrivate
	EligibleKinds                       []string
}

type privateAllModeHandoffWire struct {
	Version, Workspace, Revision, ClaimCeiling string
	Schedule                                   []PrivateAllModeSlot
	Entries                                    []privateAllModeEntryCommitment
}

// PrivateAllModeHandoff is process-local opaque admission. Persistence stores
// metadata commitments, not reconstitutable producer/source authority. Resume
// requires this exact expected opaque handoff again; no JSON admission exists.
type PrivateAllModeHandoff struct {
	id      string
	wire    privateAllModeHandoffWire
	entries []PrivateAllModeEntry
	// Scalar from the constructor's already-computed wire bytes. Not serialized
	// and not an admission, identity, replay or independent validation token.
	retainedWireBytes uint64
}

func (h PrivateAllModeHandoff) ID() string { return h.id }

// RetainedPayloadUpper observes already-admitted, immutable payload lengths.
// Charge canonical + parsed-document logical payload representations (2x) for
// every READY entry, including repeated/shared inputs and entries for which no
// SourceOriginals were supplied. The case's detached projection is separately
// retained. Complete wire/schedule/entry metadata is covered by 2x the cached
// constructor wire length. This is not a physical Go heap/sizeof assertion.
// No payload is copied, marshaled, parsed, evaluated, replayed or admitted here.
func (h PrivateAllModeHandoff) RetainedPayloadUpper() (uint64, bool) {
	if h.id == "" || len(h.entries) < 7 || len(h.entries) > privateAllModeMaxSlots || len(h.wire.Entries) != len(h.entries) || len(h.wire.Schedule) != len(h.entries) || h.retainedWireBytes == 0 || h.retainedWireBytes > uint64(rg.EvaluationMaxBytesPrivate) {
		return 0, false
	}
	var upper uint64
	add := func(n uint64) bool {
		if n > (^uint64(0)-upper)/2 {
			return false
		}
		upper += 2 * n
		return true
	}
	if !add(h.retainedWireBytes) {
		return 0, false
	}
	for i, e := range h.entries {
		w := h.wire.Entries[i]
		if e.Slot.Ordinal != i || e.Slot != h.wire.Schedule[i] || e.Slot != w.Slot || e.Disposition != w.Disposition || e.Lineage != w.Lineage || e.MissingReason != w.MissingReason {
			return 0, false
		}
		switch e.Disposition {
		case PrivateAllModeReady:
			b := w.Binding           // commitments established by the existing constructor
			p := e.Case.Projection() // opaque value header only, not canonical bytes
			if e.Input.ID() == "" || e.Policy.ID() == "" || e.Case.ID() == "" || e.Result.ID() == "" || p.ID() == "" ||
				b.Bindings.Input.ID != e.Input.ID() || b.Bindings.Input.Digest != e.Input.Digest() || b.Bindings.Input.ByteLength != e.Input.ByteLength() ||
				b.Bindings.Policy.ID != e.Policy.ID() || b.Bindings.Policy.Digest != e.Policy.Digest() || b.Bindings.Policy.ByteLength != e.Policy.ByteLength() ||
				b.Case.ID != e.Case.ID() || b.Case.Digest != e.Case.ID() ||
				b.Result.ID != e.Result.ID() || b.Result.Digest != e.Result.Digest() || b.Result.ByteLength != e.Result.ByteLength() ||
				b.Projection.ID != p.ID() || b.Projection.Digest != p.Digest() || b.Projection.ByteLength != p.ByteLength() {
				return 0, false
			}
			// Case has no ByteLength accessor; use its constructor-validated
			// commitment, never Case.CanonicalBytes or a fresh binding/replay.
			for _, n := range [...]int{e.Input.ByteLength(), e.Policy.ByteLength(), b.Case.ByteLength, e.Result.ByteLength(), p.ByteLength()} {
				if n <= 0 || n > rg.EvaluationMaxBytesPrivate || !add(uint64(n)) {
					return 0, false
				}
			}
		case PrivateAllModeDeclaredMissing:
			if e.Input.ID() != "" || e.Policy.ID() != "" || e.Case.ID() != "" || e.Result.ID() != "" {
				return 0, false
			}
		default:
			return 0, false
		}
	}
	return upper, true
}

func privateAllModeString(s string) bool {
	return s != "" && len(s) <= rg.EvaluationMaxStringPrivate && utf8.ValidString(s)
}
func privateAllModeKinds(mode string) []string {
	switch mode {
	case "C":
		return []string{"CALLS"}
	case "R":
		return []string{"REFERENCES_SYMBOL"}
	case "D":
		return []string{"RESOLVES_TO_DEFINITION"}
	case "CR":
		return []string{"CALLS", "REFERENCES_SYMBOL"}
	case "CD":
		return []string{"CALLS", "RESOLVES_TO_DEFINITION"}
	case "RD":
		return []string{"REFERENCES_SYMBOL", "RESOLVES_TO_DEFINITION"}
	case "CRD":
		return []string{"CALLS", "REFERENCES_SYMBOL", "RESOLVES_TO_DEFINITION"}
	}
	return nil
}

func NewPrivateAllModeHandoff(workspace, revision string, schedule []PrivateAllModeSlot, entries []PrivateAllModeEntry) (PrivateAllModeHandoff, error) {
	bad := func() (PrivateAllModeHandoff, error) {
		return PrivateAllModeHandoff{}, errors.New("private allmode handoff binding mismatch")
	}
	if !privateAllModeString(workspace) || !privateAllModeString(revision) || len(schedule) < 7 || len(schedule) > privateAllModeMaxSlots || len(entries) != len(schedule) {
		return bad()
	}
	w := privateAllModeHandoffWire{Version: PrivateAllModeVersion, Workspace: workspace, Revision: revision, ClaimCeiling: rg.EvaluationCeilingPrivate, Schedule: append([]PrivateAllModeSlot(nil), schedule...)}
	modes := map[string]bool{}
	for i, s := range schedule {
		if s.Ordinal != i || privateAllModeKinds(s.Mode) == nil || !privateAllModeString(s.RunID) || !privateAllModeString(s.SeedNamespace) || entries[i].Slot != s || !privateAllModeString(entries[i].Lineage) {
			return bad()
		}
		modes[s.Mode] = true
		e := entries[i]
		c := privateAllModeEntryCommitment{Slot: s, Disposition: e.Disposition, Lineage: e.Lineage, MissingReason: e.MissingReason}
		switch e.Disposition {
		case PrivateAllModeReady:
			if e.MissingReason != "" {
				return bad()
			}
			m, err := rg.ReadAllModeMetadataPrivate(e.Input)
			if err != nil {
				return bad()
			}
			policy, kinds, err := rg.ReadAllModePolicyMetadataPrivate(e.Policy)
			if err != nil || !reflect.DeepEqual(kinds, privateAllModeKinds(s.Mode)) {
				return bad()
			}
			b, err := programc.BindGroupingPairPrivate(e.Case, e.Result, e.Input)
			if err != nil || b.Seed != s.Seed || b.RunID != s.RunID || b.Bindings.Policy != policy {
				return bad()
			}
			c.Binding = b
			c.EligibleKinds = m.EligibleKinds
		case PrivateAllModeDeclaredMissing:
			if !privateAllModeString(e.MissingReason) || e.Input.ID() != "" || e.Policy.ID() != "" || e.Case.ID() != "" || e.Result.ID() != "" {
				return bad()
			}
		default:
			return bad()
		}
		w.Entries = append(w.Entries, c)
	}
	if len(modes) != 7 {
		return bad()
	}
	raw, err := json.Marshal(w)
	if err != nil || len(raw) > rg.EvaluationMaxBytesPrivate {
		return bad()
	}
	return PrivateAllModeHandoff{id: digestOf(raw), wire: w, entries: append([]PrivateAllModeEntry(nil), entries...), retainedWireBytes: uint64(len(raw))}, nil
}
