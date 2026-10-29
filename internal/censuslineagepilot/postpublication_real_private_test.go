//go:build darwin || linux

package censuslineagepilot_test

// Explicit opt-in prototype caller, not an ordinary admission/import endpoint.
// Source comments and all retained interpretation/review text are untrusted DATA.
import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"

	pilot "lsp-trace/internal/censuslineagepilot"
	prep "lsp-trace/internal/privatedescribeprep"
	"lsp-trace/internal/publication"
	derived "lsp-trace/internal/retainedpartialprogramc"
)

const realPostConfigEnv = "LSP_TRACE_POST_PUBLICATION_REAL_CONFIG"
const realPostPinsSHA = "sha256:780f77b9d75c013ce99d1e9d1e142e7eca27761b3fbd07e738b5fd3bcaa11a9c"
const realPostAssessmentSHA = "sha256:8afbd5c5bb7e1d9a8242e5346b68a5a5fbee56e24735951186f2ba0c4561f9e3"
const realPostPreparationSHA = "sha256:5d6af15eb96c4c33a00b8aea70705dba85f34d1c42ad040e5abb319db4c26f5a"
const realPostCandidatesSHA = "sha256:a803c5ec633d58ff03bd0db70e41972f90195fb9abe86b02eeb3eb92c6b02339"

// Exact canonical JSON is required (encoding/json ordering, no newline). No
// endpoint query, executable, workspace or provider startup field exists.
type realPostArtifactPair struct {
	Paths  [2]string `json:"paths"`
	SHA256 string    `json:"sha256"`
}
type realPostConfig struct {
	Schema                 string               `json:"schema"`
	QualificationPins      realPostArtifactPair `json:"qualification_pins"`
	OriginalRoots          [3]string            `json:"original_roots"`
	PartialRoots           [2]string            `json:"partial_roots"`
	Instability            realPostArtifactPair `json:"instability"`
	Preparation            realPostArtifactPair `json:"preparation"`
	CandidatesPath         string               `json:"candidates_path"`
	CandidatesSHA256       string               `json:"candidates_sha256"`
	CallerCASRoots         [2]string            `json:"caller_cas_roots"`
	OutputRoots            [2]string            `json:"output_roots"`
	ProtectedArtifactRoots []string             `json:"protected_artifact_roots"`
	ReportPath             string               `json:"report_path"`
}
type realPostCrosswalk struct {
	CandidateID, ProducerSHA256, ReviewSHA256, InputSHA256, Target, NewLeadID, SubmissionSHA256 string
}
type realPostReport struct {
	Schema, Phase string
	prep.PrivateCeilings
	Absence, AcquisitionReplay, Recomputations        string
	Phases, Errors, CloseErrors, ConversionRejections []string
	Crosswalk                                         []realPostCrosswalk
	Outcome                                           *prep.PostPublicationOutcome
	Ledger                                            *prep.StageLedger
}

func realPostCanonicalPath(path string) error {
	if path == "" || len(path) > 4096 || !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return fmt.Errorf("noncanonical absolute artifact path")
	}
	resolved, e := filepath.EvalSymlinks(path)
	if e != nil {
		return e
	}
	if resolved != path {
		return fmt.Errorf("artifact path alias")
	}
	return nil
}
func realPostContains(root, path string) bool {
	return root == path || strings.HasPrefix(path, root+string(filepath.Separator))
}
func realPostOutsideTemp(path string) bool {
	return !realPostContains("/tmp", path) && !realPostContains("/private/tmp", path)
}

// Descriptor-relative single named artifact read, with finite cap, regular-file
// check and close propagation. This does not claim an attacker-proof path lease.
func realPostRead(path string, limit int) ([]byte, error) {
	raw, _, err := realPostReadIdentity(path, limit)
	return raw, err
}

// Identity is captured from the very descriptor supplying the bounded bytes.
// It remains a boundary observation, not an attacker/namespace stability lease.
func realPostReadIdentity(path string, limit int) (raw []byte, identity os.FileInfo, err error) {
	if e := realPostCanonicalPath(path); e != nil {
		return nil, nil, e
	}
	root, e := os.OpenRoot(filepath.Dir(path))
	if e != nil {
		return nil, nil, e
	}
	defer func() { err = errors.Join(err, root.Close()) }()
	info, e := root.Lstat(filepath.Base(path))
	if e != nil {
		return nil, nil, e
	}
	if !info.Mode().IsRegular() || info.Size() < 1 || info.Size() > int64(limit) {
		return nil, nil, fmt.Errorf("artifact type/size bound")
	}
	f, e := root.Open(filepath.Base(path))
	if e != nil {
		return nil, nil, e
	}
	defer func() { err = errors.Join(err, f.Close()) }()
	actual, e := f.Stat()
	if e != nil {
		return nil, nil, e
	}
	if !os.SameFile(info, actual) {
		return nil, nil, fmt.Errorf("artifact identity changed")
	}
	raw, e = io.ReadAll(io.LimitReader(f, int64(limit)+1))
	if e != nil {
		return nil, nil, e
	}
	if len(raw) > limit || int64(len(raw)) != info.Size() {
		return nil, nil, fmt.Errorf("artifact byte bound")
	}
	return raw, actual, nil
}

var errRealPostPairAlias = errors.New("paired artifact physical alias refused")
var errRealPostCASAlias = errors.New("caller CAS directory alias refused")

func realPostReadPair(pair realPostArtifactPair, limit int) ([]byte, error) {
	if pair.Paths[0] == pair.Paths[1] {
		return nil, errRealPostPairAlias
	}
	var raw []byte
	var first os.FileInfo
	for i, path := range pair.Paths {
		b, info, e := realPostReadIdentity(path, limit)
		if e != nil {
			return nil, e
		}
		if i == 0 {
			first = info
		} else if os.SameFile(first, info) {
			return nil, errRealPostPairAlias
		}
		if pilot.Digest(b) != pair.SHA256 {
			return nil, fmt.Errorf("paired artifact SHA mismatch")
		}
		if i == 0 {
			raw = b
		} else if !bytes.Equal(raw, b) {
			return nil, fmt.Errorf("paired artifact bytes mismatch")
		}
	}
	return raw, nil
}
func realPostDecode(raw []byte, out any) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if e := d.Decode(out); e != nil {
		return e
	}
	if d.Decode(new(any)) != io.EOF {
		return fmt.Errorf("trailing JSON")
	}
	return nil
}

