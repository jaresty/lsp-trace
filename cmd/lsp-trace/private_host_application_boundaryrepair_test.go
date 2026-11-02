//go:build darwin

package main

import (
	"context"
	pilot "lsp-trace/internal/censuslineagepilot"
	product "lsp-trace/internal/privatehostproductv3"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestHostApplicationBoundaryExpiredAcquire(t *testing.T) {
	for _, kind := range []string{"unit", "application", "near-deadline", "invalid-contract"} {
		t.Run(kind, func(t *testing.T) {
			r, path, base := hostApplicationCLIFixture(t)
			tool := buildApplicationTool(t, base)
			r.UnitTimeoutMS = 600
			if kind == "near-deadline" {
				r.UnitTimeoutMS = 3000
			}
			if kind == "application" {
				r.UnitTimeoutMS = 10000
				r.Configuration.Grant.ExpiresUnixNano = time.Now().Add(2 * time.Second).UnixNano()
			}
			writeApplicationFixture(t, r, path)
			var e error
			r, e = readHostApplication(path)
			if e != nil {
				t.Fatal(e)
			}
			a, e := product.CreateApplication(r.Configuration, time.Now())
			if e != nil {
				t.Fatal(e)
			}
			st, e := os.Stat(filepath.Join(r.Configuration.SourceRoot, "unit0.go"))
			if e != nil {
				t.Fatal(e)
			}
			entry := applicationCatalogEntry{ApplicationSHA256: product.ApplicationConfigDigest(r.Configuration), Path: "unit0.go", Identity: applicationFileIdentity(st)}
			d := pilot.Digest([]byte("expired actual CLI"))
			root := filepath.Join(r.Configuration.Store, "units", d[7:])
			if e = os.MkdirAll(root, 0700); e != nil {
				t.Fatal(e)
			}
			source, f, e := deriveApplicationSource(context.Background(), r, entry, root)
			if e != nil {
				t.Fatal(e)
			}
			u := product.UnitBinding{ApplicationSHA256: entry.ApplicationSHA256, UnitID: d, InputSHA256: pilot.Digest(f.SpecBytes()), RevisionSHA256: pilot.Digest([]byte(r.Configuration.RevisionAssertion)), SourceFreezeSHA256: pilot.Digest(f.ManifestBytes()), ScopeSHA256: source.Configuration.ScopeSHA256, ProducerConfigSHA256: r.Configuration.ProducerConfigSHA256, PolicySHA256: r.Configuration.UnitPolicySHA256, RootIdentitySHA256: entry.Identity}
			allowance, e := applicationChildAllowance(r, source)
			if e != nil {
				t.Fatal(e)
			}
			if e = a.AdmitUnitWithDerivation(u, allowance, source.Configuration, applicationDerivedCohort(r, source.Configuration, d, root), time.Now()); e != nil {
				t.Fatal(e)
			}
			deadline, e := product.ReadApplicationUnitDeadline(r.Configuration.Store, u.ApplicationSHA256, u.UnitID)
			if e != nil {
				t.Fatal(e)
			}
			if kind == "invalid-contract" {
				u.InputSHA256 = pilot.Digest([]byte("unapproved"))
				entry.Path = "../untrusted"
				source.AuxiliaryRoot = filepath.Join(base, "untrusted")
			}
			id, e := a.PutRecord(struct {
				Unit   product.UnitBinding
				Entry  applicationCatalogEntry
				Source hostProductRequestV3
			}{u, entry, source})
			if e != nil {
				t.Fatal(e)
			}
			a.Close()
			if kind == "unit" || kind == "application" {
				time.Sleep(time.Until(deadline) + 30*time.Millisecond)
			}
			if kind == "near-deadline" {
				time.Sleep(time.Until(deadline) - 1200*time.Millisecond)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, tool, "private-host-application", "acquire", path, id, root)
			cmd.Env = append(os.Environ(), "LSP_HOST_APPLICATION_BLOCK_DOC=1", "LSP_HOST_APPLICATION_OWNED_SERVER=1")
			started := time.Now()
			cap := time.Until(deadline) + 350*time.Millisecond
			if applicationFixtureRace {
				cap += time.Second
			}
			_, runErr := cmd.CombinedOutput()
			if kind == "near-deadline" {
				if _, e := os.Stat(filepath.Join(root, "application-original-claim.json")); e != nil {
					t.Fatal("near deadline did not reach original claim", e)
				}
				if _, e := os.Stat(filepath.Join(root, "auxiliary", "ledger")); e != nil {
					t.Fatal("near deadline did not reach controlled provider boundary", e)
				}
				if runErr == nil || time.Since(started) < 200*time.Millisecond || time.Since(started) > cap {
					t.Fatal("ASSERT_BOUNDARY_ACQUIRE_REMAINING_DEADLINE_CAP", runErr, time.Since(started))
				}
				return
			}
			if runErr == nil {
				t.Fatal("ASSERT_BOUNDARY_EXPIRED_OR_INVALID_ACQUIRE_REFUSED")
			}
			if _, e = os.Stat(filepath.Join(root, "application-original-claim.json")); !os.IsNotExist(e) {
				t.Fatal("ASSERT_BOUNDARY_EXPIRED_ACQUIRE_NO_ORIGINAL_CLAIM")
			}
			ledgers, _ := filepath.Glob(filepath.Join(root, "auxiliary", "ledger"))
			if len(ledgers) != 0 {
				t.Fatal("ASSERT_BOUNDARY_EXPIRED_ACQUIRE_NO_LEDGER")
			}
			if _, e = os.Stat(filepath.Join(base, "untrusted")); !os.IsNotExist(e) {
				t.Fatal("ASSERT_BOUNDARY_UNTRUSTED_SOURCE_PATH_NOT_USED")
			}
		})
	}
}
