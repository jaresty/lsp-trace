//go:build darwin

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	ai "lsp-trace/internal/applicationinventory"
	pilot "lsp-trace/internal/censuslineagepilot"
	"lsp-trace/internal/publication"
	"lsp-trace/sessionruntime"
)

func censusF11ProtectedHome(t *testing.T) string {
	t.Helper()
	home, e := os.UserHomeDir()
	if e != nil {
		t.Fatal(e)
	}
	home, e = filepath.EvalSymlinks(home)
	if e != nil {
		t.Fatal(e)
	}
	base, e := os.MkdirTemp(home, "census-owner-contract-")
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		if e := os.RemoveAll(base); e != nil {
			t.Error(e)
		}
	})
	if e = pilot.ValidateDurableRoot(base); e != nil {
		t.Fatal("test-owned HOME not admitted", e)
	}
	return base
}
func censusF11Source(t *testing.T, n int) (string, string) {
	t.Helper()
	base := censusF11ProtectedHome(t)
	source, parent := filepath.Join(base, "source"), filepath.Join(base, "publication")
	for _, p := range []string{source, parent} {
		if e := os.Mkdir(p, 0700); e != nil {
			t.Fatal(e)
		}
	}
	if e := os.WriteFile(filepath.Join(source, "go.mod"), []byte("module example.test/owner\n\ngo 1.26\n"), 0600); e != nil {
		t.Fatal(e)
	}
	for i := 0; i < n; i++ {
		if e := os.WriteFile(filepath.Join(source, fmt.Sprintf("member%02d.go", i)), []byte(fmt.Sprintf("package owner\nfunc Member%02d() {}\n", i)), 0600); e != nil {
			t.Fatal(e)
		}
	}
	return source, parent
}
func censusF11Decode(t *testing.T, raw []byte, value any) {
	t.Helper()
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if e := d.Decode(value); e != nil {
		t.Fatalf("strict JSON: %v: %s", e, raw)
	}
	if e := d.Decode(new(any)); e != io.EOF {
		t.Fatalf("one JSON value required: %v", e)
	}
}
func assertCensusOwnerStartNoAcquisition(t *testing.T, workspace, parent string, members uint64) {
	t.Helper()
	t.Setenv(censusCallerFDEnv, "")
	t.Setenv(censusDurableCallerEnv, "")
	t.Setenv(censusDurableStoreEnv, "")
	deps := productionCensusRunnerDependencies()
	native := 0
	deps.lookPath = func(string) (string, error) { native++; return "", errors.New("owner start must not resolve provider") }
	deps.newStarter = func() (sessionruntime.Starter, error) {
		native++
		return nil, errors.New("owner start must not construct provider")
	}
	deps.runNativeCatalog = func(censusCLIOptions, censusCoreConfig, io.Writer, io.Writer) int { native++; return 1 }
	var out, diag bytes.Buffer
	if code := runCensusWithDependencies([]string{workspace, "--publication-root", parent, "--machine"}, &out, &diag, deps); code != 0 || native != 0 {
		t.Fatalf("ASSERT_OWNER_START_NO_ACQUISITION: code=%d native=%d out=%s diag=%s", code, native, out.String(), diag.String())
	}
	var v censusOwnerView
	censusF11Decode(t, out.Bytes(), &v)
	var start struct {
		Schema, RunID, Store string
		StatusOperation      []string
	}
	censusF11Decode(t, diag.Bytes(), &start)
	store := filepath.Join(parent, publication.StorageDirectoryName)
	status := []string{"lsp-trace", "census", "durable", "status", v.RunID, "--store", store}
	want := map[string][]string{"status": status}
	capability := "UNSUPPORTED_SCOPE_OR_PROFILE"
	if members == 17 {
		capability = "BOUNDED_ACQUISITION_READY"
		want["acquire"] = []string{"lsp-trace", "census", "durable", "acquire", v.RunID, v.Head, "--store", store}
	}
	if v.Schema != "lsp-trace.census-owner.v1" || !validApplicationDigest("sha256:"+v.RunID) || !validApplicationDigest(v.Head) || v.Store != store || v.Phase != "ReadyForAcquisition" || v.Status != "READY" || v.Capability != capability || v.JobExpiry != "NONE" || v.TaskID != "" || v.Admissions != 0 || v.Enumeration == nil || !v.Enumeration.Closed || !v.Enumeration.DenominatorComplete || v.Enumeration.UnknownRemainder || v.Enumeration.SourceContentRead || v.Enumeration.AtomicSnapshot || v.Enumeration.InventoryPublished || v.Enumeration.AcquisitionImplemented || v.Enumeration.CapabilityGap != "CLIENT_DECLARED_ELIGIBILITY_NOT_SERVER_CONFIRMED; ACQUISITION_UNAVAILABLE; metadata is not source identity; future acquisition requires paid revalidation" || v.Enumeration.Counts.Selected != members || !reflect.DeepEqual(v.Operations, want) {
		t.Fatalf("ASSERT_OWNER_START_METADATA_NOT_COMPLETION: %s", out.Bytes())
	}
	if v.StoragePolicy != "OWNED_STORE_CONTENT_LENGTH_V1" || v.Storage == nil || !strings.Contains(v.Qualification, "completeness UNKNOWN") {
		t.Fatalf("ASSERT_OWNER_START_QUALIFICATION: %+v", v)
	}
	if start.Schema != "lsp-trace.census-owner-start.v1" || start.RunID != v.RunID || start.Store != store || !reflect.DeepEqual(start.StatusOperation, status) {
		t.Fatalf("ASSERT_OWNER_START_DIAGNOSTIC: %+v", start)
	}
	var fields map[string]json.RawMessage
	if e := json.Unmarshal(out.Bytes(), &fields); e != nil {
		t.Fatal(e)
	}
	for _, k := range []string{"Published", "InventoryHead", "Accepted", "Report"} {
		if _, ok := fields[k]; ok {
			t.Fatalf("ASSERT_OWNER_START_NO_PUBLICATION: %s", k)
		}
	}
	t.Logf("OWNER_START_VIEW: %s", out.Bytes())
	t.Logf("OWNER_START_DIAGNOSTIC: %s", diag.Bytes())
	out.Reset()
	diag.Reset()
	code := runCensusOwnerOperationWithFactory(status[3:], nil, &out, &diag, func(ai.InventoryNativeBatchV1, time.Time) (sessionruntime.PrivateDiscoveryConstruction, error) {
		native++
		return sessionruntime.PrivateDiscoveryConstruction{}, errors.New("status cannot acquire")
	})
	var observed censusOwnerView
	censusF11Decode(t, out.Bytes(), &observed)
	if code != 0 || diag.Len() != 0 || native != 0 || !reflect.DeepEqual(observed, v) {
		t.Fatalf("ASSERT_OWNER_RETURNED_STATUS: code=%d native=%d status=%+v diag=%s", code, native, observed, diag.String())
	}
	t.Log("ASSERT_OWNER_START_NO_ACQUISITION: PASS metadata/operations only; no published semantic completion")
}
func TestCensusOrdinaryOwnerStartNoAcquisition(t *testing.T) {
	for _, n := range []int{2, 17} {
		t.Run(strconv.Itoa(n), func(t *testing.T) {
			home := censusF11ProtectedHome(t)
			t.Setenv("HOME", home)
			source, parent := censusF11Source(t, n)
			assertCensusOwnerStartNoAcquisition(t, source, parent, uint64(n))
		})
	}
}

