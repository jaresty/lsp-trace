package describerequest

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"lsp-trace/internal/targetpacket"
)

const PrivateSourceOnly = "PRIVATE_SOURCE_ONLY_NO_CONSUMER_CLAIM"

// FormatPrivateSourceRequestCandidate is ONLY a pure wire formatter. A valid
// Record is not producer completion, source admission or stage eligibility.
// The upper sourcecontinuation bridge calls it only after opaque producer admission.
func FormatPrivateSourceRequestCandidate(u targetpacket.PrivateSourceUnit, deadlineMS, maxBytes int) (Record, error) {
	bound := len(u.Body)*6 + 6*(len(u.URI)+len(u.Revision)+len(u.Privacy)+len(u.Workspace)+len(u.SessionID)+len(u.DocumentVersion)+len(u.PositionEncoding)+len(u.SymbolID)) + 16384
	if deadlineMS <= 0 || deadlineMS > maxDeadlineMS || maxBytes <= 0 || maxBytes > 8<<20 || bound > maxBytes {
		return Record{}, fmt.Errorf("private source request candidate bound")
	}
	if u.Version != targetpacket.PrivateSourceVersion || u.ClaimCeiling != targetpacket.PrivateSourceCeiling || u.Body == "" || u.ID == "" || u.SourceDigest == "" {
		return Record{}, fmt.Errorf("private source unit mismatch")
	}
	prompt := VariantEPromptPrefix + fmt.Sprintf("SOURCE_ONLY: independently admitted source unit; no CALLS, caller, call-site, cohort, grouping or consumer evidence is supplied. Consumer and feature identity remain UNRESOLVED. Do not infer runtime behavior, product purpose, global completeness or absence. authority=0; accepted=false; completeness=UNKNOWN; source_graph_complete=UNKNOWN.\nFull-definition provenance: SERVER_REPORTED_DOCUMENT_SYMBOL; method=textDocument/documentSymbol; position_encoding=%s; document_version=%s; source_digest=%s; display_range=%v; selection_range=%v. This classification is provider-reported, not an AST grammar or completeness proof.\nLogical source URI: %s\nC1 source:\n%s", u.PositionEncoding, u.DocumentVersion, u.SourceDigest, u.DisplayRange, u.SelectionRange, u.URI, u.Body)
	if len(prompt) > maxBytes {
		return Record{}, fmt.Errorf("private source prompt limit")
	}
	sum := sha256.Sum256([]byte(prompt))
	r := Record{Envelope: Envelope{Protocol: Protocol, MessageType: MessageType, AdmissionID: u.ID, Corpus: Corpus, ItemType: ItemType, AcquisitionMode: AcquisitionMode, InputSHA256: hex.EncodeToString(sum[:]), InputBytes: len(prompt), SourceRevision: strings.TrimPrefix(u.SourceDigest, "sha256:"), DeadlineMS: deadlineMS, Prompt: prompt}, Lineage: Lineage{PacketID: u.ID, CensusID: u.ID, ConsumerResolution: PrivateSourceOnly, AlternativeID: "SOURCE_ONLY", AlternativeOrdinal: 0, Authority: 0, Accepted: false, Completeness: "UNKNOWN", SourceGraphComplete: "UNKNOWN"}}
	r.Lineage.LineageIdentity = lineageIdentity(r)
	r.Envelope.MessageID = envelopeID("message", r)
	r.Envelope.CorrelationID = envelopeID("correlation", r)
	r.RecordID = recordID(r)
	return r, ValidateRecord(r)
}

// CanonicalPrivateSourceRequests deduplicates identical independently admitted
// source units while external private routing retains every original intent.
func CanonicalPrivateSourceRequests(records []Record) ([]Record, error) {
	if len(records) > 1000 {
		return nil, fmt.Errorf("private source request count")
	}
	out := append([]Record(nil), records...)
	sort.Slice(out, func(i, j int) bool { return out[i].Lineage.PacketID < out[j].Lineage.PacketID })
	n := 0
	for _, r := range out {
		if err := ValidateRecord(r); err != nil {
			return nil, err
		}
		if r.Lineage.ConsumerResolution != PrivateSourceOnly {
			return nil, fmt.Errorf("not source-only")
		}
		if n > 0 && out[n-1].Lineage.PacketID == r.Lineage.PacketID {
			if out[n-1].RecordID != r.RecordID {
				return nil, fmt.Errorf("source request identity conflict")
			}
			continue
		}
		out[n] = r
		n++
	}
	out = out[:n]
	return out, Validate(out)
}
