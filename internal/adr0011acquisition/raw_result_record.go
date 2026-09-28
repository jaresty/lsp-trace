package adr0011acquisition

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"lsp-trace/internal/publication"
	"lsp-trace/internal/strictjson"
)

const rawResultRole = "REFERENCES_RAW_RESULT_V1"

var errRawResultRecord = errors.New("private references raw result not verified")

type rawResultRecord struct {
	SchemaVersion     string                `json:"schema_version"`
	ResponseReadRef   targetRecordRefFields `json:"response_read_ref"`
	PayloadSelector   string                `json:"payload_selector"`
	PayloadDigest     string                `json:"payload_digest"`
	PayloadByteLength int                   `json:"payload_byte_length"`
	Access            string                `json:"access"`
}

func canonicalRawResult(r rawResultRecord) ([]byte, error) {
	if r.SchemaVersion != rawResultRole || r.ResponseReadRef.SchemaVersion != responseReadRole || r.Access != "OWNER_ONLY" || r.PayloadByteLength < 1 || r.PayloadByteLength > rawResultPayloadLimit || !validTargetIdentityString(r.ResponseReadRef.Selector) || !validTargetIdentityString(r.ResponseReadRef.Digest) || !validTargetIdentityString(r.PayloadSelector) || !validTargetIdentityString(r.PayloadDigest) || r.ResponseReadRef.Selector != responseReadSelector(r.ResponseReadRef.Digest) || r.PayloadSelector != "adr0011-references-raw-payload-v1-"+strings.TrimPrefix(r.PayloadDigest, "sha256:")+".bin" || !validPrivateDigest(r.PayloadDigest) || !validPrivateDigest(r.ResponseReadRef.Digest) {
		return nil, errRawResultRecord
	}
	field := func(s string) string { return string(targetIdentityStringJSON(s)) }
	b := []byte(fmt.Sprintf(`{"access":%s,"payload_byte_length":%d,"payload_digest":%s,"payload_selector":%s,"response_read_ref":%s,"schema_version":%s}`, field(r.Access), r.PayloadByteLength, field(r.PayloadDigest), field(r.PayloadSelector), targetIdentityRefJSON(targetResultRef{r.ResponseReadRef.SchemaVersion, r.ResponseReadRef.Selector, r.ResponseReadRef.Digest}), field(r.SchemaVersion)))
	if len(b) > sourceRecordLimit {
		return nil, errRawResultRecord
	}
	return b, nil
}

func rawResultDigest(b []byte) string {
	return privateDigest(append([]byte(rawResultRole+"\x00"), b...))
}
func rawResultSelector(d string) string {
	return "adr0011-references-issuance-v1-raw-" + strings.TrimPrefix(d, "sha256:") + ".json"
}

