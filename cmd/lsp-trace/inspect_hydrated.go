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

	"lsp-trace/internal/artifactingress"
	hi "lsp-trace/internal/hydratedinspection"
	"lsp-trace/internal/operation"
	"lsp-trace/internal/publication"
	"lsp-trace/internal/source"
)

type hydratedIDs []string

func (v *hydratedIDs) String() string     { return fmt.Sprint([]string(*v)) }
func (v *hydratedIDs) Set(s string) error { *v = append(*v, s); return nil }

type hydratedOptions struct {
	enabled, enablePrivatePaths                          bool
	request                                              hi.Request
	sidecars                                             hydratedIDs
	publicationRoot, artifactStore, privateRoot          string
	artifactSchemaID, artifactDigest, artifactGeneration string
	artifactByteLength                                   uint64
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
	fs.StringVar(&o.publicationRoot, "publication-root", "", "absolute pinned verified-publication root")
	fs.StringVar(&o.artifactStore, "artifact-store", "", "absolute pinned immutable sha256 artifact store")
	fs.StringVar(&o.privateRoot, "private-root", "", "absolute owner-controlled private custody root")
	fs.BoolVar(&o.enablePrivatePaths, "enable-private-paths", false, "explicitly enable root-confined private-path ingress")
	fs.StringVar(&o.artifactSchemaID, "artifact-schema-id", "", "exact artifact schema identity")
	fs.StringVar(&o.artifactDigest, "artifact-digest", "", "exact sha256 artifact digest")
	fs.StringVar(&o.artifactGeneration, "artifact-generation", "", "exact content-derived generation")
	fs.Uint64Var(&o.artifactByteLength, "artifact-byte-length", 0, "exact artifact byte length")
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
	case "hydrated", "node", "relation", "sibling-relation", "sidecar-record", "sidecar", "publication-root", "artifact-store", "private-root", "enable-private-paths", "artifact-schema-id", "artifact-digest", "artifact-generation", "artifact-byte-length", "include-bodies", "whole-file", "endpoint-context", "position-encoding", "page", "cursor", "max-input-bytes", "max-output-bytes", "max-body-bytes", "max-origins", "max-spans", "max-work", "max-page-bytes", "max-pages":
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
	limit := r.CorePolicy.MaxInputBytes
	raw, err := admitHydratedCLI(input, o, int64(limit))
	if err != nil {
		return fail(err)
	}
	r.Input = string(raw)
	used := len(raw)
	limit -= len(raw)
	for _, file := range o.sidecars {
		raw, err = readHydratedFile(file, limit)
		if err != nil {
			return fail(err)
		}
		r.Sidecars = append(r.Sidecars, string(raw))
		used += len(raw)
		limit -= len(raw)
	}
	var view hi.View
	var output []byte
	if used > hi.MaxArtifactBytes {
		view, err = hi.InspectAdmitted(r)
		if err != nil {
			return fail(err)
		}
		output, err = json.Marshal(view)
		if err != nil {
			return fail(err)
		}
		output = append(output, '\n')
	} else {
		request, marshalErr := json.Marshal(r)
		if marshalErr != nil {
			return fail(marshalErr)
		}
		result, failure := operation.InspectHydratedHandler(context.Background(), operation.Request{Name: operation.InspectHydrated, Input: request})
		if failure != nil {
			return fail(failure)
		}
		output = result.Artifact
		var ok bool
		view, ok = result.Value.(hi.View)
		if !ok {
			return fail(errors.New("unexpected operation result"))
		}
	}
	if !jsonOutput {
		var text string
		if used > hi.MaxArtifactBytes {
			text, err = hi.TextAdmitted(r, view)
		} else {
			text, err = hi.Text(r, view)
		}
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

func admitHydratedCLI(input string, o *hydratedOptions, limit int64) ([]byte, error) {
	modes := 0
	for _, active := range []bool{o.publicationRoot != "", o.artifactStore != "", o.privateRoot != "" || o.enablePrivatePaths} {
		if active {
			modes++
		}
	}
	if modes == 0 {
		return readHydratedFile(input, int(limit))
	}
	if modes != 1 || o.artifactSchemaID == "" || o.artifactDigest == "" || o.artifactGeneration == "" || o.artifactByteLength == 0 {
		return nil, errors.New("exactly one complete hydrated ingress mode required")
	}
	cfg := artifactingress.Config{MaxBytes: limit, Validate: operation.ValidateHydratedIngress}
	expected := artifactingress.Expected{SchemaID: o.artifactSchemaID, ByteLength: o.artifactByteLength, Generation: o.artifactGeneration}
	if o.publicationRoot != "" {
		root, err := publication.OpenRoot(o.publicationRoot)
		if err != nil {
			return nil, errors.New("publication root unavailable")
		}
		defer root.Close()
		cfg.PublicationRoot = root
		receipt := publication.Receipt{Digest: o.artifactDigest, ByteLength: o.artifactByteLength, ArtifactSchemaID: o.artifactSchemaID, PublicationMechanism: publication.VerifiedGenerationMechanism, Generation: o.artifactGeneration, VerificationSelector: input}
		got, err := cfg.FromSelector(artifactingress.SelectorRequest{Selector: input, Receipt: receipt, Expected: expected})
		if err != nil {
			return nil, errors.New(err.Error())
		}
		return got.Bytes, nil
	}
	if o.artifactStore != "" {
		root, err := publication.OpenRoot(o.artifactStore)
		if err != nil {
			return nil, errors.New("artifact store unavailable")
		}
		defer root.Close()
		cfg.ArtifactStore = root
		got, err := cfg.FromContent(artifactingress.ContentRequest{ID: input, Expected: expected})
		if err != nil {
			return nil, errors.New(err.Error())
		}
		return got.Bytes, nil
	}
	if !o.enablePrivatePaths || !filepath.IsAbs(o.privateRoot) || filepath.Clean(o.privateRoot) != o.privateRoot {
		return nil, errors.New(string(artifactingress.PathDisabled))
	}
	info, err := os.Lstat(o.privateRoot)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0077 != 0 {
		return nil, errors.New("private root unavailable")
	}
	root, err := os.OpenRoot(o.privateRoot)
	if err != nil {
		return nil, errors.New("private root unavailable")
	}
	defer root.Close()
	cfg.PrivatePaths, cfg.PrivateRoot = true, root
	got, err := cfg.FromPrivatePath(artifactingress.PrivatePathRequest{Selector: input, Digest: o.artifactDigest, Expected: expected})
	if err != nil {
		return nil, errors.New(err.Error())
	}
	return got.Bytes, nil
}
