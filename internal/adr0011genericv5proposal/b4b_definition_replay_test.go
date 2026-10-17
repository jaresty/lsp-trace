package adr0011genericv5proposal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Definition originals are held separately from the accepted references fixture.
func TestB4bDefinitionPinnedMatrix(t *testing.T) {
	root := "testdata"
	raw := filepath.Join(root, "adr0011-definition-b4b-held-originals-v1")
	held := filepath.Join(root, "adr0011-definition-b4b-held-query-originals-v1")
	capRoot := filepath.Join(root, "adr0011-definition-b4b-capability-derived-v1")
	claims := filepath.Join(root, "adr0011-definition-b4b-b4a-derived-v1")
	for _, pin := range []struct{ path, digest string }{
		{filepath.Join(raw, "manifest.json"), "2fb4c68376f5268b0f1722033565c70595545aa3df08e9f3431ecbd9762f99fe"},
		{filepath.Join(held, "manifest.json"), "f4b765e5576f6ed22c839e2686116146db4d643cd53eef8af851757e824c7001"},
		{filepath.Join(capRoot, "manifest.json"), "8e319123c02f66e9e584d16db976dfc47f488f0156903fdc571b3f2b94790d16"},
		{filepath.Join(claims, "manifest.json"), "a88290ac884b4f0662d3c5ea5a07ea8031ce540fd580a866ab304d8b4bfde00c"},
		{filepath.Join(root, "adr0011-b4b-full-candidate-private-red-v1", "candidate-manifest.json"), "aee6006bbafdfae8f33a6b011cf34b69ad85411489ac963ef4243fd34cbbd6ec"},
		{"b4b_full_candidate.go", "25059ec63fa5c2e72a89ae46b513dd8571bf9f54a8d06fb917d405763874a381"},
		{"b4b_full_candidate_test.go", "61a1e0cb6360a597438c875d55f47234cd2a92edfac7118446a2b8418b6623ca"},
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
			if err := json.Unmarshal(b4bFullPinnedRead(t, filepath.Join(raw, "manifest.json"), "2fb4c68376f5268b0f1722033565c70595545aa3df08e9f3431ecbd9762f99fe"), &rootManifest); err != nil {
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
			claimDir := filepath.Join(claims, tc.scenario, "WRITE"+strconv.FormatUint(tc.ordinal, 10))
			message, err := b4aFrame(request)
			if err != nil {
				t.Fatal(err)
			}
			lang := ctx.Language.ID
			in := B4bFullCandidateInput{Write: B4aInput{
				Session: ctx.Session, Generation: ctx.Generation, Transaction: transaction,
				Workspace: ctx.Workspace, Method: ctx.Method, URI: ctx.URI, Version: "buffer:v1",
				Encoding: ctx.PositionEncoding, Line: 1, Character: 5, Source: source, QueryOriginal: query,
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
			if got := CheckB4bDefinitionSuccessor(in); got != tc.want {
				t.Fatalf("definition chronology outcome got %s want %s", got, tc.want)
			}
			// In-memory falsifiers are not independently held replacement originals.
			changed := in
			changed.CapabilityOriginal = append([]byte(nil), in.CapabilityOriginal...)
			changed.CapabilityOriginal[len(changed.CapabilityOriginal)-1] ^= 1
			if got := CheckB4bDefinitionSuccessor(changed); got != "UNKNOWN" {
				t.Fatalf("changed /2 original got %s want UNKNOWN", got)
			}
			changed = in
			changed.Write.CompletedKey = "number:" + string(message["id"])
			if got := CheckB4bDefinitionSuccessor(changed); got != "UNKNOWN" {
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
						if got := CheckB4bDefinitionSuccessor(v); got != variant.want {
							t.Fatalf("got %s want %s", got, variant.want)
						}
					})
				}
			}
		})
	}
}
