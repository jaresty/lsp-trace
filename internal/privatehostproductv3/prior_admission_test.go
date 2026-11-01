//go:build darwin || linux

package privatehostproductv3

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

// None_Model: deterministic local publication/custody tests, not model judgment.
func TestHostProductPriorQuartetTopology(t *testing.T) {
	for _, mode := range []string{"second_machine", "human", "outside", "symlink", "hardlink", "mode", "noncanonical", "counts", "profile", "lifecycle", "empty_complete"} {
		t.Run(mode, func(t *testing.T) {
			c := testConfig(t)
			if e := Initialize(c); e != nil {
				t.Fatal(e)
			}
			prior, e := DeliverEarlyFailure(c, "INITIAL", "CONTROLLED")
			if e != nil {
				t.Fatal(e)
			}
			retained := map[string][]byte{}
			for _, p := range append(prior.MachinePaths[:], prior.HumanPaths[:]...) {
				raw, e := os.ReadFile(filepath.Join(c.Store, p))
				if e != nil {
					t.Fatal(e)
				}
				retained[p] = raw
			}
			s, e := openStore(c)
			if e != nil {
				t.Fatal(e)
			}
			fake := prior
			fake.ReportRevision = 2
			fake.MachinePaths, fake.HumanPaths = reportPaths(3)
			if mode == "outside" {
				fake.HumanPaths[0] = "/outside/inventory.md"
			}
			if mode == "counts" {
				fake.Completed = 1
			}
			if mode == "profile" {
				fake.Profile = "FOREIGN"
			}
			if mode == "lifecycle" {
				fake.Lifecycle.OriginalStopCalls = -1
			}
			raw := mustJSON(fake)
			if mode == "noncanonical" {
				raw = append([]byte(" "), raw...)
			}
			mp, hp := reportPaths(3)
			for _, p := range mp {
				if e = s.write(p, raw); e != nil {
					t.Fatal(e)
				}
			}
			for _, p := range hp {
				if e = s.write(p, human(fake)); e != nil {
					t.Fatal(e)
				}
			}
			s.close()
			target := filepath.Join(c.Store, mp[1])
			switch mode {
			case "second_machine":
				if e = os.WriteFile(target, []byte("{}"), 0600); e != nil {
					t.Fatal(e)
				}
			case "human":
				if e = os.WriteFile(filepath.Join(c.Store, hp[1]), []byte("wrong"), 0600); e != nil {
					t.Fatal(e)
				}
			case "symlink", "hardlink":
				if e = os.Remove(target); e != nil {
					t.Fatal(e)
				}
				if mode == "symlink" {
					e = os.Symlink(filepath.Join(c.Store, mp[0]), target)
				} else {
					e = os.Link(filepath.Join(c.Store, mp[0]), target)
				}
				if e != nil {
					t.Fatal(e)
				}
			case "mode":
				if e = os.Chmod(target, 0262); e != nil {
					t.Fatal(e)
				}
			}
			// Preserve the exact pre-call state, including intentionally bad fixtures.
			for _, p := range append(mp[:], hp[:]...) {
				if mode == "mode" && p == mp[1] {
					retained[p] = raw
					continue
				}
				b, e := os.ReadFile(filepath.Join(c.Store, p))
				if e != nil {
					t.Fatal(e)
				}
				retained[p] = b
			}
			got, e := Continue(context.Background(), c, Transfer{}, nil, nil)
			if e == nil || got.Profile != Profile || got.Status != "FAILED" || got.Accepted || got.Authority != 0 || got.SourceCompleteness != "UNKNOWN" || got.FeatureIdentity != "UNRESOLVED" || got.AbsenceInference != "PROHIBITED" {
				t.Fatal("ASSERT_SAFE_QUALIFIED_DTO", e)
			}
			if mode == "symlink" || mode == "hardlink" || mode == "mode" {
				if got.MachinePaths != [2]string{} || got.HumanPaths != [2]string{} || got.Admission.PriorMachinePaths != [2]string{} {
					t.Fatal("ASSERT_INVALID_STORE_NO_PAIRS")
				}
			} else {
				want := prior.MachinePaths
				if mode == "empty_complete" {
					want = mp
				}
				if got.Admission.PriorMachinePaths != want {
					t.Fatal("ASSERT_ONLY_COMPLETE_PRIOR", got.Admission.PriorMachinePaths, want)
				}
				for _, paths := range [][2]string{got.Admission.PriorMachinePaths, got.Admission.PriorHumanPaths} {
					a, e := os.ReadFile(filepath.Join(c.Store, paths[0]))
					b, e2 := os.ReadFile(filepath.Join(c.Store, paths[1]))
					if e != nil || e2 != nil || !bytes.Equal(a, b) {
						t.Fatal("ASSERT_PRIOR_EXACT_COPIES_EXIST")
					}
				}
			}
			if mode == "mode" {
				st, err := os.Stat(target)
				if err != nil || st.Mode().Perm() != 0262 {
					t.Fatal("ASSERT_PRIVATE_MODE_UNCHANGED")
				}
				// Test-only restoration permits byte readback after refusal.
				if err := os.Chmod(target, 0600); err != nil {
					t.Fatal(err)
				}
			}
			for p, before := range retained {
				after, e := os.ReadFile(filepath.Join(c.Store, p))
				if e != nil || !bytes.Equal(before, after) {
					t.Fatal("ASSERT_OLD_FOUR_VIEWS_UNCHANGED", p)
				}
			}
		})
	}
}
