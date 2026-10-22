package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/censusenumerationprojection"
	"lsp-trace/internal/censusrequest"
	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/internal/session"
	"lsp-trace/sessionruntime"
)

const sustainedFileCount = 16
const sustainedMalformedRequest = 7

type sustainedWireChild struct {
	input  *io.PipeReader
	stdin  *io.PipeWriter
	output *io.PipeWriter
	stdout *io.PipeReader

	mu                     sync.Mutex
	documentSymbolRequests int
	methods                []string
}

func newSustainedWireChild() *sustainedWireChild {
	input, stdin := io.Pipe()
	stdout, output := io.Pipe()
	child := &sustainedWireChild{input: input, stdin: stdin, output: output, stdout: stdout}
	go child.serve()
	return child
}

func (c *sustainedWireChild) serve() {
	reader := lspwire.NewReader(c.input, lspwire.DefaultLimits())
	writer := lspwire.NewWriter(c.output, lspwire.DefaultLimits())
	for {
		message, err := reader.Read()
		if err != nil {
			return
		}
		c.mu.Lock()
		c.methods = append(c.methods, message.Method)
		if message.Method == "textDocument/documentSymbol" {
			c.documentSymbolRequests++
		}
		requestOrdinal := c.documentSymbolRequests
		c.mu.Unlock()
		if message.Method == "textDocument/didOpen" || message.Method == "textDocument/didChange" {
			continue
		}
		if message.Method != "textDocument/documentSymbol" {
			continue
		}
		if requestOrdinal == sustainedMalformedRequest {
			_, _ = io.WriteString(c.output, "Content-Length: nope\r\n\r\n{}")
			continue
		}
		_ = writer.Write(lspwire.Message{JSONRPC: lspwire.Version, ID: message.ID, Result: json.RawMessage(`[]`)})
	}
}

func (c *sustainedWireChild) Stdin() io.WriteCloser { return c.stdin }
func (c *sustainedWireChild) Stdout() io.ReadCloser { return c.stdout }
func (c *sustainedWireChild) Teardown(context.Context) managedprocess.TeardownObservation {
	_ = c.stdin.Close()
	_ = c.input.Close()
	_ = c.output.Close()
	return managedprocess.TeardownObservation{Death: managedprocess.DeathObservation{Kind: managedprocess.DeathExited, Reap: managedprocess.ReapObservation{Kind: managedprocess.ReapComplete}}}
}
func (c *sustainedWireChild) Close() managedprocess.ResourceObservation {
	_ = c.stdout.Close()
	return managedprocess.ResourceObservation{Kind: managedprocess.ResourcesClosed}
}
func (c *sustainedWireChild) requestSnapshot() (int, []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.documentSymbolRequests, append([]string(nil), c.methods...)
}

type sustainedStarter struct{ child sessionruntime.Child }

func (s sustainedStarter) Start(context.Context, managedprocess.Spec) (sessionruntime.Child, managedprocess.StartObservation) {
	return s.child, managedprocess.StartObservation{Kind: managedprocess.StartStarted}
}

