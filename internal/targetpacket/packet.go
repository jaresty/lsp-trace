// Package targetpacket prepares provisional target evidence from an admitted Program C census.
package targetpacket

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"lsp-trace/internal/censusprogramc"
	"lsp-trace/internal/retainedprojection"
	"lsp-trace/internal/sourceprojection"
	"lsp-trace/internal/sourceprojectionv2"
)

type Stage string

const (
	StageInput     Stage = "INPUT"
	StageReconcile Stage = "RECONCILE"
	StageAdmit     Stage = "ADMIT"
	StageSelect    Stage = "SELECT"
	StageResolve   Stage = "RESOLVE"
	StageProject   Stage = "PROJECT"
	StageAssemble  Stage = "ASSEMBLE"
)

type Code string

const (
	CodeInvalidRequest  Code = "INVALID_REQUEST"
	CodeCustodyMismatch Code = "CUSTODY_MISMATCH"
	CodeResolveFailed   Code = "RESOLVE_FAILED"
	CodeAssemblyFailed  Code = "ASSEMBLY_FAILED"
)

type Failure struct {
	Stage Stage
	Code  Code
	Err   error
}

func (e *Failure) Error() string { return fmt.Sprintf("targetpacket %s/%s", e.Stage, e.Code) }
func (e *Failure) Unwrap() error { return e.Err }
func failed(stage Stage, code Code, err error) (Result, error) {
	return Result{State: StateFailed}, &Failure{stage, code, err}
}

type Snapshot struct {
	ConstituentIdentity string
	ConstituentOrdinal  int
	Raw                 []byte
}
type Request struct {
	Census           censusprogramc.Result
	Snapshots        []Snapshot
	Lookup           retainedprojection.Lookup
	Policy           sourceprojection.Policy
	ResolveLimits    retainedprojection.ResolveLimits
	MaxResponseBytes int
}
type PreparationState string

const (
	StateEmpty      PreparationState = "EMPTY"
	StateUnresolved PreparationState = "UNRESOLVED"
	StatePrepared   PreparationState = "PREPARED"
	StateFailed     PreparationState = "FAILED"
)

// Lineage is the packet-owned, JSON-stable structural nomination lineage. It
// deliberately retains no authority or semantic acceptance claim.
type Lineage struct {
	Status                  string   `json:"status"`
	ClaimCeiling            string   `json:"claim_ceiling"`
	SelectionState          string   `json:"selection_state"`
	Members                 []string `json:"members"`
	PreparedTargets         []string `json:"prepared_targets"`
	SCCMembers              []string `json:"scc_members"`
	CensusID                string   `json:"census_id"`
	BatchID                 string   `json:"batch_id"`
	CommunityIdentity       string   `json:"community_identity"`
	ConstituentIdentity     string   `json:"constituent_identity"`
	ConstituentOrdinal      int      `json:"constituent_ordinal"`
	Distance                int      `json:"distance"`
	ExecutionBundleID       string   `json:"execution_bundle_id"`
	SeedLabel               string   `json:"seed_label"`
	SeedAt                  string   `json:"seed_at"`
	SelectedNode            string   `json:"selected_node"`
	SelectedLogicalSourceID string   `json:"selected_logical_source_id"`
	SourceGraphComplete     string   `json:"source_graph_complete"`
	AllTraversalComplete    bool     `json:"all_traversal_complete"`
	AnyTruncated            bool     `json:"any_truncated"`
}

func cloneLineage(in censusprogramc.Representative, logical string) Lineage {
	return Lineage{Status: in.Status, ClaimCeiling: in.ClaimCeiling, SelectionState: in.SelectionState,
		Members: append([]string(nil), in.Members...), PreparedTargets: append([]string(nil), in.PreparedTargets...), SCCMembers: append([]string(nil), in.SCCMembers...),
		CensusID: in.CensusID, BatchID: in.BatchID, CommunityIdentity: in.CommunityIdentity, ConstituentIdentity: in.ConstituentIdentity, ConstituentOrdinal: in.ConstituentOrdinal,
		Distance: in.Distance, ExecutionBundleID: in.ExecutionBundleID, SeedLabel: in.SeedLabel, SeedAt: in.SeedAt, SelectedNode: in.SelectedNode, SelectedLogicalSourceID: logical,
		SourceGraphComplete: in.SourceGraphComplete, AllTraversalComplete: in.AllTraversalComplete, AnyTruncated: in.AnyTruncated}
}

