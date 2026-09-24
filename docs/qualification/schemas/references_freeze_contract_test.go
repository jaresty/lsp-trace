package schemas

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Contract fixture verifier, not a runtime producer, custody, or qualification test.
func freezeHash(b []byte) string { s := sha256.Sum256(b); return "sha256:" + hex.EncodeToString(s[:]) }
func freezeLP(b []byte) []byte {
	out := make([]byte, 8, len(b)+8)
	binary.BigEndian.PutUint64(out, uint64(len(b)))
	return append(out, b...)
}
func freezeID(role string, parts ...[]byte) string {
	all := freezeLP([]byte(role))
	for _, p := range parts {
		all = append(all, freezeLP(p)...)
	}
	return freezeHash(all)
}
func freezeRef(role, selectorRole string, raw []byte) (string, string) {
	digest := freezeHash(append(append([]byte{}, []byte(role)...), append([]byte{0}, raw...)...))
	return "adr0011-references-issuance-v1-" + selectorRole + "-" + strings.TrimPrefix(digest, "sha256:") + ".json", digest
}
func freezeStrict(b []byte) error {
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	if err := freezeValue(d); err != nil {
		return err
	}
	var tail any
	if err := d.Decode(&tail); err != io.EOF {
		return fmt.Errorf("trailing content: %v", err)
	}
	return nil
}
func freezeValue(d *json.Decoder) error {
	tok, err := d.Token()
	if err != nil {
		return err
	}
	switch tok {
	case json.Delim('{'):
		seen := map[string]bool{}
		for d.More() {
			k, err := d.Token()
			if err != nil {
				return err
			}
			key, ok := k.(string)
			if !ok || seen[key] {
				return fmt.Errorf("duplicate or invalid key")
			}
			seen[key] = true
			if err = freezeValue(d); err != nil {
				return err
			}
		}
		_, err = d.Token()
		return err
	case json.Delim('['):
		for d.More() {
			if err := freezeValue(d); err != nil {
				return err
			}
		}
		_, err = d.Token()
		return err
	}
	return nil
}

type freezeEvent struct {
	Sequence    int
	Kind        string
	Ordinal     *int
	Disposition string
}

