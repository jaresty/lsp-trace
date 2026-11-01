//go:build darwin || linux

package privatedescribeprep

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestRetainedCheckpointHeaderBeforeMarshalPrivateV3(t *testing.T) {
	for _, kind := range []string{"assessment", "checkpoint", "source-family", "source-provider", "cancel"} {
		t.Run(kind, func(t *testing.T) {
			r := OwnASTReportRetainedCheckpointPrivateV3{}
			ctx := context.Background()
			maxBytes := 32768
			switch kind {
			case "assessment":
				maxBytes = 2048
				r.Assessment = json.RawMessage(`"` + strings.Repeat("a", 2048) + `"`)
			case "checkpoint":
				r.Source.CheckpointSHA256 = strings.Repeat("\x00<&\"", 4096)
			case "source-family":
				r.Source.InputFamily = strings.Repeat("\x00<&\"", 4096)
			case "source-provider":
				r.Source.ProviderCommand = strings.Repeat("\x00<&\"", 4096)
			case "cancel":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			calls := 0
			_, e := marshalOwnASTCheckpointHeaderPrivateV3(ctx, r, maxBytes, func(v any) ([]byte, error) { calls++; return json.Marshal(v) })
			if e == nil || calls != 0 {
				t.Fatalf("ASSERT_HEADER_REFUSED_BEFORE_MARSHAL: calls=%d error=%v", calls, e)
			}
			t.Log("ASSERT_HEADER_REFUSED_BEFORE_MARSHAL: PASS")
		})
	}
}
func TestRetainedCheckpointHeaderExactJSONPrivateV3(t *testing.T) {
	for _, s := range []string{"", "<&>\"\n\t\x00", "π日本語\u2028\u2029"} {
		r := OwnASTReportRetainedCheckpointPrivateV3{RequestType: s, Assessment: json.RawMessage(`{"ok":true}`), SourceInputID: s}
		r.Source.InputFamily = s
		r.Source.CheckpointSHA256 = s
		r.Source.ProviderCommand = s
		metadata := r
		metadata.Assessment = nil
		want, e := json.Marshal(metadata)
		if e != nil {
			t.Fatal(e)
		}
		calls := 0
		got, e := marshalOwnASTCheckpointHeaderPrivateV3(context.Background(), r, 2<<20, func(v any) ([]byte, error) { calls++; return json.Marshal(v) })
		if e != nil || calls != 1 || !bytes.Equal(got, want) {
			t.Fatal("ASSERT_HEADER_VALID_EXACT_JSON", calls, e)
		}
	}
}
