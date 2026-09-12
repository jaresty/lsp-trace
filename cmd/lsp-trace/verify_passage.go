package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"

	"lsp-trace/internal/passageverification"
	"lsp-trace/internal/schema"
)

const (
	passageVerificationFamily  = "passage-verification"
	passageVerificationVersion = "lsp-trace.passage-verification.v1"
	passageVerifyUsage         = "usage: lsp-trace verify passage --artifact-digest sha256:HEX --inspection-id ID --seed LABEL --node ID --uri URI --range-mode EXACT|INTERSECTS --position-encoding utf-8|utf-16|utf-32 --start-line N --start-character N --end-line N --end-character N --passage-digest sha256:HEX [immutable ingress flags] PATH_OR_SELECTOR"
)

type passageVerificationOutput struct {
	SchemaVersion string                       `json:"schema_version"`
	Results       []passageverification.Result `json:"results"`
}

func runVerifyPassage(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("verify passage", flag.ContinueOnError)
	fs.SetOutput(stderr)
	artifactDigest := fs.String("artifact-digest", "", "expected exact sha256 artifact digest")
	inspectionID := fs.String("inspection-id", "", "exact retained inspection/execution bundle ID")
	seed := fs.String("seed", "", "exact retained seed label")
	node := fs.String("node", "", "exact retained native node ID")
	uri := fs.String("uri", "", "exact expected node URI")
	rangeMode := fs.String("range-mode", "", "EXACT or INTERSECTS")
	encoding := fs.String("position-encoding", "", "utf-8, utf-16, or utf-32")
	startLine := fs.Int("start-line", -1, "zero-based expected start line")
	startCharacter := fs.Int("start-character", -1, "zero-based expected start character")
	endLine := fs.Int("end-line", -1, "zero-based expected end line")
	endCharacter := fs.Int("end-character", -1, "zero-based expected end character")
	passageDigest := fs.String("passage-digest", "", "expected exact sha256 passage digest")
	o := &hydratedOptions{}
	fs.StringVar(&o.publicationRoot, "publication-root", "", "absolute pinned verified-publication root")
	fs.StringVar(&o.artifactStore, "artifact-store", "", "absolute pinned immutable sha256 artifact store")
	fs.StringVar(&o.privateRoot, "private-root", "", "absolute owner-controlled private custody root")
	fs.BoolVar(&o.enablePrivatePaths, "enable-private-paths", false, "explicitly enable root-confined private-path ingress")
	fs.StringVar(&o.artifactSchemaID, "artifact-schema-id", "", "exact artifact schema identity")
	fs.StringVar(&o.artifactGeneration, "artifact-generation", "", "exact content-derived generation")
	fs.Uint64Var(&o.artifactByteLength, "artifact-byte-length", 0, "exact artifact byte length")
	if err := fs.Parse(args); err != nil || fs.NArg() != 1 {
		if err == nil {
			fmt.Fprintln(stderr, passageVerifyUsage)
		}
		return 1
	}
	if *artifactDigest == "" || *inspectionID == "" || *seed == "" || *node == "" || *uri == "" || *rangeMode == "" || *encoding == "" || *passageDigest == "" || *startLine < 0 || *startCharacter < 0 || *endLine < 0 || *endCharacter < 0 {
		fmt.Fprintln(stderr, passageVerifyUsage)
		return 1
	}
	ingressMode := o.publicationRoot != "" || o.artifactStore != "" || o.privateRoot != "" || o.enablePrivatePaths
	immutableMetadata := o.artifactSchemaID != "" || o.artifactGeneration != "" || o.artifactByteLength != 0
	if ingressMode != immutableMetadata {
		fmt.Fprintln(stderr, "verify passage: INVALID_INPUT: immutable ingress and exact metadata must be supplied together")
		return 1
	}
	o.artifactDigest = *artifactDigest
	validateArtifact := func(schemaID string, raw []byte) error {
		var header struct {
			SchemaVersion string `json:"schema_version"`
		}
		if json.Unmarshal(raw, &header) != nil || header.SchemaVersion == "" || schemaID != "https://jaresty.github.io/lsp-trace/schemas/"+header.SchemaVersion+".schema.json" {
			return errors.New("artifact schema mismatch")
		}
		return passageverification.ValidateArtifact(raw)
	}
	artifact, err := admitHydratedCLIValidated(fs.Arg(0), o, passageverification.MaxArtifactBytes, validateArtifact)
	if err != nil {
		fmt.Fprintf(stderr, "verify passage: INVALID_INPUT: %v\n", err)
		return 1
	}
	request := passageverification.Request{
		Artifact:               passageverification.Artifact{Bytes: artifact},
		ExpectedArtifactSHA256: *artifactDigest, InspectionID: *inspectionID,
		SeedLabel: *seed, NodeID: *node, ExpectedURI: *uri,
		RangeMode: passageverification.RangeMode(*rangeMode), PositionEncoding: *encoding,
		PositionConvention: "LSP_ZERO_BASED_END_EXCLUSIVE",
		ExpectedRange: passageverification.Range{
			Start: passageverification.Position{Line: *startLine, Character: *startCharacter},
			End:   passageverification.Position{Line: *endLine, Character: *endCharacter},
		},
		ExpectedPassageSHA256: *passageDigest,
	}
	if o.publicationRoot != "" {
		request.Artifact.Descriptor = &passageverification.ArtifactDescriptor{AdmissionStatus: "CUSTODY_ADMITTED", ExactSHA256: *artifactDigest, InspectionID: *inspectionID}
	}
	result := passageverification.VerifyBatch(passageverification.BatchRequest{Records: []passageverification.Request{request}})
	output, err := json.Marshal(passageVerificationOutput{SchemaVersion: passageVerificationVersion, Results: result.Results})
	if err == nil {
		_, err = schema.ValidateFor(output, passageVerificationFamily, "v1")
	}
	if err != nil {
		fmt.Fprintln(stderr, "verify passage: output schema validation failed")
		return 1
	}
	output = append(output, '\n')
	if _, err := stdout.Write(output); err != nil {
		fmt.Fprintln(stderr, errors.New("verify passage: output unavailable"))
		return 1
	}
	return 0
}
