package programcrepresentative

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"lsp-trace/internal/graph"
)

type SourceContextRole string

const (
	SourceContextRoleQueryReference SourceContextRole = "QUERY_REFERENCE"
	SourceContextRoleTarget         SourceContextRole = "TARGET"
)

type SourceContextOutcome string

const (
	SourceContextSelected     SourceContextOutcome = "SELECTED"
	SourceContextUnavailable  SourceContextOutcome = "UNAVAILABLE"
	SourceContextAmbiguous    SourceContextOutcome = "AMBIGUOUS"
	SourceContextWithheld     SourceContextOutcome = "WITHHELD"
	SourceContextLimitOmitted SourceContextOutcome = "LIMIT_OMITTED"
)

type PreparedSourceDocument struct {
	URI, SessionID                    string
	Generation, DocumentVersion       uint64
	RevisionIdentity, RevisionCustody string
	SourceSHA256                      string
	ByteLength                        int
	PositionEncoding                  string
	ExactBytes                        []byte
}

type AdmittedSourceOccurrence struct {
	Identity, SchemaID, Digest               string
	ByteLength, Ordinal, Multiplicity        int
	RelationID, PredecessorIdentity, Kind    string
	CallerID, TargetID                       string
	EvidenceRange, ItemRange, SelectionRange graph.Range
}

type SourceCandidate struct {
	OccurrenceIdentity string
	DocumentURI        string
	Role               SourceContextRole
	DisplayRange       graph.Range
	ContextSpan        graph.Range
}

type RoleSelectionInput struct{ Candidates []SourceCandidate }

type SourceContextPolicy struct {
	Identity, Digest string
	WithholdURIs     []string
}

type SourceContextLimits struct {
	MaxDocumentBytes, MaxTotalBytes, MaxDocuments int
	MaxOccurrences, MaxPoints, MaxSpans           int
	MaxCandidates, MaxStringBytes                 int
	MaxCanonicalBytes                             int
	MaxWork                                       int
}

type SourceContextInput struct {
	Representative          RepresentativeResult
	Occurrences             []AdmittedSourceOccurrence
	QueryReferenceSelection RoleSelectionInput
	TargetSelection         RoleSelectionInput
	Documents               []PreparedSourceDocument
	Policy                  SourceContextPolicy
	Limits                  SourceContextLimits
}

type SourceContextPoint struct {
	PointID, DocumentURI string
	Position             graph.Position
}

type SourceContextSpan struct {
	SpanID, DocumentURI string
	Range               graph.Range
	ExactBytes          []byte
}

type SourceContextRoleResult struct {
	Role            SourceContextRole
	Outcome         SourceContextOutcome
	DocumentURI     string
	PointID, SpanID string
	BindingID       string
	EvidenceRange   graph.Range
	ItemRange       graph.Range
	SelectionRange  graph.Range
	DisplayRange    graph.Range
	ContextSpan     graph.Range
	ExactBytes      []byte
}

type SourceContextBinding struct {
	BindingID, OccurrenceIdentity, SchemaID, Digest string
	ByteLength, Ordinal, Multiplicity               int
	RelationID, PredecessorIdentity, Kind           string
	CallerID, TargetID                              string
	Role                                            SourceContextRole
	Outcome                                         SourceContextOutcome
	DocumentURI, PointID, SpanID                    string
	EvidenceRange, ItemRange, SelectionRange        graph.Range
	DisplayRange, ContextSpan                       graph.Range
	ExactBytes                                      []byte
}

type SourceContextAccounting struct {
	InputOccurrences, Bindings                                   int
	SelectedRoles, UnavailableRoles, AmbiguousRoles              int
	WithheldRoles, LimitOmittedRoles                             int
	PointCandidates, InternedPoints, ReusedPoints                int
	SpanCandidates, InternedSpans, ReusedSpans                   int
	CandidateCount, DocumentCount, DocumentBytes, MaxStringBytes int
	Work, CanonicalBytes                                         int
}

type SourceContextResult struct {
	Identity, Digest             string
	ByteLength                   int
	RepresentativeDigest         string
	RepresentativeLength         int
	PolicyIdentity, PolicyDigest string
	Authority                    int
	Accepted                     bool
	Completeness                 string
	Limits                       SourceContextLimits
	representativeCanonical      []byte
	documents                    []PreparedSourceDocument
	occurrences                  []AdmittedSourceOccurrence
	candidates                   []SourceCandidate
	bindings                     []SourceContextBinding
	roles                        []SourceContextRoleResult
	points                       []SourceContextPoint
	spans                        []SourceContextSpan
	policyWithheld               []string
	accounting                   SourceContextAccounting
	canonical                    []byte
}