func TestCensusDeclaredCallerRefusalNoAcquisition(t *testing.T) {
	home := censusF11ProtectedHome(t)
	t.Setenv("HOME", home)
	t.Setenv(censusDurableCallerEnv, "")
	workspace, parent := censusF11Source(t, 2)
	f, e := os.CreateTemp(home, "not-a-socket-")
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	t.Setenv(censusCallerFDEnv, strconv.Itoa(int(f.Fd())))
	deps := productionCensusRunnerDependencies()
	calls := 0
	deps.runOwnedInventory = func(censusCLIOptions, io.Writer, io.Writer) int {
		t.Fatal("declared caller silently selected owner default")
		return 1
	}
	deps.lookPath = func(string) (string, error) { return "/controlled/not-started", nil }
	deps.newStarter = func() (sessionruntime.Starter, error) {
		t.Fatal("caller refusal acquired provider")
		return nil, errors.New("unreachable")
	}
	deps.runNativeCatalog = func(o censusCLIOptions, _ censusCoreConfig, _ io.Writer, errout io.Writer) int {
		calls++
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		conn, _, err := openCensusCaller(ctx)
		if conn != nil || err == nil || !strings.Contains(err.Error(), "CALLER_AGENT_UNAVAILABLE") {
			t.Fatalf("ASSERT_DECLARED_NONSOCKET_REFUSED: %v %v", conn, err)
		}
		return writeCensusFailure(errout, censusStageAssembly, o.Machine, err)
	}
	var out, diag bytes.Buffer
	code := runCensusWithDependencies([]string{workspace, "--publication-root", parent}, &out, &diag, deps)
	var d censusCLIDiagnostic
	censusF11Decode(t, diag.Bytes(), &d)
	if code != 1 || calls != 1 || out.Len() != 0 || d.SchemaVersion != "lsp-trace.census-diagnostic.v1" || d.Status != "FAILED" || d.Stage != censusStageAssembly || d.Code != censusCodeAssemblyFailed {
		t.Fatalf("ASSERT_DECLARED_CALLER_ROUTE: code=%d calls=%d diagnostic=%+v", code, calls, d)
	}
	if _, e := f.Stat(); e != nil {
		t.Fatal("refusal transferred caller descriptor ownership", e)
	}
	t.Setenv(censusCallerFDEnv, "")
	conn, _, err := openCensusCaller(context.Background())
	if conn != nil || err == nil || !strings.Contains(err.Error(), "CALLER_AGENT_UNAVAILABLE") {
		t.Fatal("low-level missing caller refusal lost", err)
	}
}

