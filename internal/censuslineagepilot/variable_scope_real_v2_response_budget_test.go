//go:build darwin || linux

package censuslineagepilot_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"testing"

	"lsp-trace/sessionruntime"
)

func TestVariableCallerV2ResponseBudgetStrictBeforeClaim(t *testing.T) {
	for _, kind := range []string{"absent", "zero", "negative", "overflow", "excess", "old-schema"} {
		t.Run(kind, func(t *testing.T) {
			c, path, git := variableOwnedConfig(t, 1, 0)
			raw, e := os.ReadFile(path)
			if e != nil {
				t.Fatal(e)
			}
			// Preserve strict struct order for present fields; map reordering itself must
			// not be the reason these bad configurations are refused.
			switch kind {
			case "zero":
				c.ShutdownResponseTimeoutMS = 0
				raw, _ = json.Marshal(c)
			case "negative":
				c.ShutdownResponseTimeoutMS = -1
				raw, _ = json.Marshal(c)
			case "excess":
				c.ShutdownResponseTimeoutMS = 1001
				raw, _ = json.Marshal(c)
			case "old-schema":
				c.Schema = "private-variable-scope-v2-caller.v1"
				raw, _ = json.Marshal(c)
			case "absent":
				raw = bytes.Replace(raw, []byte(`,"shutdown_response_timeout_ms":250`), nil, 1)
			case "overflow":
				raw = bytes.Replace(raw, []byte(`"shutdown_response_timeout_ms":250`), []byte(`"shutdown_response_timeout_ms":9223372036854775808`), 1)
			}
			if e = os.WriteFile(path, raw, 0600); e != nil {
				t.Fatal(e)
			}
			factories := 0
			_, e = variableOwnedRun(context.Background(), "execute-once", path, git, func() (*sessionruntime.Manager, error) { factories++; return nil, nil }, variableClose)
			if e == nil || factories != 0 {
				t.Fatal("ASSERT_RESPONSE_CONFIG_BEFORE_CLAIM", e, factories)
			}
			variableEmpty(t, c)
		})
	}
}

func TestVariableCallerV2ResponseBudgetEqualityAndFacts(t *testing.T) {
	c, path, git := variableOwnedConfig(t, 1, 0)
	c.ShutdownResponseTimeoutMS = c.ShutdownTimeoutMS
	variableSave(t, path, c)
	factory, _, _, _ := variableOwnedFactory(t, "")
	r, e := variableOwnedRun(context.Background(), "execute-once", path, git, factory, variableClose)
	if e != nil {
		t.Fatal(e)
	}
	f := r.Cleanup.LifecyclePrivateV2
	if f.ResponseBudgetSource != "PRIVATE_V2" || f.DeclaredResponseBudget != sessionruntime.MaxShutdownResponseBudgetPrivateV2/10 || f.EffectiveResponseReadBudget <= 0 || f.EffectiveResponseReadBudget > f.DeclaredResponseBudget || r.Cleanup.StopCalls != 1 || r.Cleanup.ShutdownCalls != 1 || r.Cleanup.TerminalReads != 1 {
		t.Fatal("ASSERT_RESPONSE_CONFIG_EQUALITY_AND_FACTS", f, r.Cleanup)
	}
}
