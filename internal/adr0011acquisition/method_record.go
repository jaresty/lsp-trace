package adr0011acquisition

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"

	"lsp-trace/internal/adr0011requestkey"
	"lsp-trace/internal/publication"
	"lsp-trace/internal/strictjson"
)

// A synthetic query identity is not independently admitted by this private record.
const methodRecordVersion = "lsp-trace.adr0011.references-symbol.method.v1"

var errMethodRecord = errors.New("private references method record not verified")

type methodRecord struct {
	Version         string                `json:"Version"`
	Method          string                `json:"Method"`
	SessionID       string                `json:"SessionID"`
	Generation      uint64                `json:"Generation"`
	RequestKey      string                `json:"RequestKey"`
	InvocationID    string                `json:"InvocationID"`
	ParamsDigest    string                `json:"ParamsDigest"`
	TargetRef       targetRecordRefFields `json:"TargetRef"`
	SourceRef       targetRecordRefFields `json:"SourceRef"`
	RevisionRef     targetRecordRefFields `json:"RevisionRef"`
	ResponseReadRef targetRecordRefFields `json:"ResponseReadRef"`
	RawResultRef    targetRecordRefFields `json:"RawResultRef"`
}

type methodRecordInputs struct {
	ResponseInputs          responseReadInputs
	ResponseRead, RawResult privateBodyPublication
}

