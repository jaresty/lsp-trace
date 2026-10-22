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

func TestPrivateEnumerationProjectionEmitsScalarCauseSummary(t *testing.T) {
	root := t.TempDir()
	discovery := incompleteEnumerationDiscovery(t)
	deps := censusCoreDependencies{runSession: func(_ initializedAcquisitionRunnerConfig, cb func(context.Context, *initializedAcquisitionRuntime) int) int {
		return cb(context.Background(), &initializedAcquisitionRuntime{sessionID: "s", generation: 1})
	}, discover: func(context.Context, censusCLIOptions, *initializedAcquisitionRuntime, bool) (censusacquisition.Discovery, error) {
		return discovery, nil
	}, publishEnumeration: func(context.Context, censusacquisition.Discovery, censusrequest.Receipt, string) censusenumerationprojection.Receipt {
		return censusenumerationprojection.Unavailable("PUBLICATION_FAILED")
	}}
	out := runCensusCore(censusCLIOptions{PrivateEnumerationProjection: true, Machine: true, PublicationRoot: root, Sources: []string{"."}, DownDepth: 1, MaxNodes: 631, MaxBatchTargets: 63, Timeout: 60000000000, RequestTimeout: 30000000000}, censusCoreConfig{}, deps)

	var publicOut, publicErr, privateOut, privateErr bytes.Buffer
	public := out
	public.EnumerationReceipt = nil
	public.PrivateDiscoveryCauses = nil
	if writeCensusOutcome(&publicOut, &publicErr, true, public) != 1 || writeCensusOutcome(&privateOut, &privateErr, true, out) != 1 {
		t.Fatal("unexpected exit")
	}
	lines := bytes.Split(bytes.TrimSuffix(privateErr.Bytes(), []byte{'\n'}), []byte{'\n'})
	if len(lines) != 3 || !bytes.Equal(lines[0], bytes.TrimSuffix(publicErr.Bytes(), []byte{'\n'})) {
		t.Fatalf("ASSERT_PRIVATE_CAUSE_SUMMARY_ADDITIVE public=%q private=%q", publicErr.Bytes(), privateErr.Bytes())
	}
	var summary struct {
		SchemaVersion string `json:"schema_version"`
		Authority     int    `json:"authority"`
		Causes        []struct {
			Scope        string `json:"scope"`
			Stage        string `json:"stage"`
			Code         string `json:"code"`
			Count        uint64 `json:"count"`
			FirstOrdinal uint64 `json:"first_ordinal"`
		} `json:"causes"`
	}
	if err := json.Unmarshal(lines[2], &summary); err != nil {
		t.Fatalf("ASSERT_PRIVATE_CAUSE_SUMMARY_JSON: %v: %q", err, lines[2])
	}
	if summary.SchemaVersion != "lsp-trace.private-census-discovery-causes.v1" || summary.Authority != 0 || !reflect.DeepEqual(summary.Causes, []struct {
		Scope        string `json:"scope"`
		Stage        string `json:"stage"`
		Code         string `json:"code"`
		Count        uint64 `json:"count"`
		FirstOrdinal uint64 `json:"first_ordinal"`
	}{{Scope: "FILE", Stage: "DISCOVERY", Code: "CHILD_INCOMPLETE", Count: 1, FirstOrdinal: 0}, {Scope: "SYMBOL", Stage: "PREPARE_CALL_HIERARCHY", Code: "RESULT_MISMATCH", Count: 1, FirstOrdinal: 0}}) {
		t.Fatalf("ASSERT_PRIVATE_CAUSE_SUMMARY_VALUES: %+v", summary)
	}
	for _, forbidden := range [][]byte{[]byte(discovery.Workspace), []byte("a.go"), []byte("secret"), []byte(`"private"`)} {
		if bytes.Contains(lines[2], forbidden) {
			t.Fatalf("ASSERT_PRIVATE_CAUSE_SUMMARY_SCALAR_ONLY %q in %q", forbidden, lines[2])
		}
	}
	if publicOut.Len() != 0 || privateOut.Len() != 0 {
		t.Fatal("stdout mutated")
	}
}

