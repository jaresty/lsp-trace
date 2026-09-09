package main

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"os"

	"lsp-trace/internal/normativeanalytics"
	"lsp-trace/internal/publication"
)

type relationFilters []string

func (r *relationFilters) String() string     { return fmt.Sprint([]string(*r)) }
func (r *relationFilters) Set(v string) error { *r = append(*r, v); return nil }

func runBoundedAnalyticsV2(command string, op normativeanalytics.Operation, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet(command, flag.ContinueOnError)
	fs.SetOutput(stderr)
	operationName := fs.String("operation", "", "ANALYSIS, METRICS or RANKING")
	maxWork := fs.Int64("max-work", 0, "positive local work bound")
	output := fs.String("output", "", "immutable generation selector (no replace)")
	publicationRoot := fs.String("publication-root", "", "absolute pinned root for input selector")
	inputSelector := fs.String("publication-selector", "", "relative retained graph selector")
	inputSchema := fs.String("input-schema-id", "", "exact retained graph schema identity")
	inputDigest := fs.String("input-digest", "", "exact retained graph sha256 digest")
	inputLength := fs.Uint64("input-byte-length", 0, "exact retained graph byte length")
	var filters relationFilters
	fs.Var(&filters, "filter", "repeatable retained relation")
	if fs.Parse(args) != nil {
		return 1
	}
	selectorMode := *inputSelector != "" || *publicationRoot != "" || *inputSchema != "" || *inputDigest != "" || *inputLength != 0
	if normativeanalytics.Operation(*operationName) != op || len(filters) == 0 || (!selectorMode && fs.NArg() != 1) || (selectorMode && fs.NArg() != 0) {
		fmt.Fprintf(stderr, "usage: lsp-trace %s --operation %s --filter RELATION --max-work N [--output SELECTOR] (PATH|- | --publication-root ROOT --publication-selector SELECTOR --input-schema-id ID --input-digest sha256:HEX --input-byte-length N)\n", command, op)
		return 1
	}
	var raw []byte
	var err error
	if selectorMode {
		if *publicationRoot == "" || *inputSelector == "" || *inputSchema != "https://jaresty.github.io/lsp-trace/schemas/lsp-trace.normative-retained-graph.v1.schema.json" || *inputLength == 0 {
			fmt.Fprintln(stderr, "incomplete publication selector")
			return 1
		}
		root, openErr := publication.OpenRoot(*publicationRoot)
		if openErr != nil {
			fmt.Fprintln(stderr, openErr)
			return 1
		}
		defer root.Close()
		raw, err = root.ReadSelector(*inputSelector, normativeanalytics.MaxRetainedBytes)
		if err == nil {
			sum := sha256.Sum256(raw)
			digest := "sha256:" + hex.EncodeToString(sum[:])
			if uint64(len(raw)) != *inputLength || digest != *inputDigest {
				err = fmt.Errorf("publication selector exact-byte verification failed")
			}
		}
	} else {
		raw, err = readBoundedV2Input(fs.Arg(0), stdin)
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	graph, _, err := normativeanalytics.DecodeRetainedGraph(raw)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	result, err := normativeanalytics.EvaluateLocal(normativeanalytics.LocalRequest{Operation: op, BuildRevision: graph.BuildRevision, RetainedJSON: raw, Relations: filters, Policy: normativeanalytics.LocalPolicy{MaxWork: *maxWork}})
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	artifact, err := normativeanalytics.MarshalLocalResult(result)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if *output != "" {
		if err := publishValidatedBundle(*output, artifact, func(candidate []byte) error { return normativeanalytics.ValidateLocalResultJSON(candidate, raw) }); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return 0
	}
	stdout.Write(artifact)
	return 0
}

func readBoundedV2Input(path string, stdin io.Reader) ([]byte, error) {
	if path == "-" {
		return io.ReadAll(io.LimitReader(stdin, normativeanalytics.MaxRetainedBytes+1))
	}
	return os.ReadFile(path)
}