// Preflight is descriptor-based and close-aware, not just a lexical check.
func realPostDistinctCASRoots(paths [2]string) (err error) {
	if paths[0] == paths[1] {
		return errRealPostCASAlias
	}
	var first os.FileInfo
	for i, path := range paths {
		if e := realPostCanonicalPath(path); e != nil {
			return e
		}
		r, e := os.OpenRoot(path)
		if e != nil {
			return e
		}
		info, se := r.Stat(".")
		e = errors.Join(se, r.Close())
		if e != nil {
			return e
		}
		if info == nil || !info.IsDir() {
			return fmt.Errorf("caller CAS root is not a directory")
		}
		if i == 0 {
			first = info
		} else if os.SameFile(first, info) {
			return errRealPostCASAlias
		}
	}
	return nil
}

func realPostConfigRead(path string) (realPostConfig, error) {
	var c realPostConfig
	b, e := realPostRead(path, 64<<10)
	if e != nil {
		return c, e
	}
	if e = realPostDecode(b, &c); e != nil {
		return c, e
	}
	canonical, e := json.Marshal(c)
	if e != nil || !bytes.Equal(canonical, b) {
		return c, fmt.Errorf("noncanonical config JSON")
	}
	if c.Schema != "post-publication-real-caller.private.v1" || c.QualificationPins.SHA256 != realPostPinsSHA || c.Instability.SHA256 != realPostAssessmentSHA || c.Preparation.SHA256 != realPostPreparationSHA || c.CandidatesSHA256 != realPostCandidatesSHA || len(c.ProtectedArtifactRoots) < 1 || len(c.ProtectedArtifactRoots) > 16 {
		return c, fmt.Errorf("fixed driver profile/pins")
	}
	if e = realPostDistinctCASRoots(c.CallerCASRoots); e != nil {
		return c, e
	}
	required := append([]string{}, c.OriginalRoots[:]...)
	required = append(required, c.PartialRoots[:]...)
	required = append(required, c.CallerCASRoots[:]...)
	for _, pair := range []realPostArtifactPair{c.QualificationPins, c.Instability, c.Preparation} {
		for _, p := range pair.Paths {
			if e := realPostCanonicalPath(p); e != nil {
				return c, e
			}
			required = append(required, filepath.Dir(p))
		}
	}
	if e := realPostCanonicalPath(c.CandidatesPath); e != nil {
		return c, e
	}
	required = append(required, filepath.Dir(c.CandidatesPath), filepath.Dir(path))
	for _, p := range c.ProtectedArtifactRoots {
		if e := realPostCanonicalPath(p); e != nil {
			return c, e
		}
	}
	for _, p := range required {
		if e := realPostCanonicalPath(p); e != nil {
			return c, e
		}
		found := false
		for _, q := range c.ProtectedArtifactRoots {
			if p == q {
				found = true
			}
		}
		if !found {
			return c, fmt.Errorf("missing exact protected artifact parent: %s", p)
		}
	}
	// Report is absent-only and must not overwrite any retained input. Its parent
	// must already exist. No output/report directory is created by this driver.
	if len(c.ReportPath) > 4096 || !filepath.IsAbs(c.ReportPath) || filepath.Clean(c.ReportPath) != c.ReportPath || !realPostOutsideTemp(c.ReportPath) {
		return c, fmt.Errorf("report path bound/policy")
	}
	if e := realPostCanonicalPath(filepath.Dir(c.ReportPath)); e != nil {
		return c, e
	}
	reportParent, e := publication.OpenObservedRoot(context.Background(), filepath.Dir(c.ReportPath))
	if e != nil {
		return c, e
	}
	if e = reportParent.Close(); e != nil {
		return c, e
	}
	if _, e := os.Lstat(c.ReportPath); !errors.Is(e, os.ErrNotExist) {
		return c, fmt.Errorf("report is not unambiguously absent")
	}
	for _, p := range c.ProtectedArtifactRoots {
		if realPostContains(p, c.ReportPath) {
			return c, fmt.Errorf("report inside protected artifacts")
		}
	}
	for _, p := range c.OutputRoots {
		if e := realPostCanonicalPath(p); e != nil {
			return c, e
		}
		if !realPostOutsideTemp(p) {
			return c, fmt.Errorf("temporary output root")
		}
		dir, e := os.Open(p)
		if e != nil {
			return c, e
		}
		entries, readErr := dir.ReadDir(1)
		closeErr := dir.Close()
		if readErr != nil && readErr != io.EOF {
			return c, errors.Join(readErr, closeErr)
		}
		if closeErr != nil {
			return c, closeErr
		}
		if len(entries) != 0 {
			return c, fmt.Errorf("driver requires fresh existing empty outputs")
		}
		if realPostContains(p, c.ReportPath) {
			return c, fmt.Errorf("report inside publication output")
		}
	}
	return c, nil
}

// Only fields used by CALLER_CONVERSION_V1 are selected from the exact pinned
// candidate artifact; unselected metadata remains unchanged in that artifact.
// This is not an open metadata-shape importer: the whole input SHA is fixed.
type realPostClaim struct {
	Text            string   `json:"text"`
	EvidenceHandles []string `json:"evidence_handles"`
	SourceQuote     string   `json:"source_quote,omitempty"`
}
type realPostReference struct{ Namespace, PacketID, Handle string }

func (r *realPostReference) UnmarshalJSON(b []byte) error {
	var v struct {
		Namespace string `json:"namespace"`
		PacketID  string `json:"packet_id"`
		Handle    string `json:"handle"`
	}
	if e := realPostDecode(b, &v); e != nil {
		return e
	}
	r.Namespace, r.PacketID, r.Handle = v.Namespace, v.PacketID, v.Handle
	return nil
}