func TestPrivateDiscoveryCauseSummaryTargetLimitScalarOnly(t *testing.T) {
	symbol2, symbol5 := 2, 5
	discovery := censusacquisition.Discovery{
		Workspace:  "/private/secret/workspace",
		Accounting: census.Accounting{SymbolDenominator: 6},
		IncompleteCauses: []censusacquisition.IncompleteCause{
			{Scope: "SYMBOL", Stage: "DISCOVERY", Code: "TARGET_LIMIT", SymbolOrdinal: &symbol5},
			{Scope: "SYMBOL", Stage: "DISCOVERY", Code: "TARGET_LIMIT", SymbolOrdinal: &symbol2},
		},
	}
	summary, err := newPrivateCensusDiscoveryCausesForDiscovery(discovery)
	if err != nil {
		t.Fatalf("ASSERT_TARGET_LIMIT_PRIVATE_SCALAR_SUMMARY: %v", err)
	}
	raw, err := marshalPrivateCensusDiscoveryCauses(summary)
	if err != nil {
		t.Fatalf("ASSERT_TARGET_LIMIT_PRIVATE_SCALAR_MARSHAL: %v", err)
	}
	want := `{"schema_version":"lsp-trace.private-census-discovery-causes.v1","authority":0,"causes":[{"scope":"SYMBOL","stage":"DISCOVERY","code":"TARGET_LIMIT","count":2,"first_ordinal":2}]}`
	if string(raw) != want {
		t.Fatalf("ASSERT_TARGET_LIMIT_PRIVATE_SCALAR_EXACT: got=%s want=%s", raw, want)
	}
	for _, forbidden := range []string{discovery.Workspace, "secret", "workspace", "uri", "path", "name", "source"} {
		if bytes.Contains(raw, []byte(forbidden)) {
			t.Fatalf("ASSERT_TARGET_LIMIT_PRIVATE_SCALAR_NO_PRIVACY_LEAK: %q in %s", forbidden, raw)
		}
	}
}

func TestPrivateDiscoveryCauseSummaryDistinguishesEqualTotals(t *testing.T) {
	file0, file4, symbol2, symbol9 := 0, 4, 2, 9
	fileSummary, err := newPrivateCensusDiscoveryCauses([]censusacquisition.IncompleteCause{
		{Scope: "FILE", Stage: "DOCUMENT_SUPPLY", Code: "SESSION_UNAVAILABLE", FileOrdinal: &file4},
		{Scope: "FILE", Stage: "DOCUMENT_SUPPLY", Code: "SESSION_UNAVAILABLE", FileOrdinal: &file0},
	})
	if err != nil {
		t.Fatal(err)
	}
	symbolSummary, err := newPrivateCensusDiscoveryCauses([]censusacquisition.IncompleteCause{
		{Scope: "SYMBOL", Stage: "PREPARE_CALL_HIERARCHY", Code: "RESULT_MISMATCH", SymbolOrdinal: &symbol9},
		{Scope: "SYMBOL", Stage: "PREPARE_CALL_HIERARCHY", Code: "RESULT_MISMATCH", SymbolOrdinal: &symbol2},
	})
	if err != nil {
		t.Fatal(err)
	}
	fileRaw, _ := marshalPrivateCensusDiscoveryCauses(fileSummary)
	symbolRaw, _ := marshalPrivateCensusDiscoveryCauses(symbolSummary)
	if bytes.Equal(fileRaw, symbolRaw) || len(fileSummary.Causes) != 1 || fileSummary.Causes[0].Count != 2 || fileSummary.Causes[0].FirstOrdinal != 0 || len(symbolSummary.Causes) != 1 || symbolSummary.Causes[0].Count != 2 || symbolSummary.Causes[0].FirstOrdinal != 2 {
		t.Fatalf("ASSERT_EQUAL_TOTAL_SCOPE_DISTINCTION file=%s symbol=%s", fileRaw, symbolRaw)
	}
}

func TestPrivateDiscoveryCauseSummaryRejectsMalformedVocabulary(t *testing.T) {
	fileOrdinal := 0
	for name, causes := range map[string][]censusacquisition.IncompleteCause{
		"unknown-stage": {{Scope: "FILE", Stage: "/private/source/path", Code: "SESSION_UNAVAILABLE", FileOrdinal: &fileOrdinal}},
		"unknown-code":  {{Scope: "FILE", Stage: "DOCUMENT_SUPPLY", Code: "provider prose", FileOrdinal: &fileOrdinal}},
		"control-code":  {{Scope: "FILE", Stage: "DOCUMENT_SUPPLY", Code: "SESSION_UNAVAILABLE\x00suffix", FileOrdinal: &fileOrdinal}},
	} {
		t.Run(name, func(t *testing.T) {
			if got, err := newPrivateCensusDiscoveryCauses(causes); err == nil {
				t.Fatalf("ASSERT_PRIVATE_CAUSE_REJECTS_MALFORMED got=%+v", got)
			}
		})
	}
	malformed := privateCensusDiscoveryCauses{SchemaVersion: privateCensusDiscoveryCausesSchemaVersion, Authority: 0, Causes: []privateCensusDiscoveryCause{{Scope: "FILE", Stage: "DOCUMENT_SUPPLY", Code: "provider prose", Count: 1}}}
	if raw, err := marshalPrivateCensusDiscoveryCauses(malformed); err == nil {
		t.Fatalf("ASSERT_PRIVATE_CAUSE_MARSHAL_REJECTS_MALFORMED raw=%q", raw)
	}
}

