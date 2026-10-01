package adr0011generic

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

var a2V2Pins = map[string]originalPin{
	"SCHEMA":     {"adr0011-generic-envelope-v2.schema.json", 13243, "f843389f811aea940ed5bf1d595f03dcb6de4fa97c4dafafc41ad83b5c1c7f8e"},
	"TRANSPORT":  {"generic-lsp-exact-transport-v2.json", 1206, "0519ef89b76d966141bece6ca24d9e186b6424113339a39fce68ae3f78ffa67f"},
	"REFERENCES": {"generic-lsp-references-exact-v2.json", 724, "cd92d4167abc951432804991e576a52a43236f9061dfe21c412410a6a851ecc5"},
	"DEFINITION": {"generic-lsp-definition-exact-v2.json", 633, "f8fa1a0cb9f1379d69e347ef9573f6356d28f448f0cfb2529d5cd5747751a9af"},
}

const historicalDomain = "ADR0011-GENERIC-EXACT/1"

var historicalV1Pins = map[string]originalPin{
	"SCHEMA":     {"adr0011-generic-envelope-v1.schema.json", 10412, "efa909a074fa8b7e8949f7e70395f50312a384f0d0bd95b65168844fa6c8a9bd"},
	"TRANSPORT":  {"generic-lsp-exact-transport-v1.json", 1205, "6fbb54cf37ec6efe86cb42e8717f9610716b8a84e432a57367135b8da9e265ae"},
	"REFERENCES": {"generic-lsp-references-exact-v1.json", 723, "5d99986050dd33ff0fffaf623bd8a940a9675053361c8d137a749413dff7311a"},
	"DEFINITION": {"generic-lsp-definition-exact-v1.json", 632, "466994a66fadd65ff80692b5d0284de221d0cd1b6907ea3b11a0652400697648"},
}

func historicalOriginal(t *testing.T, role string) []byte {
	t.Helper()
	return a2Source(t, historicalV1Pins[role].name)
}
func historicalVerify(t *testing.T, role string, b []byte) bool {
	t.Helper()
	p := historicalV1Pins[role]
	return len(b) == p.length && digest(b) == p.hash && bytes.Equal(b, historicalOriginal(t, role))
}
func historicalSelector(t *testing.T, in selectorInput) string {
	t.Helper()
	role := "REFERENCES"
	if in.Method == "textDocument/definition" {
		role = "DEFINITION"
	}
	return selectorWithOriginals(in, historicalDomain, "GENERIC_LSP_"+role+"_EXACT_V1", historicalOriginal(t, "SCHEMA"), historicalOriginal(t, "TRANSPORT"), historicalOriginal(t, role))
}