type realPostCandidate struct {
	CandidateID          string   `json:"candidate_id"`
	Revision             int      `json:"revision"`
	Status               string   `json:"status"`
	WorkingLabel         string   `json:"working_label"`
	SeedLabels           []string `json:"seed_labels"`
	TechnicalDescription string   `json:"technical_description"`
	InterpretationSHA256 string   `json:"interpretation_sha256"`
	ReviewSHA256         string   `json:"review_sha256"`
	TechnicalReview      string   `json:"technical_review"`
	Evidence             struct {
		SourceDeclarations []realPostReference `json:"source_declarations"`
	} `json:"evidence"`
	Claims             []realPostClaim `json:"claims"`
	Missing            json.RawMessage `json:"missing_or_failed_evidence"`
	Alternatives       json.RawMessage `json:"competing_interpretations"`
	RejectionCondition string          `json:"rejection_condition"`
	IndependentReview  json.RawMessage `json:"independent_review_qualifications"`
	AuthorityCeiling   struct {
		Authority int    `json:"authority"`
		Accepted  bool   `json:"accepted"`
		Feature   string `json:"feature_identity"`
		Source    string `json:"source_completeness"`
		Absence   string `json:"absence_inference"`
		Describe  string `json:"ordinary_describe_admission"`
		Native    string `json:"ordinary_native_workflow_admission"`
	} `json:"authority_ceiling"`
}
type realPostCandidates struct {
	Kind       string              `json:"kind"`
	Revision   int                 `json:"revision"`
	Count      int                 `json:"candidate_count"`
	Reviewed   int                 `json:"technical_reviewed_count"`
	Authority  int                 `json:"authority"`
	Accepted   bool                `json:"accepted"`
	Feature    string              `json:"feature_identity"`
	Source     string              `json:"source_completeness"`
	Absence    string              `json:"absence_inference"`
	Candidates []realPostCandidate `json:"candidates"`
}
type realPostProducer struct {
	Target               string          `json:"target"`
	PacketID             string          `json:"packet_id"`
	Status               string          `json:"status"`
	WorkingLabel         string          `json:"working_label"`
	TechnicalDescription string          `json:"technical_description"`
	Claims               []realPostClaim `json:"claims"`
	Limitations          json.RawMessage `json:"limitations"`
	Alternatives         json.RawMessage `json:"competing_interpretations"`
	RejectionCondition   string          `json:"rejection_condition"`
	Authority            int             `json:"authority"`
	Accepted             bool            `json:"accepted"`
	Feature              string          `json:"feature_identity"`
	Source               string          `json:"source_completeness"`
	Describe             string          `json:"ordinary_describe_admission"`
}
type realPostReview struct {
	Target           string          `json:"target"`
	PacketID         string          `json:"packet_id"`
	Verdict          string          `json:"verdict"`
	Findings         json.RawMessage `json:"findings"`
	SupportedClaims  json.RawMessage `json:"supported_claims"`
	RequiredRevision string          `json:"required_revision"`
	ClaimCeiling     json.RawMessage `json:"claim_ceiling"`
}

func realPostBareSHA(s string) bool {
	b, e := hex.DecodeString(s)
	return e == nil && len(b) == 32 && hex.EncodeToString(b) == s
}
func realPostCAS(roots [2]string, sha string) ([]byte, error) {
	if !realPostBareSHA(sha) {
		return nil, fmt.Errorf("bare caller CAS digest required")
	}
	return realPostReadPair(realPostArtifactPair{[2]string{filepath.Join(roots[0], sha), filepath.Join(roots[1], sha)}, "sha256:" + sha}, 1<<20)
}
func realPostJSONString(raw json.RawMessage) (string, error) {
	if len(raw) == 0 || len(raw) > 16<<10 || !json.Valid(raw) || !utf8.Valid(raw) {
		return "", fmt.Errorf("bounded JSONString required")
	}
	return string(raw), nil
}

// Preserve ceiling JSON bytes from the pinned review; validation does not
// reinterpret its contents as admission. Only its known string/object shapes
// are accepted, and empty/null/container-only metadata is not meaningful data.
func realPostValidateReviewCeiling(raw json.RawMessage) error {
	if len(raw) == 0 || len(raw) > 16<<10 || !utf8.Valid(raw) || !json.Valid(raw) {
		return fmt.Errorf("bounded valid original review ceiling required")
	}
	trimmed := bytes.TrimSpace(raw)
	if trimmed[0] == '"' {
		var s string
		if json.Unmarshal(raw, &s) == nil && strings.TrimSpace(s) != "" {
			return nil
		}
		return fmt.Errorf("nonblank original review ceiling string required")
	}
	if trimmed[0] != '{' {
		return fmt.Errorf("original review ceiling must be string or object")
	}
	var object map[string]any
	if json.Unmarshal(raw, &object) != nil {
		return fmt.Errorf("original review ceiling object")
	}
	var meaningful func(any, int) bool
	meaningful = func(v any, depth int) bool {
		if depth > 32 {
			return false
		}
		switch x := v.(type) {
		case string:
			return strings.TrimSpace(x) != ""
		case bool, float64:
			return true
		case map[string]any:
			for k, v := range x {
				if strings.TrimSpace(k) != "" && meaningful(v, depth+1) {
					return true
				}
			}
		case []any:
			for _, v := range x {
				if meaningful(v, depth+1) {
					return true
				}
			}
		}
		return false
	}
	if !meaningful(object, 0) {
		return fmt.Errorf("meaningful original review ceiling object required")
	}
	return nil
}

// One exact encoding used by both the opt-in driver and read-only size guard.
func realPostImportedFindings(v realPostCandidate, r realPostReview) ([]byte, error) {
	return json.Marshal(struct {
		Attribution, RawReviewSHA256                                                 string
		OriginalClaimCeiling                                                         json.RawMessage
		RequiredRevision                                                             string
		OriginalFindings, OriginalSupportedClaims, OriginalIndependentQualifications json.RawMessage
	}{"CALLER_REVIEW_IMPORT_V1_ONLY_CALLER_ASSERTED", v.ReviewSHA256, r.ClaimCeiling, r.RequiredRevision, r.Findings, r.SupportedClaims, v.IndependentReview})
}

