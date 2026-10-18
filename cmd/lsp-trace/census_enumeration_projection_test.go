package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"lsp-trace/acquisitionops"
	"lsp-trace/internal/captureset"
	"lsp-trace/internal/census"
	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/censusenumerationprojection"
	"lsp-trace/internal/censusrequest"
	"lsp-trace/internal/censusresult"
	"lsp-trace/internal/publication"
)

func TestPrivateEnumerationProjectionFlagFreshMachineOnlyHiddenAndDuplicate(t *testing.T) {
	base := []string{"census", "--machine", "--private-enumeration-projection", "--workspace", "work", "--source", ".", "--publication-root", "/private", "--server", "gopls"}
	o, err := parseCensusCLIOptions(base)
	if err != nil || !o.PrivateEnumerationProjection {
		t.Fatalf("%+v %v", o, err)
	}
	bad := [][]string{
		{"census", "--private-enumeration-projection", "--workspace", "work", "--source", ".", "--publication-root", "/private", "--server", "gopls"},
		append(append([]string(nil), base...), "--private-enumeration-projection"),
		{"census", "--machine", "--private-enumeration-projection", "--catalog", "--catalog-config", "x", "--workspace", "work", "--source", ".", "--publication-root", "/private", "--server", "gopls"},
		{"census", "--machine", "--private-enumeration-projection", "--resume", "sha256:" + strings.Repeat("a", 64), "--catalog-config", "x", "--publication-root", "/private"},
	}
	for i, args := range bad {
		if _, err := parseCensusCLIOptions(args); err == nil {
			t.Fatalf("bad %d accepted", i)
		}
	}
	if strings.Contains(censusUsage, "private-enumeration-projection") || strings.Contains(usageText, "private-enumeration-projection") {
		t.Fatal("hidden flag leaked")
	}
}
func TestPrivateEnumerationReceiptSeparateAfterByteIdenticalDiscoveryLine(t *testing.T) {
	d, _ := censusresult.NewDiscoveryDiagnostic([]censusresult.DiscoveryObservation{{Dimension: censusresult.DimensionProviderCompleteness, State: censusresult.ObservationKnown, Observed: ptrU64(1)}})
	r, _ := censusrequest.New(censusrequest.RefreshCoordinate{SessionID: "s", Generation: 1}, censusrequest.SemanticFields{Sources: []string{"."}, DownDepth: 1, MaxNodes: 10000, BatchTargets: 63, TimeoutMS: 60000, RequestTimeoutMS: 30000})
	base := censusPublicationOutcome{DiscoveryDiagnostic: &d, RequestReceipt: &r}
	var aout, aerr, bout, berr bytes.Buffer
	if writeCensusOutcome(&aout, &aerr, true, base) != 1 {
		t.Fatal("baseline code")
	}
	private := censusenumerationprojection.Unavailable("PUBLICATION_FAILED")
	base.EnumerationReceipt = &private
	if writeCensusOutcome(&bout, &berr, true, base) != 1 {
		t.Fatal("enabled code")
	}
	lines := bytes.Split(bytes.TrimSuffix(berr.Bytes(), []byte{'\n'}), []byte{'\n'})
	if len(lines) != 2 || !bytes.Equal(lines[0], bytes.TrimSuffix(aerr.Bytes(), []byte{'\n'})) {
		t.Fatalf("baseline=%q enabled=%q", aerr.Bytes(), berr.Bytes())
	}
	var receipt censusenumerationprojection.Receipt
	if json.Unmarshal(lines[1], &receipt) != nil || receipt.Status != "UNAVAILABLE" {
		t.Fatalf("%q", lines[1])
	}
	if aout.Len() != 0 || bout.Len() != 0 {
		t.Fatal("stdout mutated")
	}
}
func ptrU64(v uint64) *uint64 { return &v }
func incompleteEnumerationDiscovery(t *testing.T) censusacquisition.Discovery {
	t.Helper()
	return censusacquisition.Discovery{Session: censusacquisition.SessionIdentity{SessionID: "s", Generation: 1}, Workspace: t.TempDir(), Complete: false, Accounting: census.Accounting{FileDenominator: 1, Files: []census.FileEntry{{Ordinal: 0, Disposition: census.FileIncomplete}}, SymbolDenominator: 1, Symbols: []census.SymbolEntry{{Ordinal: 0, Disposition: census.SymbolIncomplete}}}, FileLedger: captureset.Ledger{Denominator: 1, Entries: []captureset.LedgerEntry{{Ordinal: 0, Identity: "a.go", Disposition: "incomplete"}}}, SymbolLedger: captureset.Ledger{Denominator: 1, Entries: []captureset.LedgerEntry{{Ordinal: 0, Identity: "private", Disposition: "incomplete"}}}, EnumerationSymbols: []censusacquisition.EnumerationSymbol{{Ordinal: 0, FileOrdinal: 0, Kind: 12, Name: "secret", Disposition: census.SymbolIncomplete}}, ProviderFailures: []censusacquisition.ProviderFailure{{Stage: "PREPARE_CALL_HIERARCHY", Code: "RESULT_MISMATCH", SymbolOrdinal: func() *int { v := 0; return &v }()}}, IncompleteCauses: []censusacquisition.IncompleteCause{{Scope: "FILE", Stage: "DISCOVERY", Code: "CHILD_INCOMPLETE", FileOrdinal: func() *int { v := 0; return &v }()}, {Scope: "SYMBOL", Stage: "PREPARE_CALL_HIERARCHY", Code: "RESULT_MISMATCH", SymbolOrdinal: func() *int { v := 0; return &v }()}}, Observations: []censusacquisition.DiscoveryObservation{{Dimension: censusacquisition.DiscoveryProviderCompleteness, Known: true, Omitted: ptrU64(1)}}}
}

