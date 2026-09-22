// Package describerequest deterministically renders validated TARGET packets as
// ADR-0007 Describe requests. It performs no ambient I/O.
package describerequest

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"

	"lsp-trace/internal/sourceprojection"
	"lsp-trace/internal/sourceprojectionv2"
	"lsp-trace/internal/targetpacket"
)

const (
	Protocol        = "adr0007-semantic-worker"
	MessageType     = "request"
	Corpus          = "revision-bound-code"
	ItemType        = "assembled-source-projection"
	AcquisitionMode = "TARGET"
	unresolvedID    = "OUTWARD_CONSUMER_UNRESOLVED"
	maxDeadlineMS   = 24 * 60 * 60 * 1000
)

type Envelope struct {
	Protocol        string `json:"protocol"`
	MessageType     string `json:"message_type"`
	MessageID       string `json:"message_id"`
	CorrelationID   string `json:"correlation_id"`
	AdmissionID     string `json:"admission_id"`
	Corpus          string `json:"corpus"`
	ItemType        string `json:"item_type"`
	AcquisitionMode string `json:"acquisition_mode"`
	InputSHA256     string `json:"input_sha256"`
	InputBytes      int    `json:"input_bytes"`
	SourceRevision  string `json:"source_revision"`
	DeadlineMS      int    `json:"deadline_ms"`
	TargetMessageID string `json:"target_message_id"`
	Prompt          string `json:"prompt"`
}

type Lineage struct {
	PacketID            string `json:"packet_id"`
	CensusID            string `json:"census_id"`
	LineageIdentity     string `json:"lineage_identity"`
	ConsumerResolution  string `json:"consumer_resolution"`
	AlternativeID       string `json:"alternative_id"`
	AlternativeOrdinal  int    `json:"alternative_ordinal"`
	Authority           int    `json:"authority"`
	Accepted            bool   `json:"accepted"`
	Completeness        string `json:"completeness"`
	SourceGraphComplete string `json:"source_graph_complete"`
}

type Record struct {
	RecordID string   `json:"record_id"`
	Envelope Envelope `json:"envelope"`
	Lineage  Lineage  `json:"lineage"`
}

func Build(result targetpacket.Result, deadlineMS int) ([]Record, error) {
	if deadlineMS <= 0 || deadlineMS > maxDeadlineMS {
		return nil, errors.New("deadline_ms outside permitted range")
	}
	packets := append([]targetpacket.Packet(nil), result.Packets...)
	sort.Slice(packets, func(i, j int) bool { return packets[i].PacketID < packets[j].PacketID })
	for i := 1; i < len(packets); i++ {
		if packets[i-1].PacketID == packets[i].PacketID {
			return nil, errors.New("duplicate packet id")
		}
	}
	out := make([]Record, 0, len(packets))
	for _, supplied := range packets {
		raw, err := json.Marshal(supplied)
		if err != nil {
			return nil, err
		}
		p, err := targetpacket.Validate(raw)
		if err != nil {
			return nil, fmt.Errorf("packet %q: %w", supplied.PacketID, err)
		}
		target, targetSpan, err := endpointSource(p, p.Lineage.SelectedNode, p.Lineage.SelectedLogicalSourceID)
		if err != nil {
			return nil, err
		}
		if len(p.ConsumerAlternatives) == 0 {
			r, err := render(p, nil, 0, target, targetSpan, deadlineMS)
			if err != nil {
				return nil, err
			}
			out = append(out, r)
			continue
		}
		for i := range p.ConsumerAlternatives {
			a := p.ConsumerAlternatives[i]
			caller, callerSpan, err := endpointSource(p, a.CallerID, a.CallerLogicalSourceID)
			if err != nil {
				return nil, err
			}
			if !reflect.DeepEqual(caller, a.CallerDisplay) {
				return nil, errors.New("consumer endpoint substitution")
			}
			r, err := renderResolved(p, a, i, target, targetSpan, caller, callerSpan, deadlineMS)
			if err != nil {
				return nil, err
			}
			out = append(out, r)
		}
	}
	return out, Validate(out)
}

