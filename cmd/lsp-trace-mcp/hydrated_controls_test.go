package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	he "lsp-trace/internal/hydratedevidence"
	hi "lsp-trace/internal/hydratedinspection"
)

func TestHydratedPublicControls(t *testing.T) {
	raw, err := os.ReadFile("../../internal/hydratedevidence/testdata/focused-fr20.v2.json")
	if err != nil {
		t.Fatal(err)
	}
	cli := buildBinary(t, "lsp-trace", "./cmd/lsp-trace")
	mcp := buildMCPBinary(t)
	path := filepath.Join(t.TempDir(), "original.json")
	if err = os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	request := hi.DefaultRequest()
	request.Input = string(raw)
	request.RelationIDs = []string{hydratedEdge, hydratedEdge2}
	for _, mode := range []string{"private", "ranges", "whole", "endpoints", "sidecar", "page"} {
		t.Run(mode, func(t *testing.T) {
			r := request
			args := []string{"inspect", path, "--hydrated", "--relation", hydratedEdge, "--relation", hydratedEdge2, "--json"}
			if mode != "private" {
				r.IncludeBodies = true
				args = append(args, "--include-bodies")
			}
			if mode == "whole" {
				r.WholeFile = true
				args = append(args, "--whole-file")
			}
			if mode == "endpoints" {
				r.EndpointContext = true
				args = append(args, "--endpoint-context")
			}
			if mode == "sidecar" {
				native, e := he.HydrateFocused(r.InputBytes(), r.FocusRequest)
				if e != nil {
					t.Fatal(e)
				}
				side := he.Sidecar{SchemaVersion: he.SidecarVersion, ArtifactDigest: he.Digest(raw), Authority: he.Caller, Qualification: he.NonAuthoritative, Sources: []he.AssertedSource{}, Records: []he.AssertedRecord{{ID: "asserted", Kind: "CALLS", SourceIDs: []string{native.Bundle.Origins[0].Selection.SourceID}, RelationshipReferences: []string{hydratedEdge}, Range: &he.Range{Start: he.Position{Character: 1}, End: he.Position{Character: 2}}, Encoding: "utf-8"}}}
				sb, e := json.Marshal(side)
				if e != nil {
					t.Fatal(e)
				}
				sp := filepath.Join(t.TempDir(), "sidecar.json")
				if e = os.WriteFile(sp, sb, 0600); e != nil {
					t.Fatal(e)
				}
				r.Sidecars = []string{string(sb)}
				r.SidecarRecordIDs = []string{"sidecar:" + he.Digest(sb) + ":asserted"}
				args = append(args, "--sidecar", sp, "--sidecar-record", r.SidecarRecordIDs[0])
			}
			if mode == "page" {
				r.Page = true
				r.CorePolicy.MaxPageBytes = 4096
				args = append(args, "--page", "--max-page-bytes", "4096")
			}
			cliRaw := runCLIProcess(t, cli, args...)
			in, _ := json.Marshal(r)
			var values map[string]any
			json.Unmarshal(in, &values)
			calls := runMCPProcess(t, mcp, nil, []map[string]any{callRequest(1, "lsp_trace_v1_inspect_hydrated", values)})
			call := decodeProcessCall(t, calls[0])
			_ = call
			env := calls[0]["result"].(map[string]any)["structuredContent"].(map[string]any)
			text, ok := env["content"].(string)
			if !ok {
				t.Fatal("PUBLIC_CONTROL_PARITY FAIL", env)
			}
			var a, b hi.View
			if err = json.Unmarshal(cliRaw, &a); err != nil {
				t.Fatal(err)
			}
			if err = json.Unmarshal([]byte(text), &b); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(a, b) {
				t.Fatal("PUBLIC_CONTROL_PARITY FAIL: transport changed evidence")
			}
			if !r.Page {
				if err = hi.ValidateFull(r, a); err != nil {
					t.Fatal(err)
				}
			} else {
				pages := []hi.View{a}
				next := a.NextCursor
				for next != "" {
					r.Cursor = next
					in, _ = json.Marshal(r)
					json.Unmarshal(in, &values)
					calls = runMCPProcess(t, mcp, nil, []map[string]any{callRequest(1, "lsp_trace_v1_inspect_hydrated", values)})
					env = calls[0]["result"].(map[string]any)["structuredContent"].(map[string]any)
					text, ok = env["content"].(string)
					if !ok {
						t.Fatal(env)
					}
					var v hi.View
					if err = json.Unmarshal([]byte(text), &v); err != nil {
						t.Fatal(err)
					}
					pages = append(pages, v)
					next = v.NextCursor
				}
				r.Cursor = ""
				if _, err = hi.Reassemble(r, pages); err != nil {
					t.Fatal(err)
				}
				bad := append(append([]string{}, args...), "--cursor", a.NextCursor, "--node", "changed-unknown")
				if _, err = exec.Command(cli, bad...).CombinedOutput(); err == nil {
					t.Fatal("PUBLIC_CONTROL_PARITY FAIL: stale CLI cursor")
				}
			}
			t.Log("PUBLIC_CONTROL_PARITY PASS: " + mode)
		})
	}
	// Original-source filesystem is absent; the metadata URI is never a fallback.
	c, err := he.Inspect(he.Input{Artifact: raw}, he.DefaultPolicy())
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range c.Sources {
		if strings.HasPrefix(s.URI, "file:///tmp/") {
			if _, err = os.Stat(strings.TrimPrefix(s.URI, "file://")); err == nil {
				t.Fatal("PUBLIC_OFFLINE_SOURCE_ABSENT FAIL")
			}
		}
	}
	bad := []map[string]any{
		{"input": string(raw), "include_bodies": 1}, {"input": string(raw), "include_bodies": nil},
		{"input": string(raw), "core_policy": map[string]any{"max_work": json.Number("9007199254740993")}},
		{"input": string(raw), "core_policy": map[string]any{"max_work": json.Number("1.5")}},
		{"input": string(raw), "output_selector": "never-published.json"},
		{"input": map[string]any{}}, {"input": string(raw), "position_encoding": "guess"},
	}
	calls := []map[string]any{}
	for i, v := range bad {
		calls = append(calls, callRequest(i+1, "lsp_trace_v1_inspect_hydrated", v))
	}
	for i, response := range runMCPProcess(t, mcp, nil, calls) {
		if response["error"] == nil {
			t.Fatalf("PUBLIC_MCP_PREFLIGHT FAIL: %d %v", i, response)
		}
	}
	t.Log("PUBLIC_MCP_PREFLIGHT PASS; PUBLIC_OFFLINE_SOURCE_ABSENT PASS")
}
