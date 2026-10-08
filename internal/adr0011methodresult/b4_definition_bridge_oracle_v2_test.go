package adr0011methodresult

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	v5 "lsp-trace/internal/adr0011genericv5proposal"
)

// This private synthetic guard is intentionally test-first and has no observed
// server-return, exclusion, admission, authentication, or qualification authority.
const bridgeOracleSHA = "d82cf78ae7a8c024d22a5eac3b31e1a3c8e5a8f6dfc9c26630f97f5296432b59"

type bridgePin struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type bridgeAsset struct {
	bridgePin
	Filename  string `json:"filename"`
	Length    int    `json:"length"`
	Direction string `json:"direction"`
	Ordinal   uint64 `json:"ordinal"`
}

type bridgeRange struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

type bridgeOracleTarget struct {
	Ordinal         int          `json:"ordinal"`
	Kind            string       `json:"kind"`
	URI             string       `json:"uri"`
	SelectionRange  bridgeRange  `json:"selection_range"`
	TargetRange     *bridgeRange `json:"target_range"`
	TargetSourceSHA string       `json:"target_source_sha256"`
}

type bridgeOracleRow struct {
	Scenario           string               `json:"scenario"`
	WriteOrdinal       uint64               `json:"write_ordinal"`
	ResponseSHA        string               `json:"response_sha256"`
	ResultForm         string               `json:"result_form"`
	CandidateCount     int                  `json:"candidate_count"`
	ChronologyTerminal string               `json:"chronology_terminal"`
	BridgeStatus       string               `json:"bridge_status"`
	Targets            []bridgeOracleTarget `json:"targets"`
}

func bridgeFixtureFatal(t *testing.T, format string, args ...any) {
	t.Helper()
	t.Fatalf("fixture precondition: "+format, args...)
}

func bridgePinnedBytes(t *testing.T, path, digest string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		bridgeFixtureFatal(t, "read %s: %v", path, err)
	}
	got := sha256.Sum256(b)
	if hex.EncodeToString(got[:]) != digest {
		bridgeFixtureFatal(t, "SHA256 %s: got %x want %s", path, got, digest)
	}
	return b
}

func bridgeJSON(t *testing.T, b []byte, out any) {
	t.Helper()
	if err := json.Unmarshal(b, out); err != nil {
		bridgeFixtureFatal(t, "decode held JSON: %v", err)
	}
}

func bridgeFrame(t *testing.T, frame []byte) map[string]json.RawMessage {
	t.Helper()
	header, body, found := bytes.Cut(frame, []byte("\r\n\r\n"))
	if !found || !bytes.Contains(bytes.ToLower(header), []byte("content-length:")) {
		bridgeFixtureFatal(t, "held request is not a framed message")
	}
	var message map[string]json.RawMessage
	bridgeJSON(t, body, &message)
	if len(message["id"]) == 0 || len(message["params"]) == 0 {
		bridgeFixtureFatal(t, "held request lacks id or params")
	}
	return message
}

func bridgeManifestAssets(t *testing.T, path, digest string) map[string]bridgeAsset {
	t.Helper()
	var manifest struct {
		Assets []bridgeAsset `json:"assets"`
	}
	bridgeJSON(t, bridgePinnedBytes(t, path, digest), &manifest)
	if len(manifest.Assets) == 0 {
		bridgeFixtureFatal(t, "empty asset manifest %s", path)
	}
	out := make(map[string]bridgeAsset, len(manifest.Assets))
	for _, asset := range manifest.Assets {
		key := asset.Path
		if key == "" {
			key = asset.Filename
		}
		if key == "" || asset.SHA256 == "" {
			bridgeFixtureFatal(t, "incomplete asset in %s", path)
		}
		out[key] = asset
	}
	return out
}

func bridgeAssetBytes(t *testing.T, root, name string, assets map[string]bridgeAsset) []byte {
	t.Helper()
	a, ok := assets[name]
	if !ok {
		bridgeFixtureFatal(t, "missing pinned asset %s", filepath.Join(root, name))
	}
	b := bridgePinnedBytes(t, filepath.Join(root, name), a.SHA256)
	if len(b) != a.Length {
		bridgeFixtureFatal(t, "pinned asset length %s: got %d want %d", name, len(b), a.Length)
	}
	return b
}

