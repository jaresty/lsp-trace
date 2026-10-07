package adr0007locationv2

import (
	"encoding/json"
	"lsp-trace/internal/sourceadmissionv2"
	"math"
	"sort"
	"strings"
)

func terminal(q Request, o OperationOutcome, d string) Result {
	return Result{Schema: Schema, RequestID: q.ID, Outcome: o, Members: []MemberOutcome{}, Ranked: []RankedResult{}, Detail: d}
}
func add(a, b uint64) (uint64, bool) {
	if math.MaxUint64-a < b {
		return 0, false
	}
	return a + b, true
}
func mul(a, b uint64) (uint64, bool) {
	if a != 0 && b > math.MaxUint64/a {
		return 0, false
	}
	return a * b, true
}
func stopped(c Control) (OperationOutcome, bool) {
	if c == nil {
		return "", false
	}
	if c.Cancelled() {
		return Cancelled, true
	}
	if c.DeadlineExceeded() {
		return Timeout, true
	}
	return "", false
}
func Evaluate(q Request, adm *Admission, o Options) Result {
	l := o.Limits
	if q.Schema != Schema || q.ID == "" || q.TopK < 0 || l.MaxRequestBytes == 0 || l.MaxMembers == 0 || l.MaxPaths == 0 || l.MaxSelectorRanges == 0 || l.MaxMemberRanges == 0 || l.MaxPrefixExpansion == 0 || l.MaxWitnesses == 0 || l.MaxSourceBytes == 0 || l.MaxOutputBytes == 0 || l.MaxWork == 0 {
		return terminal(q, InvalidRequest, "request or limits")
	}
	rb, e := json.Marshal(q)
	if e != nil || uint64(len(rb)) > l.MaxRequestBytes {
		return terminal(q, ResourceLimit, "request bytes")
	}
	if adm == nil {
		return terminal(q, SourceAdmissionUnavailable, "admission")
	}
	if adm.Schema != sourceadmissionv2.Schema || adm.AdmissionDigest != q.AdmissionDigest {
		return terminal(q, SourceAdmissionMismatch, "admission binding")
	}
	if o.ExpectedPolicyDigest != "" && q.PolicyDigest != o.ExpectedPolicyDigest {
		return terminal(q, PolicyMismatch, "policy")
	}
	if q.Relation != Intersects && q.Relation != ContainedBy && q.Relation != Contains {
		return terminal(q, InvalidRequest, "relation")
	}
	if uint64(len(q.Members)) > l.MaxMembers {
		return terminal(q, ResourceLimit, "members")
	}
	sources := map[string]sourceRef{}
	var sourceBytes uint64
	for _, s := range adm.Sources {
		if x, ok := stopped(o.Control); ok {
			return terminal(q, x, "source loop")
		}
		if _, dup := sources[s.Path]; dup {
			return terminal(q, SourceAdmissionMismatch, "duplicate source")
		}
		if !sourceadmissionv2.CanonicalPath(s.Path) || sourceadmissionv2.Digest(s.Bytes) != s.FileDigest || s.ObjectDigest == "" {
			return terminal(q, SourceAdmissionMismatch, "source")
		}
		var ok bool
		sourceBytes, ok = add(sourceBytes, uint64(len(s.Bytes)))
		if !ok || sourceBytes > l.MaxSourceBytes {
			return terminal(q, ResourceLimit, "source bytes")
		}
		sources[s.Path] = sourceRef{s.Revision, s.FileDigest, s.ObjectDigest}
	}
	sel, paths, sr, outcome, detail := expand(q.Selector, sources, l)
	if outcome != "" {
		return terminal(q, outcome, detail)
	}
	var memberRanges uint64
	for _, m := range q.Members {
		var ok bool
		memberRanges, ok = add(memberRanges, uint64(len(m.Ranges)))
		if !ok || memberRanges > l.MaxMemberRanges {
			return terminal(q, ResourceLimit, "member ranges")
		}
	}
	work := uint64(len(q.Members)) + paths + sr + memberRanges + sourceBytes + uint64(len(rb))
	for _, m := range q.Members {
		for _, u := range sel {
			if u.Path == m.Path {
				x, ok := mul(uint64(len(u.Ranges)), uint64(len(m.Ranges)))
				if !ok {
					return terminal(q, ResourceLimit, "cross product overflow")
				}
				work, ok = add(work, x)
				if !ok {
					return terminal(q, ResourceLimit, "work overflow")
				}
			}
		}
	}
	sortCharge := uint64(len(q.Members))
	var ok bool
	work, ok = add(work, sortCharge)
	if !ok || work > l.MaxWork {
		return terminal(q, ResourceLimit, "work")
	}
	rows := make([]MemberOutcome, len(q.Members))
	seen := map[string]bool{}
	wcount := uint64(0)
	for i, m := range q.Members {
		if x, stop := stopped(o.Control); stop {
			return terminal(q, x, "member loop")
		}
		r := MemberOutcome{Ordinal: i, MemberID: m.ID, Witnesses: []Witness{}}
		switch {
		case m.ID == "" || seen[m.ID]:
			r.Outcome = DuplicateMember
		case !m.PolicyAllowed:
			r.Outcome = FilteredByPolicy
		case !m.Available:
			r.Outcome = UnavailableLocation
		case !sourceadmissionv2.CanonicalPath(m.Path) || len(m.Ranges) == 0:
			r.Outcome = InvalidLocation
		default:
			s, exists := sources[m.Path]
			if !exists || s.r != m.Revision || s.f != m.FileDigest || s.o != m.ObjectDigest {
				r.Outcome = InvalidLocation
				break
			}
			bad := false
			for _, mr := range m.Ranges {
				if !valid(mr) {
					bad = true
					break
				}
			}
			if bad {
				r.Outcome = InvalidLocation
				break
			}
			for _, u := range sel {
				if u.Path != m.Path {
					continue
				}
				for _, a := range u.Ranges {
					for _, b := range m.Ranges {
						if x, stop := stopped(o.Control); stop {
							return terminal(q, x, "range loop")
						}
						in, hit := intersection(a, b)
						pass := hit
						if q.Relation == ContainedBy {
							pass = contains(a, b)
						} else if q.Relation == Contains {
							pass = contains(b, a)
						}
						if pass {
							wcount++
							if wcount > l.MaxWitnesses {
								return terminal(q, ResourceLimit, "witnesses")
							}
							r.Witnesses = append(r.Witnesses, Witness{m.Path, a, b, in, m.Revision, m.FileDigest, m.ObjectDigest})
						}
					}
				}
			}
			if len(r.Witnesses) > 0 {
				r.Outcome = Eligible
			} else {
				r.Outcome = Ineligible
			}
		}
		seen[m.ID] = true
		rows[i] = r
	}
	ranked := []RankedResult{}
	c := Counters{Input: len(rows), Witnesses: int(wcount), Work: work, SourceBytes: sourceBytes}
	for i, r := range rows {
		switch r.Outcome {
		case Eligible:
			c.Eligible++
			ranked = append(ranked, RankedResult{i, r.MemberID, q.Members[i].Score})
		case Ineligible:
			c.Ineligible++
		case UnavailableLocation:
			c.UnavailableLocation++
		case InvalidLocation:
			c.InvalidLocation++
		case DuplicateMember:
			c.DuplicateMember++
		case FilteredByPolicy:
			c.FilteredByPolicy++
		}
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].Score == ranked[j].Score {
			return ranked[i].Ordinal < ranked[j].Ordinal
		}
		return ranked[i].Score > ranked[j].Score
	})
	if q.TopK < len(ranked) {
		ranked = ranked[:q.TopK]
	}
	c.Ranked = len(ranked)
	res := Result{Schema: Schema, RequestID: q.ID, Outcome: Complete, Members: rows, Ranked: ranked, Counters: c, Detail: ""}
	if x, stop := stopped(o.Control); stop {
		return terminal(q, x, "before serialization")
	}
	b, e := json.Marshal(res)
	if e != nil {
		return terminal(q, BackendFailure, "serialization")
	}
	c.OutputBytes = uint64(len(b))
	res.Counters = c
	work, ok = add(work, uint64(len(b)))
	if !ok || work > l.MaxWork || uint64(len(b)) > l.MaxOutputBytes {
		return terminal(q, ResourceLimit, "output precharge")
	}
	res.Counters.Work = work
	if x, stop := stopped(o.Control); stop {
		return terminal(q, x, "before commit")
	}
	return res
}

