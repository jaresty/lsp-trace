package captureset

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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

func TestPublishCaptureSetAtomicGenerationAndPrivateResolution(t *testing.T) {
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
	if !result.Receipt.NamespaceAtomic || result.Receipt.Mechanism != publication.DirectoryGenerationMechanism || result.Receipt.ConstituentCount != len(raw) {
		t.Fatalf("receipt: %+v", result.Receipt)
	}
	if result.Receipt.CrashDurability != "FINAL_DIRECTORY_SYNCED_NO_CRASH_GUARANTEE" && runtime.GOOS != "windows" {
		t.Fatalf("durability: %+v", result.Receipt)
	}
	for _, c := range m.Constituents {
		got, err := pub.ResolveConstituent(result.Receipt.Selector, c.ImmutableSelector)
		if err != nil {
			t.Fatal(err)
		}
		if sum := sha256.Sum256(got); c.SHA256 != "sha256:"+hex.EncodeToString(sum[:]) {
			t.Fatal("resolved constituent mismatch")
		}
	}
	generation := strings.TrimSuffix(result.Receipt.Selector, "/manifest.json")
	if info, err := os.Stat(filepath.Join(dir, filepath.FromSlash(generation))); err != nil || info.Mode().Perm() != 0o700 {
		t.Fatalf("generation mode: %v %v", info, err)
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

func TestPublishGenerationRejectsUnsafeNames(t *testing.T) {
	dir := t.TempDir()
	_ = os.Chmod(dir, 0o700)
	root, _ := publication.OpenRoot(dir)
	defer root.Close()
	for _, name := range []string{"../escape", "/absolute", "a/../b", ""} {
		_, err := publication.PublishGeneration(publication.GenerationRequest{Root: root, FinalSelector: "final-" + hex.EncodeToString([]byte(name)), Files: []publication.GenerationFile{{Name: name, Bytes: []byte("x")}}})
		if err == nil {
			t.Fatalf("accepted %q", name)
		}
	}
}
