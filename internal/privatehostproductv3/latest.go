//go:build darwin || linux

package privatehostproductv3

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"syscall"

	pilot "lsp-trace/internal/censuslineagepilot"
)

// A quartet is local consistency evidence, not authentication of its producer.
// Names come from the closed namespace, never from the decoded report. Each
// input is bounded before decoding and the concrete field graph before marshal.
func decodeReport(raw []byte, r *Report) bool {
	if len(raw) > stateLimit {
		return false
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(r) != nil || d.Decode(new(any)) != io.EOF || !admitReport(*r) {
		return false
	}
	return bytes.Equal(raw, mustJSON(r))
}
func reportPaths(n int) ([2]string, [2]string) {
	return [2]string{fmt.Sprintf("result/report-%08d.json", n), fmt.Sprintf("result/report-%08d.json", n+1)}, [2]string{fmt.Sprintf("result/inventory-%08d.md", n), fmt.Sprintf("result/inventory-%08d.md", n+1)}
}
func (s *store) resultNumbers() ([]int, error) {
	d, e := s.root.Open("result")
	if e != nil {
		return nil, e
	}
	defer d.Close()
	// Directory metadata is itself one bounded unit, not an arbitrary total.
	names, e := d.Readdirnames(stateLimit/128 + 1)
	if e != nil && e != io.EOF || len(names) > stateLimit/128 {
		return nil, ErrInvalid
	}
	seen := map[int]bool{}
	for _, name := range names {
		for _, kind := range []struct{ k, x string }{{"report", "json"}, {"inventory", "md"}} {
			var n int
			if _, e := fmt.Sscanf(name, kind.k+"-%08d."+kind.x, &n); e == nil && n > 0 && n <= 99999999 && name == fmt.Sprintf(kind.k+"-%08d."+kind.x, n) {
				seen[((n-1)/2)*2+1] = true
			}
		}
	}
	out := make([]int, 0, len(seen))
	for n := range seen {
		out = append(out, n)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(out)))
	return out, nil
}
func (s *store) latestReport(c Config) (Report, error) {
	s.rejectedPrior = false
	ns, e := s.resultNumbers()
	if e != nil {
		s.rejectedPrior = true
		return Report{}, e
	}
	work := 100000
	for _, n := range ns {
		r, e := s.admitPublication(c, n, &work, 0)
		if e == nil {
			return r, nil
		}
		s.rejectedPrior = true
	}
	return Report{}, ErrInvalid
}
func (s *store) admitPublication(c Config, n int, work *int, depth int) (Report, error) {
	var r Report
	*work -= 1
	if n < 1 || n%2 == 0 || *work < 0 || depth > 64 {
		return r, ErrInvalid
	}
	mp, hp := reportPaths(n)
	raw, e := s.read(mp[0], stateLimit)
	if e != nil || !decodeReport(raw, &r) || r.MachinePaths != mp || r.HumanPaths != hp || r.ReportRevision != (n+1)/2 {
		return Report{}, ErrInvalid
	}
	// Read all four exact views through no-follow/private/single-link checks.
	paths := []string{mp[0], mp[1], hp[0], hp[1]}
	infos := make([]os.FileInfo, 0, 4)
	expectedHuman := human(r)
	if len(expectedHuman) > stateLimit {
		return Report{}, ErrInvalid
	}
	for i, p := range paths {
		b, e := s.read(p, stateLimit)
		if e != nil {
			return Report{}, ErrInvalid
		}
		want := raw
		if i > 1 {
			want = expectedHuman
		}
		if !bytes.Equal(b, want) {
			return Report{}, ErrInvalid
		}
		st, e := s.root.Lstat(p)
		if e != nil || !st.Mode().IsRegular() || st.Mode().Perm() != 0600 || st.Sys().(*syscall.Stat_t).Nlink != 1 {
			return Report{}, ErrInvalid
		}
		for _, old := range infos {
			if os.SameFile(old, st) {
				return Report{}, ErrInvalid
			}
		}
		infos = append(infos, st)
	}
	cfg, ok := s.reportConfig(c, r.ConfigSHA256, work)
	if !ok || !validReportValues(r, cfg) {
		return Report{}, ErrInvalid
	}
	if a := r.Admission; a != nil {
		empty := [2]string{}
		if a.PriorMachinePaths == empty && a.PriorHumanPaths == empty {
			if r.Completed != 0 || r.Gaps != 0 || r.Spend.Calls != 0 || r.Spend.RequestBytes != 0 || r.Spend.ResponseBytes != 0 || r.CheckpointSHA256 != "" || r.LedgerTip != "" || r.LedgerGeneration != 0 || r.NextBatch != 0 || r.Resume != "" || len(r.NavigationHypotheses) != 0 || r.Lifecycle != (Lifecycle{}) || r.Spend.PolicyReservation != (pilot.PageCostPrivateV3{}) || !bytes.Equal(mustJSON(r.Spend.Excluded), mustJSON([]string{"acquisition/session/bootstrap/inference accounting not represented"})) || !bytes.Equal(mustJSON(r.FirstFailure), mustJSON(r.Failure)) || a.Coverage != "UNKNOWN" || a.Counts != "UNKNOWN" || a.LedgerObservation != "UNKNOWN" {
				return Report{}, ErrInvalid
			}
		} else {
			var prev int
			if _, e := fmt.Sscanf(a.PriorMachinePaths[0], "result/report-%08d.json", &prev); e != nil || prev >= n {
				return Report{}, ErrInvalid
			}
			prior, e := s.admitPublication(c, prev, work, depth+1)
			if e != nil || a.PriorMachinePaths != prior.MachinePaths || a.PriorHumanPaths != prior.HumanPaths || a.Coverage != "PREVIOUS_REPORT_OBSERVATION" || a.Counts != "PREVIOUS_REPORT_OBSERVATION" || a.LedgerObservation != "PREVIOUS_REPORT_OBSERVATION_NOT_RECOMPUTED" {
				return Report{}, ErrInvalid
			}
			expectedFirst := prior.FirstFailure
			if expectedFirst == nil {
				expectedFirst = prior.Failure
			}
			if expectedFirst == nil {
				expectedFirst = r.Failure
			}
			if !bytes.Equal(mustJSON(expectedFirst), mustJSON(r.FirstFailure)) {
				return Report{}, ErrInvalid
			}
			// Only these terminal-delivery fields may change on refused admission.
			carried := r
			carried.Status = prior.Status
			carried.ConfigSHA256 = prior.ConfigSHA256
			carried.Failure = prior.Failure
			carried.FirstFailure = prior.FirstFailure
			carried.Admission = prior.Admission
			carried.MachinePaths = prior.MachinePaths
			carried.HumanPaths = prior.HumanPaths
			carried.ReportRevision = prior.ReportRevision
			if !bytes.Equal(mustJSON(carried), mustJSON(prior)) {
				return Report{}, ErrInvalid
			}
		}
	} else if !s.reportProgress(r, cfg, work) {
		return Report{}, ErrInvalid
	}
	return r, nil
}
func samePolicy(a, b Config) bool {
	// The existing explicit allowance continuation changes only these fields.
	for _, c := range []*Config{&a, &b} {
		c.ApprovalSHA256 = ""
		c.PreviousConfigSHA256 = ""
		c.AllowanceRevision = 0
		c.StartBatch = 0
		c.MaxCalls = 0
		c.MaxTotalBytes = 0
	}
	return bytes.Equal(mustJSON(a), mustJSON(b))
}
func (s *store) reportConfig(c Config, digest string, work *int) (Config, bool) {
	if !admitProgress(progress{Config: c}) {
		return Config{}, false
	}
	if digest == ConfigDigest(c) {
		raw, err := s.read(fmt.Sprintf("plan/config-%08d.json", c.AllowanceRevision), stateLimit)
		return c, ValidateConfig(c) == nil && (os.IsNotExist(err) || err == nil && bytes.Equal(raw, mustJSON(c)))
	}
	// Read exact numbered grants; no source, provider, or ledger reopening here.
	for rev := c.AllowanceRevision; rev > 0 && *work > 0; rev-- {
		*work -= 1
		raw, e := s.read(fmt.Sprintf("plan/config-%08d.json", rev), stateLimit)
		if e != nil {
			continue
		}
		var old Config
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.DisallowUnknownFields()
		if dec.Decode(&old) == nil && admitProgress(progress{Config: old}) && bytes.Equal(raw, mustJSON(old)) && ValidateConfig(old) == nil && old.AllowanceRevision == rev && samePolicy(old, c) && ConfigDigest(old) == digest && old.MaxCalls <= c.MaxCalls && old.MaxTotalBytes <= c.MaxTotalBytes {
			return old, s.configAncestor(c, old, work)
		}
	}
	return Config{}, false
}

