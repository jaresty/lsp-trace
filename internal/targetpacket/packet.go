// Package targetpacket converts already-selected provisional Program C
// nominations plus separately supplied retained V2 source custody into
// immutable TARGET evidence packets. It is transport-neutral and performs no
// I/O: source bodies are reached only through a caller-supplied
// retainedprojection.Lookup, and no census mutation, source acquisition, or
// ambient filesystem fallback occurs here.
package targetpacket

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"

	"lsp-trace/internal/censusprogramc"
	"lsp-trace/internal/retainedprojection"
	"lsp-trace/internal/sourceprojection"
)

// Code is a closed, typed outcome discriminator. Every failure path returns a
// distinct code so callers can distinguish EMPTY, UNRESOLVED, and the several
// preparation failures without parsing messages.
type Code string

const (
	CodeInvalidRequest  Code = "INVALID_REQUEST"
	CodeCustodyMismatch Code = "CUSTODY_MISMATCH"
	CodeLogicalSource   Code = "LOGICAL_SOURCE_AMBIGUOUS"
	CodeResolveFailed   Code = "RESOLVE_FAILED"
	CodeAssemblyFailed  Code = "ASSEMBLY_FAILED"
	CodeUnresolved      Code = "UNRESOLVED"
	CodeEmpty           Code = "EMPTY"
)

// PreparationState is the outcome discriminator for a whole Build call. EMPTY,
// UNRESOLVED, PREPARED, and FAILED are mutually exclusive (Property [7]).
type PreparationState string

const (
	StateEmpty      PreparationState = "EMPTY"
	StateUnresolved PreparationState = "UNRESOLVED"
	StatePrepared   PreparationState = "PREPARED"
	StateFailed     PreparationState = "FAILED"
)

// SnapshotCustody is one separately supplied retained V2 source-custody record
// for a single census constituent. Its GraphDigest/GraphByteLength must exactly
// equal the constituent's immutable Graph V5 identity (Property [2]).
type SnapshotCustody struct {
	ConstituentIdentity string
	GraphDigest         string
	GraphByteLength     uint64
	Raw                 []byte
}

// Request is the closed input: the already-selected nominations, the retained
// V2 custody supplied independently, and the census result they were selected
// from. No path or ambient-checkout field exists (Property [4]).
type Request struct {
	CensusID    string
	Nominations []censusprogramc.Representative
	Unresolved  []censusprogramc.Representative
	Custody     []SnapshotCustody
	Lookup      retainedprojection.Lookup
	MaxBytes    int
}

// Packet is one immutable TARGET evidence packet bound to exactly one
// nomination (Property [1]) with a deterministic identity (Property [6]).
type Packet struct {
	CensusID            string
	BatchID             string
	ConstituentIdentity string
	ConstituentOrdinal  int
	ExecutionBundleID   string
	SeedLabel           string
	SeedPosition        string
	CommunityIdentity   string
	CommunityMembers    []string
	SelectedNode        string
	Distance            int
	SCCMembers          []string
	ClaimCeiling        string
	Authority           int
	Accepted            bool

	GraphDigest     string
	GraphByteLength uint64
	CaptureID       string
	ManifestID      string
	ResolverKind    string
	PrivacyPolicyID string
	LogicalSourceID string
	BodyDigest      string
	Body            []byte
	Completeness    string

	PacketID string // deterministic sha256 over the covered fields
}

// Result is the transport-neutral output of Build.
type Result struct {
	State   PreparationState
	Packets []Packet
}

// BuildError is the typed, closed failure of a Build preparation. Distinct
// codes let callers separate custody, lookup, and assembly faults without
// parsing text (Property [5], [7]).
type BuildError struct {
	Code   Code
	Detail string
}

func (e *BuildError) Error() string { return "targetpacket: " + string(e.Code) + ": " + e.Detail }

func fail(code Code, detail string) (Result, error) {
	return Result{State: StateFailed}, &BuildError{Code: code, Detail: detail}
}