// sourceContextWire is the only serialized representation. SourceContextResult
// deliberately keeps every identity-bearing collection private.
type sourceContextWire struct {
	Identity, Digest             string
	ByteLength                   int
	RepresentativeDigest         string
	RepresentativeLength         int
	RepresentativeCanonical      []byte
	PolicyIdentity, PolicyDigest string
	PolicyWithholdURIs           []string
	Authority                    int
	Accepted                     bool
	Completeness                 string
	Limits                       SourceContextLimits
	DocumentValues               []PreparedSourceDocument
	OccurrenceValues             []AdmittedSourceOccurrence
	CandidateValues              []SourceCandidate
	BindingValues                []SourceContextBinding
	RoleValues                   []SourceContextRoleResult
	PointValues                  []SourceContextPoint
	SpanValues                   []SourceContextSpan
	AccountingValue              SourceContextAccounting
}

func (r SourceContextResult) CanonicalBytes() []byte              { return append([]byte(nil), r.canonical...) }
func (r SourceContextResult) Documents() []PreparedSourceDocument { return cloneDocuments(r.documents) }
func (r SourceContextResult) Candidates() []SourceCandidate {
	return append([]SourceCandidate(nil), r.candidates...)
}
func (r SourceContextResult) Bindings() []SourceContextBinding       { return cloneBindings(r.bindings) }
func (r SourceContextResult) RoleResults() []SourceContextRoleResult { return cloneRoles(r.roles) }
func (r SourceContextResult) Points() []SourceContextPoint {
	return append([]SourceContextPoint(nil), r.points...)
}
func (r SourceContextResult) Spans() []SourceContextSpan          { return cloneSpans(r.spans) }
func (r SourceContextResult) Accounting() SourceContextAccounting { return r.accounting }
func (r SourceContextResult) QueryReference() SourceContextRoleResult {
	return r.role(SourceContextRoleQueryReference)
}
func (r SourceContextResult) Target() SourceContextRoleResult { return r.role(SourceContextRoleTarget) }
func (r SourceContextResult) role(role SourceContextRole) SourceContextRoleResult {
	for _, x := range r.roles {
		if x.Role == role {
			return cloneRole(x)
		}
	}
	return SourceContextRoleResult{Role: role, Outcome: SourceContextUnavailable}
}
func (r SourceContextResult) ContextComplete() bool {
	return r.QueryReference().Outcome == SourceContextSelected && r.Target().Outcome == SourceContextSelected
}
func (r SourceContextResult) SelectedContextBytes(role SourceContextRole) ([]byte, bool) {
	x := r.role(role)
	if x.Outcome != SourceContextSelected {
		return nil, false
	}
	return append([]byte(nil), x.ExactBytes...), true
}

