package adr0011methodresult

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"testing"
	"time"

	v5 "lsp-trace/internal/adr0011genericv5proposal"
	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/sessionruntime"
)

// V2 is private runtime validation, not public admission or qualification.
// The pinned oracle is independently transcribed from DECLARATION, request,
// response and target source originals; EXPECTATION.json is not an oracle.
const composedV2OracleSHA = "b9c52fb9176cc7d06f56c200e1f5546933acb8a0730fa3ebe0f2e7a5b566bb66"

type composedV2Case struct {
	Session           string               `json:"session"`
	Transaction       string               `json:"transaction"`
	CompletedOwnerKey string               `json:"completed_owner_key"`
	QueryOccurrenceID string               `json:"query_occurrence_id"`
	QueryURI          string               `json:"query_uri"`
	RequestID         uint64               `json:"request_id"`
	Generation        uint64               `json:"generation"`
	Method            string               `json:"method"`
	Line              uint64               `json:"line"`
	Character         uint64               `json:"character"`
	ResultForm        string               `json:"result_form"`
	CandidateCount    int                  `json:"candidate_count"`
	Targets           []bridgeOracleTarget `json:"targets"`
}

// Source-bound proof: these exact private implementations must retain the manager
// lease consumption, strict result parser, and unadmitted status ceiling. The manager
// pin includes accepted C13/C14 source accounting and the reviewed Manager/B4 portion
// of C15 explicit-byte accounting; it does not establish full C15, C16-C17, D/R
// occurrence, public admission, publication authority, or qualification. This older
// composed adapter is not source-custodied and does not establish real-server custody.
func TestADR0011PrivateComposedV2SourceBoundary(t *testing.T) {
	adapter := bridgePinnedBytes(t, "b4_definition_private_adapter.go", "ba853adc0d3045b4755c5ce4783f2f05ddc531007b69ab17ecfbf574f0d5ce99")
	bridge := bridgePinnedBytes(t, "b4_definition_bridge.go", "f22da69db0aad0078370387b43f973ec28a553557e88c4c641eb946640585be6")
	manager := bridgePinnedBytes(t, filepath.Join("..", "..", "sessionruntime", "b4_definition_private.go"), "3474e565ddd2857c5e699125300977304b5d0ec943213ac7315847ee8a503c4e")
	for _, removed := range []string{"func (m *Manager) PreparePrivateB4Definition(", "func (m *Manager) CommitPrivateB4Definition(", "func (m *Manager) ConsumePrivateB4Definition(", "includeResult bool", "uint64(len(capture.Result))", "capture.Result = append"} {
		if bytes.Contains(manager, []byte(removed)) {
			t.Errorf("SEMANTIC_RED_COMPATIBILITY_REMOVAL still present %q", removed)
		}
	}
	for _, proof := range []struct {
		label  string
		source []byte
		tokens []string
	}{
		{"adapter", adapter, []string{"manager.ConsumePrivateB4DefinitionBorrowed(lease, selection, func(b sessionruntime.PrivateB4DefinitionBorrow) bool", "privateB4Frame(capture.ResponseFrame)", "CheckB4DefinitionBridge(DefinitionBridgeInput", `Accepted: false, Completeness: "UNKNOWN", ClaimCeiling: "NO_PRODUCER_AUTHENTICATION"`}},
		{"bridge", bridge, []string{"parseRawUntrusted(w.Method, result, 1000)", "bridgeRangeWithinSource(source, item.Range)", "ProjectDefinitionCandidates("}},
		{"manager", manager, []string{"ConsumePrivateB4DefinitionBorrowed", "PreparePrivateB4DefinitionBorrowed", "PrivateB4Selected"}},
	} {
		for _, token := range proof.tokens {
			if !bytes.Contains(proof.source, []byte(token)) {
				t.Errorf("SEMANTIC_RED_SOURCE_BOUNDARY %s missing %q", proof.label, token)
			}
		}
	}
}

func composedV2Ceiling(t *testing.T, label string, got PrivateB4Decision) {
	t.Helper()
	if got.Authority != 0 || got.Accepted || got.Completeness != "UNKNOWN" || got.ClaimCeiling != "NO_PRODUCER_AUTHENTICATION" {
		t.Errorf("SEMANTIC_RED_CEILING %s: %+v", label, got)
	}
}

