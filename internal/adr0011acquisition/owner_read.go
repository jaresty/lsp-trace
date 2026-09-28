package adr0011acquisition

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
	"lsp-trace/internal/adr0011requestkey"
	"lsp-trace/internal/publication"
	"lsp-trace/internal/strictjson"
	"lsp-trace/sessionruntime"
)

const ownerReadRole = "REFERENCES_OWNER_READ_OBSERVATION_V1"
const ownerReadLimit = 1500000

var errOwnerRead = errors.New("owner read not verified")

// ownerReadRecord is an inert, private observation; it grants no issuance authority.
type ownerReadRecord struct {
	SchemaVersion    string `json:"schema_version"`
	SessionID        string `json:"session_id"`
	Generation       uint64 `json:"generation"`
	RequestKey       string `json:"request_key"`
	InvocationID     string `json:"invocation_id"`
	Method           string `json:"method"`
	WireID           uint64 `json:"wire_id"`
	WriteSelector    string `json:"write_selector"`
	WriteDigest      string `json:"write_digest"`
	WriteByteLength  int    `json:"write_byte_length"`
	ParamsOffset     int    `json:"params_offset"`
	ParamsByteLength int    `json:"params_byte_length"`
	ParamsDigest     string `json:"params_digest"`
	ReadSelector     string `json:"read_selector"`
	ReadDigest       string `json:"read_digest"`
	ReadByteLength   int    `json:"read_byte_length"`
	ResultPresence   string `json:"result_presence"`
	ResultOffset     int    `json:"result_offset"`
	ResultByteLength int    `json:"result_byte_length"`
	ResultDigest     string `json:"result_digest"`
}

var ownerReadKeys = []string{"generation", "invocation_id", "method", "params_byte_length", "params_digest", "params_offset", "read_byte_length", "read_digest", "read_selector", "request_key", "result_byte_length", "result_digest", "result_offset", "result_presence", "schema_version", "session_id", "wire_id", "write_byte_length", "write_digest", "write_selector"}

// canonicalOwnerRead emits the restricted JCS profile: fixed UTF-16-sorted ASCII
// keys, NFC UTF-8 strings, shortest nonnegative safe integers, no floats.
func canonicalOwnerRead(r ownerReadRecord) ([]byte, error) {
	values := map[string]any{
		"generation": r.Generation, "invocation_id": r.InvocationID, "method": r.Method,
		"params_byte_length": r.ParamsByteLength, "params_digest": r.ParamsDigest, "params_offset": r.ParamsOffset,
		"read_byte_length": r.ReadByteLength, "read_digest": r.ReadDigest, "read_selector": r.ReadSelector,
		"request_key": r.RequestKey, "result_byte_length": r.ResultByteLength, "result_digest": r.ResultDigest,
		"result_offset": r.ResultOffset, "result_presence": r.ResultPresence, "schema_version": r.SchemaVersion,
		"session_id": r.SessionID, "wire_id": r.WireID, "write_byte_length": r.WriteByteLength,
		"write_digest": r.WriteDigest, "write_selector": r.WriteSelector,
	}
	var out bytes.Buffer
	out.WriteByte('{')
	for i, k := range ownerReadKeys {
		v := values[k]
		switch x := v.(type) {
		case string:
			if !utf8.ValidString(x) || !norm.NFC.IsNormalString(x) {
				return nil, errOwnerRead
			}
		case uint64:
			if x > 9007199254740991 {
				return nil, errOwnerRead
			}
		case int:
			if x < 0 || uint64(x) > 9007199254740991 {
				return nil, errOwnerRead
			}
		}
		if i > 0 {
			out.WriteByte(',')
		}
		out.WriteString(strconv.Quote(k))
		out.WriteByte(':')
		if text, ok := v.(string); ok {
			writeJCSString(&out, text)
		} else {
			out.WriteString(fmt.Sprint(v))
		}
	}
	out.WriteByte('}')
	if out.Len() > ownerReadLimit {
		return nil, errOwnerRead
	}
	return out.Bytes(), nil
}

func writeJCSString(out *bytes.Buffer, value string) {
	out.WriteByte('"')
	for _, r := range value {
		switch r {
		case '"':
			out.WriteString(`\"`)
		case '\\':
			out.WriteString(`\\`)
		case '\b':
			out.WriteString(`\b`)
		case '\t':
			out.WriteString(`\t`)
		case '\n':
			out.WriteString(`\n`)
		case '\f':
			out.WriteString(`\f`)
		case '\r':
			out.WriteString(`\r`)
		default:
			if r < 0x20 {
				fmt.Fprintf(out, `\u%04x`, r)
			} else {
				out.WriteRune(r)
			}
		}
	}
	out.WriteByte('"')
}

func ownerReadDigest(b []byte) string {
	material := append([]byte(ownerReadRole+"\x00"), b...)
	return fmt.Sprintf("sha256:%x", sha256.Sum256(material))
}
func ownerReadSelector(d string) string {
	return "adr0011-references-issuance-v1-owner-read-" + strings.TrimPrefix(d, "sha256:") + ".json"
}

