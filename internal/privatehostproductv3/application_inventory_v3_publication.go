//go:build darwin || linux

package privatehostproductv3

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	ai "lsp-trace/internal/applicationinventory"
	dw "lsp-trace/internal/describeworker"
	cat "lsp-trace/internal/provisionalfeaturecatalog"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"syscall"
	"time"
)

// InventoryPublicationV3 is distinct from ordinary UnitResultRef/CohortOutcome.
// It refers only to actual Run artifacts; it grants no feature/source authority.
type InventoryPublicationV3 struct {
	Schema, ApplicationSHA256, RunID, SourceRevision, Directory, ManifestSHA256 string
	RequestSHA256, LedgerID                                                     string
	Manifest                                                                    ai.PublicationManifestV3
	Leads, Dispositions, UntouchedGaps, SuccessfulDescriptions                  int
	Authority                                                                   int
	Accepted                                                                    bool
	Feature, Consumer, Completeness, Absence                                    string
}

func readInventoryArtifactV3(root *os.Root, name string, limit int) ([]byte, error) {
	if filepath.Base(name) != name {
		return nil, ErrInvalid
	}
	return readInventoryFileV3(root, name, limit)
}
func readInventoryFileV3(root *os.Root, name string, limit int) ([]byte, error) {
	f, e := root.OpenFile(name, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	st, e := f.Stat()
	if e != nil || !st.Mode().IsRegular() || st.Mode().Perm() != 0600 || st.Size() > int64(limit) || st.Sys().(*syscall.Stat_t).Nlink != 1 {
		return nil, ErrInvalid
	}
	raw, e := io.ReadAll(io.LimitReader(f, int64(limit)+1))
	if len(raw) > limit {
		return nil, ErrInvalid
	}
	return raw, e
}

// The optional lane only narrows the existing readback ceiling and clock.
// It is caller accounting, not authentication or semantic/native authority.
type InventoryPublicationReadbackV3 struct {
	MaxBytes int
	Check    func() error
}

func VerifyInventoryPublicationV3(p InventoryPublicationV3, q InventoryRequestV3, lanes ...InventoryPublicationReadbackV3) error {
	remaining := (4 << 20) + (256 << 10)
	check := func() error { return nil }
	if len(lanes) > 1 {
		return ErrInvalid
	}
	if len(lanes) == 1 {
		if lanes[0].MaxBytes < 1 || lanes[0].MaxBytes > remaining || lanes[0].Check == nil {
			return ErrInvalid
		}
		remaining, check = lanes[0].MaxBytes, lanes[0].Check
	}
	read := func(root *os.Root, name string, limit int) ([]byte, error) {
		if check() != nil || remaining < 1 {
			return nil, ErrPaused
		}
		b, e := readInventoryFileV3(root, name, min(limit, remaining))
		remaining -= len(b)
		if check() != nil {
			return nil, ErrPaused
		}
		return b, e
	}
	binding, e := inventoryPublicationAdmissionV3(q, p.RunID, read)
	if e != nil {
		return fmt.Errorf("inventory admission: %w", e)
	}
	if p.ApplicationSHA256 != binding.ApplicationSHA256 || p.RequestSHA256 != q.ApprovalSHA256 || p.LedgerID != inventoryLedgerIDV3(binding) || p.Directory != filepath.Join(q.Configuration.Store, "inventory-result-"+p.RunID[7:]) || check() != nil {
		return ErrInvalid
	}
	if p.Schema != "private-application-inventory-publication-ref.v3" || !digestValue(p.ApplicationSHA256) || !digestValue(p.RunID) || p.SourceRevision != InventorySourceRevisionV3 || !digestValue(p.ManifestSHA256) || p.Authority != 0 || p.Accepted || p.Feature != "UNRESOLVED" || p.Consumer != "UNRESOLVED" || p.Completeness != "UNKNOWN" || p.Absence != "PROHIBITED" || privateDir(p.Directory) != nil {
		return ErrInvalid
	}
	root, e := os.OpenRoot(p.Directory)
	if e != nil {
		return e
	}
	defer root.Close()
	raw, e := read(root, "manifest.json", 64<<10)
	var m ai.PublicationManifestV3
	if e != nil || hash(raw) != p.ManifestSHA256 || cat.DecodeNativeV3(raw, &m) != nil || !bytes.Equal(mustJSON(m), mustJSON(p.Manifest)) || m.Version != "lsp-trace.application-inventory-publication.v3" || m.Status != "PROVISIONAL" || m.Authority != 0 || m.Accepted || m.Feature != "UNRESOLVED" || m.Consumer != "UNRESOLVED" || m.Completeness != "UNKNOWN" || m.Absence != "PROHIBITED" || len(m.Artifacts) != 4 {
		return ErrInvalid
	}
	names := []string{"catalog-base.json", "catalog-proposed.json", "catalog-corrected.json", "report.json"}
	total := 0
	var report ai.Report
	var catalogs [3]cat.NativeCatalogWireV3
	for i, a := range m.Artifacts {
		if a.Name != names[i] || a.Bytes < 1 || a.Bytes > 4<<20-total {
			return ErrInvalid
		}
		total += a.Bytes
		data, e := read(root, a.Name, a.Bytes)
		if e != nil || len(data) != a.Bytes || hash(data) != a.SHA256 {
			return ErrInvalid
		}
		if i == 3 {
			if cat.DecodeNativeV3(data, &report) != nil {
				return ErrInvalid
			}
		} else if cat.DecodeNativeV3(data, &catalogs[i]) != nil {
			return ErrInvalid
		}
		if check() != nil {
			return ErrPaused
		}
	}
	if !report.Published || report.Authority != 0 || report.Accepted || report.Feature != "UNRESOLVED" || report.Consumer != "UNRESOLVED" || report.Completeness != "UNKNOWN" || report.Absence != "PROHIBITED" || len(report.Census) != p.Leads || len(report.Dispositions) != p.Dispositions || len(report.UntouchedGaps) != p.UntouchedGaps || len(report.Census) != len(report.Dispositions) || report.SuccessfulDescriptions != p.SuccessfulDescriptions {
		return ErrInvalid
	}
	if report.CallerAccounting != "PRE_PUBLICATION_SNAPSHOT_FINAL_IN_MANIFEST" || report.CallbackContainment != "COOPERATIVE_NOT_OS_PHYSICAL" || report.CallerBytes > m.CallerBytes || report.SemanticReservedNS != m.SemanticReservedNS {
		return ErrInvalid
	}
	if !inventoryReportManifestV3(report, q.Inventory.Manifest) {
		return fmt.Errorf("inventory manifest: %w", ErrInvalid)
	}
	if e = inventoryCatalogReportV3(catalogs, report, binding); e != nil {
		return fmt.Errorf("inventory catalog report: %w", e)
	}
	if check() != nil {
		return ErrPaused
	}
	return nil
}
func (a *ApplicationJournal) PublishInventoryRunV3(q InventoryRequestV3, runID, destination string, report ai.Report) (string, error) {
	j := a.implementation
	if !digestValue(runID) || j.config.Schema != "private-host-application-config.v3" || q.Configuration != j.config || !report.Published || q.ApprovalSHA256 != InventoryRequestDigestV3(q) || destination != filepath.Join(j.config.Store, "inventory-result-"+runID[7:]) {
		return "", ErrInvalid
	}
	// This fixed upper quote was prepaid in the V3 ledger before Run effects.
	root, e := os.OpenRoot(destination)
	if e != nil {
		return "", e
	}
	raw, e := readInventoryArtifactV3(root, "manifest.json", 64<<10)
	e = errors.Join(e, root.Close())
	var m ai.PublicationManifestV3
	if e != nil || cat.DecodeNativeV3(raw, &m) != nil {
		return "", ErrInvalid
	}
	p := InventoryPublicationV3{Schema: "private-application-inventory-publication-ref.v3", ApplicationSHA256: j.head.ApplicationSHA256, RunID: runID, SourceRevision: q.Inventory.SourceRevision, RequestSHA256: q.ApprovalSHA256, LedgerID: inventoryLedgerIDV3(InventoryBindingV3{ApplicationSHA256: j.head.ApplicationSHA256, RunID: runID}), Directory: destination, ManifestSHA256: hash(raw), Manifest: m, Leads: len(report.Census), Dispositions: len(report.Dispositions), UntouchedGaps: len(report.UntouchedGaps), SuccessfulDescriptions: report.SuccessfulDescriptions, Feature: "UNRESOLVED", Consumer: "UNRESOLVED", Completeness: "UNKNOWN", Absence: "PROHIBITED"}
	if e = VerifyInventoryPublicationV3(p, q); e != nil {
		return "", e
	}
	// The supplied result and independently re-read actual Run report must agree.
	root, e = os.OpenRoot(destination)
	if e != nil {
		return "", e
	}
	rawReport, re := readInventoryArtifactV3(root, "report.json", 4<<20)
	ce := root.Close()
	var actual ai.Report
	if re != nil || ce != nil || cat.DecodeNativeV3(rawReport, &actual) != nil {
		return "", ErrInvalid
	}
	// Disk report precedes PublishV3's final charged allocation. Its manifest
	// records final usage, also returned by Run's deferred usage snapshot.
	if actual.CallerAccounting != "PRE_PUBLICATION_SNAPSHOT_FINAL_IN_MANIFEST" || actual.CallerBytes > m.CallerBytes || report.CallerBytes != m.CallerBytes || actual.SemanticReservedNS != m.SemanticReservedNS || report.SemanticReservedNS != m.SemanticReservedNS {
		return "", fmt.Errorf("Run accounting snapshot: %w", ErrInvalid)
	}
	actual.CallerBytes = report.CallerBytes
	if !reflect.DeepEqual(actual, report) {
		return "", fmt.Errorf("Run result readback: %w", ErrInvalid)
	}
	encoded := mustJSON(p)
	id := hash(encoded)
	h := j.head
	output := uint64(len(raw))
	for _, artifact := range m.Artifacts {
		output += uint64(artifact.Bytes)
	}
	if output > 4<<20 {
		return "", ErrInvalid
	}
	h.Observed.OutputBytes += output
	h.CursorSHA256 = id
	h.LastCompletePublicationSHA256 = id
	return id, j.commit("INVENTORY_RUN_PUBLICATION_V3", id, h, applicationObject{applicationObjectName("data", id), encoded})
}

// Read original admission evidence without OpenApplication/attempt/reservation:
// verification cannot renew funding, READY, deadlines, or execution authority.
func inventoryPublicationAdmissionV3(q InventoryRequestV3, runID string, read func(*os.Root, string, int) ([]byte, error)) (InventoryBindingV3, error) {
	var empty InventoryBindingV3
	if !digestValue(runID) || q.Schema != "private-host-application-inventory-request.v3" || q.ApprovalSHA256 != InventoryRequestDigestV3(q) || ValidateInventoryRequestV3(q) != nil {
		return empty, ErrInvalid
	}
	root, e := os.OpenRoot(q.Configuration.Store)
	if e != nil {
		return empty, e
	}
	defer root.Close()
	header, e := read(root, "header.json", 64<<10)
	if e != nil || !bytes.Equal(header, mustJSON(q.Configuration)) {
		return empty, ErrInvalid
	}
	get := func(kind, id string, v any) error {
		if !digestValue(id) {
			return ErrInvalid
		}
		b, e := read(root, applicationObjectName(kind, id), ApplicationRecordLimit)
		if e != nil || (kind == "data" && hash(b) != id) || cat.DecodeNativeV3(b, v) != nil {
			return ErrInvalid
		}
		return nil
	}
	var ticket struct {
		ApplicationSHA256, CursorSHA256 string
		WaitReservationSHA256           string
		Sequence, StartGlobal           uint64
	}
	app := ApplicationConfigDigest(q.Configuration)
	if get("data", runID, &ticket) != nil || ticket.ApplicationSHA256 != app {
		return empty, fmt.Errorf("ticket: %w", ErrInvalid)
	}
	var wait, single OperationReservation
	claimID := hash([]byte(runID + "/inventory-single-run"))
	if get("claim", ticket.WaitReservationSHA256, &wait) != nil || wait.EventID != ticket.WaitReservationSHA256 || wait.Kind != "coordinator_host_envelope" || wait.Sequence != ticket.Sequence || wait.Bounds.ActiveNS != uint64(10*time.Second) || wait.Bounds.ReadHashBytes < 64<<20 || wait.Bounds.WriteBytes < 16<<20 || get("claim", claimID, &single) != nil || single.EventID != claimID || single.Kind != "inventory_single_run.v3" || single.UnitID != "" || single.Bounds != (ApplicationBounds{}) || single.Sequence <= wait.Sequence {
		return empty, fmt.Errorf("run claims: %w", ErrInvalid)
	}
	var ledger inventoryLedgerV3
	id := inventoryLedgerIDV3(InventoryBindingV3{ApplicationSHA256: app, RunID: runID})
	if get("inventory-ledger", id, &ledger) != nil {
		return empty, fmt.Errorf("ledger read: %w", ErrInvalid)
	}
	b := ledger.Binding
	planned := uint64(min(len(q.Inventory.Manifest.Leads), q.Inventory.MaxNativeAttempts) + 2)
	retention, ok := applicationTimes(applicationRetentionEnvelope(), 16)
	if !ok || b.ApplicationSHA256 != app || b.ConfigSHA256 != q.ApprovalSHA256 || b.RunID != runID || b.Grant != ApplicationGrantDigest(q.Configuration.Grant) || b.SourceRevision != q.Inventory.SourceRevision || b.Pins != q.Inventory.Pins || b.Mailbox != InventoryMailboxPathV3(q.Configuration.Store, runID) || b.RootIdentity == "" || b.PlannedCalls != planned || b.ResponseLimit != (64<<10)/planned || ledger.Allowance != inventoryAllowanceV3(b) || ledger.Retention != retention || ledger.Deadline <= 0 {
		return empty, fmt.Errorf("ledger binding: %w", ErrInvalid)
	}
	for i, role := range []string{"interpreter", "reviewer"} {
		r := b.Ready[i]
		if r.Protocol != InventoryProtocolV3 || r.Role != role || r.LaunchID == "" || len(r.LaunchID) > 256 || r.PromptSHA256 != q.Configuration.RolePromptSHA256[i] {
			return empty, ErrInvalid
		}
	}
	if b.Ready[0].LaunchID == b.Ready[1].LaunchID {
		return empty, ErrInvalid
	}
	return b, nil
}

// Run normalizes nil and non-nil empty manifest collections to nil snapshots.
// Nonempty fields and ordering are exact; no sorting/deduplication is permitted.
func inventoryReportManifestV3(r ai.Report, m ai.Manifest) bool {
	return r.Version == "lsp-trace.application-inventory.v3" && r.Status == "PROVISIONAL" && r.Scope == m.Scope && r.Provenance == m.Provenance && reflect.DeepEqual(r.Census, append([]ai.Lead(nil), m.Leads...)) && reflect.DeepEqual(r.UntouchedGaps, append([]string(nil), m.UntouchedGaps...))
}
func inventoryDispositionStatusV3(s string) bool {
	switch s {
	case "UNTOUCHED", "PREPARATION_FAILED", "OBSERVED_EMPTY", "UNAVAILABLE", "DESCRIBE_FAILED", "DESCRIBED_UNMATCHED", "DESCRIBED_WITH_MEMBERSHIP":
		return true
	}
	return false
}

// This verifies exported relationships, not opaque custody or native execution.
func inventoryCatalogReportV3(cs [3]cat.NativeCatalogWireV3, r ai.Report, b InventoryBindingV3) error {
	for i, c := range cs {
		actor := b.Ready[0].LaunchID
		if i == 2 {
			actor = b.Ready[1].LaunchID
		}
		if c.Version != "lsp-trace.native-catalog.v3" || c.Status != "PROVISIONAL" || c.Authority != 0 || c.Accepted || c.Feature != "UNRESOLVED" || c.Consumer != "UNRESOLVED" || c.Completeness != "UNKNOWN" || c.Absence != "PROHIBITED" || c.Origin != "SUPPLIED_UNAUTHENTICATED" || c.ReviewIdentity != "NOT_AUTHENTICATED" || c.Actor != actor || c.Evidence == nil || c.Groups == nil || c.Affected == nil {
			return ErrInvalid
		}
		revision := c.Revision
		c.Revision = ""
		if revision != hash(mustJSON(c)) {
			return ErrInvalid
		}
		if i == 0 {
			if c.Predecessor != "" || c.Reason != "" || len(c.Groups) != 0 || len(c.Affected) != 0 {
				return ErrInvalid
			}
		} else {
			if c.Predecessor != cs[i-1].Revision || c.Reason == "" || len(c.Reason) > 4096 || !reflect.DeepEqual(c.Evidence, cs[0].Evidence) || (i == 1 && len(c.Affected) != 0) {
				return ErrInvalid
			}
		}
	}
	leadEvidence := map[string]string{}
	eligible := map[string]bool{}
	for i, e := range cs[0].Evidence {
		if e.ID == "" || (i > 0 && cs[0].Evidence[i-1].ID >= e.ID) || e.Interpreter != b.Ready[0].LaunchID || e.Origin != dw.SuppliedTerminalResponseOnly || e.Execution != dw.NoProcessOrHostExecutionVerified || e.Request.Envelope.SourceRevision != b.SourceRevision {
			return ErrInvalid
		}
		record, err := dw.ParseResponseRecordV2(e.Record)
		if err != nil || record.ID() != e.ID {
			return ErrInvalid
		}
		h := record.Host()
		if h.AttemptID == "" || leadEvidence[h.AttemptID] != "" || h.RequestRecordID != e.Request.RecordID || !inventoryPinsV3(h.Pins, b.Pins) || h.Pins.PromptSHA256 != "sha256:"+e.Request.Envelope.InputSHA256 {
			return ErrInvalid
		}
		leadEvidence[h.AttemptID] = e.ID
		eligible[e.ID] = true
	}
	var finalMembers map[string]bool
	for i, c := range cs {
		members, groups := map[string]bool{}, map[string]cat.NativeGroupV3{}
		for n, g := range c.Groups {
			if g.ID == "" || len(g.ID) > 256 || (n > 0 && c.Groups[n-1].ID >= g.ID) || g.Label == "" || len(g.Label) > 4096 || g.Reason == "" || len(g.Reason) > 4096 || len(g.Members) == 0 || !sort.StringsAreSorted(g.Members) {
				return ErrInvalid
			}
			groups[g.ID] = g
			for _, id := range g.Members {
				if !eligible[id] || members[id] {
					return ErrInvalid
				}
				members[id] = true
			}
		}
		if c.WithMembership != len(members) || c.WithoutMembership != len(eligible)-len(members) {
			return ErrInvalid
		}
		if i > 0 {
			affected := map[string]bool{}
			old := map[string]bool{}
			for _, id := range c.Affected {
				if affected[id] {
					return ErrInvalid
				}
				affected[id] = true
			}
			for _, g := range cs[i-1].Groups {
				old[g.ID] = true
				next, ok := groups[g.ID]
				if (!ok || !reflect.DeepEqual(g, next)) != affected[g.ID] {
					return ErrInvalid
				}
			}
			for id := range affected {
				if !old[id] {
					return ErrInvalid
				}
			}
		}
		finalMembers = members
	}
	if r.CatalogRevision != cs[2].Revision || r.SuccessfulDescriptions != len(eligible) || r.WithMembership != cs[2].WithMembership || r.WithoutMembership != cs[2].WithoutMembership || r.SuccessfulDescriptions != r.WithMembership+r.WithoutMembership || len(r.Dispositions) != len(r.Census) {
		return ErrInvalid
	}
	used := map[string]bool{}
	prepared, described, failedPrepare, failedDescribe := 0, 0, 0, 0
	for i, d := range r.Dispositions {
		if d.LeadID != r.Census[i].ID || used[d.LeadID] || !inventoryDispositionStatusV3(d.Status) || d.Reason == "" || len(d.Reason) > 4096 {
			return ErrInvalid
		}
		used[d.LeadID] = true
		id := leadEvidence[d.LeadID]
		isDescription := d.Status == "DESCRIBED_UNMATCHED" || d.Status == "DESCRIBED_WITH_MEMBERSHIP"
		if (id != "") != isDescription {
			return ErrInvalid
		}
		if isDescription && (finalMembers[id] != (d.Status == "DESCRIBED_WITH_MEMBERSHIP")) {
			return ErrInvalid
		}
		if d.Status != "UNTOUCHED" {
			prepared++
		}
		if isDescription {
			described++
		}
		if d.Status == "PREPARATION_FAILED" {
			failedPrepare++
		}
		if d.Status == "DESCRIBE_FAILED" {
			failedDescribe++
		}
		if uint64(i) >= b.PlannedCalls-2 && d.Status != "UNTOUCHED" {
			return ErrInvalid
		}
	}
	if described != len(eligible) || len(leadEvidence) != described || prepared != int(b.PlannedCalls-2) {
		return ErrInvalid
	}
	expected := map[string]ai.Stage{"census": {Requested: len(r.Census), Successful: len(r.Census)}, "native_preparation": {Requested: prepared, Successful: prepared - failedPrepare, Failed: failedPrepare}, "describe": {Requested: described + failedDescribe, Successful: described, Failed: failedDescribe}, "group_decision": {Requested: 1, Successful: 1}, "review": {Requested: 1, Successful: 1}, "catalog": {Requested: 3, Successful: 3}, "publication": {Requested: 1, Successful: 1}}
	if !reflect.DeepEqual(r.Stages, expected) {
		return ErrInvalid
	}
	return nil
}
