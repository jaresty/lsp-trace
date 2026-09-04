package source

import (
	"errors"
	"fmt"
	"sort"
)

const FailureStageSource = "SOURCE"

// AcquiredSource is the result of acquiring bytes for one discovery record.
type AcquiredSource struct {
	Bytes []byte
	Err   error
}

// ReceiptArtifact preserves the discovery record beside its canonical custody bytes.
type ReceiptArtifact struct {
	Record           Record
	Receipt          Receipt
	Content          []byte
	CanonicalReceipt []byte
	FailureStage     string
}

// BridgeDiscoveryReceipts adapts discovery records and acquisition results into receipts.
// It records acquisition outcomes but deliberately makes no manifest decision.
func BridgeDiscoveryReceipts(records []Record, acquired map[string]AcquiredSource) ([]ReceiptArtifact, error) {
	ordered := append([]Record(nil), records...)
	sort.Slice(ordered, func(i, j int) bool { return recordSortKey(ordered[i]) < recordSortKey(ordered[j]) })

	artifacts := make([]ReceiptArtifact, 0, len(ordered))
	for _, record := range ordered {
		if record.Name == "" {
			return nil, errors.New("discovery record name is required")
		}
		item := DiscoveredItem{ID: record.Name, Locator: record.Name}
		acquisition := Acquisition{
			Status:     Unreadable,
			Provenance: Provenance{Mechanism: "source-discovery", Locator: record.Name},
		}
		var content []byte
		failureStage := FailureStageSource

		if record.Inclusion == Included && record.Kind == SourceRegularFile {
			result, ok := acquired[record.Name]
			switch {
			case !ok:
				acquisition.Failure = &AcquisitionFailure{Reason: "acquisition result unavailable"}
			case result.Err != nil:
				acquisition.Failure = &AcquisitionFailure{Reason: result.Err.Error()}
			default:
				acquisition.Status = Readable
				content = result.Bytes
				failureStage = ""
			}
		} else {
			acquisition.Failure = &AcquisitionFailure{Reason: discoveryFailureReason(record)}
		}

		receipt, canonicalContent, canonicalReceipt, err := CanonicalizeReceipt(item, acquisition, content)
		if err != nil {
			return nil, fmt.Errorf("canonicalize discovery record %q: %w", record.Name, err)
		}
		artifacts = append(artifacts, ReceiptArtifact{
			Record:           record,
			Receipt:          receipt,
			Content:          canonicalContent,
			CanonicalReceipt: canonicalReceipt,
			FailureStage:     failureStage,
		})
	}
	return artifacts, nil
}

func discoveryFailureReason(record Record) string {
	if record.Reason != "" {
		return record.Reason
	}
	return "source cannot be acquired"
}
