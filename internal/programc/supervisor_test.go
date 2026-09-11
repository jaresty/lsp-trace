package programc

import (
	"bytes"
	"context"
	"encoding/json"
	"runtime"
	"strings"
	"testing"
)

func TestPrivateWorkerStrictProtocolBounds(t *testing.T) {
	for _, tc := range []struct {
		name  string
		input string
	}{
		{"unknown", `{"version":"lsp-trace.private-program-c-worker.v1","input":null,"seed":1,"extra":true}`},
		{"multiple", `{"version":"lsp-trace.private-program-c-worker.v1","input":null,"seed":1}{}`},
		{"version", `{"version":"wrong","input":null,"seed":1}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			handled, code := RunPrivateWorker([]string{workerArgument}, strings.NewReader(tc.input), &out, &bytes.Buffer{})
			if !handled || code != 0 {
				t.Fatalf("ASSERT_PRIVATE_PROTOCOL_%s handled=%t code=%d", tc.name, handled, code)
			}
			var response workerResponse
			if err := json.Unmarshal(out.Bytes(), &response); err != nil || response.Failure == nil || response.Failure.Code != CodeInternalAdmission {
				t.Fatalf("ASSERT_PRIVATE_PROTOCOL_%s response=%s err=%v", tc.name, out.String(), err)
			}
		})
	}
	if handled, _ := RunPrivateWorker([]string{"--help"}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{}); handled {
		t.Fatal("ASSERT_PRIVATE_PROTOCOL_UNADVERTISED")
	}
	t.Log("ASSERT_PRIVATE_PROTOCOL_BOUNDS: PASS")
}

func TestUnsupportedPlatformIsTypedBeforeExecution(t *testing.T) {
	if runtime.GOOS == "darwin" && runtime.GOARCH == "arm64" {
		t.Skip("unsupported implementation is compile-selected")
	}
	_, observation, failed := ComputeSupervised(context.Background(), nil, 1)
	if failed == nil || failed.Code != CodePlatformUnsupported || observation.Ceiling != ObservationCeiling {
		t.Fatalf("ASSERT_PLATFORM_UNSUPPORTED_PREEXECUTION: failure=%v observation=%+v", failed, observation)
	}
	t.Log("ASSERT_PLATFORM_UNSUPPORTED_PREEXECUTION: PASS")
}

func TestObservationCeilingIsQualified(t *testing.T) {
	for _, phrase := range []string{"samples aggregate descendant process-tree RSS", "every 20ms", "not continuous kernel-backed", "cannot detect an allocation wholly between snapshots"} {
		if !strings.Contains(ObservationCeiling, phrase) {
			t.Fatalf("ASSERT_SAMPLING_CEILING missing %q", phrase)
		}
	}
	t.Log("ASSERT_SAMPLING_CEILING: PASS")
}
