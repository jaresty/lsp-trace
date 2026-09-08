package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"

	he "lsp-trace/internal/hydratedevidence"
)

const hydratedEdge = "sha256:e16c80b01aa9de78ff896b411d93746a6bfa749c1a80bc7b1c3bd251df81fa4b"
const hydratedEdge2 = "sha256:42df1c10ff307e342f79d0a3d3f28cf115a501f6274619801c612e8705bef707"

func TestHydratedPublicOffline(t *testing.T) {
	raw, err := os.ReadFile("../../internal/hydratedevidence/testdata/focused-fr20.v2.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) != 154035 || he.Digest(raw) != "sha256:8913b3d062312be15531728f801e2f677f4a65852fd5fbeaf4ef1007ae1f83cf" {
		t.Fatal("original FR20 bytes changed")
	}
	cli := buildBinary(t, "lsp-trace", "./cmd/lsp-trace")
	mcp := buildMCPBinary(t)
	file := filepath.Join(t.TempDir(), "artifact.json")
	if err = os.WriteFile(file, raw, 0600); err != nil {
		t.Fatal(err)
	}
	var cliRaw []byte
	t.Run("CLI", func(t *testing.T) {
		t.Log("ASSERTION: PUBLIC_CLI_VALIDATED_FOCUS")
		cmd := exec.Command(cli, "inspect", file, "--hydrated", "--relation", hydratedEdge, "--relation", hydratedEdge2, "--node", "unknown", "--include-bodies", "--json")
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		cliRaw, err = cmd.Output()
		if err != nil {
			t.Fatalf("PUBLIC_CLI_VALIDATED_FOCUS FAIL: %v %s", err, stderr.String())
		}
		assertPublicFocus(t, raw, cliRaw)
		t.Log("PUBLIC_CLI_VALIDATED_FOCUS PASS")
	})
	t.Run("MCP", func(t *testing.T) {
		t.Log("ASSERTION: PUBLIC_MCP_VALIDATED_FOCUS")
		calls := runMCPProcess(t, mcp, nil, []map[string]any{callRequest(1, "lsp_trace_v1_inspect_hydrated", map[string]any{"input": string(raw), "relation_ids": []string{hydratedEdge, hydratedEdge2}, "node_ids": []string{"unknown"}, "include_bodies": true})})
		result, ok := calls[0]["result"].(map[string]any)
		if !ok {
			t.Fatalf("PUBLIC_MCP_VALIDATED_FOCUS FAIL: %v", calls[0])
		}
		env, ok := result["structuredContent"].(map[string]any)
		if !ok || env["isError"] != false {
			t.Fatalf("PUBLIC_MCP_VALIDATED_FOCUS FAIL: %v", result)
		}
		text, ok := env["content"].(string)
		if !ok {
			t.Fatalf("PUBLIC_MCP_VALIDATED_FOCUS FAIL: no artifact: %v", env)
		}
		assertPublicFocus(t, raw, []byte(text))
		if cliRaw != nil {
			var a, b any
			json.Unmarshal(cliRaw, &a)
			json.Unmarshal([]byte(text), &b)
			if !reflect.DeepEqual(a, b) {
				t.Fatal("PUBLIC_MCP_VALIDATED_FOCUS FAIL: CLI/MCP drift")
			}
		}
		decodeProcessCall(t, calls[0])
		t.Log("PUBLIC_MCP_VALIDATED_FOCUS PASS")
	})
}

func assertPublicFocus(t *testing.T, input, raw []byte) {
	t.Helper()
	var view struct {
		Manifest he.FocusManifest `json:"manifest"`
		Request  he.Request       `json:"request"`
		Bundle   he.Bundle        `json:"bundle"`
		Focus    he.FocusRequest  `json:"focus_request"`
		Delivery string           `json:"delivery"`
	}
	if err := json.Unmarshal(raw, &view); err != nil {
		t.Fatal(err)
	}
	if view.Delivery != "FULL" || len(view.Manifest.Origins) != 3 || view.Manifest.Origins[0].Status != "UNKNOWN_ID" || !view.Bundle.Complete {
		t.Fatalf("PUBLIC_VALIDATED_MANIFEST FAIL: %+v", view.Manifest)
	}
	if err := he.ValidateFocused(he.Input{Artifact: input}, view.Focus, he.FocusResult{Manifest: view.Manifest, Request: view.Request, Bundle: view.Bundle}); err != nil {
		t.Fatal(err)
	}
	if len(view.Bundle.Origins) != 3 || len(view.Bundle.Spans) != 3 {
		t.Fatal("PUBLIC_VALIDATED_MANIFEST FAIL: native receipt coverage")
	}
	if len(view.Bundle.Sources) != 5 {
		t.Fatal("PUBLIC_VALIDATED_MANIFEST FAIL: full catalog lost")
	}
}