func ComputeSourceContext(in SourceContextInput) (SourceContextResult, error) {
	if err := validateInput(in); err != nil {
		return SourceContextResult{}, err
	}
	in.Documents = cloneDocuments(in.Documents)
	sort.SliceStable(in.Documents, func(i, j int) bool { return in.Documents[i].URI < in.Documents[j].URI })
	in.Occurrences = append([]AdmittedSourceOccurrence(nil), in.Occurrences...)
	sort.SliceStable(in.Occurrences, func(i, j int) bool {
		if in.Occurrences[i].Identity != in.Occurrences[j].Identity {
			return in.Occurrences[i].Identity < in.Occurrences[j].Identity
		}
		if in.Occurrences[i].Ordinal != in.Occurrences[j].Ordinal {
			return in.Occurrences[i].Ordinal < in.Occurrences[j].Ordinal
		}
		return in.Occurrences[i].Multiplicity < in.Occurrences[j].Multiplicity
	})
	normalizeCandidates := func(values []SourceCandidate) []SourceCandidate {
		out := append([]SourceCandidate(nil), values...)
		sort.SliceStable(out, func(i, j int) bool {
			return fmt.Sprint(out[i]) < fmt.Sprint(out[j])
		})
		return out
	}
	in.QueryReferenceSelection.Candidates = normalizeCandidates(in.QueryReferenceSelection.Candidates)
	in.TargetSelection.Candidates = normalizeCandidates(in.TargetSelection.Candidates)
	in.Policy.WithholdURIs = append([]string(nil), in.Policy.WithholdURIs...)
	sort.Strings(in.Policy.WithholdURIs)
	for i := 1; i < len(in.Policy.WithholdURIs); i++ {
		if in.Policy.WithholdURIs[i] == in.Policy.WithholdURIs[i-1] {
			return SourceContextResult{}, fmt.Errorf("duplicate withheld URI")
		}
	}
	// Work is a protocol quantity, not an implementation cost. One unit is charged
	// for each document byte, occurrence, candidate, role, coordinate/rune byte
	// scanned, document/occurrence lookup, intern attempt, binding, and canonical byte.
	preflight, err := sourceContextPreflightWork(in)
	if err != nil {
		return SourceContextResult{}, err
	}
	if preflight > in.Limits.MaxWork {
		return SourceContextResult{}, fmt.Errorf("source context work limit exceeded")
	}
	r := SourceContextResult{RepresentativeDigest: in.Representative.Digest, RepresentativeLength: len(in.Representative.CanonicalBytes()), representativeCanonical: in.Representative.CanonicalBytes(), PolicyIdentity: in.Policy.Identity, PolicyDigest: in.Policy.Digest, Authority: 0, Accepted: false, Completeness: "UNKNOWN", Limits: in.Limits, policyWithheld: append([]string(nil), in.Policy.WithholdURIs...)}
	r.documents = cloneDocuments(in.Documents)
	r.occurrences = append([]AdmittedSourceOccurrence(nil), in.Occurrences...)
	r.candidates = append(append([]SourceCandidate(nil), in.QueryReferenceSelection.Candidates...), in.TargetSelection.Candidates...)
	r.accounting.InputOccurrences = len(in.Occurrences)
	r.accounting.CandidateCount = len(r.candidates)
	r.accounting.DocumentCount = len(in.Documents)
	for _, d := range in.Documents {
		r.accounting.DocumentBytes += len(d.ExactBytes)
	}
	pointIndex, spanIndex := map[string]int{}, map[string]int{}
	candidateUsed := 0
	roleSpecs := []struct {
		role      SourceContextRole
		selection RoleSelectionInput
	}{{SourceContextRoleQueryReference, in.QueryReferenceSelection}, {SourceContextRoleTarget, in.TargetSelection}}
	for _, spec := range roleSpecs {
		x := selectRole(spec.role, spec.selection, in, &r, pointIndex, spanIndex, &candidateUsed)
		r.roles = append(r.roles, x)
		switch x.Outcome {
		case SourceContextSelected:
			r.accounting.SelectedRoles++
		case SourceContextUnavailable:
			r.accounting.UnavailableRoles++
		case SourceContextAmbiguous:
			r.accounting.AmbiguousRoles++
		case SourceContextWithheld:
			r.accounting.WithheldRoles++
		case SourceContextLimitOmitted:
			r.accounting.LimitOmittedRoles++
		}
	}
	for _, occurrence := range in.Occurrences {
		for _, role := range r.roles {
			b := SourceContextBinding{OccurrenceIdentity: occurrence.Identity, SchemaID: occurrence.SchemaID, Digest: occurrence.Digest, ByteLength: occurrence.ByteLength, Ordinal: occurrence.Ordinal, Multiplicity: occurrence.Multiplicity, RelationID: occurrence.RelationID, PredecessorIdentity: occurrence.PredecessorIdentity, Kind: occurrence.Kind, CallerID: occurrence.CallerID, TargetID: occurrence.TargetID, Role: role.Role, Outcome: role.Outcome, DocumentURI: role.DocumentURI, PointID: role.PointID, SpanID: role.SpanID, EvidenceRange: occurrence.EvidenceRange, ItemRange: occurrence.ItemRange, SelectionRange: occurrence.SelectionRange, DisplayRange: role.DisplayRange, ContextSpan: role.ContextSpan, ExactBytes: append([]byte(nil), role.ExactBytes...)}
			b.BindingID = bindingIdentity(occurrence, role.Role)
			r.bindings = append(r.bindings, b)
		}
	}
	r.accounting.Bindings = len(r.bindings)
	r.accounting.MaxStringBytes = maxStringBytes(in, r)
	if in.Limits.MaxStringBytes < r.accounting.MaxStringBytes {
		return SourceContextResult{}, fmt.Errorf("source context string limit exceeded")
	}
	if err := sealResult(&r, in.Limits.MaxCanonicalBytes, preflight); err != nil {
		return SourceContextResult{}, err
	}
	return r, nil
}

