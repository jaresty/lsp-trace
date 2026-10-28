//go:build darwin || linux

package censuslineagepilot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

const partialBudgetConfigV4Private = "lsp-trace.scoped-rehearsal.private.v4"

// Generic deterministic serialized bytes, NOT a historical carrier reconstruction
// or a qualification of the producer/inspection workload.
func TestScopedPartialBudgetV2SerializedPrivate(t *testing.T) {
	for _, tc := range []struct {
		name, schema string
		size         int
		ok           bool
	}{
		{"observed-size-old", "lsp-trace.scoped-rehearsal.private.v3", 3487940, false},
		{"observed-size-new", partialBudgetConfigV4Private, 3487940, true},
		{"exact-new", partialBudgetConfigV4Private, 4 << 20, true},
		{"plus-one-new", partialBudgetConfigV4Private, (4 << 20) + 1, false},
		{"exact-old", "lsp-trace.scoped-rehearsal.private.v3", 2 << 20, true},
		{"plus-one-old", "lsp-trace.scoped-rehearsal.private.v3", (2 << 20) + 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, _, cfg, _ := newScopedRehearsalFixturePrivate(t, tc.schema)
			raw := bytes.Repeat([]byte("x"), tc.size)
			r := h.Retain(context.Background(), "bounded-partial-artifact", raw)
			if !tc.ok {
				if r.Error == "" || r.ID != "" || r.PrimaryReceipt != nil || r.BackupReceipt != nil || h.retainedBytes != 0 || len(h.records) != 0 {
					t.Fatal("ASSERT_CAP_REJECTION_NO_PUBLICATION", r)
				}
				t.Log("ASSERT_CAP_REJECTION_NO_PUBLICATION: PASS")
				// A size refusal must leave the terminal slot/reserve usable.
				outcome := h.Retain(context.Background(), "outcome", []byte(`{"outcome":"INDETERMINATE","failure":"serialized cap"}`))
				if !outcome.Verified || len(h.records) != 1 || outcome.Bytes > 256<<10 {
					t.Fatal("ASSERT_CAP_FAILURE_OUTCOME_PERSISTS", outcome)
				}
				return
			}
			if !r.Verified || r.Bytes != tc.size || r.ID != Digest(raw) || r.Error != "" {
				t.Fatal("ASSERT_SERIALIZED_PAIR", r)
			}
			for _, root := range cfg.OutputRoots[:2] {
				got, e := os.ReadFile(filepath.Join(root, r.ID))
				if e != nil || len(got) != tc.size || Digest(got) != r.ID || !bytes.Equal(got, raw) {
					t.Fatal("ASSERT_SERIALIZED_PAIR_BYTES_HASH", e)
				}
			}
			t.Logf("ASSERT_SERIALIZED_PAIR_BYTES_HASH: PASS size=%d id=%s", r.Bytes, r.ID)
		})
	}
}

func TestScopedPartialBudgetV2AggregatePrivate(t *testing.T) {
	h, _, cfg, _ := newScopedRehearsalFixturePrivate(t, partialBudgetConfigV4Private)
	b := cfg.Budget
	if b != (ScopedRehearsalBudgetPrivate{4 << 20, 16 << 20, 8, 32 << 20, 256 << 20}) || b.StagingBytes != 2*b.TotalBytes || b.ReadHashBytes != 16*b.TotalBytes {
		t.Fatal("ASSERT_FIXED_INDEPENDENT_CAPS", b)
	}
	// Four individually admissible records fill the nonterminal aggregate exactly.
	for i, n := range []int{4 << 20, 4 << 20, 4 << 20, (4 << 20) - (256 << 10)} {
		raw := bytes.Repeat([]byte{byte(i + 1)}, n)
		if r := h.Retain(context.Background(), fmt.Sprint(i), raw); !r.Verified {
			t.Fatal("ASSERT_AGGREGATE_PREFIX", r)
		}
	}
	before := h.retainedBytes
	for _, tc := range []struct {
		name string
		n    int
	}{{"nonterminal", 1}, {"outcome", (256 << 10) + 1}} {
		r := h.Retain(context.Background(), tc.name, bytes.Repeat([]byte("z"), tc.n))
		if r.ID != "" || r.PrimaryReceipt != nil || r.Error == "" || h.retainedBytes != before {
			t.Fatal("ASSERT_TOTAL_OR_RESERVE_PLUS_ONE", r)
		}
	}
	if r := h.Retain(context.Background(), "outcome", bytes.Repeat([]byte("o"), 256<<10)); !r.Verified || h.retainedBytes != 16<<20 {
		t.Fatal("ASSERT_EXACT_TOTAL_OUTCOME", r)
	}
	t.Log("ASSERT_TOTAL_16M_PLUS_ONE_REJECTED: PASS; exact-total=16777216; staging=33554432; read-hash=268435456")
}

