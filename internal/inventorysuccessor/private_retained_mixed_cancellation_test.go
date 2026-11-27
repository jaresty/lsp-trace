package inventorysuccessor

import (
	"context"
	"encoding/json"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	cc "lsp-trace/internal/censuscontinuation"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/publication"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/internal/sourceprojection"
	tp "lsp-trace/internal/targetpacket"
	"lsp-trace/sessionruntime"
)

// Literal protocol traffic only. Runtime originals are captured by the real
// discovery operation; no private owned-method state is constructed by this test.
func retainedCancellationPipe(t *testing.T, uri string) *discoveryKernelChildV2 {
	t.Helper()
	input, stdin := io.Pipe()
	stdout, output := io.Pipe()
	child := &discoveryKernelChildV2{input: input, stdin: stdin, stdout: stdout, output: output}
	done := make(chan struct{})
	t.Cleanup(func() { child.Close(); <-done })
	go func() {
		defer close(done)
		reader := lspwire.NewReader(input, lspwire.DefaultLimits())
		writer := lspwire.NewWriter(output, lspwire.DefaultLimits())
		for {
			m, err := reader.Read()
			if err != nil || m.Method == "exit" {
				return
			}
			if len(m.ID) == 0 {
				continue
			}
			result := json.RawMessage(`[]`)
			switch m.Method {
			case "initialize":
				result = json.RawMessage(`{"serverInfo":{"name":"retained-cancellation-pipe","version":"1"},"capabilities":{"positionEncoding":"utf-16","documentSymbolProvider":true,"referencesProvider":true}}`)
			case "shutdown":
				result = json.RawMessage(`null`)
			case "textDocument/documentSymbol":
				result, _ = json.Marshal([]lsp.DocumentSymbol{{Name: "X", Kind: 12, Range: lsp.Range{Start: lsp.Position{Line: 1}, End: lsp.Position{Line: 1, Character: 11}}, SelectionRange: lsp.Range{Start: lsp.Position{Line: 1, Character: 5}, End: lsp.Position{Line: 1, Character: 6}}}})
			case "textDocument/references":
				result, _ = json.Marshal([]lsp.Location{{URI: uri, Range: &lsp.Range{Start: lsp.Position{Line: 1, Character: 5}, End: lsp.Position{Line: 1, Character: 6}}}})
			}
			if writer.Write(lspwire.Message{JSONRPC: lspwire.Version, ID: m.ID, Result: result}) != nil {
				return
			}
		}
	}()
	return child
}

func retainedCancellationFixture(t *testing.T) (*PrivateManagedInputOwnerV2, PrivateMixedInputRequestV2, PrivateSelectedConfigurationV2) {
	t.Helper()
	workspace, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(workspace, "x.go")
	if err = os.WriteFile(path, []byte("package x\nfunc X() {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	uri := (&url.URL{Scheme: "file", Path: path}).String()
	selected, err := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: "test", Workspace: workspace, Profile: "go", EnvironmentReference: "offline"})
	if err != nil {
		t.Fatal(err)
	}
	child := retainedCancellationPipe(t, uri)
	c := sessionruntime.PrivateDiscoveryConstruction{CaptureOwnedMethods: true, NewStarter: func() (sessionruntime.Starter, error) { return discoveryKernelStarterV2{child}, nil }, Start: sessionruntime.StartRequest{Profile: runtimeprofile.Resolve(selected), LanguageID: "go"}, Deadline: time.Now().Add(10 * time.Second), Limits: sessionruntime.PrivateDiscoveryLimits{SourceBytes: 16 * (sessionruntime.MaxDocumentSupplyBytes + 1), RequestBytes: 32 << 20, ResponseBytes: 16 << 20, WireBytes: 48 << 20, RetainedBytes: 512 << 20, Work: 1 << 32, Messages: 256, Requests: 128, FrameBytes: 1 << 20, HeaderBytes: 8192}, Members: []sessionruntime.PrivateDiscoveryMember{{Path: "x.go", URI: uri, LanguageID: "go"}}}
	discovery, _, binding := discoverySyntheticConfigurationV2(t, -1, &c)
	acquisition, err := discovery.ReservePartition(context.Background(), binding, hashLabelV2("retained-cancellation-discovery"))
	if err != nil {
		t.Fatal("discovery reserve", err)
	}
	err = acquisition.Execute(context.Background(), c, func(ctx context.Context, s *sessionruntime.PrivateDiscoverySession) error {
		if err := s.Supply(ctx, uri, "go"); err != nil {
			return err
		}
		if _, err := s.DocumentSymbols(ctx, lsp.DocumentSymbolParams{TextDocument: lsp.TextDocumentIdentifier{URI: uri}}); err != nil {
			return err
		}
		_, err := s.References(ctx, uri, lsp.Position{Line: 1, Character: 5})
		return err
	})
	if err != nil {
		t.Fatal("discovery execute", err)
	}
	if err = acquisition.Complete(context.Background()); err != nil {
		t.Fatal("discovery complete", err)
	}
	owned := acquisition.OwnedMethods()
	index := -1
	for i := 0; i < owned.GroupingQueryCount(); i++ {
		q, ok := owned.GroupingQuery(i)
		if ok && q.Method == "textDocument/references" {
			index = i
		}
	}
	if index < 0 {
		t.Fatal("no captured reference query")
	}
	dir := t.TempDir()
	if err = os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	root, err := publication.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { root.Close() })
	pin := hashLabelV2("retained-test-contract")
	in := cc.ContractInput{ContinuationID: pin, ProfileID: pin, WorkerPinID: pin, CaptureLimits: cc.ProductionCaptureLimits(), PacketSourcePolicy: sourceprojection.Policy{PolicyID: "CONTROLLED_BOUNDED_CONTRACT"}, PacketMaxResponseBytes: 1 << 20, DescribeDeadlineMS: 1000, RetryPolicy: cc.RetryPolicy{MaxAttemptsPerRequest: 1}}
	contract, err := cc.NewContinuationContract(in)
	if err != nil {
		t.Fatal(err)
	}
	config := PrivateSelectedConfigurationV2{RunID: "retained-cancellation", SeedNamespace: "retained-cancellation", Lineage: "retained-cancellation", Mode: "R", Seed: 7, Attempts: [3]string{hashLabelV2("retained-M"), hashLabelV2("retained-P"), hashLabelV2("retained-S")}, Contract: contract, ContractInput: in, SourceLimits: tp.PrivateSourceLimits{MaxDocumentBytes: 1 << 20, MaxTotalBytes: 8 << 20, MaxObjects: 1000, MaxWork: 10000}}
	owner, q, err := OpenLocalFolderSelectedV1(context.Background(), root, acquisition, index, PrivateRetainedNativeCallsV1{}, config)
	if err != nil {
		t.Fatal("selected open", err)
	}
	if !q.Start.Deadline.Equal(c.Deadline) {
		t.Fatal("ASSERT_RETAINED_ORIGINAL_DEADLINE")
	}
	return owner, q, config
}

