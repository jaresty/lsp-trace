package strictjson

import "testing"

func TestRejectDuplicates(t *testing.T) {
	for _, raw := range []string{`{"x":1,"x":2}`, `{"outer":[{"x":1,"x":2}]}`, `{"x":1,"\u0078":2}`, `{"x":{"y":1,"y":2}}`, `{} {}`, `{"x":`} {
		if err := RejectDuplicates([]byte(raw)); err == nil {
			t.Errorf("accepted %s", raw)
		}
	}
	for _, raw := range []string{`{}`, `[{"x":1},{"x":2}]`, `{"x":1,"X":2}`, `{"x":1e9999}`, `null`, `[true,false,{},[]]`} {
		if err := RejectDuplicates([]byte(raw)); err != nil {
			t.Errorf("rejected %s: %v", raw, err)
		}
	}
}
