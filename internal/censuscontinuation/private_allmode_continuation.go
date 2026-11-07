package censuscontinuation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	rg "lsp-trace/internal/referencegrouping"
	"lsp-trace/internal/targetpacket"
)

const StopAfterSourceIntents StopAfter = "PRIVATE_SOURCE_INTENTS"

type PrivateAllModeResume struct {
	Version  string
	Expected PrivateAllModeHandoff
	Contract ContinuationContract
}

type PrivateAllModeDiagnostic struct {
	Slot                int
	Mode, Code, Lineage string
}
type PrivateAllModeCounts struct{ Slots, Ready, Missing, Empty, Excluded, Nominations, Intents int }
type PrivateAllModeResult struct {
	Version, ClaimCeiling, HandoffID, CheckpointID, Durability string
	Counts                                                     PrivateAllModeCounts
	Nominations                                                []targetpacket.PrivateNomination
	Intents                                                    []targetpacket.PrivateSourceIntent
	Diagnostics                                                []PrivateAllModeDiagnostic
}
type privateAllModeCheckpoint struct {
	Version, Stage, Status, ClaimCeiling, HandoffID, ContractID, Workspace, Revision string
	Artifacts                                                                        []ArtifactRef
}
type privateAllModeMaterial struct {
	kind string
	raw  []byte
}

// Same retains process-local expected handoff identity, not a caller hash claim.
func (h PrivateAllModeHandoff) Same(other PrivateAllModeHandoff) bool {
	return h.id != "" && h.id == other.id && len(h.entries) > 0 && len(other.entries) == len(h.entries) && &h.entries[0] == &other.entries[0]
}

// PreparePrivateAllModeSourceIntents exposes only bounded metadata preparation
// for the upper producer bridge's pre-Put preflight. No source admission occurs.
func PreparePrivateAllModeSourceIntents(ctx context.Context, h PrivateAllModeHandoff, c ContinuationContract, store Store, workspace string) (PrivateAllModeResult, error) {
	out, _, err := preparePrivateAllMode(ctx, h, c, store, workspace)
	return out, err
}

func privateAllModeFailure(err error) Result {
	return Result{PrivateStage: "PRIVATE_SOURCE_INTENTS", PrivateCode: "PRIVATE_ALLMODE_REFUSED", Err: err}
}

