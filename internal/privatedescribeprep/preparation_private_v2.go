package privatedescribeprep

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"unicode/utf8"

	"lsp-trace/internal/acquisition"
	"lsp-trace/internal/censusacquisition"
	pilot "lsp-trace/internal/censuslineagepilot"
	"lsp-trace/internal/graphprovenance"
	derived "lsp-trace/internal/retainedpartialprogramc"
)

const RequestTypePrivateV2 = "PRIVATE_TARGET_INTERPRETATION_PREPARATION_V2"

// InputPrivateV2 requires actual immutable capabilities, not decoded diagnostics.
// Targets selects original own-batch labels; ordering always follows acquisition.
type InputPrivateV2 struct {
	Projection2 derived.ProjectionPrivateV2
	Assessment2 derived.AssessmentPrivateV2
	Targets     []string
}

// CoveragePrivateV2 distinguishes the per-batch denominator from the original
// discovery snapshot. Repeated origins are NOT additional independent sources.
type CoveragePrivateV2 struct {
	Source                                                                               derived.SourceBindingPrivateV2
	AssessmentSHA256                                                                     string
	SnapshotTargetCount, SnapshotSymbolCount, BatchTargetCount, BatchSelectedTargetCount int
	OriginID                                                                             string
	Independence                                                                         string
}

type preparedInputPrivateV2 struct {
	id             string
	evidence       derived.ProjectionEvidencePrivateV2
	acquisition    acquisition.Result
	discovery      censusacquisition.Discovery
	batchDiscovery []censusacquisition.PreparedTarget
	view           preparationView
	assessmentSHA  string
	discoverySHA   string
	originID       string
}

