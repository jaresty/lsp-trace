//go:build darwin || linux

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	ai "lsp-trace/internal/applicationinventory"
	pilot "lsp-trace/internal/censuslineagepilot"
	product "lsp-trace/internal/privatehostproductv3"
	"os"
	"path/filepath"
	"time"
)

type applicationInventoryTerminalRecordV3 struct {
	Schema, Status, ApplicationSHA256, RunID, SourceRevision, InventoryHead string
	SourceLeads, Dispositions, UntouchedGaps, SuccessfulDescriptions        int
	DispositionCounts                                                       map[string]int
	Calls, RequestBytes, ResponseBytes                                      uint64
	LastSequences                                                           [2]uint64
	Authority                                                               int
	Accepted                                                                bool
	Feature, Consumer, Completeness, Absence, Accounting, OriginalCleanup   string
}

func applicationInventoryDigestV3(raw []byte) string { return pilot.Digest(raw) }
func applicationInventoryValidTerminalV3(raw []byte) bool {
	var t applicationInventoryTerminalRecordV3
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&t) != nil || d.Decode(new(any)) != io.EOF || t.Schema != "private-application-inventory-terminal.v3" || !validApplicationDigest(t.ApplicationSHA256) || !validApplicationDigest(t.RunID) || t.SourceRevision != product.InventorySourceRevisionV3 || t.Authority != 0 || t.Accepted || t.Feature != "UNRESOLVED" || t.Consumer != "UNRESOLVED" || t.Completeness != "UNKNOWN" || t.Absence != "PROHIBITED" || t.SourceLeads != t.Dispositions || t.UntouchedGaps < 0 || t.LastSequences[0] > 16 || t.LastSequences[1] > 16 {
		return false
	}
	switch t.Status {
	case "PROVISIONAL", "PROVISIONAL_WITH_PREPARATION_REFUSALS":
		if !validApplicationDigest(t.InventoryHead) {
			return false
		}
	case "FAILED_INVENTORY_UNKNOWN", "HOST_TOOL_UNAVAILABLE_COST_ONLY_NO_REPLAY":
	default:
		return false
	}
	if t.InventoryHead != "" && !validApplicationDigest(t.InventoryHead) {
		return false
	}
	n := 0
	for _, v := range t.DispositionCounts {
		if v < 0 {
			return false
		}
		n += v
	}
	return n == t.Dispositions
}

