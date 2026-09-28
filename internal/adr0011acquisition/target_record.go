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

// targetRecord is a private synthetic successor. No live query admission is inferred.
const targetRecordVersion = "lsp-trace.adr0011.references-symbol.target.v1"

var errTargetRecord = errors.New("private target record not verified")

// The legacy private targetResultRef uses JCS key order. Successor targetRecord
// uses encoding/json.Marshal and the schema's required order for nested refs.
type targetRecordRefFields struct {
	Selector      string `json:"selector"`
	Digest        string `json:"digest"`
	SchemaVersion string `json:"schema_version"`
}

type targetRecordPosition struct {
	Line      uint32 `json:"line"`
	Character uint32 `json:"character"`
}
type targetRecordRange struct {
	Start targetRecordPosition `json:"start"`
	End   targetRecordPosition `json:"end"`
}

func targetRecordRefFieldsFrom(ref targetResultRef) targetRecordRefFields {
	return targetRecordRefFields{ref.Selector, ref.Digest, ref.SchemaVersion}
}
func targetRecordRangeFrom(r adr0011querytarget.Range) targetRecordRange {
	return targetRecordRange{targetRecordPosition{r.Start.Line, r.Start.Character}, targetRecordPosition{r.End.Line, r.End.Character}}
}

type targetRecord struct {
	Version           string                `json:"Version"`
	Method            string                `json:"Method"`
	SessionID         string                `json:"SessionID"`
	Generation        uint64                `json:"Generation"`
	RequestKey        string                `json:"RequestKey"`
	ParamsDigest      string                `json:"ParamsDigest"`
	SourceRef         targetRecordRefFields `json:"SourceRef"`
	RevisionRef       targetRecordRefFields `json:"RevisionRef"`
	OwnerReadRef      targetRecordRefFields `json:"OwnerReadRef"`
	TargetResultRef   targetRecordRefFields `json:"TargetResultRef"`
	QueryOccurrenceID string                `json:"QueryOccurrenceID"`
	QueryURI          string                `json:"QueryURI"`
	QueryLine         uint32                `json:"QueryLine"`
	QueryCharacter    uint32                `json:"QueryCharacter"`
	Encoding          string                `json:"Encoding"`
	SymbolName        string                `json:"SymbolName"`
	SymbolKind        int                   `json:"SymbolKind"`
	DisplayRange      targetRecordRange     `json:"DisplayRange"`
	SelectionRange    targetRecordRange     `json:"SelectionRange"`
	TargetID          string                `json:"TargetID"`
	SymbolID          string                `json:"SymbolID"`
	ResultDigest      string                `json:"ResultDigest"`
}

func targetRecordSelector(d string) string {
	return "adr0011-target-" + strings.TrimPrefix(d, "sha256:") + ".json"
}
func targetRecordBytes(r targetRecord) ([]byte, error) {
	b, e := json.Marshal(r)
	if e != nil || len(b)+1 > sourceRecordLimit {
		return nil, errTargetRecord
	}
	return append(b, '\n'), nil
}
func targetRecordRef(role string, p privateBodyPublication) targetResultRef {
	return targetResultRef{role, p.selector, p.digest}
}

// targetRecordInputs are test-owner expectations, not claimant record fields.
// The owner must supply original prepared bytes and original framed request/read.
type targetRecordInputs struct {
	Frames                                                       ownedFrames
	Pair                                                         sessionruntime.OwnedMethodPair
	Binding                                                      sessionruntime.OwnedDocumentBinding
	Query                                                        adr0011querytarget.Query
	Original                                                     []byte
	Workspace                                                    string
	Git                                                          hostGitExpectation
	Prepared, Source, Revision, Before, After, OwnerRead, Result privateBodyPublication
	SchemaDigest                                                 string
}

