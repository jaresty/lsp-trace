package boundedanalysis

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/retainedcalls"
)

// Persistent assertion imported from the independent TestReviewerEmbeddedDepth.
func TestReviewerEmbeddedDepth(t *testing.T) {
	raw, _ := retainedFixture(t)
	base := evidence(t, raw, Parameters{Operation: "PROJECT"})
	deep := []byte(`{"x":` + strings.Repeat("[", 80) + `{"dup":0,"dup":1}` + strings.Repeat("]", 80) + `}`)
	for _, layer := range []string{"provenance", "graph", "retained"} {
		t.Run(layer, func(t *testing.T) {
			var r retainedcalls.Evidence
			if err := json.Unmarshal(raw, &r); err != nil {
				t.Fatal(err)
			}
			if layer == "graph" {
				var p graphprovenance.Evidence
				if err := json.Unmarshal(r.InputBytes, &p); err != nil {
					t.Fatal(err)
				}
				p.GraphBytes = deep
				p.GraphDigest = hash(graphprovenance.Version+":graph", deep)
				r.InputBytes, _ = json.Marshal(p)
			} else {
				r.InputBytes = deep
			}
			r.InputDigest = hash(retainedcalls.Version+":input", r.InputBytes)
			input, _ := json.Marshal(r)
			if layer == "retained" {
				input = deep
			}
			if layer != "retained" {
				if err := Preflight(input, MaxInputBytes); err != nil {
					t.Fatal(err)
				}
			}
			_, err := Analyze(context.Background(), input, Parameters{Operation: "PROJECT"})
			requireDepth(t, err)
			e := base
			e.InputBytes = input
			encoded, _ := json.Marshal(e)
			_, err = ValidateFor(encoded, Family, "v1")
			requireDepth(t, err)
		})
	}
}
func TestBoundedDecodedCarrier(t *testing.T) {
	for _, field := range []string{"input_bytes", "graph_bytes"} {
		for _, data := range [][]byte{[]byte("a"), []byte("ab"), []byte("abc"), []byte("abcd")} {
			encoded := base64.StdEncoding.EncodeToString(data)
			for _, text := range []string{encoded, "\r\n" + encoded + "\n"} {
				value, _ := json.Marshal(text)
				raw := []byte(`{"` + strings.ToUpper(field) + `":` + string(value) + `}`)
				got, err := decodedCarrier(raw, field, len(data))
				if err != nil || !bytes.Equal(got, data) {
					t.Fatalf("carrier compatibility: %q %v", got, err)
				}
				if _, err = decodedCarrier(raw, field, len(data)-1); err == nil || !strings.Contains(err.Error(), "byte LIMIT") {
					t.Fatalf("carrier cap: %v", err)
				}
			}
		}
		for _, value := range []string{`null`, `0`, `[]`, `{}`, `"!"`, `"="`, `"a==="`, `"YQ"`} {
			if _, err := decodedCarrier([]byte(`{"`+field+`":`+value+`}`), field, 8); err == nil {
				t.Fatalf("accepted malformed %s", value)
			}
		}
		for _, first := range []string{`"!"`, `null`, `"e30="`} {
			raw := []byte(`{"` + field + `":` + first + `,"` + strings.ToUpper(field) + `":"e30="}`)
			if err := Preflight(raw, MaxBytes); err != nil {
				t.Fatal(err)
			}
			if _, err := decodedCarrier(raw, field, 8); err == nil || !strings.Contains(err.Error(), "duplicate JSON carrier") {
				t.Fatalf("hidden earlier carrier: %v", err)
			}
		}
	}
	for _, raw := range []string{`{}`, `{"input_bytes":""}`, `{"input_bytes":"bnVsbA=="}`} {
		if err := preflightAdmission([]byte(raw), false); err == nil {
			t.Fatalf("accepted empty chain %s", raw)
		}
	}
}

func requireDepth(t *testing.T, err error) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), "nesting LIMIT") || strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("want nesting LIMIT before duplicate/schema validation, got %v", err)
	}
}