// Armed only after actual reservation. Err is called under the operation lock;
// acquiring that lock after this signal proves the entry check has finished.
type retainedEntryContext struct {
	context.Context
	armed   atomic.Bool
	once    sync.Once
	checked chan struct{}
}

func (c *retainedEntryContext) Err() error {
	err := c.Context.Err()
	if c.armed.Load() {
		c.once.Do(func() { close(c.checked) })
	}
	return err
}

func TestRetainedMixedDirectCancellation(t *testing.T) {
	for _, mode := range []string{"reservation", "caller", "deadline", "success"} {
		t.Run(mode, func(t *testing.T) {
			owner, q, config := retainedCancellationFixture(t)
			reservation, cancelReservation := context.WithCancel(context.Background())
			defer cancelReservation()
			entry := &retainedEntryContext{Context: reservation, checked: make(chan struct{})}
			plan, err := NewPrivateSelectedPreparationPlanV2(owner, q, config)
			if err != nil {
				t.Fatal("plan", err)
			}
			op, err := owner.ReserveSelectedMixedInputV2(entry, config.Attempts[0], q, plan)
			if err != nil {
				t.Fatal("reserve", err)
			}
			before, err := owner.k.reopen(context.Background(), owner.job, owner.limits)
			if err != nil {
				t.Fatal(err)
			}
			caller, cancelCaller := context.WithCancel(context.Background())
			defer cancelCaller()
			owner.mu.Lock()
			entry.armed.Store(true)
			type result struct {
				input PrivateManagedGroupingInput
				err   error
			}
			done := make(chan result, 1)
			go func() { input, err := ExecuteMixedInputV2(caller, op, q); done <- result{input, err} }()
			select {
			case <-entry.checked:
			case <-time.After(3 * time.Second):
				owner.mu.Unlock()
				t.Fatal("entry barrier")
			}
			op.state.mu.Lock()
			used := op.state.used
			op.state.mu.Unlock()
			if !used {
				owner.mu.Unlock()
				t.Fatal("entry not retired")
			}
			// The owner lock holds execution before the first kernel reopen, after the
			// direct exported route's initial check. The caller is independent and live.
			if mode == "reservation" {
				cancelReservation()
			}
			if mode == "caller" {
				cancelCaller()
			}
			if mode == "deadline" {
				// Wait out the actual acquisition cutoff, not a replacement test
				// deadline. Both explicit cancellation owners remain live.
				<-time.After(time.Until(q.Start.Deadline))
			}
			owner.mu.Unlock()
			var got result
			select {
			case got = <-done:
			case <-time.After(5 * time.Second):
				t.Fatal("execution did not finish")
			}
			after, err := owner.k.reopen(context.Background(), owner.job, owner.limits)
			if err != nil {
				t.Fatal(err)
			}
			if mode == "success" {
				if got.err != nil {
					t.Fatal("ASSERT_RETAINED_DIRECT_SUCCESS", got.err)
				}
				if got.input.retainedCompletion == nil || after.Disposition != "OBSERVED" {
					t.Fatal("ASSERT_RETAINED_DIRECT_SUCCESS: no completion/readback")
				}
				if _, err = got.input.SourceOriginalsV1(); err != nil {
					t.Fatal("ASSERT_RETAINED_DIRECT_SUCCESS: originals", err)
				}
				t.Log("ASSERT_RETAINED_DIRECT_SUCCESS: PASS")
			} else {
				if got.err == nil || got.input.retainedCompletion != nil || after.Disposition == "OBSERVED" {
					t.Fatal("ASSERT_RETAINED_POST_ENTRY_CANCEL: completion escaped", mode, got.err, after.Disposition)
				}
				if _, err = got.input.SourceOriginalsV1(); err == nil {
					t.Fatal("ASSERT_RETAINED_POST_ENTRY_CANCEL: originals escaped")
				}
				t.Log("ASSERT_RETAINED_POST_ENTRY_CANCEL: PASS", mode)
			}
			if after.Charged != before.Charged || after.Windows != before.Windows {
				t.Fatal("ASSERT_RETAINED_NO_REFUND")
			}
			if _, err = ExecuteMixedInputV2(context.Background(), op, q); err == nil {
				t.Fatal("ASSERT_RETAINED_SINGLE_USE")
			}
			t.Log("ASSERT_RETAINED_SINGLE_USE: PASS; ASSERT_RETAINED_NO_REFUND: PASS")
		})
	}
}
