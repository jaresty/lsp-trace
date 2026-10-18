package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"lsp-trace/internal/censuscontinuation"
	"lsp-trace/internal/continuationhost"
	"lsp-trace/internal/publication"
)

const (
	experimentalHandoffReceiptSchema  = "lsp-trace.experimental-handoff-prepared-receipt.v1"
	experimentalHandoffRetainedSchema = "lsp-trace.experimental-handoff-retained-result-receipt.v1"
	experimentalHandoffAdmittedSchema = "lsp-trace.experimental-handoff-admitted-result-receipt.v1"
	experimentalHandoffPrepareUsage   = "usage: lsp-trace experimental-handoff prepare --publication-root ROOT --checkpoint-selector sha256:HEX"
	experimentalHandoffUsage          = experimentalHandoffPrepareUsage + "\n" +
		"       lsp-trace experimental-handoff retain-result --publication-root ROOT --prepared-selector sha256:HEX --submission PATH|-\n" +
		"       lsp-trace experimental-handoff admit-result --publication-root ROOT --result-selector sha256:HEX"
	experimentalHandoffMaxObjectBytes     = 64 << 20
	experimentalHandoffMaxSubmissionBytes = 64 << 20
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

type experimentalHandoffRetainedResultReceipt struct {
	SchemaVersion  string `json:"schema_version"`
	ResultSelector string `json:"result_selector"`
}

type experimentalHandoffAdmittedResultReceipt struct {
	SchemaVersion     string `json:"schema_version"`
	AdmissionSelector string `json:"admission_selector"`
}

type experimentalHandoffDependencies struct {
	openStore func(string) (censuscontinuation.Store, io.Closer, error)
	openFile  func(string) (io.ReadCloser, error)
	prepare   func(context.Context, censuscontinuation.Store, string) (string, error)
	retain    func(context.Context, censuscontinuation.Store, string, []byte) (string, error)
	admit     func(context.Context, censuscontinuation.Store, string) (string, error)
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
		openFile: func(path string) (io.ReadCloser, error) {
			return os.Open(path)
		},
		prepare: censuscontinuation.PreparePrivateHandoffSelector,
		retain:  censuscontinuation.RetainPrivateCallerResultSelector,
		admit:   censuscontinuation.AdmitPrivateCallerResultSelector,
	}
}

func runExperimentalHandoff(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	return runExperimentalHandoffWithDependencies(ctx, args, stdin, stdout, stderr, productionExperimentalHandoffDependencies())
}

func runExperimentalHandoffWithDependencies(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer, deps experimentalHandoffDependencies) int {
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Fprintln(stderr, experimentalHandoffUsage)
		return 0
	}
	if len(args) == 0 {
		fmt.Fprintln(stderr, experimentalHandoffUsage)
		return 1
	}
	switch args[0] {
	case "prepare":
		return runExperimentalHandoffPrepare(ctx, args[1:], stdout, stderr, deps)
	case "retain-result":
		return runExperimentalHandoffRetain(ctx, args[1:], stdin, stdout, stderr, deps)
	case "admit-result":
		return runExperimentalHandoffAdmit(ctx, args[1:], stdout, stderr, deps)
	default:
		fmt.Fprintln(stderr, experimentalHandoffUsage)
		return 1
	}
}