// BuildV2 additively wraps historical mechanical packet bytes in the exact
// task-first Variant E prompt. Build remains byte-compatible.
func BuildV2(result targetpacket.Result, deadlineMS int) ([]Record, error) {
	out, err := Build(result, deadlineMS)
	if err != nil {
		return nil, err
	}
	for i := range out {
		out[i].Envelope.Prompt = VariantEPromptPrefix + out[i].Envelope.Prompt
		sum := sha256.Sum256([]byte(out[i].Envelope.Prompt))
		out[i].Envelope.InputSHA256 = hex.EncodeToString(sum[:])
		out[i].Envelope.InputBytes = len([]byte(out[i].Envelope.Prompt))
		out[i].RecordID, out[i].Envelope.MessageID, out[i].Envelope.CorrelationID, out[i].Lineage.LineageIdentity = "", "", "", ""
		out[i].Lineage.LineageIdentity = lineageIdentity(out[i])
		out[i].Envelope.MessageID = envelopeID("message", out[i])
		out[i].Envelope.CorrelationID = envelopeID("correlation", out[i])
		out[i].RecordID = recordID(out[i])
	}
	return out, Validate(out)
}

func endpointSource(p targetpacket.Packet, subject, logical string) (sourceprojectionv2.Unit, sourceprojection.Span, error) {
	var unit sourceprojectionv2.Unit
	count := 0
	for _, u := range p.Projection.Units {
		if u.Role == "ENDPOINT" && u.GraphSubjectID == subject && u.LogicalSourceID == logical {
			unit, count = u, count+1
		}
	}
	if count != 1 || unit.BodyDisposition != "RETURNED" || unit.Body == "" {
		return unit, sourceprojection.Span{}, errors.New("endpoint source unavailable or ambiguous")
	}
	var span sourceprojection.Span
	count = 0
	for _, s := range p.Projection.EmittedSpans {
		matches := 0
		for _, id := range s.UnitIDs {
			if id == unit.UnitID {
				matches++
			}
		}
		if matches == 1 {
			span, count = s, count+1
		}
	}
	if count != 1 || span.LogicalSourceID != logical || span.SourceDigest != unit.SourceDigest || span.ByteLength != len(span.Body) || !contains(span.Range, unit.DisplayRange) || !strings.Contains(span.Body, unit.Body) {
		return unit, span, errors.New("endpoint span mismatch")
	}
	return unit, span, nil
}

func contains(outer, inner sourceprojection.Range) bool {
	le := func(a, b sourceprojection.Position) bool {
		return a.Line < b.Line || a.Line == b.Line && a.Character <= b.Character
	}
	return le(outer.Start, inner.Start) && le(inner.End, outer.End)
}

func render(p targetpacket.Packet, _ *targetpacket.ConsumerAlternative, ordinal int, target sourceprojectionv2.Unit, targetSpan sourceprojection.Span, deadline int) (Record, error) {
	prompt := fmt.Sprintf("Task: Explain C1 TARGET. nearest_outward_consumer=OUTWARD_CONSUMER_UNRESOLVED; do not invent consumers. Explain only what the mechanically supplied source establishes. Do not claim feature identity, runtime use, ownership, product purpose, value, or completeness.\n\nMechanical metadata: authority=0; accepted=false; source_graph_complete=UNKNOWN.\nORDERED_CHAIN: OUTWARD_CONSUMER_UNRESOLVED -> C1 TARGET\n\nC1 TARGET graph_subject_id: %s\nC1 TARGET logical_source_id: %s\nC1 TARGET role: %s\nC1 TARGET source_span: %s\nC1 source:\n%s", target.GraphSubjectID, target.LogicalSourceID, target.Role, rangeText(targetSpan.Range), target.Body)
	return makeRecord(p, unresolvedID, ordinal, prompt, deadline)
}

func renderResolved(p targetpacket.Packet, a targetpacket.ConsumerAlternative, ordinal int, target sourceprojectionv2.Unit, targetSpan sourceprojection.Span, caller sourceprojectionv2.Unit, callerSpan sourceprojection.Span, deadline int) (Record, error) {
	prompt := fmt.Sprintf("Task: Explain C1 TARGET from mechanically supplied C2 OUTWARD_CONSUMER. C2 calls C1; consumer identity is not a model decision. Explain only what C1 provides to C2 and C1's one-layer contribution toward the system boundary. Do not claim feature identity, runtime use, ownership, product purpose, value, or completeness.\n\nMechanical metadata: authority=0; accepted=false; source_graph_complete=UNKNOWN.\nORDERED_CHAIN: C2 OUTWARD_CONSUMER -> C1 TARGET\nAlternative reconciliation_id: %s\nRelation id: %s\nOccurrence id: %s\nExact call_site_range: %s\n\nC1 TARGET graph_subject_id: %s\nC1 TARGET logical_source_id: %s\nC1 TARGET role: %s\nC1 TARGET source_span: %s\nC1 source:\n%s\n\nC2 OUTWARD_CONSUMER graph_subject_id: %s\nC2 OUTWARD_CONSUMER logical_source_id: %s\nC2 OUTWARD_CONSUMER role: %s\nC2 OUTWARD_CONSUMER source_span: %s\nC2 source:\n%s", a.ReconciliationID, a.RelationID, a.OccurrenceID, graphRangeText(a.CallSite), target.GraphSubjectID, target.LogicalSourceID, target.Role, rangeText(targetSpan.Range), target.Body, caller.GraphSubjectID, caller.LogicalSourceID, caller.Role, rangeText(callerSpan.Range), caller.Body)
	return makeRecord(p, a.ReconciliationID, ordinal, prompt, deadline)
}

