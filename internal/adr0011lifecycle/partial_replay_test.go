package adr0011lifecycle

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func partialFixture(t *testing.T) (*syntheticRoot, []byte, []byte) {
	t.Helper()
	parent := canonicalSyntheticTempDir(t)
	root := filepath.Join(parent, "private")
	anchor := filepath.Join(parent, "anchor")
	for _, p := range []string{root, anchor} {
		if err := os.Mkdir(p, 0700); err != nil {
			t.Fatal(err)
		}
	}
	l, err := openSynthetic(root, anchor)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.close() })
	schema, err := os.ReadFile(filepath.Join("..", "..", "docs", "qualification", "schemas", "adr0011-lifecycle-v2.proposed.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	policy, err := os.ReadFile(filepath.Join("..", "..", "docs", "qualification", "policies", "adr0011-lifecycle-v2.proposed.json"))
	if err != nil {
		t.Fatal(err)
	}
	return l, schema, policy
}
func TestPartialTwoProcessStaleAndKilledFence(t *testing.T) {
	if !supportedLock {
		t.Skip("OS lock unsupported")
	}
	l, schema, policy := partialFixture(t)
	if err := l.provisionPartial(schema, policy); err != nil {
		t.Fatal(err)
	}
	cached, err := l.partialSnapshot(schema, policy)
	if err != nil {
		t.Fatal(err)
	}
	ready := filepath.Join(filepath.Dir(l.root.Path()), "fenced")
	cmd := exec.Command(os.Args[0], "-test.run=^TestPartialFenceHelper$")
	cmd.Env = append(os.Environ(), "ADR_PARTIAL_HELPER=1", "ADR_PARTIAL_ROOT="+l.root.Path(), "ADR_PARTIAL_ANCHOR="+l.anchor.Path(), "ADR_PARTIAL_READY="+ready)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	until := time.Now().Add(4 * time.Second)
	for {
		if _, err := os.Stat(ready); err == nil {
			break
		}
		if time.Now().After(until) {
			t.Fatal("fence helper did not commit")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()
	if err := l.partialCheckSyntheticUse(schema, policy, cached); err == nil || !strings.Contains(err.Error(), "stale") {
		t.Fatalf("ASSERT_TWO_PROCESS_STALE_HEAD_DENIED: %v", err)
	}
	fresh, err := openSynthetic(l.root.Path(), l.anchor.Path())
	if err != nil {
		t.Fatal(err)
	}
	defer fresh.close()
	latest, err := fresh.partialSnapshot(schema, policy)
	if err != nil {
		t.Fatal(err)
	}
	if err := fresh.partialCheckSyntheticUse(schema, policy, latest); err == nil || !strings.Contains(err.Error(), "fence") {
		t.Fatalf("ASSERT_FENCE_SURVIVES_KILL_RESTART: %v", err)
	}
}
func TestPartialFenceHelper(t *testing.T) {
	if os.Getenv("ADR_PARTIAL_HELPER") != "1" {
		return
	}
	l, err := openSynthetic(os.Getenv("ADR_PARTIAL_ROOT"), os.Getenv("ADR_PARTIAL_ANCHOR"))
	if err != nil {
		t.Fatal(err)
	}
	defer l.close()
	schema, err := os.ReadFile(filepath.Join("..", "..", "docs", "qualification", "schemas", "adr0011-lifecycle-v2.proposed.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	policy, err := os.ReadFile(filepath.Join("..", "..", "docs", "qualification", "policies", "adr0011-lifecycle-v2.proposed.json"))
	if err != nil {
		t.Fatal(err)
	}
	previous, err := l.partialSnapshot(schema, policy)
	if err != nil {
		t.Fatal(err)
	}
	if err := l.partialFence(schema, policy, previous); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(os.Getenv("ADR_PARTIAL_READY"), []byte("done"), 0600); err != nil {
		t.Fatal(err)
	}
	for {
		time.Sleep(time.Second)
	}
}
