package adr0011methodresult

import (
	"context"
	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/sessionruntime"
	"path/filepath"
	"testing"
	"time"
)

func TestPrivateC16ProjectionRealManagerPath(t *testing.T) {
	root := filepath.Join("testdata", "adr0011-composed-b4-manager-id1-held-v1")
	assets := bridgeManifestAssets(t, filepath.Join(root, "manifest.json"), composedManifestSHA)
	in, _, sources, wants, buildChild, req, owner, profile := composedFixture(t, assets, root, "B")
	req.EnablePrivateC16ObjectAccounting = true
	validated, err := runtimeprofile.Validate(profile)
	if err != nil {
		t.Fatal(err)
	}
	child := buildChild()
	m, err := sessionruntime.New(sessionruntime.Config{Limits: sessionruntime.Limits{MaxSessions: 1, MaxRequests: 1, MaxChildren: 2, MaxCancels: 2, MaxTombstones: 4, MaxObservations: 64, MaxOperations: 2}, Starter: composedStarter{child}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = child.Teardown(context.Background())
		_ = child.Close()
		_ = m.Shutdown(context.Background())
	})
	started := m.Start(context.Background(), sessionruntime.StartRequest{Profile: runtimeprofile.Resolve(validated)})
	composedRequireReadiness(t, m, started)
	result, lease := m.RoundTripPrivateB4(context.Background(), req, owner)
	if result.Failure != "" || result.Key != (lspwire.RequestKey{Generation: 1, ID: 1}) {
		t.Fatal(result.Failure)
	}
	select {
	case err := <-child.observed:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout")
	}
	selection := sessionruntime.B4DefinitionSelectionKey{SessionID: started.SessionID, Key: result.Key, Transaction: owner.Transaction, CompletedOwnerKey: owner.CompletedOwnerKey}
	d := checkPrivateComposedB4DefinitionC16(m, lease, selection, in, sources)
	if d.Version != privateC16ProjectionVersion || d.Count != len(wants) || d.Chronology != "SUPPORTED" || d.Digest == ([32]byte{}) {
		t.Fatalf("ASSERT_C16_REAL_MANAGER %+v", d)
	}
}
