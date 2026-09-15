package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"time"

	tsr "lsp-trace/internal/transientstructuralresult"
	"lsp-trace/internal/vcssidecar"
)

func runContextChurn(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("context-churn", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var input, workspace, from, to string
	var machine bool
	fs.StringVar(&input, "input", "", "structural context V2 artifact file")
	fs.StringVar(&workspace, "workspace", "", "absolute Git workspace root")
	fs.StringVar(&from, "from", "", "exclusive starting Git revision")
	fs.StringVar(&to, "to", "", "inclusive ending Git revision")
	fs.BoolVar(&machine, "machine", false, "emit closed machine JSON")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if !machine || input == "" || workspace == "" || from == "" || to == "" || fs.NArg() != 0 {
		fmt.Fprintln(stderr, "context-churn requires --input FILE --workspace ABSOLUTE_PATH --from REVISION --to REVISION --machine")
		return 1
	}
	raw, err := readContextDeltaFile(input)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	artifact, err := tsr.DecodeV2Artifact(raw)
	if err != nil {
		fmt.Fprintln(stderr, "input:", err)
		return 2
	}
	result, err := vcssidecar.Build(raw, artifact, vcssidecar.Request{Workspace: workspace, FromRevision: from, ToRevision: to}, vcssidecar.GitHistory{Timeout: 30 * time.Second})
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if _, err := stdout.Write(append(encoded, '\n')); err != nil {
		return 1
	}
	return 0
}