func cloneRepresentative(in censusprogramc.Representative) censusprogramc.Representative {
	in.Members = append([]string(nil), in.Members...)
	in.PreparedTargets = append([]string(nil), in.PreparedTargets...)
	in.SCCMembers = append([]string(nil), in.SCCMembers...)
	return in
}

func cloneRepresentatives(in []censusprogramc.Representative) []censusprogramc.Representative {
	out := make([]censusprogramc.Representative, len(in))
	for i := range in {
		out[i] = cloneRepresentative(in[i])
	}
	return out
}

type Packet struct {
	SchemaVersion   string                                                                   `json:"schema_version"`
	PacketID        string                                                                   `json:"packet_id"`
	CensusID        string                                                                   `json:"census_id"`
	Lineage         Lineage                                                                  `json:"lineage"`
	Status          string                                                                   `json:"status"`
	Authority       int                                                                      `json:"authority"`
	Accepted        bool                                                                     `json:"accepted"`
	Completeness    string                                                                   `json:"completeness"`
	ClaimCeiling    string                                                                   `json:"claim_ceiling"`
	PrivacyPolicyID string                                                                   `json:"privacy_policy_id"`
	Custody         retainedprojection.RetainedCustodyBinding                                `json:"custody_binding"`
	Projection      sourceprojectionv2.WireResult[retainedprojection.RetainedCustodyBinding] `json:"projection"`
}
type Result struct {
	State           PreparationState                `json:"state"`
	Packets         []Packet                        `json:"packets,omitempty"`
	UnresolvedCount int                             `json:"unresolved_count"`
	Unresolved      []censusprogramc.Representative `json:"unresolved,omitempty"`
}