func targetRecordExpected(root *publication.Root, x targetRecordInputs, read func(*publication.Root, string, int64) ([]byte, error)) (targetRecord, bool) {
	var r targetRecord
	if root == nil || read == nil || x.SchemaDigest != reviewedSuccessorSchemaDigest || x.Workspace != x.Git.Root || x.Pair.Method != "textDocument/documentSymbol" || !verifyOriginalRequestKey(x.Frames, x.Pair, x.Pair.SessionID, x.Pair.Generation, x.Frames.invocation, x.SchemaDigest) ||
		x.Prepared.stage != "VERIFIED" || x.Source.stage != "VERIFIED" || x.Revision.stage != "VERIFIED" || x.OwnerRead.stage != "VERIFIED" || x.Result.stage != "VERIFIED" ||
		x.Query.SessionID != x.Pair.SessionID || x.Query.Generation != x.Pair.Generation || x.Query.URI != x.Binding.URI || x.Query.SourceDigest != x.Binding.SHA256 || x.Query.DocumentVersion != fmt.Sprint(x.Binding.Version) || x.Query.Encoding != "utf-16" ||
		!replayPreparedSource(root, x.Prepared, x.Source, x.Original, x.Binding.URI, x.Binding.Version, x.Workspace, x.SchemaDigest, read) ||
		!replayRevisionIdentity(root, x.Revision, x.Before, x.After, x.Git, x.SchemaDigest, read) ||
		!replayTargetResult(root, x.Result, x.OwnerRead, x.Pair, x.Binding, x.Pair.SessionID, x.Pair.Generation, x.Frames.invocation, x.SchemaDigest, x.Query, read) {
		return r, false
	}
	ownerBytes, e := read(root, x.OwnerRead.selector, ownerReadLimit)
	if e != nil || !replayOwnerRead(root, ownerBytes, x.OwnerRead, x.Pair, x.Binding, x.Pair.SessionID, x.Pair.Generation, x.Frames.invocation, x.SchemaDigest, read) {
		return r, false
	}
	var observation ownerReadRecord
	if json.Unmarshal(ownerBytes, &observation) != nil {
		return r, false
	}
	write, e := read(root, observation.WriteSelector, privateBodyPublicationLimit)
	if e != nil || observation.ParamsOffset < 0 || observation.ParamsOffset > len(write) || observation.ParamsByteLength < 1 || observation.ParamsByteLength > len(write)-observation.ParamsOffset {
		return r, false
	}
	response, e := read(root, observation.ReadSelector, privateBodyPublicationLimit)
	if e != nil {
		return r, false
	}
	originalWrite, writeOK := exactFrameBody(x.Frames.request)
	originalRead, readOK := exactFrameBody(x.Frames.response)
	if !writeOK || !readOK || !bytes.Equal(originalWrite, write) || !bytes.Equal(originalRead, response) {
		return r, false
	}
	params := write[observation.ParamsOffset : observation.ParamsOffset+observation.ParamsByteLength]
	if privateDigest(params) != observation.ParamsDigest {
		return r, false
	}
	resultBytes, e := read(root, x.Result.selector, sourceRecordLimit)
	if e != nil {
		return r, false
	}
	var result targetResultRecord
	if json.Unmarshal(resultBytes, &result) != nil || result.OwnerReadRef != targetRecordRef(ownerReadRole, x.OwnerRead) {
		return r, false
	}
	raw, e := read(root, result.PayloadSelector, 1048576)
	if e != nil || privateDigest(raw) != result.PayloadDigest || len(raw) != result.PayloadByteLength || observation.ResultDigest != result.PayloadDigest {
		return r, false
	}
	candidate, e := adr0011querytarget.SelectDocumentSymbolCandidateV1(x.Query, raw)
	if e != nil {
		return r, false
	}
	sourceRef := targetRecordRef(sourceIdentityRole, x.Source)
	revisionRef := targetRecordRef(revisionIdentityRole, x.Revision)
	resultRef := targetRecordRef(targetResultRole, x.Result)
	symbolID, targetID, e := targetIdentityV1(sourceRef, revisionRef, resultRef, x.Query, raw)
	if e != nil {
		return r, false
	}
	r = targetRecord{targetRecordVersion, "textDocument/documentSymbol", x.Pair.SessionID, x.Pair.Generation, adr0011requestkey.Encode(x.Pair.Key), observation.ParamsDigest, targetRecordRefFieldsFrom(sourceRef), targetRecordRefFieldsFrom(revisionRef), targetRecordRefFieldsFrom(targetRecordRef(ownerReadRole, x.OwnerRead)), targetRecordRefFieldsFrom(resultRef), x.Query.OccurrenceID, x.Query.URI, x.Query.Line, x.Query.Character, x.Query.Encoding, candidate.SymbolName, candidate.SymbolKind, targetRecordRangeFrom(candidate.DisplayRange), targetRecordRangeFrom(candidate.SelectionRange), targetID, symbolID, privateDigest(raw)}
	return r, true
}

