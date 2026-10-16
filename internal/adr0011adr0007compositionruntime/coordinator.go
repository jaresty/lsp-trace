package adr0011adr0007compositionruntime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	adr0007 "lsp-trace/internal/adr0007sourcetextsearchv4private"
	composition "lsp-trace/internal/adr0011adr0007composition"
	"lsp-trace/internal/directionalraw"
)

var ErrBehaviorNotImplementedPrivate = errors.New("adr0011/adr0007 behavior not implemented")

type FacetObservationPrivate struct{ Facet, ObservationState string }
type FacetDispositionPrivate struct {
	Facet, State                                              string
	Count                                                     uint64
	FailureFamily, FailureCode, Completeness, FeatureIdentity string
}
type CoordinateRequestPrivate struct {
	Attempt    adr0007.Attempt
	RawAttempt []byte
	Envelopes  []composition.DirectionEnvelope
	Marks      []composition.DependencyMark
	Facets     []FacetObservationPrivate
	Root       directionalraw.BidirectionalRootPrivate
}
type LeaseStatePrivate struct {
	IncomingRetained, OutgoingRetained, Released                                                                                                               bool
	RootReleaseCount, IncomingLive, OutgoingLive, RootLive, IncomingAdmitted, OutgoingAdmitted, RootAdmitted, IncomingReleased, OutgoingReleased, RootReleased uint64
}
type CandidateSuppressionPrivate struct{ Code, Stage, Direction, Family string }

type coordinateResultSnapshotPrivate struct {
	Composition                                              composition.Result
	Candidate                                                *composition.CompositionCandidate
	Refusal                                                  *composition.Failure
	Suppression                                              *CandidateSuppressionPrivate
	LeaseState                                               LeaseStatePrivate
	Accepted                                                 bool
	RawAttemptSHA256, ParsedAttemptSHA256, TerminalRawSHA256 string
	Facets                                                   []FacetDispositionPrivate
}

type CoordinateResultPrivate struct {
	composition                                              composition.Result
	candidate                                                *composition.CompositionCandidate
	refusal                                                  *composition.Failure
	suppression                                              *CandidateSuppressionPrivate
	leaseState                                               LeaseStatePrivate
	accepted                                                 bool
	rawAttemptSHA256, parsedAttemptSHA256, terminalRawSHA256 string
	facets                                                   []FacetDispositionPrivate
}

func deepCloneJSONPrivate[T any](in T) T {
	var out T
	raw, err := json.Marshal(in)
	if err != nil {
		panic(err)
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		panic(err)
	}
	return out
}
func newCoordinateResultPrivate(s coordinateResultSnapshotPrivate) CoordinateResultPrivate {
	s = deepCloneJSONPrivate(s)
	return CoordinateResultPrivate{s.Composition, s.Candidate, s.Refusal, s.Suppression, s.LeaseState, s.Accepted, s.RawAttemptSHA256, s.ParsedAttemptSHA256, s.TerminalRawSHA256, s.Facets}
}
func NewSuppressedCoordinateResultPrivate(s CandidateSuppressionPrivate) CoordinateResultPrivate {
	return newCoordinateResultPrivate(coordinateResultSnapshotPrivate{Suppression: &s})
}
func (r CoordinateResultPrivate) snapshotPrivate() coordinateResultSnapshotPrivate {
	return deepCloneJSONPrivate(coordinateResultSnapshotPrivate{r.composition, r.candidate, r.refusal, r.suppression, r.leaseState, r.accepted, r.rawAttemptSHA256, r.parsedAttemptSHA256, r.terminalRawSHA256, r.facets})
}
func (r CoordinateResultPrivate) MarshalJSON() ([]byte, error) {
	return json.Marshal(r.snapshotPrivate())
}
func (r CoordinateResultPrivate) CompositionPrivate() composition.Result {
	return r.snapshotPrivate().Composition
}
func (r CoordinateResultPrivate) CandidatePrivate() *composition.CompositionCandidate {
	return r.snapshotPrivate().Candidate
}
func (r CoordinateResultPrivate) RefusalPrivate() *composition.Failure {
	return r.snapshotPrivate().Refusal
}
func (r CoordinateResultPrivate) SuppressionPrivate() *CandidateSuppressionPrivate {
	return r.snapshotPrivate().Suppression
}
func (r CoordinateResultPrivate) LeaseStatePrivate() LeaseStatePrivate { return r.leaseState }
func (r CoordinateResultPrivate) AcceptedPrivate() bool                { return r.accepted }
func (r CoordinateResultPrivate) RawAttemptSHA256Private() string      { return r.rawAttemptSHA256 }
func (r CoordinateResultPrivate) ParsedAttemptSHA256Private() string   { return r.parsedAttemptSHA256 }
func (r CoordinateResultPrivate) TerminalRawSHA256Private() string     { return r.terminalRawSHA256 }
func (r CoordinateResultPrivate) FacetsPrivate() []FacetDispositionPrivate {
	return append([]FacetDispositionPrivate(nil), r.facets...)
}
func (r *CoordinateResultPrivate) SetLeaseStatePrivate(state LeaseStatePrivate) { r.leaseState = state }

type CoordinatorPrivate struct{}