func runExperimentalHandoffPrepare(ctx context.Context, args []string, stdout, stderr io.Writer, deps experimentalHandoffDependencies) int {
	fs := flag.NewFlagSet("experimental-handoff prepare", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	root := fs.String("publication-root", "", "existing private publication root")
	checkpoint := fs.String("checkpoint-selector", "", "exact REQUESTS_RENDERED/RUNNING checkpoint selector")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(stderr, experimentalHandoffPrepareUsage)
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}
	if fs.NArg() != 0 || *root == "" || *checkpoint == "" || deps.openStore == nil || deps.prepare == nil {
		fmt.Fprintln(stderr, experimentalHandoffPrepareUsage)
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

func runExperimentalHandoffRetain(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer, deps experimentalHandoffDependencies) int {
	fs := flag.NewFlagSet("experimental-handoff retain-result", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("publication-root", "", "existing private publication root")
	prepared := fs.String("prepared-selector", "", "exact private prepared selector")
	submission := fs.String("submission", "", "caller result path or - for stdin")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}
	if fs.NArg() != 0 || *root == "" || *prepared == "" || *submission == "" || (*submission == "-" && stdin == nil) || deps.openStore == nil || deps.retain == nil || (*submission != "-" && deps.openFile == nil) {
		fmt.Fprintln(stderr, experimentalHandoffUsage)
		return 1
	}
	raw, err := readExperimentalHandoffSubmission(*submission, stdin, deps.openFile)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return runExperimentalHandoffTransition(ctx, *root, stdout, stderr, deps.openStore, func(store censuscontinuation.Store) (string, error) {
		return deps.retain(ctx, store, *prepared, raw)
	}, func(selector string) any {
		return experimentalHandoffRetainedResultReceipt{SchemaVersion: experimentalHandoffRetainedSchema, ResultSelector: selector}
	})
}

func runExperimentalHandoffAdmit(ctx context.Context, args []string, stdout, stderr io.Writer, deps experimentalHandoffDependencies) int {
	fs := flag.NewFlagSet("experimental-handoff admit-result", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("publication-root", "", "existing private publication root")
	result := fs.String("result-selector", "", "exact private retained result selector")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}
	if fs.NArg() != 0 || *root == "" || *result == "" || deps.openStore == nil || deps.admit == nil {
		fmt.Fprintln(stderr, experimentalHandoffUsage)
		return 1
	}
	return runExperimentalHandoffTransition(ctx, *root, stdout, stderr, deps.openStore, func(store censuscontinuation.Store) (string, error) {
		return deps.admit(ctx, store, *result)
	}, func(selector string) any {
		return experimentalHandoffAdmittedResultReceipt{SchemaVersion: experimentalHandoffAdmittedSchema, AdmissionSelector: selector}
	})
}

func readExperimentalHandoffSubmission(path string, stdin io.Reader, openFile func(string) (io.ReadCloser, error)) ([]byte, error) {
	reader := stdin
	var closer io.Closer
	if path != "-" {
		file, err := openFile(path)
		if err != nil {
			return nil, err
		}
		reader, closer = file, file
	}
	raw, readErr := io.ReadAll(io.LimitReader(reader, experimentalHandoffMaxSubmissionBytes+1))
	if closer != nil {
		if closeErr := closer.Close(); readErr == nil {
			readErr = closeErr
		}
	}
	if readErr != nil {
		return nil, readErr
	}
	if len(raw) == 0 {
		return nil, errors.New("experimental handoff submission is empty")
	}
	if len(raw) > experimentalHandoffMaxSubmissionBytes {
		return nil, errors.New("experimental handoff submission exceeds 64 MiB")
	}
	return raw, nil
}

func runExperimentalHandoffTransition(ctx context.Context, root string, stdout, stderr io.Writer, openStore func(string) (censuscontinuation.Store, io.Closer, error), transition func(censuscontinuation.Store) (string, error), receipt func(string) any) int {
	store, closer, err := openStore(root)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if closer == nil {
		fmt.Fprintln(stderr, errors.New("experimental handoff store closer unavailable"))
		return 1
	}
	selector, transitionErr := transition(store)
	if errors.Is(ctx.Err(), context.Canceled) {
		if transitionErr != nil {
			fmt.Fprintln(stderr, transitionErr)
		}
		if closeErr := closer.Close(); closeErr != nil {
			fmt.Fprintln(stderr, closeErr)
		}
		return 130
	}
	if transitionErr != nil {
		closeErr := closer.Close()
		fmt.Fprintln(stderr, transitionErr)
		if closeErr != nil {
			fmt.Fprintln(stderr, closeErr)
		}
		if errors.Is(transitionErr, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
			return 130
		}
		return 1
	}
	if selector == "" {
		closeErr := closer.Close()
		fmt.Fprintln(stderr, errors.New("experimental handoff transition returned empty selector"))
		if closeErr != nil {
			fmt.Fprintln(stderr, closeErr)
		}
		return 1
	}
	if err := closer.Close(); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return writeExperimentalHandoffReceipt(stdout, stderr, receipt(selector))
}

func writeExperimentalHandoffReceipt(stdout, stderr io.Writer, receipt any) int {
	raw, err := json.Marshal(receipt)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	raw = append(raw, '\n')
	n, err := stdout.Write(raw)
	if err == nil && n != len(raw) {
		err = io.ErrShortWrite
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}