// No evaluator, source supplier, Packet/Describe builder, or worker is reachable
// from this branch. Capture limits are only finite preparation ceilings here;
// this is not a funded production flow or a grant of source/runtime authority.
func preparePrivateAllMode(ctx context.Context, h PrivateAllModeHandoff, c ContinuationContract, store Store, workspace string) (PrivateAllModeResult, []privateAllModeMaterial, error) {
	bad := func(msg string) (PrivateAllModeResult, []privateAllModeMaterial, error) {
		return PrivateAllModeResult{}, nil, errors.New(msg)
	}
	if ctx == nil {
		return bad("private context required")
	}
	if err := ctx.Err(); err != nil {
		return bad(err.Error())
	}
	if h.id == "" || h.wire.Version != PrivateAllModeVersion || workspace != h.wire.Workspace {
		return bad("private expected handoff/contract/workspace required")
	}
	bs, ok := store.(boundedStore)
	if !ok || store == nil || bs.MaxObjectBytes() <= 0 {
		return bad("private finite store required")
	}
	limits := c.CaptureLimits()
	capBytes := bs.MaxObjectBytes()
	if capBytes > int64(limits.MaxArtifactBytes) {
		capBytes = int64(limits.MaxArtifactBytes)
	}
	if capBytes > rg.EvaluationMaxBytesPrivate {
		capBytes = rg.EvaluationMaxBytesPrivate
	}
	if capBytes <= 0 || limits.MaxWork <= 0 || limits.MaxBindings <= 0 || len(h.entries) != len(h.wire.Schedule) || len(h.entries) > privateAllModeMaxSlots {
		return bad("private finite continuation ceilings required")
	}
	// The legacy contract validator hashes its canonical wire. Bound every
	// variable-size scalar first; this private preflight changes no legacy wire.
	v := c.wire.SchemaVersions
	contractUpper := int64(4096)
	for _, s := range []string{c.wire.SchemaVersion, c.wire.ContractID, c.wire.ContinuationID, c.wire.ProfileID, c.wire.WorkerPinID, c.wire.PacketSourcePolicy.PolicyID, c.wire.ResponseVersion, v.Handoff, v.ProgramC, v.Snapshots, v.Packets, v.Requests, v.Worker, v.Catalog} {
		if len(s) > rg.EvaluationMaxStringPrivate {
			return bad("private contract scalar bound")
		}
		contractUpper += 6 * int64(len(s))
	}
	if contractUpper > capBytes {
		return bad("private contract byte preflight bound")
	}
	if c.Validate() != nil {
		return bad("private contract integrity mismatch")
	}
	// Count scalar opaque metadata BEFORE Partition/Nodes/Occurrences/Custody copy.
	work, nodes, upper := int64(len(h.entries)), int64(0), int64(16384)+contractUpper+6*int64(len(h.wire.Workspace)+len(h.wire.Revision))
	for _, e := range h.wire.Entries {
		b := e.Binding
		// Include output nominations and endpoint/gap intents in the pre-copy
		// charge. Custody plus two occurrence endpoints bounds fallback rows.
		nominationUpper := b.Communities
		if nominationUpper == 0 && e.Disposition == PrivateAllModeReady {
			nominationUpper = 1
		}
		intentUpper := b.Custody + 2*b.Occurrences + 1
		work += int64(b.Nodes) + int64(b.Occurrences)*2 + int64(b.Custody)*2 + int64(b.Communities) + int64(nominationUpper) + int64(intentUpper)
		nodes += int64(b.Nodes)
		// JSON escaping costs at most six bytes per input byte. Account for
		// repeated slot metadata separately; it is not in projection bytes.
		repeated := int64(b.Custody + b.Communities + 2*b.Occurrences + 2)
		metadata := int64(len(e.Slot.RunID) + len(e.Slot.SeedNamespace) + len(e.Lineage) + len(e.MissingReason) + len(b.CaseID) + len(b.SelectorID) + 2048)
		upper += int64(b.Projection.ByteLength)*24 + 6*repeated*metadata
		if work > int64(limits.MaxWork) || nodes > int64(limits.MaxBindings) || upper > capBytes {
			return bad("private aggregate preparation bound")
		}
	}
	out := PrivateAllModeResult{Version: PrivateAllModeVersion, ClaimCeiling: rg.EvaluationCeilingPrivate, HandoffID: h.id, Durability: "METADATA_ONLY;EXACT_EXPECTED_OPAQUE_HANDOFF_REQUIRED;NO_REPLAY_OR_SOURCE_ADMISSION"}
	out.Counts.Slots = len(h.entries)
	for i, e := range h.entries {
		if err := ctx.Err(); err != nil {
			return bad(err.Error())
		}
		binding := h.wire.Entries[i].Binding
		nomination := func(community int, members []string, endpoints []targetpacket.PrivateTypedEndpoint, gap string) error {
			n := targetpacket.PrivateNomination{Status: "PROVISIONAL_STRUCTURAL_HYPOTHESIS", ClaimCeiling: rg.EvaluationCeilingPrivate, RejectionCondition: "SPLIT_OR_WITHDRAW_IF_ADMITTED_SOURCE_REVIEW_CONTRADICTS_MEMBERSHIP", Mode: e.Slot.Mode, RunID: e.Slot.RunID, InputID: binding.Bindings.Input.ID, CaseID: binding.Case.ID, PolicyID: binding.Bindings.Policy.ID, ResultID: binding.Result.ID, Lineage: e.Lineage, Seed: e.Slot.Seed, Slot: i, Community: community, Members: members, Endpoints: endpoints, Gap: gap}
			// Slot ordinal distinguishes intentional repeated seeds without changing any
			// upstream invocation/result/partition identity.
			n.ID = digestOf([]byte(fmt.Sprintf("%s/%d/%d", h.id, i, community)))
			intents, err := targetpacket.BuildPrivateSourceIntents(n, 2*rg.EvaluationMaxOccurrencesPrivate+rg.EvaluationMaxNodesPrivate)
			if err != nil {
				return err
			}
			out.Nominations = append(out.Nominations, n)
			out.Intents = append(out.Intents, intents...)
			return nil
		}
		if e.Disposition == PrivateAllModeDeclaredMissing {
			out.Counts.Missing++
			out.Diagnostics = append(out.Diagnostics, PrivateAllModeDiagnostic{i, e.Slot.Mode, PrivateAllModeDeclaredMissing, e.Lineage})
			// Missing entries generate a gap intent, not a case/result nomination.
			n := targetpacket.PrivateNomination{ID: digestOf([]byte(fmt.Sprintf("%s/%d/missing", h.id, i))), Mode: e.Slot.Mode, RunID: e.Slot.RunID, Lineage: e.Lineage, Seed: e.Slot.Seed, Slot: i, Gap: PrivateAllModeDeclaredMissing}
			intents, err := targetpacket.BuildPrivateSourceIntents(n, 1)
			if err != nil {
				return bad(err.Error())
			}
			out.Intents = append(out.Intents, intents...)
			continue
		}
		out.Counts.Ready++
		p := e.Case.Projection()
		occurrences := p.Occurrences()
		custody := p.Custody()
		partition := e.Result.Partition()
		// All projection members, including inactive/singleton members, remain local
		// to this case. No global ordinal join or union membership is introduced.
		endpoints := make(map[string][]targetpacket.PrivateTypedEndpoint, binding.Nodes)
		type endpointKey struct{ occurrence, node, role string }
		supplied := make(map[endpointKey][]rg.EndpointCustody, len(custody))
		for _, u := range custody {
			key := endpointKey{u.OccurrenceIdentity, u.Endpoint, u.EndpointRole}
			supplied[key] = append(supplied[key], u)
		}
		for _, o := range occurrences {
			fromRole, toRole := "CALLER_OCCURRENCE", "CALLEE_TARGET"
			if o.Kind == "REFERENCES_SYMBOL" {
				fromRole, toRole = "REFERENCING_OCCURRENCE", "REFERENCED_SYMBOL"
			}
			if o.Kind == "RESOLVES_TO_DEFINITION" {
				fromRole, toRole = "QUERY_OCCURRENCE", "DEFINITION_TARGET"
			}
			for _, end := range []struct{ node, role string }{{o.From, fromRole}, {o.To, toRole}} {
				refs := supplied[endpointKey{o.OccurrenceIdentity, end.node, end.role}]
				if len(refs) == 0 {
					endpoints[end.node] = append(endpoints[end.node], targetpacket.PrivateTypedEndpoint{Node: end.node, Kind: o.Kind, Occurrence: o.OccurrenceIdentity, Role: end.role, Predecessor: o.SourcePredecessorIdentity, Disposition: o.TerminalDisposition, Ordinal: o.Ordinal})
				}
				for _, u := range refs {
					endpoints[end.node] = append(endpoints[end.node], targetpacket.PrivateTypedEndpoint{Node: end.node, Kind: o.Kind, Occurrence: o.OccurrenceIdentity, Role: end.role, Predecessor: u.PredecessorSelector, Custody: u.Custody, Disposition: o.TerminalDisposition, Ordinal: o.Ordinal})
				}
			}
		}
		if binding.ActiveOccurrences == 0 {
			code := "EMPTY_ACTIVE_PROJECTION_NOT_SOURCE_ABSENCE"
			if len(occurrences) > 0 && p.Accounting().Excluded == len(occurrences) {
				code = "POLICY_EXCLUDED_NOT_UPSTREAM_EMPTY"
				out.Counts.Excluded++
			} else {
				out.Counts.Empty++
			}
			out.Diagnostics = append(out.Diagnostics, PrivateAllModeDiagnostic{i, e.Slot.Mode, code, e.Lineage})
		}
		universe := p.NodeIdentities()
		seen := make(map[string]bool, len(universe))
		for _, node := range universe {
			seen[node] = false
		}
		for j, community := range partition.Communities {
			var refs []targetpacket.PrivateTypedEndpoint
			for _, node := range community.Members {
				used, exists := seen[node]
				if !exists || used {
					return bad("private partition universe mismatch")
				}
				seen[node] = true
				refs = append(refs, endpoints[node]...)
			}
			if err := nomination(j, community.Members, refs, ""); err != nil {
				return bad(err.Error())
			}
		}
		for _, used := range seen {
			if !used {
				return bad("private partition member missing")
			}
		}
		if len(partition.Communities) == 0 {
			if err := nomination(-1, nil, nil, "EMPTY_ACTIVE_PROJECTION_NOT_SOURCE_ABSENCE"); err != nil {
				return bad(err.Error())
			}
		}
	}
	out.Counts.Nominations = len(out.Nominations)
	out.Counts.Intents = len(out.Intents)
	// Hard aggregate bounds precede serialization and Store.Put. The conservative
	// projection-derived byte estimate above precedes large defensive copies.
	if int64(len(out.Intents)+len(out.Nominations)) > int64(limits.MaxWork) {
		return bad("private output count bound")
	}
	cb, err := c.Bytes()
	if err != nil {
		return bad(err.Error())
	}
	hb, err := json.Marshal(h.wire)
	if err != nil {
		return bad(err.Error())
	}
	ob, err := json.Marshal(out)
	if err != nil {
		return bad(err.Error())
	}
	mats := []privateAllModeMaterial{{"private_contract", cb}, {"private_handoff", hb}, {"private_source_intents", ob}}
	cp := privateAllModeCheckpoint{Version: PrivateAllModeVersion, Stage: string(StopAfterSourceIntents), Status: string(StatusPaused), ClaimCeiling: rg.EvaluationCeilingPrivate, HandoffID: h.id, ContractID: c.ID(), Workspace: h.wire.Workspace, Revision: h.wire.Revision}
	total := int64(0)
	for _, m := range mats {
		if len(m.raw) == 0 || int64(len(m.raw)) > capBytes {
			return bad("private object byte bound")
		}
		total += int64(len(m.raw))
		cp.Artifacts = append(cp.Artifacts, ArtifactRef{Kind: m.kind, ID: digestOf(m.raw), Digest: digestOf(m.raw), ByteLength: uint64(len(m.raw))})
	}
	raw, err := json.Marshal(cp)
	if err != nil || int64(len(raw)) > capBytes {
		return bad("private checkpoint bound")
	}
	total += int64(len(raw))
	if total > int64(limits.MaxParentBytes) {
		return bad("private aggregate stored bytes bound")
	}
	mats = append(mats, privateAllModeMaterial{"private_checkpoint", raw})
	out.CheckpointID = digestOf(raw)
	return out, mats, nil
}

