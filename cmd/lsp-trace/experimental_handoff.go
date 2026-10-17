package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"

	"lsp-trace/internal/censuscontinuation"
	"lsp-trace/internal/continuationhost"
	"lsp-trace/internal/publication"
)

const (
	experimentalHandoffReceiptSchema  = "lsp-trace.experimental-handoff-prepared-receipt.v1"
	experimentalHandoffUsage          = "usage: lsp-trace experimental-handoff prepare --publication-root ROOT --checkpoint-selector sha256:HEX"
	experimentalHandoffMaxObjectBytes = 64 << 20
)

type experimentalHandoffReceipt struct {
	SchemaVersion      string `json:"schema_version"`
	Status             string `json:"status"`
	PrivateSelector    string `json:"private_selector"`
	CheckpointSelector string `json:"checkpoint_selector"`
	Authority          int    `json:"authority"`
	Accepted           bool   `json:"accepted"`
	Completeness       string `json:"completeness"`
	Custody            string `json:"custody"`
}

type experimentalHandoffDependencies struct {
	openStore func(string) (censuscontinuation.Store, io.Closer, error)
	prepare   func(context.Context, censuscontinuation.Store, string) (string, error)
}

func productionExperimentalHandoffDependencies() experimentalHandoffDependencies {
	return experimentalHandoffDependencies{
		openStore: func(path string) (censuscontinuation.Store, io.Closer, error) {
			root, err := publication.OpenRoot(path)
			if err != nil {
				return nil, nil, err
			}
			store, err := continuationhost.NewStore(root, experimentalHandoffMaxObjectBytes)
			if err != nil {
				_ = root.Close()
				return nil, nil, err
			}
			return store, root, nil
		},
		prepare: censuscontinuation.PreparePrivateHandoffSelector,
	}
}

func runExperimentalHandoff(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	return runExperimentalHandoffWithDependencies(ctx, args, stdout, stderr, productionExperimentalHandoffDependencies())
}

func runExperimentalHandoffWithDependencies(ctx context.Context, args []string, stdout, stderr io.Writer, deps experimentalHandoffDependencies) int {
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Fprintln(stderr, experimentalHandoffUsage)
		return 0
	}
	if len(args) == 0 || args[0] != "prepare" {
		fmt.Fprintln(stderr, experimentalHandoffUsage)
		return 1
	}
	fs := flag.NewFlagSet("experimental-handoff prepare", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("publication-root", "", "existing private publication root")
	checkpoint := fs.String("checkpoint-selector", "", "exact REQUESTS_RENDERED/RUNNING checkpoint selector")
	if err := fs.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}
	if fs.NArg() != 0 || *root == "" || *checkpoint == "" || deps.openStore == nil || deps.prepare == nil {
		fmt.Fprintln(stderr, experimentalHandoffUsage)
		return 1
	}
	store, closer, err := deps.openStore(*root)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if closer == nil {
		fmt.Fprintln(stderr, errors.New("experimental handoff store closer unavailable"))
		return 1
	}
	selector, err := deps.prepare(ctx, store, *checkpoint)
	if err != nil {
		closeErr := closer.Close()
		fmt.Fprintln(stderr, err)
		if closeErr != nil {
			fmt.Fprintln(stderr, closeErr)
		}
		if errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
			return 130
		}
		return 1
	}
	if err := closer.Close(); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	receipt := experimentalHandoffReceipt{
		SchemaVersion: experimentalHandoffReceiptSchema, Status: "HANDOFF_PREPARED",
		PrivateSelector: selector, CheckpointSelector: *checkpoint,
		Authority: 0, Accepted: false, Completeness: "UNKNOWN", Custody: "CALLER_CONTROLLED_INFERENCE",
	}
	raw, err := json.Marshal(receipt)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if _, err := fmt.Fprintf(stdout, "%s\n", raw); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}
