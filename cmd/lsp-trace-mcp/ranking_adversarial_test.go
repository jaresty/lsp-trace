package main

import (
	"bytes"
	"context"
	"encoding/json"
	"lsp-trace/internal/boundedanalysis"
	"lsp-trace/internal/boundedranking"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/operation"
	"lsp-trace/internal/retainedcalls"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRankingMalformedPublicParameters(t *testing.T) {
	raw, ids := boundedFixture(t)
	cli, mcp := buildBinary(t, "lsp-trace", "./cmd/lsp-trace"), buildMCPBinary(t)
	file := filepath.Join(t.TempDir(), "retained.json")
	_ = os.WriteFile(file, raw, 0600)
	for _, flags := range [][]string{{"--alpha", "NaN"}, {"--alpha", "Inf"}, {"--alpha", "0"}, {"--alpha", "1"}, {"--tolerance", "NaN"}, {"--tolerance", "0"}, {"--max-work", "0"}, {"--max-iterations", "0"}, {"--seed", ids[0] + "=1.5"}, {"--seed", ids[0] + "=1e2"}, {"--seed", ids[0] + "=0"}, {"--seed", ids[0] + "=1=2"}} {
		args := append([]string{"bounded-retained-ranking", "--algorithm", "PAGERANK"}, flags...)
		if _, err := exec.Command(cli, append(args, file)...).CombinedOutput(); err == nil {
			t.Fatal("CLI malformed", flags)
		}
	}
	bad := []map[string]any{{"alpha": 0}, {"alpha": 1}, {"tolerance": 1e-13}, {"max_work": 0}, {"max_iterations": 10001}, {"Alpha": .5}, {"unexpected": 1}, {"seeds": []any{}}, {"algorithm": "PPR", "seeds": []any{}}, {"algorithm": "PPR", "seeds": []any{map[string]any{"node_id": ids[0], "weight": 1.5}}}, {"algorithm": "PPR", "seeds": []any{map[string]any{"Node_ID": ids[0], "weight": 1}}}, {"algorithm": "PPR", "seeds": []any{map[string]any{"node_id": ids[0], "weight": 1}, map[string]any{"node_id": ids[0], "weight": 2}}}}
	calls := []map[string]any{}
	for i, params := range bad {
		params["input"] = string(raw)
		if params["algorithm"] == nil {
			params["algorithm"] = "PAGERANK"
		}
		b, _ := json.Marshal(params)
		if _, failure := operation.BoundedRetainedRankingHandler(context.Background(), operation.Request{Input: b}); failure == nil {
			t.Fatal("direct malformed", params)
		}
		calls = append(calls, callRequest(i+1, "lsp_trace_v1_bounded_retained_ranking", params))
	}
	results := runMCPProcess(t, mcp, nil, calls)
	for i, r := range results {
		b, _ := json.Marshal(r)
		if !bytes.Contains(b, []byte(`"error"`)) && !bytes.Contains(b, []byte(`"isError":true`)) {
			t.Fatal("MCP malformed", i, string(b))
		}
	}
}
func TestRankingCarriersDepthAndOpaque(t *testing.T) {
	for _, depth := range []int{64, 65} {
		b := []byte(strings.Repeat("[", depth) + "0" + strings.Repeat("]", depth))
		err := boundedanalysis.Preflight(b, 10000)
		if (err == nil) != (depth == 64) {
			t.Fatal("depth boundary", depth, err)
		}
	}
	raw, _ := boundedFixture(t)
	mcp := buildMCPBinary(t)
	p := boundedranking.Defaults("PAGERANK")
	for _, layer := range []string{"provenance", "graph", "hidden-provenance"} {
		var r retainedcalls.Evidence
		_ = json.Unmarshal(raw, &r)
		deep := []byte(`{"x":` + strings.Repeat("[", 80) + `{"dup":0,"dup":1}` + strings.Repeat("]", 80) + `}`)
		if layer == "graph" {
			var provenance graphprovenance.Evidence
			_ = json.Unmarshal(r.InputBytes, &provenance)
			provenance.GraphBytes = deep
			r.InputBytes, _ = json.Marshal(provenance)
		} else {
			r.InputBytes = deep
		}
		input, _ := json.Marshal(r)
		if layer == "hidden-provenance" {
			r.InputBytes = []byte(`{}`)
			later, _ := json.Marshal(r.InputBytes)
			input = append(bytes.TrimSuffix(input, []byte("}")), append([]byte(`,"Input_Bytes":`), append(later, '}')...)...)
		}
		if _, err := boundedranking.Analyze(context.Background(), input, p); err == nil {
			t.Fatal("bad carrier admitted", layer)
		}
		calls := runMCPProcess(t, mcp, nil, []map[string]any{callRequest(1, "lsp_trace_v1_bounded_retained_ranking", map[string]any{"input": string(input), "algorithm": "PAGERANK"}), callRequest(2, "lsp_trace_v1_bounded_retained_ranking", map[string]any{"input": json.RawMessage(input), "algorithm": "PAGERANK"})})
		for _, c := range calls {
			if decodeProcessCall(t, c).env["isError"] != true {
				t.Fatal(layer, c)
			}
		}
	}
	opaque, _ := boundedFixtureContent(t, []byte(strings.Repeat("[", 80)+`{"dup":0,"dup":1}`+strings.Repeat("]", 80)))
	if _, err := boundedranking.Analyze(context.Background(), opaque, p); err != nil {
		t.Fatal("opaque content parsed", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	b, err := boundedranking.Analyze(ctx, raw, p)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = boundedranking.ValidateFor(b, boundedranking.Family, "v1"); err != nil {
		t.Fatal("cancelled admission", err)
	}
}
func TestRankingLargeIntegerRawObject(t *testing.T) {
	raw, _ := boundedFixture(t)
	var r retainedcalls.Evidence
	_ = json.Unmarshal(raw, &r)
	var p graphprovenance.Evidence
	_ = json.Unmarshal(r.InputBytes, &p)
	p.Generation = 9007199254740993
	input, _ := json.Marshal(p)
	raw, err := retainedcalls.Export(input)
	if err != nil {
		t.Fatal(err)
	}
	var compact bytes.Buffer
	_ = json.Compact(&compact, raw)
	raw = compact.Bytes()
	want, err := boundedranking.Analyze(context.Background(), raw, boundedranking.Defaults("PAGERANK"))
	if err != nil {
		t.Fatal(err)
	}
	calls := runMCPProcess(t, buildMCPBinary(t), nil, []map[string]any{callRequest(1, "lsp_trace_v1_bounded_retained_ranking", map[string]any{"input": json.RawMessage(raw), "algorithm": "PAGERANK"})})
	if !bytes.Equal(inlineArtifactBytes(t, decodeProcessCall(t, calls[0]).env), want) {
		t.Fatal("raw large integer changed")
	}
}