func sourceContextAdd(total *int, value int) error {
	if value < 0 {
		return fmt.Errorf("source context work overflow")
	}
	max := int(^uint(0) >> 1)
	if *total > max-value {
		return fmt.Errorf("source context work overflow")
	}
	*total += value
	return nil
}

func sourceContextMul(a, b int) (int, error) {
	if a < 0 || b < 0 {
		return 0, fmt.Errorf("source context work overflow")
	}
	if a != 0 && b > int(^uint(0)>>1)/a {
		return 0, fmt.Errorf("source context work overflow")
	}
	return a * b, nil
}

// sourceContextPreflightWork computes all non-canonical work before result
// construction. Units are: each retained document byte; each occurrence;
// each candidate; each of the two roles; for every candidate, two full-byte
// coordinate scan allowances, one lookup against every document and occurrence,
// and two intern attempts; and every occurrence×role binding. sealResult adds
// exactly one unit for each canonical byte after fixed-point sizing.
func sourceContextPreflightWork(in SourceContextInput) (int, error) {
	total := 0
	for _, d := range in.Documents {
		if err := sourceContextAdd(&total, len(d.ExactBytes)); err != nil {
			return 0, err
		}
	}
	if err := sourceContextAdd(&total, len(in.Occurrences)); err != nil {
		return 0, err
	}
	candidates := append(append([]SourceCandidate(nil), in.QueryReferenceSelection.Candidates...), in.TargetSelection.Candidates...)
	if err := sourceContextAdd(&total, len(candidates)); err != nil {
		return 0, err
	}
	if err := sourceContextAdd(&total, 2); err != nil {
		return 0, err
	}
	for _, c := range candidates {
		scanBytes := 0
		for _, d := range in.Documents {
			if d.URI == c.DocumentURI {
				scanBytes = len(d.ExactBytes)
				break
			}
		}
		doubleScan, err := sourceContextMul(scanBytes, 2)
		if err != nil {
			return 0, err
		}
		if err := sourceContextAdd(&total, doubleScan); err != nil {
			return 0, err
		}
		if err := sourceContextAdd(&total, len(in.Documents)); err != nil {
			return 0, err
		}
		if err := sourceContextAdd(&total, len(in.Occurrences)); err != nil {
			return 0, err
		}
		if err := sourceContextAdd(&total, 2); err != nil {
			return 0, err
		}
	}
	bindings, err := sourceContextMul(len(in.Occurrences), 2)
	if err != nil {
		return 0, err
	}
	if err := sourceContextAdd(&total, bindings); err != nil {
		return 0, err
	}
	return total, nil
}

func bindingIdentity(occurrence AdmittedSourceOccurrence, role SourceContextRole) string {
	return hashValue(struct {
		Occurrence            string
		Ordinal, Multiplicity int
		Role                  SourceContextRole
	}{occurrence.Identity, occurrence.Ordinal, occurrence.Multiplicity, role})
}

func RecomputeSourceContext(in SourceContextInput) (SourceContextResult, error) {
	return ComputeSourceContext(in)
}
func ReplaySourceContext(in SourceContextInput, prior SourceContextResult) (SourceContextResult, error) {
	got, err := ComputeSourceContext(in)
	if err != nil {
		return SourceContextResult{}, err
	}
	if !bytes.Equal(got.CanonicalBytes(), prior.CanonicalBytes()) || !equalResultExported(got, prior) {
		return SourceContextResult{}, fmt.Errorf("source context replay mismatch")
	}
	return got, nil
}