func replayTargetRecord(root *publication.Root, state privateBodyPublication, x targetRecordInputs, read func(*publication.Root, string, int64) ([]byte, error)) bool {
	return replayTargetRecordInternal(root, state, x, read, false)
}

func replayTargetRecordInternal(root *publication.Root, state privateBodyPublication, x targetRecordInputs, read func(*publication.Root, string, int64) ([]byte, error), promoting bool) bool {
	if (state.stage != "VERIFIED" && !(promoting && state.stage == "COMMITTED_UNVERIFIED")) || state.selector == "" || !targetResultValidDigest(state.digest) || state.selector != targetRecordSelector(state.digest) {
		return false
	}
	if read == nil {
		read = publication.ReadVerifiedBoundFile
	}
	raw, e := read(root, state.selector, sourceRecordLimit)
	if e != nil || state.byteCount != len(raw) || privateDigest(raw) != state.digest || strictjson.RejectDuplicates(raw) != nil {
		return false
	}
	var actual targetRecord
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&actual) != nil {
		return false
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return false
	}
	expected, ok := targetRecordExpected(root, x, read)
	if !ok {
		return false
	}
	canonical, e := targetRecordBytes(expected)
	return e == nil && bytes.Equal(raw, canonical)
}

func publishTargetRecord(root *publication.Root, x targetRecordInputs, read func(*publication.Root, string, int64) ([]byte, error)) (privateBodyPublication, error) {
	absent := privateBodyPublication{stage: "ABSENT"}
	if read == nil {
		read = publication.ReadVerifiedBoundFile
	}
	record, ok := targetRecordExpected(root, x, read)
	if !ok {
		return absent, errTargetRecord
	}
	encoded, e := targetRecordBytes(record)
	if e != nil {
		return absent, errTargetRecord
	}
	digest := privateDigest(encoded)
	selector := targetRecordSelector(digest)
	precommit, uncertain := false, false
	receipt, e := publication.PublishBoundFileWithTrace(root, selector, encoded, func(got []byte) error {
		if !bytes.Equal(got, encoded) {
			return errTargetRecord
		}
		return nil
	}, func(event publication.BoundFileTraceEvent) {
		// A known failure before install leaves this attempt ABSENT. A target
		// collision or failed install has unknown prior selector custody.
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
			return absent, errTargetRecord
		}
		return privateBodyPublication{selector: selector, digest: digest, byteCount: len(encoded), stage: "COMMITTED_UNVERIFIED"}, errTargetRecord
	}
	state := privateBodyPublication{selector: selector, digest: digest, byteCount: len(encoded), stage: "COMMITTED_UNVERIFIED"}
	if !verifiedTargetPublication(receipt, selector, digest, len(encoded)) || !replayTargetRecordInternal(root, state, x, read, true) {
		return state, errTargetRecord
	}
	state.stage = "VERIFIED"
	return state, nil
}