// Build converts the selected nominations + retained custody into immutable
// TARGET packets. Transport-neutral: no I/O, no census mutation, no ambient
// fallback. Source bodies are reached only via req.Lookup (Property [4]).
func Build(req Request) (Result, error) {
	// Property [7]: EMPTY / UNRESOLVED / PREPARED / FAILED are distinct.
	if len(req.Nominations) == 0 {
		if len(req.Unresolved) > 0 {
			return Result{State: StateUnresolved}, nil
		}
		return Result{State: StateEmpty}, nil
	}
	// Property [4]: source bodies resolve only through a non-nil lookup.
	if req.Lookup == nil {
		return fail(CodeInvalidRequest, "a non-nil source lookup is required")
	}

	// Property [5]: bounded assembly requires a positive response bound.
	if req.MaxBytes <= 0 {
		return fail(CodeInvalidRequest, "a positive MaxBytes response bound is required")
	}

	// Index the separately supplied retained V2 custody by constituent, failing
	// closed on duplicates (Property [2]: duplicate custody fails closed).
	custodyByConstituent := make(map[string]SnapshotCustody, len(req.Custody))
	for _, c := range req.Custody {
		if _, dup := custodyByConstituent[c.ConstituentIdentity]; dup {
			return fail(CodeCustodyMismatch, "duplicate custody for constituent "+c.ConstituentIdentity)
		}
		custodyByConstituent[c.ConstituentIdentity] = c
	}

	packets := make([]Packet, 0, len(req.Nominations))
	for _, n := range req.Nominations {
		custody, ok := custodyByConstituent[n.ConstituentIdentity]
		if !ok {
			// Property [2]: missing custody fails closed.
			return fail(CodeCustodyMismatch, "no custody for constituent "+n.ConstituentIdentity)
		}

		// Property [2]/[4]: admit the supplied V2 snapshot through the retained
		// projection code path (validates the artifact and its embedded Graph V5
		// parent). Arbitrary or wrong-graph bytes fail closed here.
		admitted, err := retainedprojection.Admit(custody.Raw)
		if err != nil {
			return fail(CodeCustodyMismatch, "custody artifact did not admit for "+n.ConstituentIdentity)
		}

		// Property [3]: the selected node maps within the admitted snapshot to
		// exactly one (graph_subject_id, logical_source_id) display binding. Zero
		// or multiple fail closed without lexical choice.
		key, mapErr := mapSelectedNode(admitted, n.SelectedNode)
		if mapErr != nil {
			return fail(CodeLogicalSource, mapErr.Error())
		}

		plan, err := retainedprojection.Select(admitted, retainedprojection.Request{Target: key, Selections: []retainedprojection.Key{key}})
		if err != nil {
			return fail(CodeLogicalSource, "selection failed for "+n.SelectedNode)
		}

		// Property [2]: bind custody to the immutable Graph V5 identity via the
		// retained projection code (fails closed on forged/mutated/wrong-graph).
		binding, err := admitted.CustodyBinding(plan)
		if err != nil {
			return fail(CodeCustodyMismatch, "custody binding failed for "+n.ConstituentIdentity)
		}
		// Property [2]: the separately-supplied custody digest/length must match
		// the admitted artifact's Graph V5 identity.
		if custody.GraphDigest != binding.GraphDigest || custody.GraphByteLength != binding.GraphByteLength {
			return fail(CodeCustodyMismatch, "supplied custody graph identity mismatch for "+n.ConstituentIdentity)
		}

		// Property [4]: source bodies resolve only through the process-supplied
		// lookup; identity/length/SHA-256 are reverified by Resolve.
		resolved, err := retainedprojection.Resolve(plan, req.Lookup, retainedprojection.ResolveLimits{
			MaxDistinctObjects: 64, MaxUniqueSourceBytes: uint64(req.MaxBytes), MaxLogicalSelections: 64,
		})
		if err != nil {
			return fail(CodeResolveFailed, "source resolution failed for "+n.SelectedNode)
		}

		// Property [5]/[6]: bounded, typed V2 assembly with the retained custody.
		policy := sourceprojection.Policy{
			PolicyID: retainedPolicyID, BodyRequested: true,
			MaxBytes: req.MaxBytes, MaxRanges: 1024, MaxObjects: 64, MaxWork: 1 << 20, EnforceLimits: true,
		}
		wire, err := retainedprojection.AssembleV2Bounded(resolved, policy, binding, retainedPolicyID, req.MaxBytes)
		if err != nil {
			return fail(CodeAssemblyFailed, "bounded assembly failed for "+n.SelectedNode)
		}

		body, bodyDigest := resolvedBody(resolved, key)
		p := Packet{
			// Property [1]: bind exactly one nomination, all fields, authority 0,
			// accepted false.
			CensusID:            n.CensusID,
			BatchID:             n.BatchID,
			ConstituentIdentity: n.ConstituentIdentity,
			ConstituentOrdinal:  n.ConstituentOrdinal,
			ExecutionBundleID:   n.ExecutionBundleID,
			SeedLabel:           n.SeedLabel,
			SeedPosition:        n.SeedAt,
			CommunityIdentity:   n.CommunityIdentity,
			CommunityMembers:    append([]string(nil), n.Members...), // Property [8]
			SelectedNode:        n.SelectedNode,
			Distance:            n.Distance,
			SCCMembers:          append([]string(nil), n.SCCMembers...), // Property [8]
			ClaimCeiling:        n.ClaimCeiling,
			Authority:           0,
			Accepted:            false,

			GraphDigest:     binding.GraphDigest,
			GraphByteLength: binding.GraphByteLength,
			CaptureID:       binding.CaptureID,
			ManifestID:      binding.ManifestID,
			ResolverKind:    binding.ResolverKind,
			PrivacyPolicyID: wire.RequestPolicyID,
			LogicalSourceID: key.LogicalSourceID,
			BodyDigest:      bodyDigest,
			Body:            body,
			Completeness:    wire.SourceGraphComplete,
		}
		p.PacketID = packetIdentity(p)
		packets = append(packets, p)
	}
	return Result{State: StatePrepared, Packets: packets}, nil
}

