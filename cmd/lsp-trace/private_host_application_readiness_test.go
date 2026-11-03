//go:build darwin

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	pilot "lsp-trace/internal/censuslineagepilot"
	product "lsp-trace/internal/privatehostproductv3"
)

// Genuine source entries, not padded journal records or a synthetic denominator.
func readinessLiteralFixture(t *testing.T) (hostApplicationRequest, string, string) {
	t.Helper()
	r, path, base := compatibilityOneFile(t)
	selected := "chosen/deeper/leaf/twenty.go"
	if e := os.MkdirAll(filepath.Join(r.Configuration.SourceRoot, filepath.Dir(selected)), 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.Rename(filepath.Join(r.Configuration.SourceRoot, "twenty.go"), filepath.Join(r.Configuration.SourceRoot, selected)); e != nil {
		t.Fatal(e)
	}
	for branch := 0; branch < 24; branch++ {
		dir := filepath.Join(r.Configuration.SourceRoot, fmt.Sprintf("unrelated-%02d", branch))
		if e := os.Mkdir(dir, 0700); e != nil {
			t.Fatal(e)
		}
		for file := 0; file < 96; file++ {
			if e := os.WriteFile(filepath.Join(dir, fmt.Sprintf("source-%03d.go", file)), []byte("package unrelated\n"), 0600); e != nil {
				t.Fatal(e)
			}
		}
	}
	for n, arg := range r.Template.CensusArguments {
		if arg == "--include" {
			r.Template.CensusArguments[n+1] = selected
		}
	}
	r.MaxUnitsPerRun = 1
	return r, path, base
}

func TestHostApplicationReadinessLiteralDiscovery(t *testing.T) {
	r, path, _ := readinessLiteralFixture(t)
	// Coordinator10s + discovery strictly below49s leaves source1s + unit180s.
	r.Configuration.Grant.Ceiling.ActiveNS = uint64(59 * time.Second)
	writeApplicationFixture(t, r, path)
	var out, diag bytes.Buffer
	code := runPrivateHostApplication([]string{"discover", path}, bytes.NewReader(nil), &out, &diag)
	var report applicationTerminal
	if code != 0 || json.Unmarshal(out.Bytes(), &report) != nil || report.Status != "DISCOVERY_COMPLETE" || report.Selected != 1 || report.Denominator != "1" || report.Discovered != 1 || report.Reserved.ActiveNS >= uint64(59*time.Second) {
		t.Fatalf("ASSERT_LITERAL_DISCOVERY_FINITE A-fail code=%d report=%+v diag=%s", code, report, diag.String())
	}
	if report.SourceCompleteness != "UNKNOWN" || report.AbsenceInference != "PROHIBITED" || report.Authority != 0 || report.Accepted {
		t.Fatal("selection is not global source authority")
	}
	io := report.LiteralDiscovery
	if io.DirectoryReservations != 4 || io.CandidatePages != 4 || io.CandidateNames != 4 || io.DirectoryOpens != 0 || io.DirectoryReads != 0 || io.RootOpens != 1 || io.MaxPageNames != 1 || io.SourceStats > 46 || report.OwnedIO.MaxReadBuffer > 64<<10 || report.OwnedIO.PriorPageReads != 0 || report.OwnedIO.RecordLookups > 32 || report.SelectionDomain != "APPROVED_SELECTION_DOMAIN_ONLY" || !strings.Contains(report.UnvisitedExtent, "UNKNOWN") {
		t.Fatalf("ASSERT_LITERAL_DISCOVERY_BOUNDED_IO A-fail literal=%+v io=%+v", io, report.OwnedIO)
	}
	t.Logf("ASSERT_LITERAL_DISCOVERY_FINITE A-pass reserved=%+v io=%+v literal=%+v", report.Reserved, report.OwnedIO, io)
}

// Inject via JSON so this assertion compiles against the old typed envelope.
func readinessUnitTime(t *testing.T, r hostApplicationRequest, ms int) hostApplicationRequest {
	t.Helper()
	r = compatibilityEnvelope(t, r, 4)
	raw, _ := json.Marshal(r.UnitEnvelope)
	var fields map[string]json.RawMessage
	if e := json.Unmarshal(raw, &fields); e != nil {
		t.Fatal(e)
	}
	fields["UnitTimeoutMS"] = json.RawMessage(fmt.Sprint(ms))
	raw, _ = json.Marshal(fields)
	if e := json.Unmarshal(raw, r.UnitEnvelope); e != nil {
		t.Fatal(e)
	}
	r.UnitTimeoutMS = ms
	return r
}

func TestHostApplicationReadinessExplicitUnit180Loader(t *testing.T) {
	r, path, _ := hostApplicationCLIFixture(t)
	r = readinessUnitTime(t, r, 180000)
	writeApplicationFixture(t, r, path)
	admitted, e := readHostApplication(path)
	if e != nil {
		t.Fatalf("ASSERT_EXPLICIT_UNIT180_LOADER A-fail %v", e)
	}
	cost, e := applicationChildAllowance(admitted, admitted.Template)
	if e != nil || cost.ActiveNS != uint64(180*time.Second) || admitted.Template.ExchangeTimeoutMS > 60000 || admitted.Template.OperationTimeoutMS > 240000 {
		t.Fatalf("ASSERT_EXPLICIT_UNIT180_LOADER A-fail cost=%+v err=%v", cost, e)
	}
	if _, e = os.Stat(r.Configuration.Store); !os.IsNotExist(e) {
		t.Fatal("loader must not create parent ledger")
	}
	t.Logf("ASSERT_EXPLICIT_UNIT180_LOADER A-pass active=%d", cost.ActiveNS)
}

func readinessDiscover(t *testing.T, r hostApplicationRequest, requestPath string) (hostApplicationRequest, applicationTerminal) {
	t.Helper()
	writeApplicationFixture(t, r, requestPath)
	r, e := readHostApplication(requestPath)
	if e != nil {
		t.Fatal(e)
	}
	var out, diag bytes.Buffer
	code := runPrivateHostApplication([]string{"discover", requestPath}, bytes.NewReader(nil), &out, &diag)
	var report applicationTerminal
	if code != 0 || json.Unmarshal(out.Bytes(), &report) != nil || report.Status != "DISCOVERY_COMPLETE" {
		t.Fatalf("discover code=%d %+v %s", code, report, diag.String())
	}
	return r, report
}

func TestHostApplicationReadinessNativeSelection(t *testing.T) {
	cases := []struct {
		name                      string
		includes, excludes, prune []string
		literal                   bool
	}{
		{"qualified", []string{"a/b/one.go"}, nil, nil, true},
		{"shared-multiple", []string{"a/b/one.go", "a/b/two.go", "elsewhere/c/one.go", "a/b/one.go"}, nil, nil, true},
		{"basename-any-depth", []string{"one.go"}, nil, nil, false},
		{"mixed-basename", []string{"one.go", "a/b/two.go"}, nil, nil, false},
		{"glob", []string{"**/one.go"}, nil, nil, false},
		{"bracket", []string{"a/[bc]/one.go"}, nil, nil, false},
		{"escaped-separators", []string{"a\\b\\one.go"}, nil, nil, false},
		{"alias-fallback", []string{"a/./b/one.go"}, nil, nil, false},
		{"excluded", []string{"a/b/one.go", "a/b/two.go"}, []string{"**/two.go"}, nil, true},
		{"pruned", []string{"a/b/one.go", "elsewhere/c/one.go"}, nil, []string{"elsewhere"}, true},
		{"adversarial", []string{"a/b/[one].go"}, nil, nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, p, _ := hostApplicationCLIFixture(t)
			for _, name := range []string{"a/b/one.go", "a/b/two.go", "a/c/one.go", "elsewhere/c/one.go", "elsewhere/c/not-one.go", "a/b/[one].go"} {
				f := filepath.Join(r.Configuration.SourceRoot, name)
				if e := os.MkdirAll(filepath.Dir(f), 0700); e != nil {
					t.Fatal(e)
				}
				if e := os.WriteFile(f, []byte("package owned\n"), 0600); e != nil {
					t.Fatal(e)
				}
			}
			args := []string{}
			for n := 0; n < len(r.Template.CensusArguments); n++ {
				if r.Template.CensusArguments[n] == "--include" {
					n++
					continue
				}
				args = append(args, r.Template.CensusArguments[n])
			}
			for _, pattern := range tc.includes {
				args = append(args, "--include", pattern)
			}
			for _, pattern := range tc.excludes {
				args = append(args, "--exclude", pattern)
			}
			for _, pattern := range tc.prune {
				args = append(args, "--prune-dir", pattern)
			}
			r.Template.CensusArguments = args
			o, e := parseCensusCLIOptions(args)
			if e != nil {
				t.Fatal(e)
			}
			native, e := enumerateCensusPreflightContextPrivate(context.Background(), o)
			if e != nil {
				t.Fatal(e)
			}
			expected := []string{}
			for _, f := range native {
				if matchesAnyDiscoveryPattern(o.Includes, f.Path) && !matchesAnyDiscoveryPattern(o.Excludes, f.Path) {
					expected = append(expected, f.Path)
				}
			}
			r, report := readinessDiscover(t, r, p)
			a, e := product.OpenApplication(r.Configuration, time.Now())
			if e != nil {
				t.Fatal(e)
			}
			defer a.Close()
			var s applicationState
			if e = a.GetRecord(a.Head().CursorSHA256, &s); e != nil {
				t.Fatal(e)
			}
			actual := []string{}
			for id := s.Selected; id != ""; {
				var entry applicationCatalogEntry
				if e = a.GetRecord(id, &entry); e != nil {
					t.Fatal(e)
				}
				actual = append(actual, entry.Path)
				id = entry.Next
			}
			sort.Strings(actual)
			sort.Strings(expected)
			if !reflect.DeepEqual(actual, expected) || report.Selected != uint64(len(expected)) || (report.LiteralDiscovery.RootIdentity != "") != tc.literal {
				t.Fatalf("ASSERT_LITERAL_NATIVE_EQUIVALENCE A-fail expected=%v actual=%v report=%+v", expected, actual, report)
			}
			if report.SourceCompleteness != "UNKNOWN" || report.AbsenceInference != "PROHIBITED" {
				t.Fatal("false global completeness")
			}
			t.Logf("ASSERT_LITERAL_NATIVE_EQUIVALENCE A-pass paths=%v literal=%t", actual, tc.literal)
		})
	}
}

