//go:build darwin || linux

package applicationinventory

import (
	"context"
	"encoding/json"
	dw "lsp-trace/internal/describeworker"
	cat "lsp-trace/internal/provisionalfeaturecatalog"
	"strings"
	"testing"
)

func TestTaskContractStrictExamples(t *testing.T) {
	c := taskContract(DurableTask{RunID: strings.Repeat("a", 64), TaskID: strings.Repeat("b", 64), Phase: "Describe", Role: "interpreter", Actor: "main"})
	raw, _ := json.Marshal(c)
	if len(raw) > 10000 || len(c.Operations["read"]) != 9 || c.Version != "lsp-trace.census-task-contract.v1" || !strings.Contains(c.Rules, "UNCERTAIN_SPENT") || !strings.Contains(c.Rules, "DeadlineUnixNano") {
		t.Fatal("bounded self-contained task contract", string(raw))
	}
	limits := dw.SuppliedResponseLimits{MaxAdapterBytes: c.MaxSubmissionBytes, MaxSemanticBytes: c.MaxSemanticBytes}
	candidate, err := dw.ParseSuppliedTerminalResponse([]byte(c.Example), limits)
	if err != nil {
		t.Fatal("Describe example must validate", err)
	}
	semantic := candidate.Semantic()
	if semantic.Verdict != "COMPLETE" || semantic.ConsumerNeed.Status != "UNRESOLVED" || semantic.ConsumerNeed.Value != "" || semantic.ProvidedBehavior.ConsumerRelative {
		t.Fatal("task-specific admissibility", semantic)
	}
	if len(c.Operations["task"]) != 5 || len(c.Operations["current_task"]) != 5 || len(c.Operations["exact_task"]) != 6 || strings.Join(c.Operations["task"], " ") != strings.Join(c.Operations["current_task"], " ") {
		t.Fatal("current-run recovery versus exact-task lookup", c.Operations)
	}
	if strings.Contains(c.ResponseSchema, "COMPLETE|ABSTAINED") || strings.Contains(c.ResponseSchema, "RESOLVED|UNRESOLVED") || !strings.Contains(c.Rules, "generic semantic parser and reused request prompt") {
		t.Fatal("generic vocabulary must not be task admissibility", c.ResponseSchema, c.Rules)
	}
	for _, invalid := range []string{strings.TrimSuffix(c.Example, "\n"), strings.Replace(c.Example, `"tokens":0`, `"tokens":"zero"`, 1), strings.Replace(c.Example, `"status":"COMPLETE"`, `"status":"OTHER"`, 1)} {
		if _, err := dw.ParseSuppliedTerminalResponse([]byte(invalid), limits); err == nil {
			t.Fatal("invalid Describe response admitted")
		}
	}
	ctx := context.Background()
	for _, wrong := range []bool{false, true} {
		b, _ := newTestBudgetV3()
		base, err := cat.BuildNativeV3(ctx, b, "main", nil, cat.ReviewModeSameThreadV3)
		if err != nil {
			t.Fatal(err)
		}
		task := DurableTask{Phase: "GroupDecision", Actor: "main", Role: "interpreter"}
		group := taskContract(task)
		answer := strings.Replace(group.Example, "CURRENT_INPUT_REVISION", base.Revision(), 1)
		proposed, err := applyInventoryDecision(ctx, b, base, []byte(answer), false, "main")
		if err != nil {
			t.Fatal("group example must validate", err)
		}
		task.Phase = "Reviewer"
		task.Role = "reviewer"
		review := taskContract(task)
		answer = strings.Replace(review.Example, "CURRENT_INPUT_REVISION", proposed.Revision(), 1)
		if wrong {
			answer = strings.Replace(answer, `"actor":"main"`, `"actor":"other"`, 1)
		}
		_, err = applyInventoryDecision(ctx, b, proposed, []byte(answer), true, "main")
		if (err != nil) != wrong {
			t.Fatalf("review contract actor admission: wrong=%v err=%v", wrong, err)
		}
	}
}
