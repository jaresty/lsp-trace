package adr0011acquisition

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"lsp-trace/internal/adr0011querytarget"
	"lsp-trace/internal/adr0011requestkey"
	"lsp-trace/internal/publication"
	"lsp-trace/internal/strictjson"
	"lsp-trace/sessionruntime"
)

const responseReadRole = "REFERENCES_RESPONSE_READ_V1"

var errResponseRead = errors.New("private references response read not verified")

type responseReadRecord struct {
	SchemaVersion       string                `json:"schema_version"`
	TransactionID       string                `json:"transaction_id"`
	RequestKey          string                `json:"request_key"`
	InvocationID        string                `json:"invocation_id"`
	SessionID           string                `json:"session_id"`
	Generation          uint64                `json:"generation"`
	Method              string                `json:"method"`
	TargetRef           targetRecordRefFields `json:"target_ref"`
	WriteDigest         string                `json:"write_digest"`
	ReadDigest          string                `json:"read_digest"`
	OwnerReadRef        targetRecordRefFields `json:"owner_read_ref"`
	ResultPresence      string                `json:"result_presence"`
	RawResultDigest     string                `json:"raw_result_digest"`
	RawResultByteLength int                   `json:"raw_result_byte_length"`
}

// The explicit sorted keys and the nested ref encoder implement the closed JCS profile.
func canonicalResponseRead(r responseReadRecord) ([]byte, error) {
	if r.Generation == 0 || r.Generation > maxCanonicalInteger || r.RawResultByteLength < 1 || r.RawResultByteLength > rawResultPayloadLimit {
		return nil, errResponseRead
	}
	stringsToCheck := []string{r.SchemaVersion, r.TransactionID, r.RequestKey, r.InvocationID, r.SessionID, r.Method, r.WriteDigest, r.ReadDigest, r.ResultPresence, r.RawResultDigest, r.TargetRef.Selector, r.TargetRef.Digest, r.TargetRef.SchemaVersion, r.OwnerReadRef.Selector, r.OwnerReadRef.Digest, r.OwnerReadRef.SchemaVersion}
	for _, s := range stringsToCheck {
		if !validTargetIdentityString(s) {
			return nil, errResponseRead
		}
	}
	field := func(s string) string { return string(targetIdentityStringJSON(s)) }
	raw := fmt.Sprintf(`{"generation":%d,"invocation_id":%s,"method":%s,"owner_read_ref":%s,"raw_result_byte_length":%d,"raw_result_digest":%s,"read_digest":%s,"request_key":%s,"result_presence":%s,"schema_version":%s,"session_id":%s,"target_ref":%s,"transaction_id":%s,"write_digest":%s}`, r.Generation, field(r.InvocationID), field(r.Method), targetIdentityRefJSON(targetResultRef{r.OwnerReadRef.SchemaVersion, r.OwnerReadRef.Selector, r.OwnerReadRef.Digest}), r.RawResultByteLength, field(r.RawResultDigest), field(r.ReadDigest), field(r.RequestKey), field(r.ResultPresence), field(r.SchemaVersion), field(r.SessionID), targetIdentityRefJSON(targetResultRef{r.TargetRef.SchemaVersion, r.TargetRef.Selector, r.TargetRef.Digest}), field(r.TransactionID), field(r.WriteDigest))
	if len(raw) > sourceRecordLimit {
		return nil, errResponseRead
	}
	return []byte(raw), nil
}
func responseReadDigest(raw []byte) string {
	return privateDigest(append([]byte(responseReadRole+"\x00"), raw...))
}
func responseReadSelector(d string) string {
	return "adr0011-references-issuance-v1-response-read-" + strings.TrimPrefix(d, "sha256:") + ".json"
}

type responseReadInputs struct {
	TargetInputs       targetRecordInputs
	Target             privateBodyPublication
	Frames             ownedFrames
	Pair               sessionruntime.OwnedMethodPair
	Binding            sessionruntime.OwnedDocumentBinding
	Query              adr0011querytarget.Query
	OwnerRead, Payload privateBodyPublication
	SchemaDigest       string
}

