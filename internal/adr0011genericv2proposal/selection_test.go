package adr0011genericv2proposal

import (
	"fmt"
	"strings"
	"testing"
)

// Contract-only oracle for selected SOURCE metadata; deliberately not an issuer,
// replay implementation, admission validator, or imported production code.
type selectedSource struct{ selector, transaction string }

func checkSelectedSources(heldTransaction string, sources []selectedSource) error {
	if heldTransaction == "" {
		return fmt.Errorf("held transaction absent")
	}
	if len(sources) < 1 || len(sources) > 256 {
		return fmt.Errorf("SOURCE cardinality %d outside 1..256", len(sources))
	}
	seen := make(map[string]bool, len(sources))
	for _, s := range sources {
		if s.transaction != heldTransaction {
			return fmt.Errorf("cross-transaction SOURCE selection")
		}
		if s.selector == "" || seen[s.selector] {
			return fmt.Errorf("duplicate or empty SOURCE selector")
		}
		seen[s.selector] = true
	}
	return nil
}

func TestV2SelectedSourceMetadataBoundaries(t *testing.T) {
	for _, method := range []string{"references", "definition"} {
		t.Run(method, func(t *testing.T) {
			sources := make([]selectedSource, 256)
			for i := range sources {
				sources[i] = selectedSource{selector: fmt.Sprintf("%s:source:%d", method, i), transaction: "held-tx"}
			}
			if err := checkSelectedSources("held-tx", sources); err != nil {
				t.Fatalf("256 distinct SOURCEs rejected: %v", err)
			}
			if err := checkSelectedSources("held-tx", sources[:1]); err != nil {
				t.Fatalf("one query SOURCE rejected: %v", err)
			}
			extra := append(append([]selectedSource{}, sources...), selectedSource{selector: method + ":source:256", transaction: "held-tx"})
			if err := checkSelectedSources("held-tx", extra); err == nil || !strings.Contains(err.Error(), "SOURCE cardinality") {
				t.Fatalf("257 SOURCEs accepted: %v", err)
			}
			duplicate := append([]selectedSource{}, sources...)
			duplicate[1].selector = duplicate[0].selector
			if err := checkSelectedSources("held-tx", duplicate); err == nil || !strings.Contains(err.Error(), "duplicate") {
				t.Fatalf("duplicate SOURCE selector accepted: %v", err)
			}
			foreign := append([]selectedSource{}, sources...)
			foreign[1].transaction = "foreign-tx"
			if err := checkSelectedSources("held-tx", foreign); err == nil || !strings.Contains(err.Error(), "cross-transaction") {
				t.Fatalf("foreign SOURCE transaction accepted: %v", err)
			}
		})
	}
}