func freezeIssued(raw []byte, scannerCount int, events []freezeEvent, key, wantKey string, finalVerified bool) (int, int) {
	if !finalVerified || key != wantKey || freezeStrict(raw) != nil {
		return 0, 0
	}
	var value any
	if json.Unmarshal(raw, &value) != nil {
		return 0, 0
	}
	n := 0
	switch v := value.(type) {
	case nil:
	case []any:
		n = len(v)
	default:
		return 0, 0
	}
	if n != scannerCount || len(events) != 2+2*n || events[0].Sequence != 0 || events[0].Kind != "QUERY_BEGIN" || events[0].Ordinal != nil {
		return 0, 0
	}
	for i := 0; i < n; i++ {
		b, e := events[1+2*i], events[2+2*i]
		if b.Sequence != 1+2*i || e.Sequence != 2+2*i || b.Kind != "ELEMENT_BEGIN" || e.Kind != "ELEMENT_TERMINAL" || b.Ordinal == nil || e.Ordinal == nil || *b.Ordinal != i || *e.Ordinal != i || e.Disposition != "VALID_PENDING_ADMISSION" {
			return 0, 0
		}
	}
	end := events[len(events)-1]
	disp := "ITEMS"
	if n == 0 {
		disp = "EMPTY"
	}
	if end.Sequence != len(events)-1 || end.Kind != "QUERY_TERMINAL" || end.Ordinal != nil || end.Disposition != disp {
		return 0, 0
	}
	return 1, n
}
func TestFreezeIdentityAndCounterexamples(t *testing.T) {
	a, b := freezeLP([]byte("a")), freezeLP([]byte("bc"))
	c, d := freezeLP([]byte("ab")), freezeLP([]byte("c"))
	if bytes.Equal(append(a, b...), append(c, d...)) {
		t.Fatal("tuple ambiguity")
	}
	if freezeID("REFERENCES_OCCURRENCE_V1", []byte("a"), []byte("bc")) == freezeID("REFERENCES_OCCURRENCE_V1", []byte("ab"), []byte("c")) {
		t.Fatal("ID collision")
	}
	selector, digest := freezeRef("REFERENCES_RESPONSE_READ_V1", "response-read", []byte(`{"schema_version":"REFERENCES_RESPONSE_READ_V1"}`))
	if !strings.Contains(selector, strings.TrimPrefix(digest, "sha256:")) || strings.Contains(selector, "raw-") {
		t.Fatal("role selector")
	}
	other, _ := freezeRef("REFERENCES_RAW_RESULT_V1", "raw", []byte(`{"schema_version":"REFERENCES_RESPONSE_READ_V1"}`))
	if selector == other {
		t.Fatal("role substitution")
	}
	for _, s := range []string{`{"x":1,"x":2}`, `{"x":1} false`, `{"x":1,"\u0078":2}`} {
		if freezeStrict([]byte(s)) == nil {
			t.Errorf("accepted duplicate/trailing %s", s)
		}
	}
	i, j := 0, 1
	good := []freezeEvent{{0, "QUERY_BEGIN", nil, "NONE"}, {1, "ELEMENT_BEGIN", &i, "NONE"}, {2, "ELEMENT_TERMINAL", &i, "VALID_PENDING_ADMISSION"}, {3, "ELEMENT_BEGIN", &j, "NONE"}, {4, "ELEMENT_TERMINAL", &j, "VALID_PENDING_ADMISSION"}, {5, "QUERY_TERMINAL", nil, "ITEMS"}}
	raw := []byte(`[{},{}]`)
	if x, y := freezeIssued(raw, 2, good, "k", "k", true); x != 1 || y != 2 {
		t.Fatalf("valid %d,%d", x, y)
	}
	cases := []struct {
		name      string
		raw       []byte
		n         int
		events    []freezeEvent
		key, want string
		verified  bool
	}{
		{"no final", raw, 2, good, "k", "k", false}, {"scanner mismatch", raw, 1, good, "k", "k", true}, {"cross invocation", raw, 2, good, "other", "k", true}, {"event omitted", raw, 2, good[:5], "k", "k", true}, {"event reorder", raw, 2, []freezeEvent{good[0], good[2], good[1], good[3], good[4], good[5]}, "k", "k", true}, {"event duplicate", raw, 2, []freezeEvent{good[0], good[1], good[2], good[3], good[3], good[5]}, "k", "k", true}, {"malformed", []byte(`[{},]`), 2, good, "k", "k", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if x, y := freezeIssued(tc.raw, tc.n, tc.events, tc.key, tc.want, tc.verified); x != 0 || y != 0 {
				t.Fatalf("issued %d,%d", x, y)
			}
		})
	}
}
func TestFreezeSelectedPolicyBytes(t *testing.T) {
	names := []string{"method", "admission", "privacy", "retention"}
	versions := []string{"REFERENCES_METHOD_POLICY_V1", "REFERENCES_ADMISSION_POLICY_V1", "LOCAL_QUALIFICATION_PRIVACY_V1", "REFERENCES_RETENTION_POLICY_V1"}
	for i, name := range names {
		b, e := os.ReadFile(filepath.Join("..", "policies", "adr0011-references-"+name+"-policy-v1.proposed.json"))
		if e != nil {
			t.Fatal(e)
		}
		if freezeStrict(b) != nil {
			t.Fatalf("%s strict bytes", name)
		}
		var v map[string]any
		if json.Unmarshal(b, &v) != nil || v["schema_version"] != versions[i] {
			t.Fatalf("%s wrong role/version", name)
		}
		encoded, _ := json.Marshal(v)
		if !bytes.Equal(encoded, b) {
			t.Fatalf("%s noncanonical", name)
		}
		if name == "privacy" && (v["code_disclosure_default"] != "DENY_INCLUDING_ZERO" || v["capture_default"] != "OFF") {
			t.Fatal("privacy defaults")
		}
		if name == "admission" && v["selection"] != "ALL_WELL_FORMED_LOCATIONS_NO_PARTIAL_ADMISSION" {
			t.Fatal("admission selection must exclude partial issuance")
		}
	}
}
