package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	hi "lsp-trace/internal/hydratedinspection"
	"lsp-trace/internal/operation"
	"lsp-trace/internal/source"
)

type hydratedIDs []string

func (v *hydratedIDs) String() string     { return fmt.Sprint([]string(*v)) }
func (v *hydratedIDs) Set(s string) error { *v = append(*v, s); return nil }

type hydratedOptions struct {
	enabled  bool
	request  hi.Request
	sidecars hydratedIDs
}

func addHydratedFlags(fs *flag.FlagSet) *hydratedOptions {
	o := &hydratedOptions{request: hi.DefaultRequest()}
	r := &o.request
	fs.BoolVar(&o.enabled, "hydrated", false, "inspect exact retained graph node/relation context offline")
	fs.Var((*hydratedIDs)(&r.NodeIDs), "node", "exact native node ID (repeatable)")
	fs.Var((*hydratedIDs)(&r.RelationIDs), "relation", "exact native CALLS relation ID (repeatable)")
	fs.Var((*hydratedIDs)(&r.SiblingRelationIDs), "sibling-relation", "exact native sibling relation ID (repeatable)")
	fs.Var((*hydratedIDs)(&r.SidecarRecordIDs), "sidecar-record", "exact asserted sidecar record ID (repeatable)")
	fs.Var(&o.sidecars, "sidecar", "explicit hydrated-sidecar.v1 JSON file (repeatable)")
	fs.BoolVar(&r.IncludeBodies, "include-bodies", false, "include selected retained bytes")
	fs.BoolVar(&r.WholeFile, "whole-file", false, "select whole retained files instead of native ranges")
	fs.BoolVar(&r.EndpointContext, "endpoint-context", false, "also select caller/callee retained node ranges")
	fs.StringVar(&r.PositionEncoding, "position-encoding", "", "explicit utf-8, utf-16 or utf-32 conversion encoding")
	fs.BoolVar(&r.Page, "page", false, "emit one snapshot-bound JSON page; requires --json")
	fs.StringVar(&r.Cursor, "cursor", "", "continue with exact prior next_cursor and original options")
	p := &r.CorePolicy
	fs.IntVar(&p.MaxInputBytes, "max-input-bytes", p.MaxInputBytes, "core combined input budget; public cap also applies")
	fs.IntVar(&p.MaxOutputBytes, "max-output-bytes", p.MaxOutputBytes, "core output budget; public inline cap also applies")
	fs.IntVar(&p.MaxBodyBytes, "max-body-bytes", p.MaxBodyBytes, "selected body budget")
	fs.IntVar(&p.MaxOrigins, "max-origins", p.MaxOrigins, "selection and expanded origin budget")
	fs.IntVar(&p.MaxSpans, "max-spans", p.MaxSpans, "union span budget")
	fs.IntVar(&p.MaxWork, "max-work", p.MaxWork, "coordinate work budget")
	fs.IntVar(&p.MaxPageBytes, "max-page-bytes", p.MaxPageBytes, "core page budget")
	fs.IntVar(&p.MaxPages, "max-pages", p.MaxPages, "core page count budget")
	return o
}
func hydratedFlag(name string) bool {
	switch name {
	case "hydrated", "node", "relation", "sibling-relation", "sidecar-record", "sidecar", "include-bodies", "whole-file", "endpoint-context", "position-encoding", "page", "cursor", "max-input-bytes", "max-output-bytes", "max-body-bytes", "max-origins", "max-spans", "max-work", "max-page-bytes", "max-pages":
		return true
	}
	return false
}
func readHydratedFile(name string, limit int) ([]byte, error) {
	if limit < 1 {
		return nil, errors.New("explicit input byte limit")
	}
	// Reject final symlinks as well as root escapes. The scoped platform opener
	// checks the handle/nonregular state and is nonblocking for FIFO replacements.
	info, err := os.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("explicit input must be a regular nonsymlink file")
	}
	root, err := os.OpenRoot(filepath.Dir(name))
	if err != nil {
		return nil, err
	}
	defer root.Close()
	return source.ReadRegularInputBounded(root, filepath.Base(name), int64(limit))
}
func runInspectHydrated(input string, o *hydratedOptions, jsonOutput bool, stdout, stderr io.Writer) int {
	fail := func(err error) int { fmt.Fprintf(stderr, "inspect hydrated: INVALID_INPUT: %v\n", err); return 1 }
	r := o.request
	r.Input = "preflight"
	if r.Page && !jsonOutput {
		return fail(errors.New("--page requires --json"))
	}
	if len(o.sidecars) > 64 {
		return fail(errors.New("sidecar count limit"))
	}
	if err := hi.Check(r); err != nil {
		return fail(err)
	}
	limit := hi.MaxArtifactBytes
	if r.CorePolicy.MaxInputBytes < limit {
		limit = r.CorePolicy.MaxInputBytes
	}
	raw, err := readHydratedFile(input, limit)
	if err != nil {
		return fail(err)
	}
	r.Input = string(raw)
	limit -= len(raw)
	for _, file := range o.sidecars {
		raw, err = readHydratedFile(file, limit)
		if err != nil {
			return fail(err)
		}
		r.Sidecars = append(r.Sidecars, string(raw))
		limit -= len(raw)
	}
	request, err := json.Marshal(r)
	if err != nil {
		return fail(err)
	}
	result, failure := operation.InspectHydratedHandler(context.Background(), operation.Request{Name: operation.InspectHydrated, Input: request})
	if failure != nil {
		return fail(failure)
	}
	output := result.Artifact
	if !jsonOutput {
		v, ok := result.Value.(hi.View)
		if !ok {
			return fail(errors.New("unexpected operation result"))
		}
		text, err := hi.Text(r, v)
		if err != nil {
			return fail(err)
		}
		output = []byte(text)
	}
	if _, err = stdout.Write(output); err != nil {
		return fail(err)
	}
	return 0
}
