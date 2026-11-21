//go:build darwin || linux

package inventorysuccessor

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"lsp-trace/internal/publication"
	"lsp-trace/internal/source"
)

// Exercise the existing kernel and an actual directory cursor together. This
// fixture is not the production walker: it covers one flat directory only.
func TestPreparationTwoBatchesOneCursorV1(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	for _, name := range []string{"a.go", "b.go", "c.go", "d.go"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	accounting := t.TempDir()
	if err := os.Chmod(accounting, 0700); err != nil {
		t.Fatal(err)
	}
	root, err := publication.OpenRoot(accounting)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	deadline := time.Now().Add(10 * time.Second)
	limits := verificationLimitsV2{128, 4 << 20, 128}
	intent := PrivatePreparationIntentV1{Schema: "private-preparation-intent.v1", SourceRoot: dir, Roots: []string{"."}, Attempt: hashLabelV2("two-batch-run"), Deadline: deadline, Limits: source.Limits{MaxEntries: 128, MaxAccepted: 128, MaxWork: 512, MaxPathBytes: 4096, MaxDepth: 64}}
	q, ok := PrivatePreparationEnumerationQuoteV1(intent, PrivateManagedInputVerificationV2{128, 4 << 20, 128}, time.Now())
	if !ok {
		t.Fatal("fixture quote refused")
	}
	charge := countersFromV2(q)
	j, a, _ := fixtureV2(t)
	j.Scope = PrivatePreparationEnumerationScopeV1()
	j.SourceAuthorization = PrivatePreparationIntentIdentityV1(intent)
	j.ResourceRules = PrivatePreparationEnumerationRulesV1(intent)
	sealTestV2(t, &j)
	a.Job = j.Identity
	a.MaxWindows = 3
	a.MaxTranche = charge
	twice, err := addCountersV2(charge, charge)
	if err != nil {
		t.Fatal(err)
	}
	a.Ceiling, err = addCountersV2(twice, charge)
	if err != nil {
		t.Fatal(err)
	}
	if a.Ceiling.LogicalBytes > 64<<20 || a.Ceiling.Work > 1<<32 {
		t.Fatal("fixture exceeds cumulative ceilings", a.Ceiling)
	}
	a.MaxWindowNS = uint64(15 * time.Second)
	a.Expires = deadline.UnixNano()
	sealTestV2(t, &a)
	k := &kernelV2{root: root}
	head, err := k.initialize(ctx, j, a, decisionV2{j.Identity, a.Identity, a.Decision}, time.Now().UnixNano(), limits)
	if err != nil {
		t.Fatal(err)
	}
	var cursor *os.File
	defer func() {
		if cursor != nil {
			cursor.Close()
		}
	}()
	seen := map[string]bool{}
	for _, batch := range []string{"first", "second"} {
		id := hashLabelV2(batch)
		pending, e := k.reserve(ctx, j, head.Identity, id, charge, deadline.UnixNano(), time.Now().UnixNano(), limits)
		if e != nil {
			t.Fatal(e)
		}
		got, e := k.reopen(ctx, j, limits)
		if e != nil || got.Identity != pending.Identity || got.Disposition != "PENDING" {
			t.Fatal("reservation readback", e)
		}
		if cursor == nil {
			cursor, e = os.Open(dir)
			if e != nil {
				t.Fatal(e)
			}
		}
		entries, e := cursor.ReadDir(2)
		if e != nil || len(entries) != 2 {
			t.Fatal("bounded directory read", len(entries), e)
		}
		for _, entry := range entries {
			if seen[entry.Name()] {
				t.Fatal("directory cursor repeated", entry.Name())
			}
			seen[entry.Name()] = true
		}
		head, e = k.observe(ctx, j, pending.Identity, id, "OBSERVED", time.Now().UnixNano(), limits)
		if e != nil {
			t.Fatal(e)
		}
		got, e = k.reopen(ctx, j, limits)
		if e != nil || got.Identity != head.Identity {
			t.Fatal("completion readback", e)
		}
	}
	if len(seen) != 4 || head.Charged != twice || head.Windows != 2 {
		t.Fatal("two-batch accounting", seen, head.Charged)
	}
	if _, e := k.reserve(ctx, j, head.Identity, hashLabelV2("first"), charge, deadline.UnixNano(), time.Now().UnixNano(), limits); e == nil {
		t.Fatal("completed batch reserved again")
	}
	got, e := k.reopen(ctx, j, limits)
	if e != nil || got.Identity != head.Identity {
		t.Fatal("re-entry changed the accounting head", e)
	}
	// A fresh reservation at the same head isolates replay refusal from budget
	// exhaustion. It performs no directory read and closes as FAILED.
	control := hashLabelV2("fresh-control")
	pending, e := k.reserve(ctx, j, head.Identity, control, charge, deadline.UnixNano(), time.Now().UnixNano(), limits)
	if e != nil {
		t.Fatal("fresh reservation control refused", e)
	}
	if _, e = k.observe(ctx, j, pending.Identity, control, "FAILED", time.Now().UnixNano(), limits); e != nil {
		t.Fatal("fresh reservation control closure", e)
	}
}