func censusF11Tree(t *testing.T, root string) string {
	t.Helper()
	var rows []string
	e := filepath.WalkDir(root, func(p string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		i, e := d.Info()
		if e != nil {
			return e
		}
		var data []byte
		if d.Type().IsRegular() {
			data, e = os.ReadFile(p)
			if e != nil {
				return e
			}
		}
		rows = append(rows, fmt.Sprintf("%s %s %d %x", p, i.Mode(), i.ModTime().UnixNano(), sha256.Sum256(data)))
		return nil
	})
	if e != nil {
		t.Fatal(e)
	}
	return strings.Join(rows, "\n")
}
func TestCensusOwnerEmptyStoreBeforeEffects(t *testing.T) {
	home := censusF11ProtectedHome(t)
	t.Setenv("HOME", home)
	for _, selection := range []string{"", filepath.Join(home, "environment-store")} {
		t.Setenv(censusDurableStoreEnv, selection)
		for _, op := range []string{"status", "acquire", "seal"} {
			args := []string{op, strings.Repeat("a", 64)}
			if op != "status" {
				args = append(args, strings.Repeat("b", 64))
			}
			args = append(args, "--store", "")
			before := censusF11Tree(t, home)
			constructors := 0
			var out, diag bytes.Buffer
			code := runCensusOwnerOperationWithFactory(args, nil, &out, &diag, func(ai.InventoryNativeBatchV1, time.Time) (sessionruntime.PrivateDiscoveryConstruction, error) {
				constructors++
				return sessionruntime.PrivateDiscoveryConstruction{}, errors.New("must not construct")
			})
			var d struct{ Schema, Status, Reason string }
			censusF11Decode(t, diag.Bytes(), &d)
			if code != 1 || constructors != 0 || out.Len() != 0 || before != censusF11Tree(t, home) || d.Schema != "lsp-trace.census-owner-refusal.v1" || d.Status != "REFUSED_OR_UNCERTAIN" || d.Reason == "" {
				t.Fatalf("ASSERT_EMPTY_STORE_PRE_EFFECT: %s %d %d %+v", op, code, constructors, d)
			}
		}
	}
}
