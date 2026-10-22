package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"lsp-trace/internal/census"
	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/censusenumerationprojection"
	"lsp-trace/internal/censusrequest"
	"lsp-trace/internal/censusresult"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/sessionruntime"
)

type terminalManagerRuntime struct {
	manager    *sessionruntime.Manager
	prepareCtx context.Context
	sessionID  string
	generation uint64
}

func (r terminalManagerRuntime) SessionID() string  { return r.sessionID }
func (r terminalManagerRuntime) Generation() uint64 { return r.generation }
func (r terminalManagerRuntime) PrepareDocument(_ context.Context, req sessionruntime.DocumentRequest) sessionruntime.DocumentResult {
	req.SessionID, req.Generation = r.sessionID, r.generation
	return r.manager.PrepareDocument(r.prepareCtx, req)
}

type terminalWireChild struct{ bytes.Buffer }

func (c *terminalWireChild) Stdin() io.WriteCloser { return nopWriteCloser{Writer: c} }
func (*terminalWireChild) Stdout() io.ReadCloser   { return io.NopCloser(bytes.NewReader(nil)) }
func (*terminalWireChild) Teardown(context.Context) managedprocess.TeardownObservation {
	return managedprocess.TeardownObservation{Death: managedprocess.DeathObservation{Reap: managedprocess.ReapObservation{Kind: managedprocess.ReapComplete}}}
}
func (*terminalWireChild) Close() managedprocess.ResourceObservation {
	return managedprocess.ResourceObservation{Kind: managedprocess.ResourcesClosed}
}

type terminalStarter struct{ child sessionruntime.Child }

func (s terminalStarter) Start(context.Context, managedprocess.Spec) (sessionruntime.Child, managedprocess.StartObservation) {
	return s.child, managedprocess.StartObservation{Kind: managedprocess.StartStarted}
}

type nopWriteCloser struct{ io.Writer }

func (nopWriteCloser) Close() error { return nil }

