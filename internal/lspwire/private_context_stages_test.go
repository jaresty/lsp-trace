package lspwire

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
)

func TestContext80WireStageBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name, body      string
		partial, refuse bool
		want            []PrivateDiscoveryReadStage
	}{
		{"valid", `{"jsonrpc":"2.0","id":1,"result":[]}`, false, false, []PrivateDiscoveryReadStage{PrivateDiscoveryFrameComplete, PrivateDiscoveryDecodeSucceeded}},
		{"malformed", `{"jsonrpc":`, false, false, []PrivateDiscoveryReadStage{PrivateDiscoveryFrameComplete, PrivateDiscoveryDecodeRejected}},
		{"partial", `{"jsonrpc":`, true, false, nil},
		{"admission-after-body", `{"jsonrpc":"2.0","id":1,"result":[]}`, false, true, []PrivateDiscoveryReadStage{PrivateDiscoveryFrameComplete, PrivateDiscoveryDecodeRefused}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			wire := []byte(fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(tc.body), tc.body))
			if tc.partial {
				wire = wire[:len(wire)-1]
			}
			var stages []PrivateDiscoveryReadStage
			bodyRead := false
			_, _, _, _, _, e := ReadPrivateDiscoveryFrameFacts(context.Background(), bytes.NewReader(wire), Limits{MaxHeaderBytes: 8192, MaxBodyBytes: 1 << 20}, 1<<20, func(PrivateDiscoveryCharge) error {
				if tc.refuse && bodyRead {
					return errors.New("refuse")
				}
				return nil
			}, 1, func(stage PrivateDiscoveryReadStage) {
				stages = append(stages, stage)
				if stage == PrivateDiscoveryFrameComplete {
					bodyRead = true
				}
			})
			if !reflect.DeepEqual(stages, tc.want) {
				t.Fatal("ASSERT80_WIRE_BOUNDARIES", stages, tc.want)
			}
			if tc.name == "valid" && e != nil || tc.name != "valid" && e == nil {
				t.Fatal("wrong result", e)
			}
		})
	}
}