func privateAllModePaused(out PrivateAllModeResult) Result {
	return Result{Status: StatusPaused, HandoffID: out.HandoffID, CheckpointID: out.CheckpointID, PrivateStage: string(StopAfterSourceIntents), PrivateAllMode: &out}
}
func runPrivateAllMode(ctx context.Context, req Request) Result {
	if req.Handoff.HandoffID() != "" || req.StopAfter != StopAfterSourceIntents {
		return privateAllModeFailure(errors.New("exclusive private handoff and source-intents stop required"))
	}
	out, mats, err := preparePrivateAllMode(ctx, *req.PrivateAllMode, req.Contract, req.Store, req.Workspace)
	if err != nil {
		return privateAllModeFailure(err)
	}
	for _, m := range mats {
		if err = ctx.Err(); err != nil {
			return privateAllModeFailure(err)
		}
		id, putErr := req.Store.Put(ctx, m.raw)
		if putErr != nil {
			return privateAllModeFailure(errors.New("private immutable store Put failed"))
		}
		if id != digestOf(m.raw) {
			return privateAllModeFailure(errors.New("private store content identity mismatch"))
		}
	}
	return privateAllModePaused(out)
}
func resumePrivateAllMode(ctx context.Context, req ResumeRequest) Result {
	r := req.PrivateAllMode
	if r.Version != PrivateAllModeVersion || req.StopAfter != StopAfterSourceIntents || !validDigest(req.Selector) || req.UseCommittedWorkspace {
		return privateAllModeFailure(errors.New("private resume version/stop/expected workspace mismatch"))
	}
	out, mats, err := preparePrivateAllMode(ctx, r.Expected, r.Contract, req.Store, req.Workspace)
	if err != nil {
		return privateAllModeFailure(err)
	}
	if req.Selector != out.CheckpointID {
		return privateAllModeFailure(errors.New("private expected checkpoint mismatch"))
	}
	// Verify every reference, byte length and content digest against freshly
	// derived metadata from the exact expected opaque objects. No deserialized
	// result is admitted; caller-resealed outcomes cannot manufacture admission.
	for _, m := range mats {
		if err = ctx.Err(); err != nil {
			return privateAllModeFailure(err)
		}
		raw, getErr := req.Store.Get(ctx, digestOf(m.raw))
		if getErr != nil {
			return privateAllModeFailure(errors.New("private immutable store Get failed"))
		}
		if len(raw) != len(m.raw) || !bytes.Equal(raw, m.raw) {
			return privateAllModeFailure(errors.New("private persisted artifact mismatch"))
		}
	}
	return privateAllModePaused(out)
}