type sourceRef struct{ r, f, o string }

func expand(s Selector, src map[string]sourceRef, l Limits) ([]PathRanges, uint64, uint64, OperationOutcome, string) {
	if s.Kind != ExactFile && s.Kind != RangeUnion && s.Kind != PathPrefix {
		return nil, 0, 0, InvalidSelector, "kind"
	}
	seen := map[string]bool{}
	var out []PathRanges
	var ranges uint64
	addp := func(p string) bool {
		if !sourceadmissionv2.CanonicalPath(p) || seen[p] {
			return false
		}
		if _, ok := src[p]; !ok {
			return false
		}
		seen[p] = true
		return true
	}
	switch s.Kind {
	case ExactFile:
		if !addp(s.Path) {
			return nil, 0, 0, InvalidSelector, "exact path"
		}
		out = []PathRanges{{Path: s.Path, Ranges: []Range{{Position{0, 0}, Position{math.MaxUint32, math.MaxUint32}}}}}
		ranges = 1
	case RangeUnion:
		if s.Path != "" || len(s.FrozenPaths) > 0 || len(s.Union) == 0 {
			return nil, 0, 0, InvalidSelector, "union shape"
		}
		for _, u := range s.Union {
			if !addp(u.Path) || len(u.Ranges) == 0 {
				return nil, 0, 0, InvalidSelector, "union path"
			}
			for _, r := range u.Ranges {
				if !valid(r) {
					return nil, 0, 0, InvalidRange, "selector range"
				}
				ranges++
			}
			out = append(out, u)
		}
	case PathPrefix:
		if !sourceadmissionv2.CanonicalPath(s.Path) || len(s.FrozenPaths) == 0 || len(s.Union) > 0 {
			return nil, 0, 0, InvalidSelector, "prefix shape"
		}
		expected := []string{}
		for p := range src {
			if p == s.Path || strings.HasPrefix(p, s.Path+"/") {
				expected = append(expected, p)
			}
		}
		sort.Strings(expected)
		got := append([]string(nil), s.FrozenPaths...)
		if !sort.StringsAreSorted(got) || len(got) != len(expected) {
			return nil, 0, 0, InvalidSelector, "frozen expansion"
		}
		for i, p := range got {
			if p != expected[i] || !addp(p) {
				return nil, 0, 0, InvalidSelector, "frozen expansion"
			}
			out = append(out, PathRanges{Path: p, Ranges: []Range{{Position{0, 0}, Position{math.MaxUint32, math.MaxUint32}}}})
			ranges++
		}
	}
	if uint64(len(seen)) > l.MaxPaths || ranges > l.MaxSelectorRanges || uint64(len(s.FrozenPaths)) > l.MaxPrefixExpansion {
		return nil, 0, 0, ResourceLimit, "selector limits"
	}
	return out, uint64(len(seen)), ranges, "", ""
}
func less(a, b Position) bool {
	return a.Line < b.Line || a.Line == b.Line && a.Character < b.Character
}
func valid(r Range) bool { return less(r.Start, r.End) }
func intersection(a, b Range) (Range, bool) {
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