// A Run version is not evidence that census admission happened. Reconcile only
// unique approved-lead observations; ambiguous entries never replace valid work
// on other leads. This is metadata conservation, never semantic replay.
func applicationInventoryReconcileV3(m ai.Manifest, report ai.Report) (ai.Report, error) {
	approved := make(map[string]ai.Lead, len(m.Leads))
	for _, l := range m.Leads {
		approved[l.ID] = l
	}
	census := make(map[string]int, len(report.Census))
	invalid := report.Version != "" && report.Version != "lsp-trace.application-inventory.v3" || report.Scope != "" && report.Scope != m.Scope || report.Provenance != "" && report.Provenance != m.Provenance
	unadmitted := len(report.Census) == 0
	for _, l := range report.Census {
		census[l.ID]++
		if census[l.ID] > 1 {
			invalid = true
		}
		if expected, ok := approved[l.ID]; !ok || expected != l {
			census[l.ID] += 2
			invalid = true
		}
	}
	occurrences := make(map[string]int, len(report.Dispositions))
	for _, d := range report.Dispositions {
		occurrences[d.LeadID]++
	}
	valid := make(map[string]ai.Disposition, len(m.Leads))
	for _, d := range report.Dispositions {
		_, ok := approved[d.LeadID]
		if !ok || occurrences[d.LeadID] != 1 || census[d.LeadID] != 1 || d.Reason == "" || len(d.Reason) > 4096 {
			invalid = true
			continue
		}
		switch d.Status {
		case "UNTOUCHED", "PREPARATION_FAILED", "OBSERVED_EMPTY", "UNAVAILABLE", "DESCRIBE_FAILED", "DESCRIBED_UNMATCHED", "DESCRIBED_WITH_MEMBERSHIP":
			valid[d.LeadID] = d
		default:
			invalid = true
		}
	}
	report.Scope, report.Provenance = m.Scope, m.Provenance
	report.Census = append([]ai.Lead(nil), m.Leads...)
	report.UntouchedGaps = append([]string(nil), m.UntouchedGaps...)
	report.Dispositions = nil
	report.SuccessfulDescriptions, report.WithMembership, report.WithoutMembership = 0, 0, 0
	for _, l := range m.Leads {
		d, ok := valid[l.ID]
		if !ok {
			reason := "LEAD_NOT_COMPLETED_NO_SEMANTIC_REPLAY"
			if unadmitted {
				reason = "RUN_CENSUS_NOT_ADMITTED_NO_SEMANTIC_REPLAY"
			}
			if occurrences[l.ID] > 0 {
				reason = "DISPOSITION_REFUSED_NO_SEMANTIC_REPLAY"
			}
			d = ai.Disposition{LeadID: l.ID, Status: "UNTOUCHED", Reason: reason}
		}
		report.Dispositions = append(report.Dispositions, d)
		switch d.Status {
		case "DESCRIBED_UNMATCHED":
			report.SuccessfulDescriptions++
			report.WithoutMembership++
		case "DESCRIBED_WITH_MEMBERSHIP":
			report.SuccessfulDescriptions++
			report.WithMembership++
		}
	}
	if invalid {
		return report, product.ErrInvalid
	}
	return report, nil
}
func applicationInventoryTerminalV3(a *product.ApplicationJournal, q product.InventoryRequestV3, runID string, report ai.Report, status string, code int, out, errout io.Writer, cause error, last ...[2]uint64) int {
	var reconcileErr error
	report, reconcileErr = applicationInventoryReconcileV3(q.Inventory.Manifest, report)
	if reconcileErr != nil {
		status, code, cause = "FAILED_INVENTORY_UNKNOWN", 1, reconcileErr
	}
	counts := map[string]int{}
	for _, d := range report.Dispositions {
		counts[d.Status]++
	}
	h := a.Head()
	t := applicationInventoryTerminalRecordV3{Schema: "private-application-inventory-terminal.v3", Status: status, ApplicationSHA256: h.ApplicationSHA256, RunID: runID, SourceRevision: q.Inventory.SourceRevision, InventoryHead: h.LastCompletePublicationSHA256, SourceLeads: len(report.Census), Dispositions: len(report.Dispositions), UntouchedGaps: len(report.UntouchedGaps), SuccessfulDescriptions: report.SuccessfulDescriptions, DispositionCounts: counts, Calls: h.Observed.Calls, RequestBytes: h.Observed.RequestBytes, ResponseBytes: h.Observed.ResponseBytes, Feature: "UNRESOLVED", Consumer: "UNRESOLVED", Completeness: "UNKNOWN", Absence: "PROHIBITED", Accounting: "KNOWN_PHYSICAL_BYTES_CALLER_LOGICAL_COST_SEPARATE_NOT_AUTHENTICATION", OriginalCleanup: "UNKNOWN_NO_NEW_NATIVE_QUALIFICATION"}
	if len(last) == 1 {
		t.LastSequences = last[0]
	}
	raw, _ := json.Marshal(t)
	if !applicationInventoryValidTerminalV3(raw) || applicationWriteOnce(applicationTerminalPath(q.Configuration.Store, runID), raw) != nil {
		fmt.Fprintln(errout, "private application inventory V3: TERMINAL_PUBLICATION_FAILED")
		return 1
	}
	if cause != nil {
		fmt.Fprintln(errout, "private application inventory V3: execution/refusal retained; no native or cleanup success inferred")
	}
	if encodeHostProductV3(out, t) != 0 {
		return 1
	}
	return code
}

