package describeworker

import "encoding/json"

const SuppliedTerminalResponseOnly = "SUPPLIED_TERMINAL_RESPONSE_ONLY"
const NoProcessOrHostExecutionVerified = "NO_PROCESS_OR_HOST_EXECUTION_VERIFIED"

// SuppliedResponseLimits bounds caller-supplied bytes, not execution resources.
// The adapter ceiling accommodates JSON escaping of the existing 64 KiB text.
type SuppliedResponseLimits struct {
	MaxAdapterBytes  int
	MaxSemanticBytes int
}

func (l SuppliedResponseLimits) Valid() bool {
	return l.MaxAdapterBytes > 0 && l.MaxAdapterBytes <= 512*1024 && l.MaxSemanticBytes > 0 && l.MaxSemanticBytes <= 64*1024
}

// SuppliedTerminalCandidate is parse-only. It proves neither source admission,
// producer identity, HOST origin nor successful process execution.
type SuppliedTerminalCandidate struct{ state *suppliedTerminalState }
type suppliedTerminalState struct {
	semantic  SemanticResponseV2
	canonical []byte
	rawDigest string
}

// ParseSuppliedTerminalResponse never invokes a worker. Raw byte limits precede
// decoding, canonicalization and hashing. No reader or N+1 overread is involved.
func ParseSuppliedTerminalResponse(line []byte, limits SuppliedResponseLimits) (SuppliedTerminalCandidate, error) {
	bad := func(sub string) (SuppliedTerminalCandidate, error) {
		return SuppliedTerminalCandidate{}, failureWithSubcode(StageOutput, CodeOutputInvalid, sub)
	}
	if !limits.Valid() || len(line) > limits.MaxAdapterBytes || len(line) < 2 || line[len(line)-1] != '\n' || line[len(line)-2] == '\n' {
		return bad(subcodeOuterProtocolV2)
	}
	var outer workerResultV2
	// Decode only the bounded adapter first, so the semantic byte ceiling is
	// enforced before strictCanonical re-marshals it or semantic JSON is parsed.
	if err := json.Unmarshal(line[:len(line)-1], &outer); err != nil {
		return bad(subcodeOuterProtocolV2)
	}
	if len(outer.Text) > limits.MaxSemanticBytes {
		return bad(subcodeSemanticSyntaxV2)
	}
	if err := strictCanonical(line[:len(line)-1], &outer); err != nil || outer.Status != "COMPLETE" || outer.Text == "" || outer.Error != "" {
		return bad(subcodeOuterProtocolV2)
	}
	semantic, canonical, err := ParseSemanticResponseV2([]byte(outer.Text))
	if err != nil {
		return bad(recordV2Subcode(err))
	}
	return SuppliedTerminalCandidate{&suppliedTerminalState{semantic, canonical, digestOfBytesV2(line)}}, nil
}

func (c SuppliedTerminalCandidate) RawAdapterDigest() string {
	if c.state == nil {
		return ""
	}
	return c.state.rawDigest
}
func (c SuppliedTerminalCandidate) CanonicalSemanticDigest() string {
	if c.state == nil {
		return ""
	}
	return digestOfBytesV2(c.state.canonical)
}
func (c SuppliedTerminalCandidate) CanonicalSemantic() []byte {
	if c.state == nil {
		return nil
	}
	return append([]byte(nil), c.state.canonical...)
}
func (c SuppliedTerminalCandidate) Semantic() SemanticResponseV2 {
	if c.state == nil {
		return SemanticResponseV2{}
	}
	return cloneSemanticV2(c.state.semantic)
}

// MarshalJSON deliberately exports only a non-admitting description. JSON
// round trips cannot reconstruct the private candidate state.
func (c SuppliedTerminalCandidate) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Origin       string `json:"origin"`
		Execution    string `json:"execution"`
		Authority    int    `json:"authority"`
		Accepted     bool   `json:"accepted"`
		Identity     string `json:"identity"`
		Completeness string `json:"completeness"`
		Absence      string `json:"absence"`
	}{SuppliedTerminalResponseOnly, NoProcessOrHostExecutionVerified, 0, false, "UNRESOLVED", "UNKNOWN", "PROHIBITED"})
}