func Build(req Request) (Result, error) {
	// Build only reads the caller-owned census; outputs clone representative slices.
	census := req.Census
	reps := census.Representatives
	unresolved := cloneRepresentatives(reps.Unresolved)
	if len(reps.Nominations) == 0 {
		if len(unresolved) > 0 {
			sort.Slice(unresolved, func(i, j int) bool { return lineageKey(unresolved[i]) < lineageKey(unresolved[j]) })
			return Result{State: StateUnresolved, UnresolvedCount: len(unresolved), Unresolved: unresolved}, nil
		}
		return Result{State: StateEmpty}, nil
	}
	if req.Lookup == nil || req.MaxResponseBytes <= 0 {
		return failed(StageInput, CodeInvalidRequest, errors.New("lookup and positive response bound required"))
	}
	if req.Policy.PolicyID == "" {
		return failed(StageInput, CodeInvalidRequest, errors.New("caller policy required"))
	}
	if req.Policy.BodyRequested && (req.Policy.MaxBytes <= 0 || req.Policy.MaxRanges <= 0 || req.Policy.MaxObjects <= 0 || req.Policy.MaxWork <= 0) {
		return failed(StageProject, CodeAssemblyFailed, errors.New("positive projection policy limits required"))
	}
	byOrdinal := map[int]Snapshot{}
	for _, s := range req.Snapshots {
		if s.ConstituentOrdinal < 0 || s.ConstituentOrdinal >= len(census.Admission.Artifact.Constituents) {
			return failed(StageReconcile, CodeCustodyMismatch, errors.New("snapshot ordinal outside census"))
		}
		if _, ok := byOrdinal[s.ConstituentOrdinal]; ok {
			return failed(StageReconcile, CodeCustodyMismatch, errors.New("duplicate snapshot"))
		}
		byOrdinal[s.ConstituentOrdinal] = Snapshot{ConstituentIdentity: s.ConstituentIdentity, ConstituentOrdinal: s.ConstituentOrdinal, Raw: append([]byte(nil), s.Raw...)}
	}
	used := map[int]bool{}
	nominations := cloneRepresentatives(reps.Nominations)
	sort.Slice(nominations, func(i, j int) bool { return lineageKey(nominations[i]) < lineageKey(nominations[j]) })
	packets := make([]Packet, 0, len(nominations))
	ids := map[string]bool{}
	for _, n := range nominations {
		if !validRepresentative(census, n) {
			return failed(StageSelect, CodeInvalidRequest, errors.New("invalid selected census lineage"))
		}
		constituent := census.Admission.Artifact.Constituents[n.ConstituentOrdinal]
		if n.ConstituentIdentity != constituent.Identity {
			return failed(StageSelect, CodeCustodyMismatch, errors.New("nomination constituent mismatch"))
		}
		s, ok := byOrdinal[n.ConstituentOrdinal]
		if !ok || s.ConstituentIdentity != constituent.Identity {
			return failed(StageReconcile, CodeCustodyMismatch, errors.New("missing or foreign snapshot"))
		}
		used[n.ConstituentOrdinal] = true
		admitted, err := retainedprojection.Admit(s.Raw)
		if err != nil {
			return failed(StageAdmit, CodeCustodyMismatch, errors.New("snapshot admission failed"))
		}
		key, err := mapSelectedNode(admitted, n.SelectedNode)
		if err != nil {
			return failed(StageSelect, CodeInvalidRequest, err)
		}
		plan, err := retainedprojection.Select(admitted, retainedprojection.Request{Target: key, Selections: []retainedprojection.Key{key}})
		if err != nil {
			return failed(StageSelect, CodeInvalidRequest, err)
		}
		binding, err := admitted.CustodyBinding(plan)
		if err != nil {
			return failed(StageAdmit, CodeCustodyMismatch, err)
		}
		if binding.GraphDigest != constituent.GraphSHA256 || int(binding.GraphByteLength) != constituent.GraphByteLength {
			return failed(StageReconcile, CodeCustodyMismatch, errors.New("snapshot graph differs from census admission"))
		}
		resolved, err := retainedprojection.Resolve(plan, req.Lookup, req.ResolveLimits)
		if err != nil {
			return failed(StageResolve, CodeResolveFailed, safe(err))
		}
		wire, err := retainedprojection.AssembleV2Bounded(resolved, req.Policy, binding, req.Policy.PolicyID, req.MaxResponseBytes)
		if err != nil {
			stage := StageAssemble
			var assembly *retainedprojection.AssemblyError
			if errors.As(err, &assembly) && assembly.Code == retainedprojection.CodeProjectionFailed {
				stage = StageProject
			}
			return failed(stage, CodeAssemblyFailed, safe(err))
		}
		p := Packet{SchemaVersion: "lsp-trace.targetpacket.v2", CensusID: census.CensusID, Lineage: cloneLineage(n, key.LogicalSourceID), Status: "PROVISIONAL", Authority: 0, Accepted: false, Completeness: "UNKNOWN", ClaimCeiling: n.ClaimCeiling, PrivacyPolicyID: req.Policy.PolicyID, Custody: binding, Projection: wire}
		p.PacketID, err = packetIdentity(p)
		if err != nil {
			return failed(StageAssemble, CodeAssemblyFailed, errors.New("packet encoding failed"))
		}
		if ids[p.PacketID] {
			return failed(StageSelect, CodeInvalidRequest, errors.New("duplicate packet identity"))
		}
		ids[p.PacketID] = true
		packets = append(packets, p)
	}
	for ordinal := range byOrdinal {
		if !used[ordinal] {
			return failed(StageReconcile, CodeCustodyMismatch, errors.New("unused extra snapshot"))
		}
	}
	sort.Slice(packets, func(i, j int) bool { return packets[i].PacketID < packets[j].PacketID })
	sort.Slice(unresolved, func(i, j int) bool { return lineageKey(unresolved[i]) < lineageKey(unresolved[j]) })
	return Result{State: StatePrepared, Packets: packets, UnresolvedCount: len(unresolved), Unresolved: unresolved}, nil
}
func safe(err error) error {
	var selected *retainedprojection.Error
	if errors.As(err, &selected) {
		return selected
	}
	var assembly *retainedprojection.AssemblyError
	if errors.As(err, &assembly) {
		return assembly
	}
	return errors.New("retained projection failed")
}
func lineageKey(n censusprogramc.Representative) string {
	// Representative has only JSON-stable fields and no maps; its canonical JSON
	// covers every lineage field used to deterministically order nominations.
	b, err := json.Marshal(n)
	if err != nil {
		panic("representative canonical encoding: " + err.Error())
	}
	return string(b)
}

func canonicalStrings(values []string) bool {
	if len(values) == 0 {
		return false
	}
	for i, value := range values {
		if value == "" || (i > 0 && values[i-1] >= value) {
			return false
		}
	}
	return true
}

