package privatedescribeprep

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

// This is a bounded correspondence check over detached observations. In
// particular physical response hashes are declarations, NOT transport witnesses:
// the creator wire does not retain the nonce-bound physical response bytes.
// No postState, native owner, backend, or capability is constructed here.
func observationCohortPrivateV2(ctx context.Context, p Result, inv AutomaticInventoryPrivateV2, ledger StageLedgerPrivateV2, post PostPreparationLimits) error {
	bad := func(s string) error { return fmt.Errorf("cohort observation %s", s) }
	c := inv.Cohort
	if c == nil || c.MaxItems < 1 || c.MaxItems > 63 || len(c.Frames) > 16 || inv.Calls != len(c.Frames) || inv.Bytes < 0 || inv.Bytes > 8<<20 {
		return bad("bounds")
	}
	encoded, _ := json.Marshal(inv)
	human, _ := json.Marshal(renderAutomatic(inv))
	if len(encoded)+len(human) > 4<<20 {
		return bad("output bound")
	}
	frames := 0
	responseBytes, requestBytes := 0, 0
	requestBound := func(r CohortRequestPrivateV2) error {
		b, err := json.Marshal(r)
		requestBytes += len(b)
		if err != nil || len(b) > 1<<20 || requestBytes+responseBytes > 8<<20 {
			return bad("request/total bounds")
		}
		return nil
	}
	next := func(stage string) (CohortFrameEvidencePrivateV2, error) {
		if ctx.Err() != nil {
			return CohortFrameEvidencePrivateV2{}, ctx.Err()
		}
		if frames >= len(c.Frames) {
			return CohortFrameEvidencePrivateV2{}, bad("missing frame")
		}
		f := c.Frames[frames]
		frames++
		if f.Stage != stage || len(f.Response) == 0 || len(f.Response) > 64<<10 || f.ResponseByteLength != len(f.Response) || f.ResponseSHA256 != postPublicationDigest(f.Response) {
			return f, bad("frame response digest/length/stage")
		}
		responseBytes += len(f.Response)
		if responseBytes > 8<<20 {
			return f, bad("response total")
		}
		if (f.PhysicalResponseSHA256 == "") != (f.PhysicalResponseByteLength == 0) {
			return f, bad("physical declaration pair")
		}
		if f.PhysicalResponseSHA256 != "" {
			h := strings.TrimPrefix(f.PhysicalResponseSHA256, "sha256:")
			b, e := hex.DecodeString(h)
			if e != nil || len(b) != 32 || "sha256:"+hex.EncodeToString(b) != f.PhysicalResponseSHA256 || f.PhysicalResponseByteLength < len(f.Response) || f.PhysicalResponseByteLength > 64<<10 {
				return f, bad("physical declaration bounds")
			}
			// Declared wire size is only a conservative accounting lower bound;
			// it is not a recomputed hash or an authenticated wire witness.
			responseBytes += f.PhysicalResponseByteLength - len(f.Response)
		}
		if err := observationUniqueKeysPrivateV2(ctx, f.Response); err != nil {
			return f, bad("response encoding")
		}
		return f, nil
	}
	// Derive own-body input solely from the already validated preparation.
	packets := map[postKey]Packet{}
	for _, pk := range p.Packets {
		packets[postKey{pk.InputSHA256, pk.Target}] = pk
	}
	items := make([]CohortItemPrivateV2, len(inv.Rows))
	own := make([]map[string]string, len(inv.Rows))
	revisions := make([][]AutomaticRevisionPrivateV2, len(inv.Rows))
	var pending []int
	for j, row := range inv.Rows {
		ix := p.Index[j]
		if ix.Status != "PREPARED" {
			if row.Status != ix.Status || len(row.Revisions) != 0 || row.CandidateID != "" {
				return bad("gap row")
			}
			continue
		}
		pk := packets[postKey{row.InputSHA256, row.Target}]
		item := CohortItemPrivateV2{InputSHA256: row.InputSHA256, Target: row.Target, Packet: &pk}
		own[j] = map[string]string{}
		for _, part := range pk.Parts {
			for _, r := range part.Records {
				if r.Kind == "RETAINED_SOURCE_AST_DECLARATION" {
					var d Declaration
					if json.Unmarshal(r.Value, &d) != nil {
						return bad("own declaration")
					}
					if _, ok := own[j][r.Handle]; !ok {
						own[j][r.Handle] = d.Body
						item.Bodies = append(item.Bodies, OwnBodyPrivateV2{r.Handle, d.Body})
					}
				}
			}
		}
		items[j] = item
		if len(item.Bodies) > 0 {
			pending = append(pending, j)
		} else if row.Status != "UNSUPPORTED" || len(row.Revisions) != 0 {
			return bad("unsupported row")
		}
	}
	approved := map[int]bool{}
	for attempt := 0; attempt <= 1 && len(pending) > 0; attempt++ {
		var reviewPending []int
		for _, j := range pending {
			items[j].Attempt = attempt
			items[j].PriorRevisions = postClone(revisions[j])
		}
		for start := 0; start < len(pending); {
			f, e := next("INTERPRET_COHORT")
			if e != nil {
				return e
			}
			var out CohortResponsePrivateV2
			if strictAutomatic(f.Response, 64<<10, &out) != nil || len(out.Items) < 1 || len(out.Items) > c.MaxItems || len(out.Items) > len(pending)-start {
				return bad("interpret frame shape")
			}
			var chunk []CohortItemPrivateV2
			for _, j := range pending[start : start+len(out.Items)] {
				chunk = append(chunk, items[j])
			}
			req := cohortRequest("INTERPRET_COHORT", inv.PreparationSHA256, chunk, post)
			if e = requestBound(req); e != nil {
				return e
			}
			out, e = cohortResponse(f.Response, req, 64<<10)
			if e != nil {
				return bad("interpret bindings")
			}
			var hashes []string
			for k, r := range out.Items {
				j := pending[start+k]
				raw, _ := json.Marshal(r)
				sha := postPublicationDigest(raw)
				rev := AutomaticRevisionPrivateV2{PredecessorDigest: items[j].PredecessorDigest, Response: raw, ResponseSHA256: sha, ResponseByteLength: len(raw), Interpretation: r.Interpretation}
				rev.Digest = automaticDigest(struct{ Previous, RawDigest string }{rev.PredecessorDigest, sha})
				rev.Status = validateExactAutomatic(*r.Interpretation, own[j], post)
				revisions[j] = append(revisions[j], rev)
				if rev.Status == "" {
					items[j].Interpretation = r.Interpretation
					items[j].InterpretationDigest = rev.Digest
					reviewPending = append(reviewPending, j)
				}
				hashes = append(hashes, automaticDigest(r))
			}
			if !reflect.DeepEqual(hashes, f.ItemResponseSHA256) {
				return bad("interpret item hashes")
			}
			start += len(out.Items)
		}
		for start := 0; start < len(reviewPending); {
			f, e := next("REVIEW_TECHNICAL_COHORT")
			if e != nil {
				return e
			}
			var out CohortResponsePrivateV2
			if strictAutomatic(f.Response, 64<<10, &out) != nil || len(out.Items) < 1 || len(out.Items) > c.MaxItems || len(out.Items) > len(reviewPending)-start {
				return bad("review frame shape")
			}
			var chunk []CohortItemPrivateV2
			for _, j := range reviewPending[start : start+len(out.Items)] {
				chunk = append(chunk, items[j])
			}
			req := cohortRequest("REVIEW_TECHNICAL_COHORT", inv.PreparationSHA256, chunk, post)
			if e = requestBound(req); e != nil {
				return e
			}
			out, e = cohortResponse(f.Response, req, 64<<10)
			if e != nil {
				return bad("review bindings")
			}
			var hashes []string
			for k, r := range out.Items {
				j := reviewPending[start+k]
				rev := &revisions[j][len(revisions[j])-1]
				raw, _ := json.Marshal(r.Review)
				v, status := automaticReview(raw, 64<<10, rev.Digest, post.MaxStringBytes)
				rev.Review = &v
				rev.Findings = v.Findings
				rev.Status = status
				if status == "" {
					rev.Status = "APPROVED"
					approved[j] = true
				}
				hashes = append(hashes, automaticDigest(r))
			}
			if !reflect.DeepEqual(hashes, f.ItemResponseSHA256) {
				return bad("review item hashes")
			}
			start += len(out.Items)
		}
		var again []int
		for _, j := range pending {
			rev := revisions[j][len(revisions[j])-1]
			if approved[j] {
				continue
			}
			items[j].PredecessorDigest = rev.Digest
			items[j].Feedback = rev.Status + ": " + rev.Findings
			items[j].Interpretation = nil
			items[j].InterpretationDigest = ""
			if attempt == 0 && (rev.Status == "MALFORMED" || rev.Status == "REJECTED") {
				// MaxCorrections is not in creator wire. A second attempt must actually
				// appear in this row; no correction may be invented by this observer.
				if len(inv.Rows[j].Revisions) > 1 {
					again = append(again, j)
				}
			}
		}
		pending = again
	}
	unresolved := false
	var candidates []AutomaticCandidatePrivateV2
	for j, row := range inv.Rows {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !approved[j] {
			unresolved = true
		}
		if !reflect.DeepEqual(row.Revisions, revisions[j]) {
			return bad("row revisions correspondence")
		}
		if !approved[j] {
			// The cohort creator submits/reviews only approved technical rows.
			// Reconcile leaves all other rows in this exact empty state, even
			// when their automatic revision says ABSTAINED or UNSUPPORTED.
			want := LedgerTarget{PrivateCeilings: privateCeilings(), TargetIndex: p.Index[j], ReviewStatus: "PENDING"}
			if p.Index[j].Status == "PREPARED" {
				want.SubmissionStatus = "PENDING"
			}
			if !reflect.DeepEqual(ledger.Targets[j], want) {
				return bad("unapproved ledger correspondence")
			}
			if len(revisions[j]) > 0 && (row.Status != revisions[j][len(revisions[j])-1].Status || row.Reason != row.Status) {
				return bad("unapproved row status")
			}
			if row.CandidateID != "" || row.Status == "PROVISIONAL" {
				return bad("unapproved candidate")
			}
			continue
		}
		rev := revisions[j][len(revisions[j])-1]
		v := *rev.Interpretation
		id := automaticDigest(struct{ InputSHA256, Target string }{row.InputSHA256, row.Target})
		if row.Status != "PROVISIONAL" || row.Reason != "CONFIGURED_REVIEW_NOT_SEMANTIC_TRUTH" || row.CandidateID != id {
			return bad("approved row")
		}
		candidates = append(candidates, AutomaticCandidatePrivateV2{PrivateCeilings: privateCeilings(), ID: id, InputSHA256: row.InputSHA256, Target: row.Target, RevisionDigest: rev.Digest, Label: v.Label, Description: v.Description, Claims: postClone(v.Claims), Limitations: v.Limitations, Alternatives: v.Alternatives, RejectionCondition: v.RejectionCondition})
		// All legacy lead fields must describe this same approved revision.
		found := false
		claims, e := automaticLegacyClaims(v, post)
		if e != nil {
			return bad("legacy claims")
		}
		for _, lead := range ledger.Leads {
			if lead.Submission.InputSHA256 == row.InputSHA256 && lead.Submission.Target == row.Target {
				s := lead.Submission
				lr := ledger.Targets[j]
				if lr.Submission == nil || lr.Review == nil || lr.SubmissionStatus != "SUBMITTED" || lr.ReviewStatus != "RECORDED_SUPPORT_WITH_LIMITS" || !reflect.DeepEqual(*lr.Submission, s) || !reflect.DeepEqual(*lr.Review, lead.Review) || lead.Status != "PROVISIONAL" || s.Disposition != "DESCRIBED" || lead.Review.Verdict != "SUPPORT_WITH_LIMITS" || lead.Review.InputSHA256 != row.InputSHA256 || lead.Review.Target != row.Target {
					return bad("lead ledger correspondence")
				}
				if found || s.WorkingLabel != v.Label || s.TechnicalDescription != v.Description || s.Limitations != v.Limitations || s.Alternatives != v.Alternatives || s.RejectionCondition != v.RejectionCondition || !reflect.DeepEqual(s.Claims, claims) || lead.Review.Findings != rev.Findings {
					return bad("lead approved correspondence")
				}
				found = true
			}
		}
		if !found {
			return bad("missing approved lead")
		}
	}
	if !reflect.DeepEqual(candidates, inv.Candidates) {
		return bad("candidate revision correspondence")
	}
	previous := ""
	var navigation []NavigationPrivateV2
	if len(candidates) == 0 && len(inv.NavigationRevisions) != 0 {
		return bad("navigation without candidates")
	}
	if len(candidates) > 0 && (len(inv.NavigationRevisions) < 1 || len(inv.NavigationRevisions) > 2) {
		return bad("navigation revisions bounds")
	}
	navApproved, navAbstained := false, false
	feedback := ""
	for attempt, nr := range inv.NavigationRevisions {
		if navAbstained {
			return bad("navigation after abstention")
		}
		nav := AutomaticRequestPrivateV2{Protocol: CohortProtocolPrivateV2, Stage: "NAVIGATE", PreparationSHA256: inv.PreparationSHA256, Candidates: postClone(inv.Candidates), Attempt: attempt, PredecessorDigest: previous, Feedback: feedback, PostLimits: post, LedgerQuoteChunkBytes: automaticLedgerQuoteChunkBytes, PriorNavigationRevisions: postClone(inv.NavigationRevisions[:attempt])}
		// A nil prefix is what the creator emits on the first attempt.
		if attempt == 0 {
			nav.PriorNavigationRevisions = nil
		}
		req := cohortRequest("NAVIGATE", inv.PreparationSHA256, nil, post)
		req.Navigation = &nav
		if e := requestBound(req); e != nil {
			return e
		}
		if navApproved {
			return bad("navigation after approval")
		}
		f, e := next("NAVIGATE")
		if e != nil {
			return e
		}
		if len(f.ItemResponseSHA256) != 0 || !reflect.DeepEqual(f.Response, nr.Response) || nr.ResponseSHA256 != f.ResponseSHA256 || nr.ResponseByteLength != f.ResponseByteLength || nr.PredecessorDigest != previous || nr.Digest != automaticDigest(struct{ Previous, RawDigest string }{previous, f.ResponseSHA256}) {
			return bad("navigation response correspondence")
		}
		var groups []NavigationPrivateV2
		status := "MALFORMED"
		if strictAutomatic(f.Response, 64<<10, &groups) == nil {
			status = validateNavigation(groups, candidates)
		}
		if !reflect.DeepEqual(groups, nr.Hypotheses) {
			return bad("navigation hypotheses")
		}
		if status == "" {
			nav.Stage = "REVIEW_NAVIGATION"
			nav.Navigation = groups
			nav.RevisionDigest = nr.Digest
			req.Stage = nav.Stage
			if e := requestBound(req); e != nil {
				return e
			}
			rf, e := next("REVIEW_NAVIGATION")
			if e != nil {
				return e
			}
			v, st := automaticReview(rf.Response, 64<<10, nr.Digest, post.MaxStringBytes)
			if len(rf.ItemResponseSHA256) != 0 || !reflect.DeepEqual(nr.Review, &v) || nr.Findings != v.Findings {
				return bad("navigation review binding")
			}
			status = st
			if status == "" {
				status = "APPROVED"
				navigation = groups
				navApproved = true
			}
		} else if nr.Review != nil {
			return bad("unexpected navigation review")
		}
		if nr.Status != status {
			return bad("navigation status")
		}
		navAbstained = status == "ABSTAINED"
		previous = nr.Digest
		feedback = nr.Status + ": " + nr.Findings
	}
	if len(candidates) > 0 && !navApproved {
		unresolved = true
	}
	wantStatus := "PROVISIONAL_COMPLETE"
	if unresolved {
		wantStatus = "PROVISIONAL_WITH_UNRESOLVED"
	}
	if inv.Status != wantStatus {
		return bad("inventory completion status")
	}
	if !reflect.DeepEqual(navigation, inv.Navigation) || frames != len(c.Frames) || responseBytes+requestBytes > inv.Bytes {
		return bad("navigation/frame accounting")
	}
	return ctx.Err()
}