func TestADR0011PrivateComposedV2IndependentOracle(t *testing.T) {
	root := filepath.Join("testdata", "adr0011-composed-b4-manager-id1-held-v1")
	assets := bridgeManifestAssets(t, filepath.Join(root, "manifest.json"), composedManifestSHA)
	var oracle struct {
		ManifestSHA   string                    `json:"manifest_sha256"`
		Qualification string                    `json:"qualification"`
		Cases         map[string]composedV2Case `json:"cases"`
		Ceiling       struct {
			Authority    int
			Accepted     bool
			Completeness string
			ClaimCeiling string `json:"claim_ceiling"`
		} `json:"ceiling"`
	}
	oraclePath := filepath.Join("testdata", "adr0011-composed-private-b4-review-v2", "ORACLE.json")
	bridgeJSON(t, bridgePinnedBytes(t, oraclePath, composedV2OracleSHA), &oracle)
	if oracle.ManifestSHA != composedManifestSHA || oracle.Qualification != "0/162" || len(oracle.Cases) != 2 || oracle.Ceiling.Authority != 0 || oracle.Ceiling.Accepted || oracle.Ceiling.Completeness != "UNKNOWN" || oracle.Ceiling.ClaimCeiling != "NO_PRODUCER_AUTHENTICATION" {
		bridgeFixtureFatal(t, "BLOCKED_NOT_RED: V2 oracle envelope")
	}
	for _, name := range []string{"A", "B"} {
		t.Run(name, func(t *testing.T) {
			want := oracle.Cases[name]
			get := func(path string) []byte { return bridgeAssetBytes(t, root, name+"/"+path, assets) }
			// Independently bind the frozen expected identities and ranges to originals.
			var declaration struct {
				Session           string
				Transaction       string
				CompletedOwnerKey string `json:"completed_owner_key"`
				QueryOccurrenceID string `json:"query_occurrence_id"`
				URI               string
				Method            string
				Generation        uint64
				Line              uint64
				Character         uint64
			}
			bridgeJSON(t, get("DECLARATION.json"), &declaration)
			if declaration.Session != want.Session || declaration.Transaction != want.Transaction || declaration.CompletedOwnerKey != want.CompletedOwnerKey || declaration.QueryOccurrenceID != want.QueryOccurrenceID || declaration.URI != want.QueryURI || declaration.Method != want.Method || declaration.Generation != want.Generation || declaration.Line != want.Line || declaration.Character != want.Character {
				bridgeFixtureFatal(t, "BLOCKED_NOT_RED: %s independent declaration mismatch", name)
			}
			frame := bridgeFrame(t, get("request.frame"))
			if string(frame["id"]) != fmt.Sprint(want.RequestID) || string(frame["method"]) != `"textDocument/definition"` || !bytes.Equal(frame["params"], get("request.params")) {
				bridgeFixtureFatal(t, "BLOCKED_NOT_RED: %s request originals", name)
			}
			var params struct {
				Position     Position
				TextDocument struct{ URI string } `json:"textDocument"`
			}
			bridgeJSON(t, get("request.params"), &params)
			if params.TextDocument.URI != want.QueryURI || uint64(params.Position.Line) != want.Line || uint64(params.Position.Character) != want.Character {
				bridgeFixtureFatal(t, "BLOCKED_NOT_RED: %s query position", name)
			}
			var response struct {
				ID     json.RawMessage
				Result json.RawMessage
			}
			bridgeJSON(t, get("response.json"), &response)
			if string(response.ID) != fmt.Sprint(want.RequestID) || !bytes.Equal(response.Result, get("response.result")) {
				bridgeFixtureFatal(t, "BLOCKED_NOT_RED: %s response originals", name)
			}
			raw := bytes.TrimSpace(response.Result)
			if want.ResultForm == "scalar" && (len(raw) == 0 || raw[0] != '{') || want.ResultForm == "array" && (len(raw) == 0 || raw[0] != '[') {
				bridgeFixtureFatal(t, "BLOCKED_NOT_RED: %s result form", name)
			}
			var locations []struct {
				URI   string
				Range bridgeRange
			}
			if want.ResultForm == "scalar" {
				var one struct {
					URI   string
					Range bridgeRange
				}
				bridgeJSON(t, raw, &one)
				locations = append(locations, one)
			} else {
				bridgeJSON(t, raw, &locations)
			}
			if len(locations) != want.CandidateCount || len(want.Targets) != want.CandidateCount {
				bridgeFixtureFatal(t, "BLOCKED_NOT_RED: %s cardinality", name)
			}
			for i, target := range want.Targets {
				if target.Ordinal != i || target.Kind != "LOCATION" || target.TargetRange != nil || target.URI != locations[i].URI || target.SelectionRange != locations[i].Range || bridgeDigest(get(fmt.Sprintf("target-%c.go", 'a'+i))) != target.TargetSourceSHA {
					bridgeFixtureFatal(t, "BLOCKED_NOT_RED: %s independent target %d", name, i)
				}
				// Verify the byte-indexed Go identifier is the exact declared selection.
				lines := bytes.Split(get(fmt.Sprintf("target-%c.go", 'a'+i)), []byte("\n"))
				r := target.SelectionRange
				if r.Start.Line != r.End.Line || int(r.Start.Line) >= len(lines) || r.Start.Character >= r.End.Character || int(r.End.Character) > len(lines[r.Start.Line]) || !bytes.Contains(lines[r.Start.Line][r.Start.Character:r.End.Character], []byte("ZZ")) {
					bridgeFixtureFatal(t, "BLOCKED_NOT_RED: %s target source selection %d", name, i)
				}
			}
			in, occurrence, sources, _, buildChild, req, owner, profile := composedFixture(t, assets, root, name)
			if occurrence != want.QueryOccurrenceID || req.SessionID != want.Session || owner.Transaction != want.Transaction || owner.CompletedOwnerKey != want.CompletedOwnerKey {
				bridgeFixtureFatal(t, "BLOCKED_NOT_RED: %s helper identity", name)
			}
			t.Run("manager-oracle", func(t *testing.T) {
				composedV2Run(t, name, in, occurrence, sources, want, buildChild(), req, owner, profile, "valid")
			})
			t.Run("substituted-source", func(t *testing.T) {
				altered := make(map[string][]byte, len(sources))
				for k, v := range sources {
					altered[k] = v
				}
				altered[want.Targets[0].URI] = []byte("package other\n")
				composedV2Run(t, name, in, occurrence, altered, want, buildChild(), req, owner, profile, "substituted-source")
			})
			t.Run("mismatched-replay", func(t *testing.T) {
				changed := in
				changed.Write = in.Write
				changed.Write.RequestParams = []byte(`{}`)
				composedV2Run(t, name, changed, occurrence, sources, want, buildChild(), req, owner, profile, "mismatched-replay")
			})
			t.Run("malformed-result", func(t *testing.T) {
				bad := []byte(`{"jsonrpc":"2.0","id":1,"result":[{"uri":"file:///w/target-a.go","range":{"start":{"line":1,"character":5},"end":{"line":1,"character":12}}},false]}`)
				wire := []byte(fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(bad), bad))
				input, stdin := io.Pipe()
				stdout, output := io.Pipe()
				child := &composedChild{input: input, stdin: stdin, output: output, stdout: stdout, observed: make(chan error, 1)}
				go func() {
					reader := lspwire.NewReader(input, lspwire.DefaultLimits())
					if err := composedServeReadiness(reader, output); err != nil {
						child.observed <- err
						return
					}
					_, _, captured, retained, err := reader.ReadWithFrameIfWithin(4096)
					if err != nil || !retained || !bytes.Equal(captured, in.Write.RequestFrame) {
						child.observed <- fmt.Errorf("exact WRITE: %v", err)
						return
					}
					_, err = output.Write(wire)
					child.observed <- err
				}()
				changed := req
				changed.CaptureDefinitionResponseFrameMaxBytes = int64(len(wire))
				composedV2Run(t, name, in, occurrence, sources, want, child, changed, owner, profile, "malformed-result")
			})
		})
	}
}