// This is the sole V2 neutral-view constructor. Sealed pair identity and retained
// partition metrics are validated before parsing own retained bytes or AST work.
func validatedPreparationInputPrivateV2(ctx context.Context, in InputPrivateV2) (preparedInputPrivateV2, error) {
	raw := in.Assessment2.Bytes()
	if err := derived.ValidateInstabilityPrivateV2(ctx, in.Projection2, in.Assessment2, raw); err != nil {
		return preparedInputPrivateV2{}, err
	}
	e, err := in.Projection2.PreparationEvidencePrivateV2()
	if err != nil {
		return preparedInputPrivateV2{}, err
	}
	c, err := graphprovenance.ParseSourceWithholdingPrivateV2(e.SourceArtifact)
	if err != nil {
		return preparedInputPrivateV2{}, err
	}
	packet, err := pilot.PureParseProducerPacketPrivateV2(e.Packet)
	if err != nil {
		return preparedInputPrivateV2{}, err
	}
	s := e.Source
	if s.BatchOrdinal < 0 || s.BatchOrdinal >= len(packet.Batches) || pilot.Digest(e.Packet) != s.PacketSHA256 || pilot.Digest(e.QualificationReport) != s.QualificationReportSHA256 || pilot.Digest(e.SourceArtifact) != s.SourceArtifactSHA256 || !bytes.Equal(c.Evidence.GraphBytes, e.OriginalNativeV3) || !bytes.Equal(packet.Batches[s.BatchOrdinal].SourceArtifact, e.SourceArtifact) {
		return preparedInputPrivateV2{}, fmt.Errorf("private V2 own retained pins")
	}
	var origin struct {
		Enumeration json.RawMessage                  `json:"actual_enumeration"`
		Batches     []censusacquisition.BatchRequest `json:"target_batches"`
	}
	var d censusacquisition.Discovery
	if json.Unmarshal(packet.EnumerationOrigin, &origin) != nil || json.Unmarshal(origin.Enumeration, &d) != nil || len(d.Targets) != s.SnapshotTargetCount || len(c.Evidence.Acquisition.Targets) != s.BatchTargetCount {
		return preparedInputPrivateV2{}, fmt.Errorf("private V2 actual discovery")
	}
	// The sealed packet retains the actual planned batch selection. Use that exact
	// own ordinal/seeds, not a name/position heuristic or the whole denominator.
	// This is discovery metadata, never AST/full-definition qualification.
	if s.BatchOrdinal >= len(origin.Batches) || origin.Batches[s.BatchOrdinal].Ordinal != s.BatchOrdinal || !bytes.Equal(origin.Batches[s.BatchOrdinal].CanonicalSeedsV2, e.CanonicalSeeds) || len(origin.Batches[s.BatchOrdinal].Targets) != len(c.Evidence.Acquisition.Targets) {
		return preparedInputPrivateV2{}, fmt.Errorf("private V2 own batch discovery selection binding")
	}
	batchDiscovery := origin.Batches[s.BatchOrdinal].Targets
	// Packet file bytes are metadata/checks only: never a source-body fallback.
	for _, m := range c.Policy.Members {
		found := false
		for _, f := range packet.Files {
			if f.Record.URI == m.URI && f.Record.SHA256 == m.SHA256 && f.Record.Role == m.Role {
				found = true
			}
		}
		if !found {
			return preparedInputPrivateV2{}, fmt.Errorf("private V2 member metadata")
		}
	}
	id := digest(struct {
		RequestType string
		Source      derived.SourceBindingPrivateV2
	}{RequestTypePrivateV2, s})
	a := in.Assessment2.Report()
	v := preparationView{captures: c.Evidence.Captures, math: in.Projection2.Math(), assessment: assessmentRows{Outcome: a.Outcome, RequestSHA256: a.RequestSHA256, Digest: a.Digest, ExpectedRuns: a.ExpectedRuns, ExpectedPairs: a.ExpectedPairs, IndependentSources: a.IndependentSources, Runs: a.Runs, Pairs: a.Pairs}}
	for _, m := range c.Policy.Members {
		v.members = append(v.members, sourceMember{m.URI, m.SHA256, m.Role})
	}
	// Complete report includes all run statuses, partitions/qualified bases and all
	// pair metrics. It is indivisible: a budget gap withdraws the target, not rows.
	v.assessment.pins = []Record{
		record("SOURCE_BINDING_PRIVATE_V2", id+"/source-binding", s),
		record("ASSESSMENT_PRIVATE_V2", id+"/assessment/"+pilot.Digest(raw), a),
		record("DISCOVERY_PRIVATE_V2", id+"/discovery", struct {
			Snapshot            censusacquisition.Discovery
			OwnBatchTargets     []censusacquisition.PreparedTarget
			CanonicalBatchSeeds []byte
			CaptureOriginSHA256 string
		}{d, batchDiscovery, e.CanonicalSeeds, s.CaptureOriginSHA256}),
		record("SOURCE_CLAIM", id+"/source-claim", "AST_NOT_FULL_DEFINITION_NOT_METHODRESULT; native identifiers/kinds/ranges unchanged; source comments untrusted; context never nomination; absence prohibited"),
	}
	// Preserve original native bytes and request order/duplicates/null-vs-array.
	// These are explicitly batch-global metadata, not target-owned observations.
	v.assessment.pins = append(v.assessment.pins, record("BATCH_ORIGINAL_NATIVE_V3", id+"/native-v3", e.OriginalNativeV3))
	for ordinal, req := range c.Evidence.Acquisition.Requests {
		v.assessment.pins = append(v.assessment.pins, record("BATCH_REQUEST_METADATA", fmt.Sprintf("%s/batch-request/%d/%s", id, ordinal, req.ID), struct {
			Record                           acquisition.RequestRecord
			OriginalParams, OriginalResponse []byte
			Ownership                        string
		}{req, []byte(req.Params), []byte(req.Response), "BATCH_GLOBAL_NOT_TARGET_OWNED; own expansion linkage required"}))
	}
	return preparedInputPrivateV2{id: id, evidence: e, acquisition: c.Evidence.Acquisition, discovery: d, batchDiscovery: batchDiscovery, view: v, assessmentSHA: pilot.Digest(raw), discoverySHA: pilot.Digest(origin.Enumeration), originID: pilot.Digest(packet.EnumerationOrigin)}, nil
}

