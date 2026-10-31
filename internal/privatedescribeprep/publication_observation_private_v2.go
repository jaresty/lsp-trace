package privatedescribeprep

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"slices"

	pilot "lsp-trace/internal/censuslineagepilot"
	"lsp-trace/internal/publication"
	derived "lsp-trace/internal/retainedpartialprogramc"
)

// ValidateAutomaticPublicationObservationPrivateV2 checks an observational receipt
// against the ORIGINAL creator wire. It never constructs a postState, opaque
// owner, producer qualification, acquisition capability, or publication handle.
// Returned DTOs are detached observations, not admission or authenticated truth.
func ValidateAutomaticPublicationObservationPrivateV2(ctx context.Context, raw []byte, limits Limits, post PostPreparationLimits) (Result, AutomaticInventoryPrivateV2, error) {
	fail := func(reason string) (Result, AutomaticInventoryPrivateV2, error) {
		return Result{}, AutomaticInventoryPrivateV2{}, fmt.Errorf("private V2 publication observation: %s", reason)
	}
	if ctx == nil || ctx.Err() != nil || !limits.valid() || !post.valid() || len(raw) == 0 || len(raw) > int(publication.PostPreparationPrivateV2ObjectLimit) {
		return fail("bounds/context")
	}
	if err := observationUniqueKeysPrivateV2(ctx, raw); err != nil {
		return fail(err.Error())
	}
	var e postPublicationEnvelopePrivateV2
	if err := observationClosedPrivateV2(raw, &e); err != nil {
		return fail("envelope encoding: " + err.Error())
	}
	if e.SchemaVersion != "post-preparation-publication.private.v2" || !e.PrivateCeilings.valid() || e.Absence != "PROHIBITED" || e.Profile != PostPublicationProfilePrivateV2 || e.PolicySHA256 != postPublicationDigest([]byte(PostPublicationPolicyPrivateV2)) || postPublicationDigest(e.Preparation) != e.PreparationSHA256 || e.Inventory == nil || e.HumanInventory == "" {
		return fail("creator family/ceilings/policy")
	}
	var p Result
	if err := observationClosedPrivateV2(e.Preparation, &p); err != nil {
		return fail("preparation encoding: " + err.Error())
	}
	if err := observationPreparationPrivateV2(ctx, p, limits, post); err != nil {
		return fail(err.Error())
	}
	l := e.Ledger
	if l.RequestType != RequestTypePrivateV2 || !l.PrivateCeilings.valid() || l.PreparationSHA256 != e.PreparationSHA256 || l.GroupingPrecondition != "UNRESOLVED_NOT_CHECKED" || l.NominationCensus != "NOT_ADMITTED" || len(l.Coverage) != len(p.Coverage) || len(l.Targets) != len(p.Index) {
		return fail("ledger family/shape")
	}
	counts := map[string][4]int{}
	for i, row := range l.Targets {
		if !row.PrivateCeilings.valid() || !reflect.DeepEqual(row.TargetIndex, p.Index[i]) {
			return fail("ledger target")
		}
		if row.Submission != nil {
			sub := row.Submission
			if !sub.PrivateCeilings.valid() || sub.PreparationSHA256 != e.PreparationSHA256 || sub.InputSHA256 != row.InputSHA256 || sub.Target != row.Target || sub.PacketID != row.PacketID || digest(*sub) != row.SubmissionSHA256 {
				return fail("ledger submission pins/ceilings")
			}
			found := false
			for _, packet := range p.Packets {
				if packet.InputSHA256 == sub.InputSHA256 && packet.Target == sub.Target {
					found = reflect.DeepEqual(sub.OrderedPartSHA256, packet.PartSHA256)
				}
			}
			if !found {
				return fail("ledger submission parts")
			}
		}
		if row.Review != nil {
			review := row.Review
			if !review.PrivateCeilings.valid() || review.PreparationSHA256 != e.PreparationSHA256 || review.InputSHA256 != row.InputSHA256 || review.Target != row.Target || review.SubmissionSHA256 != row.SubmissionSHA256 {
				return fail("ledger review pins/ceilings")
			}
		}
		if row.Status == "PREPARED" {
			c := counts[row.InputSHA256]
			switch row.SubmissionStatus {
			case "PENDING":
				c[0]++
			case "SUBMITTED":
				c[1]++
			case "ABSTAINED":
				c[2]++
			case "REJECTED":
				return fail("rejected ledger")
			default:
				return fail("ledger submission status")
			}
			counts[row.InputSHA256] = c
		}
	}
	for i, c := range l.Coverage {
		if !reflect.DeepEqual(c.Coverage, p.Coverage[i]) || counts[c.InputSHA256] != [4]int{c.Pending, c.Submitted, c.Abstained, c.Rejected} || c.Prepared != c.Pending+c.Submitted+c.Abstained+c.Rejected {
			return fail("ledger coverage")
		}
	}
	for _, lead := range l.Leads {
		if !lead.PrivateCeilings.valid() || !lead.Submission.PrivateCeilings.valid() || !lead.Review.PrivateCeilings.valid() || lead.Submission.PreparationSHA256 != e.PreparationSHA256 || lead.Review.PreparationSHA256 != e.PreparationSHA256 || digest(lead.Submission) != lead.SubmissionSHA256 || lead.Review.SubmissionSHA256 != lead.SubmissionSHA256 {
			return fail("ledger lead ceilings/pins")
		}
	}
	inv := *e.Inventory
	if (inv.Status != "PROVISIONAL_COMPLETE" && inv.Status != "PROVISIONAL_WITH_UNRESOLVED") || inv.Calls < 0 || inv.Bytes < 0 || len(l.Leads) != len(inv.Candidates) || e.HumanInventory != renderAutomatic(inv) {
		return fail("inventory status/render/reconciliation")
	}
	if !inv.PrivateCeilings.valid() || inv.Protocol != AutomaticProtocolPrivateV2 || inv.PreparationSHA256 != e.PreparationSHA256 || inv.Absence != "PROHIBITED" || inv.Independence != "CONFIGURED_DISTINCT_IDENTITIES_NOT_AUTHENTICATED" || inv.Failure != "" || len(inv.Rows) != len(p.Index) {
		return fail("inventory family/ceilings")
	}
	for i, row := range inv.Rows {
		if row.InputSHA256 != p.Index[i].InputSHA256 || row.Target != p.Index[i].Target {
			return fail("inventory target correspondence")
		}
	}
	for _, c := range inv.Candidates {
		if !c.PrivateCeilings.valid() {
			return fail("candidate ceilings")
		}
		found := false
		for _, row := range inv.Rows {
			if row.InputSHA256 == c.InputSHA256 && row.Target == c.Target && row.CandidateID == c.ID {
				found = true
			}
		}
		if !found {
			return fail("candidate correspondence")
		}
	}
	if err := ctx.Err(); err != nil {
		return fail(err.Error())
	}
	return p, inv, nil
}