func bridgeFixture(t *testing.T, evidence string, row bridgeOracleRow, responseAssets map[string]bridgeAsset) DefinitionBridgeInput {
	t.Helper()
	rawRoot := filepath.Join(evidence, "adr0011-definition-b4b-held-originals-v1")
	queryRoot := filepath.Join(evidence, "adr0011-definition-b4b-held-query-originals-v1")
	claimsRoot := filepath.Join(evidence, "adr0011-definition-b4b-b4a-derived-v1")
	capRoot := filepath.Join(evidence, "adr0011-definition-b4b-capability-derived-v1")
	responseRoot := filepath.Join(evidence, "adr0011-definition-response-held-originals-v1-corrected")
	write := row.Scenario + "/WRITE" + strconv.FormatUint(row.WriteOrdinal, 10)

	var rawManifest struct {
		Scenarios []bridgePin `json:"scenarios"`
	}
	bridgeJSON(t, bridgePinnedBytes(t, filepath.Join(rawRoot, "manifest.json"), "2fb4c68376f5268b0f1722033565c70595545aa3df08e9f3431ecbd9762f99fe"), &rawManifest)
	var scenarioDigest string
	for _, s := range rawManifest.Scenarios {
		if s.Path == row.Scenario+"/manifest.json" {
			scenarioDigest = s.SHA256
		}
	}
	if scenarioDigest == "" {
		bridgeFixtureFatal(t, "missing scenario manifest %s", row.Scenario)
	}
	scenarioRoot := filepath.Join(rawRoot, row.Scenario)
	assets := bridgeManifestAssets(t, filepath.Join(scenarioRoot, "manifest.json"), scenarioDigest)
	var frames []v5.B4Frame
	var request, source, selector []byte
	for _, a := range assets {
		data := bridgeAssetBytes(t, scenarioRoot, a.Filename, assets)
		switch a.Filename {
		case "source.bytes":
			source = data
		case "client-selector.json":
			selector = data
		default:
			if strings.HasSuffix(a.Filename, ".frame") {
				frames = append(frames, v5.B4Frame{Ordinal: a.Ordinal, Direction: a.Direction, Bytes: data})
				if a.Ordinal == row.WriteOrdinal {
					request = data
				}
			}
		}
	}
	if len(request) == 0 || len(source) == 0 || len(selector) == 0 || len(frames) == 0 {
		bridgeFixtureFatal(t, "incomplete raw stream for %s", write)
	}
	// A map iteration does not define chronological order; the replay does.
	for i := 0; i < len(frames); i++ {
		for j := i + 1; j < len(frames); j++ {
			if frames[j].Ordinal < frames[i].Ordinal {
				frames[i], frames[j] = frames[j], frames[i]
			}
		}
	}
	var ctx struct {
		Session, Workspace, URI, Method string
		PositionEncoding                string `json:"position_encoding"`
		Generation                      uint64
		Language                        struct{ ID string }
		Writes                          []struct {
			Ordinal     uint64
			Transaction string
			OwnerKey    string `json:"owner_key"`
		}
	}
	bridgeJSON(t, bridgeAssetBytes(t, scenarioRoot, "context.json", assets), &ctx)
	var transaction, ownerKey string
	for _, w := range ctx.Writes {
		if w.Ordinal == row.WriteOrdinal {
			transaction, ownerKey = w.Transaction, w.OwnerKey
		}
	}
	if transaction == "" || ownerKey == "" || ctx.Method != "textDocument/definition" || ctx.Language.ID != "go" {
		bridgeFixtureFatal(t, "held context does not identify %s as definition WRITE", write)
	}
	var bindings struct {
		Bindings []struct {
			Scenario     string `json:"scenario"`
			WriteOrdinal uint64 `json:"write_ordinal"`
			Originals    struct {
				Query              bridgePin `json:"query"`
				QueryApplicability bridgePin `json:"query_applicability"`
			} `json:"originals"`
		} `json:"bindings"`
	}
	queryAssets := bridgeManifestAssets(t, filepath.Join(queryRoot, "manifest.json"), "f4b765e5576f6ed22c839e2686116146db4d643cd53eef8af851757e824c7001")
	bridgeJSON(t, bridgeAssetBytes(t, queryRoot, "bindings.json", queryAssets), &bindings)
	var query, app []byte
	for _, binding := range bindings.Bindings {
		if binding.Scenario == row.Scenario && binding.WriteOrdinal == row.WriteOrdinal {
			for _, pin := range []bridgePin{binding.Originals.Query, binding.Originals.QueryApplicability} {
				a, ok := queryAssets[pin.Path]
				if !ok || a.SHA256 != pin.SHA256 {
					bridgeFixtureFatal(t, "held query binding not pinned for %s", write)
				}
			}
			query = bridgeAssetBytes(t, queryRoot, binding.Originals.Query.Path, queryAssets)
			app = bridgeAssetBytes(t, queryRoot, binding.Originals.QueryApplicability.Path, queryAssets)
		}
	}
	if len(query) == 0 || len(app) == 0 {
		bridgeFixtureFatal(t, "missing held QUERY/APP for %s", write)
	}
	message := bridgeFrame(t, request)
	var response struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
		Result  json.RawMessage `json:"result"`
	}
	responsePath := write + "/response.json"
	responseFrame := bridgeAssetBytes(t, responseRoot, responsePath, responseAssets)
	if responseAssets[responsePath].SHA256 != row.ResponseSHA {
		bridgeFixtureFatal(t, "oracle response digest disagrees with corrected held asset for %s", write)
	}
	bridgeJSON(t, responseFrame, &response)
	var requestID, responseID json.Number
	if err := json.Unmarshal(message["id"], &requestID); err != nil {
		bridgeFixtureFatal(t, "request ID is not typed numeric for %s: %v", write, err)
	}
	if len(message["id"]) == 0 || message["id"][0] == '"' || len(response.ID) == 0 || response.ID[0] == '"' {
		bridgeFixtureFatal(t, "held request/response ID is not numeric for %s", write)
	}
	if err := json.Unmarshal(response.ID, &responseID); err != nil || requestID != responseID || response.JSONRPC != "2.0" {
		bridgeFixtureFatal(t, "typed numeric response ID does not match WRITE %s", write)
	}
	// Corroboration only: response bytes never generate an expected oracle target.
	if row.ResultForm == "NULL" && string(response.Result) != "null" {
		bridgeFixtureFatal(t, "oracle NULL disagrees with held response %s", write)
	}
	if row.ResultForm == "LOCATION" && (len(response.Result) == 0 || response.Result[0] != '{') {
		bridgeFixtureFatal(t, "oracle scalar form disagrees with held response %s", write)
	}
	if (row.ResultForm == "LOCATION_ARRAY" || row.ResultForm == "LOCATION_LINK_ARRAY" || row.ResultForm == "INVALID") && (len(response.Result) == 0 || response.Result[0] != '[') {
		bridgeFixtureFatal(t, "oracle array form disagrees with held response %s", write)
	}
	// Corroborate held response coordinates against the independently pinned oracle;
	// never construct expected candidates from these response members.
	if row.ResultForm != "NULL" && row.ResultForm != "INVALID" {
		var members []json.RawMessage
		if row.ResultForm == "LOCATION" {
			members = []json.RawMessage{response.Result}
		} else {
			bridgeJSON(t, response.Result, &members)
		}
		if len(members) != len(row.Targets) {
			bridgeFixtureFatal(t, "oracle/response member count for %s", write)
		}
		for i, raw := range members {
			var wire struct {
				URI, TargetURI                           string
				Range, TargetRange, TargetSelectionRange bridgeRange
			}
			bridgeJSON(t, raw, &wire)
			want := row.Targets[i]
			if want.Ordinal != i {
				bridgeFixtureFatal(t, "oracle ordinal %d for %s", i, write)
			}
			if want.Kind == "LOCATION" {
				if wire.URI != want.URI || wire.Range != want.SelectionRange || want.TargetRange != nil {
					bridgeFixtureFatal(t, "oracle/response Location %d for %s", i, write)
				}
			} else if want.Kind == "LOCATION_LINK" {
				if want.TargetRange == nil || wire.TargetURI != want.URI || wire.TargetSelectionRange != want.SelectionRange || wire.TargetRange != *want.TargetRange {
					bridgeFixtureFatal(t, "oracle/response LocationLink %d for %s", i, write)
				}
			} else {
				bridgeFixtureFatal(t, "unknown oracle kind %q for %s", want.Kind, write)
			}
		}
	}
	claimAssets := bridgeManifestAssets(t, filepath.Join(claimsRoot, "manifest.json"), "a88290ac884b4f0662d3c5ea5a07ea8031ce540fd580a866ab304d8b4bfde00c")
	capAssets := bridgeManifestAssets(t, filepath.Join(capRoot, "manifest.json"), "8e319123c02f66e9e584d16db976dfc47f488f0156903fdc571b3f2b94790d16")
	lang := ctx.Language.ID
	replay := v5.B4bFullCandidateInput{Write: v5.B4aInput{
		Session: ctx.Session, Generation: ctx.Generation, Transaction: transaction,
		Workspace: ctx.Workspace, Method: ctx.Method, URI: ctx.URI, Version: "buffer:v1",
		Encoding: ctx.PositionEncoding, Line: 1, Character: 5, Source: source,
		QueryOriginal: query, ApplicabilityOriginal: app, Custody: "OWNER_BUFFER", Language: &lang,
		RequestFrame: request, RequestParams: message["params"], RequestID: message["id"],
		CompletedKey: ownerKey, CompletedOrdinal: row.WriteOrdinal, WriteCompleted: true,
	}, Frames: frames, HeldClientSelector: selector}
	for _, role := range []struct {
		name, code string
		original   []byte
	}{
		{"source-envelope.json", "SOURCE", source},
		{"query-envelope.json", "QUERY", query},
		{"query-applicability-envelope.json", "QUERY_APPLICABILITY", app},
		{"request-write-envelope.json", "REQUEST_WRITE", request},
	} {
		envelope := bridgeAssetBytes(t, claimsRoot, write+"/"+role.name, claimAssets)
		artifact := role.original
		if role.code == "SOURCE" {
			artifact = bridgeAssetBytes(t, claimsRoot, write+"/source-artifact.bin", claimAssets)
		}
		replay.Write.Claims = append(replay.Write.Claims, v5.B4aClaim{Role: role.code, Envelope: envelope, Original: role.original, Artifact: artifact})
	}
	if err := v5.CheckB4a(replay.Write); err != nil {
		bridgeFixtureFatal(t, "accepted B4a originals invalid for %s: %v", write, err)
	}
	replay.CapabilityEnvelope = bridgeAssetBytes(t, capRoot, write+"/capability-envelope.json", capAssets)
	replay.CapabilityOriginal = bridgeAssetBytes(t, capRoot, write+"/capability-artifact.bin", capAssets)
	if err := v5.A4Validate("CAPABILITY_EVENTS", replay.CapabilityEnvelope); err != nil {
		bridgeFixtureFatal(t, "held CAP envelope invalid for %s: %v", write, err)
	}
	targetSources := make(map[string][]byte)
	for uri, name := range map[string]string{"file:///w/target-a.go": "targets/target-a.go", "file:///w/target-b.go": "targets/target-b.go"} {
		targetSources[uri] = bridgeAssetBytes(t, responseRoot, name, responseAssets)
	}
	return DefinitionBridgeInput{Replay: replay, ResponseFrame: responseFrame,
		QueryOccurrenceID: fmt.Sprintf("definition-bridge:%s:%d", row.Scenario, row.WriteOrdinal), TargetSources: targetSources}
}

