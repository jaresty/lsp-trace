package main

import (
	"encoding/json"
	"fmt"
	v3 "lsp-trace/internal/adr0007sourcetextsearchv3private"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: evaluate <attempt.json>")
		os.Exit(2)
	}
	b, err := os.ReadFile(os.Args[1])
	if err != nil {
		panic(err)
	}
	var a v3.Attempt
	if err := json.Unmarshal(b, &a); err != nil {
		fmt.Println(`{"schema_version":"lsp-trace.adr0007.source-text-search.terminal-result.private.v3","terminal_sequence":0,"outcome":"FAILED","failure":{"code":"INVALID_INPUT","detail":{"json":"malformed"}},"authority":0,"accepted":false,"completeness":"UNKNOWN","featureIdentity":"UNRESOLVED","matches":[],"accounting":{"schema_version":"lsp-trace.adr0007.source-text-search.accounting.private.v3","failure_counters":{"INVALID_INPUT":1}},"custody":{"schema_version":"lsp-trace.adr0007.source-text-search.custody.private.v3","terminal_sequence":0,"terminal_count":1},"replay":{"schema_version":"lsp-trace.adr0007.source-text-search.replay.private.v3"}}`)
		return
	}
	os.Stdout.Write(v3.Canon(v3.Evaluate(a)))
}