func TestCensusSustainedSessionMalformedDocumentSymbolPoisonsAndProjectsLifecycleSupply(t *testing.T) {
	workspace := t.TempDir()
	files := make(censusacquisition.StaticFiles, sustainedFileCount)
	for i := range files {
		name := fmt.Sprintf("f%02d.go", i)
		path := filepath.Join(workspace, name)
		if err := os.WriteFile(path, []byte("package p\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		files[i] = censusacquisition.SourceFile{Path: name, URI: (&url.URL{Scheme: "file", Path: path}).String(), LanguageID: "go"}
	}
	selected, err := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: "test", Workspace: workspace, Profile: "go", EnvironmentReference: "local"})
	if err != nil {
		t.Fatal(err)
	}
	child := newSustainedWireChild()
	manager, err := sessionruntime.New(sessionruntime.Config{Limits: sessionruntime.Limits{MaxSessions: 1, MaxRequests: 1, MaxChildren: 2, MaxCancels: 2, MaxTombstones: 32, MaxObservations: 128}, Starter: sustainedStarter{child: child}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = manager.Shutdown(context.Background()) })
	started := manager.Start(context.Background(), sessionruntime.StartRequest{Profile: runtimeprofile.Resolve(selected), LanguageID: "go"})
	if started.Failure != "" {
		t.Fatal(started)
	}
	if initialized := manager.ObserveInitialization(started.SessionID, started.Generation, true); initialized.Failure != "" || initialized.State != session.Ready {
		t.Fatal(initialized)
	}
	privateRuntime := &privateAcquisitionRuntime{manager: manager}
	runtime := &initializedAcquisitionRuntime{privateAcquisitionRuntime: privateRuntime, sessionID: started.SessionID, generation: started.Generation, requestTimeout: time.Second}
	client := censusProductionDiscoveryClient{productionDiscoveryClient: productionDiscoveryClient{runtime: privateRuntime, sessionID: started.SessionID, generation: started.Generation, timeout: time.Second}}
	discovery, discoverErr := (censusacquisition.DiscoveryAdapter{Workspace: workspace, Files: files, Supplier: censusRuntimeDocumentSupplier{runtime: runtime}, Client: client}).Discover(context.Background(), censusacquisition.SessionIdentity{SessionID: started.SessionID, Generation: started.Generation})
	if discoverErr != nil {
		t.Fatal(discoverErr)
	}

	triggerOrdinal := sustainedMalformedRequest - 1
	if manager.Records()[0].State != session.Poisoned {
		t.Fatalf("ASSERT_SUSTAINED_MANAGER_POISONED records=%+v", manager.Records())
	}
	requestCount, methods := child.requestSnapshot()
	if requestCount != sustainedMalformedRequest {
		t.Fatalf("ASSERT_SUSTAINED_NO_POST_POISON_DOCUMENT_SYMBOL requests=%d methods=%v", requestCount, methods)
	}
	if len(discovery.ProviderFailures) != 1+(sustainedFileCount-sustainedMalformedRequest) {
		t.Fatalf("ASSERT_SUSTAINED_PROVIDER_FAILURE_COUNT got=%d failures=%+v", len(discovery.ProviderFailures), discovery.ProviderFailures)
	}
	trigger := discovery.ProviderFailures[0]
	if trigger.Stage != "DOCUMENT_SYMBOL" || trigger.Code != "REQUEST_FAILED" || trigger.FileOrdinal == nil || *trigger.FileOrdinal != triggerOrdinal {
		t.Fatalf("ASSERT_SUSTAINED_TRIGGER_SEPARATE failure=%+v", trigger)
	}
	for i, failure := range discovery.ProviderFailures[1:] {
		wantOrdinal := sustainedMalformedRequest + i
		if failure.Stage != "DOCUMENT_SUPPLY" || failure.Code != "REQUEST_FAILED" || failure.FileOrdinal == nil || *failure.FileOrdinal != wantOrdinal {
			t.Fatalf("ASSERT_SUSTAINED_PUBLIC_SUPPLY_FAILURE[%d] failure=%+v", i, failure)
		}
	}
	if len(discovery.IncompleteCauses) != sustainedFileCount-sustainedMalformedRequest {
		t.Fatalf("ASSERT_SUSTAINED_PRIVATE_CAUSE_COUNT got=%d causes=%+v", len(discovery.IncompleteCauses), discovery.IncompleteCauses)
	}
	for i, cause := range discovery.IncompleteCauses {
		wantOrdinal := sustainedMalformedRequest + i
		if cause.Scope != "FILE" || cause.Stage != "DOCUMENT_SUPPLY" || cause.Code != "LIFECYCLE_CONFLICT" || cause.LifecycleSubcause != censusacquisition.LifecycleSubcauseSessionNotReady || cause.FileOrdinal == nil || *cause.FileOrdinal != wantOrdinal {
			t.Fatalf("ASSERT_SUSTAINED_PRIVATE_CAUSE[%d] cause=%+v", i, cause)
		}
	}

	request, err := censusrequest.New(censusrequest.RefreshCoordinate{SessionID: started.SessionID, Generation: started.Generation}, censusrequest.SemanticFields{Sources: []string{"."}, DownDepth: 1, MaxNodes: 631, BatchTargets: 63, TimeoutMS: 60000, RequestTimeoutMS: 30000})
	if err != nil {
		t.Fatal(err)
	}
	projectionInput := sustainedProjectionInput(discovery, request)
	projection, err := censusenumerationprojection.Build(projectionInput)
	if err != nil {
		t.Fatalf("ASSERT_SUSTAINED_PROJECTION_BUILD: %v", err)
	}
	raw, err := censusenumerationprojection.EncodeCanonical(projection)
	if err != nil {
		t.Fatalf("ASSERT_SUSTAINED_PROJECTION_ENCODE: %v", err)
	}
	decoded, err := censusenumerationprojection.Decode(raw)
	if err != nil || !reflect.DeepEqual(decoded, projection) {
		t.Fatalf("ASSERT_SUSTAINED_PROJECTION_ROUNDTRIP err=%v", err)
	}
	privateCauses, err := newPrivateCensusDiscoveryCausesForDiscovery(discovery)
	if err != nil || len(privateCauses.Causes) != 1 || privateCauses.Causes[0].Code != "LIFECYCLE_CONFLICT" || privateCauses.Causes[0].Count != uint64(sustainedFileCount-sustainedMalformedRequest) || privateCauses.Causes[0].FirstOrdinal != uint64(sustainedMalformedRequest) {
		t.Fatalf("ASSERT_SUSTAINED_PRIVATE_CAUSE_SUMMARY summary=%+v err=%v", privateCauses, err)
	}
	terminal, err := newPrivateCensusTerminalObservation(discovery, censusenumerationprojection.Receipt{}, nil)
	if err != nil || terminal.Cause != "LIFECYCLE_CONFLICT" || terminal.LifecycleSubcause != "SESSION_NOT_READY" || terminal.EncodingCategory != terminalNotApplicable || terminal.FirstOrdinal != uint64(sustainedMalformedRequest) || terminal.Count != uint64(sustainedFileCount-sustainedMalformedRequest) || terminal.ContiguousSuffix != "TRUE" {
		t.Fatalf("ASSERT_SUSTAINED_TERMINAL_REDUCER terminal=%+v err=%v", terminal, err)
	}
}