func validPrivateDigest(d string) bool {
	if len(d) != 71 || !strings.HasPrefix(d, "sha256:") {
		return false
	}
	for _, c := range d[7:] {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

func rawResultExpected(root *publication.Root, responseRead, payload privateBodyPublication, x responseReadInputs, read func(*publication.Root, string, int64) ([]byte, error)) (rawResultRecord, bool) {
	var zero rawResultRecord
	if root == nil || responseRead.stage != "VERIFIED" || payload.stage != "VERIFIED" || x.SchemaDigest != reviewedSuccessorSchemaDigest || payload != x.Payload || !replayResponseRead(root, responseRead, x, read) {
		return zero, false
	}
	if read == nil {
		read = publication.ReadVerifiedBoundFile
	}
	responseBytes, e := read(root, responseRead.selector, sourceRecordLimit)
	if e != nil {
		return zero, false
	}
	var response responseReadRecord
	if json.Unmarshal(responseBytes, &response) != nil || response.RawResultDigest != payload.digest || response.RawResultByteLength != payload.byteCount || response.ResultPresence != "PRESENT" {
		return zero, false
	}
	ownerBytes, e := read(root, x.OwnerRead.selector, ownerReadLimit)
	if e != nil {
		return zero, false
	}
	var owner ownerReadRecord
	if json.Unmarshal(ownerBytes, &owner) != nil {
		return zero, false
	}
	original, ok := exactFrameBody(x.Frames.response)
	if !ok || privateDigest(original) != owner.ReadDigest || owner.ResultOffset < 0 || owner.ResultByteLength < 1 || owner.ResultOffset > len(original) || owner.ResultByteLength > len(original)-owner.ResultOffset {
		return zero, false
	}
	token := original[owner.ResultOffset : owner.ResultOffset+owner.ResultByteLength]
	retained, e := read(root, payload.selector, rawResultPayloadLimit)
	if e != nil || !bytes.Equal(token, retained) || len(retained) != payload.byteCount || privateDigest(retained) != payload.digest {
		return zero, false
	}
	return rawResultRecord{rawResultRole, targetRecordRefFields{responseRead.selector, responseRead.digest, responseReadRole}, payload.selector, payload.digest, len(token), "OWNER_ONLY"}, true
}

func replayRawResult(root *publication.Root, state, responseRead, payload privateBodyPublication, x responseReadInputs, read func(*publication.Root, string, int64) ([]byte, error)) bool {
	return replayRawResultInternal(root, state, responseRead, payload, x, read, false)
}
func replayRawResultInternal(root *publication.Root, state, responseRead, payload privateBodyPublication, x responseReadInputs, read func(*publication.Root, string, int64) ([]byte, error), promoting bool) bool {
	if state.stage != "VERIFIED" && !(promoting && state.stage == "COMMITTED_UNVERIFIED") {
		return false
	}
	if read == nil {
		read = publication.ReadVerifiedBoundFile
	}
	b, e := read(root, state.selector, sourceRecordLimit)
	if e != nil || state.byteCount != len(b) || state.digest != rawResultDigest(b) || state.selector != rawResultSelector(state.digest) || strictjson.RejectDuplicates(b) != nil {
		return false
	}
	var record rawResultRecord
	decoder := json.NewDecoder(bytes.NewReader(b))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&record) != nil {
		return false
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return false
	}
	expected, ok := rawResultExpected(root, responseRead, payload, x, read)
	if !ok {
		return false
	}
	canonical, e := canonicalRawResult(expected)
	return e == nil && bytes.Equal(b, canonical)
}

func publishRawResult(root *publication.Root, responseRead, payload privateBodyPublication, x responseReadInputs, read func(*publication.Root, string, int64) ([]byte, error)) (privateBodyPublication, error) {
	absent := privateBodyPublication{stage: "ABSENT"}
	if read == nil {
		read = publication.ReadVerifiedBoundFile
	}
	record, ok := rawResultExpected(root, responseRead, payload, x, read)
	if !ok {
		return absent, errRawResultRecord
	}
	b, e := canonicalRawResult(record)
	if e != nil {
		return absent, errRawResultRecord
	}
	digest := rawResultDigest(b)
	selector := rawResultSelector(digest)
	precommit, uncertain := false, false
	receipt, e := publication.PublishBoundFileWithTrace(root, selector, b, func(got []byte) error {
		if !bytes.Equal(got, b) {
			return errRawResultRecord
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
			return absent, errRawResultRecord
		}
		return privateBodyPublication{selector: selector, digest: digest, byteCount: len(b), stage: "COMMITTED_UNVERIFIED"}, errRawResultRecord
	}
	state := privateBodyPublication{selector: selector, digest: digest, byteCount: len(b), stage: "COMMITTED_UNVERIFIED"}
	if !verifiedTargetPublication(receipt, selector, privateDigest(b), len(b)) || !replayRawResultInternal(root, state, responseRead, payload, x, read, true) {
		return state, errRawResultRecord
	}
	state.stage = "VERIFIED"
	return state, nil
}
