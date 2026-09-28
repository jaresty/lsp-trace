package adr0011acquisition

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"

	"lsp-trace/internal/publication"
	"lsp-trace/sessionruntime"
)

const rawResultPayloadLimit = 1048576

var errRawResultPayload = errors.New("private raw result payload not verified")

// publishRawResultPayload is an inert test-owner checkpoint, not a typed issuance.
// Its source is the independently retained complete response, never pair.Result.
func publishRawResultPayload(root *publication.Root, owner privateBodyPublication, expected sessionruntime.OwnedMethodPair, binding sessionruntime.OwnedDocumentBinding, sessionID string, generation uint64, invocation, schemaDigest string, read func(*publication.Root, string, int64) ([]byte, error)) (privateBodyPublication, error) {
	absent := privateBodyPublication{stage: "ABSENT"}
	if root == nil || owner.stage != "VERIFIED" || expected.Method != "textDocument/references" || schemaDigest != reviewedSuccessorSchemaDigest {
		return absent, errRawResultPayload
	}
	if read == nil {
		read = publication.ReadVerifiedBoundFile
	}
	recordBytes, err := read(root, owner.selector, ownerReadLimit)
	if err != nil || !replayOwnerRead(root, recordBytes, owner, expected, binding, sessionID, generation, invocation, schemaDigest, read) {
		return absent, errRawResultPayload
	}
	var record ownerReadRecord
	if json.Unmarshal(recordBytes, &record) != nil {
		return absent, errRawResultPayload
	}
	body, err := read(root, record.ReadSelector, privateBodyPublicationLimit)
	if err != nil || len(body) != record.ReadByteLength || privateDigest(body) != record.ReadDigest || record.ResultByteLength <= 0 || record.ResultByteLength > rawResultPayloadLimit || record.ResultOffset < 0 || record.ResultOffset > len(body) || record.ResultByteLength > len(body)-record.ResultOffset {
		return absent, errRawResultPayload
	}
	raw := body[record.ResultOffset : record.ResultOffset+record.ResultByteLength]
	if privateDigest(raw) != record.ResultDigest {
		return absent, errRawResultPayload
	}
	slice := privateBodySlice{body: body, value: raw, offset: record.ResultOffset, length: record.ResultByteLength, bodyDigest: record.ReadDigest, valueDigest: record.ResultDigest}
	if !replayPrivateBodySlice(body, slice, "result", expected.Method, record.WireID, raw) {
		return absent, errRawResultPayload
	}
	digest := privateDigest(raw)
	selector := "adr0011-references-raw-payload-v1-" + strings.TrimPrefix(digest, "sha256:") + ".bin"
	receipt, err := publication.PublishBoundFile(root, selector, raw, func(got []byte) error {
		if !bytes.Equal(got, raw) || privateDigest(got) != digest {
			return errRawResultPayload
		}
		return nil
	})
	if err != nil || receipt == nil {
		return absent, errRawResultPayload
	}
	result := privateBodyPublication{selector: selector, digest: digest, byteCount: len(raw), stage: "COMMITTED_UNVERIFIED"}
	if receipt.VerificationStatus != "VERIFIED" || receipt.Mechanism != publication.BoundFileMechanism || receipt.DirectorySyncStatus != publication.DirectorySyncComplete || receipt.CloseStatus != publication.CloseComplete || receipt.FinalSelector != selector || receipt.Digest != digest || receipt.ByteLength != uint64(len(raw)) || !receipt.NamespaceAtomic {
		return result, errRawResultPayload
	}
	got, err := read(root, selector, rawResultPayloadLimit)
	if err != nil || !bytes.Equal(got, raw) || privateDigest(got) != digest {
		return result, errRawResultPayload
	}
	result.stage = "VERIFIED"
	return result, nil
}