func methodRecordSelector(d string) string {
	return "adr0011-method-" + strings.TrimPrefix(d, "sha256:") + ".json"
}
func methodRecordBytes(r methodRecord) ([]byte, error) {
	b, e := json.Marshal(r)
	if e != nil || len(b)+1 > sourceRecordLimit {
		return nil, errMethodRecord
	}
	return append(b, '\n'), nil
}
func methodRecordExpected(root *publication.Root, x methodRecordInputs, read func(*publication.Root, string, int64) ([]byte, error)) (methodRecord, bool) {
	var zero methodRecord
	if root == nil || read == nil || x.ResponseRead.stage != "VERIFIED" || x.RawResult.stage != "VERIFIED" || !replayResponseRead(root, x.ResponseRead, x.ResponseInputs, read) || !replayRawResult(root, x.RawResult, x.ResponseRead, x.ResponseInputs.Payload, x.ResponseInputs, read) {
		return zero, false
	}
	ri := x.ResponseInputs
	if ri.TargetInputs.Pair.SessionID != ri.Pair.SessionID || ri.TargetInputs.Pair.Generation != ri.Pair.Generation || ri.TargetInputs.Source.stage != "VERIFIED" || ri.TargetInputs.Revision.stage != "VERIFIED" || ri.TargetInputs.Query != ri.Query || ri.Frames.invocation == "" || !verifyOriginalRequestKey(ri.Frames, ri.Pair, ri.Pair.SessionID, ri.Pair.Generation, ri.Frames.invocation, ri.SchemaDigest) {
		return zero, false
	}
	target, ok := targetRecordExpected(root, ri.TargetInputs, read)
	if !ok || !replayTargetRecord(root, ri.Target, ri.TargetInputs, read) || target.SourceRef != targetRecordRefFieldsFrom(targetRecordRef(sourceIdentityRole, ri.TargetInputs.Source)) || target.RevisionRef != targetRecordRefFieldsFrom(targetRecordRef(revisionIdentityRole, ri.TargetInputs.Revision)) {
		return zero, false
	}
	response, ok := responseReadExpected(root, ri, read)
	if !ok || response.SessionID != ri.Pair.SessionID || response.Generation != ri.Pair.Generation || response.RequestKey != adr0011requestkey.Encode(ri.Pair.Key) || response.InvocationID != ri.Frames.invocation || response.TargetRef != (targetRecordRefFields{ri.Target.selector, ri.Target.digest, targetRecordVersion}) {
		return zero, false
	}
	raw, ok := rawResultExpected(root, x.ResponseRead, ri.Payload, ri, read)
	if !ok || raw.ResponseReadRef != (targetRecordRefFields{x.ResponseRead.selector, x.ResponseRead.digest, responseReadRole}) {
		return zero, false
	}
	ownerBytes, e := read(root, ri.OwnerRead.selector, ownerReadLimit)
	if e != nil {
		return zero, false
	}
	var owner ownerReadRecord
	if json.Unmarshal(ownerBytes, &owner) != nil {
		return zero, false
	}
	write, e := read(root, owner.WriteSelector, privateBodyPublicationLimit)
	if e != nil || owner.ParamsOffset < 0 || owner.ParamsByteLength < 1 || owner.ParamsOffset > len(write) || owner.ParamsByteLength > len(write)-owner.ParamsOffset {
		return zero, false
	}
	original, valid := exactFrameBody(ri.Frames.request)
	originalRead, readValid := exactFrameBody(ri.Frames.response)
	retainedRead, e := read(root, owner.ReadSelector, privateBodyPublicationLimit)
	if !valid || !readValid || e != nil || !bytes.Equal(original, write) || !bytes.Equal(originalRead, retainedRead) || privateDigest(write[owner.ParamsOffset:owner.ParamsOffset+owner.ParamsByteLength]) != owner.ParamsDigest || response.WriteDigest != owner.WriteDigest || response.ReadDigest != owner.ReadDigest {
		return zero, false
	}
	return methodRecord{methodRecordVersion, "textDocument/references", ri.Pair.SessionID, ri.Pair.Generation, response.RequestKey, response.InvocationID, owner.ParamsDigest, targetRecordRefFields{ri.Target.selector, ri.Target.digest, targetRecordVersion}, target.SourceRef, target.RevisionRef, targetRecordRefFields{x.ResponseRead.selector, x.ResponseRead.digest, responseReadRole}, targetRecordRefFields{x.RawResult.selector, x.RawResult.digest, rawResultRole}}, true
}
func replayMethodRecord(root *publication.Root, state privateBodyPublication, x methodRecordInputs, read func(*publication.Root, string, int64) ([]byte, error)) bool {
	return replayMethodRecordInternal(root, state, x, read, false)
}
func replayMethodRecordInternal(root *publication.Root, state privateBodyPublication, x methodRecordInputs, read func(*publication.Root, string, int64) ([]byte, error), promoting bool) bool {
	if state.stage != "VERIFIED" && !(promoting && state.stage == "COMMITTED_UNVERIFIED") {
		return false
	}
	if read == nil {
		read = publication.ReadVerifiedBoundFile
	}
	if !validPrivateDigest(state.digest) || state.selector != methodRecordSelector(state.digest) {
		return false
	}
	b, e := read(root, state.selector, sourceRecordLimit)
	if e != nil || state.byteCount != len(b) || privateDigest(b) != state.digest || strictjson.RejectDuplicates(b) != nil {
		return false
	}
	var actual methodRecord
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if dec.Decode(&actual) != nil {
		return false
	}
	var extra any
	if dec.Decode(&extra) != io.EOF {
		return false
	}
	expected, ok := methodRecordExpected(root, x, read)
	if !ok {
		return false
	}
	canonical, e := methodRecordBytes(expected)
	return e == nil && bytes.Equal(b, canonical)
}
func publishMethodRecord(root *publication.Root, x methodRecordInputs, read func(*publication.Root, string, int64) ([]byte, error)) (privateBodyPublication, error) {
	absent := privateBodyPublication{stage: "ABSENT"}
	if read == nil {
		read = publication.ReadVerifiedBoundFile
	}
	record, ok := methodRecordExpected(root, x, read)
	if !ok {
		return absent, errMethodRecord
	}
	b, e := methodRecordBytes(record)
	if e != nil {
		return absent, errMethodRecord
	}
	digest := privateDigest(b)
	selector := methodRecordSelector(digest)
	precommit, uncertain := false, false
	receipt, e := publication.PublishBoundFileWithTrace(root, selector, b, func(got []byte) error {
		if !bytes.Equal(got, b) {
			return errMethodRecord
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
			return absent, errMethodRecord
		}
		return privateBodyPublication{selector, digest, len(b), "COMMITTED_UNVERIFIED"}, errMethodRecord
	}
	state := privateBodyPublication{selector, digest, len(b), "COMMITTED_UNVERIFIED"}
	if !verifiedTargetPublication(receipt, selector, digest, len(b)) || !replayMethodRecordInternal(root, state, x, read, true) {
		return state, errMethodRecord
	}
	state.stage = "VERIFIED"
	return state, nil
}
