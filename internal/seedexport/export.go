// Package seedexport projects validated capture inputs into a transport-neutral
// representation of the original requested acquisition targets.
package seedexport

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

const (
	OutcomeAvailable    Outcome = "AVAILABLE"
	OutcomeNotAvailable Outcome = "NOT_AVAILABLE"
	OutcomeInvalid      Outcome = "INVALID"

	BodyCompletenessUnassessed = "UNASSESSED"
	maxDiagnostics             = 8
)

type Outcome string

type LocatorKind string

const (
	LocatorPosition LocatorKind = "POSITION"
	LocatorSymbol   LocatorKind = "SYMBOL"
)

type PathSemantics string

const (
	PathSemanticsURI  PathSemantics = "URI"
	PathSemanticsPath PathSemantics = "PATH"
)

type Position struct {
	Line      uint32
	Character uint32
}

type Locator struct {
	Kind          LocatorKind
	Value         string
	PathSemantics PathSemantics
	Position      *Position
	Symbol        string
	LanguageID    string
}

type Target struct {
	Ordinal   uint32
	ID        string
	Label     string
	Locator   Locator
	DownDepth int
	UpDepth   int
}

// Provenance contains references only. CustodyClaimed is always false in an
// available export; hash and identity equality do not authenticate custody.
type Provenance struct {
	SessionID            string
	Generation           uint64
	InvocationID         string
	Source               string
	Revision             string
	EnvelopeSHA256       string
	GraphV5SHA256        string
	ManifestSHA256       string
	ManifestSessionID    string
	ManifestGeneration   uint64
	ManifestInvocationID string
	CustodyClaimed       bool
}

type Model struct {
	Targets          []Target
	Provenance       Provenance
	BodyCompleteness string
}

type Diagnostic struct {
	Code    string
	Message string
}

type Result struct {
	Outcome     Outcome
	Model       *Model
	Diagnostics []Diagnostic
}

// ValidatedCarrier is populated only after the caller authoritatively validates
// the Graph Provenance V5 envelope and original acquisition request carrier.
type ValidatedCarrier struct {
	GraphProvenanceV5 json.RawMessage
	Targets           []Target
	Binding           Provenance
}

// ExportV5 cannot reconstruct original requested targets from V5 alone. In
// particular it never treats invocation seeds, graph nodes, frontier, CALLS,
// names, or source bodies as a substitute original-request carrier.
func ExportV5(_ []byte) Result {
	return notAvailable("ORIGINAL_REQUEST_CARRIER_MISSING", "validated original-request target carrier with exact locators and bounds is required")
}

