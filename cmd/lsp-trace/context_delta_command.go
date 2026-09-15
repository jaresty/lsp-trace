package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"lsp-trace/internal/transientstructuraldelta"
	tsr "lsp-trace/internal/transientstructuralresult"
)

const contextDeltaMaxFileBytes int64 = 1048576

func runContextDelta(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("context-delta", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var before, after string
	var machine bool
	fs.StringVar(&before, "before", "", "before V2 result file")
	fs.StringVar(&after, "after", "", "after V2 result file")
	fs.BoolVar(&machine, "machine", false, "emit closed machine JSON")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if !machine || before == "" || after == "" || fs.NArg() != 0 {
		fmt.Fprintln(stderr, "context-delta requires --before FILE --after FILE --machine")
		return 1
	}
	b, e := readContextDeltaFile(before)
	if e != nil {
		fmt.Fprintln(stderr, e)
		return 1
	}
	a, e := readContextDeltaFile(after)
	if e != nil {
		fmt.Fprintln(stderr, e)
		return 1
	}
	bv, e := tsr.DecodeV2Artifact(b)
	if e != nil {
		fmt.Fprintln(stderr, "before:", e)
		return 2
	}
	av, e := tsr.DecodeV2Artifact(a)
	if e != nil {
		fmt.Fprintln(stderr, "after:", e)
		return 2
	}
	raw, e := json.Marshal(transientstructuraldelta.Input{Before: bv, After: av})
	if e != nil {
		fmt.Fprintln(stderr, e)
		return 1
	}
	r, e := transientstructuraldelta.Execute(raw)
	if e != nil {
		fmt.Fprintln(stderr, e)
		return 2
	}
	out, e := json.Marshal(r)
	if e != nil {
		fmt.Fprintln(stderr, e)
		return 1
	}
	_, e = stdout.Write(append(out, '\n'))
	if e != nil {
		return 1
	}
	return 0
}
func readContextDeltaFile(name string) ([]byte, error) {
	before, e := os.Lstat(name)
	if e != nil {
		return nil, e
	}
	if !before.Mode().IsRegular() || before.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("context-delta inputs must be regular nonsymlink files")
	}
	f, e := os.Open(name)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	opened, e := f.Stat()
	if e != nil {
		return nil, e
	}
	if !opened.Mode().IsRegular() || !os.SameFile(before, opened) {
		return nil, errors.New("context-delta input identity changed")
	}
	if opened.Size() < 1 || opened.Size() > contextDeltaMaxFileBytes {
		return nil, errors.New("context-delta input exceeds bounded file size")
	}
	content, e := io.ReadAll(io.LimitReader(f, contextDeltaMaxFileBytes+1))
	if e != nil || int64(len(content)) > contextDeltaMaxFileBytes {
		return nil, errors.New("context-delta input exceeds bounded file size")
	}
	after, e := os.Lstat(name)
	if e != nil || !os.SameFile(opened, after) || after.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("context-delta input identity changed")
	}
	return content, nil
}