func validRepresentative(census censusprogramc.Result, n censusprogramc.Representative) bool {
	if n.Status != censusprogramc.CandidateStatus || n.SelectionState != "SELECTED" || n.Authority != 0 || n.SourceGraphComplete != "UNKNOWN" || n.CensusID != census.CensusID || n.ConstituentOrdinal < 0 || n.ConstituentOrdinal >= len(census.Admission.Artifact.Constituents) || n.Distance < 0 || n.BatchID == "" || n.CommunityIdentity == "" || n.ExecutionBundleID == "" || n.SeedLabel == "" || n.SeedAt == "" || n.SelectedNode == "" || !canonicalStrings(n.Members) || !canonicalStrings(n.SCCMembers) {
		return false
	}
	if census.Outcome.ClaimCeiling != "" && n.ClaimCeiling != census.Outcome.ClaimCeiling {
		return false
	}
	if census.Admission.Artifact.ClaimCeiling != "" && n.ClaimCeiling != census.Admission.Artifact.ClaimCeiling {
		return false
	}
	return n.ClaimCeiling != ""
}
func mapSelectedNode(a retainedprojection.Admitted, node string) (retainedprojection.Key, error) {
	var found retainedprojection.Key
	count := 0
	for _, k := range a.DisplayKeys() {
		if k.GraphSubjectID == node {
			found = k
			count++
		}
	}
	if count != 1 {
		return retainedprojection.Key{}, errors.New("selected node must map to exactly one display binding")
	}
	return found, nil
}
func packetIdentity(p Packet) (string, error) {
	p.PacketID = ""
	b, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}
func EncodeCanonical(p Packet) ([]byte, error) {
	id, err := packetIdentity(p)
	if err != nil {
		return nil, err
	}
	p.PacketID = id
	return json.Marshal(p)
}
func Validate(raw []byte) (Packet, error) {
	if err := rejectDuplicateKeys(raw); err != nil {
		return Packet{}, err
	}
	var p Packet
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&p); err != nil {
		return Packet{}, err
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return Packet{}, errors.New("trailing json")
	}
	if !validPacket(p) {
		return Packet{}, errors.New("packet semantic invariant mismatch")
	}
	id, err := packetIdentity(p)
	if err != nil {
		return Packet{}, err
	}
	if p.PacketID == "" || p.PacketID != id {
		return Packet{}, errors.New("packet identity mismatch")
	}
	return p, nil
}
func validPacket(p Packet) bool {
	l := p.Lineage
	if p.SchemaVersion != "lsp-trace.targetpacket.v2" || p.Status != "PROVISIONAL" || p.Authority != 0 || p.Accepted || p.Completeness != "UNKNOWN" || p.CensusID == "" || l.CensusID != p.CensusID || l.Status != censusprogramc.CandidateStatus || l.SelectionState != "SELECTED" || l.SourceGraphComplete != "UNKNOWN" || l.ClaimCeiling == "" || l.ClaimCeiling != p.ClaimCeiling || l.ConstituentIdentity == "" || l.ConstituentOrdinal < 0 || l.Distance < 0 || l.BatchID == "" || l.CommunityIdentity == "" || l.ExecutionBundleID == "" || l.SeedLabel == "" || l.SeedAt == "" || l.SelectedNode == "" || l.SelectedLogicalSourceID == "" || !canonicalStrings(l.Members) || !canonicalStrings(l.SCCMembers) {
		return false
	}
	w := p.Projection
	if w.SchemaVersion != sourceprojectionv2.SchemaVersion || !validProjectionStatus(w.Status) || w.Authority != 0 || w.SourceGraphComplete != "UNKNOWN" || w.CustodyMode != "RETAINED" || w.RequestPolicyID == "" || p.PrivacyPolicyID == "" || p.PrivacyPolicyID != w.RequestPolicyID || p.Custody != w.CustodyBinding || !validCustody(p.Custody) || !validPhysicalProjectionID(w) || w.DocumentSelection.TargetURI != l.SelectedLogicalSourceID || len(w.DocumentSelection.SelectedURIs) == 0 || w.DocumentSelection.SelectedURIs[0] != w.DocumentSelection.TargetURI {
		return false
	}
	var binding *sourceprojectionv2.DocumentBinding
	for i := range w.DocumentBindings {
		b := &w.DocumentBindings[i]
		if b.URI == l.SelectedLogicalSourceID && b.Role == "TARGET" && b.Ordinal == 0 && b.Status == "ACQUIRED" {
			if binding != nil {
				return false
			}
			binding = b
		}
	}
	if binding == nil {
		return false
	}
	var target *sourceprojectionv2.Unit
	for i := range w.Units {
		u := &w.Units[i]
		// Retained projection encodes the requested target as its sole ENDPOINT.
		if u.Role == "ENDPOINT" && u.GraphSubjectID == l.SelectedNode && u.LogicalSourceID == l.SelectedLogicalSourceID {
			if target != nil {
				return false
			}
			target = u
		}
	}
	if target == nil || target.SourceDigest == "" || target.SourceDigest != binding.SourceDigest || target.SourceByteLength <= 0 || target.SourceByteLength != binding.SourceByteLength || !validBody(*target) || !hasCitation(w.Citations, *target) || !hasSpan(w.EmittedSpans, *target) {
		return false
	}
	return true
}

