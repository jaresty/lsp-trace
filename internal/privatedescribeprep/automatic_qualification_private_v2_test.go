package privatedescribeprep

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	pilot "lsp-trace/internal/censuslineagepilot"
)

func automaticQualificationCheck(t *testing.T, path string, ok bool, detail any) {
	t.Helper()
	if ok {
		t.Logf("ASSERT_QUAL/%s: PASS %v", path, detail)
	} else {
		t.Errorf("ASSERT_QUAL/%s: FAIL %v", path, detail)
	}
}
func TestAutomaticQualificationFailureRelation(t *testing.T) {
	// Expected values are a literal contract table, not automaticClass(err).
	for _, tc := range []struct {
		name string
		err  error
		want string
	}{
		{"transport", errors.New("owned transport I/O failure"), "UNAVAILABLE"},
		{"cancel", context.Canceled, "CANCELLED"},
		{"deadline", context.DeadlineExceeded, "CANCELLED"},
		{"read-prefix", AutomaticBackendErrorPrivateV2{"UNAVAILABLE"}, "UNAVAILABLE"},
		{"overflow", AutomaticBackendErrorPrivateV2{"BUDGET_EXHAUSTED"}, "BUDGET_EXHAUSTED"},
		{"unsupported", AutomaticBackendErrorPrivateV2{"UNSUPPORTED"}, "UNSUPPORTED"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			i, v := automaticTestPair()
			i.fn = func(AutomaticRequestPrivateV2) ([]byte, error) { return nil, tc.err }
			a, e := AutomatePrivateV2(context.Background(), automaticTestOwner(t, 1), automaticTestLimits(), i, v)
			r := a.Inventory()
			ok := e == nil && len(r.Rows) == 1 && r.Rows[0].Status == tc.want
			automaticQualificationCheck(t, "L/"+tc.name, ok, fmt.Sprint("want=", tc.want, " rows=", r.Rows))
		})
	}
	t.Run("format", func(t *testing.T) {
		i, v := automaticTestPair()
		i.fn = func(AutomaticRequestPrivateV2) ([]byte, error) { return []byte(`{"wrong":true}`), nil }
		a, e := AutomatePrivateV2(context.Background(), automaticTestOwner(t, 1), automaticTestLimits(), i, v)
		r := a.Inventory()
		automaticQualificationCheck(t, "L/format", e == nil && len(r.Rows) == 1 && r.Rows[0].Status == "MALFORMED", r.Rows)
	})
}
func TestAutomaticQualificationRevisionFingerprint(t *testing.T) {
	h := automaticTestOwner(t, 1)
	i, v := automaticTestPair()
	old := i.fn
	var original [][]byte
	i.fn = func(r AutomaticRequestPrivateV2) ([]byte, error) {
		if r.Stage != "INTERPRET" {
			return old(r)
		}
		var raw []byte
		if r.Attempt == 0 {
			raw = []byte("not JSON: original immutable bytes")
		} else {
			raw = automaticTestEncode(automaticTestInterpret(r))
		}
		original = append(original, append([]byte(nil), raw...))
		return raw, nil
	}
	a, e := AutomatePrivateV2(context.Background(), h, automaticTestLimits(), i, v)
	got := a.Inventory()
	if e != nil || len(got.Rows) != 1 {
		t.Fatal("revision fixture setup", e)
	}
	revs := got.Rows[0].Revisions
	automaticQualificationCheck(t, "M2/preserved-attempts", len(revs) == 2 && len(original) == 2, len(revs))
	previous := ""
	for n, r := range revs {
		if n >= len(original) {
			t.Logf("ASSERT_QUAL/M2/revision-%d: UNOBSERVED missing original response", n)
			continue
		}
		rawSHA := pilot.Digest(original[n])
		canonical, _ := json.Marshal(struct{ Previous, RawDigest string }{previous, rawSHA})
		want := pilot.Digest(canonical)
		automaticQualificationCheck(t, fmt.Sprintf("M2/revision-%d/predecessor", n), r.PredecessorDigest == previous, r.PredecessorDigest)
		automaticQualificationCheck(t, fmt.Sprintf("M2/revision-%d/response", n), r.ResponseSHA256 == rawSHA && r.ResponseByteLength == len(original[n]), r.ResponseSHA256)
		automaticQualificationCheck(t, fmt.Sprintf("M2/revision-%d/fingerprint", n), r.Digest == want, r.Digest)
		previous = want
	}
}