func TestScopedPartialBudgetV2ObjectReservePrivate(t *testing.T) {
	h, _, _, _ := newScopedRehearsalFixturePrivate(t, partialBudgetConfigV4Private)
	for i := 0; i < 7; i++ {
		if r := h.Retain(context.Background(), fmt.Sprint(i), []byte{byte(i + 1)}); !r.Verified {
			t.Fatal(r)
		}
	}
	if r := h.Retain(context.Background(), "eighth-nonterminal", []byte("x")); r.Error == "" || r.ID != "" {
		t.Fatal("ASSERT_RESERVED_EIGHTH", r)
	}
	if r := h.Retain(context.Background(), "outcome", []byte("terminal")); !r.Verified || len(h.records) != 8 {
		t.Fatal("ASSERT_EIGHTH_OUTCOME", r)
	}
	if r := h.Retain(context.Background(), "outcome", []byte("ninth")); r.Error == "" || r.ID != "" {
		t.Fatal("ASSERT_NINTH_REFUSED", r)
	}
}

func TestScopedPartialBudgetV2CanonicalPrivate(t *testing.T) {
	h, _, cfg, _ := newScopedRehearsalFixturePrivate(t, partialBudgetConfigV4Private)
	head, e := parseScopedExecutionHeadPrivate(h.owner.HeadBytes())
	if e != nil || head.Schema != scopedExecutionClaimSchemaV3Private || head.Binding.Policy != scopedExecutionPartialInspectionPolicyV2Private() || !bytes.Contains(h.owner.HeadBytes(), []byte(`"serialized_artifact_bytes":4194304`)) {
		t.Fatal("ASSERT_EXPLICIT_CANONICAL_BUDGET_CLAIM", e)
	}
	if limits, e := head.Binding.Policy.discoveryLimits(); e != nil || limits.MaxSymbols != 256 || limits.MaxFiles != 4 || limits.MaxDepth != 64 {
		t.Fatal("ASSERT_DISCOVERY_UNCHANGED", e)
	}
	// The claim selector remains one coordinator slot across every exact policy.
	for _, p := range []scopedExecutionPolicyPrivate{scopedExecutionPolicyFixedPrivate(), scopedExecutionPolicyV2Private(), scopedExecutionPartialInspectionPolicyPrivate()} {
		old, e := newScopedExecutionClaimWithPolicyPrivate(context.Background(), h.restart, cfg.CodeHEAD, cfg.Session, cfg.Workspace, h.roots, p)
		if e != nil || old.selector != h.owner.selector || bytes.Equal(old.raw, h.owner.raw) || bytes.Contains(old.raw, []byte("serialized_artifact_bytes")) {
			t.Fatal("ASSERT_OLD_CLAIM_UNCHANGED_SINGLE_SLOT", e)
		}
	}
	// Parsing requires nontemporary output path syntax, but never opens these paths.
	canonical := cfg
	canonical.OutputRoots = [3]string{"/unopened/private-primary", "/unopened/private-backup", "/unopened/private-coordinator"}
	raw, _ := json.Marshal(canonical)
	if _, e := ParseScopedRehearsalConfigPrivate(raw); e != nil {
		t.Fatal("ASSERT_V4_CANONICAL_CONFIG", e)
	}
	for _, kind := range []string{"v3-new-budget", "v4-old-budget", "total", "objects", "staging", "readhash", "unknown", "extra", "noncanonical"} {
		t.Run(kind, func(t *testing.T) {
			c := canonical
			switch kind {
			case "v3-new-budget":
				c.Schema = "lsp-trace.scoped-rehearsal.private.v3"
			case "v4-old-budget":
				c.Budget = ScopedRehearsalBudgetFixedPrivate()
			case "total":
				c.Budget.TotalBytes++
			case "objects":
				c.Budget.Objects++
			case "staging":
				c.Budget.StagingBytes++
			case "readhash":
				c.Budget.ReadHashBytes++
			case "unknown":
				c.Schema = "lsp-trace.scoped-rehearsal.private.v5"
			}
			raw, _ := json.Marshal(c)
			if kind == "extra" {
				raw = append(raw[:len(raw)-1], []byte(`,"serialized_artifact_bytes":4194304}`)...)
			}
			if kind == "noncanonical" {
				raw = append(raw, '\n')
			}
			if _, e := ParseScopedRehearsalConfigPrivate(raw); e == nil {
				t.Fatal("ASSERT_MIXED_CONFIG_PREOPEN_REFUSAL")
			}
		})
	}
	for _, kind := range []string{"omit-budget", "old-schema", "old-policy", "budget-drift"} {
		t.Run(kind, func(t *testing.T) {
			c := head
			switch kind {
			case "omit-budget":
				c.Binding.Policy.SerializedArtifactBytes = 0
			case "old-schema":
				c.Schema = scopedExecutionClaimSchemaV2Private
			case "old-policy":
				c.Binding.Policy.Name = "one-partial-inspection-private-v1"
			case "budget-drift":
				c.Binding.Policy.SerializedArtifactBytes++
			}
			raw, _ := json.Marshal(c)
			if _, e := parseScopedExecutionHeadPrivate(raw); e == nil {
				t.Fatal("ASSERT_MIXED_CLAIM_REFUSAL")
			}
		})
	}
}
