package adr0011acquisition

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"lsp-trace/internal/adr0011querytarget"
	"lsp-trace/internal/publication"
	"lsp-trace/internal/strictjson"
	"lsp-trace/sessionruntime"
)

const targetResultRole = "REFERENCES_TARGET_RESULT_V1"

var errTargetResult = errors.New("private target result not verified")

type targetResultRef struct {
	SchemaVersion string `json:"schema_version"`
	Selector      string `json:"selector"`
	Digest        string `json:"digest"`
}
type targetResultRecord struct {
	SchemaVersion     string          `json:"schema_version"`
	OwnerReadRef      targetResultRef `json:"owner_read_ref"`
	PayloadSelector   string          `json:"payload_selector"`
	PayloadDigest     string          `json:"payload_digest"`
	PayloadByteLength int             `json:"payload_byte_length"`
	Access            string          `json:"access"`
}

func targetResultDigest(b []byte) string {
	return fmt.Sprintf("sha256:%x", sha256.Sum256(append([]byte(targetResultRole+"\x00"), b...)))
}
func targetResultSelector(d string) string {
	return "adr0011-references-issuance-v1-target-result-" + strings.TrimPrefix(d, "sha256:") + ".json"
}
func targetPayloadSelector(d string) string {
	return "adr0011-references-target-result-v1-" + strings.TrimPrefix(d, "sha256:") + ".bin"
}
func targetResultValidDigest(s string) bool {
	if len(s) != 71 || !strings.HasPrefix(s, "sha256:") {
		return false
	}
	decoded, e := hex.DecodeString(s[7:])
	return e == nil && len(decoded) == 32 && "sha256:"+hex.EncodeToString(decoded) == s
}
func canonicalTargetResult(r targetResultRecord) ([]byte, error) {
	if !targetResultValidDigest(r.OwnerReadRef.Digest) || !targetResultValidDigest(r.PayloadDigest) {
		return nil, errTargetResult
	}
	if r.SchemaVersion != targetResultRole || r.Access != "OWNER_ONLY" || r.OwnerReadRef.SchemaVersion != ownerReadRole || r.OwnerReadRef.Selector != ownerReadSelector(r.OwnerReadRef.Digest) || r.PayloadSelector != targetPayloadSelector(r.PayloadDigest) || r.PayloadByteLength < 1 || r.PayloadByteLength > 1048576 {
		return nil, errTargetResult
	}
	// ASCII keys in UTF-16 code-unit order; the nested ref is likewise ordered.
	var out bytes.Buffer
	out.WriteString(`{"access":`)
	writeJCSString(&out, r.Access)
	out.WriteString(`,"owner_read_ref":{"digest":`)
	writeJCSString(&out, r.OwnerReadRef.Digest)
	out.WriteString(`,"schema_version":`)
	writeJCSString(&out, r.OwnerReadRef.SchemaVersion)
	out.WriteString(`,"selector":`)
	writeJCSString(&out, r.OwnerReadRef.Selector)
	out.WriteString(`},"payload_byte_length":`)
	fmt.Fprint(&out, r.PayloadByteLength)
	out.WriteString(`,"payload_digest":`)
	writeJCSString(&out, r.PayloadDigest)
	out.WriteString(`,"payload_selector":`)
	writeJCSString(&out, r.PayloadSelector)
	out.WriteString(`,"schema_version":`)
	writeJCSString(&out, r.SchemaVersion)
	out.WriteByte('}')
	return out.Bytes(), nil
}

