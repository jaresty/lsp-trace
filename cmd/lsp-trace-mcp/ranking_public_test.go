package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"lsp-trace/internal/boundedranking"
	"lsp-trace/internal/retainedcalls"
	"lsp-trace/internal/schema"
	"lsp-trace/internal/verification"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// Invoked on the source-deleted installed-gopls specimen as well as hermetic data.
func testRankingRealOffline(t *testing.T, cli, mcp string, raw []byte) {
	t.Helper()
	var retained retainedcalls.Evidence
	if err := json.Unmarshal(raw, &retained); err != nil {
		t.Fatal(err)
	}
	for _, algorithm := range []string{"PAGERANK", "PPR"} {
		t.Run("ranking-"+algorithm, func(t *testing.T) {
			p := boundedranking.Defaults(algorithm)
			args := []string{"bounded-retained-ranking", "--algorithm", algorithm}
			params := map[string]any{"input": string(raw), "algorithm": algorithm}
			if algorithm == "PPR" {
				id := retained.Tables.Endpoints[0].ID
				p.Seeds = []boundedranking.Seed{{NodeID: id, Weight: 3}}
				args = append(args, "--seed", id+"=3")
				params["seeds"] = p.Seeds
			}
			want, err := boundedranking.Analyze(context.Background(), raw, p)
			if err != nil {
				t.Fatal(err)
			}
			dir := t.TempDir()
			input := filepath.Join(dir, "retained.json")
			if err = os.WriteFile(input, raw, 0600); err != nil {
				t.Fatal(err)
			}
			got := runCLIProcess(t, cli, append(append([]string{}, args...), input)...)
			if !bytes.Equal(got, want) {
				t.Fatal("CLI/direct bytes")
			}
			if _, err = boundedranking.ValidateFor(got, boundedranking.Family, "v1"); err != nil {
				t.Fatal(err)
			}
			if _, err = schema.ValidateFor(got, boundedranking.Family, "v1"); err == nil {
				t.Fatal("lower shape-only route admitted ranking")
			}
			path := filepath.Join(dir, "ranking.json")
			_ = os.WriteFile(path, got, 0600)
			runCLIProcess(t, cli, "validate", "--family", boundedranking.Family, "--version", "v1", path)
			selected := filepath.Join(dir, "selected.json")
			publishArgs := append(append([]string{}, args...), "--output", selected, input)
			runCLIProcess(t, cli, publishArgs...)
			runCLIProcess(t, cli, "verify", "--family", boundedranking.Family, "--version", "v1", selected)
			if _, err = exec.Command(cli, publishArgs...).CombinedOutput(); err == nil {
				t.Fatal("overwrite")
			}
			if _, err = exec.Command(cli, "verify", selected).CombinedOutput(); err == nil {
				t.Fatal("default verify changed")
			}
			sb, _ := os.ReadFile(selected)
			selector, err := verification.DecodeSelector(sb)
			if err != nil {
				t.Fatal(err)
			}
			artifactPath := filepath.Join(dir, selector.Generation, "artifact.json")
			b, _ := os.ReadFile(artifactPath)
			if !bytes.Equal(b, got) {
				t.Fatal("selected bytes")
			}
			_ = os.WriteFile(artifactPath, append(b, ' '), 0600)
			if _, err = exec.Command(cli, "verify", "--family", boundedranking.Family, "--version", "v1", selected).CombinedOutput(); err == nil {
				t.Fatal("tamper")
			}
			root := t.TempDir()
			copyParams := func() map[string]any {
				m := map[string]any{}
				for k, v := range params {
					m[k] = v
				}
				return m
			}
			pub := copyParams()
			pub["output_selector"] = "ranking.json"
			compact := copyParams()
			compact["output_selector"] = "compact.json"
			compact["detail"] = "compact"
			noSelector := copyParams()
			noSelector["detail"] = "compact"
			unsafe := copyParams()
			unsafe["output_selector"] = "../bad.json"
			ref := map[string]any{"family": boundedranking.Family, "version": "v1"}
			calls := runMCPProcess(t, mcp, []string{"--publication-root", root}, []map[string]any{
				callRequest(1, "lsp_trace_v1_bounded_retained_ranking", params), callRequest(2, "lsp_trace_bounded_retained_ranking", params), callRequest(3, "lsp_trace_v1_validate", map[string]any{"input": string(got), "schema": ref}), callRequest(4, "lsp_trace_v1_bounded_retained_ranking", pub), callRequest(5, "lsp_trace_v1_bounded_retained_ranking", pub), callRequest(6, "lsp_trace_v1_bounded_retained_ranking", compact), callRequest(7, "lsp_trace_v1_schema_get", map[string]any{"schema": ref}), callRequest(8, "lsp_trace_v1_bounded_retained_ranking", noSelector), callRequest(9, "lsp_trace_v1_bounded_retained_ranking", unsafe),
			})
			for _, i := range []int{0, 1, 2} {
				if !bytes.Equal(inlineArtifactBytes(t, decodeProcessCall(t, calls[i]).env), got) {
					t.Fatal("MCP exact", i)
				}
			}
			schemaBytes := runCLIProcess(t, cli, "schema", "get", "--family", boundedranking.Family, "--version", "v1")
			if !bytes.Equal(inlineArtifactBytes(t, decodeProcessCall(t, calls[6]).env), schemaBytes) {
				t.Fatal("schema bytes")
			}
			for i, code := range map[int]string{4: "PUBLICATION_FAILED", 7: "OUTPUT_REQUIRES_SELECTOR", 8: "OUTPUT_SELECTOR_UNSAFE"} {
				if decodeProcessCall(t, calls[i]).env["code"] != code {
					t.Fatal("error envelope", i, calls[i])
				}
			}
			for _, name := range []string{"ranking.json", "compact.json"} {
				b, err := os.ReadFile(filepath.Join(root, name))
				if err != nil || !bytes.Equal(b, got) {
					t.Fatal("MCP publication bytes", err)
				}
			}
			if decodeProcessCall(t, calls[5]).env["summary"] == nil {
				t.Fatal("compact summary")
			}
		})
	}
}
func TestRankingPublicParity(t *testing.T) {
	raw, _ := boundedFixture(t)
	testRankingRealOffline(t, buildBinary(t, "lsp-trace", "./cmd/lsp-trace"), buildMCPBinary(t), raw)
}
func TestRankingRawObjectOversizeTamper(t *testing.T) {
	raw, _ := boundedFixture(t)
	var buf bytes.Buffer
	_ = json.Compact(&buf, raw)
	raw = buf.Bytes()
	p := boundedranking.Defaults("PAGERANK")
	want, err := boundedranking.Analyze(context.Background(), raw, p)
	if err != nil {
		t.Fatal(err)
	}
	large := append(append([]byte{}, raw...), bytes.Repeat([]byte(" "), 800000)...)
	root := t.TempDir()
	calls := runMCPProcess(t, buildMCPBinary(t), []string{"--publication-root", root}, []map[string]any{
		callRequest(1, "lsp_trace_v1_bounded_retained_ranking", map[string]any{"input": json.RawMessage(raw), "algorithm": "PAGERANK"}),
		callRequest(2, "lsp_trace_v1_validate", map[string]any{"input": json.RawMessage(bytes.TrimSpace(want)), "schema": map[string]any{"family": boundedranking.Family, "version": "v1"}}),
		callRequest(3, "lsp_trace_v1_bounded_retained_ranking", map[string]any{"input": string(large), "algorithm": "PAGERANK"}),
		callRequest(4, "lsp_trace_v1_bounded_retained_ranking", map[string]any{"input": string(large), "algorithm": "PAGERANK", "output_selector": "large.json"}),
	})
	if !bytes.Equal(inlineArtifactBytes(t, decodeProcessCall(t, calls[0]).env), want) || !bytes.Equal(inlineArtifactBytes(t, decodeProcessCall(t, calls[1]).env), bytes.TrimSpace(want)) {
		t.Fatal("raw object bytes")
	}
	if decodeProcessCall(t, calls[2]).env["code"] != "OUTPUT_REQUIRES_SELECTOR" {
		t.Fatal(calls[2])
	}
	b, err := os.ReadFile(filepath.Join(root, "large.json"))
	if err != nil || len(b) <= 1048576 {
		t.Fatal("oversize", err)
	}
	if _, err = boundedranking.ValidateFor(b, boundedranking.Family, "v1"); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*boundedranking.Evidence){func(e *boundedranking.Evidence) { e.Scores[0].Score += .1 }, func(e *boundedranking.Evidence) { e.Ranks[0], e.Ranks[1] = e.Ranks[1], e.Ranks[0] }, func(e *boundedranking.Evidence) { *e.Residual = 0 }, func(e *boundedranking.Evidence) { e.Personalization[0].Score += .1 }, func(e *boundedranking.Evidence) { e.Parameters.MaxWork = 1 }, func(e *boundedranking.Evidence) { e.Scope = "AUTHENTICATED" }, func(e *boundedranking.Evidence) { e.Iterations++ }} {
		var e boundedranking.Evidence
		_ = json.Unmarshal(want, &e)
		mutate(&e)
		basisBytes, _ := json.Marshal(map[string]any{"policy": e.Policy, "policies": e.Policies, "input_bytes": e.InputBytes, "parameters": e.Parameters})
		var basisValue any
		bd := json.NewDecoder(bytes.NewReader(basisBytes))
		bd.UseNumber()
		_ = bd.Decode(&basisValue)
		basisBytes, _ = json.Marshal(basisValue)
		e.BasisDigest = fmt.Sprintf("sha256:%x", sha256.Sum256(append(append([]byte(boundedranking.Version+":basis"), 0), basisBytes...)))
		e.Digest = ""
		encoded, _ := json.Marshal(e)
		var v any
		d := json.NewDecoder(bytes.NewReader(encoded))
		d.UseNumber()
		_ = d.Decode(&v)
		canon, _ := json.Marshal(v)
		e.Digest = fmt.Sprintf("sha256:%x", sha256.Sum256(append(append([]byte(boundedranking.Version+":result"), 0), canon...)))
		encoded, _ = json.Marshal(e)
		if _, err = boundedranking.ValidateFor(encoded, boundedranking.Family, "v1"); err == nil {
			t.Fatal("coherent reseal admitted")
		}
	}
}