func realPostConvert(c realPostCandidate, m prep.PacketManifest, prepSHA string, producerRaw, reviewRaw []byte) (prep.CallerSubmission, realPostReview, error) {
	var sub prep.CallerSubmission
	var producer realPostProducer
	var review realPostReview
	// Whole candidate and CAS bytes were hash pinned before this helper. Selected
	// original fields must still match; this conversion never fixes citations.
	if json.Unmarshal(producerRaw, &producer) != nil || realPostDecode(reviewRaw, &review) != nil {
		return sub, review, fmt.Errorf("original producer/review JSON")
	}
	if c.Revision != 1 || c.Status != "PROVISIONAL" || c.TechnicalReview != "SUPPORTED_WITH_LIMITS" || len(c.SeedLabels) != 1 || c.SeedLabels[0] != m.Target || producer.Target != m.Target || producer.PacketID != m.PacketID || review.Target != m.Target || review.PacketID != m.PacketID || review.Verdict != "SUPPORTED_WITH_LIMITS" || producer.Status != "DESCRIBED" {
		return sub, review, fmt.Errorf("original identity/status mismatch")
	}
	z := c.AuthorityCeiling
	if z.Authority != 0 || z.Accepted || z.Feature != "UNRESOLVED" || z.Source != "UNKNOWN" || z.Absence != "PROHIBITED" || z.Describe != "NOT_ATTEMPTED" || z.Native != "NOT_ATTEMPTED" || producer.Authority != 0 || producer.Accepted || producer.Feature != "UNRESOLVED" || producer.Describe != "NOT_ATTEMPTED" || (producer.Source != "" && producer.Source != "UNKNOWN") {
		return sub, review, fmt.Errorf("original identity ceiling mismatch")
	}
	if c.WorkingLabel != producer.WorkingLabel || c.TechnicalDescription != producer.TechnicalDescription || c.RejectionCondition != producer.RejectionCondition || !reflect.DeepEqual(c.Claims, producer.Claims) {
		return sub, review, fmt.Errorf("candidate/producer description mismatch")
	}
	if e := realPostValidateReviewCeiling(review.ClaimCeiling); e != nil {
		return sub, review, e
	}
	limitations, e := realPostJSONString(c.Missing)
	if e != nil {
		return sub, review, e
	}
	alternatives, e := realPostJSONString(c.Alternatives)
	if e != nil {
		return sub, review, e
	}
	// Preserve the pinned object-or-array as JSONString, not a semantic rewrite.
	if (c.Missing[0] != '[' && c.Missing[0] != '{') || c.Alternatives[0] != '[' {
		return sub, review, fmt.Errorf("fixed limitations/alternatives shapes")
	}
	handles := map[string]string{}
	for _, h := range m.Handles {
		handles[h.Handle] = h.Kind
	}
	sub = prep.CallerSubmission{PrivateCeilings: ownedPostCeilings(), PreparationSHA256: prepSHA, InputSHA256: m.InputSHA256, Target: m.Target, PacketID: m.PacketID, OrderedPartSHA256: append([]string{}, m.OrderedPartSHA256...), AuthorTag: "CALLER_CONVERSION_V1:producer-sha256:" + c.InterpretationSHA256, Disposition: "DESCRIBED", WorkingLabel: c.WorkingLabel, TechnicalDescription: c.TechnicalDescription, Limitations: limitations, Alternatives: alternatives, RejectionCondition: c.RejectionCondition}
	if len(c.Claims) == 0 || len(c.Claims) > 16 {
		return sub, review, fmt.Errorf("claim count")
	}
	for _, claim := range c.Claims {
		if len(claim.Text) > 16<<10 || len(claim.EvidenceHandles) == 0 || len(claim.EvidenceHandles) > 32 || len(claim.SourceQuote) > 240 || !utf8.ValidString(claim.SourceQuote) {
			return sub, review, fmt.Errorf("claim/quote bound")
		}
		quoteHandle := ""
		if claim.SourceQuote != "" {
			for _, ref := range claim.EvidenceHandles {
				for _, decl := range c.Evidence.SourceDeclarations {
					if ref == decl.Handle && decl.Namespace == "RETAINED_SOURCE_AST_DECLARATION" && decl.PacketID == m.PacketID && handles[ref] == "RETAINED_SOURCE_AST_DECLARATION" {
						if quoteHandle != "" && quoteHandle != ref {
							return sub, review, fmt.Errorf("ambiguous own AST quote mapping")
						}
						quoteHandle = ref
					}
				}
			}
			if quoteHandle == "" {
				return sub, review, fmt.Errorf("unmappable own AST quote; no citation added")
			}
		}
		converted := prep.TechnicalClaim{Text: claim.Text}
		for _, ref := range claim.EvidenceHandles {
			if _, ok := handles[ref]; !ok {
				return sub, review, fmt.Errorf("foreign evidence handle")
			}
			citation := prep.Citation{Handle: ref}
			if ref == quoteHandle {
				citation.SourceQuote = claim.SourceQuote
			}
			converted.Citations = append(converted.Citations, citation)
		}
		sub.Claims = append(sub.Claims, converted)
	}
	return sub, review, nil
}