func selectRole(role SourceContextRole, selection RoleSelectionInput, in SourceContextInput, r *SourceContextResult, pointIndex, spanIndex map[string]int, candidateUsed *int) SourceContextRoleResult {
	x := SourceContextRoleResult{Role: role, Outcome: SourceContextUnavailable}
	if len(selection.Candidates) == 0 {
		return x
	}
	if *candidateUsed+len(selection.Candidates) > in.Limits.MaxCandidates {
		x.Outcome = SourceContextLimitOmitted
		return x
	}
	*candidateUsed += len(selection.Candidates)
	if len(selection.Candidates) != 1 {
		x.Outcome = SourceContextAmbiguous
		return x
	}
	c := selection.Candidates[0]
	if c.Role != role {
		x.Outcome = SourceContextUnavailable
		return x
	}
	for _, uri := range in.Policy.WithholdURIs {
		if uri == c.DocumentURI {
			x.Outcome = SourceContextWithheld
			return x
		}
	}
	var doc *PreparedSourceDocument
	for i := range in.Documents {
		if in.Documents[i].URI == c.DocumentURI {
			doc = &in.Documents[i]
			break
		}
	}
	if doc == nil {
		return x
	}
	if len(doc.ExactBytes) > in.Limits.MaxDocumentBytes {
		x.Outcome = SourceContextLimitOmitted
		return x
	}
	spanBytes, err := ExactUTF16Slice(doc.ExactBytes, c.ContextSpan)
	if err != nil {
		return SourceContextRoleResult{Role: role, Outcome: SourceContextUnavailable}
	}
	occ := occurrenceByID(in.Occurrences, c.OccurrenceIdentity)
	if occ == nil {
		return x
	}
	pointKey := c.DocumentURI + "|" + fmt.Sprint(c.DisplayRange.Start)
	spanKey := c.DocumentURI + "|" + fmt.Sprint(c.ContextSpan)
	_, pointExists := pointIndex[pointKey]
	_, spanExists := spanIndex[spanKey]
	if (!pointExists && len(r.points) >= in.Limits.MaxPoints) || (!spanExists && len(r.spans) >= in.Limits.MaxSpans) {
		x.Outcome = SourceContextLimitOmitted
		return x
	}
	r.accounting.PointCandidates++
	pointID := hashValue(struct {
		URI      string
		Position graph.Position
	}{c.DocumentURI, c.DisplayRange.Start})
	if _, ok := pointIndex[pointKey]; ok {
		r.accounting.ReusedPoints++
	} else if len(r.points) >= in.Limits.MaxPoints {
		x.Outcome = SourceContextLimitOmitted
		return x
	} else {
		pointIndex[pointKey] = len(r.points)
		r.points = append(r.points, SourceContextPoint{pointID, c.DocumentURI, c.DisplayRange.Start})
		r.accounting.InternedPoints++
	}
	r.accounting.SpanCandidates++
	spanID := hashValue(struct {
		URI   string
		Range graph.Range
		Bytes []byte
	}{c.DocumentURI, c.ContextSpan, spanBytes})
	if _, ok := spanIndex[spanKey]; ok {
		r.accounting.ReusedSpans++
	} else if len(r.spans) >= in.Limits.MaxSpans {
		x.Outcome = SourceContextLimitOmitted
		return x
	} else {
		spanIndex[spanKey] = len(r.spans)
		r.spans = append(r.spans, SourceContextSpan{spanID, c.DocumentURI, c.ContextSpan, append([]byte(nil), spanBytes...)})
		r.accounting.InternedSpans++
	}
	x.Outcome, x.DocumentURI, x.PointID, x.SpanID = SourceContextSelected, c.DocumentURI, pointID, spanID
	x.BindingID = hashValue(struct {
		Occurrence            string
		Ordinal, Multiplicity int
		Role                  SourceContextRole
	}{occ.Identity, occ.Ordinal, occ.Multiplicity, role})
	x.EvidenceRange, x.ItemRange, x.SelectionRange = occ.EvidenceRange, occ.ItemRange, occ.SelectionRange
	x.DisplayRange, x.ContextSpan, x.ExactBytes = c.DisplayRange, c.ContextSpan, append([]byte(nil), spanBytes...)
	return x
}