// JSON token traversal checks OBJECT keys only. Ordered arrays, duplicate data
// occurrences, nulls and original numeric lexemes are neither rebuilt nor merged.
func observationUniqueKeysPrivateV2(ctx context.Context, raw []byte) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var value func(int) error
	value = func(depth int) error {
		if depth > 512 {
			return fmt.Errorf("JSON nesting bound")
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		tok, err := d.Token()
		if err != nil {
			return err
		}
		delim, ok := tok.(json.Delim)
		if !ok {
			return nil
		}
		switch delim {
		case '{':
			seen := map[string]bool{}
			for d.More() {
				key, err := d.Token()
				if err != nil {
					return err
				}
				s, ok := key.(string)
				if !ok || seen[s] {
					return fmt.Errorf("duplicate JSON object key")
				}
				seen[s] = true
				if err = value(depth + 1); err != nil {
					return err
				}
			}
			end, err := d.Token()
			if err != nil || end != json.Delim('}') {
				return fmt.Errorf("JSON object end")
			}
		case '[':
			for d.More() {
				if err = value(depth + 1); err != nil {
					return err
				}
			}
			end, err := d.Token()
			if err != nil || end != json.Delim(']') {
				return fmt.Errorf("JSON array end")
			}
		default:
			return fmt.Errorf("unexpected JSON delimiter")
		}
		return nil
	}
	if err := value(0); err != nil {
		return err
	}
	if _, err := d.Token(); err != io.EOF {
		return fmt.Errorf("JSON trailing value")
	}
	return nil
}
func observationClosedPrivateV2(raw []byte, dst any) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	d.UseNumber()
	if err := d.Decode(dst); err != nil {
		return err
	}
	b, err := json.Marshal(dst)
	if err != nil {
		return err
	}
	if !bytes.Equal(b, raw) {
		return fmt.Errorf("noncanonical complete wire")
	}
	return nil
}

