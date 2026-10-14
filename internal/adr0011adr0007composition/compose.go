package adr0011adr0007composition

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"

	adr0007 "lsp-trace/internal/adr0007sourcetextsearchv4private"
)

const (
	SchemaVersion = "lsp-trace.adr0011-adr0007.private-wrapper-composition.v1"

	AcceptedRepositoryHEAD                         = "336ec84b4688d456813861abcb45ddffaacdd6c4"
	AcceptedBidirectionalPackageChecksumSHA256     = "041247bf62ec7b981584971bcaafa24d29081f730fbb2ef55284b73d0db192b2"
	AcceptedBidirectionalCompositionArtifactSHA256 = "07212f4175aeb115dda5828761b3338b56233704673e3a3bbf861373f174e3eb"

	IncomingProfileID        = "CALLS_GO_GOPLS_INCOMING_EXACT_V1"
	IncomingGenericProfileID = "CALLS_LSP_INCOMING_EXACT_V1"
	IncomingEvidenceSHA256   = "e9c8cce4b77dc91a8ed435166fbf88ce9d6e12bbc28bd4ad0db4fbbb45cc4154"
	IncomingPackageSHA256    = "e0b1a9ea0b24e51768fad3bd89194ac50023dab8e370e6cc9a96ff6d78135abf"
	IncomingManifestSHA256   = "675456686a9b073b4462685ad833456ae02942b659f6ecc70bc8504dab401579"

	OutgoingProfileID        = "CALLS_GO_GOPLS_OUTGOING_EXACT_V1"
	OutgoingGenericProfileID = "CALLS_LSP_OUTGOING_EXACT_V1"
	OutgoingEvidenceSHA256   = "e6a60c88f4bd00d2e220d160765c8ee903903523cb86cfb7d814e31a7c433c9f"
	OutgoingPackageSHA256    = "411e2195723fc5ac5bb6a7af27331b187ca016d0d75767b634454223e5978e80"
	OutgoingManifestSHA256   = "7fd171adce13cda709193365fac10c34566a3472e093f347650f2cbf336fa543"

	OrdinalSemanticsNoSelectedItems      = "ONE_FINAL_DISPOSITION_PER_TARGET_ORDINAL_NO_SELECTED_ITEMS_EXPOSED"
	CanonicalOneFinalDispositionOrdinals = 5900
	IncomingOrdinalIdentitySHA256        = "273d62e8e91d0b332ada4e1d18e68ef4404f549fed14923879a3e15a8928677d"
	OutgoingOrdinalIdentitySHA256        = "7843f1430e5ddb401c8f8ce76eb3a1b3188769ff9f625ee6e6a10b9b7d1544e4"
)

const (
	StatusSuccess = "SUCCESS"
	StatusFailure = "FAILURE"
	ActionOmit    = "OMIT"
	ActionOrder   = "ORDER"
)

type Limits struct {
	MaxEnvelopes   int
	MaxMarks       int
	MaxReferences  int
	MaxStringBytes int
	MaxWork        uint64
}

type Request struct {
	Attempt     adr0007.Attempt
	RawAttempt  []byte
	Composition *CompositionInput
	Limits      Limits
}

type CompositionInput struct {
	Envelopes []DirectionEnvelope
	Marks     []DependencyMark
}

type DirectionEnvelope struct {
	Direction                        string
	Status                           string
	ProfileID                        string
	GenericProfileID                 string
	EvidenceSHA256                   string
	ProviderPackageChecksumSHA256    string
	ProviderManifestSHA256           string
	CompositionPackageChecksumSHA256 string
	CompositionArtifactSHA256        string
	OrdinalSemantics                 string
	OneFinalDispositionOrdinals      uint64
	OrdinalIdentitySHA256            string
	SelectedOrdinals                 []uint64
	FailureCode                      string
	FailureStage                     string
}

type DependencyMark struct {
	Direction                   string
	MatchID                     string
	Action                      string
	CandidateOrdinal            uint64
	OneFinalDispositionOrdinals uint64
	OrdinalIdentitySHA256       string
	SelectedOrdinal             uint64
	RelationKind                string
	ProfileID                   string
	EvidenceSHA256              string
}