func validateInput(in SourceContextInput) error {
	l := in.Limits
	if l.MaxDocumentBytes < 0 || l.MaxTotalBytes < 0 || l.MaxDocuments < 0 || l.MaxOccurrences < 0 || l.MaxPoints < 0 || l.MaxSpans < 0 || l.MaxCandidates < 0 || l.MaxStringBytes < 0 || l.MaxCanonicalBytes < 0 || l.MaxWork < 0 {
		return fmt.Errorf("negative source context limit")
	}
	if len(in.Documents) > l.MaxDocuments || len(in.Occurrences) > l.MaxOccurrences {
		return fmt.Errorf("source context cardinality limit exceeded")
	}
	total := 0
	seen := map[string]bool{}
	var custody *PreparedSourceDocument
	for _, d := range in.Documents {
		if d.URI == "" || !strings.HasSuffix(d.URI, ".go") || seen[d.URI] || d.SessionID == "" || d.Generation == 0 || d.DocumentVersion == 0 || d.RevisionIdentity == "" || d.RevisionCustody != "PROVIDER_VERIFIED" || d.PositionEncoding != "utf-16" {
			return fmt.Errorf("invalid document custody")
		}
		seen[d.URI] = true
		if custody == nil {
			copy := d
			custody = &copy
		} else if d.SessionID != custody.SessionID || d.Generation != custody.Generation || d.RevisionIdentity != custody.RevisionIdentity || d.RevisionCustody != custody.RevisionCustody || d.PositionEncoding != custody.PositionEncoding {
			return fmt.Errorf("inconsistent document custody")
		}
		if len(d.ExactBytes) > l.MaxDocumentBytes {
			return fmt.Errorf("source context document byte limit exceeded")
		}
		total += len(d.ExactBytes)
		if d.ByteLength != len(d.ExactBytes) || d.SourceSHA256 != digestBytes(d.ExactBytes) {
			return fmt.Errorf("invalid document bytes")
		}
	}
	if total > l.MaxTotalBytes {
		return fmt.Errorf("source context total byte limit exceeded")
	}

	representativeBytes := in.Representative.CanonicalBytes()
	if in.Representative.Digest == "" || len(representativeBytes) == 0 || in.Representative.Digest != digestBytes(representativeBytes) || in.Policy.Identity == "" || in.Policy.Digest == "" {
		return fmt.Errorf("missing or inconsistent predecessor identity")
	}
	occurrences := make(map[string]bool, len(in.Occurrences))
	for _, o := range in.Occurrences {
		if o.Identity == "" || occurrences[o.Identity] || o.SchemaID == "" || o.Digest == "" || o.ByteLength <= 0 || o.Ordinal < 0 || o.Multiplicity <= 0 || o.RelationID == "" || o.PredecessorIdentity == "" || o.Kind != "CALLS" || o.CallerID == "" || o.TargetID == "" || !validRange(o.EvidenceRange) || !validRange(o.ItemRange) || !validRange(o.SelectionRange) {
			return fmt.Errorf("invalid source occurrence")
		}
		occurrences[o.Identity] = true
	}
	for _, selection := range []struct {
		input RoleSelectionInput
		role  SourceContextRole
	}{{in.QueryReferenceSelection, SourceContextRoleQueryReference}, {in.TargetSelection, SourceContextRoleTarget}} {
		for _, c := range selection.input.Candidates {
			if !occurrences[c.OccurrenceIdentity] || c.DocumentURI == "" || c.Role != selection.role || !validRange(c.DisplayRange) || !validRange(c.ContextSpan) {
				return fmt.Errorf("invalid source candidate")
			}
		}
	}
	return nil
}

func validRange(r graph.Range) bool {
	return r.Start.Line < r.End.Line || (r.Start.Line == r.End.Line && r.Start.Character <= r.End.Character)
}

func occurrenceByID(values []AdmittedSourceOccurrence, id string) *AdmittedSourceOccurrence {
	for i := range values {
		if values[i].Identity == id {
			return &values[i]
		}
	}
	return nil
}

func ExactUTF16Slice(body []byte, span graph.Range) ([]byte, error) {
	start, err := utf16Offset(body, span.Start)
	if err != nil {
		return nil, err
	}
	end, err := utf16Offset(body, span.End)
	if err != nil || end < start {
		return nil, fmt.Errorf("invalid UTF-16 range")
	}
	return append([]byte(nil), body[start:end]...), nil
}

func utf16Offset(body []byte, p graph.Position) (int, error) {
	line, offset := uint32(0), 0
	for line < p.Line {
		i := bytes.IndexByte(body[offset:], '\n')
		if i < 0 {
			return 0, fmt.Errorf("line out of bounds")
		}
		offset += i + 1
		line++
	}
	units := uint32(0)
	for offset < len(body) {
		if units == p.Character {
			return offset, nil
		}
		if body[offset] == '\n' || body[offset] == '\r' {
			break
		}
		r, n := utf8.DecodeRune(body[offset:])
		if r == utf8.RuneError && n == 1 {
			return 0, fmt.Errorf("invalid UTF-8")
		}
		step := uint32(1)
		if r > 0xffff {
			step = 2
		}
		if units+step > p.Character {
			return 0, fmt.Errorf("surrogate split")
		}
		units += step
		offset += n
	}
	if units == p.Character {
		return offset, nil
	}
	return 0, fmt.Errorf("character out of bounds")
}

