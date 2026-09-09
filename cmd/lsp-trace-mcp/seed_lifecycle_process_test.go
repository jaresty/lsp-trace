package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"lsp-trace/internal/provider"
	"lsp-trace/internal/seedbinding"
	"lsp-trace/sessionruntime"
)

func TestBuiltFakeLSPSeedAdmissionLifecycle(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("LOCAL_DARWIN_SUPERVISION_ONLY")
	}
	fake := buildBinary(t, "fake-lsp-seed", "./cmd/fake-lsp")
	for _, tc := range []struct {
		name, mode, providerVersion string
		want                        seedbinding.Status
		local                       bool
	}{
		{"match", "", "1", seedbinding.Match, false}, {"local-match", "", "1", seedbinding.Match, true}, {"mismatch", "mismatch", "1", seedbinding.Mismatch, false},
		{"invalid", "invalid", "1", seedbinding.Invalid, false}, {"timeout-unavailable", "hang", "1", seedbinding.Unavailable, false},
		{"provider-version-substitution", "", "2", seedbinding.Mismatch, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			workspace := t.TempDir()
			trace := filepath.Join(t.TempDir(), "trace")
			source := []byte("leaf")
			path := filepath.Join(workspace, "seed.go")
			if err := os.WriteFile(path, source, 0600); err != nil {
				t.Fatal(err)
			}
			sourceSum := sha256.Sum256(source)
			uri := (&url.URL{Scheme: "file", Path: path}).String()
			manifest := &seedbinding.Manifest{SchemaVersion: seedbinding.VersionV2, ID: "seed", Locator: seedbinding.Locator{URI: uri, Line: 0, Character: 1, Encoding: "utf-16"}, ExpectedSymbol: "leaf", ExpectedDeclaringFile: "seed.go", ExpectedDeclarationRange: seedbinding.Range{StartLine: 0, StartCharacter: 0, EndLine: 0, EndCharacter: 4}, SourceRevision: "revision", SourceSHA256: fmt.Sprintf("%x", sourceSum), Validator: seedbinding.ValidatorIdentity{Language: "go", Authority: "MANAGED_LSP", Name: "fake-lsp-fixture", Version: tc.providerVersion}}
			if tc.local {
				manifest.SchemaVersion = seedbinding.VersionV3
				manifest.CustodyMode = seedbinding.CallerAssertedLocal
			}
			execution := managedExecutionAuthority{Path: fake, Directory: workspace, Environment: append(os.Environ(), "LSP_TRACE_FAKE_LSP_TRACE="+trace, "LSP_TRACE_FAKE_LSP_DOCUMENT_SYMBOL="+tc.mode)}
			bindProviderIdentity(t, manifest, execution)
			manifestBytes, _ := json.Marshal(manifest)
			manifestSum := sha256.Sum256(manifestBytes)
			receipt := seedbinding.HostCustodyReceipt{Repository: workspace, SourceRevision: "revision", TargetPath: "seed.go", TargetSourceSHA256: manifest.SourceSHA256, SeedManifestSHA256: fmt.Sprintf("%x", manifestSum)}
			selector, trust := signedSeedTrust(t, receipt)
			custodySelector := selector
			var authority seedbinding.RevisionAuthority
			if !tc.local {
				authority = seedAuthoritiesFromConfig(bootstrapConfig{Processes: []bootstrapProcessConfig{{SeedBinding: manifest, SeedCustodySelector: selector}}}, trust)
			} else {
				custodySelector = ""
			}
			config := bootstrapConfig{Version: 1, Processes: []bootstrapProcessConfig{{Profile: bootstrapProfileIdentity{TrustDomain: "test", Workspace: workspace, Profile: "fake", EnvironmentReference: "test"}, Execution: execution, SeedBinding: manifest, SeedCustodySelector: custodySelector}}}
			_, manager, err := newServerRuntimeWithSeedAuthorities(false, provider.ConfiguredInventory{}, nil, authority)
			if err != nil {
				t.Fatal(err)
			}
			sessions, err := startBootstrap(context.Background(), manager, config, 5*time.Second)
			if err != nil {
				t.Fatal(err)
			}
			defer stopBootstrap(context.Background(), manager, sessions)
			deadline := time.Now().Add(100 * time.Millisecond)
			got := manager.AdmitSeedBinding(context.Background(), sessions[0].SessionID, sessions[0].Generation, deadline, 8, 1<<20)
			if got.Status != tc.want {
				t.Fatalf("ASSERT_REAL_SEED_%s_STATUS: got=%+v", strings.ToUpper(strings.ReplaceAll(tc.name, "-", "_")), got)
			}
			if got.Status == seedbinding.Match {
				result := manager.RoundTrip(context.Background(), sessionruntime.RoundTripRequest{SessionID: sessions[0].SessionID, Generation: sessions[0].Generation, Method: "textDocument/prepareCallHierarchy", Params: json.RawMessage(`{"textDocument":{"uri":"` + uri + `"},"position":{"line":0,"character":1}}`), Deadline: time.Now().Add(time.Second), MaxMessages: 8, MaxBytes: 1 << 20})
				if result.Failure != "" {
					t.Fatal(result.Failure)
				}
				restarted := manager.Restart(context.Background(), sessions[0].SessionID, "seed-restart-test")
				if restarted.Failure != "" {
					t.Fatal(restarted.Failure)
				}
				var fresh sessionruntime.Record
				until := time.Now().Add(5 * time.Second)
				for time.Now().Before(until) {
					for _, record := range manager.Records() {
						if record.SessionID == sessions[0].SessionID && record.Generation > sessions[0].Generation && record.State == "READY" {
							fresh = record
						}
					}
					if fresh.Generation != 0 {
						break
					}
					time.Sleep(10 * time.Millisecond)
				}
				if fresh.Generation == 0 {
					t.Fatal("ASSERT_REAL_RESTART_REACHES_FRESH_READY_GENERATION")
				}
				freshAdmission := manager.AdmitSeedBinding(context.Background(), fresh.SessionID, fresh.Generation, time.Now().Add(time.Second), 8, 1<<20)
				if freshAdmission.Status != seedbinding.Match {
					t.Fatalf("ASSERT_REAL_RESTART_REQUIRES_FRESH_ADMISSION: %+v", freshAdmission)
				}
			}
			raw, err := os.ReadFile(trace)
			if err != nil {
				t.Fatal(err)
			}
			events := strings.Fields(string(raw))
			wantSpawns := 1
			if tc.want == seedbinding.Match {
				wantSpawns = 2
			}
			if count(events, "spawn") != wantSpawns {
				t.Fatalf("ASSERT_REAL_ONE_PROVIDER_SPAWN_PER_GENERATION: %v", events)
			}
			if tc.want == seedbinding.Match && (count(events, "textDocument/didOpen") != 2 || count(events, "textDocument/documentSymbol") != 2) {
				t.Fatalf("ASSERT_REAL_RESTART_CANNOT_INHERIT_ADMISSION: %v", events)
			}
			prepare := index(events, "textDocument/prepareCallHierarchy")
			if tc.want == seedbinding.Match {
				for _, event := range []string{"initialize", "initialized", "textDocument/didOpen", "textDocument/documentSymbol", "textDocument/prepareCallHierarchy"} {
					if index(events, event) < 0 {
						t.Fatalf("ASSERT_REAL_LIFECYCLE_MISSING_%s: %v", event, events)
					}
				}
				if !(index(events, "initialize") < index(events, "initialized") && index(events, "initialized") < index(events, "textDocument/didOpen") && index(events, "textDocument/didOpen") < index(events, "textDocument/documentSymbol") && index(events, "textDocument/documentSymbol") < prepare) {
					t.Fatalf("ASSERT_REAL_SEMANTIC_BEFORE_PREPARE: %v", events)
				}
			} else if prepare >= 0 {
				t.Fatalf("ASSERT_REAL_REJECTED_SEED_NO_PREPARE: %v", events)
			}
			if tc.name == "provider-version-substitution" && (index(events, "initialize") < 0 || index(events, "textDocument/didOpen") >= 0 || index(events, "textDocument/documentSymbol") >= 0) {
				t.Fatalf("ASSERT_PROVIDER_MISMATCH_TERMINAL_BEFORE_SEMANTIC_REQUEST: %v", events)
			}
		})
	}
}

func count(values []string, target string) int {
	n := 0
	for _, value := range values {
		if value == target {
			n++
		}
	}
	return n
}
func index(values []string, target string) int {
	for i, value := range values {
		if value == target {
			return i
		}
	}
	return -1
}