type Result struct {
	ADR0007Terminal        adr0007.TerminalResult
	ADR0007CanonicalBytes  []byte
	ADR0007CanonicalSHA256 string
	DirectionDispositions  []DirectionDisposition
	CompositionCandidate   *CompositionCandidate
	Failure                *Failure
	Accounting             Accounting
}

type DirectionDisposition struct {
	Direction                        string
	Status                           string
	ProfileID                        string
	GenericProfileID                 string
	EvidenceSHA256                   string
	ProviderPackageChecksumSHA256    string
	ProviderManifestSHA256           string
	CompositionPackageChecksumSHA256 string
	CompositionArtifactSHA256        string
	OrdinalSemantics                 string
	OneFinalDispositionOrdinals      uint64
	OrdinalIdentitySHA256            string
	SelectedOrdinals                 []uint64
	FailureCode                      string
	FailureStage                     string
}

type CompositionCandidate struct {
	SchemaVersion           string
	Operation               string
	CandidateOnly           bool
	ExecutedLocation        bool
	AdmissionDigest         string
	BaselineCandidateDigest string
	BaselineMatchIDs        []string
	MemberMatchIDs          []string
	DirectionDispositions   []DirectionDisposition
	AppliedMarks            []AppliedMark
	Authority               int
	Accepted                bool
	Completeness            string
	FeatureIdentity         string
	CandidateDigest         string
}

type AppliedMark struct {
	Direction                   string
	MatchID                     string
	Action                      string
	CandidateOrdinal            uint64
	OneFinalDispositionOrdinals uint64
	OrdinalIdentitySHA256       string
	SelectedOrdinal             uint64
	ProfileID                   string
	EvidenceSHA256              string
}

type Accounting struct {
	Envelopes  uint64
	Marks      uint64
	References uint64
	Work       uint64
}

type Failure struct {
	Code      string
	Stage     string
	Direction string
	Reason    string
}

func Compose(ctx context.Context, req Request) (Result, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return cancelledResult(req, err), err
	}
	raw := append([]byte(nil), req.RawAttempt...)
	if raw == nil {
		raw = adr0007.Canon(req.Attempt)
	}
	terminal := adr0007.Evaluate(req.Attempt, append([]byte(nil), raw...))
	canonical := adr0007.Canon(terminal)
	result := Result{ADR0007Terminal: terminal, ADR0007CanonicalBytes: append([]byte(nil), canonical...), ADR0007CanonicalSHA256: digest(canonical)}
	if err := ctx.Err(); err != nil {
		result.Failure = &Failure{Code: "CANCELLED", Stage: "post_adr0007", Reason: err.Error()}
		return result, err
	}
	if req.Composition == nil {
		return result, nil
	}
	limits := withDefaultLimits(req.Limits)
	candidate, dispositions, accounting, failure := composeCandidate(ctx, terminal, *req.Composition, limits)
	result.DirectionDispositions = dispositions
	result.Accounting = accounting
	if failure != nil {
		result.Failure = failure
		return result, nil
	}
	result.CompositionCandidate = candidate
	return result, nil
}