// publishOwnerRead validates Manager capture and the two independent complete
// body publications before writing the closed observation. No caller can select
// the expected invocation or schema via the record.
func publishOwnerRead(root *publication.Root, frames ownedFrames, expected sessionruntime.OwnedMethodPair, binding sessionruntime.OwnedDocumentBinding, sessionID string, generation uint64, invocation, schemaDigest string, read func(*publication.Root, string, int64) ([]byte, error)) (privateBodyPublication, error) {
	absent := privateBodyPublication{stage: "ABSENT"}
	if root == nil || !verifyOriginalRequestKey(frames, expected, sessionID, generation, invocation, schemaDigest) || !replayOwnedFrames(frames, expected, binding, sessionID, generation, 1<<20) {
		return absent, errOwnerRead
	}
	writeSlice, ok := privateBodySliceFromFrame(frames.request, frames.paramsSpan)
	if !ok {
		return absent, errOwnerRead
	}
	readSlice, ok := privateBodySliceFromFrame(frames.response, frames.resultSpan)
	if !ok {
		return absent, errOwnerRead
	}
	writePub, err := publishPrivateBody(root, "write", writeSlice, read)
	if err != nil {
		return absent, errOwnerRead
	}
	readPub, err := publishPrivateBody(root, "read", readSlice, read)
	if err != nil {
		return absent, errOwnerRead
	}
	r := ownerReadRecord{SchemaVersion: ownerReadRole, SessionID: sessionID, Generation: generation, RequestKey: adr0011requestkey.Encode(expected.Key), InvocationID: invocation, Method: expected.Method, WireID: expected.Key.ID,
		WriteSelector: writePub.selector, WriteDigest: writePub.digest, WriteByteLength: writePub.byteCount, ParamsOffset: writeSlice.offset, ParamsByteLength: writeSlice.length, ParamsDigest: writeSlice.valueDigest,
		ReadSelector: readPub.selector, ReadDigest: readPub.digest, ReadByteLength: readPub.byteCount, ResultPresence: "PRESENT", ResultOffset: readSlice.offset, ResultByteLength: readSlice.length, ResultDigest: readSlice.valueDigest}
	raw, err := canonicalOwnerRead(r)
	if err != nil {
		return absent, errOwnerRead
	}
	digest := ownerReadDigest(raw)
	selector := ownerReadSelector(digest)
	receipt, err := publication.PublishBoundFile(root, selector, raw, func(got []byte) error {
		if !bytes.Equal(got, raw) {
			return errOwnerRead
		}
		return nil
	})
	if err != nil || receipt == nil {
		return absent, errOwnerRead
	}
	outcome := privateBodyPublication{selector: selector, digest: digest, byteCount: len(raw), stage: "COMMITTED_UNVERIFIED"}
	if receipt.VerificationStatus != "VERIFIED" || receipt.Mechanism != publication.BoundFileMechanism || receipt.DirectorySyncStatus != publication.DirectorySyncComplete || receipt.CloseStatus != publication.CloseComplete || receipt.FinalSelector != selector || receipt.Digest != privateDigest(raw) || receipt.ByteLength != uint64(len(raw)) || !receipt.NamespaceAtomic {
		return outcome, errOwnerRead
	}
	if read == nil {
		read = publication.ReadVerifiedBoundFile
	}
	got, err := read(root, selector, ownerReadLimit)
	if err != nil || !bytes.Equal(got, raw) {
		return outcome, errOwnerRead
	}
	if !replayOwnerRead(root, got, outcome, expected, binding, sessionID, generation, invocation, schemaDigest, read) {
		return outcome, errOwnerRead
	}
	outcome.stage = "VERIFIED"
	return outcome, nil
}

// ownerReadParams enforces the two closed method-specific shapes independently
// of the Manager pair's byte-equality assertion.
func ownerReadParams(raw []byte, method, uri string) bool {
	if strictjson.RejectDuplicates(raw) != nil {
		return false
	}
	var outer map[string]json.RawMessage
	if json.Unmarshal(raw, &outer) != nil || len(outer) != 1 && (method != "textDocument/references" || len(outer) != 3) {
		return false
	}
	var doc map[string]json.RawMessage
	if json.Unmarshal(outer["textDocument"], &doc) != nil || len(doc) != 1 {
		return false
	}
	var gotURI string
	if json.Unmarshal(doc["uri"], &gotURI) != nil || gotURI != uri {
		return false
	}
	if method == "textDocument/documentSymbol" {
		return len(outer) == 1
	}
	if method != "textDocument/references" || len(outer) != 3 {
		return false
	}
	var pos map[string]json.RawMessage
	if json.Unmarshal(outer["position"], &pos) != nil || len(pos) != 2 {
		return false
	}
	for _, field := range []string{"line", "character"} {
		var number uint32
		if json.Unmarshal(pos[field], &number) != nil || len(pos[field]) == 0 || pos[field][0] < '0' || pos[field][0] > '9' {
			return false
		}
	}
	var context map[string]json.RawMessage
	if json.Unmarshal(outer["context"], &context) != nil || len(context) != 1 || string(context["includeDeclaration"]) != "false" {
		return false
	}
	return true
}

