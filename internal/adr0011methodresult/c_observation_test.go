package adr0011methodresult

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"lsp-trace/internal/adr0011cobserve"
	v5 "lsp-trace/internal/adr0011genericv5proposal"
	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/publication"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/internal/session"
	"lsp-trace/sessionruntime"
)

// The subset manifest is derived solely from exact held A/B originals; each
// listed asset is checked by both byte length and SHA-256 before any C control.
const (
	cABManifestSHA    = "69aa9f2857111f93ab4b3d74104d6d3b3f1faeb349d95e1451d6ebe61c8e0ee0"
	cABManifestLength = 7770
	cABAssetCount     = 49
)

func cTrackedABAssets(t *testing.T) (string, map[string]bridgeAsset) {
	t.Helper()
	root := filepath.Join("testdata", "adr0011-c-ab-"+composedManifestSHA)
	manifest := filepath.Join(root, "manifest.json")
	if len(bridgePinnedBytes(t, manifest, cABManifestSHA)) != cABManifestLength {
		bridgeFixtureFatal(t, "BLOCKED_NOT_RED: tracked A/B manifest length")
	}
	assets := bridgeManifestAssets(t, manifest, cABManifestSHA)
	if len(assets) != cABAssetCount {
		bridgeFixtureFatal(t, "BLOCKED_NOT_RED: tracked A/B asset count")
	}
	for name := range assets {
		_ = bridgeAssetBytes(t, root, name, assets)
	}
	return root, assets
}

type cProbe struct {
	mu       sync.Mutex
	counts   [5]int
	order    [16]adr0011cobserve.Stage
	n        int
	overflow bool
}

func (p *cProbe) observe(stage adr0011cobserve.Stage) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.n >= len(p.order) || stage < adr0011cobserve.DecodeEntry || stage > adr0011cobserve.PrivateCandidatePublishEntry {
		p.overflow = true
		return
	}
	p.order[p.n] = stage
	p.n++
	p.counts[stage]++
}
func (p *cProbe) snapshot() ([5]int, []adr0011cobserve.Stage, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.counts, append([]adr0011cobserve.Stage(nil), p.order[:p.n]...), p.overflow
}

func cFixtureManager(t *testing.T, profile runtimeprofile.Selector, child *composedChild, req sessionruntime.RoundTripRequest) (*sessionruntime.Manager, sessionruntime.StartResult) {
	t.Helper()
	validated, err := runtimeprofile.Validate(profile)
	if err != nil {
		bridgeFixtureFatal(t, "BLOCKED_NOT_RED: profile: %v", err)
	}
	manager, err := sessionruntime.New(sessionruntime.Config{Limits: sessionruntime.Limits{MaxSessions: 1, MaxRequests: 1, MaxChildren: 2, MaxCancels: 2, MaxTombstones: 4, MaxObservations: 64, MaxOperations: 2}, Starter: composedStarter{child}})
	if err != nil {
		bridgeFixtureFatal(t, "BLOCKED_NOT_RED: manager: %v", err)
	}
	t.Cleanup(func() {
		_ = child.Teardown(context.Background())
		_ = child.Close()
		_ = manager.Shutdown(context.Background())
	})
	started := manager.Start(context.Background(), sessionruntime.StartRequest{Profile: runtimeprofile.Resolve(validated)})
	if started.SessionID != req.SessionID || started.Generation != req.Generation {
		bridgeFixtureFatal(t, "BLOCKED_NOT_RED: identity")
	}
	composedRequireReadiness(t, manager, started)
	return manager, started
}
func cExactWrite(t *testing.T, child *composedChild) {
	t.Helper()
	select {
	case err := <-child.observed:
		if err != nil {
			bridgeFixtureFatal(t, "BLOCKED_NOT_RED: exact WRITE/READ: %v", err)
		}
	case <-time.After(4 * time.Second):
		bridgeFixtureFatal(t, "BLOCKED_NOT_RED: exact WRITE/READ timeout")
	}
}
func cPrivateRoot(t *testing.T) (*publication.Root, string) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "private")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	root, err := publication.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = root.Close() })
	return root, dir
}

