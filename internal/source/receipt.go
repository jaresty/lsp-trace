package source

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
)

type AcquisitionStatus string

const (
	Readable   AcquisitionStatus = "READABLE"
	Unreadable AcquisitionStatus = "UNREADABLE"
)

type DiscoveredItem struct {
	ID      string `json:"id"`
	Locator string `json:"locator"`
}

type Provenance struct {
	Mechanism string `json:"mechanism"`
	Locator   string `json:"locator"`
	Revision  string `json:"revision,omitempty"`
}

type AcquisitionFailure struct {
	Reason string `json:"reason"`
}

type Acquisition struct {
	Status     AcquisitionStatus   `json:"status"`
	Provenance Provenance          `json:"provenance"`
	Failure    *AcquisitionFailure `json:"failure,omitempty"`
}

type ContentIdentity struct {
	Algorithm string `json:"algorithm"`
	Digest    string `json:"digest"`
	Scope     string `json:"scope"`
}

type Receipt struct {
	ReceiptVersion  string              `json:"receipt_version"`
	Item            DiscoveredItem      `json:"item"`
	Status          AcquisitionStatus   `json:"acquisition_status"`
	Provenance      Provenance          `json:"provenance"`
	ContentIdentity *ContentIdentity    `json:"content_identity,omitempty"`
	Failure         *AcquisitionFailure `json:"failure,omitempty"`
}

func CanonicalizeReceipt(item DiscoveredItem, acquisition Acquisition, content []byte) (Receipt, []byte, []byte, error) {
	if item.ID == "" || item.Locator == "" {
		return Receipt{}, nil, nil, errors.New("discovered item id and locator are required")
	}
	if acquisition.Provenance.Mechanism == "" {
		return Receipt{}, nil, nil, errors.New("provenance mechanism is required")
	}
	if acquisition.Provenance.Locator == "" {
		return Receipt{}, nil, nil, errors.New("provenance locator is required")
	}
	r := Receipt{ReceiptVersion: "lsp-trace.source-receipt.v1", Item: item, Status: acquisition.Status, Provenance: acquisition.Provenance, Failure: acquisition.Failure}
	switch acquisition.Status {
	case Readable:
		sum := sha256.Sum256(content)
		r.ContentIdentity = &ContentIdentity{Algorithm: "sha256", Digest: "sha256:" + hex.EncodeToString(sum[:]), Scope: "ACQUIRED_BYTES"}
	case Unreadable:
		if acquisition.Failure == nil || acquisition.Failure.Reason == "" {
			return Receipt{}, nil, nil, errors.New("unreadable acquisition requires failure reason")
		}
		if content != nil {
			return Receipt{}, nil, nil, errors.New("unreadable acquisition must not include content")
		}
	default:
		return Receipt{}, nil, nil, errors.New("unknown acquisition status")
	}
	encoded, err := json.Marshal(r)
	if err != nil {
		return Receipt{}, nil, nil, err
	}
	encoded = append(encoded, '\n')
	return r, append([]byte(nil), content...), encoded, nil
}
