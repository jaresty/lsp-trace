package adr0011generic

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func TestA3ExactPrivateV3Originals(t *testing.T) {
	for role, pin := range v3OriginalPins {
		embedded, err := loadV3Original(role)
		if err != nil {
			t.Fatal(err)
		}
		disk := a2Source(t, pin.name)
		if len(embedded) != pin.length || digest(embedded) != pin.hash || !bytes.Equal(embedded, disk) {
			t.Fatalf("V3 %s not exact disk original", role)
		}
		if err := verifyV3Original(role, disk); err != nil {
			t.Fatal(err)
		}
		altered := bytes.Clone(disk)
		altered[0] ^= 1
		if err := verifyV3Original(role, altered); err == nil {
			t.Fatalf("V3 %s accepted substitution", role)
		}
		embedded[0] ^= 1
		next, err := loadV3Original(role)
		if err != nil || !bytes.Equal(next, disk) {
			t.Fatalf("V3 %s pin mutated", role)
		}
		if role == "SCHEMA" || role == "TRANSPORT" {
			old, err := loadOriginal(role)
			if err != nil || !bytes.Equal(old, disk) {
				t.Fatalf("V2 shared %s changed", role)
			}
		}
	}
	for _, method := range []string{"REFERENCES", "DEFINITION"} {
		v2, err := loadOriginal(method)
		if err != nil {
			t.Fatal(err)
		}
		if verifyV3Original(method, v2) == nil || verifyOriginal(method, a2Source(t, v3OriginalPins[method].name)) == nil {
			t.Fatalf("%s V2/V3 method policy substitution", method)
		}
		var p struct {
			Shared string `json:"shared_applicability_sha256"`
			Domain string `json:"selector_domain"`
			Method string `json:"method"`
		}
		b, _ := loadV3Original(method)
		if err := json.Unmarshal(b, &p); err != nil {
			t.Fatal(err)
		}
		if p.Shared != v3OriginalPins["APPLICABILITY"].hash || p.Domain != v3Domain || p.Method != "textDocument/"+strings.ToLower(method) {
			t.Fatalf("%s shared contract or method mismatch", method)
		}
	}
	if verifyV3Original("REFERENCES", a2Source(t, v3OriginalPins["DEFINITION"].name)) == nil || verifyV3Original("DEFINITION", a2Source(t, v3OriginalPins["REFERENCES"].name)) == nil {
		t.Fatal("cross-method policy substitution")
	}
	if _, err := loadV3Original("UNKNOWN"); err == nil {
		t.Fatal("unknown V3 original accepted")
	}
}

type a3Case struct {
	Identity []json.RawMessage `json:"identity"`
	URI      string            `json:"uri_hex"`
	Version  *string           `json:"version_hex"`
	Content  *string           `json:"content_hex"`
	Custody  SourceCustodyKind `json:"custody"`
}