func composeCandidate(ctx context.Context, terminal adr0007.TerminalResult, input CompositionInput, limits Limits) (*CompositionCandidate, []DirectionDisposition, Accounting, *Failure) {
	accounting := Accounting{Envelopes: uint64(len(input.Envelopes)), Marks: uint64(len(input.Marks))}
	if err := ctx.Err(); err != nil {
		return nil, nil, accounting, &Failure{Code: "CANCELLED", Stage: "pre_effect", Reason: err.Error()}
	}
	if terminal.Terminal != "COMPLETE" || terminal.RangeUnionCandidate == nil {
		return nil, nil, accounting, &Failure{Code: "ADR0007_NON_POSITIVE", Stage: "adr0007_terminal", Reason: terminal.Terminal}
	}
	if len(input.Envelopes) != 2 {
		return nil, nil, accounting, &Failure{Code: "DIRECTION_ENVELOPES_REQUIRED", Stage: "pre_effect", Reason: "incoming and outgoing envelopes must both be present"}
	}
	if len(input.Envelopes) > limits.MaxEnvelopes || len(input.Marks) > limits.MaxMarks {
		return nil, nil, accounting, &Failure{Code: "RESOURCE_EXHAUSTED", Stage: "bounds", Reason: "envelope or mark bound"}
	}
	envelopes, dispositions, failure := validateEnvelopes(input.Envelopes, limits)
	if !addReferences(&accounting, uint64(len(dispositions)), limits) {
		return nil, dispositions, accounting, &Failure{Code: "RESOURCE_EXHAUSTED", Stage: "references", Reason: "max_references"}
	}
	if failure != nil {
		return nil, dispositions, accounting, failure
	}
	if failureEnvelope(envelopes) {
		if len(input.Marks) > 0 {
			return nil, dispositions, accounting, &Failure{Code: "FAILURE_WITH_POSITIVE_MARKS", Stage: "pre_effect", Reason: "direction failure cannot yield positive marks"}
		}
		return nil, dispositions, accounting, &Failure{Code: "DIRECTION_NON_POSITIVE", Stage: "direction_envelope", Reason: "one or more directions failed"}
	}
	if len(input.Marks) == 0 {
		return nil, dispositions, accounting, &Failure{Code: "EMPTY_COMPOSITION_EVIDENCE", Stage: "pre_effect", Reason: "success envelopes require explicit dependency marks"}
	}
	matchIDs := append([]string(nil), terminal.RangeUnionCandidate.MemberMatchIDs...)
	matchIndex := make(map[string]int, len(matchIDs))
	for i, id := range matchIDs {
		matchIndex[id] = i
	}
	marks := append([]DependencyMark(nil), input.Marks...)
	sort.SliceStable(marks, func(i, j int) bool { return markLess(marks[i], marks[j]) })
	seen := map[string]bool{}
	omit := map[string]bool{}
	ordered := []DependencyMark{}
	applied := []AppliedMark{}
	for _, mark := range marks {
		if err := ctx.Err(); err != nil {
			return nil, dispositions, accounting, &Failure{Code: "CANCELLED", Stage: "marks", Reason: err.Error()}
		}
		accounting.Work++
		if accounting.Work > limits.MaxWork {
			return nil, dispositions, accounting, &Failure{Code: "RESOURCE_EXHAUSTED", Stage: "work", Reason: "max_work"}
		}
		if failure := validateMark(mark, envelopes, matchIndex, seen, limits); failure != nil {
			return nil, dispositions, accounting, failure
		}
		seen[mark.MatchID] = true
		if mark.Action == ActionOmit {
			omit[mark.MatchID] = true
		} else {
			ordered = append(ordered, mark)
		}
		applied = append(applied, AppliedMark{Direction: mark.Direction, MatchID: mark.MatchID, Action: mark.Action, CandidateOrdinal: mark.CandidateOrdinal, OneFinalDispositionOrdinals: mark.OneFinalDispositionOrdinals, OrdinalIdentitySHA256: mark.OrdinalIdentitySHA256, SelectedOrdinal: mark.SelectedOrdinal, ProfileID: mark.ProfileID, EvidenceSHA256: mark.EvidenceSHA256})
		if !addReferences(&accounting, 1, limits) {
			return nil, dispositions, accounting, &Failure{Code: "RESOURCE_EXHAUSTED", Stage: "references", Reason: "max_references"}
		}
	}
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].CandidateOrdinal != ordered[j].CandidateOrdinal {
			return ordered[i].CandidateOrdinal < ordered[j].CandidateOrdinal
		}
		return markLess(ordered[i], ordered[j])
	})
	memberIDs := make([]string, 0, len(matchIDs))
	already := map[string]bool{}
	for _, mark := range ordered {
		if omit[mark.MatchID] {
			continue
		}
		memberIDs = append(memberIDs, mark.MatchID)
		already[mark.MatchID] = true
	}
	for _, id := range matchIDs {
		if omit[id] || already[id] {
			continue
		}
		memberIDs = append(memberIDs, id)
	}
	candidate := &CompositionCandidate{
		SchemaVersion:           SchemaVersion,
		Operation:               "ADR0011_PRIVATE_DEPENDENCY_MARKED_RANGE_UNION_PROJECTION",
		CandidateOnly:           true,
		ExecutedLocation:        false,
		AdmissionDigest:         terminal.Admission.AdmissionDigest,
		BaselineCandidateDigest: terminal.RangeUnionCandidate.CandidateDigest,
		BaselineMatchIDs:        append([]string(nil), matchIDs...),
		MemberMatchIDs:          append([]string(nil), memberIDs...),
		DirectionDispositions:   cloneDispositions(dispositions),
		AppliedMarks:            append([]AppliedMark(nil), applied...),
		Authority:               0,
		Accepted:                false,
		Completeness:            "UNKNOWN",
		FeatureIdentity:         "UNRESOLVED",
	}
	candidate.CandidateDigest = digest(adr0007.Canon(*candidate))
	return candidate, dispositions, accounting, nil
}