// replayOwnerRead resolves original immutable bodies independently of claimant
// offsets. expected and invocation are selected outside the record.
func replayOwnerRead(root *publication.Root, raw []byte, ref privateBodyPublication, expected sessionruntime.OwnedMethodPair, binding sessionruntime.OwnedDocumentBinding, sessionID string, generation uint64, invocation, schemaDigest string, read func(*publication.Root, string, int64) ([]byte, error)) bool {
	if root == nil || read == nil || schemaDigest != reviewedSuccessorSchemaDigest || len(raw) == 0 || len(raw) > ownerReadLimit || strictjson.RejectDuplicates(raw) != nil || ref.selector != ownerReadSelector(ownerReadDigest(raw)) || ref.digest != ownerReadDigest(raw) || ref.byteCount != len(raw) {
		return false
	}
	stored, err := read(root, ref.selector, ownerReadLimit)
	if err != nil || !bytes.Equal(stored, raw) {
		return false
	}
	var r ownerReadRecord
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if dec.Decode(&r) != nil {
		return false
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return false
	}
	canonical, err := canonicalOwnerRead(r)
	if err != nil || !bytes.Equal(canonical, raw) {
		return false
	}
	if r.SchemaVersion != ownerReadRole || r.SessionID == "" || r.SessionID != sessionID || r.Generation == 0 || r.Generation != generation || r.InvocationID == "" || r.InvocationID != invocation || r.Method != expected.Method || r.WireID == 0 || r.WireID != expected.Key.ID || r.RequestKey != adr0011requestkey.Encode(expected.Key) || r.ResultPresence != "PRESENT" || expected.SessionID != sessionID || expected.Generation != generation || expected.Source == nil || *expected.Source != binding || binding.URI == "" || binding.Version <= 0 || binding.SHA256 == "" || expected.Key.Generation != generation || expected.Write.Key != expected.Key || expected.Read.Key != expected.Key || expected.Write.SessionID != sessionID || expected.Read.SessionID != sessionID || expected.Write.Generation != generation || expected.Read.Generation != generation || expected.Write.Method != r.Method {
		return false
	}
	if r.WriteByteLength <= 0 || r.WriteByteLength > 4194304 || r.ReadByteLength <= 0 || r.ReadByteLength > 4194304 || r.ParamsByteLength <= 0 || r.ParamsByteLength > 65536 || r.ResultByteLength <= 0 || r.ResultByteLength > 1048576 || r.WriteSelector != "adr0011-references-write-v1-"+strings.TrimPrefix(r.WriteDigest, "sha256:")+".bin" || r.ReadSelector != "adr0011-references-read-v1-"+strings.TrimPrefix(r.ReadDigest, "sha256:")+".bin" {
		return false
	}
	write, err := read(root, r.WriteSelector, 4194304)
	if err != nil || len(write) != r.WriteByteLength || privateDigest(write) != r.WriteDigest {
		return false
	}
	response, err := read(root, r.ReadSelector, 4194304)
	if err != nil || len(response) != r.ReadByteLength || privateDigest(response) != r.ReadDigest {
		return false
	}
	// Re-parse the original complete bodies. Synthetic framing is a parser input,
	// never a replacement source for retained evidence.
	frame := func(body []byte) []byte {
		return append([]byte(fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body))), body...)
	}
	requestFrame, responseFrame := frame(write), frame(response)
	p, ok := exactOwnedSpan(requestFrame, "params", r.Method, r.WireID)
	if !ok {
		return false
	}
	result, ok := exactOwnedSpan(responseFrame, "result", r.Method, r.WireID)
	if !ok {
		return false
	}
	if r.ParamsOffset != p.Offset-(len(requestFrame)-len(write)) || r.ParamsByteLength != p.Length || r.ParamsDigest != p.ValueDigest || !bytes.Equal(p.Value, expected.Params) || !ownerReadParams(p.Value, r.Method, binding.URI) || r.ResultOffset != result.Offset-(len(responseFrame)-len(response)) || r.ResultByteLength != result.Length || r.ResultDigest != result.ValueDigest || !bytes.Equal(result.Value, expected.Result) {
		return false
	}
	requestID, ok := exactJSONField(write, "id")
	if !ok {
		return false
	}
	responseID, ok := exactJSONField(response, "id")
	if !ok {
		return false
	}
	return adr0011requestkey.Identity(r.RequestKey, expected.Key, generation, r.WireID, requestID, responseID, r.InvocationID, invocation) == nil
}
