package privatedescribeprep

import (
	"context"
	"runtime"
	"strings"
	"testing"
)

func TestSelectionBoundsBeforeMapCopyOrAssessmentSerialization(t *testing.T) {
	limits := Limits{MaxTargets: 64, MaxPartBytes: 64 << 10, MaxTotalBytes: 8 << 20, MaxParts: 256, MaxSourceBytes: 2 << 20}
	huge := strings.Repeat("x", 1<<20)
	cases := []struct {
		name   string
		inputs []Input
	}{
		{"inputs", make([]Input, 65)},
		{"target-count", []Input{{Targets: make([]string, 65537)}}},
		{"label-bytes", []Input{{Targets: []string{huge}}}},
		{"label-utf8", []Input{{Targets: []string{string([]byte{255})}}}},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			runtime.GC()
			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			_, err := Prepare(context.Background(), tt.inputs, limits)
			runtime.ReadMemStats(&after)
			if err == nil {
				t.Fatal("selection admitted")
			}
			if delta := after.TotalAlloc - before.TotalAlloc; delta > 128<<10 {
				t.Fatalf("precheck allocated %d bytes", delta)
			}
			// The selection error precedes even the intentionally absent projection seal.
			if !strings.Contains(err.Error(), "bound") && !strings.Contains(err.Error(), "UTF8") && !strings.Contains(err.Error(), "invalid private preparation invocation") {
				t.Fatalf("selection not checked first: %v", err)
			}
		})
	}
}
