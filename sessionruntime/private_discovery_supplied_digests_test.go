package sessionruntime

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/runtimeprofile"
)

// The mirror allows the pre-export runtime to compile and reach the actual
// release assertion rather than mistaking an undefined field for RED.
type suppliedDigestMirror struct {
	Available bool
	Count     uint8
	SHA256    [16][32]byte
}

func suppliedDigestObservation(t *testing.T, o PrivateDiscoveryObservation) suppliedDigestMirror {
	t.Helper()
	raw, e := json.Marshal(o)
	if e != nil {
		t.Fatal(e)
	}
	var decoded struct{ SuppliedDocuments suppliedDigestMirror }
	if e = json.Unmarshal(raw, &decoded); e != nil {
		t.Fatal(e)
	}
	return decoded.SuppliedDocuments
}
func suppliedDigestConstruction(t *testing.T, n int) (PrivateDiscoveryConstruction, [][32]byte) {
	t.Helper()
	root, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	p, e := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: "test", Workspace: root, Profile: "go", EnvironmentReference: "local"})
	if e != nil {
		t.Fatal(e)
	}
	c := PrivateDiscoveryConstruction{Start: StartRequest{Profile: runtimeprofile.Resolve(p), LanguageID: "go"}, Deadline: time.Now().Add(10 * time.Second), Limits: discoveryLimitsTest(), NewStarter: func() (Starter, error) { return oneChildStarter{newDiscoveryPipeChild()}, nil }}
	var expected [][32]byte
	for j := 0; j < n; j++ {
		path := filepath.Join(root, fmt.Sprintf("paid-%02d.go", j))
		body := []byte(fmt.Sprintf("package paid\r\n// document %d — exact bytes\n", j))
		if e = os.WriteFile(path, body, 0600); e != nil {
			t.Fatal(e)
		}
		expected = append(expected, sha256.Sum256(body))
		c.Members = append(c.Members, PrivateDiscoveryMember{Path: filepath.Base(path), URI: (&url.URL{Scheme: "file", Path: path}).String(), LanguageID: "go"})
	}
	return c, expected
}
func TestPrivateDiscoverySuppliedDigests(t *testing.T) {
	c, expected := suppliedDigestConstruction(t, 16)
	var window *privateDiscoveryWindow
	var sourceAfterSupply uint64
	o, e := RunPrivateDiscovery(context.Background(), c, func(ctx context.Context, s *PrivateDiscoverySession) error {
		window = s.client.w
		for _, f := range c.Members {
			if e := s.Supply(ctx, f.URI, f.LanguageID); e != nil {
				return e
			}
			if e := os.Remove(filepath.Join(c.Start.Profile.Workspace().String(), f.Path)); e != nil {
				return e
			}
		}
		window.mu.Lock()
		sourceAfterSupply = window.remaining.SourceBytes
		window.mu.Unlock()
		return nil
	})
	if e != nil || o.Failed || !o.CleanupJoined {
		t.Fatal("SETUP_PAID_DIGEST_DISCOVERY", e, o)
	}
	d := suppliedDigestObservation(t, o)
	if !d.Available || d.Count != 16 {
		t.Fatal("ASSERT_PAID_DIGEST_RELEASE FAIL successful close withheld data", d.Available, d.Count)
	}
	t.Log("ASSERT_PAID_DIGEST_RELEASE: PASS")
	for j, want := range expected {
		if d.SHA256[j] != want {
			t.Fatal("ASSERT_PAID_DIGEST_BINDING FAIL paid bytes replaced", j)
		}
	}
	t.Log("ASSERT_PAID_DIGEST_BINDING: PASS")
	window.mu.Lock()
	after := window.remaining.SourceBytes
	window.mu.Unlock()
	if after != sourceAfterSupply {
		t.Fatal("ASSERT_PAID_DIGEST_METADATA FAIL additional source debit")
	}
	d.SHA256[0][0] ^= 255
	if suppliedDigestObservation(t, o).SHA256[0] != expected[0] {
		t.Fatal("ASSERT_PAID_DIGEST_METADATA FAIL aliased data")
	}
	t.Log("ASSERT_PAID_DIGEST_METADATA: PASS")
}
func TestPrivateDiscoverySuppliedDigestRefusals(t *testing.T) {
	for _, kind := range []string{"partial", "consumer-error", "unknown-cleanup"} {
		t.Run(kind, func(t *testing.T) {
			c, _ := suppliedDigestConstruction(t, 2)
			if kind == "unknown-cleanup" {
				c.NewStarter = func() (Starter, error) { return oneChildStarter{discoveryUnknownCleanup{newDiscoveryPipeChild()}}, nil }
			}
			o, e := RunPrivateDiscovery(context.Background(), c, func(ctx context.Context, s *PrivateDiscoverySession) error {
				for j, f := range c.Members {
					if kind == "partial" && j == 1 {
						break
					}
					if e := s.Supply(ctx, f.URI, f.LanguageID); e != nil {
						return e
					}
				}
				if kind == "consumer-error" {
					return errors.New("controlled consumer refusal")
				}
				return nil
			})
			if e == nil || !o.Failed {
				t.Fatal("SETUP_PAID_DIGEST_REFUSAL", kind, e)
			}
			d := suppliedDigestObservation(t, o)
			if d != (suppliedDigestMirror{}) {
				t.Fatal("ASSERT_PAID_DIGEST_RELEASE FAIL unsuccessful lifetime released data", kind)
			}
			t.Log("ASSERT_PAID_DIGEST_RELEASE: PASS suppressed " + kind)
		})
	}
}
func TestPrivateDiscoverySuppliedDigestAdmission(t *testing.T) {
	c, _ := suppliedDigestConstruction(t, 1)
	// The exact fixed export allowance includes eight scalar copies and eight
	// bounded 4096-byte serializations. One byte short must stop before Starter.
	const exportAllowance = uint64(8*(2+16*32) + 8*4096)
	backend := uint64(managedprocess.PrivateProviderStderrReadLimit + managedprocess.PrivateProviderStderrScratch + 16384)
	c.Limits.RetainedBytes = (4 << 20) + backend + privateResponseFactsCharge + exportAllowance - 1
	c.Limits.Work = 1 << 32
	starts := 0
	c.NewStarter = func() (Starter, error) { starts++; return nil, errors.New("controlled constructor") }
	_, e := RunPrivateDiscovery(context.Background(), c, func(context.Context, *PrivateDiscoverySession) error { return nil })
	if e == nil || starts != 0 {
		t.Fatal("ASSERT_PAID_DIGEST_ADMISSION FAIL export storage unpaid before constructor", starts, e)
	}
	t.Log("ASSERT_PAID_DIGEST_ADMISSION: PASS")
}