// Pure conversion fixtures never open retained artifacts or construct producer
// seals. They exercise text/reference preservation and quote refusal only.
func TestRealPostCallerConversionMechanics(t *testing.T) {
	for _, kind := range []string{"exact", "quote-one-over", "unmappable", "foreign", "object-limitations", "object-ceiling"} {
		t.Run(kind, func(t *testing.T) {
			m := prep.PacketManifest{InputSHA256: "input", Target: "target", PacketID: "packet", OrderedPartSHA256: []string{"part"}, Handles: []prep.EvidenceHandle{{Handle: "own-ast", Kind: "RETAINED_SOURCE_AST_DECLARATION"}, {Handle: "own-call", Kind: "CALL_OCCURRENCE"}}}
			c := realPostCandidate{CandidateID: "historical-candidate", Revision: 1, Status: "PROVISIONAL", WorkingLabel: "label", SeedLabels: []string{"target"}, TechnicalDescription: "description", InterpretationSHA256: strings.Repeat("a", 64), ReviewSHA256: strings.Repeat("b", 64), TechnicalReview: "SUPPORTED_WITH_LIMITS", Missing: json.RawMessage(`["gap"]`), Alternatives: json.RawMessage(`["alternative"]`), RejectionCondition: "contrary evidence"}
			c.AuthorityCeiling.Feature = "UNRESOLVED"
			c.AuthorityCeiling.Source = "UNKNOWN"
			c.AuthorityCeiling.Absence = "PROHIBITED"
			c.AuthorityCeiling.Describe = "NOT_ATTEMPTED"
			c.AuthorityCeiling.Native = "NOT_ATTEMPTED"
			c.Evidence.SourceDeclarations = []realPostReference{{Namespace: "RETAINED_SOURCE_AST_DECLARATION", PacketID: "packet", Handle: "own-ast"}}
			c.Claims = []realPostClaim{{Text: "exact unchanged claim", EvidenceHandles: []string{"own-call", "own-ast"}, SourceQuote: strings.Repeat("q", 240)}}
			switch kind {
			case "quote-one-over":
				c.Claims[0].SourceQuote += "q"
			case "unmappable":
				c.Claims[0].EvidenceHandles = []string{"own-call"}
			case "foreign":
				c.Claims[0].EvidenceHandles = append(c.Claims[0].EvidenceHandles, "foreign")
			case "object-limitations":
				c.Missing = json.RawMessage(`{"source_completeness":"UNKNOWN"}`)
			}
			producer := realPostProducer{Target: "target", PacketID: "packet", Status: "DESCRIBED", WorkingLabel: c.WorkingLabel, TechnicalDescription: c.TechnicalDescription, Claims: c.Claims, RejectionCondition: c.RejectionCondition, Feature: "UNRESOLVED", Describe: "NOT_ATTEMPTED"}
			review := realPostReview{Target: "target", PacketID: "packet", Verdict: "SUPPORTED_WITH_LIMITS", Findings: json.RawMessage(`[]`), SupportedClaims: json.RawMessage(`[]`), ClaimCeiling: json.RawMessage(`"Original authority=0; accepted=false; no semantic admission"`)}
			if kind == "object-ceiling" {
				review.ClaimCeiling = json.RawMessage(`{"authority":0,"accepted":false,"feature_identity":"UNRESOLVED","source_completeness":"UNKNOWN","qualification":"untrusted original metadata"}`)
			}
			p, _ := json.Marshal(producer)
			r, _ := json.Marshal(review)
			sub, decoded, e := realPostConvert(c, m, "preparation", p, r)
			good := kind == "exact" || kind == "object-limitations" || kind == "object-ceiling"
			if !good {
				if e == nil {
					t.Fatal("unsafe conversion admitted")
				}
				return
			}
			if e != nil {
				t.Fatal(e)
			}
			if !bytes.Equal(decoded.ClaimCeiling, review.ClaimCeiling) {
				t.Fatal("original raw ceiling changed")
			}
			findings, e := realPostImportedFindings(c, decoded)
			if e != nil {
				t.Fatal(e)
			}
			var imported struct{ OriginalClaimCeiling json.RawMessage }
			if e = json.Unmarshal(findings, &imported); e != nil {
				t.Fatal(e)
			}
			if !bytes.Equal(imported.OriginalClaimCeiling, review.ClaimCeiling) || sub.Authority != 0 || sub.PrivateCeilings != ownedPostCeilings() {
				t.Fatal("ceiling type/value changed or caller authority escalated")
			}
			if sub.Claims[0].Text != c.Claims[0].Text || sub.Claims[0].Citations[0].Handle != "own-call" || sub.Claims[0].Citations[0].SourceQuote != "" || sub.Claims[0].Citations[1].SourceQuote != c.Claims[0].SourceQuote || sub.Limitations != string(c.Missing) || sub.Alternatives != string(c.Alternatives) || !strings.Contains(sub.AuthorTag, c.InterpretationSHA256) || sub.Accepted {
				t.Fatal("conversion normalized or elevated source")
			}
		})
	}
}

func TestRealPostReviewCeilingShapes(t *testing.T) {
	cases := []struct {
		name  string
		raw   json.RawMessage
		valid bool
	}{
		{"string", json.RawMessage(`"authority=0; accepted=false"`), true},
		{"object", json.RawMessage(`{"authority":0,"accepted":false}`), true},
		{"null", json.RawMessage(`null`), false},
		{"array", json.RawMessage(`["ceiling"]`), false},
		{"number", json.RawMessage(`0`), false},
		{"bool", json.RawMessage(`false`), false},
		{"blank", json.RawMessage(`"  "`), false},
		{"empty-object", json.RawMessage(`{}`), false},
		{"empty-data", json.RawMessage(`{"qualifications":[null,"",{}]}`), false},
		{"blank-key", json.RawMessage(`{" ":"data"}`), false},
		{"invalid", json.RawMessage(`{"authority":`), false},
		{"utf8", json.RawMessage{'"', 0xff, '"'}, false},
		{"empty", nil, false},
		{"bound-equal", json.RawMessage(`"` + strings.Repeat("x", (16<<10)-2) + `"`), true},
		{"bound-over", json.RawMessage(`"` + strings.Repeat("x", (16<<10)-1) + `"`), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := realPostValidateReviewCeiling(tc.raw)
			if (e == nil) != tc.valid {
				t.Fatalf("ceiling shape admission=%v expected=%v: %v", e == nil, tc.valid, e)
			}
		})
	}
	var review realPostReview
	if e := realPostDecode([]byte(`{"target":"x","claim_ceiling":{},"unknown":true}`), &review); e == nil {
		t.Fatal("strict review keys relaxed")
	}
	if e := realPostDecode([]byte(`{"required_revision":null,"claim_ceiling":{"authority":0}}`), &review); e != nil || review.RequiredRevision != "" {
		t.Fatal("null required revision compatibility", e)
	}
}

func TestRealPostReadPairDistinctArtifacts(t *testing.T) {
	for _, kind := range []string{"distinct", "same-path", "hardlink", "corrupt"} {
		t.Run(kind, func(t *testing.T) {
			base, e := filepath.EvalSymlinks(t.TempDir())
			if e != nil {
				t.Fatal(e)
			}
			a, b := filepath.Join(base, "a"), filepath.Join(base, "b")
			raw := []byte(`{"owned":"artifact"}`)
			if e = os.WriteFile(a, raw, 0600); e != nil {
				t.Fatal(e)
			}
			switch kind {
			case "same-path":
				b = a
			case "hardlink":
				if e = os.Link(a, b); e != nil {
					t.Fatal(e)
				}
			default:
				other := raw
				if kind == "corrupt" {
					other = []byte(`{"corrupt":true}`)
				}
				if e = os.WriteFile(b, other, 0600); e != nil {
					t.Fatal(e)
				}
			}
			got, e := realPostReadPair(realPostArtifactPair{Paths: [2]string{a, b}, SHA256: pilot.Digest(raw)}, 4096)
			switch kind {
			case "distinct":
				if e != nil || !bytes.Equal(got, raw) {
					t.Fatal("distinct equal pair refused", e)
				}
			case "same-path", "hardlink":
				if !errors.Is(e, errRealPostPairAlias) {
					t.Fatal("expected specific physical alias refusal", e)
				}
			case "corrupt":
				if e == nil || e.Error() != "paired artifact SHA mismatch" {
					t.Fatal("expected corrupt-copy SHA refusal", e)
				}
			}
		})
	}
}