func responseReadExpected(root *publication.Root, x responseReadInputs, read func(*publication.Root, string, int64) ([]byte, error)) (responseReadRecord, bool) {
	var zero responseReadRecord
	if root == nil || read == nil || x.SchemaDigest != reviewedSuccessorSchemaDigest || x.TargetInputs.SchemaDigest != reviewedSuccessorSchemaDigest || x.Target.stage != "VERIFIED" || x.OwnerRead.stage != "VERIFIED" || x.Payload.stage != "VERIFIED" || x.Pair.Method != "textDocument/references" || x.Pair.Source == nil || *x.Pair.Source != x.Binding || !verifyOriginalRequestKey(x.Frames, x.Pair, x.Pair.SessionID, x.Pair.Generation, x.Frames.invocation, x.SchemaDigest) || !replayTargetRecord(root, x.Target, x.TargetInputs, read) || x.TargetInputs.Source.stage != "VERIFIED" || x.TargetInputs.Revision.stage != "VERIFIED" {
		return zero, false
	}
	target, ok := targetRecordExpected(root, x.TargetInputs, read)
	if !ok || x.Query != x.TargetInputs.Query || target.QueryURI != x.Binding.URI || x.Query.SessionID != x.Pair.SessionID || x.Query.Generation != x.Pair.Generation || x.Query.SourceDigest != x.Binding.SHA256 || x.Query.DocumentVersion != fmt.Sprint(x.Binding.Version) {
		return zero, false
	}
	ownerBytes, e := read(root, x.OwnerRead.selector, ownerReadLimit)
	if e != nil || !replayOwnerRead(root, ownerBytes, x.OwnerRead, x.Pair, x.Binding, x.Pair.SessionID, x.Pair.Generation, x.Frames.invocation, x.SchemaDigest, read) {
		return zero, false
	}
	var owner ownerReadRecord
	if json.Unmarshal(ownerBytes, &owner) != nil {
		return zero, false
	}
	write, e := read(root, owner.WriteSelector, privateBodyPublicationLimit)
	if e != nil {
		return zero, false
	}
	response, e := read(root, owner.ReadSelector, privateBodyPublicationLimit)
	if e != nil {
		return zero, false
	}
	originalWrite, wok := exactFrameBody(x.Frames.request)
	originalRead, rok := exactFrameBody(x.Frames.response)
	if !wok || !rok || !bytes.Equal(originalWrite, write) || !bytes.Equal(originalRead, response) || privateDigest(write) != owner.WriteDigest || privateDigest(response) != owner.ReadDigest {
		return zero, false
	}
	if owner.ParamsOffset < 0 || owner.ParamsByteLength < 1 || owner.ParamsOffset > len(write) || owner.ParamsByteLength > len(write)-owner.ParamsOffset || !responseReadPoint(write[owner.ParamsOffset:owner.ParamsOffset+owner.ParamsByteLength], x.Query) {
		return zero, false
	}
	if owner.ResultOffset < 0 || owner.ResultByteLength < 1 || owner.ResultOffset > len(response) || owner.ResultByteLength > len(response)-owner.ResultOffset {
		return zero, false
	}
	raw := response[owner.ResultOffset : owner.ResultOffset+owner.ResultByteLength]
	if len(raw) > rawResultPayloadLimit || privateDigest(raw) != owner.ResultDigest || x.Payload.digest != owner.ResultDigest || x.Payload.byteCount != len(raw) || x.Payload.selector != "adr0011-references-raw-payload-v1-"+strings.TrimPrefix(x.Payload.digest, "sha256:")+".bin" {
		return zero, false
	}
	payload, e := read(root, x.Payload.selector, rawResultPayloadLimit)
	if e != nil || !bytes.Equal(payload, raw) {
		return zero, false
	}
	sourceRef := targetRecordRef(sourceIdentityRole, x.TargetInputs.Source)
	revisionRef := targetRecordRef(revisionIdentityRole, x.TargetInputs.Revision)
	tx, e := referencesTransactionID(x.Pair.SessionID, x.Pair.Generation, x.Pair.Key, x.Frames.invocation, x.Query.OccurrenceID, sourceRef, revisionRef)
	if e != nil {
		return zero, false
	}
	return responseReadRecord{responseReadRole, tx, adr0011requestkey.Encode(x.Pair.Key), x.Frames.invocation, x.Pair.SessionID, x.Pair.Generation, x.Pair.Method, targetRecordRefFields{x.Target.selector, x.Target.digest, targetRecordVersion}, owner.WriteDigest, owner.ReadDigest, targetRecordRefFields{x.OwnerRead.selector, x.OwnerRead.digest, ownerReadRole}, "PRESENT", privateDigest(raw), len(raw)}, true
}
func responseReadPoint(raw []byte, q adr0011querytarget.Query) bool {
	if strictjson.RejectDuplicates(raw) != nil || !ownerReadParams(raw, "textDocument/references", q.URI) {
		return false
	}
	var v struct {
		Position struct {
			Line      uint32 `json:"line"`
			Character uint32 `json:"character"`
		} `json:"position"`
	}
	return json.Unmarshal(raw, &v) == nil && v.Position.Line == q.Line && v.Position.Character == q.Character
}
func replayResponseRead(root *publication.Root, state privateBodyPublication, x responseReadInputs, read func(*publication.Root, string, int64) ([]byte, error)) bool {
	return replayResponseReadInternal(root, state, x, read, false)
}
func replayResponseReadInternal(root *publication.Root, state privateBodyPublication, x responseReadInputs, read func(*publication.Root, string, int64) ([]byte, error), promoting bool) bool {
	if state.stage != "VERIFIED" && !(promoting && state.stage == "COMMITTED_UNVERIFIED") {
		return false
	}
	if read == nil {
		read = publication.ReadVerifiedBoundFile
	}
	raw, e := read(root, state.selector, sourceRecordLimit)
	if e != nil || state.byteCount != len(raw) || state.digest != responseReadDigest(raw) || state.selector != responseReadSelector(state.digest) || strictjson.RejectDuplicates(raw) != nil {
		return false
	}
	var record responseReadRecord
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&record) != nil {
		return false
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return false
	}
	expected, ok := responseReadExpected(root, x, read)
	if !ok {
		return false
	}
	canonical, e := canonicalResponseRead(expected)
	return e == nil && bytes.Equal(raw, canonical)
}
func publishResponseRead(root *publication.Root, x responseReadInputs, read func(*publication.Root, string, int64) ([]byte, error)) (privateBodyPublication, error) {
	absent := privateBodyPublication{stage: "ABSENT"}
	if read == nil {
		read = publication.ReadVerifiedBoundFile
	}
	record, ok := responseReadExpected(root, x, read)
	if !ok {
		return absent, errResponseRead
	}
	raw, e := canonicalResponseRead(record)
	if e != nil {
		return absent, errResponseRead
	}
	digest := responseReadDigest(raw)
	selector := responseReadSelector(digest)
	precommit, uncertain := false, false
	receipt, e := publication.PublishBoundFileWithTrace(root, selector, raw, func(got []byte) error {
		if !bytes.Equal(got, raw) {
			return errResponseRead
		}
		return nil
	}, func(event publication.BoundFileTraceEvent) {
		if !event.OK {
			switch event.Stage {
			case "OPEN_VALIDATE", "CANDIDATE", "TEMP", "WRITE_FSYNC":
				precommit = true
			case "TARGET", "HARDLINK":
				if event.Result == "INJECTED_BEFORE_INSTALL" {
					precommit = true
				} else {
					uncertain = true
				}
			}
		}
		if event.Stage == "HARDLINK" && event.Result == "INSTALLED" {
			uncertain = true
		}
	})
	if e != nil || receipt == nil {
		if precommit && !uncertain {
			return absent, errResponseRead
		}
		return privateBodyPublication{selector: selector, digest: digest, byteCount: len(raw), stage: "COMMITTED_UNVERIFIED"}, errResponseRead
	}
	state := privateBodyPublication{selector: selector, digest: digest, byteCount: len(raw), stage: "COMMITTED_UNVERIFIED"}
	if !verifiedTargetPublication(receipt, selector, privateDigest(raw), len(raw)) || !replayResponseReadInternal(root, state, x, read, true) {
		return state, errResponseRead
	}
	state.stage = "VERIFIED"
	return state, nil
}
