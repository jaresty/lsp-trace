package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"lsp-trace/internal/graphprovenance"
)

func TestFR20GraphMatrix(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("managed CLI requires Darwin")
	}
	cli := buildBinary(t, "lsp-trace", "./cmd/lsp-trace")
	mcp := buildMCPBinary(t)
	server := buildBinary(t, "fr20-server", "./cmd/lsp-trace-mcp/testdata/fr20-server")
	for _, mode := range []string{"slice", "incoming"} {
		for _, scenario := range []string{"connected", "reverse", "root-ambiguous", "request-budget", "node-budget", "path-budget", "evidence-budget"} {
			t.Run(mode+"/"+scenario, func(t *testing.T) {
				root := t.TempDir()
				for _, n := range []string{"a", "b", "c", "d", "ambiguous", "missing", "failed"} {
					if err := os.WriteFile(filepath.Join(root, n+".go"), []byte("abcdefgh\n"), 0600); err != nil {
						t.Fatal(err)
					}
				}
				target := func(id, name string) map[string]any {
					return map[string]any{"id": id, "locator": map[string]any{"uri": (&url.URL{Scheme: "file", Path: filepath.Join(root, name+".go")}).String(), "symbol": name}, "down_depth": 2, "up_depth": 2}
				}
				rootName, end := "a", "c"
				if mode == "incoming" {
					rootName, end = "c", "a"
				}
				if scenario == "reverse" {
					rootName, end = end, rootName
				}
				if scenario == "root-ambiguous" {
					rootName = "ambiguous"
				}
				limits := map[string]any{"max_nodes": 20, "max_requests": 80, "max_path_work": 10000, "max_evidence_bytes": 1048576}
				switch scenario {
				case "request-budget":
					limits["max_requests"] = 2
				case "node-budget":
					limits["max_nodes"] = 1
				case "path-budget":
					limits["max_path_work"] = 0
				case "evidence-budget":
					limits["max_evidence_bytes"] = 0
				}
				manifest := map[string]any{"schema_version": "lsp-trace.seed-manifest.v2", "coordinate_convention": "zero-based-session", "root": target("root", rootName), "required_targets": []any{target("end", end), target("isolated", "d"), target("alias", rootName), target("ambiguous", "ambiguous"), target("missing", "missing"), target("failed", "failed")}, "limits": limits}
				raw, _ := json.Marshal(manifest)
				manifestPath := filepath.Join(t.TempDir(), "seeds.json")
				if err := os.WriteFile(manifestPath, raw, 0600); err != nil {
					t.Fatal(err)
				}
				args := []string{mode, "--acquisition-version", "v2", "--workspace", root, "--server", server, "--seed-manifest", manifestPath, "--language-id", "go"}
				cliRaw := runCLIProcess(t, cli, args...)
				config := map[string]any{"version": 1, "processes": []any{map[string]any{"alias": "fixture", "language_id": "go", "profile": map[string]any{"trust_domain": "public-acquisition", "workspace": root, "profile": "cli", "environment_reference": "cli"}, "execution": map[string]any{"path": server, "directory": root, "environment": os.Environ()}}}}
				publication := t.TempDir()
				// Use separate processes so actual first-supply observations match byte-for-byte.
				request := func(extra map[string]any) map[string]any {
					in := map[string]any{"session_id": "fixture", "generation": 1, "seed_manifest": manifest}
					for k, v := range extra {
						in[k] = v
					}
					return callRequest(1, "lsp_trace_v2_"+mode, in)
				}
				responses := runMCPProcess(t, mcp, []string{"--bootstrap-config", writeBootstrapJSON(t, config)}, []map[string]any{request(nil)})
				call := decodeProcessCall(t, responses[0])
				if call.env["operation_status"] != "SUCCEEDED" {
					t.Fatalf("ASSERT_MATRIX_MCP: %v", call.env)
				}
				mcpRaw := inlineArtifactBytes(t, call.env)
				if !bytes.Equal(cliRaw, mcpRaw) {
					t.Fatal("ASSERT_MATRIX_EXACT_PARITY")
				}
				var e graphprovenance.EvidenceV2
				if err := json.Unmarshal(mcpRaw, &e); err != nil {
					t.Fatal(err)
				}
				a := e.Acquisition
				if len(a.Targets) != 7 {
					t.Fatal("ASSERT_MATRIX_ALL_REQUESTED")
				}
				for i, id := range []string{"root", "end", "isolated", "alias", "ambiguous", "missing", "failed"} {
					if a.Targets[i].Requested.ID != id {
						t.Fatal("ASSERT_MATRIX_ORDER")
					}
				}
				if a.Usage.Requests > limits["max_requests"].(int) || a.Usage.Nodes > limits["max_nodes"].(int) || a.Usage.EvidenceBytes > limits["max_evidence_bytes"].(int) {
					t.Fatal("ASSERT_MATRIX_GLOBAL_BOUNDS")
				}
				if scenario == "connected" || scenario == "reverse" {
					status := "FOUND"
					if scenario == "reverse" {
						status = "NOT_FOUND_IN_RETAINED_GRAPH"
					}
					if a.Targets[1].Connection.Status != status {
						t.Fatalf("ASSERT_MATRIX_DIRECTION: %+v", a.Targets[1].Connection)
					}
					if scenario == "connected" && (len(a.Targets[1].Connection.Path.GroupIDs) != 2 || len(a.Targets[1].Connection.Path.Nodes) != 3) {
						t.Fatal("ASSERT_MATRIX_INTERMEDIATE_WITNESS")
					}
					if a.Targets[2].Connection.Status != "NOT_FOUND_IN_RETAINED_GRAPH" || a.Targets[3].Connection.Status != "FOUND" {
						t.Fatal("ASSERT_MATRIX_DISCONNECTED_ALIAS")
					}
					if a.Targets[4].Resolution.Status != "AMBIGUOUS" || a.Targets[5].Resolution.Status != "MISSING" || a.AcquisitionComplete {
						t.Fatal("ASSERT_MATRIX_PARTIAL_NOT_EMPTY")
					}
				}
				if scenario == "root-ambiguous" && (a.Targets[0].Resolution.Status != "AMBIGUOUS" || a.Targets[1].Resolution.Status != "RESOLVED" || a.Targets[1].Connection.Status != "NOT_EVALUABLE") {
					t.Fatal("ASSERT_MATRIX_ROOT_FAILURE_PRESERVES_OTHERS")
				}
				if scenario == "path-budget" && a.Targets[1].Connection.Status != "INCOMPLETE" {
					t.Fatal("ASSERT_MATRIX_PATH_BUDGET_NOT_NEGATIVE")
				}
				published := runMCPProcess(t, mcp, []string{"--bootstrap-config", writeBootstrapJSON(t, config), "--publication-root", publication}, []map[string]any{request(map[string]any{"output_selector": "evidence.json", "detail": "compact"})})
				p := decodeProcessCall(t, published[0])
				if p.env["operation_status"] != "SUCCEEDED" {
					t.Fatalf("ASSERT_MATRIX_PUBLICATION: %v", p.env)
				}
				got, err := os.ReadFile(filepath.Join(publication, "evidence.json"))
				if err != nil || !bytes.Equal(got, mcpRaw) {
					t.Fatal("ASSERT_MATRIX_PUBLICATION_BYTES")
				}
				if err := os.RemoveAll(root); err != nil {
					t.Fatal(err)
				}
				if err := graphprovenance.ValidateV2(e); err != nil {
					t.Fatalf("ASSERT_MATRIX_SOURCE_GONE: %v", err)
				}
				offline := runMCPProcess(t, mcp, nil, []map[string]any{callRequest(1, "lsp_trace_v1_validate", map[string]any{"input": string(mcpRaw), "schema": map[string]any{"family": "graph-provenance", "version": "v2"}})})
				if v := decodeProcessCall(t, offline[0]); v.env["operation_status"] != "SUCCEEDED" {
					t.Fatalf("ASSERT_MATRIX_OFFLINE_VALIDATE: %v", v.env)
				}
				// Removing one target must fail closed, not silently drop requested coverage.
				e.Acquisition.Targets = e.Acquisition.Targets[:6]
				if graphprovenance.ValidateV2(e) == nil {
					t.Fatal("ASSERT_MATRIX_MISSING_ACCOUNTING_REJECTED")
				}
			})
		}
	}
}

