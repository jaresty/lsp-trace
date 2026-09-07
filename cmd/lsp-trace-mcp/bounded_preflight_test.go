package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"lsp-trace/internal/boundedanalysis"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/retainedcalls"
	"lsp-trace/internal/verification"
)

func TestBoundedPreflightPublicCarriers(t *testing.T) {
	raw, _ := boundedFixture(t)
	cli, mcp := buildBinary(t, "lsp-trace", "./cmd/lsp-trace"), buildMCPBinary(t)
	base, err := boundedanalysis.Analyze(context.Background(), raw, boundedanalysis.Parameters{Operation: "PROJECT"})
	if err != nil {
		t.Fatal(err)
	}
	deep := []byte(`{"x":` + strings.Repeat("[", 80) + `{"dup":0,"dup":1}` + strings.Repeat("]", 80) + `}`)
	digest := func(domain string, b []byte) string {
		return fmt.Sprintf("sha256:%x", sha256.Sum256(append([]byte(domain+"\x00"), b...)))
	}
	for _, layer := range []string{"provenance", "graph"} {
		t.Run(layer, func(t *testing.T) {
			var r retainedcalls.Evidence
			if err := json.Unmarshal(raw, &r); err != nil {
				t.Fatal(err)
			}
			if layer == "graph" {
				var p graphprovenance.Evidence
				if err := json.Unmarshal(r.InputBytes, &p); err != nil {
					t.Fatal(err)
				}
				p.GraphBytes = deep
				p.GraphDigest = digest(graphprovenance.Version+":graph", deep)
				r.InputBytes, err = json.Marshal(p)
				if err != nil {
					t.Fatal(err)
				}
			} else {
				r.InputBytes = deep
			}
			r.InputDigest = digest(retainedcalls.Version+":input", r.InputBytes)
			input, err := json.Marshal(r)
			if err != nil {
				t.Fatal(err)
			}
			var e boundedanalysis.Evidence
			if err = json.Unmarshal(base, &e); err != nil {
				t.Fatal(err)
			}
			e.InputBytes = input
			artifact, err := json.Marshal(e)
			if err != nil {
				t.Fatal(err)
			}
			dir := t.TempDir()
			write := func(name string, b []byte) string {
				t.Helper()
				p := filepath.Join(dir, name)
				if err := os.WriteFile(p, b, 0600); err != nil {
					t.Fatal(err)
				}
				return p
			}
			retainedPath, artifactPath := write("retained.json", input), write("analysis.json", artifact)
			if err := os.Mkdir(filepath.Join(dir, "generation"), 0700); err != nil {
				t.Fatal(err)
			}
			write("generation/artifact.json", artifact)
			receipt, err := verification.ReceiptBytes(artifact, verification.DirectoryDurabilityChecked)
			if err != nil {
				t.Fatal(err)
			}
			write("generation/receipt.json", receipt)
			selector := write("selected.json", []byte(`{"generation":"generation"}`))
			for _, args := range [][]string{
				{"bounded-retained-analysis", "--operation", "PROJECT", retainedPath},
				{"validate", "--family", boundedanalysis.Family, "--version", "v1", artifactPath},
				{"verify", "--family", boundedanalysis.Family, "--version", "v1", selector},
			} {
				out, err := exec.Command(cli, args...).CombinedOutput()
				if err == nil || !bytes.Contains(out, []byte("nesting LIMIT")) || bytes.Contains(out, []byte("duplicate")) {
					t.Fatalf("%v: %v %s", args, err, out)
				}
			}
			calls := runMCPProcess(t, mcp, nil, []map[string]any{
				callRequest(1, "lsp_trace_v1_bounded_retained_analysis", map[string]any{"input": string(input), "operation": "PROJECT"}),
				callRequest(2, "lsp_trace_v1_validate", map[string]any{"input": string(artifact), "schema": map[string]any{"family": boundedanalysis.Family, "version": "v1"}}),
				callRequest(3, "lsp_trace_v1_bounded_retained_analysis", map[string]any{"input": json.RawMessage(input), "operation": "PROJECT"}),
				callRequest(4, "lsp_trace_v1_validate", map[string]any{"input": json.RawMessage(artifact), "schema": map[string]any{"family": boundedanalysis.Family, "version": "v1"}}),
			})
			for _, call := range calls {
				env := decodeProcessCall(t, call).env
				b, _ := json.Marshal(env)
				if env["operation_status"] == "SUCCEEDED" || !bytes.Contains(b, []byte("nesting LIMIT")) || bytes.Contains(b, []byte("duplicate")) {
					t.Fatalf("MCP: %s", b)
				}
			}
		})
	}
}

func TestBoundedOpaqueDeepSourcePublicRoundtrip(t *testing.T) {
	content := []byte(strings.Repeat("[", 100) + `{"dup":0,"dup":1}` + strings.Repeat("]", 100))
	raw, _ := boundedFixtureContent(t, content)
	var r retainedcalls.Evidence
	if err := json.Unmarshal(raw, &r); err != nil {
		t.Fatal(err)
	}
	if len(r.Tables.Captures) == 0 {
		t.Fatal("missing content fixture")
	}
	for _, c := range r.Tables.Captures {
		if !bytes.Equal(c.Content, content) {
			t.Fatal("source content bytes changed")
		}
	}
	testBoundedRealOffline(t, buildBinary(t, "lsp-trace", "./cmd/lsp-trace"), buildMCPBinary(t), raw)
}