func bridgeAssertNoPositiveExclusion(t *testing.T, label string, got DefinitionBridgeResult) {
	t.Helper()
	if string(got.Status) == "EXCLUDED_VERIFIED" {
		t.Errorf("%s: semantic assertion: positive exclusion without ordinal witness", label)
	}
}

func TestB4DefinitionBridgeIndependentOracleV2(t *testing.T) {
	evidence := "testdata"
	oracleRoot := filepath.Join("testdata", "adr0011-definition-bridge-independent-oracle-v1")
	bridgePinnedBytes(t, filepath.Join(oracleRoot, "manifest.json"), "377b3c25bc7dec58c28e10e853c038e09970b4c262443de91afe136c475c2c5a")
	var oracle struct {
		Rows []bridgeOracleRow `json:"rows"`
	}
	bridgeJSON(t, bridgePinnedBytes(t, filepath.Join(oracleRoot, "oracle.json"), bridgeOracleSHA), &oracle)
	if len(oracle.Rows) != 8 {
		bridgeFixtureFatal(t, "independent oracle has %d rows, want eight", len(oracle.Rows))
	}
	responseRoot := filepath.Join(evidence, "adr0011-definition-response-held-originals-v1-corrected")
	responseAssets := bridgeManifestAssets(t, filepath.Join(responseRoot, "manifest.json"), "2621c234bd4aef4e7fc2e190b9672c89faa1ebada39aac9d0543ff91b804f1d7")
	seen := map[string]bool{}
	for _, row := range oracle.Rows {
		row := row
		name := row.Scenario + "/WRITE" + strconv.FormatUint(row.WriteOrdinal, 10)
		if seen[name] {
			bridgeFixtureFatal(t, "duplicate oracle row %s", name)
		}
		seen[name] = true
		t.Run(name, func(t *testing.T) {
			in := bridgeFixture(t, evidence, row, responseAssets)
			if len(row.Targets) != row.CandidateCount || (row.ResultForm == "INVALID" || row.ResultForm == "NULL") && len(row.Targets) != 0 {
				bridgeFixtureFatal(t, "oracle count/form/target contradiction in %s", name)
			}
			got := CheckB4DefinitionBridge(in) // No test-side candidate or ledger construction.
			if string(got.Status) != row.BridgeStatus {
				t.Errorf("semantic assertion status: got %q want %q", got.Status, row.BridgeStatus)
			}
			if got.ChronologyTerminal != row.ChronologyTerminal {
				t.Errorf("semantic assertion chronology: got %q want %q", got.ChronologyTerminal, row.ChronologyTerminal)
			}
			if len(got.Candidates) != row.CandidateCount {
				t.Errorf("semantic assertion candidate_count: got %d want %d", len(got.Candidates), row.CandidateCount)
			}
			bridgeAssertNoPositiveExclusion(t, name, got)
			for i, want := range row.Targets {
				if i >= len(got.Candidates) {
					break
				}
				c := got.Candidates[i]
				if c.Ordinal != want.Ordinal || c.TargetURI != want.URI || string(c.TargetKind) != want.Kind ||
					c.TargetSelectionRange != (Range{Start: want.SelectionRange.Start, End: want.SelectionRange.End}) {
					t.Errorf("semantic assertion ordered target[%d]: got ordinal=%d URI=%q kind=%q selection=%+v want %+v", i, c.Ordinal, c.TargetURI, c.TargetKind, c.TargetSelectionRange, want)
				}
				if want.TargetRange == nil {
					if c.TargetRange != nil {
						t.Errorf("semantic assertion targetRange[%d]: got %+v want nil", i, c.TargetRange)
					}
				} else if c.TargetRange == nil || *c.TargetRange != (Range{Start: want.TargetRange.Start, End: want.TargetRange.End}) {
					t.Errorf("semantic assertion targetRange[%d]: got %+v want %+v", i, c.TargetRange, want.TargetRange)
				}
				b, ok := in.TargetSources[c.TargetURI]
				if !ok {
					t.Errorf("semantic assertion source[%d]: unknown URI %q", i, c.TargetURI)
				} else {
					digest := sha256.Sum256(b)
					if hex.EncodeToString(digest[:]) != want.TargetSourceSHA {
						t.Errorf("semantic assertion source digest[%d]: got %x want %s", i, digest, want.TargetSourceSHA)
					}
				}
			}
			if row.Scenario == "CORE" && row.WriteOrdinal == 10 && len(got.Candidates) == 3 {
				a, b := got.Candidates[0], got.Candidates[1]
				if a.Ordinal == b.Ordinal || a.OccurrenceID == b.OccurrenceID || a.TargetID == "" || a.TargetID != b.TargetID {
					t.Errorf("semantic assertion duplicate ordinals/identity: %+v %+v", a, b)
				}
			}
			if row.Scenario == "CORE" && row.WriteOrdinal == 6 {
				for _, variant := range []struct {
					name  string
					alter func(*DefinitionBridgeInput)
				}{
					{"SOURCE", func(v *DefinitionBridgeInput) {
						v.Replay.Write.Source = append([]byte(nil), v.Replay.Write.Source...)
						v.Replay.Write.Source[0] ^= 1
					}},
					{"QUERY", func(v *DefinitionBridgeInput) {
						v.Replay.Write.QueryOriginal = append([]byte(nil), v.Replay.Write.QueryOriginal...)
						v.Replay.Write.QueryOriginal[0] ^= 1
					}},
					{"transaction", func(v *DefinitionBridgeInput) { v.Replay.Write.Transaction += "-substituted" }},
					{"references", func(v *DefinitionBridgeInput) { v.Replay.Write.Method = "textDocument/references" }},
					{"typed-response-ID", func(v *DefinitionBridgeInput) {
						v.ResponseFrame = bytes.Replace(v.ResponseFrame, []byte(`"id":141`), []byte(`"id":"141"`), 1)
					}},
				} {
					t.Run(variant.name, func(t *testing.T) {
						changed := in
						variant.alter(&changed)
						if variant.name == "typed-response-ID" && bytes.Equal(changed.ResponseFrame, in.ResponseFrame) {
							bridgeFixtureFatal(t, "held response lacks numeric ID substitution site")
						}
						negative := CheckB4DefinitionBridge(changed)
						if len(negative.Candidates) != 0 {
							t.Errorf("semantic assertion %s substitution: %d candidates, want zero", variant.name, len(negative.Candidates))
						}
						bridgeAssertNoPositiveExclusion(t, variant.name, negative)
					})
				}
			}
		})
	}
}
