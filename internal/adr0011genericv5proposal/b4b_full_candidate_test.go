package adr0011genericv5proposal

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func b4bFullPinnedRead(t *testing.T, path, digest string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	h := sha256.Sum256(b)
	if hex.EncodeToString(h[:]) != digest {
		t.Fatalf("pin changed %s", path)
	}
	return b
}

func TestB4bFullCandidatePinnedMatrix(t *testing.T) {
	root := "testdata"
	raw := filepath.Join(root, "adr0011-b4b-multievent-held-originals-v1")
	held := filepath.Join(root, "adr0011-b4b-multievent-held-query-originals-v1")
	capRoot := filepath.Join(root, "adr0011-b4b-multievent-capability-derived-v1")
	pilot := filepath.Join(root, "adr0011-b4b-multievent-b4a-core-write4-v1")
	remaining := filepath.Join(root, "adr0011-b4b-multievent-b4a-remaining-v1")
	for _, pin := range []struct{ path, digest string }{
		{filepath.Join(raw, "manifest.json"), "f2984fd734c6ac81fe3d28ce59494a0ace1c19df046c51246dfaeaa75c762a8d"},
		{filepath.Join(root, "adr0011-b4b-multievent-ledger-correction-v1", "manifest.json"), "4a9db86b72e83df7cc2d0390794bcd56135e4cd83620e09ff8d125de4434e474"},
		{filepath.Join(held, "manifest.json"), "eb8b85d49163fb099f9347fc660e8ca7f2cae171ab8917f7d839aa4daea9a034"},
		{filepath.Join(capRoot, "manifest.json"), "4e951a0bc247b40b9c5e81c2015d9778a2c06287c391a332d129a9a53f6f3d1a"},
		{filepath.Join(pilot, "manifest.json"), "06a7de788395cbc96987a56d3f1bcdde944e9feaf2c8d8e63c22cc6334609cbf"},
		{filepath.Join(remaining, "manifest.json"), "2f8f3c56d3314e4714f9e94959013354eafbcf7545a1f386c9f78fcfc2c1933b"},
	} {
		b4bFullPinnedRead(t, pin.path, pin.digest)
	}
	cases := []struct {
		scenario string
		ordinal  uint64
		want     string
	}{
		{"CORE", 4, "UNKNOWN"}, {"CORE", 6, "SUPPORTED"},
		{"CORE", 8, "SUPPORTED"}, {"CORE", 10, "SUPPORTED"},
		{"CORE", 12, "SUPPORTED"}, {"CORE", 14, "UNSUPPORTED"},
		{"STATIC_VALID_CONFLICT", 20, "INVALID_CHRONOLOGY"},
		{"STATIC_MALFORMED_PRECEDENCE", 26, "MALFORMED"},
	}
	for _, tc := range cases {
		t.Run(tc.scenario+"/WRITE"+strconv.FormatUint(tc.ordinal, 10), func(t *testing.T) {
			scenarioDir := filepath.Join(raw, tc.scenario)
			var original struct {
				Assets []struct {
					Filename, SHA256, Direction string
					Ordinal                     uint64
				} `json:"assets"`
			}
			rootManifest := map[string]any{}
			if err := json.Unmarshal(b4bFullPinnedRead(t, filepath.Join(raw, "manifest.json"), "f2984fd734c6ac81fe3d28ce59494a0ace1c19df046c51246dfaeaa75c762a8d"), &rootManifest); err != nil {
				t.Fatal(err)
			}
			for _, entry := range rootManifest["scenarios"].([]any) {
				row := entry.(map[string]any)
				if row["path"] == tc.scenario+"/manifest.json" {
					if err := json.Unmarshal(b4bFullPinnedRead(t, filepath.Join(scenarioDir, "manifest.json"), row["sha256"].(string)), &original); err != nil {
						t.Fatal(err)
					}
				}
			}
			if len(original.Assets) == 0 {
				t.Fatal("missing pinned raw scenario")
			}
			var frames []B4Frame
			var source, selector, request []byte
			for _, asset := range original.Assets {
				data := b4bFullPinnedRead(t, filepath.Join(scenarioDir, asset.Filename), asset.SHA256)
				switch asset.Filename {
				case "source.bytes":
					source = data
				case "client-selector.json":
					selector = data
				default:
					if strings.HasSuffix(asset.Filename, ".frame") {
						frames = append(frames, B4Frame{Ordinal: asset.Ordinal, Direction: asset.Direction, Bytes: data})
						if asset.Ordinal == tc.ordinal {
							request = data
						}
					}
				}
			}
			if len(frames) == 0 || len(source) == 0 || len(request) == 0 {
				t.Fatal("missing independently held stream")
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
			// The pinned raw scenario manifest fixes context.json as an asset too.
			contextBytes, err := os.ReadFile(filepath.Join(scenarioDir, "context.json"))
			if err != nil || json.Unmarshal(contextBytes, &ctx) != nil {
				t.Fatal("held context")
			}
			var transaction, ownerKey string
			for _, w := range ctx.Writes {
				if w.Ordinal == tc.ordinal {
					transaction, ownerKey = w.Transaction, w.OwnerKey
				}
			}
			if transaction == "" || ownerKey == "" || ctx.Language.ID != "go" {
				t.Fatal("held identity")
			}
			var heldBindings struct {
				Bindings []struct {
					Scenario     string `json:"scenario"`
					WriteOrdinal uint64 `json:"write_ordinal"`
					Originals    struct {
						Query              struct{ Path, SHA256 string }
						QueryApplicability struct {
							Path   string `json:"path"`
							SHA256 string `json:"sha256"`
						} `json:"query_applicability"`
					} `json:"originals"`
				} `json:"bindings"`
			}
			bindingsBytes, err := os.ReadFile(filepath.Join(held, "bindings.json"))
			if err != nil || json.Unmarshal(bindingsBytes, &heldBindings) != nil {
				t.Fatal("held bindings")
			}
			var query, app []byte
			for _, binding := range heldBindings.Bindings {
				if binding.Scenario == tc.scenario && binding.WriteOrdinal == tc.ordinal {
					query = b4bFullPinnedRead(t, filepath.Join(held, binding.Originals.Query.Path), binding.Originals.Query.SHA256)
					app = b4bFullPinnedRead(t, filepath.Join(held, binding.Originals.QueryApplicability.Path), binding.Originals.QueryApplicability.SHA256)
				}
			}
			if len(query) == 0 || len(app) == 0 {
				t.Fatal("held query/app")
			}
			claimDir := pilot
			if tc.ordinal != 4 {
				claimDir = filepath.Join(remaining, tc.scenario, "WRITE"+strconv.FormatUint(tc.ordinal, 10))
			}
			message, err := b4aFrame(request)
			if err != nil {
				t.Fatal(err)
			}
			lang := ctx.Language.ID
			in := B4bFullCandidateInput{Write: B4aInput{
				Session: ctx.Session, Generation: ctx.Generation, Transaction: transaction,
				Workspace: ctx.Workspace, Method: ctx.Method, URI: ctx.URI, Version: "buffer:v1",
				Encoding: ctx.PositionEncoding, Source: source, QueryOriginal: query,
				ApplicabilityOriginal: app, Custody: "OWNER_BUFFER", Language: &lang,
				RequestFrame: request, RequestParams: message["params"], RequestID: message["id"],
				CompletedKey: ownerKey, CompletedOrdinal: tc.ordinal, WriteCompleted: true,
			}, Frames: frames, HeldClientSelector: selector}
			for _, role := range []struct {
				name, code string
				original   []byte
			}{
				{"source-envelope.json", "SOURCE", source}, {"query-envelope.json", "QUERY", query},
				{"query-applicability-envelope.json", "QUERY_APPLICABILITY", app}, {"request-write-envelope.json", "REQUEST_WRITE", request},
			} {
				envelope, err := os.ReadFile(filepath.Join(claimDir, role.name))
				if err != nil {
					t.Fatal(err)
				}
				artifact := role.original
				if role.code == "SOURCE" {
					artifact, err = os.ReadFile(filepath.Join(claimDir, "source-artifact.bin"))
					if err != nil {
						t.Fatal(err)
					}
				}
				in.Write.Claims = append(in.Write.Claims, B4aClaim{Role: role.code, Envelope: envelope, Original: role.original, Artifact: artifact})
			}
			if err := CheckB4a(in.Write); err != nil {
				t.Fatalf("precondition held B4a: %v", err)
			}
			capDir := filepath.Join(capRoot, tc.scenario, "WRITE"+strconv.FormatUint(tc.ordinal, 10))
			in.CapabilityEnvelope, err = os.ReadFile(filepath.Join(capDir, "capability-envelope.json"))
			if err != nil {
				t.Fatal(err)
			}
			in.CapabilityOriginal, err = os.ReadFile(filepath.Join(capDir, "capability-artifact.bin"))
			if err != nil {
				t.Fatal(err)
			}
			if err := A4Validate("CAPABILITY_EVENTS", in.CapabilityEnvelope); err != nil {
				t.Fatalf("precondition CAP A4: %v", err)
			}
			if got := CheckB4bFullCandidate(in); got != tc.want {
				t.Fatalf("chronology outcome got %s want %s", got, tc.want)
			}
			// In-memory falsifiers are not independently held replacement originals.
			changed := in
			changed.CapabilityOriginal = append([]byte(nil), in.CapabilityOriginal...)
			changed.CapabilityOriginal[len(changed.CapabilityOriginal)-1] ^= 1
			if got := CheckB4bFullCandidate(changed); got != "UNKNOWN" {
				t.Fatalf("changed /2 original got %s want UNKNOWN", got)
			}
			changed = in
			changed.Write.CompletedKey = "number:" + string(message["id"])
			if got := CheckB4bFullCandidate(changed); got != "UNKNOWN" {
				t.Fatalf("owner-key substitution got %s want UNKNOWN", got)
			}
			if tc.scenario == "CORE" && tc.ordinal == 6 {
				for _, variant := range []struct{ name, selector, want string }{
					{"missing-held-selector", "", "UNKNOWN"},
					{"invalid-after-match", `[{"scheme":"file"},{"scheme":"1bad"}]`, "MALFORMED"},
					{"wrong-known-type", `[{"scheme":3}]`, "MALFORMED"},
					{"invalid-notebook-known-type", `[{"scheme":"file","notebook":{"scheme":3}}]`, "MALFORMED"},
					{"ordinary-notebook-only", `[{"notebook":"go-notebook"}]`, "UNSUPPORTED"},
					{"path-dependent-positive", `[{"scheme":"file","pattern":"**/*.go"}]`, "UNKNOWN"},
					{"proven-scheme-mismatch", `[{"scheme":"untitled","pattern":"**/*.go"}]`, "UNSUPPORTED"},
					{"language-only", `[{"language":"go"}]`, "SUPPORTED"},
					{"unknown-extension-retained", `[{"scheme":"file","futureField":true}]`, "SUPPORTED"},
				} {
					t.Run(variant.name, func(t *testing.T) {
						v := in
						v.HeldClientSelector = []byte(variant.selector)
						if got := CheckB4bFullCandidate(v); got != variant.want {
							t.Fatalf("got %s want %s", got, variant.want)
						}
					})
				}
			}
		})
	}
}
