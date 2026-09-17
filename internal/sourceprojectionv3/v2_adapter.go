package sourceprojectionv3

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"lsp-trace/internal/sourceprojectionv2"
)

const SchemaVersion = "lsp-trace.source-projection.v3"

func PaginateV2[T any](source sourceprojectionv2.WireResult[T], requestDigest string, limits Limits, cursor string) (Page, error) {
	if source.SchemaVersion != sourceprojectionv2.SchemaVersion || source.Authority != 0 || source.SourceGraphComplete != "UNKNOWN" || source.GraphFactsAdded != 0 || source.CustodyMode == "" || source.PhysicalProjectionID == "" || source.RequestPolicyID == "" || source.Status == "" || requestDigest == "" {
		return Page{}, ErrInvalidRequest
	}
	records, custodyDigest, err := v2Records(source)
	if err != nil {
		return Page{}, err
	}
	projectionDigest, err := RecordDigest(records)
	if err != nil {
		return Page{}, err
	}
	return Paginate(records, Request{
		Header: Header{
			SchemaVersion: SchemaVersion, Authority: 0, SourceGraphComplete: "UNKNOWN", GraphFactsAdded: 0,
			CustodyMode: source.CustodyMode, PhysicalProjectionID: source.PhysicalProjectionID,
			RequestPolicyID: source.RequestPolicyID, Status: source.Status,
		},
		Binding: Binding{RequestDigest: requestDigest, CustodyMode: source.CustodyMode, CustodyDigest: custodyDigest, ProjectionDigest: projectionDigest},
		Limits:  limits,
		Cursor:  cursor,
	})
}

func v2Records[T any](source sourceprojectionv2.WireResult[T]) ([]Record, string, error) {
	custodyRaw, err := json.Marshal(source.CustodyBinding)
	if err != nil || !json.Valid(custodyRaw) {
		return nil, "", errors.New("sourceprojectionv3: encode custody binding")
	}
	custodyHash := sha256.Sum256(custodyRaw)
	custodyDigest := "sha256:" + hex.EncodeToString(custodyHash[:])
	records := make([]Record, 0, 6+len(source.DocumentBindings)+len(source.Units)+len(source.Citations)+len(source.EmittedSpans)+len(source.Omissions))
	appendValue := func(kind, key string, value any, objects, ranges, sourceBytes, work uint64) error {
		raw, err := json.Marshal(value)
		if err != nil {
			return err
		}
		records = append(records, Record{Kind: kind, Key: key, Data: raw, Objects: objects, Ranges: ranges, SourceBytes: sourceBytes, Work: work})
		return nil
	}
	if err := appendValue("CUSTODY_BINDING", source.CustodyMode, source.CustodyBinding, 1, 0, 0, 1); err != nil {
		return nil, "", err
	}
	if err := appendValue("DOCUMENT_SELECTION", "selection", source.DocumentSelection, 1, 0, 0, 1); err != nil {
		return nil, "", err
	}
	for i, binding := range source.DocumentBindings {
		if binding.SourceByteLength < 0 {
			return nil, "", ErrInvalidRequest
		}
		if err := appendValue("DOCUMENT_BINDING", fmt.Sprintf("%08d:%s", i, binding.URI), binding, 1, 0, uint64(binding.SourceByteLength), 1); err != nil {
			return nil, "", err
		}
	}
	if err := appendValue("DOCUMENT_ACCOUNTING", "documents", source.DocumentAccounting, 1, 0, 0, 1); err != nil {
		return nil, "", err
	}
	for i, unit := range source.Units {
		ranges := uint64(2)
		if unit.ItemRange != nil {
			ranges++
		}
		if unit.SelectionRange != nil {
			ranges++
		}
		if err := appendValue("UNIT", fmt.Sprintf("%08d:%s", i, unit.UnitID), unit, 1, ranges, 0, 1); err != nil {
			return nil, "", err
		}
	}
	for i, citation := range source.Citations {
		if err := appendValue("CITATION", fmt.Sprintf("%08d:%s", i, citation.CitationID), citation, 1, 2, 0, 1); err != nil {
			return nil, "", err
		}
	}
	for i, span := range source.EmittedSpans {
		if err := appendValue("EMITTED_SPAN", fmt.Sprintf("%08d", i), span, 1, 1, 0, 1); err != nil {
			return nil, "", err
		}
	}
	if err := appendValue("ACCOUNTING", "projection", source.Accounting, 1, 0, 0, 1); err != nil {
		return nil, "", err
	}
	for i, omission := range source.Omissions {
		if err := appendValue("OMISSION", fmt.Sprintf("%08d", i), omission, 1, 0, 0, 1); err != nil {
			return nil, "", err
		}
	}
	if err := appendValue("PRIVACY_SUMMARY", "privacy", source.PrivacySummary, 1, 0, 0, 1); err != nil {
		return nil, "", err
	}
	return records, custodyDigest, nil
}