func AcceptedSuccessEnvelope(direction string) DirectionEnvelope {
	env := DirectionEnvelope{Direction: direction, Status: StatusSuccess, CompositionPackageChecksumSHA256: AcceptedBidirectionalPackageChecksumSHA256, CompositionArtifactSHA256: AcceptedBidirectionalCompositionArtifactSHA256, OrdinalSemantics: OrdinalSemanticsNoSelectedItems, OneFinalDispositionOrdinals: CanonicalOneFinalDispositionOrdinals}
	switch direction {
	case "incoming":
		env.ProfileID, env.GenericProfileID, env.EvidenceSHA256 = IncomingProfileID, IncomingGenericProfileID, IncomingEvidenceSHA256
		env.ProviderPackageChecksumSHA256, env.ProviderManifestSHA256 = IncomingPackageSHA256, IncomingManifestSHA256
		env.OrdinalIdentitySHA256 = IncomingOrdinalIdentitySHA256
	case "outgoing":
		env.ProfileID, env.GenericProfileID, env.EvidenceSHA256 = OutgoingProfileID, OutgoingGenericProfileID, OutgoingEvidenceSHA256
		env.ProviderPackageChecksumSHA256, env.ProviderManifestSHA256 = OutgoingPackageSHA256, OutgoingManifestSHA256
		env.OrdinalIdentitySHA256 = OutgoingOrdinalIdentitySHA256
	}
	return env
}

func AcceptedFailureEnvelope(direction string) DirectionEnvelope {
	env := AcceptedSuccessEnvelope(direction)
	env.Status = StatusFailure
	env.OrdinalSemantics = OrdinalSemanticsNoSelectedItems
	env.FailureCode = strings.ToUpper(direction) + "_FIXTURE_NATIVE_FAILURE"
	env.FailureStage = "composition_fixture"
	return env
}

func validateEnvelopes(input []DirectionEnvelope, limits Limits) (map[string]DirectionEnvelope, []DirectionDisposition, *Failure) {
	envelopes := map[string]DirectionEnvelope{}
	dispositions := []DirectionDisposition{}
	for _, env := range input {
		if env.Direction != "incoming" && env.Direction != "outgoing" {
			return nil, dispositions, &Failure{Code: "INVALID_DIRECTION", Stage: "envelope", Direction: env.Direction}
		}
		if _, exists := envelopes[env.Direction]; exists {
			return nil, dispositions, &Failure{Code: "DUPLICATE_DIRECTION", Stage: "envelope", Direction: env.Direction}
		}
		if exceedsStrings(limits.MaxStringBytes, env.Direction, env.Status, env.ProfileID, env.GenericProfileID, env.EvidenceSHA256, env.ProviderPackageChecksumSHA256, env.ProviderManifestSHA256, env.CompositionPackageChecksumSHA256, env.CompositionArtifactSHA256, env.OrdinalSemantics, env.FailureCode, env.FailureStage) {
			return nil, dispositions, &Failure{Code: "RESOURCE_EXHAUSTED", Stage: "string_bound", Direction: env.Direction}
		}
		if failure := validatePinnedEnvelope(env); failure != nil {
			return nil, dispositions, failure
		}
		copyEnv := env
		copyEnv.SelectedOrdinals = append([]uint64(nil), env.SelectedOrdinals...)
		envelopes[env.Direction] = copyEnv
		dispositions = append(dispositions, dispositionFromEnvelope(copyEnv))
	}
	if _, ok := envelopes["incoming"]; !ok {
		return nil, dispositions, &Failure{Code: "MISSING_DIRECTION", Stage: "envelope", Direction: "incoming"}
	}
	if _, ok := envelopes["outgoing"]; !ok {
		return nil, dispositions, &Failure{Code: "MISSING_DIRECTION", Stage: "envelope", Direction: "outgoing"}
	}
	sort.SliceStable(dispositions, func(i, j int) bool { return dispositions[i].Direction < dispositions[j].Direction })
	return envelopes, dispositions, nil
}