// replayTargetResult has no caller-selected record or payload path. The owner
// expectation identifies the original request, query and response independently.
func replayTargetResult(root *publication.Root, ref privateBodyPublication, owner privateBodyPublication, expected sessionruntime.OwnedMethodPair, binding sessionruntime.OwnedDocumentBinding, sessionID string, generation uint64, invocation, schemaDigest string, query adr0011querytarget.Query, read func(*publication.Root, string, int64) ([]byte, error)) bool {
	if root == nil || ref.stage == "ABSENT" || owner.stage != "VERIFIED" || expected.Method != "textDocument/documentSymbol" || query.SessionID != sessionID || query.Generation != generation || query.URI != binding.URI || query.SourceDigest != binding.SHA256 || query.DocumentVersion != fmt.Sprint(binding.Version) || query.Encoding != "utf-16" {
		return false
	}
	if read == nil {
		read = publication.ReadVerifiedBoundFile
	}
	recordBytes, err := read(root, ref.selector, ownerReadLimit)
	if err != nil || strictjson.RejectDuplicates(recordBytes) != nil || ref.digest != targetResultDigest(recordBytes) || ref.selector != targetResultSelector(ref.digest) || ref.byteCount != len(recordBytes) {
		return false
	}
	var r targetResultRecord
	dec := json.NewDecoder(bytes.NewReader(recordBytes))
	dec.DisallowUnknownFields()
	if dec.Decode(&r) != nil {
		return false
	}
	var extra any
	if dec.Decode(&extra) != io.EOF {
		return false
	}
	canonical, e := canonicalTargetResult(r)
	if e != nil || !bytes.Equal(recordBytes, canonical) || r.OwnerReadRef != (targetResultRef{ownerReadRole, owner.selector, owner.digest}) {
		return false
	}
	ownerBytes, e := read(root, owner.selector, ownerReadLimit)
	if e != nil || !replayOwnerRead(root, ownerBytes, owner, expected, binding, sessionID, generation, invocation, schemaDigest, read) {
		return false
	}
	var observation ownerReadRecord
	if json.Unmarshal(ownerBytes, &observation) != nil {
		return false
	}
	body, e := read(root, observation.ReadSelector, privateBodyPublicationLimit)
	if e != nil || len(body) != observation.ReadByteLength || privateDigest(body) != observation.ReadDigest || observation.ResultOffset < 0 || observation.ResultOffset > len(body) || observation.ResultByteLength < 1 || observation.ResultByteLength > 1048576 || observation.ResultByteLength > len(body)-observation.ResultOffset {
		return false
	}
	raw := body[observation.ResultOffset : observation.ResultOffset+observation.ResultByteLength]
	if observation.ResultDigest != privateDigest(raw) || r.PayloadDigest != privateDigest(raw) || r.PayloadSelector != targetPayloadSelector(r.PayloadDigest) || r.PayloadByteLength != len(raw) {
		return false
	}
	slice := privateBodySlice{body: body, value: raw, offset: observation.ResultOffset, length: observation.ResultByteLength, bodyDigest: observation.ReadDigest, valueDigest: observation.ResultDigest}
	if !replayPrivateBodySlice(body, slice, "result", expected.Method, observation.WireID, raw) {
		return false
	}
	payload, e := read(root, r.PayloadSelector, 1048576)
	if e != nil || !bytes.Equal(payload, raw) {
		return false
	}
	_, e = adr0011querytarget.SelectDocumentSymbolCandidateV1(query, payload)
	return e == nil
}

