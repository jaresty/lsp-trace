package adr0011genericv3proposal

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"
)

// Test-only independent recomputation, never imported by a selector issuer.
type sourceCase struct {
	Identity []json.RawMessage `json:"identity"`
	URI      string            `json:"uri_hex"`
	Version  *string           `json:"version_hex"`
	Content  *string           `json:"content_hex"`
	Custody  string            `json:"custody"`
}
type v3Fixture struct {
	Domain    string            `json:"domain"`
	Selectors map[string]string `json:"selectors"`
	Pins      map[string]struct {
		Length int    `json:"length"`
		SHA    string `json:"sha256"`
	} `json:"pins"`
}

func hash(b []byte) string { sum := sha256.Sum256(b); return hex.EncodeToString(sum[:]) }
func lp(b []byte) []byte {
	out := make([]byte, 8+len(b))
	binary.BigEndian.PutUint64(out[:8], uint64(len(b)))
	copy(out[8:], b)
	return out
}
func selector(fields ...string) string {
	var data []byte
	for _, f := range fields {
		data = append(data, lp([]byte(f))...)
	}
	return "sha256:" + hash(data)
}
func sourceSelector(method string, c sourceCase, schema, transport, policy []byte) (string, error) {
	if len(c.Identity) != 3 || c.Version == nil || c.Content == nil {
		return "", fmt.Errorf("absent source/identity")
	}
	var session, transaction string
	var generation uint64
	if err := json.Unmarshal(c.Identity[0], &session); err != nil {
		return "", err
	}
	if err := json.Unmarshal(c.Identity[1], &generation); err != nil {
		return "", err
	}
	if err := json.Unmarshal(c.Identity[2], &transaction); err != nil {
		return "", err
	}
	if session == "" || transaction == "" || generation == 0 {
		return "", fmt.Errorf("invalid transaction")
	}
	uri, err := hex.DecodeString(c.URI)
	if err != nil {
		return "", err
	}
	version, err := hex.DecodeString(*c.Version)
	if err != nil {
		return "", err
	}
	content, err := hex.DecodeString(*c.Content)
	if err != nil {
		return "", err
	}
	if len(uri) == 0 || version == nil || content == nil {
		return "", fmt.Errorf("missing bytes")
	}
	if !utf8.Valid(uri) || !utf8.Valid(version) {
		return "", fmt.Errorf("invalid UTF-8 source identity")
	}
	switch c.Custody {
	case "OWNER_BUFFER", "MANAGED_VIRTUAL", "CLEAN_REGISTERED_WORKTREE", "IMMUTABLE_SOURCE_SNAPSHOT":
	default:
		return "", fmt.Errorf("unknown custody")
	}
	tx := selector("ADR0011-GENERIC-TRANSACTION/1", session, strconv.FormatUint(generation, 10), transaction)
	var artifact []byte
	for _, field := range [][]byte{[]byte("ADR0011-GENERIC-SOURCE-ARTIFACT/1"), []byte(tx), uri, []byte("present"), version, []byte(strconv.Itoa(len(content))), []byte(hash(content)), []byte(c.Custody)} {
		artifact = append(artifact, lp(field)...)
	}
	prefix := []string{"ADR0011-GENERIC-EXACT/3", "GENERIC_LSP_" + strings.ToUpper(method) + "_EXACT_V3", "textDocument/" + method, "SOURCE", hash(schema), hash(transport), hash(policy), session, strconv.FormatUint(generation, 10), hash(artifact), tx}
	return selector(prefix...), nil
}
func TestV3AllPolicyDependentSelectorsIndependent(t *testing.T) {
	root := filepath.Join("..", "..", "docs", "qualification", "originals")
	pins := map[string]struct {
		name string
		size int
		sha  string
	}{
		"schema":            {"adr0011-generic-envelope-v2.schema.json", 13243, "f843389f811aea940ed5bf1d595f03dcb6de4fa97c4dafafc41ad83b5c1c7f8e"},
		"transport":         {"generic-lsp-exact-transport-v2.json", 1206, "0519ef89b76d966141bece6ca24d9e186b6424113339a39fce68ae3f78ffa67f"},
		"shared":            {"generic-lsp-selector-applicability-v1.json", 2247, "c73263cb1356abd9f2898e016ec31afbb6cb112924c881c0a1ef6034bb1d5a0d"},
		"references":        {"generic-lsp-references-exact-v3.json", 1134, "8c76a71da5e888e278280bbb1802979b907ba7a4ad6b0aec910dfe06e24e225e"},
		"definition":        {"generic-lsp-definition-exact-v3.json", 1043, "52dd415cb5a331462042103014991fcd4ff43a777d45ab5a9abd7d83a45a8051"},
		"v2_vectors":        {"generic-lsp-v2-selector-vectors.json", 2220, "6992776962db2a815c5f2a3ea9d96a89a7330ec40f6b862cd3894a1c13b7614e"},
		"v2_source_vectors": {"generic-lsp-source-selector-v2-proposed-vectors.json", 5551, "7de0399f1fbd711abea08d26043492de4417574d8bdf13faf05577f14f48650f"},
	}
	fixtureBytes, err := os.ReadFile(filepath.Join(root, "generic-lsp-v3-selector-vectors.proposed.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixture v3Fixture
	if err := json.Unmarshal(fixtureBytes, &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.Domain != "ADR0011-GENERIC-EXACT/3" || len(fixture.Selectors) != 36 || len(fixture.Pins) != 7 {
		t.Fatal("V3 vector domain/coverage")
	}
	originals := make(map[string][]byte)
	for key, pin := range pins {
		body, err := os.ReadFile(filepath.Join(root, pin.name))
		if err != nil {
			t.Fatal(err)
		}
		if len(body) != pin.size || hash(body) != pin.sha || fixture.Pins[key].Length != pin.size || fixture.Pins[key].SHA != pin.sha {
			t.Fatalf("pinned %s changed", key)
		}
		originals[key] = body
	}
	var shared struct {
		Filters struct {
			NotebookString   string `json:"notebook_string"`
			NotebookLanguage string `json:"notebook_language"`
		} `json:"filters"`
	}
	if err := json.Unmarshal(originals["shared"], &shared); err != nil {
		t.Fatal(err)
	}
	if shared.Filters.NotebookString != "EXACT_UTF8_BYTES_OR_STAR_ALL" || shared.Filters.NotebookLanguage != "EXACT_UTF8_BYTES_OR_STAR_ALL" {
		t.Fatal("notebook string/language wildcard semantics missing from shared policy")
	}
	var previous struct {
		QueryHex  string            `json:"query_artifact_utf8_hex"`
		EventHex  string            `json:"event_artifact_utf8_hex"`
		Selectors map[string]string `json:"selectors"`
	}
	if err := json.Unmarshal(originals["v2_vectors"], &previous); err != nil {
		t.Fatal(err)
	}
	var oldSources struct {
		Cases     map[string]sourceCase `json:"cases"`
		Rejected  map[string]sourceCase `json:"rejected_inputs"`
		Selectors map[string]string     `json:"selectors"`
	}
	if err := json.Unmarshal(originals["v2_source_vectors"], &oldSources); err != nil {
		t.Fatal(err)
	}
	query, err := hex.DecodeString(previous.QueryHex)
	if err != nil {
		t.Fatal(err)
	}
	event, err := hex.DecodeString(previous.EventHex)
	if err != nil {
		t.Fatal(err)
	}
	seen := make(map[string]bool)
	for _, method := range []string{"references", "definition"} {
		policy := originals[method]
		base := []string{"ADR0011-GENERIC-EXACT/3", "GENERIC_LSP_" + strings.ToUpper(method) + "_EXACT_V3", "textDocument/" + method}
		cases := []struct {
			name, role     string
			artifact       []byte
			extra          []string
			schema, policy []byte
		}{
			{"query_version1_line0", "QUERY", query, []string{"file:///a", "buffer:v1", hash([]byte("a")), "utf-16", "0", "0"}, nil, nil},
			{"query_version2_line0", "QUERY", query, []string{"file:///a", "buffer:v2", hash([]byte("a")), "utf-16", "0", "0"}, nil, nil},
			{"query_version1_line1", "QUERY", query, []string{"file:///a", "buffer:v1", hash([]byte("a")), "utf-16", "1", "0"}, nil, nil},
			{"event_ordinal0", "TARGET_EVENTS", event, []string{"observed-key-1", hash([]byte("[]")), "0"}, nil, nil},
			{"event_ordinal1", "TARGET_EVENTS", event, []string{"observed-key-1", hash([]byte("[]")), "1"}, nil, nil},
			{"query_schema_substitution", "QUERY", query, []string{"file:///a", "buffer:v1", hash([]byte("a")), "utf-16", "0", "0"}, append(append([]byte(nil), originals["schema"]...), '\n'), nil},
			{"query_policy_substitution", "QUERY", query, []string{"file:///a", "buffer:v1", hash([]byte("a")), "utf-16", "0", "0"}, nil, append(append([]byte(nil), policy...), '\n')},
		}
		for _, tc := range cases {
			schema := originals["schema"]
			if tc.schema != nil {
				schema = tc.schema
			}
			p := policy
			if tc.policy != nil {
				p = tc.policy
			}
			fields := append(append([]string(nil), base...), tc.role, hash(schema), hash(originals["transport"]), hash(p), "s", "1", hash(tc.artifact))
			fields = append(fields, tc.extra...)
			key := method + "_" + tc.name
			got := selector(fields...)
			if fixture.Selectors[key] != got {
				t.Errorf("%s differs", key)
			}
			seen[key] = true
		}
		for name, entry := range oldSources.Cases {
			got, err := sourceSelector(method, entry, originals["schema"], originals["transport"], policy)
			if err != nil {
				t.Fatal(err)
			}
			key := method + "_source_" + name
			if fixture.Selectors[key] != got {
				t.Errorf("%s differs", key)
			}
			seen[key] = true
		}
		baseSource := oldSources.Cases["query"]
		for name, change := range map[string]string{"schema_lf": "schema", "policy_lf": "policy"} {
			schema, p := originals["schema"], policy
			if change == "schema" {
				schema = append(append([]byte(nil), schema...), '\n')
			} else {
				p = append(append([]byte(nil), policy...), '\n')
			}
			got, err := sourceSelector(method, baseSource, schema, originals["transport"], p)
			if err != nil {
				t.Fatal(err)
			}
			key := method + "_source_" + name
			if fixture.Selectors[key] != got {
				t.Errorf("%s differs", key)
			}
			seen[key] = true
		}
		if len(oldSources.Rejected) != 7 {
			t.Fatal("missing rejected input coverage")
		}
		for name, entry := range oldSources.Rejected {
			if _, err := sourceSelector(method, entry, originals["schema"], originals["transport"], policy); err == nil {
				t.Errorf("%s/%s invalid source received selector", method, name)
			}
		}
	}
	if len(seen) != 36 {
		t.Fatalf("recomputed %d/36", len(seen))
	}
	for _, sel := range fixture.Selectors {
		for _, old := range previous.Selectors {
			if sel == old {
				t.Fatal("V2 query/event selector collision")
			}
		}
		for _, old := range oldSources.Selectors {
			if sel == old {
				t.Fatal("V2 SOURCE selector collision")
			}
		}
	}
}