func TestRealPostDistinctCASRoots(t *testing.T) {
	base, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	a, b := filepath.Join(base, "a"), filepath.Join(base, "b")
	for _, p := range []string{a, b} {
		if e = os.Mkdir(p, 0700); e != nil {
			t.Fatal(e)
		}
	}
	if e = realPostDistinctCASRoots([2]string{a, b}); e != nil {
		t.Fatal(e)
	}
	if e = realPostDistinctCASRoots([2]string{a, a}); !errors.Is(e, errRealPostCASAlias) {
		t.Fatal("expected specific CAS alias refusal", e)
	}
	c := realPostConfig{Schema: "post-publication-real-caller.private.v1", QualificationPins: realPostArtifactPair{SHA256: realPostPinsSHA}, Instability: realPostArtifactPair{SHA256: realPostAssessmentSHA}, Preparation: realPostArtifactPair{SHA256: realPostPreparationSHA}, CandidatesSHA256: realPostCandidatesSHA, CallerCASRoots: [2]string{a, a}, ProtectedArtifactRoots: []string{base}}
	encoded, e := json.Marshal(c)
	if e != nil {
		t.Fatal(e)
	}
	config := filepath.Join(base, "config.json")
	if e = os.WriteFile(config, encoded, 0600); e != nil {
		t.Fatal(e)
	}
	if _, e = realPostConfigRead(config); !errors.Is(e, errRealPostCASAlias) {
		t.Fatal("config did not preflight CAS alias", e)
	}
}

// Separate explicit opt-in: READ-ONLY artifact accounting, never the real
// driver. Decoded preparation packets supply MECHANICAL-ONLY untrusted DTO
// manifests here; they confer no capability seal and cannot be published.
func TestRealPostReadOnlyPinnedConversionSizing(t *testing.T) {
	if os.Getenv("LSP_TRACE_POST_PUBLICATION_READONLY_CONVERSION") != "1" {
		t.Skip("explicit read-only pinned-artifact conversion accounting opt-in required")
	}
	home, e := os.UserHomeDir()
	if e != nil {
		t.Fatal(e)
	}
	base := filepath.Join(home, "Library", "Application Support")
	old := filepath.Join(base, "lsp-trace-private-target-interpretation-n4wbs2pu")
	candidatesRaw, e := realPostRead(filepath.Join(old, "provisional-candidates-v1", "candidates."+realPostCandidatesSHA[7:]+".json"), 4<<20)
	if e != nil {
		t.Fatal(e)
	}
	if pilot.Digest(candidatesRaw) != realPostCandidatesSHA {
		t.Fatal("candidate SHA mismatch")
	}
	var candidates realPostCandidates
	if e = json.Unmarshal(candidatesRaw, &candidates); e != nil {
		t.Fatal(e)
	}
	if len(candidates.Candidates) != 25 {
		t.Fatal("pinned count")
	}
	preparationBase := filepath.Join(base, "lsp-trace-private-target-preparation-bound-dv27oj35")
	name := "preparationresult." + realPostPreparationSHA[7:] + ".json"
	preparationRaw, e := realPostReadPair(realPostArtifactPair{Paths: [2]string{filepath.Join(preparationBase, "primary", name), filepath.Join(preparationBase, "backup", name)}, SHA256: realPostPreparationSHA}, 8<<20)
	if e != nil {
		t.Fatal(e)
	}
	var mechanical prep.Result
	if e = json.Unmarshal(preparationRaw, &mechanical); e != nil {
		t.Fatal(e)
	}
	var manifests []prep.PacketManifest
	for _, packet := range mechanical.Packets {
		m := prep.PacketManifest{InputSHA256: packet.InputSHA256, Target: packet.Target, PacketID: packet.ID, OrderedPartSHA256: packet.PartSHA256}
		for _, part := range packet.Parts {
			for _, record := range part.Records {
				m.Handles = append(m.Handles, prep.EvidenceHandle{Handle: record.Handle, Kind: record.Kind})
			}
		}
		manifests = append(manifests, m)
	}
	roots := [2]string{filepath.Join(old, "retained-primary"), filepath.Join(old, "retained-backup")}
	if e = realPostDistinctCASRoots(roots); e != nil {
		t.Fatal(e)
	}
	producers := make([][]byte, 25)
	reviews := make([][]byte, 25)
	for i, v := range candidates.Candidates {
		producers[i], e = realPostCAS(roots, v.InterpretationSHA256)
		if e != nil {
			t.Fatal(e)
		}
		reviews[i], e = realPostCAS(roots, v.ReviewSHA256)
		if e != nil {
			t.Fatal(e)
		}
	}
	stringsCount, objectsCount, converted, rejected, maxBytes := 0, 0, 0, 0, 0
	for i, v := range candidates.Candidates {
		var review realPostReview
		if e = realPostDecode(reviews[i], &review); e != nil {
			t.Fatal(e)
		}
		if e = realPostValidateReviewCeiling(review.ClaimCeiling); e != nil {
			t.Fatal(e)
		}
		if bytes.TrimSpace(review.ClaimCeiling)[0] == '{' {
			objectsCount++
		} else {
			stringsCount++
		}
		var manifest prep.PacketManifest
		for _, m := range manifests {
			if len(v.SeedLabels) == 1 && m.Target == v.SeedLabels[0] {
				manifest = m
				break
			}
		}
		_, review, e = realPostConvert(v, manifest, realPostPreparationSHA, producers[i], reviews[i])
		if e != nil {
			rejected++
			t.Logf("MECHANICAL_ONLY conversion rejection %s: %v", v.CandidateID, e)
			continue
		}
		findings, e := realPostImportedFindings(v, review)
		if e != nil {
			t.Fatal(e)
		}
		if len(findings) > maxBytes {
			maxBytes = len(findings)
		}
		if len(findings) > 16<<10 {
			rejected++
			t.Logf("MECHANICAL_ONLY bounded findings rejection %s bytes=%d", v.CandidateID, len(findings))
			continue
		}
		converted++
		t.Logf("MECHANICAL_ONLY %s imported_findings_bytes=%d", v.CandidateID, len(findings))
	}
	t.Logf("READ_ONLY decoded=25 ceilings_strings=%d ceilings_objects=%d converted=%d rejections=%d max_imported_findings_bytes=%d; no seal, recomputation, acquisition or publication", stringsCount, objectsCount, converted, rejected, maxBytes)
}