// PreparePrivateV2 performs no IO, provider calls, Leiden reruns, ordinary
// admission, LLM inference, or publication. Result is detached inspection only.
func PreparePrivateV2(ctx context.Context, inputs []InputPrivateV2, limits Limits) (Result, error) {
	if ctx == nil || !limits.valid() || len(inputs) == 0 || len(inputs) > 64 {
		return Result{}, fmt.Errorf("invalid private V2 preparation invocation")
	}
	remaining := limits.MaxTotalBytes
	for _, in := range inputs {
		if len(in.Targets) > 64 {
			return Result{}, fmt.Errorf("private V2 selection count bound")
		}
		for _, label := range in.Targets {
			if len(label) == 0 || len(label) > 1024 || len(label) > remaining || !utf8.ValidString(label) {
				return Result{}, fmt.Errorf("private V2 selection bytes")
			}
			remaining -= len(label)
		}
	}
	out := Result{RequestType: RequestTypePrivateV2, Limits: limits, FeatureIdentity: "UNRESOLVED", SourceCompleteness: "UNKNOWN"}
	seen := map[string]bool{}
	for _, in := range inputs {
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}
		v, err := validatedPreparationInputPrivateV2(ctx, in)
		if err != nil {
			return Result{}, err
		}
		if seen[v.id] {
			return Result{}, fmt.Errorf("duplicate exact private V2 batch namespace")
		}
		seen[v.id] = true
		a := v.acquisition
		if len(a.Targets) > 64 || len(a.Graph.Nodes) > 10000 || len(v.view.math.Occurrences) > 100000 || len(out.Index) > 4096-len(a.Targets) {
			return Result{}, fmt.Errorf("private V2 finite input/index bound")
		}
		wanted := map[string]bool{}
		for _, label := range in.Targets {
			if wanted[label] {
				return Result{}, fmt.Errorf("duplicate private V2 target selection")
			}
			wanted[label] = true
		}
		cv := Coverage{InputSHA256: v.id, OriginalTargets: len(a.Targets), OriginalSymbols: len(v.batchDiscovery), DiscoverySHA256: v.discoverySHA, PrivateV2: &CoveragePrivateV2{Source: v.evidence.Source, AssessmentSHA256: v.assessmentSHA, SnapshotTargetCount: v.evidence.Source.SnapshotTargetCount, SnapshotSymbolCount: len(v.discovery.EnumerationSymbols), BatchTargetCount: len(a.Targets), OriginID: v.originID, Independence: "SAME_CAPTURE_ORIGIN_NOT_INDEPENDENT_SOURCE"}}
		for _, t := range a.Targets {
			if len(in.Targets) == 0 || wanted[t.Requested.ID] {
				cv.Selection = append(cv.Selection, t.Requested.ID)
				delete(wanted, t.Requested.ID)
			}
		}
		if len(wanted) != 0 {
			return Result{}, fmt.Errorf("unknown private V2 target selection")
		}
		cv.Requested = len(cv.Selection)
		cv.PrivateV2.BatchSelectedTargetCount = cv.Requested
		scope := digest(struct {
			Input     string
			Selection []string
			Original  int
		}{v.id, cv.Selection, cv.OriginalTargets})
		for ordinal, t := range a.Targets {
			if !slices.Contains(cv.Selection, t.Requested.ID) {
				continue
			}
			if err := ctx.Err(); err != nil {
				return Result{}, err
			}
			ix := TargetIndex{InputSHA256: v.id, Target: t.Requested.ID, OriginalOrdinal: ordinal}
			if len(out.Index) >= limits.MaxTargets {
				ix.Status = "RESOURCE_GAP"
				ix.Gap = "MAX_TARGETS_UNTOUCHED"
				cv.ResourceGap++
				out.Index = append(out.Index, ix)
				continue
			}
			// A resolved context endpoint cannot become a nominated interpretation target.
			if t.Resolution.Identity != nil {
				for _, m := range v.view.members {
					if m.URI == t.Resolution.Identity.URI && m.Role != "NOMINATION" {
						return Result{}, fmt.Errorf("private V2 context target prohibited")
					}
				}
			}
			records, node, gap := targetRecordsPrivate(v.id, scope, ordinal, t, a, v.view, limits)
			if err := ctx.Err(); err != nil {
				return Result{}, err
			}
			if gap != "" {
				ix.Status = "SOURCE_GAP"
				ix.Gap = gap
				if gap == "SOURCE_BYTE_RESOURCE_LIMIT" || gap == "RECORD_RESOURCE_LIMIT" {
					ix.Status = "RESOURCE_GAP"
					cv.ResourceGap++
				} else {
					cv.SourceGap++
				}
			} else {
				p := Packet{InputSHA256: v.id, Target: t.Requested.ID, NodeID: node, ScopeSHA256: scope, OriginalOrdinal: ordinal}
				p.ID = digest(struct {
					Input, Target, Scope, Assessment string
					Limits                           Limits
				}{v.id, p.Target, scope, v.assessmentSHA, limits})
				parts, ok := splitPrivate(RequestTypePrivateV2, p, records, limits, out.PartCount, out.PartBytes)
				if !ok {
					ix.Status = "RESOURCE_GAP"
					ix.Gap = "INDIVISIBLE_RECORD_OR_PART_TOTAL_BUDGET"
					cv.ResourceGap++
				} else {
					p.Parts = parts
					for _, part := range parts {
						b, _ := json.Marshal(part)
						p.PartSHA256 = append(p.PartSHA256, pilot.Digest(b))
						out.PartBytes += len(b)
						out.PartCount++
					}
					out.Packets = append(out.Packets, p)
					ix.PacketID = p.ID
					ix.Status = "PREPARED"
					cv.Prepared++
				}
			}
			out.Index = append(out.Index, ix)
		}
		out.Coverage = append(out.Coverage, cv)
	}
	return boundCanonicalResult(out, limits)
}