func makeRecord(p targetpacket.Packet, alternativeID string, ordinal int, prompt string, deadline int) (Record, error) {
	promptSum := sha256.Sum256([]byte(prompt))
	r := Record{Envelope: Envelope{Protocol: Protocol, MessageType: MessageType, AdmissionID: p.CensusID, Corpus: Corpus, ItemType: ItemType, AcquisitionMode: AcquisitionMode, InputSHA256: hex.EncodeToString(promptSum[:]), InputBytes: len([]byte(prompt)), SourceRevision: strings.TrimPrefix(p.Custody.GraphDigest, "sha256:"), DeadlineMS: deadline, Prompt: prompt}, Lineage: Lineage{PacketID: p.PacketID, CensusID: p.CensusID, ConsumerResolution: string(p.ConsumerResolution), AlternativeID: alternativeID, AlternativeOrdinal: ordinal, Authority: 0, Accepted: false, Completeness: "UNKNOWN", SourceGraphComplete: "UNKNOWN"}}
	r.Lineage.LineageIdentity = lineageIdentity(r)
	r.Envelope.MessageID = envelopeID("message", r)
	r.Envelope.CorrelationID = envelopeID("correlation", r)
	r.RecordID = recordID(r)
	return r, nil
}

func envelopeID(kind string, r Record) string {
	r.RecordID, r.Envelope.MessageID, r.Envelope.CorrelationID = "", "", ""
	raw, _ := json.Marshal(r)
	s := sha256.Sum256(append([]byte("lsp-trace.describerequest."+kind+".v1\x00"), raw...))
	return hex.EncodeToString(s[:])
}

func lineageIdentity(r Record) string {
	r.RecordID, r.Envelope.MessageID, r.Envelope.CorrelationID = "", "", ""
	r.Lineage.LineageIdentity = ""
	raw, _ := json.Marshal(r)
	s := sha256.Sum256(append([]byte("lsp-trace.describerequest.lineage.v2\x00"), raw...))
	return hex.EncodeToString(s[:])
}

func recordID(r Record) string {
	r.RecordID = ""
	raw, _ := json.Marshal(r)
	s := sha256.Sum256(append([]byte("lsp-trace.describerequest.record.v1\x00"), raw...))
	return hex.EncodeToString(s[:])
}
func rangeText(r sourceprojection.Range) string {
	return fmt.Sprintf("%d:%d-%d:%d", r.Start.Line, r.Start.Character, r.End.Line, r.End.Character)
}
func graphRangeText(r interface{}) string { raw, _ := json.Marshal(r); return string(raw) }

// ValidateRecord checks intrinsic identity without requiring a preceding
// alternative. Batch ordering remains the responsibility of Validate.
func ValidateRecord(r Record) error {
	if r.Envelope.Protocol != Protocol || r.Envelope.MessageType != MessageType || r.Envelope.Corpus != Corpus || r.Envelope.ItemType != ItemType || r.Envelope.AcquisitionMode != AcquisitionMode || r.Envelope.DeadlineMS <= 0 || r.Envelope.DeadlineMS > maxDeadlineMS || r.Envelope.TargetMessageID != "" {
		return errors.New("envelope constants mismatch")
	}
	s := sha256.Sum256([]byte(r.Envelope.Prompt))
	if r.Lineage.AlternativeOrdinal < 0 || r.Envelope.AdmissionID == "" || r.Envelope.AdmissionID != r.Lineage.CensusID || r.Envelope.SourceRevision == "" || r.Lineage.PacketID == "" || r.Lineage.CensusID == "" || r.Lineage.AlternativeID == "" || r.Lineage.LineageIdentity != lineageIdentity(r) || r.Envelope.InputSHA256 != hex.EncodeToString(s[:]) || r.Envelope.InputBytes != len([]byte(r.Envelope.Prompt)) || r.Envelope.MessageID != envelopeID("message", r) || r.Envelope.CorrelationID != envelopeID("correlation", r) || r.Lineage.Authority != 0 || r.Lineage.Accepted || r.Lineage.Completeness != "UNKNOWN" || r.Lineage.SourceGraphComplete != "UNKNOWN" || r.RecordID != recordID(r) {
		return errors.New("record integrity mismatch")
	}
	return nil
}

