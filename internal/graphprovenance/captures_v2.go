package graphprovenance

import "errors"

// CaptureBudgetV2 is declared historical read accounting, not an authenticated
// filesystem event. Failed attempted reads conservatively charge their bound.
type CaptureBudgetV2 struct {
	RootStatus   string `json:"root_status"`
	Attempts     int    `json:"attempts"`
	ChargedBytes int    `json:"charged_bytes"`
}

func validateCapturesV2(e EvidenceV2, uris []string) error {
	return validateCapturesV2Profile(e, uris, nil)
}

func validateCapturesV2Profile(e EvidenceV2, uris []string, policy *SourcePolicyPrivateV1) error {
	var view *sourceProfilePrivate
	if policy != nil {
		var err error
		view, err = sourceProfileV1Private(*policy)
		if err != nil {
			return err
		}
	}
	return validateCapturesV2ProfilePrivate(e, uris, view)
}

func validateCapturesV2ProfilePrivate(e EvidenceV2, uris []string, policy *sourceProfilePrivate) error {
	if e.CaptureBudget.RootStatus != "OPENED" && e.CaptureBudget.RootStatus != "UNAVAILABLE" {
		return errors.New("V2 missing capture root outcome")
	}
	attempts, used := 0, 0
	cancelled := false
	for i, r := range e.Captures {
		if r.URI != uris[i] || r.Classification != PostTraversal || r.Supply != nil {
			return errors.New("V2 capture order/class mismatch")
		}
		if policy != nil && policy.withholds(e.WorkspaceURI, r.URI) {
			if !policy.validWithheld(e.WorkspaceURI, r) {
				return errors.New("private policy withholding receipt mismatch")
			}
			continue
		}
		if err := validateReceiptV2(e.WorkspaceURI, r); err != nil {
			return err
		}
		_, scope := RelativeURI(e.WorkspaceURI, r.URI)
		if scope != "" {
			continue
		}
		if r.Status == "CANCELLED" {
			cancelled = true
			continue
		}
		if cancelled {
			return errors.New("V2 post-cancellation read")
		}
		if attempts >= MaxFiles || used >= MaxTotalBytes {
			if r.Status != "BUDGET_EXCEEDED" {
				return errors.New("V2 omitted exhausted capture budget")
			}
			continue
		}
		if e.CaptureBudget.RootStatus == "UNAVAILABLE" {
			if r.Status != "UNREADABLE" {
				return errors.New("V2 capture without opened root")
			}
			continue
		}
		limit := min(MaxFileBytes, MaxTotalBytes-used)
		attempts++
		switch r.Status {
		case "READABLE":
			if len(r.Content) > limit {
				return errors.New("V2 capture byte budget")
			}
			used += len(r.Content)
		case "UNREADABLE", "MISSING", "NONREGULAR", "BYTE_LIMIT_EXCEEDED":
			used += limit
		default:
			return errors.New("V2 unmotivated capture disposition")
		}
	}
	if attempts != e.CaptureBudget.Attempts || used != e.CaptureBudget.ChargedBytes {
		return errors.New("V2 capture budget accounting mismatch")
	}
	return nil
}
