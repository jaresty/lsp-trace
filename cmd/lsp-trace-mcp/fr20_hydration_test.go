package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"lsp-trace/internal/graphprovenance"
	h "lsp-trace/internal/hydratedevidence"
)

// Actual public producer -> offline consumer. The wire fixture is synthetic;
// the provenance envelope is the original CLI output, never a fabricated graph.
func TestFR20HydrationIntegration(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("managed public CLI requires Darwin supervisor")
	}
	for k, v := range map[string]string{"GOPROXY": "off", "GOSUMDB": "off", "GOTOOLCHAIN": "local", "GOTELEMETRY": "off"} {
		t.Setenv(k, v)
	}
	cli := buildBinary(t, "lsp-trace", "./cmd/lsp-trace")
	fake := buildBinary(t, "fr20-server", "./cmd/lsp-trace-mcp/testdata/fr20-server")
	t.Run("fake-wire", func(t *testing.T) { proveFR20Hydration(t, cli, fake, false) })
	t.Run("installed-gopls-positional", func(t *testing.T) {
		server := os.Getenv("LSP_TRACE_FR20_GOPLS")
		if server == "" {
			var err error
			server, err = exec.LookPath("gopls")
			if err != nil {
				t.Skip("installed gopls unavailable; no install attempted")
			}
		}
		if !filepath.IsAbs(server) {
			t.Fatal("absolute installed gopls path required")
		}
		version, err := exec.Command(server, "version").CombinedOutput()
		if err != nil {
			t.Fatalf("gopls version: %v %s", err, version)
		}
		t.Logf("positional only; named selector UNQUALIFIED: %s", version)
		proveFR20Hydration(t, cli, server, true)
	})
}