func TestFR20ManifestFIFOIsBounded(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Unix FIFO fixture")
	}
	cli := buildBinary(t, "lsp-trace", "./cmd/lsp-trace")
	root := t.TempDir()
	fifo := filepath.Join(root, "manifest.fifo")
	if out, err := exec.Command("mkfifo", fifo).CombinedOutput(); err != nil {
		t.Fatalf("fixture: %v %s", err, out)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, cli, "slice", "--acquisition-version", "v2", "--workspace", root, "--server", "must-not-launch", "--seed-manifest", fifo).CombinedOutput()
	if ctx.Err() != nil || err == nil {
		t.Fatalf("ASSERT_MANIFEST_FIFO_FAILS_CLOSED: %v %s", ctx.Err(), out)
	}
}

func TestFR20InvalidManifestNoLaunch(t *testing.T) {
	cli := buildBinary(t, "lsp-trace", "./cmd/lsp-trace")
	root := t.TempDir()
	marker := filepath.Join(root, "launched")
	script := filepath.Join(root, "server.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\ntouch '"+marker+"'\nexit 1\n"), 0700); err != nil {
		t.Fatal(err)
	}
	base := `{"schema_version":"lsp-trace.seed-manifest.v2","coordinate_convention":"zero-based-session","root":{"id":"root","locator":{"uri":"file:///fixture/a.go","line":0,"character":0}},"required_targets":[]}`
	invalid := []string{`null`, `"/tmp/seeds.json"`, strings.Replace(base, ".v2", ".v99", 1), strings.Replace(base, "zero-based-session", "one-based", 1), strings.Replace(base, `"character":0`, `"symbol":"a"`, 1), strings.Replace(base, `"line":0`, `"line":null`, 1), strings.Replace(base, `"line":0`, `"line":-1`, 1), strings.Replace(base, `"line":0`, `"line":0,"unknown":0`, 1), strings.Replace(base, `"line":0`, `"line":0,"line":1`, 1), strings.Replace(base, `file:///fixture/a.go`, `relative.go`, 1), strings.Replace(base, `"required_targets":[]`, `"required_targets":null`, 1), strings.Replace(base, `"required_targets":[]`, `"required_targets":[{"id":"root","locator":{"uri":"file:///fixture/a.go","symbol":"a"}}]`, 1), strings.Replace(base, `"required_targets":[]`, `"required_targets":[],"limits":{"max_requests":100001}`, 1), strings.Repeat(" ", 256<<10) + base}
	for i, raw := range invalid {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "manifest.json")
			if err := os.WriteFile(p, []byte(raw), 0600); err != nil {
				t.Fatal(err)
			}
			for _, mode := range []string{"slice", "incoming"} {
				if out, err := exec.Command(cli, mode, "--acquisition-version", "v2", "--workspace", root, "--server", script, "--seed-manifest", p).CombinedOutput(); err == nil {
					t.Fatalf("ASSERT_INVALID_REJECTED: %s", out)
				}
				if _, err := os.Stat(marker); !os.IsNotExist(err) {
					t.Fatal("ASSERT_INVALID_NO_LAUNCH")
				}
			}
		})
	}
	p := filepath.Join(t.TempDir(), "manifest.json")
	_ = os.WriteFile(p, []byte(base), 0600)
	for _, extra := range [][]string{{"--at", "a.go:1:1"}, {"--max-nodes", "1"}, {"--seed-file", p}, {"--from-file", "a.go"}, {"--acquisition-version", "v1"}} {
		args := append([]string{"slice", "--acquisition-version", "v2", "--workspace", root, "--server", script, "--seed-manifest", p}, extra...)
		if _, err := exec.Command(cli, args...).CombinedOutput(); err == nil {
			t.Fatal("ASSERT_CONFLICT_REJECTED")
		}
		if _, err := os.Stat(marker); !os.IsNotExist(err) {
			t.Fatal("ASSERT_CONFLICT_NO_LAUNCH")
		}
	}
}