const retainedPolicyID = "targetpacket.retained.v1"

// mapSelectedNode finds the single display binding whose graph subject equals
// the selected node and returns its (graph_subject_id, logical_source_id) key.
// Zero or multiple compatible bindings fail closed (Property [3]).
func mapSelectedNode(admitted retainedprojection.Admitted, selectedNode string) (retainedprojection.Key, error) {
	var found retainedprojection.Key
	matches := 0
	for _, key := range admitted.DisplayKeys() {
		if key.GraphSubjectID == selectedNode {
			found = key
			matches++
		}
	}
	if matches == 0 {
		return retainedprojection.Key{}, errors.New("selected node maps to no logical source: " + selectedNode)
	}
	if matches > 1 {
		return retainedprojection.Key{}, errors.New("selected node maps to multiple logical sources: " + selectedNode)
	}
	return found, nil
}

// resolvedBody returns the resolved source bytes and their sha256 digest for the
// selection matching key, if present.
func resolvedBody(resolved retainedprojection.ResolveResult, key retainedprojection.Key) ([]byte, string) {
	for _, sel := range resolved.Selections {
		if sel.Selection.Key == key {
			body := append([]byte(nil), sel.Bytes...)
			sum := sha256.Sum256(body)
			return body, "sha256:" + hex.EncodeToString(sum[:])
		}
	}
	return nil, ""
}

// packetIdentity is the deterministic SHA-256 over the covered packet fields
// (Property [6]): representative lineage, retained graph custody, resolver,
// logical source, authority, and completeness. PacketID itself is excluded.
func packetIdentity(p Packet) string {
	clone := p
	clone.PacketID = ""
	encoded, _ := json.Marshal(clone)
	sum := sha256.Sum256(encoded)
	return "sha256:" + hex.EncodeToString(sum[:])
}