func a3Held(t *testing.T, c a3Case) (syntheticTransactionIdentity, HeldSourceOriginal) {
	t.Helper()
	if len(c.Identity) != 3 || c.Version == nil || c.Content == nil {
		t.Fatal("bad fixture")
	}
	var id syntheticTransactionIdentity
	if err := json.Unmarshal(c.Identity[0], &id.session); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(c.Identity[1], &id.generation); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(c.Identity[2], &id.transaction); err != nil {
		t.Fatal(err)
	}
	uri, e := hex.DecodeString(c.URI)
	if e != nil {
		t.Fatal(e)
	}
	version, e := hex.DecodeString(*c.Version)
	if e != nil {
		t.Fatal(e)
	}
	content, e := hex.DecodeString(*c.Content)
	if e != nil {
		t.Fatal(e)
	}
	tx, e := heldSourceTX(id)
	if e != nil {
		t.Fatal(e)
	}
	return id, HeldSourceOriginal{TransactionIdentity: tx, URIBytes: uri, VersionBytes: version, ContentBytes: content, CustodyKind: c.Custody}
}
func TestA3V3All36VectorsAndV2NonSubstitution(t *testing.T) {
	raw := a2Source(t, "generic-lsp-v3-selector-vectors.proposed.json")
	if len(raw) != 4715 || digest(raw) != "d5a8e14631fbc10d4082ee1f8c2bfedd1b34c5821993986e301cb1f65141d0ee" {
		t.Fatal("V3 vector bytes changed")
	}
	var fixture struct {
		Domain    string            `json:"domain"`
		Selectors map[string]string `json:"selectors"`
		Pins      map[string]struct {
			Length int    `json:"length"`
			SHA    string `json:"sha256"`
		} `json:"pins"`
		V2CollisionCount int `json:"v2_collision_count"`
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.Domain != v3Domain || len(fixture.Selectors) != 36 || fixture.V2CollisionCount != 0 || len(fixture.Pins) != 7 {
		t.Fatal("V3 fixture shape")
	}
	for key, role := range map[string]string{"schema": "SCHEMA", "transport": "TRANSPORT", "shared": "APPLICABILITY", "references": "REFERENCES", "definition": "DEFINITION"} {
		p := fixture.Pins[key]
		if p.Length != v3OriginalPins[role].length || p.SHA != v3OriginalPins[role].hash {
			t.Fatalf("V3 fixture pin %s", key)
		}
	}
	oldRaw := a2Source(t, "generic-lsp-v2-selector-vectors.json")
	sourcesRaw := a2Source(t, "generic-lsp-source-selector-v2-proposed-vectors.json")
	if len(oldRaw) != 2220 || digest(oldRaw) != "6992776962db2a815c5f2a3ea9d96a89a7330ec40f6b862cd3894a1c13b7614e" || len(sourcesRaw) != 5551 || digest(sourcesRaw) != "7de0399f1fbd711abea08d26043492de4417574d8bdf13faf05577f14f48650f" {
		t.Fatal("V2 vectors changed")
	}
	for key, want := range map[string]originalPin{"v2_vectors": {"", 2220, "6992776962db2a815c5f2a3ea9d96a89a7330ec40f6b862cd3894a1c13b7614e"}, "v2_source_vectors": {"", 5551, "7de0399f1fbd711abea08d26043492de4417574d8bdf13faf05577f14f48650f"}} {
		p := fixture.Pins[key]
		if p.Length != want.length || p.SHA != want.hash {
			t.Fatal("V2 pin in V3 fixture changed")
		}
	}
	var old struct {
		Query     string            `json:"query_artifact_utf8_hex"`
		Event     string            `json:"event_artifact_utf8_hex"`
		Selectors map[string]string `json:"selectors"`
	}
	var sources struct {
		Cases     map[string]a3Case `json:"cases"`
		Rejected  map[string]a3Case `json:"rejected_inputs"`
		Selectors map[string]string `json:"selectors"`
	}
	if err := json.Unmarshal(oldRaw, &old); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(sourcesRaw, &sources); err != nil {
		t.Fatal(err)
	}
	query, e := hex.DecodeString(old.Query)
	if e != nil {
		t.Fatal(e)
	}
	event, e := hex.DecodeString(old.Event)
	if e != nil {
		t.Fatal(e)
	}
	tested := map[string]bool{}
	check := func(key, got string) {
		t.Helper()
		if fixture.Selectors[key] != got {
			t.Errorf("%s differs: %s != %s", key, got, fixture.Selectors[key])
		}
		tested[key] = true
	}
	for _, suffix := range []string{"references", "definition"} {
		method := "textDocument/" + suffix
		role := strings.ToUpper(suffix)
		q := selectorInput{Method: method, Role: "QUERY", SessionID: "s", Generation: 1, Artifact: query, URI: "file:///a", Version: "buffer:v1", SourceDigest: digest([]byte("a")), Encoding: "utf-16"}
		ev := selectorInput{Method: method, Role: "TARGET_EVENTS", SessionID: "s", Generation: 1, Artifact: event, ActualWriteKey: "observed-key-1", ResultDigest: digest([]byte("[]"))}
		for _, tc := range []struct {
			name string
			in   selectorInput
		}{{"query_version1_line0", q}, {"query_version2_line0", func() selectorInput { x := q; x.Version = "buffer:v2"; return x }()}, {"query_version1_line1", func() selectorInput { x := q; x.Line = 1; return x }()}, {"event_ordinal0", ev}, {"event_ordinal1", func() selectorInput { x := ev; x.Ordinal = 1; return x }()}} {
			got, err := calculateV3Selector(tc.in)
			if err != nil {
				t.Fatal(err)
			}
			check(suffix+"_"+tc.name, got)
			if prior, err := calculateSelector(tc.in); err != nil || prior == got {
				t.Fatalf("V2 selector substituted as V3: %s", tc.name)
			}
		}
		for name, c := range sources.Cases {
			id, source := a3Held(t, c)
			got, err := heldV3SourceSelector(method, id, source)
			if err != nil {
				t.Fatal(err)
			}
			check(suffix+"_source_"+name, got)
			tx, _ := heldSourceTX(id)
			oldSel, err := heldSourceSelector(method, id, source, tx)
			if err != nil || oldSel == got {
				t.Fatal("V2 SOURCE substituted")
			}
		}
		schema, _ := loadV3Original("SCHEMA")
		transport, _ := loadV3Original("TRANSPORT")
		policy, _ := loadV3Original(role)
		for _, tc := range []struct {
			name           string
			schema, policy []byte
		}{{"query_schema_substitution", append(bytes.Clone(schema), '\n'), policy}, {"query_policy_substitution", schema, append(bytes.Clone(policy), '\n')}} {
			check(suffix+"_"+tc.name, selectorWithOriginals(q, v3Domain, "GENERIC_LSP_"+role+"_EXACT_V3", tc.schema, transport, tc.policy))
		}
		if len(sources.Rejected) != 7 {
			t.Fatal("missing invalid SOURCE fixtures")
		}
		for name, c := range sources.Rejected {
			if len(c.Identity) != 3 {
				t.Fatal("invalid fixture identity")
			}
			var id syntheticTransactionIdentity
			e0 := json.Unmarshal(c.Identity[0], &id.session)
			e1 := json.Unmarshal(c.Identity[1], &id.generation)
			e2 := json.Unmarshal(c.Identity[2], &id.transaction)
			if e0 != nil || e1 != nil || e2 != nil {
				continue
			} // Boolean/fractional generation rejected at typed decode.
			uri, e := hex.DecodeString(c.URI)
			if e != nil {
				t.Fatal(e)
			}
			var version, content []byte
			if c.Version != nil {
				version, e = hex.DecodeString(*c.Version)
				if e != nil {
					t.Fatal(e)
				}
			}
			if c.Content != nil {
				content, e = hex.DecodeString(*c.Content)
				if e != nil {
					t.Fatal(e)
				}
			}
			tx, e := heldSourceTX(id)
			if e != nil {
				continue
			}
			bad := HeldSourceOriginal{TransactionIdentity: tx, URIBytes: uri, VersionBytes: version, ContentBytes: content, CustodyKind: c.Custody}
			if got, e := heldV3SourceSelector(method, id, bad); e == nil {
				t.Errorf("%s/%s invalid SOURCE received %s", suffix, name, got)
			}
		}
		id, source := a3Held(t, sources.Cases["query"])
		tx, _ := heldSourceTX(id)
		var artifact []byte
		for _, f := range [][]byte{[]byte("ADR0011-GENERIC-SOURCE-ARTIFACT/1"), []byte(tx), source.URIBytes, []byte("present"), source.VersionBytes, []byte(strconv.Itoa(len(source.ContentBytes))), []byte(digest(source.ContentBytes)), []byte(source.CustodyKind)} {
			artifact = append(artifact, component(string(f))...)
		}
		for _, tc := range []struct {
			name           string
			schema, policy []byte
		}{{"schema_lf", append(bytes.Clone(schema), '\n'), policy}, {"policy_lf", schema, append(bytes.Clone(policy), '\n')}} {
			prefix := []string{v3Domain, "GENERIC_LSP_" + role + "_EXACT_V3", method, "SOURCE", digest(tc.schema), digest(transport), digest(tc.policy), id.session, "1", digest(artifact), tx}
			check(suffix+"_source_"+tc.name, v3HashParts(prefix))
		}
		other := "DEFINITION"
		if role == other {
			other = "REFERENCES"
		}
		otherPolicy, _ := loadV3Original(other)
		if got := selectorWithOriginals(q, v3Domain, "GENERIC_LSP_"+role+"_EXACT_V3", schema, transport, otherPolicy); got == fixture.Selectors[suffix+"_query_version1_line0"] {
			t.Fatal("method policy substituted")
		}
	}
	if len(tested) != 36 {
		t.Fatalf("A3 checked %d/36 selectors", len(tested))
	}
	seen := map[string]bool{}
	for key, value := range fixture.Selectors {
		if !tested[key] || seen[value] {
			t.Fatalf("missing or duplicated %s", key)
		}
		seen[value] = true
		for _, prior := range old.Selectors {
			if prior == value {
				t.Fatal("V2 QUERY/TARGET_EVENTS collision")
			}
		}
		for _, prior := range sources.Selectors {
			if prior == value {
				t.Fatal("V2 SOURCE collision")
			}
		}
	}
}
func TestA3NoRuntimeRoute(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(file), "..", "..")
	cmd := exec.Command("go", "list", "-deps", "./cmd/lsp-trace", "./cmd/lsp-trace-mcp", "./sessionruntime")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("runtime route inspection failed: %v: %s", err, out)
	}
	for _, p := range strings.Split(string(out), "\n") {
		if p == "lsp-trace/internal/adr0011generic" || p == "lsp-trace/internal/adr0011genericv3proposal" {
			t.Fatalf("V3 private dependency routed: %s", p)
		}
	}
}
