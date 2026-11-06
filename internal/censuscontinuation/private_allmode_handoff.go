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
}

func (h PrivateAllModeHandoff) ID() string { return h.id }

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
	return PrivateAllModeHandoff{id: digestOf(raw), wire: w, entries: append([]PrivateAllModeEntry(nil), entries...)}, nil
}