func publishTargetResult(root *publication.Root, owner privateBodyPublication, expected sessionruntime.OwnedMethodPair, binding sessionruntime.OwnedDocumentBinding, sessionID string, generation uint64, invocation, schemaDigest string, query adr0011querytarget.Query, read func(*publication.Root, string, int64) ([]byte, error)) (privateBodyPublication, error) {
	absent := privateBodyPublication{stage: "ABSENT"}
	if root == nil || owner.stage != "VERIFIED" || expected.Method != "textDocument/documentSymbol" || query.SessionID != sessionID || query.Generation != generation || query.URI != binding.URI || query.SourceDigest != binding.SHA256 || query.DocumentVersion != fmt.Sprint(binding.Version) || query.Encoding != "utf-16" {
		return absent, errTargetResult
	}
	if read == nil {
		read = publication.ReadVerifiedBoundFile
	}
	ownerBytes, e := read(root, owner.selector, ownerReadLimit)
	if e != nil || !replayOwnerRead(root, ownerBytes, owner, expected, binding, sessionID, generation, invocation, schemaDigest, read) {
		return absent, errTargetResult
	}
	var observation ownerReadRecord
	if json.Unmarshal(ownerBytes, &observation) != nil {
		return absent, errTargetResult
	}
	body, e := read(root, observation.ReadSelector, privateBodyPublicationLimit)
	if e != nil || len(body) != observation.ReadByteLength || privateDigest(body) != observation.ReadDigest || observation.ResultOffset < 0 || observation.ResultOffset > len(body) || observation.ResultByteLength < 1 || observation.ResultByteLength > 1048576 || observation.ResultByteLength > len(body)-observation.ResultOffset {
		return absent, errTargetResult
	}
	raw := body[observation.ResultOffset : observation.ResultOffset+observation.ResultByteLength]
	slice := privateBodySlice{body: body, value: raw, offset: observation.ResultOffset, length: observation.ResultByteLength, bodyDigest: observation.ReadDigest, valueDigest: observation.ResultDigest}
	if observation.ResultDigest != privateDigest(raw) || !replayPrivateBodySlice(body, slice, "result", expected.Method, observation.WireID, raw) {
		return absent, errTargetResult
	}
	if _, e = adr0011querytarget.SelectDocumentSymbolCandidateV1(query, raw); e != nil {
		return absent, errTargetResult
	}
	digest := privateDigest(raw)
	selector := targetPayloadSelector(digest)
	receipt, e := publication.PublishBoundFile(root, selector, raw, func(got []byte) error {
		if !bytes.Equal(got, raw) {
			return errTargetResult
		}
		return nil
	})
	if e != nil || receipt == nil {
		return absent, errTargetResult
	}
	payloadState := privateBodyPublication{selector: selector, digest: digest, byteCount: len(raw), stage: "COMMITTED_UNVERIFIED"}
	if !verifiedTargetPublication(receipt, selector, digest, len(raw)) {
		return payloadState, errTargetResult
	}
	got, e := read(root, selector, 1048576)
	if e != nil || !bytes.Equal(got, raw) {
		return payloadState, errTargetResult
	}
	record := targetResultRecord{targetResultRole, targetResultRef{ownerReadRole, owner.selector, owner.digest}, selector, digest, len(raw), "OWNER_ONLY"}
	encoded, e := canonicalTargetResult(record)
	if e != nil {
		return payloadState, errTargetResult
	}
	recordDigest := targetResultDigest(encoded)
	recordSelector := targetResultSelector(recordDigest)
	receipt, e = publication.PublishBoundFile(root, recordSelector, encoded, func(got []byte) error {
		if !bytes.Equal(got, encoded) {
			return errTargetResult
		}
		return nil
	})
	if e != nil || receipt == nil {
		return payloadState, errTargetResult
	}
	recordState := privateBodyPublication{selector: recordSelector, digest: recordDigest, byteCount: len(encoded), stage: "COMMITTED_UNVERIFIED"}
	if !verifiedTargetPublication(receipt, recordSelector, privateDigest(encoded), len(encoded)) || !replayTargetResult(root, recordState, owner, expected, binding, sessionID, generation, invocation, schemaDigest, query, read) {
		return recordState, errTargetResult
	}
	recordState.stage = "VERIFIED"
	return recordState, nil
}
func verifiedTargetPublication(r *publication.BoundFileReceipt, selector, digest string, length int) bool {
	return r.VerificationStatus == "VERIFIED" && r.Mechanism == publication.BoundFileMechanism && r.DirectorySyncStatus == publication.DirectorySyncComplete && r.CloseStatus == publication.CloseComplete && r.FinalSelector == selector && r.Digest == digest && r.ByteLength == uint64(length) && r.NamespaceAtomic
}
