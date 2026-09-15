package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestContextChurnSymbolsRequiresClosedInvocation(t *testing.T) {
	var out, stderr bytes.Buffer
	code := runContextChurnSymbols([]string{"--machine"}, &out, &stderr)
	if code != 1 || out.Len() != 0 || !strings.Contains(stderr.String(), "requires --input") {
		t.Fatalf("ASSERT_CONTEXT_CHURN_SYMBOLS_CLOSED_INVOCATION: code=%d out=%q stderr=%q", code, out.String(), stderr.String())
	}
}
