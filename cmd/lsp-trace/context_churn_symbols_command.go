package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"time"

	tsr "lsp-trace/internal/transientstructuralresult"
	"lsp-trace/internal/vcssymbolsidecar"
)

func runContextChurnSymbols(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("context-churn-symbols", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var input, workspace, from, to, serverPath, language string
	var machine bool
	var serverArgs stringsFlag
	var timeout, requestTimeout time.Duration
	fs.StringVar(&input, "input", "", "structural context V2 artifact file")
	fs.StringVar(&workspace, "workspace", "", "absolute Git workspace root")
	fs.StringVar(&from, "from", "", "starting Git revision")
	fs.StringVar(&to, "to", "", "ending Git revision")
	fs.StringVar(&serverPath, "server", "", "absolute language-server executable")
	fs.Var(&serverArgs, "server-arg", "repeatable language-server argument")
	fs.StringVar(&language, "language-id", "", "LSP language identifier")
	fs.DurationVar(&timeout, "timeout", 60*time.Second, "overall timeout")
	fs.DurationVar(&requestTimeout, "request-timeout", 30*time.Second, "per-request timeout")
	fs.BoolVar(&machine, "machine", false, "emit closed machine JSON")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if !machine || input == "" || workspace == "" || from == "" || to == "" || serverPath == "" || language == "" || !filepath.IsAbs(serverPath) || fs.NArg() != 0 {
		fmt.Fprintln(stderr, "context-churn-symbols requires --input FILE --workspace ABSOLUTE_PATH --from REVISION --to REVISION --server ABSOLUTE_PATH --language-id ID --machine")
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
	set := map[string]struct{}{}
	for _, node := range artifact.Nodes {
		set[node.Path] = struct{}{}
	}
	paths := make([]string, 0, len(set))
	for path := range set {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	result, err := vcssymbolsidecar.BuildV2(ctx, raw, vcssymbolsidecar.BuildRequest{Repository: workspace, FromRevision: from, ToRevision: to, Paths: paths, LanguageID: language}, vcssymbolsidecar.GitDiff{Timeout: timeout}, vcssymbolsidecar.GitWorktreeProvider{Timeout: timeout}, vcssymbolsidecar.LSPProcessProvider{Command: serverPath, Args: serverArgs, RequestTimeout: requestTimeout})
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if err = json.NewEncoder(stdout).Encode(result); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}