// Independent test-only preimage oracle. It cannot select production originals.
func selectorWithOriginals(in selectorInput, dom, profile string, schema, transport, policy []byte) string {
	parts := []string{dom, profile, in.Method, in.Role, digest(schema), digest(transport), digest(policy), in.SessionID, strconv.FormatUint(in.Generation, 10), digest(in.Artifact)}
	if in.Role == "QUERY" {
		parts = append(parts, in.URI, in.Version, in.SourceDigest, in.Encoding, strconv.FormatUint(in.Line, 10), strconv.FormatUint(in.Character, 10))
	} else {
		parts = append(parts, in.ActualWriteKey, in.ResultDigest, strconv.FormatUint(in.Ordinal, 10))
	}
	h := sha256.New()
	for _, p := range parts {
		var n [8]byte
		binary.BigEndian.PutUint64(n[:], uint64(len([]byte(p))))
		h.Write(n[:])
		h.Write([]byte(p))
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

func a2Source(t *testing.T, name string) []byte {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller unavailable")
	}
	b, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "docs", "qualification", "originals", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestA2V2EmbeddedPinsAndHistoricalV1(t *testing.T) {
	for role, want := range a2V2Pins {
		got, err := loadOriginal(role)
		if err != nil {
			t.Fatalf("%s: %v", role, err)
		}
		if originalPins[role] != want || len(got) != want.length || digest(got) != want.hash || !bytes.Equal(got, a2Source(t, want.name)) {
			t.Errorf("A2 V2 pinned %s identity mismatch", role)
		}
		bad := bytes.Clone(got)
		bad[0] ^= 1
		if verifyOriginal(role, bad) == nil {
			t.Errorf("A2 V2 substituted %s original accepted", role)
		}
	}
	for name, pin := range historicalV1Pins {
		b, err := originals.ReadFile("originals/" + pin.name)
		if err != nil || len(b) != pin.length || digest(b) != pin.hash || !bytes.Equal(b, a2Source(t, pin.name)) {
			t.Errorf("A2 historical V1 %s changed: %v", name, err)
		}
	}
	if len(a2Source(t, "generic-lsp-final-selector-vectors.json")) != 1429 || digest(a2Source(t, "generic-lsp-final-selector-vectors.json")) != "d511db2c3c1454eadbb197aef747ab5441c7bcfc96dd2d960b067e12badec9f0" {
		t.Error("A2 historical V1 vector changed")
	}
}

func TestA2V2FrozenSelectorVectors(t *testing.T) {
	b := a2Source(t, "generic-lsp-v2-selector-vectors.json")
	if len(b) != 2220 || digest(b) != "6992776962db2a815c5f2a3ea9d96a89a7330ec40f6b862cd3894a1c13b7614e" {
		t.Fatal("A2 V2 frozen vector identity mismatch")
	}
	// Field names are explicit because selector fixture keys use snake_case.
	var fixture struct {
		Domain       string            `json:"domain"`
		QueryHex     string            `json:"query_artifact_utf8_hex"`
		EventHex     string            `json:"event_artifact_utf8_hex"`
		SchemaSHA    string            `json:"schema_sha256"`
		TransportSHA string            `json:"transport_sha256"`
		PolicySHA    map[string]string `json:"policy_sha256"`
		Selectors    map[string]string `json:"selectors"`
	}
	if err := json.Unmarshal(b, &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.Domain != domain || fixture.SchemaSHA != a2V2Pins["SCHEMA"].hash || fixture.TransportSHA != a2V2Pins["TRANSPORT"].hash || len(fixture.Selectors) != 14 {
		t.Fatal("A2 V2 selector domain/digest/count mismatch")
	}
	query, err := hex.DecodeString(fixture.QueryHex)
	if err != nil {
		t.Fatal(err)
	}
	event, err := hex.DecodeString(fixture.EventHex)
	if err != nil {
		t.Fatal(err)
	}
	for _, suffix := range []string{"references", "definition"} {
		method := "textDocument/" + suffix
		role := strings.ToUpper(suffix)
		if fixture.PolicySHA[suffix] != a2V2Pins[role].hash {
			t.Fatalf("%s policy mismatch", suffix)
		}
		queryIn := selectorInput{Method: method, Role: "QUERY", SessionID: "s", Generation: 1, Artifact: query, URI: "file:///a", Version: "buffer:v1", SourceDigest: digest([]byte("a")), Encoding: "utf-16"}
		eventIn := selectorInput{Method: method, Role: "TARGET_EVENTS", SessionID: "s", Generation: 1, Artifact: event, ActualWriteKey: "observed-key-1", ResultDigest: digest([]byte("[]"))}
		cases := []struct {
			name  string
			input selectorInput
		}{
			{"query_version1_line0", queryIn},
			{"query_version2_line0", func() selectorInput { x := queryIn; x.Version = "buffer:v2"; return x }()},
			{"query_version1_line1", func() selectorInput { x := queryIn; x.Line = 1; return x }()},
			{"event_ordinal0", eventIn},
			{"event_ordinal1", func() selectorInput { x := eventIn; x.Ordinal = 1; return x }()},
		}
		seen := map[string]bool{}
		for _, tc := range cases {
			got, err := calculateSelector(tc.input)
			if err != nil || got != fixture.Selectors[suffix+"_"+tc.name] {
				t.Errorf("A2 V2 %s %s selector mismatch: %s %v", suffix, tc.name, got, err)
			}
			if seen[got] {
				t.Errorf("A2 V2 %s substitution collapsed at %s", suffix, tc.name)
			}
			seen[got] = true
		}
		changed := queryIn
		changed.SourceDigest = digest([]byte("different"))
		other, err := calculateSelector(changed)
		if err != nil || other == fixture.Selectors[suffix+"_query_version1_line0"] {
			t.Errorf("A2 V2 %s source substitution collapsed: %v", suffix, err)
		}
		schema, _ := loadOriginal("SCHEMA")
		transport, _ := loadOriginal("TRANSPORT")
		policy, _ := loadOriginal(role)
		for _, sub := range []struct {
			name           string
			schema, policy []byte
		}{
			{"query_schema_substitution", append(bytes.Clone(schema), '\n'), policy},
			{"query_policy_substitution", schema, append(bytes.Clone(policy), '\n')},
		} {
			hypothetical := selectorWithOriginals(queryIn, domain, "GENERIC_LSP_"+role+"_EXACT_V2", sub.schema, transport, sub.policy)
			if hypothetical != fixture.Selectors[suffix+"_"+sub.name] || hypothetical == fixture.Selectors[suffix+"_query_version1_line0"] {
				t.Errorf("A2 V2 %s %s mismatch", suffix, sub.name)
			}
		}
		v1 := selectorWithOriginals(queryIn, historicalDomain, "GENERIC_LSP_"+role+"_EXACT_V1", historicalOriginal(t, "SCHEMA"), historicalOriginal(t, "TRANSPORT"), historicalOriginal(t, role))
		if v1 == fixture.Selectors[suffix+"_query_version1_line0"] {
			t.Errorf("A2 V1 %s accepted as V2", suffix)
		}
		invalid := queryIn
		invalid.Role = "TARGET_EVENTS"
		invalid.Artifact = event
		invalid.ActualWriteKey = eventIn.ActualWriteKey
		invalid.ResultDigest = eventIn.ResultDigest
		if got, _ := calculateSelector(invalid); got == fixture.Selectors[suffix+"_query_version1_line0"] {
			t.Errorf("A2 V2 %s role substitution collapsed", suffix)
		}
	}
}

func a2Deps(roles ...string) []any {
	out := make([]any, 0, len(roles))
	for i, r := range roles {
		out = append(out, map[string]any{"role": r, "selector": "s:" + strconv.Itoa(i), "digest": "sha256:" + strings.Repeat("0", 64)})
	}
	return out
}
func a2Envelope(role string, pred []any) map[string]any {
	original := map[string]any{"length": 1, "sha256": "sha256:" + strings.Repeat("0", 64), "private_ref": "test:bytes"}
	var payload any
	switch role {
	case "TERMINAL":
		payload = map[string]any{"N": 0, "B": 0, "T": 0, "E": 0, "E_B": 0, "E_T": 0, "P": 0, "A": 0, "disposition": "SYNTHETIC", "publication": "NOT_COMMITTED", "authority": 0, "accepted": false, "completeness": "UNKNOWN"}
	case "TARGET_EVENTS":
		payload = map[string]any{"denominator": nil, "ordinals": []any{}}
	case "READBACK":
		payload = map[string]any{"originals_verified": []any{}, "fresh_external_final": false, "correspondence": "UNAVAILABLE"}
	}
	return map[string]any{"role": role, "identity": map[string]any{"session": "s", "generation": 1, "transaction": "t"}, "original": original, "predecessors": pred, "payload": payload}
}
func TestA2V2EmbeddedRoleCapsAndCardinality(t *testing.T) {
	b, err := loadOriginal("SCHEMA")
	if err != nil {
		t.Fatal(err)
	}
	var doc any
	if err = json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	// Test-only in-memory adversarial schema, never an admitted original.
	if os.Getenv("ADR0011_A2_BAD_SOURCE_MIN") == "1" {
		defs := doc.(map[string]any)["$defs"].(map[string]any)
		terminal := defs["TERMINAL"].(map[string]any)["allOf"].([]any)[1].(map[string]any)
		predecessors := terminal["properties"].(map[string]any)["predecessors"].(map[string]any)
		found := false
		for _, item := range predecessors["allOf"].([]any) {
			constraint := item.(map[string]any)
			contains := constraint["contains"].(map[string]any)
			props := contains["properties"].(map[string]any)
			if props["role"].(map[string]any)["const"] == "SOURCE" {
				constraint["minContains"] = float64(0)
				found = true
			}
		}
		if !found {
			t.Fatal("A2 SOURCE predecessor rule unavailable for perturbation")
		}
	}
	c := jsonschema.NewCompiler()
	const uri = "https://jaresty.github.io/lsp-trace/schemas/adr0011-generic-envelope-v2.schema.json"
	if err = c.AddResource(uri, doc); err != nil {
		t.Fatal(err)
	}
	schema, err := c.Compile(uri)
	if err != nil {
		t.Fatal(err)
	}
	fixed := []string{"POLICY", "SCHEMA", "QUERY", "CAPABILITY_EVENTS", "REQUEST_WRITE", "INBOUND_FRAMES", "RESULT_READ", "TARGET_EVENTS"}
	many := func(n int) []string {
		s := make([]string, n)
		for i := range s {
			s[i] = "SOURCE"
		}
		return s
	}
	roles := append(append([]string{}, fixed...), many(256)...)
	valid := a2Deps(roles...)
	check := func(name, role string, pred []any, accept bool) {
		t.Helper()
		err := schema.Validate(a2Envelope(role, pred))
		if (err == nil) != accept {
			t.Errorf("A2 V2 %s valid=%v: %v", name, accept, err)
		}
	}
	check("TERMINAL 264", "TERMINAL", valid, true)
	check("TERMINAL 265", "TERMINAL", append(append([]any{}, valid...), a2Deps("PROCESS")[0]), false)
	check("TERMINAL empty", "TERMINAL", []any{}, false)
	check("TERMINAL eight fixed zero SOURCE", "TERMINAL", a2Deps(fixed...), false)
	check("TERMINAL eight fixed one SOURCE", "TERMINAL", a2Deps(append(append([]string{}, fixed...), "SOURCE")...), true)
	for i, r := range fixed {
		x := append(append([]any{}, valid[:i]...), valid[i+1:]...)
		check("TERMINAL missing "+r, "TERMINAL", x, false)
	}
	check("TERMINAL duplicate RESULT_READ", "TERMINAL", append(append([]any{}, valid...), a2Deps("RESULT_READ")[0]), false)
	target := a2Deps(append([]string{"RESULT_READ"}, many(255)...)...)
	check("TARGET_EVENTS 256", "TARGET_EVENTS", target, true)
	check("TARGET_EVENTS 257", "TARGET_EVENTS", append(append([]any{}, target...), a2Deps("SOURCE")[0]), false)
	readback := a2Deps(append(append(append([]string{}, fixed...), "TERMINAL", "PROCESS"), many(256)...)...)
	check("READBACK 266", "READBACK", readback, true)
	check("READBACK 267", "READBACK", append(append([]any{}, readback...), a2Deps("SOURCE")[0]), false)
}

// Production entry points must not acquire the private A2 selector package.
func TestA2V2NoRuntimeRoute(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller unavailable")
	}
	root := filepath.Join(filepath.Dir(file), "..", "..")
	cmd := exec.Command("go", "list", "-deps", "./cmd/lsp-trace", "./cmd/lsp-trace-mcp", "./sessionruntime")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("A2 runtime dependency inspection unavailable: %v: %s", err, out)
	}
	for _, pkg := range strings.Split(string(out), "\n") {
		if pkg == "lsp-trace/internal/adr0011generic" || pkg == "lsp-trace/internal/adr0011genericv2proposal" {
			t.Fatalf("A2 private V2 selected by production dependency route: %s", pkg)
		}
	}
}

// Each inherited per-role cap and mandatory predecessor is checked against the
// selected embedded V2 schema, not a claimant-selected schema or a test copy.
func TestA2V2InheritedRoleCapsAndMandatoryEdges(t *testing.T) {
	b, err := loadOriginal("SCHEMA")
	if err != nil {
		t.Fatal(err)
	}
	var doc any
	if err = json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	c := jsonschema.NewCompiler()
	const uri = "https://jaresty.github.io/lsp-trace/schemas/adr0011-generic-envelope-v2.schema.json"
	if err = c.AddResource(uri, doc); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		role    string
		valid   []string
		extra   string
		missing string
	}{
		{"QUERY", []string{"POLICY", "SCHEMA", "SOURCE"}, "SOURCE", "POLICY"},
		{"CAPABILITY_EVENTS", []string{"PROCESS"}, "PROCESS", ""},
		{"REQUEST_WRITE", []string{"QUERY", "CAPABILITY_EVENTS", "POLICY", "PROCESS"}, "PROCESS", "QUERY"},
		{"INBOUND_FRAMES", []string{"REQUEST_WRITE"}, "REQUEST_WRITE", "REQUEST_WRITE"},
		{"RESULT_READ", []string{"INBOUND_FRAMES", "REQUEST_WRITE"}, "INBOUND_FRAMES", "REQUEST_WRITE"},
	}
	for _, tc := range cases {
		t.Run(tc.role, func(t *testing.T) {
			defs := doc.(map[string]any)["$defs"].(map[string]any)
			key := tc.role
			if ref, ok := defs[key].(map[string]any)["$ref"].(string); ok {
				key = strings.TrimPrefix(ref, "#/$defs/")
			}
			sub, e := c.Compile(uri + "#/$defs/" + key + "/allOf/1/properties/predecessors")
			if e != nil {
				t.Fatal(e)
			}
			check := func(label string, roles []string, want bool) {
				t.Helper()
				err := sub.Validate(a2Deps(roles...))
				if (err == nil) != want {
					t.Errorf("A2 V2 %s %s valid=%v: %v", tc.role, label, want, err)
				}
			}
			check("at cap", tc.valid, true)
			check("cap+1", append(append([]string{}, tc.valid...), tc.extra), false)
			if tc.missing != "" {
				without := []string{}
				for _, r := range tc.valid {
					if r != tc.missing {
						without = append(without, r)
					}
				}
				check("missing "+tc.missing, without, false)
			}
			if tc.role == "CAPABILITY_EVENTS" {
				check("optional zero", []string{}, true)
			}
		})
	}
	for _, role := range []string{"POLICY", "SCHEMA", "SOURCE", "PROCESS"} {
		t.Run(role+" zero", func(t *testing.T) {
			defs := doc.(map[string]any)["$defs"].(map[string]any)
			key := role
			if ref, ok := defs[role].(map[string]any)["$ref"].(string); ok {
				key = strings.TrimPrefix(ref, "#/$defs/")
			}
			subschema, e := c.Compile(uri + "#/$defs/" + key + "/allOf/1/properties/predecessors")
			if e != nil {
				t.Fatalf("A2 V2 %s predecessor fragment: %v", role, e)
			}
			if e = subschema.Validate([]any{}); e != nil {
				t.Errorf("A2 V2 %s zero predecessors: %v", role, e)
			}
			if e = subschema.Validate(a2Deps("SOURCE")); e == nil {
				t.Errorf("A2 V2 %s one predecessor accepted", role)
			}
		})
	}
}
