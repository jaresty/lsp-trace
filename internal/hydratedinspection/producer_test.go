package hydratedinspection

import (
	"errors"
	he "lsp-trace/internal/hydratedevidence"
	"testing"
)

func TestPublicProducerValidation(t *testing.T) {
	r := fixture(t)
	r.NodeIDs = []string{"unknown"}
	r.IncludeBodies = true
	for _, kind := range []string{"error", "manifest", "core"} {
		t.Run(kind, func(t *testing.T) {
			_, err := inspectWithProducer(r, func(in he.Input, f he.FocusRequest) (he.FocusResult, error) {
				result, e := he.HydrateFocused(in, f)
				if e != nil {
					return result, e
				}
				switch kind {
				case "error":
					return result, errors.New("producer failed")
				case "manifest":
					result.Manifest.Origins = result.Manifest.Origins[1:]
				case "core":
					result.Bundle.Spans[0].Content = []byte("forged")
				}
				return result, nil
			})
			if err == nil {
				t.Fatalf("PUBLIC_PRODUCER_VALIDATION FAIL: %s", kind)
			}
			t.Logf("PUBLIC_PRODUCER_VALIDATION PASS: %s", kind)
		})
	}
}