func sustainedProjectionInput(discovery censusacquisition.Discovery, request censusrequest.Receipt) censusenumerationprojection.Input {
	files := make([]censusenumerationprojection.File, len(discovery.FileLedger.Entries))
	for i, entry := range discovery.FileLedger.Entries {
		files[i] = censusenumerationprojection.File{Ordinal: entry.Ordinal, Path: entry.Identity, Disposition: entry.Disposition}
	}
	failures := make([]censusenumerationprojection.ProviderFailure, len(discovery.ProviderFailures))
	for i, failure := range discovery.ProviderFailures {
		failures[i] = censusenumerationprojection.ProviderFailure{Stage: failure.Stage, Code: failure.Code, FileOrdinal: cloneInt(failure.FileOrdinal), SymbolOrdinal: cloneInt(failure.SymbolOrdinal)}
	}
	causes := make([]censusenumerationprojection.IncompleteCause, len(discovery.IncompleteCauses))
	for i, cause := range discovery.IncompleteCauses {
		causes[i] = censusenumerationprojection.IncompleteCause{Scope: cause.Scope, Stage: cause.Stage, Code: cause.Code, FileOrdinal: cloneInt(cause.FileOrdinal), SymbolOrdinal: cloneInt(cause.SymbolOrdinal)}
	}
	observations := make([]censusenumerationprojection.Observation, len(discovery.Observations))
	for i, observation := range discovery.Observations {
		state := "UNKNOWN"
		if observation.Known {
			state = "KNOWN"
		}
		observations[i] = censusenumerationprojection.Observation{Dimension: string(observation.Dimension), State: state, Observed: observation.Observed, Limit: observation.Limit, Omitted: observation.Omitted}
	}
	return censusenumerationprojection.Input{SessionID: discovery.Session.SessionID, Generation: discovery.Session.Generation, RequestFingerprint: request.Fingerprint, WorkspaceURI: (&url.URL{Scheme: "file", Path: discovery.Workspace}).String(), Files: files, Observations: observations, ProviderFailures: failures, IncompleteCauses: causes, RequestDownDepth: request.Semantic.DownDepth, RequestUpDepth: request.Semantic.UpDepth, RequestMaxNodes: request.Semantic.MaxNodes, RequestBatchTargets: request.Semantic.BatchTargets, RequestTimeoutMS: request.Semantic.RequestTimeoutMS, TimeoutMS: request.Semantic.TimeoutMS}
}

type cancelingTerminalSupplier struct {
	cancel context.CancelFunc
	err    error
}

func (s cancelingTerminalSupplier) Supply(context.Context, censusacquisition.SourceFile) error {
	s.cancel()
	return s.err
}

