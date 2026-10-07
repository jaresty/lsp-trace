package adr0007locationv1

import (
	"encoding/json"
	"path"
	"sort"
	"strings"
	"unicode/utf8"
)

func less(a, b Position) bool {
	return a.Line < b.Line || a.Line == b.Line && a.Character < b.Character
}
func valid(r Range) bool { return less(r.Start, r.End) }
func overlap(a, b Range) (Range, bool) {
	s := a.Start
	if less(s, b.Start) {
		s = b.Start
	}
	e := a.End
	if less(b.End, e) {
		e = b.End
	}
	return Range{s, e}, less(s, e)
}
func contains(a, b Range) bool { return !less(b.Start, a.Start) && !less(a.End, b.End) }
func canon(p string) bool {
	return p != "" && utf8.ValidString(p) && !strings.Contains(p, "\\") && !strings.Contains(p, "//") && !strings.Contains(p, ":") && !strings.HasPrefix(p, "/") && path.Clean(p) == p && !strings.HasPrefix(p, "../") && !strings.Contains(p, "/../")
}
func terminal(req Request, o OperationOutcome, d string) Result {
	return Result{Schema: Schema, RequestID: req.ID, Outcome: o, Detail: d}
}
func Evaluate(req Request, adm *Admission, opt Options) Result {
	l := opt.Limits
	if req.ID == "" || req.TopK < 0 || l.MaxRequestBytes < 1 || l.MaxMembers < 1 || l.MaxPaths < 1 || l.MaxRanges < 1 || l.MaxPrefixExpansion < 1 || l.MaxWitnesses < 1 || l.MaxSourceBytes < 1 || l.MaxOutputBytes < 1 || l.MaxWork < 1 {
		return terminal(req, InvalidRequest, "request or limits")
	}
	raw, _ := json.Marshal(req)
	if len(raw) > l.MaxRequestBytes {
		return terminal(req, ResourceLimit, "request bytes")
	}
	if adm == nil {
		return terminal(req, SourceAdmissionUnavailable, "admission")
	}
	if adm.AdmissionDigest != req.AdmissionDigest {
		return terminal(req, SourceAdmissionMismatch, "admission digest")
	}
	if opt.ExpectedPolicyDigest != "" && req.PolicyDigest != opt.ExpectedPolicyDigest {
		return terminal(req, PolicyMismatch, "policy digest")
	}
	if req.Relation != Intersects && req.Relation != ContainedBy && req.Relation != Contains {
		return terminal(req, InvalidRequest, "relation")
	}
	if req.Selector.Kind != ExactFile && req.Selector.Kind != RangeUnion && req.Selector.Kind != PathPrefix {
		return terminal(req, InvalidSelector, "kind")
	}
	if len(req.Members) > l.MaxMembers {
		return terminal(req, ResourceLimit, "members")
	}
	paths := map[string]bool{}
	ranges := 0
	switch req.Selector.Kind {
	case ExactFile:
		if !canon(req.Selector.Path) {
			return terminal(req, InvalidSelector, "path")
		}
		paths[req.Selector.Path] = true
	case PathPrefix:
		if !canon(req.Selector.Path) || len(req.Selector.FrozenPaths) == 0 || len(req.Selector.FrozenPaths) > l.MaxPrefixExpansion {
			return terminal(req, InvalidSelector, "prefix expansion")
		}
		for _, p := range req.Selector.FrozenPaths {
			if !canon(p) || !(p == req.Selector.Path || strings.HasPrefix(p, req.Selector.Path+"/")) {
				return terminal(req, InvalidSelector, "frozen path")
			}
			paths[p] = true
		}
	case RangeUnion:
		if len(req.Selector.Union) == 0 {
			return terminal(req, InvalidSelector, "empty union")
		}
		for _, u := range req.Selector.Union {
			if !canon(u.Path) || len(u.Ranges) == 0 {
				return terminal(req, InvalidSelector, "union")
			}
			paths[u.Path] = true
			for _, r := range u.Ranges {
				if !valid(r) {
					return terminal(req, InvalidRange, "selector range")
				}
				ranges++
			}
		}
	}
	if len(paths) > l.MaxPaths || ranges > l.MaxRanges {
		return terminal(req, ResourceLimit, "selector size")
	}
	work := len(req.Members) + len(paths) + ranges
	for _, m := range req.Members {
		work += len(m.Ranges)
	}
	if work > l.MaxWork {
		return terminal(req, ResourceLimit, "work")
	}
	if opt.Control != nil && opt.Control.Cancelled() {
		return terminal(req, Cancelled, "before commit")
	}
	if opt.Control != nil && opt.Control.DeadlineExceeded() {
		return terminal(req, Timeout, "before commit")
	}
	source := map[string]sourceRef{}
	total := 0
	for _, s := range adm.Sources {
		total += len(s.Bytes)
		source[s.Path] = sourceRef{s.Revision, s.FileDigest, s.ObjectDigest}
	}
	if total > l.MaxSourceBytes {
		return terminal(req, ResourceLimit, "source bytes")
	}
	out := make([]MemberOutcome, len(req.Members))
	seen := map[string]bool{}
	wcount := 0
	for i, m := range req.Members {
		o := MemberOutcome{Ordinal: i, MemberID: m.ID}
		if seen[m.ID] || m.ID == "" {
			o.Outcome = DuplicateMember
		} else if !m.PolicyAllowed {
			o.Outcome = FilteredByPolicy
		} else if !m.Available {
			o.Outcome = UnavailableLocation
		} else if !canon(m.Path) || len(m.Ranges) == 0 {
			o.Outcome = InvalidLocation
		} else if s, ok := source[m.Path]; !ok || s.r != m.Revision || s.f != m.FileDigest || s.o != m.ObjectDigest {
			o.Outcome = InvalidLocation
		} else {
			bad := false
			for _, r := range m.Ranges {
				if !valid(r) {
					bad = true
				}
			}
			if bad {
				o.Outcome = InvalidLocation
			} else {
				o.Witnesses = match(req.Selector, req.Relation, m)
				if len(o.Witnesses) > 0 {
					o.Outcome = Eligible
				} else {
					o.Outcome = Ineligible
				}
			}
		}
		seen[m.ID] = true
		wcount += len(o.Witnesses)
		if wcount > l.MaxWitnesses {
			return terminal(req, ResourceLimit, "witnesses")
		}
		out[i] = o
	}
	ranked := []RankedResult{}
	for i, o := range out {
		if o.Outcome == Eligible {
			ranked = append(ranked, RankedResult{i, o.MemberID, req.Members[i].Score})
		}
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].Score == ranked[j].Score {
			return ranked[i].Ordinal < ranked[j].Ordinal
		}
		return ranked[i].Score > ranked[j].Score
	})
	if req.TopK < len(ranked) {
		ranked = ranked[:req.TopK]
	}
	a := Accounting{Input: len(out), Ranked: len(ranked), Witnesses: wcount, Work: work}
	for _, o := range out {
		switch o.Outcome {
		case Eligible:
			a.Eligible++
		case Ineligible:
			a.Ineligible++
		case UnavailableLocation:
			a.Unavailable++
		case InvalidLocation:
			a.Invalid++
		case DuplicateMember:
			a.Duplicate++
		case FilteredByPolicy:
			a.Filtered++
		}
	}
	res := Result{Schema: Schema, RequestID: req.ID, Outcome: Complete, Members: out, Ranked: ranked, Accounting: a}
	b, _ := json.Marshal(res)
	if len(b) > l.MaxOutputBytes {
		return terminal(req, ResourceLimit, "output bytes")
	}
	if opt.Control != nil && (opt.Control.Cancelled() || opt.Control.DeadlineExceeded()) {
		if opt.Control.Cancelled() {
			return terminal(req, Cancelled, "before commit")
		}
		return terminal(req, Timeout, "before commit")
	}
	return res
}

type sourceRef struct{ r, f, o string }

func match(s Selector, rel Relation, m Member) []Witness {
	sel := []PathRanges{}
	switch s.Kind {
	case ExactFile:
		sel = []PathRanges{{Path: s.Path, Ranges: []Range{{Position{0, 0}, Position{^uint32(0), ^uint32(0)}}}}}
	case PathPrefix:
		for _, p := range s.FrozenPaths {
			sel = append(sel, PathRanges{Path: p, Ranges: []Range{{Position{0, 0}, Position{^uint32(0), ^uint32(0)}}}})
		}
	case RangeUnion:
		sel = s.Union
	}
	var w []Witness
	for _, u := range sel {
		if u.Path != m.Path {
			continue
		}
		for _, sr := range u.Ranges {
			for _, cr := range m.Ranges {
				in, ok := overlap(sr, cr)
				pass := ok
				if rel == ContainedBy {
					pass = contains(sr, cr)
				} else if rel == Contains {
					pass = contains(cr, sr)
				}
				if pass {
					w = append(w, Witness{m.Path, sr, cr, in, m.Revision, m.FileDigest, m.ObjectDigest})
				}
			}
		}
	}
	return w
}
