package adr0011genericproposal

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

func originals(t *testing.T) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "docs", "qualification", "originals")
}
func load(t *testing.T, name string) map[string]any {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(originals(t), name))
	if err != nil {
		t.Fatal(err)
	}
	var v map[string]any
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatal(err)
	}
	return v
}
func TestProposedRoleDiscriminator(t *testing.T) {
	doc := load(t, "adr0011-generic-envelope-v1.proposed.schema.json")
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	id := doc["$id"].(string)
	if err := c.AddResource(id, doc); err != nil {
		t.Fatal(err)
	}
	schema, err := c.Compile(id)
	if err != nil {
		t.Fatal(err)
	}
	digest := "sha256:" + strings.Repeat("0", 64)
	orig := map[string]any{"length": 1, "sha256": digest, "private_ref": "private:one"}
	base := map[string]any{"role": "QUERY", "identity": map[string]any{"session": "s", "generation": 1, "transaction": "t"}, "original": orig, "predecessors": []any{map[string]any{"role": "POLICY", "selector": "p", "digest": digest}, map[string]any{"role": "SCHEMA", "selector": "s", "digest": digest}, map[string]any{"role": "SOURCE", "selector": "src", "digest": digest}}, "payload": map[string]any{"uri": "file:///a", "version": "1", "method": "textDocument/references", "line": 0, "character": 0, "encoding": "utf-16", "source": digest}}
	check := func(name string, valid bool) {
		t.Helper()
		b, _ := json.Marshal(base)
		var v any
		_ = json.Unmarshal(b, &v)
		err := schema.Validate(v)
		if (err == nil) != valid {
			t.Fatalf("%s valid=%v error=%v", name, valid, err)
		}
	}
	check("query", true)
	base["role"] = "SOURCE"
	check("role substitution", false)
	base["role"] = "QUERY"
	base["payload"].(map[string]any)["source_bytes"] = orig
	check("payload cross substitution", false)
	delete(base["payload"].(map[string]any), "source_bytes")
	base["payload"].(map[string]any)["surplus"] = true
	check("nested surplus", false)
}
func TestResultReadPresenceConditional(t *testing.T) {
	doc := load(t, "adr0011-generic-envelope-v1.proposed.schema.json")
	c := jsonschema.NewCompiler()
	id := doc["$id"].(string)
	if err := c.AddResource(id, doc); err != nil {
		t.Fatal(err)
	}
	schema, err := c.Compile(id)
	if err != nil {
		t.Fatal(err)
	}
	digest := "sha256:" + strings.Repeat("0", 64)
	original := map[string]any{"length": 1, "sha256": digest, "private_ref": "private:one"}
	dependency := func(role string) any {
		return map[string]any{"role": role, "selector": "selected:" + role, "digest": digest}
	}
	base := map[string]any{"role": "RESULT_READ", "identity": map[string]any{"session": "s", "generation": 1, "transaction": "t"}, "original": original, "predecessors": []any{dependency("INBOUND_FRAMES"), dependency("REQUEST_WRITE")}, "payload": map[string]any{"actual_key": "1", "matched_frame_index": 0, "result_present": true, "result_token": nil, "parse": "WHOLE_PARSE_SUCCESS"}}
	check := func(name string, valid bool) {
		t.Helper()
		b, _ := json.Marshal(base)
		var v any
		if err := json.Unmarshal(b, &v); err != nil {
			t.Fatal(err)
		}
		err := schema.Validate(v)
		if (err == nil) != valid {
			t.Fatalf("%s valid=%v error=%v", name, valid, err)
		}
	}
	check("present-null-token", false)
	p := base["payload"].(map[string]any)
	p["result_token"] = map[string]any{"length": 2, "sha256": digest, "private_ref": "private:result"}
	check("present-token", true)
	base["predecessors"] = []any{dependency("INBOUND_FRAMES")}
	check("missing-write-predecessor", false)
	base["predecessors"] = []any{dependency("INBOUND_FRAMES"), dependency("REQUEST_WRITE")}
	p["result_present"] = false
	p["result_token"] = nil
	check("absent-whole-success", false)
	p["parse"] = "MALFORMED"
	check("absent-malformed", true)
}
func TestSelectorIndependentFixture(t *testing.T) {
	files := []string{"adr0011-generic-envelope-v1.proposed.schema.json", "generic-lsp-exact-transport-v1.proposed.json", "generic-lsp-references-exact-v1.proposed.json"}
	fields := []string{"ADR0011-GENERIC-EXACT/PROPOSED/1", "GENERIC_LSP_REFERENCES_EXACT_V1", "textDocument/references", "QUERY"}
	for _, name := range files {
		b, err := os.ReadFile(filepath.Join(originals(t), name))
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(b)
		fields = append(fields, hex.EncodeToString(sum[:]))
	}
	sum := sha256.Sum256([]byte("a"))
	source := hex.EncodeToString(sum[:])
	fields = append(fields, "s", "1", source)
	hash := func(parts []string) string {
		h := sha256.New()
		for _, s := range parts {
			var length [8]byte
			binary.BigEndian.PutUint64(length[:], uint64(len([]byte(s))))
			h.Write(length[:])
			h.Write([]byte(s))
		}
		return hex.EncodeToString(h.Sum(nil))
	}
	for _, tc := range []struct {
		version string
		line    int
		want    string
	}{{"1", 0, "e8342490d8614a73a4af9fd4057b7e2941c6a14ba543973d555cf2d5a41eaa30"}, {"2", 0, "344e3405a72f2379dcf46b16c334af9178b011dc384acdf13f348ca74bc970bc"}, {"1", 1, "97c413af05963eaf41ed5677aab3833ddcd396ed6f3ea3f0f906215514a878da"}} {
		parts := append(append([]string{}, fields...), "file:///a", tc.version, source, "utf-16", strconv.Itoa(tc.line), "0")
		if got := hash(parts); got != tc.want {
			t.Errorf("query version=%s line=%d: %s", tc.version, tc.line, got)
		}
	}
	result := sha256.Sum256([]byte("[]"))
	resultHex := hex.EncodeToString(result[:])
	fields[3] = "TARGET_EVENTS"
	fields[len(fields)-1] = resultHex
	for i, want := range []string{"aab477070cd2e11be36b46d156841966f136e64c64f564dbdf256cb4d7a34460", "1560cfafd84abf990f6be1286091be9fcdf547c35a536808874311d15b2b91e6"} {
		parts := append(append([]string{}, fields...), "key-1", resultHex, strconv.Itoa(i))
		if got := hash(parts); got != want {
			t.Errorf("ordinal %d: %s", i, got)
		}
	}
}
func TestProposedPolicyLimitsAndAggregate(t *testing.T) {
	p := load(t, "generic-lsp-exact-transport-v1.proposed.json")
	numbers := map[string]float64{"frame_bytes": 2097152, "partial_header_bytes": 65536, "inbound_messages": 64, "cumulative_wire_bytes": 8388608, "request_ms": 15000, "transaction_ms": 60000, "source_documents": 256, "source_document_bytes": 4194304, "logical_owned_buffer_bytes": 33554432, "mandatory_retained_transaction_bytes": 67108864, "objects": 4096, "events": 8192, "same_process_access_elapsed_ms_exclusive": 86400000, "optional_private_executable_snapshot_bytes": 536870912}
	for k, want := range numbers {
		if p[k] != want {
			t.Errorf("%s=%v want %v", k, p[k], want)
		}
	}
	if _, ok := p["work_units"]; ok {
		t.Fatal("generic work units forbidden")
	}
	if p["cross_process_access"] != false || p["optional_snapshot_gates_admission"] != false {
		t.Fatal("process/snapshot gate")
	}
	for _, name := range []string{"generic-lsp-references-exact-v1.proposed.json", "generic-lsp-definition-exact-v1.proposed.json"} {
		m := load(t, name)
		if m["raw_result_token_bytes"] != float64(524288) {
			t.Fatal(name, "result")
		}
		want := float64(1048576)
		if strings.Contains(name, "references") {
			want = 1572864
		}
		if m["decoded_message_remarshal_bytes"] != want {
			t.Fatal(name, "decoded")
		}
	}
	// Deliberately category-valid aggregate: 17 documents of 4 MiB each exceed the independent 64 MiB quota.
	if 17*4194304 <= int(p["mandatory_retained_transaction_bytes"].(float64)) || 17 > int(p["source_documents"].(float64)) {
		t.Fatal("aggregate/category discriminator")
	}
	if 67108864+1 <= int(p["mandatory_retained_transaction_bytes"].(float64)) {
		t.Fatal("quota +1")
	}
}
