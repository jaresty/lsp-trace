package acquisition

import (
	"context"
	"errors"
	"reflect"

	"lsp-trace/internal/retainedpath"
)

// Replay one shared budget in original target order. A cancellation report is
// a declared interruption boundary, not proof that an external event occurred.
// It must describe a prefix which had not already completed and clears all
// witnesses. Cancellation and limit state carry forward across targets.
func validatePathAccounting(r Result) error {
	nodes, edges := PathInput(r.Graph, r.Request.Context.ID)
	budget := &retainedpath.Budget{Context: context.Background(), Left: r.Request.Limits.MaxPathWork}
	globallyCancelled := false
	for _, rec := range r.Requests {
		if rec.Outcome == "BUDGET_BLOCKED" && cancellationReason(rec.Reason) {
			globallyCancelled = true
		}
	}
	for _, t := range r.Targets {
		for _, d := range []DirectionResult{t.Outgoing, t.Incoming} {
			for _, e := range d.Expansions {
				if e.Cached && e.Status == BudgetBlocked && cancellationReason(e.Reason) {
					globallyCancelled = true
				}
			}
		}
	}
	if globallyCancelled {
		budget.Reason = "CANCELLED"
	}
	for i, t := range r.Targets {
		if i == 0 || r.Targets[0].Admission != Admitted || t.Admission != Admitted {
			continue
		}
		c := t.Connection
		if c.Work < 0 || c.Work > budget.Left {
			return errors.New("path work outside remaining budget")
		}
		if c.Status == "INCOMPLETE" && c.Reason == "CANCELLED" {
			prefix := &retainedpath.Budget{Context: context.Background(), Left: c.Work, Reason: budget.Reason}
			result, err := retainedpath.Search(nodes, edges, c.From, c.To, prefix)
			if err != nil {
				return err
			}
			if result.Status != "INCOMPLETE" || prefix.Left != 0 || !reflect.DeepEqual(c.Path, emptyPath()) {
				return errors.New("cancellation is not an unfinished search prefix")
			}
			budget.Left -= c.Work
			budget.Reason = "CANCELLED"
			continue
		}
		before := budget.Left
		result, err := retainedpath.Search(nodes, edges, c.From, c.To, budget)
		if err != nil {
			return err
		}
		if c.Status != result.Status || c.Reason != result.Reason || !reflect.DeepEqual(c.Path, result.Path) || c.Work != before-budget.Left {
			return errors.New("connection differs from shared path-budget replay")
		}
	}
	if r.Usage.PathWork != r.Request.Limits.MaxPathWork-budget.Left {
		return errors.New("path usage differs from shared replay")
	}
	return nil
}

func cancellationReason(s string) bool {
	return s == context.Canceled.Error() || s == context.DeadlineExceeded.Error()
}