func TestPrivateDiscoveryCauseSummaryRejectsInvalidOrdinals(t *testing.T) {
	file0, file1, symbol0, symbol1 := 0, 1, 0, 1
	for name, discovery := range map[string]censusacquisition.Discovery{
		"file-out-of-range":   {Accounting: census.Accounting{FileDenominator: 1}, IncompleteCauses: []censusacquisition.IncompleteCause{{Scope: "FILE", Stage: "DOCUMENT_SUPPLY", Code: "SESSION_UNAVAILABLE", FileOrdinal: &file1}}},
		"symbol-out-of-range": {Accounting: census.Accounting{SymbolDenominator: 1}, IncompleteCauses: []censusacquisition.IncompleteCause{{Scope: "SYMBOL", Stage: "PREPARE_CALL_HIERARCHY", Code: "RESULT_MISMATCH", SymbolOrdinal: &symbol1}}},
		"wrong-file-kind":     {Accounting: census.Accounting{FileDenominator: 1, SymbolDenominator: 1}, IncompleteCauses: []censusacquisition.IncompleteCause{{Scope: "FILE", Stage: "DOCUMENT_SUPPLY", Code: "SESSION_UNAVAILABLE", SymbolOrdinal: &symbol0}}},
		"both-kinds":          {Accounting: census.Accounting{FileDenominator: 1, SymbolDenominator: 1}, IncompleteCauses: []censusacquisition.IncompleteCause{{Scope: "SYMBOL", Stage: "PREPARE_CALL_HIERARCHY", Code: "RESULT_MISMATCH", FileOrdinal: &file0, SymbolOrdinal: &symbol0}}},
	} {
		t.Run(name, func(t *testing.T) {
			if got, err := newPrivateCensusDiscoveryCausesForDiscovery(discovery); err == nil {
				t.Fatalf("ASSERT_PRIVATE_CAUSE_REJECTS_INVALID_ORDINAL got=%+v", got)
			}
		})
	}
}

func TestPrivateDiscoveryCauseSummaryV2AndZeroCauseOrdering(t *testing.T) {
	diagnostic, err := censusresult.NewDiscoveryDiagnostic([]censusresult.DiscoveryObservation{{Dimension: censusresult.DimensionProviderCompleteness, State: censusresult.ObservationKnown, Omitted: ptrU64(1)}})
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := censusrequest.NewV2(censusrequest.RefreshCoordinate{SessionID: "s", Generation: 1}, censusrequest.SemanticFieldsV2{Sources: []string{"."}, PrunedDirectories: []string{"vendor"}, DownDepth: 1, MaxNodes: 10000, BatchTargets: 63, TimeoutMS: 60000, RequestTimeoutMS: 30000})
	if err != nil {
		t.Fatal(err)
	}
	zero := censusPublicationOutcome{DiscoveryDiagnostic: &diagnostic, RequestReceiptV2: &receipt}
	var zeroOut, zeroErr bytes.Buffer
	if code := writeCensusOutcome(&zeroOut, &zeroErr, true, zero); code != 1 || zeroOut.Len() != 0 || bytes.Count(zeroErr.Bytes(), []byte{'\n'}) != 1 {
		t.Fatalf("ASSERT_PRIVATE_CAUSE_V2_ZERO_COMPAT code=%d stdout=%q stderr=%q", code, zeroOut.Bytes(), zeroErr.Bytes())
	}
	fileOrdinal := 3
	summary, err := newPrivateCensusDiscoveryCauses([]censusacquisition.IncompleteCause{{Scope: "FILE", Stage: "DOCUMENT_SUPPLY", Code: "SESSION_UNAVAILABLE", FileOrdinal: &fileOrdinal}})
	if err != nil {
		t.Fatal(err)
	}
	withCauses := zero
	withCauses.PrivateDiscoveryCauses = &summary
	var privateOut, privateErr bytes.Buffer
	if code := writeCensusOutcome(&privateOut, &privateErr, true, withCauses); code != 1 || privateOut.Len() != 0 {
		t.Fatalf("ASSERT_PRIVATE_CAUSE_V2_EXIT code=%d stdout=%q", code, privateOut.Bytes())
	}
	lines := bytes.Split(bytes.TrimSuffix(privateErr.Bytes(), []byte{'\n'}), []byte{'\n'})
	canonical, _ := marshalPrivateCensusDiscoveryCauses(summary)
	if len(lines) != 2 || !bytes.Equal(lines[0], bytes.TrimSuffix(zeroErr.Bytes(), []byte{'\n'})) || !bytes.Equal(lines[1], canonical) {
		t.Fatalf("ASSERT_PRIVATE_CAUSE_V2_ORDER zero=%q private=%q", zeroErr.Bytes(), privateErr.Bytes())
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
