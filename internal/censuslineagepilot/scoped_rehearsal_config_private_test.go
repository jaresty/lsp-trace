//go:build darwin || linux

package censuslineagepilot

import (
	"fmt"
	"strings"
	"testing"
)

func TestScopedRehearsalSharedPureConfigPrivate(t *testing.T) {
	_, _, cfg, _ := NewScopedPartialExact25V4RehearsalFixturePrivate(t)
	if e := ValidateScopedRehearsalConfigPurePrivate(cfg); e != nil {
		t.Fatal("ASSERT_PURE_CONFIG_VALID", e)
	}
	for _, tc := range []struct {
		name   string
		change func(*ScopedRehearsalConfigPrivate)
		valid  bool
	}{
		{"schema-v1", func(c *ScopedRehearsalConfigPrivate) {
			c.Schema = "lsp-trace.scoped-rehearsal.private.v1"
			c.Budget = ScopedRehearsalBudgetFixedPrivate()
		}, true},
		{"schema-v2", func(c *ScopedRehearsalConfigPrivate) {
			c.Schema = "lsp-trace.scoped-rehearsal.private.v2"
			c.Budget = ScopedRehearsalBudgetFixedPrivate()
		}, true},
		{"schema-v3", func(c *ScopedRehearsalConfigPrivate) {
			c.Schema = "lsp-trace.scoped-rehearsal.private.v3"
			c.Budget = ScopedRehearsalBudgetFixedPrivate()
		}, true},
		{"schema-invalid", func(c *ScopedRehearsalConfigPrivate) { c.Schema = "unknown" }, false},
		{"profile-empty", func(c *ScopedRehearsalConfigPrivate) { c.Profile = "" }, false},
		{"trust-empty", func(c *ScopedRehearsalConfigPrivate) { c.TrustDomain = "" }, false},
		{"envref-invalid", func(c *ScopedRehearsalConfigPrivate) { c.EnvironmentReference = "/invalid" }, false},
		{"environment-unsorted", func(c *ScopedRehearsalConfigPrivate) { c.Environment = []string{"B=1", "A=1"} }, false},
		{"environment-duplicate", func(c *ScopedRehearsalConfigPrivate) { c.Environment = []string{"A=1", "A=2"} }, false},
		{"environment-no-equals", func(c *ScopedRehearsalConfigPrivate) { c.Environment = []string{"A"} }, false},
		{"environment-invalid-name", func(c *ScopedRehearsalConfigPrivate) { c.Environment = []string{"1A=x"} }, false},
		{"environment-nul", func(c *ScopedRehearsalConfigPrivate) { c.Environment = []string{"A=\x00"} }, false},
		{"environment-empty", func(c *ScopedRehearsalConfigPrivate) { c.Environment = []string{} }, false},
		{"environment-at-count-cap", func(c *ScopedRehearsalConfigPrivate) {
			c.Environment = nil
			for i := 0; i < 32; i++ {
				c.Environment = append(c.Environment, fmt.Sprintf("A%02d=1", i))
			}
		}, true},
		{"environment-over-count", func(c *ScopedRehearsalConfigPrivate) { c.Environment = make([]string, 33) }, false},
		{"environment-at-byte-cap", func(c *ScopedRehearsalConfigPrivate) { c.Environment = []string{"A=" + strings.Repeat("x", 4094)} }, true},
		{"environment-over-bytes", func(c *ScopedRehearsalConfigPrivate) { c.Environment = []string{"A=" + strings.Repeat("x", 4095)} }, false},
		{"args-at-count-cap", func(c *ScopedRehearsalConfigPrivate) { c.Args = make([]string, 16) }, true},
		{"args-over-count", func(c *ScopedRehearsalConfigPrivate) { c.Args = make([]string, 17) }, false},
		{"args-at-byte-cap", func(c *ScopedRehearsalConfigPrivate) { c.Args = []string{strings.Repeat("x", 4096)} }, true},
		{"args-over-bytes", func(c *ScopedRehearsalConfigPrivate) { c.Args = []string{strings.Repeat("x", 4097)} }, false},
		{"args-nul", func(c *ScopedRehearsalConfigPrivate) { c.Args = []string{"\x00"} }, false},
		{"server-relative", func(c *ScopedRehearsalConfigPrivate) { c.Server = "relative" }, false},
		{"server-noncanonical", func(c *ScopedRehearsalConfigPrivate) { c.Server = "/fixture/../server" }, false},
		// Syntax and comparison values only: no requirement that a server exists.
		{"server-not-opened", func(c *ScopedRehearsalConfigPrivate) { c.Server = "/does-not-exist-retained-validation/server" }, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := cfg
			tc.change(&c)
			pure, producer := ValidateScopedRehearsalConfigPurePrivate(c), c.validate(false)
			if (pure == nil) != tc.valid || (producer == nil) != tc.valid {
				t.Fatalf("ASSERT_SHARED_CONFIG_CONTRACT: pure=%v producer=%v want=%v", pure, producer, tc.valid)
			}
		})
	}
}