func validatePinnedEnvelope(env DirectionEnvelope) *Failure {
	want := AcceptedSuccessEnvelope(env.Direction)
	if env.Status != StatusSuccess && env.Status != StatusFailure {
		return &Failure{Code: "INVALID_DIRECTION_STATUS", Stage: "envelope", Direction: env.Direction, Reason: env.Status}
	}
	if env.Status == StatusFailure {
		want = AcceptedFailureEnvelope(env.Direction)
	}
	if env.ProfileID != want.ProfileID || env.GenericProfileID != want.GenericProfileID || env.EvidenceSHA256 != want.EvidenceSHA256 || env.ProviderPackageChecksumSHA256 != want.ProviderPackageChecksumSHA256 || env.ProviderManifestSHA256 != want.ProviderManifestSHA256 || env.CompositionPackageChecksumSHA256 != AcceptedBidirectionalPackageChecksumSHA256 || env.CompositionArtifactSHA256 != AcceptedBidirectionalCompositionArtifactSHA256 || env.OrdinalSemantics != want.OrdinalSemantics || env.OneFinalDispositionOrdinals != want.OneFinalDispositionOrdinals || env.OrdinalIdentitySHA256 != want.OrdinalIdentitySHA256 {
		return &Failure{Code: "PIN_DRIFT", Stage: "envelope", Direction: env.Direction}
	}
	if env.Status == StatusFailure {
		if env.FailureCode == "" || !strings.HasPrefix(env.FailureCode, strings.ToUpper(env.Direction)+"_") || env.FailureStage == "" {
			return &Failure{Code: "INVALID_FAILURE_SUM_TYPE", Stage: "envelope", Direction: env.Direction}
		}
		if len(env.SelectedOrdinals) != 0 {
			return &Failure{Code: "FAILURE_ORDINALS_FORBIDDEN", Stage: "envelope", Direction: env.Direction}
		}
		return nil
	}
	if len(env.SelectedOrdinals) != 0 {
		return &Failure{Code: "SELECTED_ORDINALS_NOT_EXPOSED", Stage: "envelope", Direction: env.Direction}
	}
	return nil
}

func validateMark(mark DependencyMark, envelopes map[string]DirectionEnvelope, matchIndex map[string]int, seen map[string]bool, limits Limits) *Failure {
	env, ok := envelopes[mark.Direction]
	if !ok {
		return &Failure{Code: "UNKNOWN_MARK_DIRECTION", Stage: "mark", Direction: mark.Direction}
	}
	if env.Status != StatusSuccess {
		return &Failure{Code: "MARK_ON_FAILED_DIRECTION", Stage: "mark", Direction: mark.Direction}
	}
	if _, ok := matchIndex[mark.MatchID]; !ok {
		return &Failure{Code: "UNKNOWN_MATCH_ID", Stage: "mark", Direction: mark.Direction}
	}
	if seen[mark.MatchID] {
		return &Failure{Code: "DUPLICATE_MATCH_MARK", Stage: "mark", Direction: mark.Direction}
	}
	if mark.Action != ActionOmit && mark.Action != ActionOrder {
		return &Failure{Code: "INVALID_MARK_ACTION", Stage: "mark", Direction: mark.Direction, Reason: mark.Action}
	}
	if mark.RelationKind != "CALLS" {
		return &Failure{Code: "RELATION_KIND_REFUSED", Stage: "mark", Direction: mark.Direction, Reason: mark.RelationKind}
	}
	if mark.ProfileID != env.ProfileID || mark.EvidenceSHA256 != env.EvidenceSHA256 {
		return &Failure{Code: "MARK_PIN_DRIFT", Stage: "mark", Direction: mark.Direction}
	}
	if mark.OneFinalDispositionOrdinals != env.OneFinalDispositionOrdinals || mark.OrdinalIdentitySHA256 != env.OrdinalIdentitySHA256 || mark.SelectedOrdinal != 0 {
		return &Failure{Code: "ORDINAL_IDENTITY_DRIFT", Stage: "mark", Direction: mark.Direction}
	}
	if exceedsStrings(limits.MaxStringBytes, mark.Direction, mark.MatchID, mark.Action, mark.RelationKind, mark.ProfileID, mark.EvidenceSHA256, mark.OrdinalIdentitySHA256) {
		return &Failure{Code: "RESOURCE_EXHAUSTED", Stage: "mark_string_bound", Direction: mark.Direction}
	}
	return nil
}