func TestDocumentSupplyProviderFailureNormalizationTable(t *testing.T) {
	for _, tc := range []struct {
		name         string
		err          error
		cancel       bool
		wantProvider string
		wantCause    string
		wantSubcause censusacquisition.LifecycleSubcause
	}{
		{name: "session-poisoned", err: censusacquisition.ErrSessionPoisoned, wantProvider: "REQUEST_FAILED", wantCause: "SESSION_POISONED"},
		{name: "lifecycle-session-not-ready", err: &censusacquisition.DocumentSupplyFailure{Cause: censusacquisition.ErrLifecycleConflict, LifecycleSubcause: censusacquisition.LifecycleSubcauseSessionNotReady}, wantProvider: "REQUEST_FAILED", wantCause: "LIFECYCLE_CONFLICT", wantSubcause: censusacquisition.LifecycleSubcauseSessionNotReady},
		{name: "session-unavailable", err: censusacquisition.ErrSessionUnavailable, wantProvider: "UNAVAILABLE", wantCause: "SESSION_UNAVAILABLE"},
		{name: "canceled", err: context.Canceled, cancel: true, wantProvider: "CANCELED", wantCause: "CANCELED"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			workspace := t.TempDir()
			path := filepath.Join(workspace, "a.go")
			if err := os.WriteFile(path, []byte("package p\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			file := censusacquisition.SourceFile{Path: "a.go", URI: (&url.URL{Scheme: "file", Path: path}).String(), LanguageID: "go"}
			ctx := context.Background()
			var supplier censusacquisition.DocumentSupplier = &fakeTerminalSupplier{err: tc.err}
			if tc.cancel {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				supplier = cancelingTerminalSupplier{cancel: cancel, err: tc.err}
			}
			discovery, err := (censusacquisition.DiscoveryAdapter{Workspace: workspace, Files: censusacquisition.StaticFiles{file}, Supplier: supplier, Client: terminalDiscoveryClient{}}).Discover(ctx, censusacquisition.SessionIdentity{SessionID: "s", Generation: 1})
			if err != nil {
				t.Fatal(err)
			}
			if len(discovery.ProviderFailures) != 1 || discovery.ProviderFailures[0].Stage != "DOCUMENT_SUPPLY" || discovery.ProviderFailures[0].Code != tc.wantProvider {
				t.Fatalf("ASSERT_SUPPLY_PROVIDER_NORMALIZATION failure=%+v", discovery.ProviderFailures)
			}
			if len(discovery.IncompleteCauses) != 1 || discovery.IncompleteCauses[0].Code != tc.wantCause || discovery.IncompleteCauses[0].LifecycleSubcause != tc.wantSubcause {
				t.Fatalf("ASSERT_SUPPLY_PRIVATE_CAUSE cause=%+v", discovery.IncompleteCauses)
			}

			request, err := censusrequest.New(censusrequest.RefreshCoordinate{SessionID: "s", Generation: 1}, censusrequest.SemanticFields{Sources: []string{"."}, DownDepth: 1, MaxNodes: 631, BatchTargets: 63, TimeoutMS: 60000, RequestTimeoutMS: 30000})
			if err != nil {
				t.Fatal(err)
			}
			valid := sustainedProjectionInput(discovery, request)
			if _, err := censusenumerationprojection.Build(valid); err != nil {
				t.Fatalf("ASSERT_SUPPLY_PROJECTION_EXACT_PAIR_ACCEPTED private=%s public=%s err=%v", tc.wantCause, tc.wantProvider, err)
			}
			for _, publicCode := range []string{"UNAVAILABLE", "REQUEST_FAILED", "CANCELED"} {
				if publicCode == tc.wantProvider {
					continue
				}
				t.Run("reject-"+publicCode, func(t *testing.T) {
					mismatch := valid
					mismatch.ProviderFailures = append([]censusenumerationprojection.ProviderFailure(nil), valid.ProviderFailures...)
					mismatch.ProviderFailures[0].Code = publicCode
					if _, err := censusenumerationprojection.Build(mismatch); err == nil {
						t.Fatalf("ASSERT_SUPPLY_PROJECTION_MISMATCH_REJECTED private=%s public=%s", tc.wantCause, publicCode)
					}
				})
			}
		})
	}
}

func TestDocumentSupplyProjectionRejectsWrongInRangeOrdinalPair(t *testing.T) {
	zero, one := 0, 1
	input := censusenumerationprojection.Input{
		SessionID:          "s",
		Generation:         1,
		RequestFingerprint: "sha256:" + strings.Repeat("a", 64),
		WorkspaceURI:       "file:///workspace",
		Files: []censusenumerationprojection.File{
			{Ordinal: zero, Path: "a.go", Disposition: "incomplete"},
			{Ordinal: one, Path: "b.go", Disposition: "incomplete"},
		},
		ProviderFailures: []censusenumerationprojection.ProviderFailure{
			{Stage: "DOCUMENT_SUPPLY", Code: "UNAVAILABLE", FileOrdinal: cloneInt(&zero)},
			{Stage: "DOCUMENT_SUPPLY", Code: "REQUEST_FAILED", FileOrdinal: cloneInt(&one)},
		},
		IncompleteCauses: []censusenumerationprojection.IncompleteCause{
			{Scope: "FILE", Stage: "DOCUMENT_SUPPLY", Code: "SESSION_POISONED", FileOrdinal: cloneInt(&zero)},
			{Scope: "FILE", Stage: "DOCUMENT_SUPPLY", Code: "SESSION_UNAVAILABLE", FileOrdinal: cloneInt(&one)},
		},
		RequestMaxNodes:  631,
		RequestTimeoutMS: 30000,
		TimeoutMS:        60000,
	}
	if _, err := censusenumerationprojection.Build(input); err == nil {
		t.Fatal("ASSERT_SUPPLY_PROJECTION_WRONG_IN_RANGE_ORDINAL_REJECTED")
	}
}

func TestDocumentSupplyProjectionRejectsMalformedCompatibilityEntries(t *testing.T) {
	ordinal := 0
	base := censusenumerationprojection.Input{
		SessionID:          "s",
		Generation:         1,
		RequestFingerprint: "sha256:" + strings.Repeat("a", 64),
		WorkspaceURI:       "file:///workspace",
		Files:              []censusenumerationprojection.File{{Ordinal: ordinal, Path: "a.go", Disposition: "incomplete"}},
		ProviderFailures:   []censusenumerationprojection.ProviderFailure{{Stage: "DOCUMENT_SUPPLY", Code: "REQUEST_FAILED", FileOrdinal: cloneInt(&ordinal)}},
		IncompleteCauses:   []censusenumerationprojection.IncompleteCause{{Scope: "FILE", Stage: "DOCUMENT_SUPPLY", Code: "SESSION_POISONED", FileOrdinal: cloneInt(&ordinal)}},
		RequestMaxNodes:    631,
		RequestTimeoutMS:   30000,
		TimeoutMS:          60000,
	}
	for _, tc := range []struct {
		name   string
		mutate func(*censusenumerationprojection.Input)
	}{
		{name: "unknown-public", mutate: func(in *censusenumerationprojection.Input) { in.ProviderFailures[0].Code = "UNKNOWN" }},
		{name: "duplicate-provider", mutate: func(in *censusenumerationprojection.Input) {
			in.ProviderFailures = append(in.ProviderFailures, in.ProviderFailures[0])
		}},
		{name: "missing-provider", mutate: func(in *censusenumerationprojection.Input) { in.ProviderFailures = nil }},
		{name: "duplicate-cause", mutate: func(in *censusenumerationprojection.Input) {
			in.IncompleteCauses = append(in.IncompleteCauses, in.IncompleteCauses[0])
		}},
		{name: "missing-cause", mutate: func(in *censusenumerationprojection.Input) { in.IncompleteCauses = nil }},
		{name: "wrong-provider-ordinal", mutate: func(in *censusenumerationprojection.Input) { wrong := 1; in.ProviderFailures[0].FileOrdinal = &wrong }},
		{name: "wrong-cause-ordinal", mutate: func(in *censusenumerationprojection.Input) { wrong := 1; in.IncompleteCauses[0].FileOrdinal = &wrong }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := base
			input.ProviderFailures = append([]censusenumerationprojection.ProviderFailure(nil), base.ProviderFailures...)
			input.IncompleteCauses = append([]censusenumerationprojection.IncompleteCause(nil), base.IncompleteCauses...)
			tc.mutate(&input)
			if _, err := censusenumerationprojection.Build(input); err == nil {
				t.Fatalf("ASSERT_SUPPLY_PROJECTION_MALFORMED_REJECTED case=%s", tc.name)
			}
		})
	}
}