func TestHostApplicationReadinessTimePolicyBounds(t *testing.T) {
	r, path, _ := hostApplicationCLIFixture(t)
	oldDigest := hostApplicationUnitPolicyDigest(r)
	for _, tc := range []struct {
		name               string
		ms, optin          int
		nilPolicy, allowed bool
	}{
		{"legacy60", 60000, 0, true, true}, {"legacy61", 60001, 0, true, false},
		{"envelope-not-optin", 180000, 0, false, false}, {"explicit60", 60000, 60000, false, true},
		{"explicit180", 180000, 180000, false, true}, {"explicit-over", 180001, 180001, false, false},
		{"mismatch", 180000, 179999, false, false}, {"negative", -1, -1, false, false},
		{"zero", 0, 0, false, false}, {"overflow", int(^uint(0) >> 1), int(^uint(0) >> 1), false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			probe := r
			if !tc.nilPolicy {
				probe = readinessUnitTime(t, probe, tc.optin)
			}
			probe.UnitTimeoutMS = tc.ms
			writeApplicationFixture(t, probe, path)
			admitted, e := readHostApplication(path)
			if (e == nil) != tc.allowed {
				t.Fatalf("ASSERT_UNIT_TIME_POLICY_BOUND A-fail allowed=%t err=%v", tc.allowed, e)
			}
			if e == nil {
				b, e := applicationChildAllowance(admitted, admitted.Template)
				if e != nil || b.ActiveNS != uint64(tc.ms)*1000000 {
					t.Fatal(b, e)
				}
			}
			t.Log("ASSERT_UNIT_TIME_POLICY_BOUND A-pass")
		})
	}
	changed := readinessUnitTime(t, r, 180000)
	if hostApplicationUnitPolicyDigest(changed) == oldDigest {
		t.Fatal("new time policy reused legacy digest")
	}
	// A changed policy cannot retain any old config/request approval.
	raw, _ := json.Marshal(changed)
	if e := os.WriteFile(path, raw, 0600); e != nil {
		t.Fatal(e)
	}
	if _, e := readHostApplication(path); e == nil {
		t.Fatal("ASSERT_UNIT_TIME_OLD_HASH_REFUSED")
	}
	if _, e := os.Stat(r.Configuration.Store); !os.IsNotExist(e) {
		t.Fatal("loader spent source/provider work")
	}
}