// Test-only conditional path, never a production B4 or D orchestration.
func cContinue(result sessionruntime.RoundTripResult, lease sessionruntime.B4DefinitionLease, next func()) bool {
	if result.Failure != "" || result.ServerError != nil || lease == (sessionruntime.B4DefinitionLease{}) {
		return false
	}
	next()
	return true
}
func cSelection(started sessionruntime.StartResult, result sessionruntime.RoundTripResult, owner sessionruntime.B4DefinitionOwner) sessionruntime.B4DefinitionSelectionKey {
	return sessionruntime.B4DefinitionSelectionKey{SessionID: started.SessionID, Key: result.Key, Transaction: owner.Transaction, CompletedOwnerKey: owner.CompletedOwnerKey}
}

// The successor mirrors the frozen adapter; each A/B comparison uses a separate
// manager because the original and test successor both consume the lease once.
func TestCObservedB4SuccessorEquivalenceAndOrder(t *testing.T) {
	fixtureRoot, assets := cTrackedABAssets(t)
	for _, name := range []string{"A", "B"} {
		t.Run(name, func(t *testing.T) {
			in, occurrence, sources, wants, buildChild, req, owner, profile := composedFixture(t, assets, fixtureRoot, name)
			probe := &cProbe{}
			child := buildChild()
			manager, started := cFixtureManager(t, profile, child, req)
			result, lease := manager.RoundTripPrivateB4(adr0011cobserve.With(context.Background(), probe.observe), req, owner)
			cExactWrite(t, child)
			if result.Failure != "" || lease == (sessionruntime.B4DefinitionLease{}) {
				bridgeFixtureFatal(t, "BLOCKED_NOT_RED: selected private READ %s", result.Failure)
			}
			candidateRoot, _ := cPrivateRoot(t)
			raw, _ := retentionFixture(t) // unrelated synthetic private publisher control
			var successor PrivateB4Decision
			var receipt *publication.BoundFileReceipt
			var publishErr error
			continued := cContinue(result, lease, func() {
				successor = cObservedB4(probe.observe, manager, lease, cSelection(started, result, owner), in, occurrence, sources)
				if successor.Status == DefinitionBridgeCandidateItems {
					receipt, publishErr = publishPrivateCandidateObserved(func() { adr0011cobserve.Notify(probe.observe, adr0011cobserve.PrivateCandidatePublishEntry) }, candidateRoot, "synthetic.json", raw)
				}
			})
			counts, order, overflow := probe.snapshot()
			if !continued || overflow || counts[adr0011cobserve.DecodeEntry] != 1 || counts[adr0011cobserve.RetainEntry] == 0 || counts[adr0011cobserve.B4EvaluationEntry] != 1 || counts[adr0011cobserve.PrivateCandidatePublishEntry] != 1 || len(order) < 4 || order[0] != adr0011cobserve.RetainEntry || order[len(order)-2] != adr0011cobserve.B4EvaluationEntry || order[len(order)-1] != adr0011cobserve.PrivateCandidatePublishEntry || receipt == nil || publishErr != nil || successor.Status != DefinitionBridgeCandidateItems || len(successor.Candidates) != len(wants) || successor.Authority != 0 || successor.Accepted || successor.Completeness != "UNKNOWN" {
				t.Fatalf("OBSERVATION_SHARED_ORDER: continued=%v counts=%v order=%v overflow=%v decision=%+v receipt=%+v err=%v", continued, counts, order, overflow, successor, receipt, publishErr)
			}
			// Independently run the unchanged frozen adapter with a fresh matched lease.
			originalChild := buildChild()
			originalManager, originalStarted := cFixtureManager(t, profile, originalChild, req)
			req.Deadline = time.Now().Add(4 * time.Second)
			originalResult, originalLease := originalManager.RoundTripPrivateB4(context.Background(), req, owner)
			cExactWrite(t, originalChild)
			if originalResult.Failure != "" || originalLease == (sessionruntime.B4DefinitionLease{}) {
				bridgeFixtureFatal(t, "BLOCKED_NOT_RED: original B4 control %s", originalResult.Failure)
			}
			original := CheckPrivateComposedB4Definition(originalManager, originalLease, cSelection(originalStarted, originalResult, owner), in, occurrence, sources)
			if !reflect.DeepEqual(successor, original) {
				t.Fatalf("OBSERVATION_SUCCESSOR_DIFFERS_FROM_PINNED_B4: successor=%+v original=%+v", successor, original)
			}
		})
	}
}