func NewCoordinatorPrivate() (*CoordinatorPrivate, error) { return &CoordinatorPrivate{}, nil }
func leasePrivate(root directionalraw.BidirectionalRootPrivate) LeaseStatePrivate {
	s, _ := root.SnapshotPrivate()
	return LeaseStatePrivate{s.IncomingLiveBytes > 0, s.OutgoingLiveBytes > 0, s.Released, s.ReleaseCount, s.IncomingLiveBytes, s.OutgoingLiveBytes, s.LiveBytes, s.IncomingAdmittedBytes, s.OutgoingAdmittedBytes, s.RootAdmittedBytes, s.IncomingReleasedBytes, s.OutgoingReleasedBytes, s.RootReleasedBytes}
}
func (c *CoordinatorPrivate) CoordinatePrivate(ctx context.Context, req CoordinateRequestPrivate) (CoordinateResultPrivate, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	release := func(out coordinateResultSnapshotPrivate, prior error) (CoordinateResultPrivate, error) {
		releaseErr := req.Root.Release()
		out.LeaseState = leasePrivate(req.Root)
		if releaseErr != nil {
			out.Accepted = false
			out.Candidate = nil
			out.Suppression = &CandidateSuppressionPrivate{"TERMINAL_RELEASE_FAILED", "root_release", "", "GENERIC"}
			return newCoordinateResultPrivate(out), errors.Join(prior, releaseErr)
		}
		return newCoordinateResultPrivate(out), prior
	}
	facets, err := ValidateFacetsPrivate(req.Facets)
	if err != nil {
		return release(coordinateResultSnapshotPrivate{}, err)
	}
	parsed, parseErr := adr0007.StrictParseAttempt(req.RawAttempt)
	if parseErr != "" || !bytes.Equal(adr0007.Canon(parsed), adr0007.Canon(req.Attempt)) {
		return release(coordinateResultSnapshotPrivate{Suppression: &CandidateSuppressionPrivate{"ATTEMPT_MISMATCH", "typed_raw_pair", "", "GENERIC"}, Facets: facets}, nil)
	}
	rawDigest := adr0007.Digest(req.RawAttempt)
	parsedDigest := adr0007.Digest(adr0007.Canon(parsed))
	_ = adr0007.EvaluateRaw(req.RawAttempt)
	result, err := composition.Compose(ctx, composition.Request{Attempt: parsed, RawAttempt: req.RawAttempt, Composition: &composition.CompositionInput{Envelopes: req.Envelopes, Marks: req.Marks}})
	out := coordinateResultSnapshotPrivate{Composition: result, Candidate: result.CompositionCandidate, Refusal: result.Failure, Accepted: result.Failure == nil && result.CompositionCandidate != nil, RawAttemptSHA256: rawDigest, ParsedAttemptSHA256: parsedDigest, TerminalRawSHA256: rawDigest, LeaseState: leasePrivate(req.Root), Facets: facets}
	if result.Failure != nil {
		out.Suppression = &CandidateSuppressionPrivate{result.Failure.Code, result.Failure.Stage, result.Failure.Direction, "GENERIC"}
	}
	return release(out, err)
}
func CoordinatePrivate(ctx context.Context, req CoordinateRequestPrivate) (CoordinateResultPrivate, error) {
	c, _ := NewCoordinatorPrivate()
	return c.CoordinatePrivate(ctx, req)
}
func ValidateFacetsPrivate(in []FacetObservationPrivate) ([]FacetDispositionPrivate, error) {
	const requiredFacetCount = 4
	if len(in) != requiredFacetCount {
		return nil, fmt.Errorf("facets: require exactly %d records: got %d", requiredFacetCount, len(in))
	}
	validFacets := map[string]bool{"D": true, "R": true, "CALLS_INCOMING": true, "CALLS_OUTGOING": true}
	validStates := map[string]bool{"OBSERVED_NONEMPTY": true, "OBSERVED_EMPTY": true, "UNAVAILABLE": true}
	seen := make(map[string]bool, requiredFacetCount)
	out := make([]FacetDispositionPrivate, len(in))
	for i, v := range in {
		if !validFacets[v.Facet] {
			return nil, fmt.Errorf("facets: unknown facet %q", v.Facet)
		}
		if !validStates[v.ObservationState] {
			return nil, fmt.Errorf("facets: unknown observation state %q for %s", v.ObservationState, v.Facet)
		}
		if seen[v.Facet] {
			return nil, fmt.Errorf("facets: duplicate facet %q", v.Facet)
		}
		seen[v.Facet] = true
		d := FacetDispositionPrivate{Facet: v.Facet, State: v.ObservationState, Completeness: "UNKNOWN", FeatureIdentity: "UNRESOLVED"}
		switch v.ObservationState {
		case "OBSERVED_NONEMPTY":
			d.Count = 1
		case "UNAVAILABLE":
			switch v.Facet {
			case "D":
				d.FailureFamily, d.FailureCode = "PROVIDER_ONLY", "DEFINITION_PROVIDER_UNAVAILABLE"
			case "R":
				d.FailureFamily, d.FailureCode = "MIXED_PROVIDER_GENERIC", "REFERENCES_MIXED_PROVIDER_GENERIC_UNAVAILABLE"
			case "CALLS_INCOMING":
				d.FailureFamily, d.FailureCode = "GENERIC", "CALLS_INCOMING_UNAVAILABLE"
			case "CALLS_OUTGOING":
				d.FailureFamily, d.FailureCode = "GENERIC", "CALLS_OUTGOING_UNAVAILABLE"
			}
		}
		out[i] = d
	}
	for facet := range validFacets {
		if !seen[facet] {
			return nil, fmt.Errorf("facets: missing facet %q", facet)
		}
	}
	return out, nil
}