func readinessAdmission(t *testing.T, r hostApplicationRequest, a *product.ApplicationJournal, name string) (product.UnitBinding, hostProductRequestV3, product.ApplicationBounds, error) {
	t.Helper()
	selected := "unit0.go"
	if _, e := os.Stat(filepath.Join(r.Configuration.SourceRoot, selected)); os.IsNotExist(e) {
		selected = "chosen/deeper/leaf/twenty.go"
	}
	st, e := os.Stat(filepath.Join(r.Configuration.SourceRoot, selected))
	if e != nil {
		t.Fatal(e)
	}
	entry := applicationCatalogEntry{ApplicationSHA256: product.ApplicationConfigDigest(r.Configuration), Path: selected, Identity: applicationFileIdentity(st)}
	id := pilot.Digest([]byte(name))
	root := filepath.Join(r.Configuration.Store, "units", id[7:])
	if e = os.MkdirAll(root, 0700); e != nil {
		t.Fatal(e)
	}
	source, f, e := deriveApplicationSource(context.Background(), r, entry, root)
	if e != nil {
		t.Fatal(e)
	}
	u := product.UnitBinding{ApplicationSHA256: entry.ApplicationSHA256, UnitID: id, InputSHA256: pilot.Digest(f.SpecBytes()), RevisionSHA256: pilot.Digest([]byte(r.Configuration.RevisionAssertion)), SourceFreezeSHA256: pilot.Digest(f.ManifestBytes()), ScopeSHA256: source.Configuration.ScopeSHA256, ProducerConfigSHA256: r.Configuration.ProducerConfigSHA256, PolicySHA256: r.Configuration.UnitPolicySHA256, RootIdentitySHA256: entry.Identity}
	b, e := applicationChildAllowance(r, source)
	if e != nil {
		t.Fatal(e)
	}
	return u, source, b, a.AdmitUnitWithDerivation(u, b, source.Configuration, applicationDerivedCohort(r, source.Configuration, id, root), time.Now())
}