func TestIncompleteOptInPublishesOnceBeforeAcquisitionAndSuccessPublishesNone(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	incomplete := incompleteEnumerationDiscovery(t)
	acquired := 0
	deps := censusCoreDependencies{runSession: func(_ initializedAcquisitionRunnerConfig, cb func(context.Context, *initializedAcquisitionRuntime) int) int {
		return cb(context.Background(), &initializedAcquisitionRuntime{sessionID: "s", generation: 1})
	}, discover: func(context.Context, censusCLIOptions, *initializedAcquisitionRuntime, bool) (censusacquisition.Discovery, error) {
		return incomplete, nil
	}, acquire: func(context.Context, *initializedAcquisitionRuntime, censusacquisition.Discoverer, acquisitionops.Limits, int, int, int) (censusAssembly, error) {
		acquired++
		return censusAssembly{}, nil
	}}
	opts := censusCLIOptions{PrivateEnumerationProjection: true, Machine: true, PublicationRoot: root, Sources: []string{"."}, DownDepth: 1, MaxNodes: 10000, MaxBatchTargets: 63, Timeout: 60000000000, RequestTimeout: 30000000000}
	out := runCensusCore(opts, censusCoreConfig{}, deps)
	if out.EnumerationReceipt == nil || out.EnumerationReceipt.Status != "PUBLISHED" || acquired != 0 {
		t.Fatalf("out=%+v private=%+v acquired=%d", out, out.EnumerationReceipt, acquired)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 1 {
		t.Fatalf("entries=%v err=%v", entries, err)
	}
	projection := censusPublicationProjection(t, 1)
	complete := discoveryFromProjection(projection)
	deps.runSession = func(_ initializedAcquisitionRunnerConfig, cb func(context.Context, *initializedAcquisitionRuntime) int) int {
		return cb(context.Background(), &initializedAcquisitionRuntime{sessionID: complete.Session.SessionID, generation: complete.Session.Generation})
	}
	published := successfulCorePublication(t, censusPublicationCapabilityFor(t, projection))
	deps.discover = func(context.Context, censusCLIOptions, *initializedAcquisitionRuntime, bool) (censusacquisition.Discovery, error) {
		return complete, nil
	}
	deps.acquire = func(context.Context, *initializedAcquisitionRuntime, censusacquisition.Discoverer, acquisitionops.Limits, int, int, int) (censusAssembly, error) {
		c := censusPublicationCapabilityFor(t, projection)
		return censusAssembly{state: c.state, token: c.token}, nil
	}
	deps.capability = func(a censusAssembly) (censusPublicationCapability, error) { return a.publicationCapability() }
	deps.publish = func(context.Context, censusPublicationCapability, string) censusPublicationOutcome { return published }
	out = runCensusCore(opts, censusCoreConfig{}, deps)
	if out.Result == nil || out.EnumerationReceipt != nil {
		t.Fatalf("success out=%+v", out)
	}
}

func TestPrivateEnumerationCancellationGatesAndPostCommit(t *testing.T) {
	for _, tc := range []struct {
		name   string
		setup  func(context.CancelFunc, *censusCoreDependencies)
		reason string
		files  int
	}{
		{name: "discover-return", reason: "CANCELED_PRE_RECONCILIATION", setup: func(cancel context.CancelFunc, deps *censusCoreDependencies) {
			base := deps.discover
			deps.discover = func(ctx context.Context, o censusCLIOptions, r *initializedAcquisitionRuntime, b bool) (censusacquisition.Discovery, error) {
				d, err := base(ctx, o, r, b)
				cancel()
				return d, err
			}
		}},
		{name: "pre-publish", reason: "CANCELED_PRE_PUBLICATION", setup: func(cancel context.CancelFunc, deps *censusCoreDependencies) {
			deps.publishEnumeration = func(ctx context.Context, d censusacquisition.Discovery, r censusrequest.Receipt, root string) censusenumerationprojection.Receipt {
				cancel()
				return publishCensusEnumerationProjection(ctx, d, r, root)
			}
		}},
		{name: "postcommit", reason: "", files: 1, setup: func(cancel context.CancelFunc, deps *censusCoreDependencies) {
			deps.publishEnumeration = func(ctx context.Context, d censusacquisition.Discovery, r censusrequest.Receipt, root string) censusenumerationprojection.Receipt {
				got := publishCensusEnumerationProjection(ctx, d, r, root)
				cancel()
				return got
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			_ = os.Chmod(root, 0700)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			discovery := incompleteEnumerationDiscovery(t)
			deps := censusCoreDependencies{runSession: func(_ initializedAcquisitionRunnerConfig, cb func(context.Context, *initializedAcquisitionRuntime) int) int {
				return cb(ctx, &initializedAcquisitionRuntime{sessionID: "s", generation: 1})
			}, discover: func(context.Context, censusCLIOptions, *initializedAcquisitionRuntime, bool) (censusacquisition.Discovery, error) {
				return discovery, nil
			}}
			tc.setup(cancel, &deps)
			out := runCensusCore(censusCLIOptions{PrivateEnumerationProjection: true, Machine: true, PublicationRoot: root, Sources: []string{"."}, DownDepth: 1, MaxNodes: 10000, MaxBatchTargets: 63, Timeout: 60000000000, RequestTimeout: 30000000000}, censusCoreConfig{}, deps)
			if out.DiscoveryDiagnostic == nil || out.RequestReceipt == nil || out.EnumerationReceipt == nil {
				t.Fatalf("typed outcome lost: %+v", out)
			}
			if tc.reason != "" && (out.EnumerationReceipt.Status != "UNAVAILABLE" || out.EnumerationReceipt.UnavailableReason != tc.reason || out.EnumerationReceipt.Selector != "" || out.EnumerationReceipt.Digest != "") {
				t.Fatalf("receipt=%+v", out.EnumerationReceipt)
			}
			if tc.reason == "" && out.EnumerationReceipt.Status != "PUBLISHED" {
				t.Fatalf("committed receipt lost: %+v", out.EnumerationReceipt)
			}
			entries, _ := os.ReadDir(root)
			if len(entries) != tc.files {
				t.Fatalf("writes=%d want=%d", len(entries), tc.files)
			}
		})
	}
}

func TestEnumerationPublicationLinearizationPreEntryAndPostEntryCancellation(t *testing.T) {
	discovery := incompleteEnumerationDiscovery(t)
	request, err := censusrequest.New(censusrequest.RefreshCoordinate{SessionID: "s", Generation: 1}, censusrequest.SemanticFields{Sources: []string{"."}, DownDepth: 1, MaxNodes: 631, BatchTargets: 63, TimeoutMS: 60000, RequestTimeoutMS: 30000})
	if err != nil {
		t.Fatal(err)
	}
	old := publishEnumerationProjection
	defer func() { publishEnumerationProjection = old }()
	calls := 0
	publishEnumerationProjection = func(root *publication.Root, p censusenumerationprojection.Projection) (censusenumerationprojection.Receipt, error) {
		calls++
		return old(root, p)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	pre := publishCensusEnumerationProjection(ctx, discovery, request, t.TempDir())
	if calls != 0 || pre.Status != "UNAVAILABLE" || pre.UnavailableReason != "CANCELED_PRE_PUBLICATION" {
		t.Fatalf("calls=%d receipt=%+v", calls, pre)
	}
	root := t.TempDir()
	_ = os.Chmod(root, 0700)
	ctx, cancel = context.WithCancel(context.Background())
	publishEnumerationProjection = func(pubRoot *publication.Root, p censusenumerationprojection.Projection) (censusenumerationprojection.Receipt, error) {
		calls++
		cancel()
		return old(pubRoot, p)
	}
	post := publishCensusEnumerationProjection(ctx, discovery, request, root)
	if calls != 1 || post.Status != "PUBLISHED" {
		t.Fatalf("calls=%d receipt=%+v", calls, post)
	}
}

func TestEnumerationFlagAbsentRunnerOutputCallsAndFilesFrozen(t *testing.T) {
	root := t.TempDir()
	discovery := incompleteEnumerationDiscovery(t)
	calls := 0
	deps := censusCoreDependencies{runSession: func(_ initializedAcquisitionRunnerConfig, cb func(context.Context, *initializedAcquisitionRuntime) int) int {
		return cb(context.Background(), &initializedAcquisitionRuntime{sessionID: "s", generation: 1})
	}, discover: func(context.Context, censusCLIOptions, *initializedAcquisitionRuntime, bool) (censusacquisition.Discovery, error) {
		return discovery, nil
	}, publishEnumeration: func(context.Context, censusacquisition.Discovery, censusrequest.Receipt, string) censusenumerationprojection.Receipt {
		calls++
		return censusenumerationprojection.Unavailable("PUBLICATION_FAILED")
	}}
	out := runCensusCore(censusCLIOptions{Machine: true, PublicationRoot: root, Sources: []string{"."}, DownDepth: 1, MaxNodes: 631, MaxBatchTargets: 63, Timeout: 60000000000, RequestTimeout: 30000000000}, censusCoreConfig{}, deps)
	files, err := os.ReadDir(root)
	if err != nil || calls != 0 || len(files) != 0 || out.DiscoveryDiagnostic == nil || out.RequestReceipt == nil || out.EnumerationReceipt != nil {
		t.Fatalf("calls=%d files=%d out=%+v err=%v", calls, len(files), out, err)
	}
	var stdout, stderr bytes.Buffer
	if code := writeCensusOutcome(&stdout, &stderr, true, out); code != 1 || stdout.Len() != 0 || bytes.Count(stderr.Bytes(), []byte{'\n'}) != 1 {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.Bytes(), stderr.Bytes())
	}
}

func TestFlagAbsentOptionsCompatibility(t *testing.T) {
	args := []string{"census", "--machine", "--workspace", "work", "--source", ".", "--publication-root", "/private", "--server", "gopls"}
	a, e := parseCensusCLIOptions(args)
	if e != nil {
		t.Fatal(e)
	}
	b, e := parseCensusCLIOptions(args)
	if e != nil || !reflect.DeepEqual(a, b) || a.PrivateEnumerationProjection {
		t.Fatalf("%+v %+v %v", a, b, e)
	}
}