func composedV2Run(t *testing.T, name string, in v5.B4bFullCandidateInput, occurrence string, sources map[string][]byte, want composedV2Case, child *composedChild, req sessionruntime.RoundTripRequest, owner sessionruntime.B4DefinitionOwner, profile runtimeprofile.Selector, mode string) {
	t.Helper()
	validated, err := runtimeprofile.Validate(profile)
	if err != nil {
		bridgeFixtureFatal(t, "BLOCKED_NOT_RED: %s profile: %v", name, err)
	}
	manager, err := sessionruntime.New(sessionruntime.Config{Limits: sessionruntime.Limits{MaxSessions: 1, MaxRequests: 1, MaxChildren: 2, MaxCancels: 2, MaxTombstones: 4, MaxObservations: 64, MaxOperations: 2}, Starter: composedStarter{child}})
	if err != nil {
		bridgeFixtureFatal(t, "BLOCKED_NOT_RED: %s manager: %v", name, err)
	}
	t.Cleanup(func() {
		_ = child.Teardown(context.Background())
		_ = child.Close()
		_ = manager.Shutdown(context.Background())
	})
	started := manager.Start(context.Background(), sessionruntime.StartRequest{Profile: runtimeprofile.Resolve(validated)})
	if started.SessionID != req.SessionID || started.Generation != req.Generation {
		bridgeFixtureFatal(t, "BLOCKED_NOT_RED: %s identity", name)
	}
	composedRequireReadiness(t, manager, started)
	result, lease := manager.RoundTripPrivateB4(context.Background(), req, owner)
	if result.Failure != "" || result.ServerError != nil || lease == (sessionruntime.B4DefinitionLease{}) || result.Key != (lspwire.RequestKey{Generation: 1, ID: 1}) {
		bridgeFixtureFatal(t, "BLOCKED_NOT_RED: %s manager capture failure=%s", name, result.Failure)
	}
	select {
	case err := <-child.observed:
		if err != nil {
			bridgeFixtureFatal(t, "BLOCKED_NOT_RED: %s IO: %v", name, err)
		}
	case <-time.After(3 * time.Second):
		bridgeFixtureFatal(t, "BLOCKED_NOT_RED: %s IO timeout", name)
	}
	selection := sessionruntime.B4DefinitionSelectionKey{SessionID: started.SessionID, Key: result.Key, Transaction: owner.Transaction, CompletedOwnerKey: owner.CompletedOwnerKey}
	decision := CheckPrivateComposedB4Definition(manager, lease, selection, in, occurrence, sources)
	composedV2Ceiling(t, name+"/"+mode, decision)
	if mode == "valid" {
		if decision.Status != DefinitionBridgeCandidateItems || decision.ChronologyTerminal != "SUPPORTED" || len(decision.Candidates) != want.CandidateCount {
			t.Fatalf("SEMANTIC_RED_ORACLE %s: status=%s chronology=%s count=%d", name, decision.Status, decision.ChronologyTerminal, len(decision.Candidates))
		}
		for i, target := range want.Targets {
			if !composedTargetEqual(decision.Candidates[i], target) || decision.Candidates[i].QueryOccurrenceID != want.QueryOccurrenceID {
				t.Errorf("SEMANTIC_RED_ORACLE %s target[%d]: %+v", name, i, decision.Candidates[i])
			}
		}
	} else if decision.Status == DefinitionBridgeCandidateItems || len(decision.Candidates) != 0 {
		t.Errorf("SEMANTIC_RED_REJECTION %s/%s: status=%s candidates=%d", name, mode, decision.Status, len(decision.Candidates))
	}
	if mode == "malformed-result" && decision.Status != DefinitionBridgeMalformedResult {
		t.Errorf("SEMANTIC_RED_MALFORMED %s: %s", name, decision.Status)
	}
	if mode == "mismatched-replay" && decision.Status != DefinitionBridgeCorrespondenceInvalid {
		t.Errorf("SEMANTIC_RED_MISMATCH %s: %s", name, decision.Status)
	}
	if mode == "substituted-source" && decision.Status != DefinitionBridgeMalformedResult {
		t.Errorf("SEMANTIC_RED_SUBSTITUTE %s: %s", name, decision.Status)
	}
}