func wireFromResult(r SourceContextResult) sourceContextWire {
	return sourceContextWire{
		Identity: r.Identity, Digest: r.Digest, ByteLength: r.ByteLength,
		RepresentativeDigest: r.RepresentativeDigest, RepresentativeLength: r.RepresentativeLength, RepresentativeCanonical: append([]byte(nil), r.representativeCanonical...),
		PolicyIdentity: r.PolicyIdentity, PolicyDigest: r.PolicyDigest, PolicyWithholdURIs: append([]string(nil), r.policyWithheld...),
		Authority: r.Authority, Accepted: r.Accepted, Completeness: r.Completeness, Limits: r.Limits,
		DocumentValues: cloneDocuments(r.documents), OccurrenceValues: append([]AdmittedSourceOccurrence(nil), r.occurrences...), CandidateValues: append([]SourceCandidate(nil), r.candidates...),
		BindingValues: cloneBindings(r.bindings), RoleValues: cloneRoles(r.roles), PointValues: append([]SourceContextPoint(nil), r.points...), SpanValues: cloneSpans(r.spans), AccountingValue: r.accounting,
	}
}

func sealResult(r *SourceContextResult, max, baseWork int) error {
	for i := 0; i < 12; i++ {
		w := wireFromResult(*r)
		w.Identity, w.Digest, w.ByteLength, w.AccountingValue.CanonicalBytes = "", "", 0, 0
		core, err := json.Marshal(w)
		if err != nil {
			return err
		}
		digest := digestBytes(core)
		w.Identity, w.Digest, w.ByteLength, w.AccountingValue = digest, digest, r.ByteLength, r.accounting
		raw, err := json.Marshal(w)
		if err != nil {
			return err
		}
		n := len(raw) + 1
		work, err := checkedAdd(baseWork, n)
		if err != nil {
			return fmt.Errorf("source context work overflow")
		}
		if r.ByteLength == n && r.accounting.CanonicalBytes == n && r.accounting.Work == work {
			if n > max {
				return fmt.Errorf("source context canonical limit exceeded")
			}
			if work > r.Limits.MaxWork {
				return fmt.Errorf("source context work limit exceeded")
			}
			r.Identity, r.Digest, r.canonical = digest, digest, append(raw, '\n')
			return nil
		}
		r.Identity, r.Digest, r.ByteLength = digest, digest, n
		r.accounting.CanonicalBytes, r.accounting.Work = n, work
	}
	return fmt.Errorf("source context canonical length did not converge")
}