// ExportValidated validates only export-critical attribution invariants. It
// performs no filesystem, network, LSP, Git, clock, or live-source operation.
func ExportValidated(c ValidatedCarrier) Result {
	if len(c.Targets) == 0 {
		return notAvailable("ORIGINAL_TARGETS_ABSENT", "original requested targets are not retained")
	}
	if c.Binding.ManifestSHA256 == "" || c.Binding.ManifestSessionID == "" || c.Binding.ManifestGeneration == 0 || c.Binding.ManifestInvocationID == "" {
		return notAvailable("MANIFEST_BINDING_MISSING", "validated manifest binding reference is incomplete")
	}
	if c.Binding.ManifestSessionID != c.Binding.SessionID || c.Binding.ManifestGeneration != c.Binding.Generation || c.Binding.ManifestInvocationID != c.Binding.InvocationID {
		return invalid("MANIFEST_BINDING_MISMATCH", "manifest and admitted capture identities differ")
	}
	if !digest(c.Binding.ManifestSHA256) {
		return invalid("MANIFEST_BINDING_MALFORMED", "manifest binding digest is malformed")
	}

	var envelope struct {
		SchemaVersion string `json:"schema_version"`
		SessionID     string `json:"session_id"`
		Generation    uint64 `json:"generation"`
		GraphV5       string `json:"graph_v5"`
		GraphSHA256   string `json:"graph_v5_sha256"`
	}
	if json.Unmarshal(c.GraphProvenanceV5, &envelope) != nil || envelope.SchemaVersion != "lsp-trace.graph-provenance.v5" {
		return invalid("PROVENANCE_MISMATCH", "admitted envelope identity is inconsistent")
	}
	envelopeSum := sha256.Sum256(c.GraphProvenanceV5)
	if c.Binding.EnvelopeSHA256 != fmt.Sprintf("sha256:%x", envelopeSum) || envelope.SessionID != c.Binding.SessionID || envelope.Generation != c.Binding.Generation {
		return invalid("CAPTURE_BINDING_MISMATCH", "admitted capture binding does not match the envelope")
	}
	graphBytes, err := base64.StdEncoding.DecodeString(envelope.GraphV5)
	if err != nil {
		return invalid("PROVENANCE_MISMATCH", "embedded graph encoding is inconsistent")
	}
	graphSum := sha256.Sum256(graphBytes)
	actualGraphDigest := fmt.Sprintf("sha256:%x", graphSum)
	if envelope.GraphSHA256 != actualGraphDigest || c.Binding.GraphV5SHA256 != actualGraphDigest {
		return invalid("PROVENANCE_MISMATCH", "embedded graph binding does not match the admitted capture")
	}
	var graph struct {
		SchemaVersion string `json:"schema_version"`
		Invocation    struct {
			Seeds []struct {
				Label string `json:"label"`
			} `json:"seeds"`
			Provenance struct {
				InvocationID string `json:"invocation_id"`
				Source       string `json:"source"`
				Revision     string `json:"source_revision"`
			} `json:"provenance"`
		} `json:"invocation"`
	}
	if json.Unmarshal(graphBytes, &graph) != nil || graph.SchemaVersion != "lsp-trace.graph.v5" {
		return invalid("PROVENANCE_MISMATCH", "embedded graph identity is inconsistent")
	}
	if graph.Invocation.Provenance.InvocationID != c.Binding.InvocationID || graph.Invocation.Provenance.Source != c.Binding.Source || graph.Invocation.Provenance.Revision != c.Binding.Revision {
		return invalid("CAPTURE_BINDING_MISMATCH", "invocation provenance does not match the admitted carrier")
	}
	if len(graph.Invocation.Seeds) != len(c.Targets) {
		return invalid("TARGET_ATTRIBUTION_MISMATCH", "target count does not match the admitted invocation")
	}

	seenID := make(map[string]bool, len(c.Targets))
	seenLabel := make(map[string]bool, len(c.Targets))
	seenOrdinal := make(map[uint32]bool, len(c.Targets))
	seedLabels := make(map[string]int, len(graph.Invocation.Seeds))
	for _, seed := range graph.Invocation.Seeds {
		seedLabels[seed.Label]++
	}
	out := append([]Target(nil), c.Targets...)
	for _, target := range out {
		if target.ID == "" || target.Label == "" || seenID[target.ID] {
			return invalid("DUPLICATE_TARGET", "target identity is absent or duplicated")
		}
		seenID[target.ID] = true
		if seenLabel[target.Label] {
			return invalid("DUPLICATE_TARGET_LABEL", "target label is duplicated")
		}
		seenLabel[target.Label] = true
		if seenOrdinal[target.Ordinal] || target.Ordinal >= uint32(len(out)) {
			return invalid("DUPLICATE_TARGET_ORDINAL", "target ordinal is duplicated or non-canonical")
		}
		seenOrdinal[target.Ordinal] = true
		if seedLabels[target.Label] != 1 {
			return invalid("TARGET_ATTRIBUTION_MISMATCH", "target label is not uniquely attributable to the invocation")
		}
		if target.DownDepth < 0 || target.DownDepth > 64 || target.UpDepth < 0 || target.UpDepth > 64 {
			return invalid("MALFORMED_BOUNDS", "target bounds must be between zero and 64")
		}
		if !validLocator(target.Locator) {
			return invalid("MALFORMED_LOCATOR", "target locator is incomplete or inconsistent")
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Ordinal < out[j].Ordinal })
	p := c.Binding
	p.CustodyClaimed = false
	return Result{Outcome: OutcomeAvailable, Model: &Model{Targets: out, Provenance: p, BodyCompleteness: BodyCompletenessUnassessed}, Diagnostics: []Diagnostic{}}
}

func validLocator(l Locator) bool {
	if strings.TrimSpace(l.Value) == "" || (l.PathSemantics != PathSemanticsURI && l.PathSemantics != PathSemanticsPath) {
		return false
	}
	switch l.Kind {
	case LocatorPosition:
		return l.Position != nil && l.Symbol == ""
	case LocatorSymbol:
		return l.Position == nil && strings.TrimSpace(l.Symbol) != ""
	default:
		return false
	}
}

func digest(s string) bool {
	if len(s) != 71 || !strings.HasPrefix(s, "sha256:") {
		return false
	}
	for _, r := range s[7:] {
		if !strings.ContainsRune("0123456789abcdef", r) {
			return false
		}
	}
	return true
}

func notAvailable(code, message string) Result {
	return result(OutcomeNotAvailable, code, message)
}

func invalid(code, message string) Result {
	return result(OutcomeInvalid, code, message)
}

func result(outcome Outcome, code, message string) Result {
	d := []Diagnostic{{Code: code, Message: message}}
	if len(d) > maxDiagnostics {
		d = d[:maxDiagnostics]
	}
	return Result{Outcome: outcome, Diagnostics: d}
}
