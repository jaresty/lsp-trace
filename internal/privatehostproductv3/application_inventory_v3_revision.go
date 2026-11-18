//go:build darwin || linux

package privatehostproductv3

import (
	dr "lsp-trace/internal/describerequest"
	cat "lsp-trace/internal/provisionalfeaturecatalog"
)

// The outer frame keeps the application revision. Native source-only records
// instead identify their admitted content in Envelope.SourceRevision. This
// intrinsic check grants no source admission or producer authentication.
func inventoryDescribeRevisionV3(r dr.Record) bool {
	if r.Lineage.ConsumerResolution != dr.PrivateSourceOnly {
		return r.Envelope.SourceRevision == InventorySourceRevisionV3
	}
	return digestValue("sha256:"+r.Envelope.SourceRevision) && dr.ValidateRecord(r) == nil
}

// Readback checks exported relationships, not opaque native custody.
func inventoryEvidenceRevisionV3(e cat.SelectedEvidenceV3, revision string) bool {
	if e.Unit.Version == "" && e.Request.Lineage.ConsumerResolution != dr.PrivateSourceOnly {
		return e.Request.Envelope.SourceRevision == revision
	}
	// Do not compare a content digest to the caller's revision assertion. Bind
	// each in its own domain, including the exact source-only prompt and IDs.
	if e.Unit.Revision != revision || !digestValue(e.Unit.SourceDigest) || e.Request.Lineage.ConsumerResolution != dr.PrivateSourceOnly {
		return false
	}
	expected, err := dr.FormatPrivateSourceRequestCandidate(e.Unit, e.Request.Envelope.DeadlineMS, 8<<20)
	return err == nil && expected == e.Request
}
