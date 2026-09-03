// Package verification defines pure selector and exact-byte custody receipt
// semantics. Filesystem loading, pinning, and publication remain adapter-owned.
package verification

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"

	"lsp-trace/internal/graph"
)

const (
	DirectoryDurabilityChecked     = "CHECKED"
	DirectoryDurabilityUnavailable = "UNAVAILABLE_ON_PLATFORM"

	receiptVersion  = "lsp-trace.byte-custody-receipt.v1"
	digestAlgorithm = "SHA-256"
	integrityClaim  = "INTEGRITY_AND_CUSTODY_ONLY"
)

type Selector struct {
	Generation string `json:"generation"`
}

type receipt struct {
	ReceiptVersion             string `json:"receipt_version"`
	ExactSerializedBytesDigest string `json:"exact_serialized_bytes_digest"`
	DigestAlgorithm            string `json:"digest_algorithm"`
	DigestScope                string `json:"digest_scope"`
	IntegrityClaim             string `json:"integrity_claim"`
	AuthenticityClaim          bool   `json:"authenticity_claim"`
	DirectoryDurability        string `json:"directory_durability"`
}

func DecodeSelector(data []byte) (Selector, error) {
	var selector Selector
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&selector); err != nil {
		return selector, fmt.Errorf("malformed generation selector: %w", err)
	}
	if selector.Generation == "" || filepath.Base(selector.Generation) != selector.Generation {
		return selector, fmt.Errorf("malformed generation selector: invalid generation")
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return selector, fmt.Errorf("malformed generation selector: trailing JSON content")
	}
	return selector, nil
}

func ReceiptBytes(data []byte, directoryDurability string) ([]byte, error) {
	r := receipt{
		ReceiptVersion:             receiptVersion,
		ExactSerializedBytesDigest: graph.ExactBytesDigest(data),
		DigestAlgorithm:            digestAlgorithm,
		DigestScope:                graph.ByteDigestScope,
		IntegrityClaim:             integrityClaim,
		AuthenticityClaim:          false,
		DirectoryDurability:        directoryDurability,
	}
	b, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

func VerifyReceipt(artifact, receiptData []byte) error {
	var r receipt
	decoder := json.NewDecoder(bytes.NewReader(receiptData))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&r); err != nil {
		return fmt.Errorf("malformed: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("malformed: trailing JSON content")
	}
	if r.ReceiptVersion != receiptVersion || r.DigestScope != graph.ByteDigestScope || r.DigestAlgorithm != digestAlgorithm || r.IntegrityClaim != integrityClaim || r.AuthenticityClaim || (r.DirectoryDurability != DirectoryDurabilityChecked && r.DirectoryDurability != DirectoryDurabilityUnavailable) {
		return fmt.Errorf("receipt metadata mismatch")
	}
	if r.ExactSerializedBytesDigest != graph.ExactBytesDigest(artifact) {
		return fmt.Errorf("exact-byte integrity mismatch")
	}
	return nil
}