func cMalformedChild(t *testing.T, in v5.B4bFullCandidateInput) (*composedChild, int64) {
	t.Helper()
	body := []byte(`{"jsonrpc":"2.0","id":1,"result":[`)
	wire := []byte(fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(body), body))
	input, stdin := io.Pipe()
	stdout, output := io.Pipe()
	child := &composedChild{input: input, stdin: stdin, output: output, stdout: stdout, observed: make(chan error, 1)}
	go func() {
		reader := lspwire.NewReader(input, lspwire.DefaultLimits())
		if err := composedServeReadiness(reader, output); err != nil {
			child.observed <- err
			return
		}
		_, _, frame, retained, err := reader.ReadWithFrameIfWithin(4096)
		if err != nil || !retained || !bytes.Equal(frame, in.Write.RequestFrame) {
			child.observed <- fmt.Errorf("exact WRITE: %v", err)
			return
		}
		_, err = output.Write(wire)
		child.observed <- err
	}()
	return child, int64(len(wire))
}
func TestCFailedManagerReadSkipsSuccessorAndPrivatePublisher(t *testing.T) {
	fixtureRoot, assets := cTrackedABAssets(t)
	in, occurrence, sources, _, _, req, owner, profile := composedFixture(t, assets, fixtureRoot, "A")
	child, cap := cMalformedChild(t, in)
	req.CaptureDefinitionResponseFrameMaxBytes = cap
	manager, started := cFixtureManager(t, profile, child, req)
	probe := &cProbe{}
	result, lease := manager.RoundTripPrivateB4(adr0011cobserve.With(context.Background(), probe.observe), req, owner)
	cExactWrite(t, child)
	if result.RequestMessages != 1 || result.Failure != session.SessionPoisoned || lease != (sessionruntime.B4DefinitionLease{}) {
		bridgeFixtureFatal(t, "BLOCKED_NOT_RED: reached malformed READ %s", result.Failure)
	}
	privateRoot, dir := cPrivateRoot(t)
	raw, _ := retentionFixture(t)
	continued := cContinue(result, lease, func() {
		decision := cObservedB4(probe.observe, manager, lease, cSelection(started, result, owner), in, occurrence, sources)
		if decision.Status == DefinitionBridgeCandidateItems {
			_, _ = publishPrivateCandidateObserved(func() { adr0011cobserve.Notify(probe.observe, adr0011cobserve.PrivateCandidatePublishEntry) }, privateRoot, "synthetic.json", raw)
		}
	})
	counts, _, overflow := probe.snapshot()
	if continued || overflow || counts[adr0011cobserve.DecodeEntry] != 1 || counts[adr0011cobserve.RetainEntry] == 0 || counts[adr0011cobserve.B4EvaluationEntry] != 0 || counts[adr0011cobserve.PrivateCandidatePublishEntry] != 0 {
		t.Fatalf("OBSERVATION_FAILED_MANAGER: continued=%v counts=%v overflow=%v", continued, counts, overflow)
	}
	if _, err := os.Stat(filepath.Join(dir, "synthetic.json")); !os.IsNotExist(err) {
		t.Fatalf("OBSERVATION_FAILED_MANAGER_PUBLISHED: %v", err)
	}
}

func TestCPrivatePublisherNotificationPanicNeutral(t *testing.T) {
	root, _ := cPrivateRoot(t)
	raw, _ := retentionFixture(t)
	probe := &cProbe{}
	receipt, err := publishPrivateCandidateObserved(func() { adr0011cobserve.Notify(probe.observe, adr0011cobserve.PrivateCandidatePublishEntry) }, root, "candidate.json", raw)
	counts, _, overflow := probe.snapshot()
	if err != nil || receipt == nil || counts[adr0011cobserve.PrivateCandidatePublishEntry] != 1 || overflow {
		t.Fatalf("OBSERVATION_PRIVATE_PUBLISH_ENTRY: counts=%v overflow=%v err=%v", counts, overflow, err)
	}
	_, err = publishPrivateCandidateObserved(func() { panic("observer only") }, root, "candidate.json", raw)
	if !errors.Is(err, os.ErrExist) {
		t.Fatalf("OBSERVATION_PANIC_NEUTRAL: %v", err)
	}
}
