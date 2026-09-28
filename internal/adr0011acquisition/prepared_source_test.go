package adr0011acquisition

import (
	"bytes"
	"errors"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"lsp-trace/internal/publication"
	"lsp-trace/sessionruntime"
)

func TestPreparedSourceCheckpoint(t *testing.T) {
	workspace := t.TempDir()
	uri := (&url.URL{Scheme: "file", Path: filepath.ToSlash(filepath.Join(workspace, "query.go"))}).String()
	root := privatePublicationRoot(t)
	text := []byte("package fixture\n")
	prepared, source, err := publishPreparedSource(root, text, uri, 4, workspace, reviewedSuccessorSchemaDigest, nil)
	if err != nil || prepared.stage != "VERIFIED" || source.stage != "VERIFIED" || !replayPreparedSource(root, prepared, source, text, uri, 4, workspace, reviewedSuccessorSchemaDigest, publication.ReadVerifiedBoundFile) {
		t.Fatalf("ASSERT_PREPARED_SOURCE_VERIFIED: %+v %+v %v", prepared, source, err)
	}
	for name, tc := range map[string]struct {
		data      []byte
		uri       string
		version   int
		workspace string
		read      func(*publication.Root, string, int64) ([]byte, error)
	}{
		"substituted-text": {data: []byte("wrong"), uri: uri, version: 4, workspace: workspace},
		"changed-version":  {data: text, uri: uri, version: 5, workspace: workspace},
		"wrong-workspace":  {data: text, uri: uri, version: 4, workspace: filepath.Join(workspace, "other")},
		"missing-payload": {data: text, uri: uri, version: 4, workspace: workspace, read: func(r *publication.Root, s string, n int64) ([]byte, error) {
			if strings.Contains(s, "prepared-text") {
				return nil, errors.New("missing")
			}
			return publication.ReadVerifiedBoundFile(r, s, n)
		}},
		"missing-ref": {data: text, uri: uri, version: 4, workspace: workspace, read: func(r *publication.Root, s string, n int64) ([]byte, error) {
			if s == prepared.selector {
				return nil, errors.New("missing")
			}
			return publication.ReadVerifiedBoundFile(r, s, n)
		}},
	} {
		t.Run(name, func(t *testing.T) {
			if tc.read == nil {
				tc.read = publication.ReadVerifiedBoundFile
			}
			if replayPreparedSource(root, prepared, source, tc.data, tc.uri, tc.version, tc.workspace, reviewedSuccessorSchemaDigest, tc.read) {
				t.Fatal("ASSERT_PREPARED_SOURCE_REJECT")
			}
		})
	}
	if _, _, e := publishPreparedSource(root, text, uri, 4, workspace, reviewedSuccessorSchemaDigest, nil); e == nil {
		t.Fatal("ASSERT_PREPARED_SOURCE_NO_REPLACE")
	}
	failed := func(r *publication.Root, s string, n int64) ([]byte, error) {
		return nil, errors.New("readback unavailable")
	}
	other := []byte("different")
	if _, state, e := publishPreparedSource(root, other, uri, 4, workspace, reviewedSuccessorSchemaDigest, failed); e == nil || state.stage == "VERIFIED" {
		t.Fatalf("ASSERT_PREPARED_SOURCE_READBACK: %+v %v", state, e)
	}
	if _, _, e := publishPreparedSource(privatePublicationRoot(t), bytes.Repeat([]byte{'a'}, sessionruntime.MaxDocumentSupplyBytes+1), uri, 4, workspace, reviewedSuccessorSchemaDigest, nil); e == nil {
		t.Fatal("ASSERT_PREPARED_SOURCE_BOUND")
	}
	boundRoot := privatePublicationRoot(t)
	atBound := bytes.Repeat([]byte{'a'}, sessionruntime.MaxDocumentSupplyBytes)
	bp, bs, e := publishPreparedSource(boundRoot, atBound, uri, 4, workspace, reviewedSuccessorSchemaDigest, nil)
	if e != nil || !replayPreparedSource(boundRoot, bp, bs, atBound, uri, 4, workspace, reviewedSuccessorSchemaDigest, publication.ReadVerifiedBoundFile) {
		t.Fatalf("ASSERT_PREPARED_SOURCE_AT_BOUND: %v", e)
	}
	if replayPreparedSource(root, prepared, source, text, uri, 4, workspace, "sha256:wrong", publication.ReadVerifiedBoundFile) {
		t.Fatal("ASSERT_PREPARED_SOURCE_SCHEMA_SUBSTITUTION")
	}
	changed := source
	changed.digest = privateDigest([]byte("wrong"))
	if replayPreparedSource(root, prepared, changed, text, uri, 4, workspace, reviewedSuccessorSchemaDigest, publication.ReadVerifiedBoundFile) {
		t.Fatal("ASSERT_PREPARED_SOURCE_CHANGED_DIGEST")
	}
}
func TestPreparedSourceCanonicalURI(t *testing.T) {
	workspace := t.TempDir()
	path := filepath.ToSlash(filepath.Join(workspace, "a.go"))
	valid := (&url.URL{Scheme: "file", Path: path}).String()
	if !canonicalPreparedURI(valid, workspace) {
		t.Fatal("ASSERT_CANONICAL_URI_VALID")
	}
	for _, candidate := range []string{valid + "?x=1", valid + "#x", strings.Replace(valid, "a.go", "%61.go", 1), "file://localhost" + path, "file://" + filepath.ToSlash(workspace) + "/sub/../a.go", strings.Replace(valid, "a.go", "%2Fa.go", 1), (&url.URL{Scheme: "file", Path: filepath.ToSlash(filepath.Join(filepath.Dir(workspace), "outside.go"))}).String()} {
		if canonicalPreparedURI(candidate, workspace) {
			t.Fatalf("ASSERT_CANONICAL_URI_ALIAS: %s", candidate)
		}
	}
}
func TestPreparedSourceEmptyPayloadControl(t *testing.T) {
	workspace := t.TempDir()
	uri := (&url.URL{Scheme: "file", Path: filepath.ToSlash(filepath.Join(workspace, "empty.go"))}).String()
	root := privatePublicationRoot(t)
	prepared, source, err := publishPreparedSource(root, []byte{}, uri, 0, workspace, reviewedSuccessorSchemaDigest, nil)
	if err == nil || prepared.stage == "VERIFIED" || source.stage == "VERIFIED" || replayPreparedSource(root, prepared, source, []byte{}, uri, 0, workspace, reviewedSuccessorSchemaDigest, publication.ReadVerifiedBoundFile) {
		t.Fatalf("ASSERT_PREPARED_SOURCE_EMPTY_REJECT: %v", err)
	}
}