func TestRealManagerProviderFreeTerminalObservationE2EReachableLeaseTokenMismatch(t *testing.T) {
	workspace := t.TempDir()
	path := filepath.Join(workspace, "a.go")
	if err := os.WriteFile(path, []byte("package p\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	selected, err := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: "test", Workspace: workspace, Profile: "go", EnvironmentReference: "local"})
	if err != nil {
		t.Fatal(err)
	}
	manager, err := sessionruntime.New(sessionruntime.Config{Limits: sessionruntime.Limits{MaxSessions: 1, MaxRequests: 1, MaxChildren: 2, MaxCancels: 2, MaxTombstones: 2, MaxObservations: 16}, Starter: terminalStarter{child: &terminalWireChild{}}})
	if err != nil {
		t.Fatal(err)
	}
	started := manager.Start(context.Background(), sessionruntime.StartRequest{Profile: runtimeprofile.Resolve(selected), LanguageID: "go"})
	if started.Failure != "" {
		t.Fatal(started)
	}
	if initialized := manager.ObserveInitialization(started.SessionID, started.Generation, true); initialized.Failure != "" {
		t.Fatal(initialized)
	}
	var staleLease context.Context
	if err := manager.WithReadySessionLease(context.Background(), sessionruntime.SessionLeaseRequest{SessionID: started.SessionID, Generation: started.Generation, Workspace: workspace}, func(ctx context.Context) error { staleLease = ctx; return nil }); err != nil {
		t.Fatal(err)
	}
	runtime := terminalManagerRuntime{manager: manager, prepareCtx: staleLease, sessionID: started.SessionID, generation: started.Generation}
	file := censusacquisition.SourceFile{Path: "a.go", URI: (&url.URL{Scheme: "file", Path: path}).String(), LanguageID: "go"}
	d, discoverErr := (censusacquisition.DiscoveryAdapter{Workspace: workspace, Files: censusacquisition.StaticFiles{file}, Supplier: censusRuntimeDocumentSupplier{runtime: runtime}, Client: terminalDiscoveryClient{}}).Discover(context.Background(), censusacquisition.SessionIdentity{SessionID: started.SessionID, Generation: started.Generation})
	if discoverErr != nil || len(d.IncompleteCauses) != 1 || d.IncompleteCauses[0].LifecycleSubcause != censusacquisition.LifecycleSubcauseLeaseTokenMismatch {
		t.Fatalf("ASSERT_PRIVATE_TERMINAL_REAL_MANAGER_DISCOVERY discovery=%+v err=%v", d, discoverErr)
	}
	receipt := censusenumerationprojection.Unavailable("PUBLICATION_FAILED")
	observation, err := newPrivateCensusTerminalObservation(d, receipt, nil)
	if err != nil {
		t.Fatal(err)
	}
	request, _ := censusrequest.New(censusrequest.RefreshCoordinate{SessionID: started.SessionID, Generation: started.Generation}, censusrequest.SemanticFields{Sources: []string{"."}, DownDepth: 1, MaxNodes: 1, BatchTargets: 1, TimeoutMS: 1, RequestTimeoutMS: 1})
	diagnostic, _ := censusresult.NewDiscoveryDiagnosticFromAcquisition(d.Observations, request.Semantic)
	outcome := censusPublicationOutcome{DiscoveryDiagnostic: &diagnostic, RequestReceipt: &request, EnumerationReceipt: &receipt, PrivateTerminalObservation: &observation}
	var stdout, stderr bytes.Buffer
	if code := writeCensusOutcome(&stdout, &stderr, true, outcome); code != 1 || !bytes.Contains(stderr.Bytes(), []byte(`"lifecycle_subcause":"LEASE_TOKEN_MISMATCH"`)) {
		t.Fatalf("ASSERT_PRIVATE_TERMINAL_REAL_MANAGER_E2E code=%d stdout=%q stderr=%q", code, stdout.Bytes(), stderr.Bytes())
	}
}

type fakeTerminalSupplier struct{ err error }

func (s *fakeTerminalSupplier) Supply(context.Context, censusacquisition.SourceFile) error {
	return s.err
}

type terminalDiscoveryClient struct{}

func (terminalDiscoveryClient) SupportsDocumentSymbols() bool { return true }
func (terminalDiscoveryClient) SupportsCallHierarchy() bool   { return true }
func (terminalDiscoveryClient) DocumentSymbols(context.Context, lsp.DocumentSymbolParams) ([]lsp.DocumentSymbol, error) {
	return nil, nil
}
func (terminalDiscoveryClient) PrepareCallHierarchy(context.Context, lsp.PrepareCallHierarchyParams) ([]lsp.CallHierarchyItem, error) {
	return nil, nil
}

func TestPrivateTerminalObservationVocabularyValidationRankingAndCanonicalBytes(t *testing.T) {
	if err := validatePrivateCensusTerminalVocabularyClosure(sessionruntime.ProducedLifecycleSubcauses()); err != nil {
		t.Fatalf("ASSERT_PRIVATE_TERMINAL_VOCABULARY_CLOSURE %v", err)
	}
	first := 1
	d := censusacquisition.Discovery{Accounting: census.Accounting{FileDenominator: 4}, IncompleteCauses: []censusacquisition.IncompleteCause{{Scope: "FILE", Stage: "DOCUMENT_SUPPLY", Code: "LIFECYCLE_CONFLICT", LifecycleSubcause: censusacquisition.LifecycleSubcauseProtocolOwned, FileOrdinal: &first}}}
	r := censusenumerationprojection.Unavailable("PUBLICATION_FAILED")
	o, err := newPrivateCensusTerminalObservation(d, r, context.Canceled)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := marshalPrivateCensusTerminalObservation(o)
	if err != nil {
		t.Fatal(err)
	}
	const want = `{"schema_version":"lsp-trace.private-census-terminal-observation.v1","authority":0,"scope":"FILE","stage":"DOCUMENT_SUPPLY","cause":"LIFECYCLE_CONFLICT","lifecycle_subcause":"PROTOCOL_OWNED","context_state":"CANCELED","encoding_category":"NOT_APPLICABLE","first_ordinal":1,"count":1,"contiguous_suffix":"FALSE"}`
	if string(raw) != want {
		t.Fatalf("ASSERT_PRIVATE_TERMINAL_CANONICAL got=%s want=%s", raw, want)
	}
	var round privateCensusTerminalObservation
	if json.Unmarshal(raw, &round) != nil || !reflect.DeepEqual(round, o) {
		t.Fatalf("ASSERT_PRIVATE_TERMINAL_ROUNDTRIP %+v", round)
	}
	bad := o
	bad.Authority = 1
	if _, err := marshalPrivateCensusTerminalObservation(bad); err == nil {
		t.Fatal("ASSERT_PRIVATE_TERMINAL_AUTHORITY_FAIL_CLOSED")
	}
	bad = o
	bad.LifecycleSubcause = "provider prose"
	if _, err := marshalPrivateCensusTerminalObservation(bad); err == nil {
		t.Fatal("ASSERT_PRIVATE_TERMINAL_VOCAB_FAIL_CLOSED")
	}
	for _, value := range privateCensusTerminalVocabulary {
		ordinal := 0
		cause := censusacquisition.IncompleteCause{Scope: value.scope, Stage: value.stage, Code: value.cause}
		discovery := censusacquisition.Discovery{Accounting: census.Accounting{FileDenominator: 1, SymbolDenominator: 1}}
		if value.scope == "FILE" {
			cause.FileOrdinal = &ordinal
		} else {
			cause.SymbolOrdinal = &ordinal
		}
		if value.cause == "LIFECYCLE_CONFLICT" {
			cause.LifecycleSubcause = censusacquisition.LifecycleSubcauseProtocolOwned
		}
		discovery.IncompleteCauses = []censusacquisition.IncompleteCause{cause}
		got, buildErr := newPrivateCensusTerminalObservation(discovery, censusenumerationprojection.Unavailable("PUBLICATION_FAILED"), nil)
		if buildErr != nil {
			t.Fatalf("ASSERT_PRIVATE_TERMINAL_CAUSE_CLOSURE cause=%+v err=%v", value, buildErr)
		}
		if _, marshalErr := marshalPrivateCensusTerminalObservation(got); marshalErr != nil {
			t.Fatalf("ASSERT_PRIVATE_TERMINAL_CAUSE_SERIALIZE cause=%+v err=%v", value, marshalErr)
		}
	}
	later := 2
	mixed, mixedErr := newPrivateCensusTerminalObservation(censusacquisition.Discovery{Accounting: census.Accounting{FileDenominator: 4}, IncompleteCauses: []censusacquisition.IncompleteCause{
		{Scope: "FILE", Stage: "DISCOVERY", Code: "FILE_LIMIT", FileOrdinal: &first},
		{Scope: "FILE", Stage: "DOCUMENT_SUPPLY", Code: "LIFECYCLE_CONFLICT", LifecycleSubcause: censusacquisition.LifecycleSubcauseProtocolOwned, FileOrdinal: &later},
	}}, censusenumerationprojection.Unavailable("PUBLICATION_FAILED"), nil)
	if mixedErr != nil || mixed.Cause != "LIFECYCLE_CONFLICT" || mixed.FirstOrdinal != uint64(later) {
		t.Fatalf("ASSERT_PRIVATE_TERMINAL_MIXED_RANKING got=%+v err=%v", mixed, mixedErr)
	}
	base := o
	for _, state := range []string{"ACTIVE", "CANCELED", "DEADLINE_EXCEEDED"} {
		base.ContextState = state
		if _, err := marshalPrivateCensusTerminalObservation(base); err != nil {
			t.Fatalf("ASSERT_PRIVATE_TERMINAL_CONTEXT_CLOSURE state=%s err=%v", state, err)
		}
	}
	base.ContextState = "ACTIVE"
	for _, state := range []string{"BOUND_EXHAUSTION", "INVALID_INPUT", "OTHER_ENCODING_FAILURE", terminalNotApplicable} {
		base.EncodingCategory = state
		if _, err := marshalPrivateCensusTerminalObservation(base); err != nil {
			t.Fatalf("ASSERT_PRIVATE_TERMINAL_ENCODING_CLOSURE state=%s err=%v", state, err)
		}
	}
	base.EncodingCategory = terminalNotApplicable
	for _, state := range []string{"TRUE", "FALSE", terminalUnknown} {
		base.ContiguousSuffix = state
		if _, err := marshalPrivateCensusTerminalObservation(base); err != nil {
			t.Fatalf("ASSERT_PRIVATE_TERMINAL_SUFFIX_CLOSURE state=%s err=%v", state, err)
		}
	}
	base.ContiguousSuffix = "TRUE"
	base.LifecycleSubcause = terminalUnknown
	if _, err := marshalPrivateCensusTerminalObservation(base); err != nil {
		t.Fatalf("ASSERT_PRIVATE_TERMINAL_LEGACY_UNKNOWN_VALID err=%v", err)
	}
	for _, mutate := range []func(*privateCensusTerminalObservation){
		func(v *privateCensusTerminalObservation) {
			v.Cause = "FILE_LIMIT"
			v.Stage = "DISCOVERY"
			v.LifecycleSubcause = terminalUnknown
		},
		func(v *privateCensusTerminalObservation) { v.LifecycleSubcause = terminalNotApplicable },
		func(v *privateCensusTerminalObservation) { v.ContextState = "UNKNOWN" },
		func(v *privateCensusTerminalObservation) { v.EncodingCategory = "UNKNOWN" },
		func(v *privateCensusTerminalObservation) { v.ContiguousSuffix = terminalNotApplicable },
	} {
		invalid := base
		mutate(&invalid)
		if _, err := marshalPrivateCensusTerminalObservation(invalid); err == nil {
			t.Fatalf("ASSERT_PRIVATE_TERMINAL_INVALID_COMBINATION_REJECTED value=%+v", invalid)
		}
	}
}

func TestPrivateTerminalObservationRejectsMalformedOrdinals(t *testing.T) {
	file, symbol, negative, zero, one := 0, 0, -1, 0, 1
	base := censusacquisition.IncompleteCause{Scope: "FILE", Stage: "DISCOVERY", Code: "FILE_LIMIT", FileOrdinal: &zero}
	cases := []struct {
		name        string
		denominator int
		causes      []censusacquisition.IncompleteCause
	}{
		{"missing", 1, []censusacquisition.IncompleteCause{{Scope: "FILE", Stage: "DISCOVERY", Code: "FILE_LIMIT"}}},
		{"both", 1, []censusacquisition.IncompleteCause{{Scope: "FILE", Stage: "DISCOVERY", Code: "FILE_LIMIT", FileOrdinal: &file, SymbolOrdinal: &symbol}}},
		{"file-wrong-kind", 1, []censusacquisition.IncompleteCause{{Scope: "FILE", Stage: "DISCOVERY", Code: "FILE_LIMIT", SymbolOrdinal: &symbol}}},
		{"symbol-wrong-kind", 1, []censusacquisition.IncompleteCause{{Scope: "SYMBOL", Stage: "DISCOVERY", Code: "SYMBOL_LIMIT", FileOrdinal: &file}}},
		{"negative", 1, []censusacquisition.IncompleteCause{{Scope: "FILE", Stage: "DISCOVERY", Code: "FILE_LIMIT", FileOrdinal: &negative}}},
		{"duplicate", 2, []censusacquisition.IncompleteCause{base, base}},
		{"equal-denominator", 1, []censusacquisition.IncompleteCause{{Scope: "FILE", Stage: "DISCOVERY", Code: "FILE_LIMIT", FileOrdinal: &one}}},
		{"zero-denominator", 0, []censusacquisition.IncompleteCause{base}},
		{"count-overflow", 1, []censusacquisition.IncompleteCause{base, {Scope: "FILE", Stage: "DISCOVERY", Code: "FILE_LIMIT", FileOrdinal: &one}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := censusacquisition.Discovery{Accounting: census.Accounting{FileDenominator: tc.denominator, SymbolDenominator: tc.denominator}, IncompleteCauses: tc.causes}
			if got, err := newPrivateCensusTerminalObservation(d, censusenumerationprojection.Unavailable("PUBLICATION_FAILED"), nil); err == nil || got != (privateCensusTerminalObservation{}) {
				t.Fatalf("ASSERT_PRIVATE_TERMINAL_MALFORMED_ORDINAL_REJECTED case=%s got=%+v err=%v", tc.name, got, err)
			}
		})
	}
}

func TestPrivateTerminalObservationEncodingContextSuffixAndWriteOrder(t *testing.T) {
	for _, tc := range []struct {
		name    string
		receipt censusenumerationprojection.Receipt
		want    string
	}{
		{"bound", mustEncodingReceipt(t, censusenumerationprojection.EncodingBoundExhaustion), "BOUND_EXHAUSTION"},
		{"invalid", mustEncodingReceipt(t, censusenumerationprojection.EncodingInvalidInput), "INVALID_INPUT"},
		{"other", mustEncodingReceipt(t, censusenumerationprojection.EncodingOtherFailure), "OTHER_ENCODING_FAILURE"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			i := 2
			d := censusacquisition.Discovery{Accounting: census.Accounting{FileDenominator: 3}, IncompleteCauses: []censusacquisition.IncompleteCause{{Scope: "FILE", Stage: "DISCOVERY", Code: "FILE_LIMIT", FileOrdinal: &i}}}
			o, err := newPrivateCensusTerminalObservation(d, tc.receipt, context.DeadlineExceeded)
			if err != nil || o.EncodingCategory != tc.want || o.ContextState != "DEADLINE_EXCEEDED" || o.ContiguousSuffix != "TRUE" {
				t.Fatalf("ASSERT_PRIVATE_TERMINAL_ENCODER_CONTEXT_SUFFIX %+v err=%v", o, err)
			}
		})
	}

	cause := privateCensusDiscoveryCauses{SchemaVersion: privateCensusDiscoveryCausesSchemaVersion, Authority: 0, Causes: []privateCensusDiscoveryCause{{Scope: "FILE", Stage: "DISCOVERY", Code: "FILE_LIMIT", Count: 1}}}
	o := privateCensusTerminalObservation{SchemaVersion: privateCensusTerminalObservationSchemaVersion, Authority: 0, Scope: "FILE", Stage: "DISCOVERY", Cause: "FILE_LIMIT", LifecycleSubcause: "NOT_APPLICABLE", ContextState: "ACTIVE", EncodingCategory: "NOT_APPLICABLE", FirstOrdinal: 0, Count: 1, ContiguousSuffix: "TRUE"}
	diagnostic, _ := censusresult.NewDiscoveryDiagnostic([]censusresult.DiscoveryObservation{{Dimension: censusresult.DimensionProviderCompleteness, State: censusresult.ObservationKnown, Omitted: ptrU64(1)}})
	request, _ := censusrequest.New(censusrequest.RefreshCoordinate{SessionID: "s", Generation: 1}, censusrequest.SemanticFields{Sources: []string{"."}, DownDepth: 1, MaxNodes: 100, BatchTargets: 63, TimeoutMS: 1000, RequestTimeoutMS: 1000})
	outcome := censusPublicationOutcome{DiscoveryDiagnostic: &diagnostic, RequestReceipt: &request, PrivateDiscoveryCauses: &cause, PrivateTerminalObservation: &o}
	var stdout, stderr bytes.Buffer
	if code := writeCensusOutcome(&stdout, &stderr, true, outcome); code != 1 {
		t.Fatalf("ASSERT_PRIVATE_TERMINAL_WRITE_EXIT %d", code)
	}
	lines := bytes.Split(bytes.TrimSpace(stderr.Bytes()), []byte{'\n'})
	if len(lines) < 2 || !bytes.Contains(lines[len(lines)-2], []byte(privateCensusDiscoveryCausesSchemaVersion)) || !bytes.Contains(lines[len(lines)-1], []byte(privateCensusTerminalObservationSchemaVersion)) {
		t.Fatalf("ASSERT_PRIVATE_TERMINAL_WRITE_ORDER %q", stderr.Bytes())
	}
	terminalRaw, _ := marshalPrivateCensusTerminalObservation(o)
	requestV2, _ := censusrequest.NewV2(censusrequest.RefreshCoordinate{SessionID: "s", Generation: 1}, censusrequest.SemanticFieldsV2{Sources: []string{"."}, PrunedDirectories: []string{"vendor"}, DownDepth: 1, MaxNodes: 100, BatchTargets: 63, TimeoutMS: 1000, RequestTimeoutMS: 1000})
	invalidTerminal := o
	invalidTerminal.Count = 0
	invalidOutcome := outcome
	invalidOutcome.PrivateTerminalObservation = &invalidTerminal
	var invalidOut, invalidErr bytes.Buffer
	if code := writeCensusOutcome(&invalidOut, &invalidErr, true, invalidOutcome); code != 1 || invalidOut.Len() != 0 || bytes.Contains(invalidErr.Bytes(), []byte(privateCensusTerminalObservationSchemaVersion)) || !bytes.Contains(invalidErr.Bytes(), []byte(`"status":"FAILED"`)) {
		t.Fatalf("ASSERT_PRIVATE_TERMINAL_WRITER_FAILS_CLOSED code=%d stdout=%q stderr=%q", code, invalidOut.Bytes(), invalidErr.Bytes())
	}
	for _, tc := range []struct {
		name  string
		value censusPublicationOutcome
	}{
		{"receipt", outcome},
		{"receipt-v2", censusPublicationOutcome{DiscoveryDiagnostic: &diagnostic, RequestReceiptV2: &requestV2, PrivateDiscoveryCauses: &cause, PrivateTerminalObservation: &o}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			baseline := tc.value
			baseline.PrivateTerminalObservation = nil
			var baselineOut, baselineErr, optInOut, optInErr bytes.Buffer
			baselineCode := writeCensusOutcome(&baselineOut, &baselineErr, true, baseline)
			optInCode := writeCensusOutcome(&optInOut, &optInErr, true, tc.value)
			wantErr := append(append([]byte(nil), baselineErr.Bytes()...), append(terminalRaw, '\n')...)
			if baselineCode != optInCode || !bytes.Equal(baselineOut.Bytes(), optInOut.Bytes()) || !bytes.Equal(optInErr.Bytes(), wantErr) {
				t.Fatalf("ASSERT_PRIVATE_TERMINAL_EXACT_ADDITIVE case=%s baseline_code=%d optin_code=%d baseline_out=%q optin_out=%q baseline_err=%q optin_err=%q want_err=%q", tc.name, baselineCode, optInCode, baselineOut.Bytes(), optInOut.Bytes(), baselineErr.Bytes(), optInErr.Bytes(), wantErr)
			}
		})
	}
}

func mustEncodingReceipt(t *testing.T, c censusenumerationprojection.EncodingFailureCategory) censusenumerationprojection.Receipt {
	t.Helper()
	r, err := censusenumerationprojection.UnavailableEncodingFailure(c)
	if err != nil {
		t.Fatal(err)
	}
	return r
}
