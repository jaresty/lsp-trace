//go:build darwin || linux

package privatehostproductv3

// The prior-admission grammar accepts only a prefix of the actual generator:
// ordered initial cohort -> same-cohort review -> rejected subset correction ->
// same-subset re-review; optional navigation follows terminal item dispositions.
type cohortGrammar struct {
	last     *CohortTask
	response *CohortResponse
	next     int
}

func (g *cohortGrammar) accept(t CohortTask, r CohortReport) error {
	switch t.Role {
	case "interpreter":
		switch t.Stage {
		case "PROPOSE":
			if g.last != nil {
				if g.last.Role != "reviewer" || g.response == nil {
					return ErrInvalid
				}
				if g.last.Stage == "REVIEW" {
					for _, d := range g.response.Items {
						if d.Disposition == "UNSUPPORTED" {
							return ErrInvalid
						}
					}
				}
			}
			if g.next > r.Requested-len(t.Items) {
				return ErrInvalid
			}
			for n, i := range t.Items {
				if i.ItemID != r.Outcomes[g.next+n].Item.ItemID {
					return ErrInvalid
				}
			}
			g.next += len(t.Items)
		case "CORRECT":
			if g.last == nil || g.last.Role != "reviewer" || g.last.Stage != "REVIEW" || g.response == nil {
				return ErrInvalid
			}
			ids := []string{}
			for _, d := range g.response.Items {
				if d.Disposition == "UNSUPPORTED" {
					ids = append(ids, d.ItemID)
				}
			}
			if len(ids) != len(t.Items) {
				return ErrInvalid
			}
			for n, i := range t.Items {
				if i.ItemID != ids[n] {
					return ErrInvalid
				}
			}
		default:
			return ErrInvalid
		}
	case "reviewer":
		if g.last == nil || g.last.Role != "interpreter" || g.response == nil || len(g.last.Items) != len(t.Items) {
			return ErrInvalid
		}
		stage := "REVIEW"
		if g.last.Stage == "CORRECT" {
			stage = "REREVIEW"
		}
		if t.Stage != stage {
			return ErrInvalid
		}
		for n, i := range t.Items {
			if i.ItemID != g.last.Items[n].ItemID {
				return ErrInvalid
			}
		}
	default:
		return ErrInvalid
	}
	copy := t
	g.last = &copy
	g.response = nil
	return nil
}
func (g *cohortGrammar) acceptNavigation(r CohortReport) bool {
	if g.last == nil || g.last.Role != "reviewer" || g.response == nil || g.next != r.Requested || r.Pending != 0 {
		return false
	}
	if g.last.Stage == "REVIEW" {
		for _, d := range g.response.Items {
			if d.Disposition == "UNSUPPORTED" {
				return false
			}
		}
	}
	return true
}
