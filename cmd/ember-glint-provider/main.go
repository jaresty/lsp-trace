package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"

	"lsp-trace/internal/emberglintprovider"
)

const processByteLimit = int64(4 << 20)

type noQualifiedAnalyzer struct{}

func (noQualifiedAnalyzer) QualifiedRelationKinds() []emberglintprovider.RelationKind { return nil }
func (noQualifiedAnalyzer) Analyze(context.Context, emberglintprovider.AnalysisRequest) (emberglintprovider.AnalysisResult, error) {
	return emberglintprovider.AnalysisResult{}, emberglintprovider.ErrUnavailable
}

type localFileCustody struct{}

func (localFileCustody) Resolve(_ context.Context, req emberglintprovider.CustodyRequest) ([]emberglintprovider.Document, error) {
	u, err := url.Parse(req.OriginalURI)
	if err != nil || u.Scheme != "file" || u.Path == "" {
		return nil, errors.New("custody unavailable: original_uri must be an absolute file URI")
	}
	body, err := os.ReadFile(u.Path)
	if err != nil {
		return nil, fmt.Errorf("custody unavailable: %w", err)
	}
	revision := fmt.Sprint(req.WorkspaceRevision)
	if revision == "" || revision == "<nil>" {
		if req.FailOnUnknownRevision {
			return nil, errors.New("custody unavailable: workspace revision is unknown")
		}
		revision = "unknown"
	}
	sum := sha256.Sum256(body)
	return []emberglintprovider.Document{{"document_id": req.OriginalURI + "@" + revision, "uri": req.OriginalURI, "revision": revision, "blob": "sha256:" + hex.EncodeToString(sum[:]), "range": map[string]any{}}}, nil
}

func run(args []string, stdin io.Reader, stdout io.Writer) error {
	if len(args) != 1 || args[0] != "--stdio" {
		return errors.New("usage: ember-glint-provider --stdio")
	}
	p := emberglintprovider.Provider{Analyzer: noQualifiedAnalyzer{}, Custody: localFileCustody{}, MaxRequestBytes: processByteLimit, MaxResponseBytes: processByteLimit}
	return p.Serve(context.Background(), stdin, stdout)
}
func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