func Validate(records []Record) error {
	for i, r := range records {
		if err := ValidateRecord(r); err != nil {
			return err
		}
		if i == 0 && r.Lineage.AlternativeOrdinal != 0 {
			return errors.New("noncanonical request order")
		}
		if i > 0 {
			previous := records[i-1]
			newPacket := previous.Lineage.PacketID != r.Lineage.PacketID
			if previous.Lineage.PacketID > r.Lineage.PacketID || (!newPacket && r.Lineage.AlternativeOrdinal != previous.Lineage.AlternativeOrdinal+1) || (newPacket && r.Lineage.AlternativeOrdinal != 0) {
				return errors.New("noncanonical request order")
			}
		}
	}
	return nil
}
func Bytes(records []Record) ([]byte, error) {
	if err := Validate(records); err != nil {
		return nil, err
	}
	return json.Marshal(records)
}
func NDJSON(records []Record) ([]byte, error) {
	if err := Validate(records); err != nil {
		return nil, err
	}
	var b bytes.Buffer
	for _, r := range records {
		raw, _ := json.Marshal(r.Envelope)
		b.Write(raw)
		b.WriteByte('\n')
	}
	return b.Bytes(), nil
}
func ParseNDJSON(raw []byte, expected []Record) ([]Envelope, error) {
	if len(raw) == 0 || raw[len(raw)-1] != '\n' || bytes.HasPrefix(raw, []byte{0xef, 0xbb, 0xbf}) {
		return nil, errors.New("noncanonical ndjson framing")
	}
	lines := bytes.Split(raw[:len(raw)-1], []byte{'\n'})
	if len(lines) != len(expected) {
		return nil, errors.New("ndjson record count mismatch")
	}
	out := make([]Envelope, len(lines))
	for i, line := range lines {
		if len(line) == 0 || bytes.ContainsAny(line, "\r\n") || rejectDuplicates(line) != nil {
			return nil, errors.New("invalid ndjson record")
		}
		d := json.NewDecoder(bytes.NewReader(line))
		d.DisallowUnknownFields()
		if err := d.Decode(&out[i]); err != nil {
			return nil, err
		}
		if err := d.Decode(&struct{}{}); err != io.EOF {
			return nil, errors.New("trailing json")
		}
		canonical, _ := json.Marshal(out[i])
		expectedRaw, _ := json.Marshal(expected[i].Envelope)
		if !bytes.Equal(line, canonical) || !bytes.Equal(line, expectedRaw) {
			return nil, errors.New("noncanonical or unexpected ndjson envelope")
		}
	}
	if err := Validate(expected); err != nil {
		return nil, err
	}
	return append([]Envelope(nil), out...), nil
}

func Parse(raw []byte) ([]Record, error) {
	if err := rejectDuplicates(raw); err != nil {
		return nil, err
	}
	var out []Record
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&out); err != nil {
		return nil, err
	}
	if err := d.Decode(&struct{}{}); err != io.EOF {
		return nil, errors.New("trailing json")
	}
	if err := Validate(out); err != nil {
		return nil, err
	}
	canonical, _ := json.Marshal(out)
	if !bytes.Equal(raw, canonical) {
		return nil, errors.New("noncanonical json")
	}
	return append(make([]Record, 0, len(out)), out...), nil
}

func rejectDuplicates(raw []byte) error {
	var v any
	d := json.NewDecoder(bytes.NewReader(raw))
	if err := d.Decode(&v); err != nil {
		return err
	}
	return scan(raw)
}
func scan(raw []byte) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	tok, err := d.Token()
	if err != nil {
		return err
	}
	x, ok := tok.(json.Delim)
	if !ok {
		return nil
	}
	if x == '{' {
		seen := map[string]bool{}
		for d.More() {
			k, err := d.Token()
			if err != nil {
				return err
			}
			key := k.(string)
			if seen[key] {
				return errors.New("duplicate json key")
			}
			seen[key] = true
			var child json.RawMessage
			if err = d.Decode(&child); err != nil {
				return err
			}
			if err = scan(child); err != nil {
				return err
			}
		}
		_, err = d.Token()
		return err
	}
	if x == '[' {
		for d.More() {
			var child json.RawMessage
			if err = d.Decode(&child); err != nil {
				return err
			}
			if err = scan(child); err != nil {
				return err
			}
		}
		_, err = d.Token()
		return err
	}
	return nil
}