// Called only AFTER the unchanged root-wide attempt charge at the ordinary
// finalization entry. There is no early V3 parsing/publication bypass.
func applicationInventoryFinalizeV3(args []string, raw []byte, store string, p applicationFinalizePolicy, lane *applicationFinalizeLane, input io.Reader, out, errout io.Writer) int {
	fail := func(s string) int { fmt.Fprintln(errout, "private application inventory HOST V3:", s); return 1 }
	q, e := product.DecodeInventoryRequestV3(raw)
	if e != nil || lane.check() != nil || q.Configuration.Store != store || product.ApplicationConfigDigest(q.Configuration) != p.ApplicationSHA256 {
		return fail("FINALIZE_CONFIG_REFUSED_CHARGED")
	}
	header, e := lane.read(filepath.Join(store, "header.json"), 64<<10)
	canonical, _ := json.Marshal(q.Configuration)
	if e != nil || !bytes.Equal(header, canonical) || lane.check() != nil {
		return fail("FINALIZE_APPLICATION_REFUSED_CHARGED")
	}
	var reservation product.OperationReservation
	if lane.object("claim", p.ReservationSHA256, &reservation) != nil || reservation.EventID != p.ReservationSHA256 || reservation.Kind != "coordinator_host_envelope" || reservation.Bounds.ActiveNS != uint64(10*time.Second) || reservation.Bounds.ReadHashBytes < 64<<20 || reservation.Bounds.WriteBytes < 16<<20 {
		return fail("FINALIZE_PREPAYMENT_REFUSED_CHARGED")
	}
	var ticket applicationRunTicket
	if lane.object("data", args[2], &ticket) != nil || ticket.ApplicationSHA256 != p.ApplicationSHA256 || ticket.WaitReservationSHA256 != p.ReservationSHA256 || ticket.Sequence != reservation.Sequence {
		return fail("FINALIZE_TICKET_REFUSED_CHARGED")
	}
	observation, e := lane.observations(input)
	var returns [2]product.CohortRoleReturn
	if e != nil || json.Unmarshal(observation, &returns) != nil {
		return fail("INVALID_HOST_OBSERVATIONS_CHARGED")
	}
	terminal, e := lane.read(applicationTerminalPath(store, args[2]), 64<<10)
	var t applicationInventoryTerminalRecordV3
	if e != nil || !applicationInventoryValidTerminalV3(terminal) || json.Unmarshal(terminal, &t) != nil || t.ApplicationSHA256 != p.ApplicationSHA256 || t.RunID != args[2] || t.SourceLeads != len(q.Inventory.Manifest.Leads) || t.Dispositions != len(q.Inventory.Manifest.Leads) || t.UntouchedGaps != len(q.Inventory.Manifest.UntouchedGaps) || lane.check() != nil {
		return fail("FINALIZE_TERMINAL_REFUSED_CHARGED")
	}
	ids := map[string]bool{}
	kind := "HOST_OBSERVED_RETURNS_NOT_AUTHENTICATED"
	for i, role := range []string{"interpreter", "reviewer"} {
		b, e := lane.read(applicationRoleReadyPath(store, args[2], role), 4096)
		var ready product.InventoryReadyV3
		if e != nil || json.Unmarshal(b, &ready) != nil || ready.ReviewMode != q.Inventory.ReviewMode || ready.Protocol != product.InventoryProtocolV3 || ready.Role != role || ready.PromptSHA256 != q.Configuration.RolePromptSHA256[i] || applicationReturnReason(returns[i], ready.Ready, role, pilot.Digest(terminal), ids[returns[i].LaunchID] && !q.Inventory.ReviewMode.AllowsSameActor()) != "" || returns[i].Sequence != t.LastSequences[i] || lane.check() != nil {
			return fail("FINALIZE_ROLE_BINDING_REFUSED_CHARGED")
		}
		ids[returns[i].LaunchID] = true
		if returns[i].Kind == "SCRIPTED_CONTRACT" {
			kind = "SCRIPTED_CONTRACT_RETURNS_NOT_AGENTS"
		}
	}
	if t.InventoryHead != "" {
		var pub product.InventoryPublicationV3
		if lane.object("data", t.InventoryHead, &pub) != nil || pub.RunID != args[2] || pub.ApplicationSHA256 != p.ApplicationSHA256 || pub.Directory != filepath.Join(store, "inventory-result-"+args[2][7:]) {
			return fail("FINALIZE_PUBLICATION_REFUSED_CHARGED")
		}
		// The existing 512KiB attempt partition is unchanged, including readback.
		// Larger publications remain valid Run artifacts but cannot finalize here.
		total := 0
		for _, a := range pub.Manifest.Artifacts {
			if a.Bytes < 1 || a.Bytes > 384<<10-total {
				return fail("FINALIZE_READBACK_QUOTE_REFUSED_CHARGED")
			}
			total += a.Bytes
		}
		visible, e := os.Lstat(store)
		held, he := lane.root.Stat(".")
		if e != nil || he != nil || !os.SameFile(visible, held) || lane.check() != nil || product.VerifyInventoryPublicationV3(pub, q, product.InventoryPublicationReadbackV3{MaxBytes: 384 << 10, Check: lane.check}) != nil || lane.check() != nil {
			return fail("FINALIZE_PUBLICATION_READBACK_REFUSED_CHARGED")
		}
	}
	view := struct {
		Schema, Status, ApplicationSHA256, RunID, TerminalSHA256, InventoryHead string
		Authority                                                               int
		Accepted                                                                bool
		Completeness, OriginalCleanup                                           string
		ReviewMode                                                              string `json:",omitempty"`
	}{"private-application-inventory-host-observation.v3", kind, p.ApplicationSHA256, args[2], pilot.Digest(terminal), t.InventoryHead, 0, false, "UNKNOWN", t.OriginalCleanup, string(q.Inventory.ReviewMode)}
	b, _ := json.Marshal(view)
	if lane.check() != nil || applicationFinalizePublish(lane.root, "host-observation-"+args[2][7:]+".json", b, true, lane) != nil || lane.check() != nil {
		return fail("FINALIZE_PUBLICATION_FAILED_CHARGED")
	}
	return encodeHostProductV3(out, view)
}