// Historical allowance links are immutable observations, not fresh permission.
func (s *store) configAncestor(current, old Config, work *int) bool {
	for current.AllowanceRevision > old.AllowanceRevision {
		*work -= 1
		if *work < 0 {
			return false
		}
		next := old
		if current.AllowanceRevision != old.AllowanceRevision+1 {
			raw, err := s.read(fmt.Sprintf("plan/config-%08d.json", current.AllowanceRevision-1), stateLimit)
			if err != nil {
				return false
			}
			d := json.NewDecoder(bytes.NewReader(raw))
			d.DisallowUnknownFields()
			if d.Decode(&next) != nil || !admitProgress(progress{Config: next}) || !bytes.Equal(raw, mustJSON(next)) || ValidateConfig(next) != nil {
				return false
			}
		}
		if next.AllowanceRevision+1 != current.AllowanceRevision || current.PreviousConfigSHA256 != ConfigDigest(next) || !samePolicy(current, next) || current.MaxCalls < next.MaxCalls || current.MaxTotalBytes < next.MaxTotalBytes {
			return false
		}
		current = next
	}
	return current.AllowanceRevision == old.AllowanceRevision && ConfigDigest(current) == ConfigDigest(old)
}
func digestValue(v string) bool {
	if len(v) != 71 || !strings.HasPrefix(v, "sha256:") {
		return false
	}
	for _, c := range v[7:] {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}
func validReportValues(r Report, c Config) bool {
	if r.Schema != "private-host-product-report.v3.1" || r.Profile != Profile || r.Application != c.Application || r.Authority != 0 || r.Accepted || r.FeatureIdentity != "UNRESOLVED" || r.SourceCompleteness != "UNKNOWN" || r.AbsenceInference != "PROHIBITED" || r.Requested != c.Requested || r.Completed < 0 || r.Completed > r.Requested || r.Gaps < 0 || r.Gaps > r.Requested-r.Completed || r.Pending != r.Requested-r.Completed-r.Gaps || r.Completed != len(r.Candidates) || r.Gaps != len(r.TargetGaps) || len(r.Unassigned) != r.Completed || len(r.NavigationHypotheses) != 0 {
		return false
	}
	switch r.Status {
	case "PROVISIONAL":
		if r.Failure != nil {
			return false
		}
	case "FAILED", "CANCELLED", "PAUSED_BUDGET":
		if r.Failure == nil {
			return false
		}
	default:
		return false
	}
	if r.Spend.Calls < 2*r.Completed || r.Spend.Calls > c.MaxCalls || r.Spend.RequestBytes < 0 || r.Spend.ResponseBytes < 0 || r.Spend.RequestBytes > c.MaxTotalBytes || r.Spend.ResponseBytes > c.MaxTotalBytes-r.Spend.RequestBytes || r.NextBatch < 0 || r.NextBatch > r.Requested {
		return false
	}
	if r.Lifecycle.OriginalStopCalls < 0 || r.Lifecycle.OriginalStopCalls > 1 || r.Lifecycle.OriginalJoinCalls < 0 || r.Lifecycle.OriginalJoinCalls > 1 || r.Lifecycle.OuterOriginalProcessWaited && r.Lifecycle.OriginalJoinCalls != 1 || r.Lifecycle.InnerGraceful && r.Lifecycle.OriginalStopCalls != 1 {
		return false
	}
	switch r.Lifecycle.RoleCleanup {
	case "", "UNOBSERVED_HOST_TOOL_CONTRACT", "UNOBSERVED_HOST_ROLE_RETURN":
	default:
		return false
	}
	for _, f := range []*Failure{r.Failure, r.FirstFailure} {
		if f != nil && (f.Phase == "" || f.Class == "" || f.Resource == "" || f.ExactOriginalError != "UNKNOWN") {
			return false
		}
	}
	if r.Admission != nil {
		a := r.Admission
		if a.UnadmittedPublications && a.PriorPublicationValidation != "UNKNOWN" {
			return false
		}
		if r.Status != "FAILED" || a.CurrentValidation != "FAILED" || a.SourceValidation != "UNKNOWN" || a.LedgerValidation != "UNKNOWN" || a.ResourceCounts != "UNKNOWN" || a.PriorPublicationValidation != "" && a.PriorPublicationValidation != "UNKNOWN" && a.PriorPublicationValidation != "LOCAL_CONSISTENCY_ONLY" {
			return false
		}
	} else if r.Failure != nil && r.Failure.Class != r.Status && !(r.Status == "FAILED" && r.Failure.Class == ErrItemEvidence.Error()) {
		return false
	} else if !bytes.Equal(mustJSON(r.Spend.Excluded), mustJSON([]string{"original source enumeration/acquisition/session", "caller bootstrap verification budget (separate)", "model inference billing/tokens/host orchestration", "application-store/report IO not fully reserved"})) {
		return false
	} else if !bytes.Equal(mustJSON(r.FirstFailure), mustJSON(r.Failure)) {
		return false
	} else if !digestValue(r.CheckpointSHA256) || !digestValue(r.LedgerTip) || r.LedgerGeneration == 0 || r.Resume != fmt.Sprintf("%s:%d:%s", r.Application, r.Completed, r.CheckpointSHA256) {
		return false
	}
	seen := map[string]bool{}
	for i, v := range r.Candidates {
		if v.ID != "candidate-"+pilot.Digest([]byte(v.InputID))[7:23] || seen[v.ID] || v.InputID == "" || v.Target == "" || v.Status != "PROVISIONAL" || v.Revision != 1 || len(v.Predecessors) != 0 || v.InventoryStateDelta != "ADD_PROVISIONAL_CANDIDATE" || v.WorkingLabel == "" || v.Explanation == "" || v.RejectionCondition == "" || v.Quote == "" || v.QuoteStart < 0 || v.QuoteEnd < v.QuoteStart || v.QuoteEnd-v.QuoteStart != len(v.Quote) || len(v.Alternatives) != 1 || len(v.Gaps) != 1 || len(v.EvidenceRefs) != 9 || len(v.Evidence) != 9 || r.Unassigned[i] != v.ID {
			return false
		}
		seen[v.ID] = true
		families := []string{"V3_RETAINED_PREPARED_OBSERVATION", "V3_RETAINED_ASSESSMENT", "V3_DERIVED_CANONICAL", "SOURCE_SPECIFIC_NODE_ID", "OWN_AST_BODY", "ORIGINAL_RAW_BATCH_SEEDS", "ORIGINAL_RAW_CAPTURE_ORIGIN", "ORIGINAL_RAW_SOURCE_ARTIFACT", "ORIGINAL_RAW_MANIFEST_INPUT"}
		for j, ref := range v.Evidence {
			if j != 3 && !digestValue(ref.Ref) {
				return false
			}
			if ref.Family != families[j] || ref.Ref != v.EvidenceRefs[j] || ref.InputID != v.InputID || ref.Ref == "" {
				return false
			}
		}
	}
	if len(r.TargetGaps) > 1 {
		return false
	}
	for _, g := range r.TargetGaps {
		if g.Target == "" || r.Status != "FAILED" || r.Failure == nil || g.Phase != "HOST_SEMANTIC_EXCHANGE" || g.Phase != r.Failure.Phase || g.Class != r.Failure.Class {
			return false
		}
	}
	return true
}
func (s *store) reportProgress(r Report, c Config, work *int) bool {
	grant, err := s.read(fmt.Sprintf("plan/config-%08d.json", c.AllowanceRevision), stateLimit)
	if err != nil || !bytes.Equal(grant, mustJSON(c)) {
		return false
	}
	d, e := s.root.Open("pages")
	if e != nil {
		return false
	}
	defer d.Close()
	names, e := d.Readdirnames(stateLimit/128 + 1)
	if e != nil && e != io.EOF || len(names) > stateLimit/128 {
		return false
	}
	sort.Sort(sort.Reverse(sort.StringSlice(names)))
	for _, name := range names {
		*work -= 1
		if *work < 0 {
			return false
		}
		raw, e := s.read("pages/"+name, stateLimit)
		if e != nil {
			continue
		}
		var p progress
		// Bounded decode precedes the bounded concrete field graph and marshal.
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.DisallowUnknownFields()
		if dec.Decode(&p) != nil || !admitProgress(p) || !bytes.Equal(raw, mustJSON(p)) || ConfigDigest(p.Config) != ConfigDigest(c) || p.LastStatus != r.Status || p.Calls != r.Spend.Calls || p.RequestBytes != r.Spend.RequestBytes || p.ResponseBytes != r.Spend.ResponseBytes || !bytes.Equal(mustJSON(p.Candidates), mustJSON(r.Candidates)) {
			continue
		}
		var m pilot.RetainedPageCheckpointManifestPrivateV3
		if strict(p.LastCheckpoint, &m) != nil || pilot.Digest(p.LastCheckpoint) != r.CheckpointSHA256 || m.Application != r.Application || m.ScopeSHA256 != c.ScopeSHA256 || m.Revision.Policy.ConfigSHA256 != c.ProducerConfigSHA256 || m.NextBatch != r.NextBatch || m.LedgerGeneration > r.LedgerGeneration || m.LedgerGeneration == r.LedgerGeneration && m.LedgerTip != r.LedgerTip {
			continue
		}
		if m.Schema != "lsp-trace.retained-page-checkpoint.private.v3.1" || m.Profile != pilot.RetainedCheckpointProfilePrivateV3 || m.PreparationRevision != pilot.RetainedCheckpointPreparationRevisionPrivateV3 || m.Authority != 0 || m.Accepted || m.FeatureIdentity != "UNRESOLVED" || m.SourceCompleteness != "UNKNOWN" || m.AbsenceInference != "PROHIBITED" || m.Targets != uint64(c.Requested) || !digestValue(m.LedgerTip) {
			continue
		}
		consistentSpend := true
		for _, costs := range [][3]uint64{
			{m.Charged.LogicalBytes, r.Spend.PolicyReservation.LogicalBytes, m.Allowance.LogicalBytes},
			{m.Charged.ReadHashBytes, r.Spend.PolicyReservation.ReadHashBytes, m.Allowance.ReadHashBytes},
			{m.Charged.WriteBytes, r.Spend.PolicyReservation.WriteBytes, m.Allowance.WriteBytes},
			{m.Charged.Work, r.Spend.PolicyReservation.Work, m.Allowance.Work},
		} {
			if costs[1] < costs[0] || costs[1] > costs[2] {
				consistentSpend = false
			}
		}
		if !consistentSpend {
			continue
		}
		if len(r.TargetGaps) != 0 {
			nextTarget := ""
			seen := map[string]bool{}
			for _, v := range p.Candidates {
				seen[v.InputID] = true
			}
			for _, o := range p.Observations {
				for _, b := range o.Bodies {
					if nextTarget == "" && !seen[o.InputID+"/"+b.Target] {
						nextTarget = b.Target
					}
				}
			}
			if nextTarget != r.TargetGaps[0].Target {
				continue
			}
		}
		// Candidate labels/citations must also agree with the immutable own-body
		// observation and the physically spent interpreter/reviewer exchange.
		if s.candidateObservations(p, c, work) {
			return true
		}
	}
	return false
}
func (s *store) candidateObservations(p progress, c Config, work *int) bool {
	for ci, v := range p.Candidates {
		found := false
		var own ownBody
		var obs preparedObservation
		for _, o := range p.Observations {
			for _, b := range o.Bodies {
				if o.InputID+"/"+b.Target != v.InputID {
					continue
				}
				if b.Target != v.Target || v.QuoteStart < 0 || v.QuoteEnd < v.QuoteStart || v.QuoteEnd > len(b.Body) || b.Body[v.QuoteStart:v.QuoteEnd] != v.Quote || b.OwnBodySHA256 != pilot.Digest([]byte(b.Body)) || v.EvidenceRefs[0] != o.References[2] || v.EvidenceRefs[1] != o.References[1] || v.EvidenceRefs[2] != o.References[0] || v.EvidenceRefs[3] != b.NodeID || v.EvidenceRefs[4] != b.OwnBodySHA256 {
					return false
				}
				for i, ref := range o.RawReferences {
					if v.EvidenceRefs[i+5] != ref {
						return false
					}
				}
				if found {
					return false
				}
				own, obs = b, o
				found = true
			}
		}
		if !found {
			return false
		}
		var proposal Response
		for roleIndex, role := range []string{"interpreter", "reviewer"} {
			seq := ci*2 + roleIndex + 1
			*work -= 1
			if seq > p.Calls || *work < 0 {
				return false
			}
			raw, e := s.read(fmt.Sprintf("claim/claim-%08d.json", seq), min(stateLimit, c.MaxRequestBytes))
			if e != nil {
				return false
			}
			var t Task
			d := json.NewDecoder(bytes.NewReader(raw))
			d.DisallowUnknownFields()
			if d.Decode(&t) != nil || !admitTask(t, c.MaxRequestBytes) || !bytes.Equal(raw, mustJSON(t)) {
				return false
			}
			grant, ok := s.reportConfig(c, t.ConfigSHA256, work)
			if !ok {
				return false
			}
			digest := t.RequestDigest
			t.RequestDigest = ""
			if digest != pilot.Digest(mustJSON(t)) || t.Protocol != Protocol || t.Role != role || t.Sequence != uint64(seq) || t.ScopeSHA256 != c.ScopeSHA256 || t.Application != c.Application || t.InputID != v.InputID || t.Target != v.Target || t.Revision != 1 || t.Predecessor != "" || t.Attempt != fmt.Sprintf("grant-%d/batch-%d/item-%d", grant.AllowanceRevision, obs.Batch, ci) || t.CheckpointSHA256 != obs.Checkpoint || t.PreparedSHA256 != obs.References[2] || t.OriginalRawReferences != obs.RawReferences || t.Body != own.Body || t.OwnBodySHA256 != own.OwnBodySHA256 || t.InputFamily != pilot.RetainedCheckpointProfilePrivateV3 || t.MaxRequestBytes != grant.MaxRequestBytes || t.MaxResponseBytes != grant.MaxResponseBytes {
				return false
			}
			if roleIndex == 0 && t.Proposal != nil || roleIndex == 1 && (t.Proposal == nil || !bytes.Equal(mustJSON(t.Proposal), mustJSON(proposal))) {
				return false
			}
			reply, e := s.read(fmt.Sprintf("host/response-%08d.json", seq), min(64<<10, grant.MaxResponseBytes))
			if e != nil {
				return false
			}
			var a Response
			d = json.NewDecoder(bytes.NewReader(reply))
			d.DisallowUnknownFields()
			if d.Decode(&a) != nil || !admitResponse(a, grant.MaxResponseBytes) || !bytes.Equal(reply, mustJSON(a)) || a.Protocol != Protocol || a.RequestDigest != digest || a.Start < 0 || a.End < a.Start || a.End > len(own.Body) || own.Body[a.Start:a.End] != a.Quote || a.Quote == "" || a.Label == "" || a.Explanation == "" || a.Alternative == "" || a.RejectionCondition == "" {
				return false
			}
			if roleIndex == 1 && (!a.Supported || a.Label != v.WorkingLabel || a.Explanation != v.Explanation || a.Alternative != v.Alternatives[0] || a.RejectionCondition != v.RejectionCondition || a.Quote != v.Quote || a.Start != v.QuoteStart || a.End != v.QuoteEnd) {
				return false
			}
			proposal = a
		}
	}
	return true
}