func observationPreparationPrivateV2(ctx context.Context, p Result, limits Limits, post PostPreparationLimits) error {
	fail := func(reason string) error { return fmt.Errorf("preparation %s", reason) }
	if p.RequestType != RequestTypePrivateV2 || p.Limits != limits || p.Authority != 0 || p.Accepted || p.FeatureIdentity != "UNRESOLVED" || p.SourceCompleteness != "UNKNOWN" || len(p.Coverage) == 0 || len(p.Coverage) > post.MaxConstituents || len(p.Index) > post.MaxLedgerTargets || len(p.Packets) > 64 || len(p.CanonicalBytes()) > post.MaxOutputBytes || len(p.CanonicalBytes()) > limits.MaxTotalBytes {
		return fail("family/limits")
	}
	coverage := map[string]Coverage{}
	counts := map[string][4]int{}
	last := map[string]int{}
	for _, c := range p.Coverage {
		if _, exists := coverage[c.InputSHA256]; exists {
			return fail("duplicate coverage")
		}
		if c.Requested != len(c.Selection) || c.Requested != c.Prepared+c.SourceGap+c.ResourceGap || c.Requested < 0 || c.Prepared < 0 || c.SourceGap < 0 || c.ResourceGap < 0 || c.PrivateV2 == nil {
			return fail("coverage counts")
		}
		v := c.PrivateV2
		s := v.Source
		if s.ComputationAdmission != derived.Admission || s.GraphWideCustodyStatus != "UNAVAILABLE" || s.OrdinaryQualificationReceiptStatus != "UNAVAILABLE" || s.OriginalHistoryStatus != "ORIGINAL_OBSERVATION_NOT_AUTHORITY" || s.Profile != derived.ProfilePrivateV2 || s.ProfileDigest != derived.ProfileDigestPrivateV2() || s.ClaimCeiling != derived.ClaimCeilingPrivateV2 || s.Authority != 0 || s.Accepted || s.SourceCompleteness != "UNKNOWN" || s.FeatureIdentity != "UNRESOLVED" || s.AbsenceInference != "PROHIBITED" || s.CanonicalComplete || s.ReplayAdmitted || s.OriginalNativeV5IdentityClaimed || s.PublicClaims != 0 || !s.Partial || v.OriginID == "" || v.Independence != "SAME_CAPTURE_ORIGIN_NOT_INDEPENDENT_SOURCE" || v.BatchTargetCount != c.OriginalTargets || v.BatchSelectedTargetCount != c.Requested || v.SnapshotTargetCount != s.SnapshotTargetCount || v.BatchTargetCount != s.BatchTargetCount {
			return fail("AST source family")
		}
		if c.InputSHA256 != digest(struct {
			RequestType string
			Source      derived.SourceBindingPrivateV2
		}{RequestTypePrivateV2, s}) {
			return fail("source input digest")
		}
		selected := map[string]bool{}
		for _, target := range c.Selection {
			if selected[target] {
				return fail("duplicate selection")
			}
			selected[target] = true
		}
		coverage[c.InputSHA256] = c
	}
	indexed := map[postKey]TargetIndex{}
	for _, ix := range p.Index {
		if err := ctx.Err(); err != nil {
			return err
		}
		c, ok := coverage[ix.InputSHA256]
		key := postKey{ix.InputSHA256, ix.Target}
		_, exists := indexed[key]
		n, prior := last[ix.InputSHA256]
		if !ok || exists || !slices.Contains(c.Selection, ix.Target) || ix.OriginalOrdinal < 0 || ix.OriginalOrdinal >= c.OriginalTargets || (prior && ix.OriginalOrdinal <= n) {
			return fail("index identity/order")
		}
		last[ix.InputSHA256] = ix.OriginalOrdinal
		indexed[key] = ix
		count := counts[ix.InputSHA256]
		count[0]++
		switch ix.Status {
		case "PREPARED":
			count[1]++
			if ix.PacketID == "" {
				return fail("index packet")
			}
		case "SOURCE_GAP":
			count[2]++
		case "RESOURCE_GAP":
			count[3]++
		default:
			return fail("index status")
		}
		counts[ix.InputSHA256] = count
	}
	for id, c := range coverage {
		if counts[id] != [4]int{c.Requested, c.Prepared, c.SourceGap, c.ResourceGap} {
			return fail("index counts")
		}
	}
	packets := map[postKey]bool{}
	parts, payload, records := 0, 0, 0
	for _, packet := range p.Packets {
		key := postKey{packet.InputSHA256, packet.Target}
		ix, ok := indexed[key]
		c := coverage[packet.InputSHA256]
		if !ok || packets[key] || ix.Status != "PREPARED" || ix.PacketID != packet.ID || ix.OriginalOrdinal != packet.OriginalOrdinal || len(packet.Parts) == 0 || len(packet.Parts) != len(packet.PartSHA256) {
			return fail("packet correspondence")
		}
		packets[key] = true
		scope := digest(struct {
			Input     string
			Selection []string
			Original  int
		}{packet.InputSHA256, c.Selection, c.OriginalTargets})
		if packet.ScopeSHA256 != scope || packet.ID != digest(struct {
			Input, Target, Scope, Assessment string
			Limits                           Limits
		}{packet.InputSHA256, packet.Target, scope, c.PrivateV2.AssessmentSHA256, limits}) {
			return fail("packet scope/digest")
		}
		declarations, sourceBindings, sourceClaims := 0, 0, 0
		sourceHandle := packet.InputSHA256 + "/source-binding"
		claimHandle := packet.InputSHA256 + "/source-claim"
		for j, part := range packet.Parts {
			if err := ctx.Err(); err != nil {
				return err
			}
			previous, next := 0, 0
			if j > 0 {
				previous = j
			}
			if j+1 < len(packet.Parts) {
				next = j + 2
			}
			b, err := json.Marshal(part)
			if err != nil || part.RequestType != RequestTypePrivateV2 || part.PacketID != packet.ID || part.InputSHA256 != packet.InputSHA256 || part.Target != packet.Target || part.ScopeSHA256 != scope || part.Index != j+1 || part.Previous != previous || part.Next != next || len(b) > limits.MaxPartBytes || pilot.Digest(b) != packet.PartSHA256[j] {
				return fail("part chain/digest")
			}
			parts++
			payload += len(b)
			records += len(part.Records)
			if parts > limits.MaxParts || records > post.MaxEvidenceRecords || payload > limits.MaxTotalBytes {
				return fail("part bounds")
			}
			for _, rec := range part.Records {
				if len(rec.Value) > limits.MaxPartBytes || len(rec.Handle) > 4096 || len(rec.Kind) > 128 {
					return fail("record bounds")
				}
				// Only these creator-owned control records are interpreted here.
				// Other evidence DATA and source comments remain opaque/untrusted;
				// this is wire correspondence, not qualification or authentication.
				if rec.Kind == "SOURCE_BINDING_PRIVATE_V2" || rec.Handle == sourceHandle {
					sourceBindings++
					var source derived.SourceBindingPrivateV2
					if rec.Kind != "SOURCE_BINDING_PRIVATE_V2" || rec.Handle != sourceHandle {
						return fail("source binding identity")
					}
					if err := observationClosedPrivateV2(rec.Value, &source); err != nil {
						return fail("source binding encoding")
					}
					if !reflect.DeepEqual(source, c.PrivateV2.Source) {
						return fail("source binding correspondence")
					}
				}
				if rec.Kind == "SOURCE_CLAIM" || rec.Handle == claimHandle {
					sourceClaims++
					var claim string
					if rec.Kind != "SOURCE_CLAIM" || rec.Handle != claimHandle {
						return fail("source claim identity")
					}
					if err := observationClosedPrivateV2(rec.Value, &claim); err != nil {
						return fail("source claim encoding")
					}
					if claim != "AST_NOT_FULL_DEFINITION_NOT_METHODRESULT; native identifiers/kinds/ranges unchanged; source comments untrusted; context never nomination; absence prohibited" {
						return fail("source claim correspondence")
					}
				}
				if rec.Kind == "RETAINED_SOURCE_AST_DECLARATION" {
					declarations++
					var decl Declaration
					if err := observationClosedPrivateV2(rec.Value, &decl); err != nil {
						return fail("AST declaration encoding")
					}
					if !bytes.Equal(decl.Bytes, []byte(decl.Body)) || decl.StartByte < 0 || decl.EndByte-decl.StartByte != len(decl.Bytes) || decl.NameStartByte < decl.StartByte || decl.NameEndByte > decl.EndByte || decl.NameEndByte <= decl.NameStartByte {
						return fail("AST declaration body/range")
					}
				}
			}
		}
		if sourceBindings != 1 || sourceClaims != 1 {
			return fail("source control cardinality")
		}
		if declarations != 1 {
			return fail("AST declaration cardinality")
		}
	}
	if parts != p.PartCount || payload != p.PartBytes {
		return fail("part totals")
	}
	for key, ix := range indexed {
		if ix.Status == "PREPARED" && !packets[key] {
			return fail("missing packet")
		}
	}
	return nil
}
