package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"lsp-trace/internal/operation"
)

func TestContextSymbolChurnUnavailableProfileListsSortedConfiguredNames(t *testing.T) {
	executor := &contextSymbolChurnExecutor{profiles: map[string]historicalLSPProfile{"project": {}, "gopls": {}}}
	input, _ := json.Marshal(contextSymbolChurnInput{Input: "{}", Workspace: "/repo", FromRevision: "old", ToRevision: "new", Profile: "missing", LanguageID: "go"})
	_, failure := executor.Execute(context.Background(), operation.Request{Name: operation.Name("context_symbol_churn"), Input: input})
	if failure == nil || failure.Code != "PROFILE_UNAVAILABLE" || !strings.Contains(failure.Err.Error(), "available profiles: gopls, project") {
		t.Fatalf("ASSERT_SYMBOL_CHURN_PROFILE_DIAGNOSTICS: %+v", failure)
	}
}