func proveFR20Hydration(t *testing.T, cli, server string, native bool) {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{"a.go": "abcdefgh\n", "b.go": "abcdefgh\n", "c.go": "abcdefgh\n"}
	line, character := 0, 0
	if native {
		files = map[string]string{"go.mod": "module hydrationfixture\n\ngo 1.23\n", "a.go": "package fixture\nfunc A(){ B() }\n", "b.go": "package fixture\nfunc B(){ C() }\n", "c.go": "package fixture\nfunc C(){}\n"}
		line, character = 1, 5
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	target := func(id, name string) map[string]any {
		return map[string]any{"id": id, "locator": map[string]any{"uri": (&url.URL{Scheme: "file", Path: filepath.Join(root, name)}).String(), "line": line, "character": character}, "down_depth": 2, "up_depth": 2}
	}
	manifest := map[string]any{"schema_version": "lsp-trace.seed-manifest.v2", "coordinate_convention": "zero-based-session", "root": target("root", "a.go"), "required_targets": []any{target("end", "c.go")}, "limits": map[string]any{"timeout_ms": 60000, "request_timeout_ms": 10000}}
	artifacts := t.TempDir() // outside checkout; exact original retained independently
	save := func(name string, raw []byte) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(artifacts, name), raw, 0600); err != nil {
			t.Fatal(err)
		}
		if dir := os.Getenv("LSP_TRACE_HYDRATION_EVIDENCE_DIR"); dir != "" {
			dir = filepath.Join(dir, strings.ReplaceAll(t.Name(), "/", "_"))
			if err := os.MkdirAll(dir, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, name), raw, 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	marshal := func(v any) []byte {
		t.Helper()
		raw, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	save("manifest.json", marshal(manifest))
	args := []string{"slice", "--acquisition-version", "v2", "--workspace", root, "--server", server, "--seed-manifest", filepath.Join(artifacts, "manifest.json"), "--language-id", "go"}
	save("producer-command.json", marshal(append([]string{cli}, args...)))
	raw := runCLIProcess(t, cli, args...)
	original := append([]byte(nil), raw...)
	save("graph-provenance.v2.json", raw)
	digest := h.Digest(raw)
	save("artifact.sha256", []byte(digest+"\n"))
	t.Logf("actual producer=%q family=graph-provenance/v2 digest=%s", append([]string{cli}, args...), digest)
	input := h.Input{Artifact: raw}
	policy := h.DefaultPolicy()
	cat, err := h.Inspect(input, policy)
	if err != nil {
		t.Fatalf("ASSERT_ACTUAL_FR20_ADMISSION: %v", err)
	}
	save("catalog.json", marshal(cat))
	// Select a real native binding and its joined supply receipt, not a URI lookup
	// and not the synthetic receipt-selection handle. Captures must remain distinct.
	var record h.Record
	var source h.Source
	found := false
	for _, r := range cat.Records {
		if r.Authority != h.Native || !strings.HasPrefix(r.ID, "native:") || r.Range == nil {
			continue
		}
		for _, id := range r.SourceIDs {
			for _, s := range cat.Sources {
				if s.ID == id && s.Classification == graphprovenance.Supplied && s.State == "RETAINED_BYTES" {
					record, source, found = r, s, true
					break
				}
			}
			if found {
				break
			}
		}
		if found {
			break
		}
	}
	if !found {
		t.Fatal("ASSERT_EXPLICIT_SUPPLY_JOIN: no native returned range record with retained supply")
	}
	var evidence graphprovenance.EvidenceV2
	if err := json.Unmarshal(raw, &evidence); err != nil {
		t.Fatal(err)
	} // read metadata only; never remarshal as input
	var retained []byte
	for _, s := range evidence.Supplies {
		if s.Receipt != nil && "native:"+s.Receipt.ID == source.ID {
			retained = append([]byte(nil), s.Receipt.Content...)
		}
	}
	if len(retained) == 0 || source.ContentHash == nil || *source.ContentHash != h.Digest(retained) {
		t.Fatal("ASSERT_RETAINED_RECEIPT_BYTES")
	}
	captureDistinct := false
	for _, s := range cat.Sources {
		if s.URI == source.URI && s.Classification == graphprovenance.PostTraversal && s.ID != source.ID {
			captureDistinct = true
		}
	}
	if !captureDistinct || source.AnalyzedVersion != graphprovenance.Unverified {
		t.Fatal("ASSERT_SUPPLY_CAPTURE_AUTHORITY")
	}
	t.Logf("selected record=%s source=%s classification=%s receipt=%s content_hash=%s", record.ID, source.ID, source.Classification, source.ReceiptReference, *source.ContentHash)
	claimRef := "synthetic-integration-claim:shared-retained-context;actual-receipt=" + source.ReceiptReference
	side := h.Sidecar{SchemaVersion: h.SidecarVersion, ArtifactDigest: digest, Authority: h.Caller, Qualification: h.NonAuthoritative,
		Sources: []h.AssertedSource{
			{ID: "reference", URI: source.URI, ReceiptReference: source.ReceiptReference, VersionReference: source.VersionReference, SourceEncoding: "utf-8", State: "REFERENCE_ONLY"},
			{ID: "missing", URI: source.URI, ReceiptReference: "synthetic-missing-receipt", VersionReference: "synthetic-missing-version", SourceEncoding: "utf-8", State: "MISSING"},
		}, Records: []h.AssertedRecord{
			{ID: "claim", Kind: "SYNTHETIC_SHARED_CONTEXT_CLAIM", SourceIDs: []string{source.ID}, RelationshipReferences: []string{claimRef}},
			{ID: "reference", Kind: "SYNTHETIC_REFERENCE_ONLY", SourceIDs: []string{"reference"}, RelationshipReferences: []string{claimRef}},
			{ID: "missing", Kind: "SYNTHETIC_MISSING", SourceIDs: []string{"missing"}, RelationshipReferences: []string{claimRef}},
		}}
	sideRaw := marshal(side)
	save("hydrated-sidecar.v1.json", sideRaw)
	input.Sidecars = [][]byte{sideRaw}
	prefix := "sidecar:" + h.Digest(sideRaw) + ":"
	policy.IncludeBodies = true
	policy.MaxPageBytes = 4096 // exercise actual continuation, not a one-page shortcut
	req := h.Request{Policy: policy, Selections: []h.Selection{
		{ID: "native", RecordID: record.ID, SourceID: source.ID, Mode: "WHOLE_FILE"},
		{ID: "asserted", RecordID: prefix + "claim", SourceID: source.ID, Mode: "WHOLE_FILE"},
		{ID: "reference", RecordID: prefix + "reference", SourceID: prefix + "reference", Mode: "WHOLE_FILE"},
		{ID: "missing", RecordID: prefix + "missing", SourceID: prefix + "missing", Mode: "WHOLE_FILE"},
	}}
	save("request.json", marshal(req))
	if err := os.RemoveAll(root); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatal("ASSERT_CHECKOUT_REMOVED")
	}
	bundle, err := h.Hydrate(input, req)
	if err != nil {
		t.Fatalf("ASSERT_ACTUAL_FR20_HYDRATION: %v", err)
	}
	if err := h.Validate(input, req, bundle); err != nil {
		t.Fatal("ASSERT_OFFLINE_VALIDATE", err)
	}
	serialized := marshal(bundle)
	save("bundle.json", serialized)
	if err := h.ValidateJSON(input, req, serialized); err != nil {
		t.Fatal(err)
	}
	if bundle.TotalOrigins != 4 || len(bundle.Origins) != 4 || bundle.TotalSpans != 1 || len(bundle.Spans) != 1 || bundle.Complete {
		t.Fatal("ASSERT_ALL_ORIGIN_ACCOUNTING")
	}
	for i, status := range []string{"EXPORTED", "EXPORTED", "REFERENCE_ONLY", "MISSING_BYTES"} {
		o := bundle.Origins[i]
		if o.Status != status || o.Selection != req.Selections[i] {
			t.Fatalf("ASSERT_OMISSIONS: origin %d = %+v", i, o)
		}
		if i >= 2 && len(o.SpanIDs) != 0 {
			t.Fatal("ASSERT_OMITTED_ORIGIN_NO_SPAN")
		}
	}
	for i, authority := range []string{h.Native, h.Caller} {
		o := bundle.Origins[i]
		if o.Record == nil || o.Record.Authority != authority || o.CoordinateAuthority != "UNAVAILABLE" || o.OriginalRange != nil || !reflect.DeepEqual(o.SpanIDs, []string{bundle.Spans[0].ID}) {
			t.Fatal("ASSERT_DISTINCT_RECORD_COORDINATE_AUTHORITY")
		}
	}
	if bundle.Origins[1].Record.Qualification != h.NonAuthoritative || !reflect.DeepEqual(bundle.Origins[1].Record.RelationshipReferences, []string{claimRef}) {
		t.Fatal("ASSERT_SYNTHETIC_CLAIM_RETAINED")
	}
	span := bundle.Spans[0]
	if span.SourceID != source.ID || !bytes.Equal(span.Content, retained) || span.ContentHash != h.Digest(retained) || span.Bytes.Start != 0 || span.Bytes.End != len(retained) || !reflect.DeepEqual(span.OriginIDs, []string{"asserted", "native"}) {
		t.Fatalf("ASSERT_SHARED_EXACT_BODY: %+v", span)
	}
	text, err := h.Text(input, req, bundle)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{h.Native, h.Caller, "REFERENCE_ONLY", "MISSING_BYTES", record.ID, source.ID} {
		if !strings.Contains(text, want) {
			t.Fatalf("ASSERT_TEXT_IDENTITY: missing %q", want)
		}
	}
	save("bundle.txt", []byte(text))
	snap, err := h.NewSnapshot(input, req, bundle)
	if err != nil {
		t.Fatal(err)
	}
	var pages []h.Page
	for next := ""; ; {
		page, err := snap.Page(next)
		if err != nil {
			t.Fatal(err)
		}
		again, err := snap.Page(next)
		if err != nil || !reflect.DeepEqual(page, again) {
			t.Fatal("ASSERT_STABLE_PAGE")
		}
		if page.Snapshot != bundle.Digest || page.Ordinal != len(pages) || page.TotalOrigins != 4 || page.TotalSpans != 1 {
			t.Fatal("ASSERT_PAGE_IDENTITY")
		}
		pages = append(pages, page)
		if page.Next == "" {
			break
		}
		next = page.Next
	}
	if len(pages) < 2 {
		t.Fatal("ASSERT_MULTIPAGE_EXERCISED")
	}
	for _, p := range pages {
		if p.TotalPages != len(pages) {
			t.Fatal("ASSERT_ALL_PAGES")
		}
	}
	save("pages.json", marshal(pages))
	replayed, err := h.Reassemble(input, req, pages)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(serialized, marshal(replayed)) {
		t.Fatal("ASSERT_EXACT_LOGICAL_REASSEMBLY")
	}
	// Caller-selected coordinates do not inherit the native record's authority.
	coordinateRequest := req
	coordinateRequest.Selections = []h.Selection{{ID: "caller-coordinates", RecordID: record.ID, SourceID: source.ID, Mode: "SPAN", Encoding: "utf-8", Bytes: &h.Interval{Start: 0, End: len(retained)}}}
	coordinateBundle, err := h.Hydrate(input, coordinateRequest)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.Validate(input, coordinateRequest, coordinateBundle); err != nil {
		t.Fatal(err)
	}
	if len(coordinateBundle.Origins) != 1 || coordinateBundle.Origins[0].Status != "EXPORTED" || coordinateBundle.Origins[0].Record.Authority != h.Native || coordinateBundle.Origins[0].CoordinateAuthority != h.Caller {
		t.Fatal("ASSERT_CALLER_COORDINATES_NOT_NATIVE")
	}
	save("caller-coordinate-bundle.json", marshal(coordinateBundle))
	// Controlled violating inputs must fail without manufacturing native authority.
	badSide := side
	badSide.ArtifactDigest = h.Digest([]byte("different artifact"))
	if _, err := h.Inspect(h.Input{Artifact: raw, Sidecars: [][]byte{marshal(badSide)}}, policy); err == nil {
		t.Fatal("ASSERT_WRONG_DIGEST_REJECTED")
	}
	badSide = side
	badSide.Authority = h.Native
	if _, err := h.Inspect(h.Input{Artifact: raw, Sidecars: [][]byte{marshal(badSide)}}, policy); err == nil {
		t.Fatal("ASSERT_SIDECAR_ESCALATION_REJECTED")
	}
	if _, err := h.Reassemble(input, req, pages[:len(pages)-1]); err == nil {
		t.Fatal("ASSERT_MISSING_PAGE_REJECTED")
	}
	kept, err := os.ReadFile(filepath.Join(artifacts, "graph-provenance.v2.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(raw, original) || !bytes.Equal(kept, original) || h.Digest(kept) != digest {
		t.Fatal("ASSERT_ORIGINAL_ARTIFACT_UNCHANGED")
	}
	save("result.txt", []byte(fmt.Sprintf("PASS actual public FR20 -> offline hydration; digest=%s; pages=%d; origins=4; spans=1; checkout removed; synthetic sidecar only; named gopls UNQUALIFIED\n", digest, len(pages))))
	t.Logf("PASS ASSERT_ACTUAL_FR20_OFFLINE_CONNECTION digest=%s pages=%d origins=4 shared_spans=1", digest, len(pages))
}
