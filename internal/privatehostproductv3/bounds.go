//go:build darwin || linux

package privatehostproductv3

// Concrete conservative JSON field-graph bounds precede encoding. Six bytes per
// string byte cover Go JSON escaping; fixed envelopes cover keys/scalars/null.
func admitStrings(limit, fixed int, strings ...string) bool {
	if limit < fixed {
		return false
	}
	left := limit - fixed
	for _, v := range strings {
		if len(v) > left/6 {
			return false
		}
		left -= 6 * len(v)
	}
	return true
}
func admitResponse(r Response, limit int) bool {
	return admitStrings(limit, 1024, r.Protocol, r.RequestDigest, r.Label, r.Explanation, r.Alternative, r.RejectionCondition, r.Quote)
}
func admitTask(t Task, limit int) bool {
	if !admitStrings(limit, 8192, t.Application, t.ConfigSHA256, t.ScopeSHA256, t.Protocol, t.Role, t.InputID, t.Target, t.RequestDigest, t.Attempt, t.Predecessor, t.InputFamily, t.PreparedSHA256, t.CheckpointSHA256, t.Body, t.OwnBodySHA256, t.StructuralFacts, t.OriginalRawReferences[0], t.OriginalRawReferences[1], t.OriginalRawReferences[2], t.OriginalRawReferences[3]) {
		return false
	}
	// Review includes one complete prior proposal. Combined concrete graph must
	// fit, not two independent limits mistaken for one request bound.
	if t.Proposal != nil {
		p := t.Proposal
		strings := []string{t.Application, t.ConfigSHA256, t.ScopeSHA256, t.Protocol, t.Role, t.InputID, t.Target, t.RequestDigest, t.Attempt, t.Predecessor, t.InputFamily, t.PreparedSHA256, t.CheckpointSHA256, t.Body, t.OwnBodySHA256, t.StructuralFacts, t.OriginalRawReferences[0], t.OriginalRawReferences[1], t.OriginalRawReferences[2], t.OriginalRawReferences[3], p.Protocol, p.RequestDigest, p.Label, p.Explanation, p.Alternative, p.RejectionCondition, p.Quote}
		return admitStrings(limit, 9216, strings...)
	}
	return true
}
