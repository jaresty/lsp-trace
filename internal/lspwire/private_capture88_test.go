package lspwire

import (
	"bytes"
	"context"
	"testing"
)

func TestCapture88OriginalSpan(t *testing.T) {
	for _, tc := range []struct{ body, object string }{
		{`{"jsonrpc":"2.0","id":7,"error": {"code":0,"message":"PRIVATE_OBJECT"} }`, `{"code":0,"message":"PRIVATE_OBJECT"}`},
		{`{"jsonrpc":"2.0","id":7,"result":["PRIVATE_RESULT"],"error":{"code":0}}`, `{"code":0}`},
		{`{"jsonrpc":"2.0","id":7,"error":{"code":true}}`, `{"code":true}`},
		{`{"jsonrpc":"2.0","id":7,"error":null,"result":[]}`, ""},
		{`{"jsonrpc":"2.0","id":7,"error":{"code":0},"error":null,"result":[]}`, ""},
		{`{"jsonrpc":"2.0","id":7,"result":[]}`, ""},
	} {
		frame := discoveryTestFrame([]byte(tc.body))
		var span PrivateDiscoveryErrorObjectSpan
		_, _, raw, _, facts, e := ReadPrivateDiscoveryFrameCapture(context.Background(), bytes.NewReader(frame), Limits{1 << 20, 8192}, 1<<20, func(PrivateDiscoveryCharge) error { return nil }, 7, &span, nil)
		if facts.Association != ResponseIDMatched {
			t.Fatal("ASSERT_CAPTURE88_SPAN_ASSOCIATION")
		}
		if tc.object == "" {
			if span != (PrivateDiscoveryErrorObjectSpan{}) {
				t.Fatal("ASSERT_CAPTURE88_NO_ERROR_SPAN")
			}
		} else if span.Start < 0 || span.End > len(frame) || string(frame[span.Start:span.End]) != tc.object {
			t.Fatal("ASSERT_CAPTURE88_ORIGINAL_OBJECT_SPAN")
		}
		if e == nil && len(raw) == 0 {
			t.Fatal("ASSERT_CAPTURE88_OWNED_FRAME")
		}
		if e != nil && raw != nil {
			t.Fatal("ASSERT_CAPTURE88_REJECTION_COMPATIBILITY")
		}
	}
	var span PrivateDiscoveryErrorObjectSpan
	_, _, _, _, facts, _ := ReadPrivateDiscoveryFrameCapture(context.Background(), bytes.NewReader(discoveryTestFrame([]byte(`{"id":7,"error":{`))), Limits{1 << 20, 8192}, 1<<20, func(PrivateDiscoveryCharge) error { return nil }, 7, &span, nil)
	if span != (PrivateDiscoveryErrorObjectSpan{}) || facts.State != ResponseInvalidJSON {
		t.Fatal("ASSERT_CAPTURE88_MALFORMED_INELIGIBLE")
	}
}
