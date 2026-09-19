package targetpacket

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"testing"
)

// Property [10]: persistent tests map exactly to properties [1]-[9] — every
// retained property has at least one assertion and every assertion maps to a
// retained property. This is a coverage gate over the suite itself: it scans
// the test sources for ASSERT_P<n>_ tags and checks the bijection against the
// declared retained-property set. It fails if a property gains no test or a
// test tags a property outside the retained set.
func TestPropertyAssertionBijection(t *testing.T) {
	// The retained, testable properties for this package. [10] itself is this
	// gate and is intentionally not a runtime-behavioural property.
	retained := map[string]bool{
		"P1": true, // one nomination bound, authority 0, accepted false
		"P2": true, // custody <-> Graph V5, fails closed
		"P3": true, // selected node -> exactly one logical source
		"P4": true, // lookup-only source resolution
		"P5": true, // bounded, typed resolve + assemble
		"P6": true, // deterministic packet identity
		"P7": true, // EMPTY/UNRESOLVED/PREPARED/FAILED distinct
		"P8": true, // defensive clone / input isolation
		"P9": true, // census result not mutated
	}

	tagRE := regexp.MustCompile(`ASSERT_(P\d+)_`)
	seen := map[string]bool{}
	surplus := map[string]bool{}

	files, err := filepath.Glob("*_test.go")
	if err != nil || len(files) == 0 {
		t.Fatalf("ASSERT_P10_TEST_SOURCES_FOUND: files=%v err=%v", files, err)
	}
	for _, f := range files {
		if f == "coverage_test.go" {
			continue // don't count the gate's own tag literals
		}
		body, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range tagRE.FindAllStringSubmatch(string(body), -1) {
			prop := m[1]
			if retained[prop] {
				seen[prop] = true
			} else {
				surplus[prop] = true
			}
		}
	}

	// Every retained property must have at least one assertion (coverage gap).
	var gaps []string
	for prop := range retained {
		if !seen[prop] {
			gaps = append(gaps, prop)
		}
	}
	sort.Strings(gaps)
	if len(gaps) > 0 {
		t.Fatalf("ASSERT_P10_EVERY_PROPERTY_HAS_ASSERTION: uncovered=%v", gaps)
	}

	// Every assertion tag must map to a retained property (surplus assertion).
	var extra []string
	for prop := range surplus {
		extra = append(extra, prop)
	}
	sort.Strings(extra)
	if len(extra) > 0 {
		t.Fatalf("ASSERT_P10_EVERY_ASSERTION_MAPS_TO_PROPERTY: surplus=%v", extra)
	}
}