func TestHostApplicationReadinessParentPrepayment(t *testing.T) {
	for _, mode := range []string{"full180", "insufficient", "dual", "expiry", "amendment"} {
		t.Run(mode, func(t *testing.T) {
			r, p, _ := hostApplicationCLIFixture(t)
			r = readinessUnitTime(t, r, 180000)
			if mode == "expiry" {
				r.Configuration.Grant.ExpiresUnixNano = time.Now().Add(90 * time.Second).UnixNano()
			}
			writeApplicationFixture(t, r, p)
			r, e := readHostApplication(p)
			if e != nil {
				t.Fatal(e)
			}
			a, e := product.CreateApplication(r.Configuration, time.Now())
			if e != nil {
				t.Fatal(e)
			}
			defer a.Close()
			if mode == "insufficient" {
				if _, e = a.ReserveOperation(pilot.Digest([]byte(mode)), "controlled_prior_time", product.ApplicationBounds{ActiveNS: uint64(61 * time.Second)}, time.Now()); e != nil {
					t.Fatal(e)
				}
			}
			before := a.Head().Reserved
			u, source, b, e := readinessAdmission(t, r, a, "prepaid-first")
			admittedBy := time.Now() // native freezing precedes admission inside helper
			if mode == "insufficient" {
				if !errors.Is(e, product.ErrPaused) {
					t.Fatal("ASSERT_UNIT180_INSUFFICIENT_BEFORE_PROVIDER", e)
				}
				if _, e = product.ReadApplicationUnitDeadline(r.Configuration.Store, u.ApplicationSHA256, u.UnitID); e == nil {
					t.Fatal("refused unit has deadline")
				}
				ledgers, _ := filepath.Glob(filepath.Join(r.Configuration.Store, "units", "*", "original"))
				if len(ledgers) != 0 || a.Head().Reserved.ActiveNS != before.ActiveNS {
					t.Fatal("refused unit spent child work")
				}
				return
			}
			if e != nil {
				t.Fatal(e)
			}
			if a.Head().Reserved.ActiveNS-before.ActiveNS != uint64(180*time.Second) || product.VerifyApplicationUnitPrepayment(r.Configuration.Store, u, b, source.Configuration) != nil {
				t.Fatal("ASSERT_UNIT180_FULL_PARENT_PREPAY")
			}
			deadline, e := product.ReadApplicationUnitDeadline(r.Configuration.Store, u.ApplicationSHA256, u.UnitID)
			if e != nil {
				t.Fatal(e)
			}
			if deadline.After(admittedBy.Add(180*time.Second)) || deadline.After(time.Unix(0, r.Configuration.Grant.ExpiresUnixNano)) {
				t.Fatal("ASSERT_UNIT180_ORIGINAL_SHARED_WINDOW")
			}
			ctx, cancel := context.WithDeadline(context.Background(), deadline)
			defer cancel()
			got, ok := ctx.Deadline()
			if !ok || !got.Equal(deadline) {
				t.Fatal("actual context deadline")
			}
			time.Sleep(20 * time.Millisecond)
			again, e := product.ReadApplicationUnitDeadline(r.Configuration.Store, u.ApplicationSHA256, u.UnitID)
			if e != nil || again.After(deadline.Add(5*time.Millisecond)) {
				t.Fatal("ASSERT_UNIT180_NO_PHASE_RESET", e)
			}
			ledgers, _ := filepath.Glob(filepath.Join(r.Configuration.Store, "units", "*", "original"))
			if len(ledgers) != 0 {
				t.Fatal("prepayment created original provider ledger")
			}
			if mode == "dual" {
				_, _, _, e = readinessAdmission(t, r, a, "prepaid-second")
				if !errors.Is(e, product.ErrPaused) {
					t.Fatal("ASSERT_UNIT180_DUAL_NO_REPLENISH", e)
				}
			}
			if mode == "amendment" {
				a.Close()
				changed := r
				changed.UnitTimeoutMS = 60000
				changed.UnitEnvelope.UnitTimeoutMS = 60000
				writeApplicationFixture(t, changed, p)
				changed, e = readHostApplication(p)
				if e != nil {
					t.Fatal(e)
				}
				if other, e := product.OpenApplication(changed.Configuration, time.Now()); e == nil {
					other.Close()
					t.Fatal("ASSERT_UNIT180_HEADER_AMENDMENT_REFUSED")
				}
			}
			t.Logf("ASSERT_UNIT180_PARENT_WINDOW A-pass mode=%s active=%d remaining=%s", mode, b.ActiveNS, time.Until(deadline))
		})
	}
}