func validProjectionStatus(status string) bool {
	switch status {
	case "COMPLETE", "PARTIAL", "TRUNCATED", "SOURCE_UNAVAILABLE", "SUCCESSFUL_EMPTY":
		return true
	}
	return false
}
func validPhysicalProjectionID(w sourceprojectionv2.WireResult[retainedprojection.RetainedCustodyBinding]) bool {
	raw, err := json.Marshal(w.DocumentBindings)
	if err != nil {
		return false
	}
	digest := sha256.Sum256(raw)
	return w.PhysicalProjectionID == "sha256:"+hex.EncodeToString(digest[:])
}
func validCustody(c retainedprojection.RetainedCustodyBinding) bool {
	if c.Custody != retainedprojection.RetainedCustody || c.GraphSchemaID == "" || c.CaptureID == "" || c.ManifestID == "" || c.ResolverKind == "" || c.GraphByteLength == 0 || len(c.GraphDigest) != len("sha256:")+64 || !strings.HasPrefix(c.GraphDigest, "sha256:") {
		return false
	}
	_, err := hex.DecodeString(c.GraphDigest[len("sha256:"):])
	return err == nil
}
func validBody(u sourceprojectionv2.Unit) bool {
	return (u.BodyDisposition == "RETURNED" && u.Body != "") || (u.BodyDisposition == "NOT_REQUESTED" && u.Body == "") || (u.BodyDisposition == "OMITTED" && u.Body == "")
}
func hasCitation(citations []sourceprojectionv2.Citation, u sourceprojectionv2.Unit) bool {
	for _, c := range citations {
		if c.UnitID == u.UnitID && c.Role == u.Role && c.SubjectID == u.GraphSubjectID && c.OccurrenceID == u.OccurrenceID && c.EvidenceRange == u.EvidenceRange && c.DisplayRange == u.DisplayRange {
			return true
		}
	}
	return false
}
func hasSpan(spans []sourceprojection.Span, u sourceprojectionv2.Unit) bool {
	for _, s := range spans {
		if s.LogicalSourceID == u.LogicalSourceID && s.Range == u.DisplayRange && s.SourceDigest == u.SourceDigest && s.Body == u.Body && (u.BodyDisposition != "RETURNED" || s.ByteLength == len(s.Body)) {
			count := 0
			for _, id := range s.UnitIDs {
				if id == u.UnitID {
					count++
				}
			}
			if count == 1 {
				return true
			}
		}
	}
	return false
}

func rejectDuplicateKeys(raw []byte) error {
	var value any
	d := json.NewDecoder(bytes.NewReader(raw))
	if err := d.Decode(&value); err != nil {
		return err
	}
	return scanObject(raw)
}
func scanObject(raw []byte) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	tok, err := d.Token()
	if err != nil {
		return err
	}
	switch x := tok.(type) {
	case json.Delim:
		if x == '{' {
			seen := map[string]bool{}
			for d.More() {
				k, e := d.Token()
				if e != nil {
					return e
				}
				key := k.(string)
				if seen[key] {
					return errors.New("duplicate json key")
				}
				seen[key] = true
				var child json.RawMessage
				if e = d.Decode(&child); e != nil {
					return e
				}
				if e = scanObject(child); e != nil {
					return e
				}
			}
			_, err = d.Token()
			return err
		}
		if x == '[' {
			for d.More() {
				var child json.RawMessage
				if err := d.Decode(&child); err != nil {
					return err
				}
				if err := scanObject(child); err != nil {
					return err
				}
			}
			_, err = d.Token()
			return err
		}
	}
	return nil
}