func ParseSourceContext(raw []byte) (SourceContextResult, error) {
	if len(raw) == 0 || raw[len(raw)-1] != '\n' || bytes.Contains(raw, []byte("\r")) {
		return SourceContextResult{}, fmt.Errorf("noncanonical source context")
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var w sourceContextWire
	if err := dec.Decode(&w); err != nil {
		return SourceContextResult{}, err
	}
	if dec.More() {
		return SourceContextResult{}, fmt.Errorf("trailing source context")
	}
	identity, digest, length, canonicalBytes := w.Identity, w.Digest, w.ByteLength, w.AccountingValue.CanonicalBytes
	w.Identity, w.Digest, w.ByteLength, w.AccountingValue.CanonicalBytes = "", "", 0, 0
	core, err := json.Marshal(w)
	if err != nil || identity == "" || identity != digest || digest != digestBytes(core) {
		return SourceContextResult{}, fmt.Errorf("identity mismatch")
	}
	w.Identity, w.Digest, w.ByteLength, w.AccountingValue.CanonicalBytes = identity, digest, length, canonicalBytes
	encoded, err := json.Marshal(w)
	if err != nil || length != len(encoded)+1 || canonicalBytes != len(encoded)+1 || !bytes.Equal(append(encoded, '\n'), raw) {
		return SourceContextResult{}, fmt.Errorf("source context canonical mismatch: declared=%d accounting=%d encoded=%d raw=%d equal=%t", length, canonicalBytes, len(encoded)+1, len(raw), bytes.Equal(append(encoded, '\n'), raw))
	}
	if w.RepresentativeLength <= 0 || w.RepresentativeLength != len(w.RepresentativeCanonical) || w.RepresentativeDigest != digestBytes(w.RepresentativeCanonical) {
		return SourceContextResult{}, fmt.Errorf("invalid representative claim")
	}
	in := SourceContextInput{Representative: RepresentativeResult{Digest: w.RepresentativeDigest, canonical: append([]byte(nil), w.RepresentativeCanonical...)}, Occurrences: append([]AdmittedSourceOccurrence(nil), w.OccurrenceValues...), Documents: cloneDocuments(w.DocumentValues), Policy: SourceContextPolicy{Identity: w.PolicyIdentity, Digest: w.PolicyDigest, WithholdURIs: append([]string(nil), w.PolicyWithholdURIs...)}, Limits: w.Limits}
	for _, c := range w.CandidateValues {
		switch c.Role {
		case SourceContextRoleQueryReference:
			in.QueryReferenceSelection.Candidates = append(in.QueryReferenceSelection.Candidates, c)
		case SourceContextRoleTarget:
			in.TargetSelection.Candidates = append(in.TargetSelection.Candidates, c)
		default:
			return SourceContextResult{}, fmt.Errorf("invalid source candidate role")
		}
	}
	expected, err := ComputeSourceContext(in)
	if err != nil || !bytes.Equal(expected.CanonicalBytes(), raw) {
		return SourceContextResult{}, fmt.Errorf("source context semantic mismatch")
	}
	return expected, nil
}

func equalResultExported(a, b SourceContextResult) bool {
	return a.Identity == b.Identity && a.Digest == b.Digest && a.ByteLength == b.ByteLength &&
		a.RepresentativeDigest == b.RepresentativeDigest && a.RepresentativeLength == b.RepresentativeLength &&
		a.PolicyIdentity == b.PolicyIdentity && a.PolicyDigest == b.PolicyDigest && a.Authority == b.Authority &&
		a.Accepted == b.Accepted && a.Completeness == b.Completeness && a.Limits == b.Limits &&
		bytes.Equal(a.CanonicalBytes(), b.CanonicalBytes())
}
func hashValue(v any) string {
	raw, _ := json.Marshal(v)
	sum := sha256.Sum256(raw)
	return fmt.Sprintf("sha256:%x", sum)
}
func cloneDocuments(in []PreparedSourceDocument) []PreparedSourceDocument {
	out := append([]PreparedSourceDocument(nil), in...)
	for i := range out {
		out[i].ExactBytes = append([]byte(nil), out[i].ExactBytes...)
	}
	return out
}
func cloneRole(x SourceContextRoleResult) SourceContextRoleResult {
	x.ExactBytes = append([]byte(nil), x.ExactBytes...)
	return x
}
func cloneRoles(in []SourceContextRoleResult) []SourceContextRoleResult {
	out := append([]SourceContextRoleResult(nil), in...)
	for i := range out {
		out[i] = cloneRole(out[i])
	}
	return out
}
func cloneBindings(in []SourceContextBinding) []SourceContextBinding {
	out := append([]SourceContextBinding(nil), in...)
	for i := range out {
		out[i].ExactBytes = append([]byte(nil), out[i].ExactBytes...)
	}
	return out
}
func cloneSpans(in []SourceContextSpan) []SourceContextSpan {
	out := append([]SourceContextSpan(nil), in...)
	for i := range out {
		out[i].ExactBytes = append([]byte(nil), out[i].ExactBytes...)
	}
	return out
}
func maxStringBytes(in SourceContextInput, r SourceContextResult) int {
	max := 0
	add := func(s string) {
		if len(s) > max {
			max = len(s)
		}
	}
	add(in.Representative.Digest)
	add(in.Policy.Identity)
	add(in.Policy.Digest)
	for _, uri := range in.Policy.WithholdURIs {
		add(uri)
	}
	for _, d := range in.Documents {
		add(d.URI)
		add(d.SessionID)
		add(d.RevisionIdentity)
		add(d.RevisionCustody)
		add(d.SourceSHA256)
		add(d.PositionEncoding)
	}
	for _, o := range in.Occurrences {
		add(o.Identity)
		add(o.SchemaID)
		add(o.Digest)
		add(o.RelationID)
		add(o.PredecessorIdentity)
		add(o.Kind)
		add(o.CallerID)
		add(o.TargetID)
	}
	for _, c := range r.candidates {
		add(c.OccurrenceIdentity)
		add(c.DocumentURI)
		add(string(c.Role))
	}
	return max
}

var _ = sort.Strings