func cancelledResult(req Request, err error) Result {
	return Result{Failure: &Failure{Code: "CANCELLED", Stage: "pre_adr0007", Reason: err.Error()}}
}

func withDefaultLimits(l Limits) Limits {
	if l.MaxEnvelopes == 0 {
		l.MaxEnvelopes = 2
	}
	if l.MaxMarks == 0 {
		l.MaxMarks = 64
	}
	if l.MaxReferences == 0 {
		l.MaxReferences = 128
	}
	if l.MaxStringBytes == 0 {
		l.MaxStringBytes = 256
	}
	if l.MaxWork == 0 {
		l.MaxWork = 4096
	}
	return l
}

func failureEnvelope(envelopes map[string]DirectionEnvelope) bool {
	for _, env := range envelopes {
		if env.Status == StatusFailure {
			return true
		}
	}
	return false
}

func dispositionFromEnvelope(env DirectionEnvelope) DirectionDisposition {
	return DirectionDisposition{Direction: env.Direction, Status: env.Status, ProfileID: env.ProfileID, GenericProfileID: env.GenericProfileID, EvidenceSHA256: env.EvidenceSHA256, ProviderPackageChecksumSHA256: env.ProviderPackageChecksumSHA256, ProviderManifestSHA256: env.ProviderManifestSHA256, CompositionPackageChecksumSHA256: env.CompositionPackageChecksumSHA256, CompositionArtifactSHA256: env.CompositionArtifactSHA256, OrdinalSemantics: env.OrdinalSemantics, OneFinalDispositionOrdinals: env.OneFinalDispositionOrdinals, OrdinalIdentitySHA256: env.OrdinalIdentitySHA256, SelectedOrdinals: append([]uint64(nil), env.SelectedOrdinals...), FailureCode: env.FailureCode, FailureStage: env.FailureStage}
}

func cloneDispositions(in []DirectionDisposition) []DirectionDisposition {
	out := append([]DirectionDisposition(nil), in...)
	for i := range out {
		out[i].SelectedOrdinals = append([]uint64(nil), out[i].SelectedOrdinals...)
	}
	return out
}

func markLess(a, b DependencyMark) bool {
	if a.Direction != b.Direction {
		return a.Direction < b.Direction
	}
	if a.Action != b.Action {
		return a.Action < b.Action
	}
	if a.CandidateOrdinal != b.CandidateOrdinal {
		return a.CandidateOrdinal < b.CandidateOrdinal
	}
	if a.OrdinalIdentitySHA256 != b.OrdinalIdentitySHA256 {
		return a.OrdinalIdentitySHA256 < b.OrdinalIdentitySHA256
	}
	return a.MatchID < b.MatchID
}

func addReferences(accounting *Accounting, n uint64, limits Limits) bool {
	if n > ^uint64(0)-accounting.References {
		return false
	}
	next := accounting.References + n
	if limits.MaxReferences > 0 && next > uint64(limits.MaxReferences) {
		return false
	}
	accounting.References = next
	return true
}

func exceedsStrings(max int, values ...string) bool {
	for _, value := range values {
		if len(value) > max {
			return true
		}
		if strings.ContainsRune(value, '\x00') {
			return true
		}
	}
	return false
}

func digest(b []byte) string {
	s := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(s[:])
}
