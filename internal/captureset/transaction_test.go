package captureset

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	"lsp-trace/internal/publication"
)

func transactionalFixture(t *testing.T) (Manifest, [][]byte, ExactBytesAuthority) {
	t.Helper()
	targets, _, files, symbols := fixture(64)
	raw := [][]byte{[]byte(`{"schema_version":"lsp-trace.graph-provenance.v5","name":"α"}`), []byte(`{"schema_version":"lsp-trace.graph-provenance.v5","name":"β"}`)}
	a := ExactBytesAuthority{AdmitGraphProvenanceV5: func(b []byte) (string, error) {
		s := sha256.Sum256(b)
		return "native:" + hex.EncodeToString(s[:]), nil
	}}
	cs := make([]Constituent, len(raw))
	for i := range raw {
		var err error
		cs[i], err = a.Constituent(raw[i])
		if err != nil {
			t.Fatal(err)
		}
	}
	m, err := Prepare(targets, cs, files, symbols, "census.v1", "retain-exact.v1")
	if err != nil {
		t.Fatal(err)
	}
	m, err = AssociateBatches(m, cs)
	if err != nil {
		t.Fatal(err)
	}
	return m, raw, a
}

func TestPublishCaptureSetAtomicBundleAndPrivateResolution(t *testing.T) {
	m, raw, authority := transactionalFixture(t)
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	root, err := publication.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	pub := NewPublisher(root)
	result := pub.PublishCaptureSet(m, raw, authority)
	if result.Err != nil {
		t.Fatal(result.Err)
	}
	if !result.Receipt.NamespaceAtomic || result.Receipt.Mechanism != publication.BoundFileMechanism || result.Receipt.ConstituentCount != len(raw) {
		t.Fatalf("receipt: %+v", result.Receipt)
	}
	if runtime.GOOS != "windows" && (result.Receipt.CrashDurability != publication.DirectorySyncComplete || result.Receipt.DirectorySyncStatus != publication.DirectorySyncComplete || result.Receipt.CloseStatus != publication.CloseComplete) {
		t.Fatalf("postcommit status: %+v", result.Receipt)
	}
	for _, c := range m.Constituents {
		got, err := pub.ResolveConstituent(result.Receipt.Selector, c.ImmutableSelector, authority)
		if err != nil {
			t.Fatal(err)
		}
		if sum := sha256.Sum256(got); c.SHA256 != "sha256:"+hex.EncodeToString(sum[:]) {
			t.Fatal("resolved constituent mismatch")
		}
	}
	if info, err := os.Stat(filepath.Join(dir, filepath.FromSlash(result.Receipt.Selector))); err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
		t.Fatalf("bundle mode: %v %v", info, err)
	}
}

func TestResolveCaptureSetRequiresIndependentNativeAdmission(t *testing.T) {
	m, raw, authority := transactionalFixture(t)
	dir := t.TempDir()
	_ = os.Chmod(dir, 0o700)
	root, _ := publication.OpenRoot(dir)
	defer root.Close()
	pub := NewPublisher(root)
	result := pub.PublishCaptureSet(m, raw, authority)
	if result.Err != nil {
		t.Fatal(result.Err)
	}
	if _, err := pub.Verify(result.Receipt.Selector, ExactBytesAuthority{}); err == nil {
		t.Fatal("self-consistent bundle resolved without native V5 admission")
	}
	calls := 0
	counting := ExactBytesAuthority{AdmitGraphProvenanceV5: func(b []byte) (string, error) {
		calls++
		return authority.AdmitGraphProvenanceV5(b)
	}}
	if _, err := pub.Verify(result.Receipt.Selector, counting); err != nil {
		t.Fatal(err)
	}
	if calls != len(m.Constituents) {
		t.Fatalf("native admissions=%d want=%d", calls, len(m.Constituents))
	}
}

func TestResolveCaptureSetRejectsInvalidSelfConsistentConstituent(t *testing.T) {
	m, raw, authority := transactionalFixture(t)
	dir := t.TempDir()
	_ = os.Chmod(dir, 0o700)
	root, _ := publication.OpenRoot(dir)
	defer root.Close()
	pub := NewPublisher(root)
	result := pub.PublishCaptureSet(m, raw, authority)
	if result.Err != nil {
		t.Fatal(result.Err)
	}
	rejecting := ExactBytesAuthority{AdmitGraphProvenanceV5: func([]byte) (string, error) { return "", errors.New("native admission rejected") }}
	if _, err := pub.Verify(result.Receipt.Selector, rejecting); err == nil {
		t.Fatal("invalid self-consistent bundle gained custody")
	}
}

func TestResolveCaptureSetRejectsPostcommitCorruption(t *testing.T) {
	m, raw, authority := transactionalFixture(t)
	dir := t.TempDir()
	_ = os.Chmod(dir, 0o700)
	root, _ := publication.OpenRoot(dir)
	defer root.Close()
	pub := NewPublisher(root)
	result := pub.PublishCaptureSet(m, raw, authority)
	if result.Err != nil {
		t.Fatal(result.Err)
	}
	path := filepath.Join(dir, filepath.FromSlash(result.Receipt.Selector))
	if err := os.WriteFile(path, []byte("corrupt"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := pub.Verify(result.Receipt.Selector, authority); err == nil {
		t.Fatal("corrupt committed bundle resolved")
	}
}

func TestPublishCaptureSetRejectsAssociationWithoutVisibility(t *testing.T) {
	m, raw, authority := transactionalFixture(t)
	dir := t.TempDir()
	_ = os.Chmod(dir, 0o700)
	root, _ := publication.OpenRoot(dir)
	defer root.Close()
	raw[0] = []byte(`{"schema_version":"lsp-trace.graph-provenance.v5","mutated":true}`)
	result := NewPublisher(root).PublishCaptureSet(m, raw, authority)
	if result.Err == nil {
		t.Fatal("association mutation accepted")
	}
	if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(CaptureSetPublicationSelector(m)))); !os.IsNotExist(err) {
		t.Fatalf("visible selector after failure: %v", err)
	}
}

func TestPublishCaptureSetFinalCompetitorHasOneWinner(t *testing.T) {
	m, raw, authority := transactionalFixture(t)
	dir := t.TempDir()
	_ = os.Chmod(dir, 0o700)
	root, _ := publication.OpenRoot(dir)
	defer root.Close()
	pub := NewPublisher(root)
	var wg sync.WaitGroup
	results := make(chan PublicationResult, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); results <- pub.PublishCaptureSet(m, raw, authority) }()
	}
	wg.Wait()
	close(results)
	var success, exists int
	for r := range results {
		if r.Err == nil {
			success++
		} else if r.Code == publication.CodeTargetExists {
			exists++
		}
	}
	if success != 1 || exists != 1 {
		t.Fatalf("success=%d exists=%d", success, exists)
	}
}
