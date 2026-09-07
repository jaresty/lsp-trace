package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"lsp-trace/internal/boundedranking"
	"lsp-trace/internal/mcpcontract"
	"lsp-trace/internal/operation"
	"os"
	"strconv"
	"strings"
)

type rankingSeeds []boundedranking.Seed

func (s *rankingSeeds) String() string { return fmt.Sprint([]boundedranking.Seed(*s)) }
func (s *rankingSeeds) Set(v string) error {
	parts := strings.Split(v, "=")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return fmt.Errorf("seed must be EXACT_ID=POSITIVE_DECIMAL_INTEGER")
	}
	for _, c := range parts[1] {
		if c < '0' || c > '9' {
			return fmt.Errorf("seed weight must contain only decimal digits")
		}
	}
	for _, c := range parts[0] {
		if !strings.ContainsRune("0123456789abcdef", c) {
			return fmt.Errorf("seed ID must be exact lowercase hexadecimal")
		}
	}
	w, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || w < 1 || w > 1000000 {
		return fmt.Errorf("seed weight must be 1..1000000")
	}
	*s = append(*s, boundedranking.Seed{NodeID: parts[0], Weight: w})
	return nil
}
func admitBoundedRanking(raw []byte) error {
	_, err := boundedranking.ValidateFor(raw, boundedranking.Family, "v1")
	return err
}
func runBoundedRanking(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("bounded-retained-ranking", flag.ContinueOnError)
	fs.SetOutput(stderr)
	algorithm := fs.String("algorithm", "", "PAGERANK or PPR")
	alpha := fs.Float64("alpha", .85, "damping in (0,.99]")
	tol := fs.Float64("tolerance", 1e-9, "stationary L1 tolerance")
	iterations := fs.Int("max-iterations", 1000, "maximum updates")
	work := fs.Int("max-work", 1000000, "numerical scan budget")
	output := fs.String("output", "", "immutable generation selector")
	var seeds rankingSeeds
	fs.Var(&seeds, "seed", "repeatable EXACT_ID=POSITIVE_DECIMAL_INTEGER")
	if fs.Parse(args) != nil || fs.NArg() != 1 {
		fmt.Fprintln(stderr, "usage: lsp-trace bounded-retained-ranking --algorithm PAGERANK|PPR [--seed ID=WEIGHT] [--output SELECTOR] PATH|-")
		return 1
	}
	reader := stdin
	if fs.Arg(0) != "-" {
		f, err := os.Open(fs.Arg(0))
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		defer f.Close()
		reader = f
	}
	raw, err := io.ReadAll(io.LimitReader(reader, boundedranking.MaxInputBytes+1))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	values := map[string]any{"input": string(raw), "algorithm": *algorithm, "alpha": *alpha, "tolerance": *tol, "max_iterations": *iterations, "max_work": *work}
	if seeds != nil {
		values["seeds"] = seeds
	}
	input, err := json.Marshal(values)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	validator, err := mcpcontract.NewOperationInputValidator()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	executor := operation.NewOffline(validator, map[operation.Name]operation.Handler{operation.BoundedRetainedRanking: operation.BoundedRetainedRankingHandler})
	result, failure := executor.Execute(context.Background(), operation.Request{Name: operation.BoundedRetainedRanking, Input: input})
	if failure != nil {
		fmt.Fprintln(stderr, failure)
		return 1
	}
	if *output != "" {
		err = publishValidatedBundle(*output, result.Artifact, admitBoundedRanking)
	} else {
		_, err = stdout.Write(result.Artifact)
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}
