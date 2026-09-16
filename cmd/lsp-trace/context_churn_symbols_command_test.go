package main

import (
	"bytes"
	"os"
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

func TestContextChurnSymbolsEmitsVersionedV3Result(t *testing.T) {
	raw, err := os.ReadFile("context_churn_symbols_command.go")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(raw, []byte("vcssymbolsidecar.BuildV3(")) {
		t.Fatal("ASSERT_CONTEXT_CHURN_SYMBOLS_V3_OUTPUT_ROUTE: CLI does not select BuildV3")
	}
}
