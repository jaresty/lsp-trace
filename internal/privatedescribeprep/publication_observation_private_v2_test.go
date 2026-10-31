package privatedescribeprep

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestPublicationObservationJSONPrivateV2(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		valid     bool
	}{
		{"ordered-data", `{"Data":[null,[],[1,1],9007199254740993,1e-100,{"v":1},{"v":1}]}`, true},
		{"duplicate-object", `{"Data":{"v":1,"v":2}}`, false},
		{"escaped-key", `{"Data":{"v":1,"\u0076":2}}`, false},
		{"nested-array-object", `{"Data":[{"v":1,"v":1}]}`, false},
		{"trailing", `{"Data":null}{}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := []byte(tc.raw)
			e := observationUniqueKeysPrivateV2(context.Background(), raw)
			if (e == nil) != tc.valid {
				t.Fatalf("ASSERT_OBSERVATION_OBJECT_KEYS_%s: FAIL %v", tc.name, e)
			}
			if tc.valid {
				var v struct{ Data json.RawMessage }
				if e = observationClosedPrivateV2(raw, &v); e != nil {
					t.Fatal(e)
				}
				got, _ := json.Marshal(v)
				if !bytes.Equal(raw, got) {
					t.Fatal("exact lexemes/ordered data changed")
				}
			}
			t.Logf("ASSERT_OBSERVATION_OBJECT_KEYS_%s: PASS", tc.name)
		})
	}
	t.Run("unknown", func(t *testing.T) {
		var e postPublicationEnvelopePrivateV2
		if observationClosedPrivateV2([]byte(`{"Unknown":null}`), &e) == nil {
			t.Fatal("unknown envelope field accepted")
		}
	})
	t.Run("omitted", func(t *testing.T) {
		var e postPublicationEnvelopePrivateV2
		if observationClosedPrivateV2([]byte(`{}`), &e) == nil {
			t.Fatal("omitted original fields accepted")
		}
	})
	t.Run("cancel", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if observationUniqueKeysPrivateV2(ctx, []byte(`{}`)) == nil {
			t.Fatal("cancel ignored")
		}
	})
	t.Run("depth", func(t *testing.T) {
		if observationUniqueKeysPrivateV2(context.Background(), []byte(strings.Repeat("[", 514)+"0"+strings.Repeat("]", 514))) == nil {
			t.Fatal("nesting ignored")
		}
	})
}