func TestReal25PrivatePostPublication(t *testing.T) {
	configPath := os.Getenv(realPostConfigEnv)
	if configPath == "" {
		t.Skip("explicit retained-artifact configuration required; no real operation by default")
	}
	c, e := realPostConfigRead(configPath)
	if e != nil {
		t.Fatal(e)
	}
	report := realPostReport{Schema: "post-publication-real-report.private.v1", PrivateCeilings: ownedPostCeilings(), Absence: "PROHIBITED", AcquisitionReplay: "NOT_ATTEMPTED", Recomputations: "NOT_ATTEMPTED"}
	ctx := context.Background()
	var closers []interface{ Close() error }
	closeAll := func() {
		for i := len(closers) - 1; i >= 0; i-- {
			if e := closers[i].Close(); e != nil {
				report.CloseErrors = append(report.CloseErrors, e.Error())
			}
		}
		closers = nil
	}
	phase := func(s string) { report.Phase = s; report.Phases = append(report.Phases, s) }
	run := func() error {
		phase("HASH_PAIR_VERIFY_EXISTING_PINS_INPUTS")
		pinsRaw, e := realPostReadPair(c.QualificationPins, 32<<10)
		if e != nil {
			return e
		}
		var pins pilot.RetainedPartialQualificationPinsPrivate
		if e = realPostDecode(pinsRaw, &pins); e != nil {
			return e
		}
		assessmentRaw, e := realPostReadPair(c.Instability, 16<<20)
		if e != nil {
			return e
		}
		preparationRaw, e := realPostReadPair(c.Preparation, 8<<20)
		if e != nil {
			return e
		}
		candidateRaw, e := realPostRead(c.CandidatesPath, 4<<20)
		if e != nil {
			return e
		}
		if pilot.Digest(candidateRaw) != c.CandidatesSHA256 {
			return fmt.Errorf("fixed candidate SHA mismatch")
		}
		var candidates realPostCandidates
		if e = json.Unmarshal(candidateRaw, &candidates); e != nil {
			return e
		}
		if candidates.Kind != "EXTERNAL_PRIVATE_PROVISIONAL_CANDIDATES_NOT_ORDINARY_ADMISSION" || candidates.Revision != 1 || candidates.Count != 25 || candidates.Reviewed != 25 || len(candidates.Candidates) != 25 || candidates.Authority != 0 || candidates.Accepted || candidates.Feature != "UNRESOLVED" || candidates.Source != "UNKNOWN" || candidates.Absence != "PROHIBITED" {
			return fmt.Errorf("pinned candidate ceilings/shape")
		}
		// Verify ALL 50 original producer/review records on BOTH CAS roots before any
		// candidate conversion. Bare hex is the historical namespace, not sha256:.
		producers := make([][]byte, 25)
		reviewsRaw := make([][]byte, 25)
		for i, v := range candidates.Candidates {
			producers[i], e = realPostCAS(c.CallerCASRoots, v.InterpretationSHA256)
			if e != nil {
				return e
			}
			reviewsRaw[i], e = realPostCAS(c.CallerCASRoots, v.ReviewSHA256)
			if e != nil {
				return e
			}
		}
		var original [3]*publication.ContentRetentionRoot
		var partial [2]*publication.ContentRetentionRoot
		for i, path := range c.OriginalRoots {
			r, e := publication.OpenContentRetentionRoot(ctx, path)
			if e != nil {
				return e
			}
			original[i] = r
			closers = append(closers, r)
		}
		for i, path := range c.PartialRoots {
			r, e := publication.OpenContentRetentionRoot(ctx, path)
			if e != nil {
				return e
			}
			partial[i] = r
			closers = append(closers, r)
		}
		// This constructor and qualifier check the exact eight pins plus the retained
		// input locator/head/scope/metadata. No CLAIMED head execution is recovered.
		phase("NEW_EXACT_RETAINED_PARTIAL_QUALIFICATION_INPUT")
		input, e := pilot.NewExactRetainedPartialQualificationInput(pinsRaw, original, partial)
		if e != nil {
			return e
		}
		phase("QUALIFY_RETAINED_PARTIAL_NATIVE_PRIVATE")
		qualified, e := pilot.QualifyRetainedPartialNativePrivate(ctx, input)
		if e != nil {
			return e
		}
		phase("CLOSE_BORROWED_FIVE_ROOTS")
		closeAll()
		if len(report.CloseErrors) != 0 {
			return fmt.Errorf("qualification root close failure")
		}
		phase("DERIVE_RETAINED_PARTIAL_NATIVE_PRIVATE")
		proof, e := pilot.DeriveRetainedPartialNativePrivate(ctx, qualified, pins, pilot.RetainedPartialDerivationBudgetPrivate{MaxInputBytes: 8 << 20, MaxOutputBytes: 8 << 20, MaxWorkBytes: 2 << 30})
		if e != nil {
			return e
		}
		phase("PROJECT")
		projection, failure := derived.Project(proof)
		if failure != nil {
			return fmt.Errorf("projection: %v", failure)
		}
		phase("EXPLICIT_COMPUTE_INSTABILITY_SEEDS_42_THROUGH_47")
		report.Recomputations = "EXPLICIT_FINITE_RECOMPUTATION_NOT_ACQUISITION_REPLAY"
		assessment, e := derived.ComputeInstabilityPrivate(ctx, projection, []uint64{42, 43, 44, 45, 46, 47})
		if e != nil {
			return e
		}
		phase("COMPARE_FULL_RETAINED_ASSESSMENT")
		if e = derived.ValidateInstabilityPrivate(ctx, projection, assessment, assessmentRaw); e != nil {
			return e
		}
		phase("NEW_POST_PREPARATION_EXACT_ORIGINAL_LIMITS")
		originalLimits := prep.Limits{MaxTargets: 64, MaxPartBytes: 64 << 10, MaxTotalBytes: 8 << 20, MaxParts: 256, MaxSourceBytes: 2 << 20}
		postLimits := ownedPostLimits()
		postLimits.MaxStringBytes = 16 << 10
		h, e := prep.NewPostPreparation(ctx, []prep.Input{{Projection: projection, Instability: assessment}}, originalLimits, postLimits)
		if e != nil {
			return e
		}
		prepared, e := h.PreparedBytes()
		if e != nil {
			return e
		}
		prepSHA, e := h.PreparationSHA256()
		if e != nil {
			return e
		}
		if prepSHA != realPostPreparationSHA || !bytes.Equal(prepared, preparationRaw) {
			return fmt.Errorf("original canonical preparation mismatch; no partition or scope normalization permitted")
		}
		phase("CALLER_CONVERSION_V1")
		manifests, e := h.Manifest()
		if e != nil {
			return e
		}
		var subs []prep.CallerSubmission
		var reviews []prep.CallerReview
		var convertedCandidates []realPostCandidate
		var rawReviews []realPostReview
		seen := map[string]bool{}
		for i, v := range candidates.Candidates {
			if len(v.SeedLabels) != 1 || seen[v.SeedLabels[0]] {
				report.ConversionRejections = append(report.ConversionRejections, v.CandidateID+": duplicate/missing seed")
				continue
			}
			seen[v.SeedLabels[0]] = true
			var m prep.PacketManifest
			found := false
			for _, manifest := range manifests {
				if manifest.Target == v.SeedLabels[0] {
					m = manifest
					found = true
					break
				}
			}
			if !found {
				report.ConversionRejections = append(report.ConversionRejections, v.CandidateID+": no exact prepared manifest")
				continue
			}
			sub, review, e := realPostConvert(v, m, prepSHA, producers[i], reviewsRaw[i])
			if e != nil {
				report.ConversionRejections = append(report.ConversionRejections, v.CandidateID+": "+e.Error())
				continue
			}
			subs = append(subs, sub)
			convertedCandidates = append(convertedCandidates, v)
			rawReviews = append(rawReviews, review)
		}
		phase("RECONCILE_CANONICAL_SUBMISSION_SHA")
		first, e := h.Reconcile(ctx, subs, nil)
		if e != nil {
			return e
		}
		for i, sub := range subs {
			v := convertedCandidates[i]
			r := rawReviews[i]
			sha := ""
			for _, target := range first.Targets {
				if target.InputSHA256 == sub.InputSHA256 && target.Target == sub.Target {
					sha = target.SubmissionSHA256
				}
			}
			findings, e := realPostImportedFindings(v, r)
			if e != nil || len(findings) > 16<<10 {
				report.ConversionRejections = append(report.ConversionRejections, v.CandidateID+": bounded imported review findings exceeded")
				continue
			}
			reviews = append(reviews, prep.CallerReview{PrivateCeilings: ownedPostCeilings(), PreparationSHA256: prepSHA, InputSHA256: sub.InputSHA256, Target: sub.Target, SubmissionSHA256: sha, ReviewerAuthorTag: "CALLER_REVIEW_IMPORT_V1:review-sha256:" + v.ReviewSHA256, Verdict: "SUPPORT_WITH_LIMITS", Findings: string(findings)})
			report.Crosswalk = append(report.Crosswalk, realPostCrosswalk{CandidateID: v.CandidateID, ProducerSHA256: v.InterpretationSHA256, ReviewSHA256: v.ReviewSHA256, InputSHA256: sub.InputSHA256, Target: sub.Target, SubmissionSHA256: sha})
		}
		ledger, e := h.Reconcile(ctx, subs, reviews)
		if e != nil {
			return e
		}
		report.Ledger = &ledger
		for i := range report.Crosswalk {
			for _, lead := range ledger.Leads {
				if lead.Submission.InputSHA256 == report.Crosswalk[i].InputSHA256 && lead.Submission.Target == report.Crosswalk[i].Target {
					report.Crosswalk[i].NewLeadID = lead.ID
				}
			}
		}
		phase("OPEN_PROTECTED_AND_FRESH_OUTPUT_CAPABILITIES")
		var protected []*publication.ObservedRoot
		var outputs [2]*publication.ContentRetentionRoot
		for _, path := range c.ProtectedArtifactRoots {
			r, e := publication.OpenObservedRoot(ctx, path)
			if e != nil {
				return e
			}
			protected = append(protected, r)
			closers = append(closers, r)
		}
		for i, path := range c.OutputRoots {
			r, e := publication.OpenContentRetentionRoot(ctx, path)
			if e != nil {
				return e
			}
			outputs[i] = r
			closers = append(closers, r)
		}
		phase("RETAIN_ONCE")
		out, e := prep.RetainPostPreparationPrivate(ctx, h, subs, reviews, outputs, protected)
		report.Outcome = &out
		return e
	}
	e = run()
	if e != nil {
		report.Errors = append(report.Errors, e.Error())
	}
	closeAll() // every borrowed root closes AFTER all its calls, before report.
	if len(report.CloseErrors) != 0 {
		e = errors.Join(e, fmt.Errorf("root close failures"))
	}
	raw, encodeErr := json.Marshal(report)
	if encodeErr != nil || len(raw) > 16<<20 {
		t.Fatal("bounded report encoding", encodeErr)
	}
	root, openErr := os.OpenRoot(filepath.Dir(c.ReportPath))
	if openErr != nil {
		t.Fatal(openErr)
	}
	f, openErr := root.OpenFile(filepath.Base(c.ReportPath), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if openErr != nil {
		t.Fatal(errors.Join(openErr, root.Close()))
	}
	n, writeErr := f.Write(raw)
	if n != len(raw) && writeErr == nil {
		writeErr = io.ErrShortWrite
	}
	reportErr := errors.Join(writeErr, f.Sync(), f.Close(), root.Close())
	if reportErr != nil {
		t.Error("absent-only report finalization (not retroactively included in report bytes)", reportErr)
	}
	if e != nil {
		t.Fatal("private real operation stopped; partial report retained", e)
	}
	t.Logf("caller conversion only: %d provisional leads; %d conversion rejections; no forced count or semantic acceptance", len(report.Ledger.Leads), len(report.ConversionRejections))
}
