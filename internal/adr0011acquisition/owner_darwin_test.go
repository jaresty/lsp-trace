//go:build darwin

package adr0011acquisition

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/publication"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/sessionruntime"
)

func TestADR0011AcquisitionPeer(t *testing.T) {
	if os.Getenv("ADR0011_ACQUISITION_PEER") != "1" {
		return
	}
	reader := lspwire.NewReader(os.Stdin, lspwire.DefaultLimits())
	writer := lspwire.NewWriter(os.Stdout, lspwire.DefaultLimits())
	for {
		msg, err := reader.Read()
		if err != nil {
			os.Exit(2)
		}
		var result json.RawMessage
		switch msg.Method {
		case "initialize":
			result = []byte(`{"capabilities":{"positionEncoding":"utf-16","documentSymbolProvider":true,"referencesProvider":true},"serverInfo":{"name":"synthetic","version":"1"}}`)
		case "initialized":
			continue
		case "textDocument/documentSymbol":
			result = []byte(`[{"name":"Query","kind":12,"range":{"start":{"line":1,"character":0},"end":{"line":1,"character":15}},"selectionRange":{"start":{"line":1,"character":5},"end":{"line":1,"character":10}}}]`)
		case "textDocument/references":
			result = []byte(`[{"uri":"file:///ref.go","range":{"start":{"line":1,"character":0},"end":{"line":1,"character":1}}},{"uri":"file:///ref.go","range":{"start":{"line":1,"character":0},"end":{"line":1,"character":1}}}]`)
		case "shutdown":
			result = []byte(`null`)
		case "exit":
			os.Exit(0)
		default:
			if len(msg.ID) == 0 {
				continue
			}
			os.Exit(3)
		}
		response := lspwire.Message{JSONRPC: lspwire.Version, ID: msg.ID, Result: result}
		if msg.Method == "textDocument/references" && os.Getenv("ADR0011_PEER_AT_LIMIT_FRAME") == "1" {
			body, marshalErr := json.Marshal(response)
			if marshalErr != nil || len(body) < 2 || body[len(body)-1] != '}' {
				os.Exit(4)
			}
			const frameSize = 2 << 20
			bodySize := frameSize - len(fmt.Sprintf("Content-Length: %d\r\n\r\n", frameSize))
			for {
				headerSize := len(fmt.Sprintf("Content-Length: %d\r\n\r\n", bodySize))
				if bodySize+headerSize == frameSize {
					break
				}
				bodySize = frameSize - headerSize
			}
			if bodySize < len(body) {
				os.Exit(4)
			}
			body = append(append(append([]byte(nil), body[:len(body)-1]...), bytes.Repeat([]byte(" "), bodySize-len(body))...), '}')
			frame := append([]byte(fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body))), body...)
			if len(frame) != frameSize {
				os.Exit(4)
			}
			for len(frame) != 0 {
				n, writeErr := os.Stdout.Write(frame)
				if writeErr != nil || n <= 0 {
					os.Exit(4)
				}
				frame = frame[n:]
			}
			continue
		}
		if err = writer.Write(response); err != nil {
			os.Exit(4)
		}
	}
}

func TestADR0011OwnerManagedTwoEqualLocations(t *testing.T) {
	testADR0011OwnerManagedTwoEqualLocations(t, true)
}

func TestADR0011OwnerPrivateFinalTwoEqualLocations(t *testing.T) {
	testADR0011OwnerManagedTwoEqualLocations(t, false)
}

func TestADR0011OwnerPrivateFinalAtLimitFrame(t *testing.T) {
	t.Setenv("ADR0011_PEER_AT_LIMIT_FRAME", "1")
	testADR0011OwnerManagedTwoEqualLocations(t, false)
}

func testADR0011OwnerManagedTwoEqualLocations(t *testing.T, legacy bool) {
	workspace, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(workspace, "query.go")
	if err := os.WriteFile(file, []byte("package fixture\nfunc Query() {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = workspace
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("fixture git %v: %v %s", args, err, out)
		}
		return string(out)
	}
	git("init", "-q")
	git("-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "add", "query.go")
	git("-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "fixture")
	commit := git("rev-parse", "HEAD")
	uri := (&url.URL{Scheme: "file", Path: filepath.ToSlash(file)}).String()
	supervisor, err := managedprocess.NewLocalDarwinSupervisor(managedprocess.Options{StderrLimit: 4096, GracePeriod: 100 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	manager, err := sessionruntime.New(sessionruntime.Config{Limits: sessionruntime.Limits{MaxSessions: 1, MaxRequests: 2, MaxChildren: 1, MaxCancels: 2, MaxTombstones: 4, MaxObservations: 16}, Starter: sessionruntime.ManagedStarter{Manager: supervisor}})
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Shutdown(context.Background())
	validated, err := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: "test", Workspace: workspace, Profile: "gopls", EnvironmentReference: "local"})
	if err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	started := manager.Start(context.Background(), sessionruntime.StartRequest{Profile: runtimeprofile.Resolve(validated), LanguageID: "go", Process: managedprocess.Spec{Path: executable, Args: []string{"-test.run=^TestADR0011AcquisitionPeer$"}, Env: append(os.Environ(), "ADR0011_ACQUISITION_PEER=1")}})
	if started.Failure != "" {
		t.Fatalf("fixture start: %+v", started)
	}
	pending := manager.BeginReadiness(context.Background(), started.SessionID, started.Generation, time.Now().Add(10*time.Second))
	ready, found := manager.WaitReadiness(context.Background(), pending.ID)
	if !found || ready.State != sessionruntime.ReadinessReady || ready.Failure != "" {
		t.Fatalf("fixture readiness: %+v found=%v", ready, found)
	}
	dir := filepath.Join(t.TempDir(), "root")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	root, err := publication.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	canonical, err := filepath.EvalSymlinks(workspace)
	if err != nil {
		t.Fatal(err)
	}
	if observed, probeErr := observeHostGit(context.Background(), canonical, strings.TrimSpace(commit)); probeErr != nil {
		t.Fatalf("ASSERT_ADR0011_HOST_GIT_CLEAN_PROBE: err=%v observation=%+v", probeErr, observed)
	}
	prepared := manager.PrepareDocument(context.Background(), sessionruntime.DocumentRequest{SessionID: started.SessionID, Generation: started.Generation, URI: uri, LanguageID: "go", CaptureSupply: true, PreparedNoFollow: true})
	if prepared.Failure != "" || prepared.Supply == nil {
		t.Fatalf("preinvoke prepare: %+v", prepared)
	}
	selected, err := selectGitExecutable("")
	if err != nil {
		t.Fatal(err)
	}
	facts := preinvokeFacts{SessionID: started.SessionID, Generation: started.Generation, Workspace: canonical, URI: uri, Line: 1, Character: 6, Version: prepared.Version, Source: append([]byte(nil), prepared.Supply.Content...), SourceDigest: privateDigest(prepared.Supply.Content), SourceLength: len(prepared.Supply.Content), GitRoot: canonical, GitCommit: strings.TrimSpace(commit), Executable: selected}
	repeated := manager.PrepareDocument(context.Background(), sessionruntime.DocumentRequest{SessionID: started.SessionID, Generation: started.Generation, URI: uri, LanguageID: "go", CaptureSupply: true, PreparedNoFollow: true})
	if repeated.Failure != "" || repeated.Version != prepared.Version || repeated.Supply == nil || privateDigest(repeated.Supply.Content) != facts.SourceDigest {
		t.Fatalf("ASSERT_ADR0011_PREPARE_REPEAT_STABLE: first=%d second=%d failure=%s", prepared.Version, repeated.Version, repeated.Failure)
	}
	selector, digest, occurrenceID, err := publishPreinvokeOccurrence(root, facts)
	if err != nil {
		t.Fatal(err)
	}
	owner := newTestOwner(manager, root)
	owner.preinvokeSelector, owner.preinvokeDigest = selector, digest
	owner.captureFrames = true
	keyGuardReached := false
	ownerReadReached := false
	rawPayloadReached := false
	targetResultReached := false
	targetRecordReached := false
	responseReadReached := false
	rawRecordReached := false
	scannerRecordReached := false
	eventsRecordReached := false
	policiesReached := false
	methodRecordReached := false
	revisionReached := false
	for name, change := range map[string]func(*preinvokeFacts){
		"session":       func(f *preinvokeFacts) { f.SessionID += "x" },
		"generation":    func(f *preinvokeFacts) { f.Generation++ },
		"workspace":     func(f *preinvokeFacts) { f.Workspace += "x" },
		"uri":           func(f *preinvokeFacts) { f.URI += "x" },
		"line":          func(f *preinvokeFacts) { f.Line++ },
		"character":     func(f *preinvokeFacts) { f.Character++ },
		"version":       func(f *preinvokeFacts) { f.Version++ },
		"source":        func(f *preinvokeFacts) { f.Source[0] ^= 1 },
		"source-digest": func(f *preinvokeFacts) { f.SourceDigest += "x" },
		"source-length": func(f *preinvokeFacts) { f.SourceLength++ },
		"git-root":      func(f *preinvokeFacts) { f.GitRoot += "x" },
		"commit":        func(f *preinvokeFacts) { f.GitCommit += "x" },
		"executable":    func(f *preinvokeFacts) { f.Executable.digest += "x" },
	} {
		t.Run("preinvoke-"+name, func(t *testing.T) {
			altered := facts
			altered.Source = append([]byte(nil), facts.Source...)
			change(&altered)
			if verifyPreinvokeOccurrence(root, selector, digest, altered, occurrenceID) {
				t.Fatal("substituted preinvoke facts admitted")
			}
		})
	}
	if verifyPreinvokeOccurrence(root, selector, digest, facts, "sha256:"+strings.Repeat("1", 64)) || verifyPreinvokeOccurrence(root, "", digest, facts, occurrenceID) || verifyPreinvokeOccurrence(root, selector, "sha256:"+strings.Repeat("0", 64), facts, occurrenceID) {
		t.Fatal("missing or changed receipt admitted")
	}
	methodCalls := 0
	owner.beforeMethodRequestTestHook = func() { methodCalls++ }
	owner.afterKeyGuardTestHook = func() { keyGuardReached = true }
	owner.afterOwnerReadTestHook = func() { ownerReadReached = true }
	owner.afterRawPayloadTestHook = func() { rawPayloadReached = true }
	owner.afterTargetResultTestHook = func() { targetResultReached = true }
	owner.afterTargetRecordTestHook = func() { targetRecordReached = true }
	owner.afterResponseReadTestHook = func() { responseReadReached = true }
	owner.afterRawRecordTestHook = func() { rawRecordReached = true }
	owner.afterScannerRecordTestHook = func() { scannerRecordReached = true }
	owner.afterEventsRecordTestHook = func() { eventsRecordReached = true }
	owner.afterPoliciesTestHook = func() { policiesReached = true }
	owner.afterMethodRecordTestHook = func() { methodRecordReached = true }
	owner.afterRevisionVerifiedTestHook = func() { revisionReached = true }
	missing := newTestOwner(manager, root)
	missing.beforeMethodRequestTestHook = func() { methodCalls++ }
	validQuery := Query{SessionID: started.SessionID, Generation: started.Generation, URI: uri, Line: 1, Character: 6, OccurrenceID: occurrenceID, Revision: strings.TrimSpace(commit)}
	if got, e := missing.Acquire(context.Background(), validQuery); got != nil || e != ErrAcquisition || methodCalls != 0 {
		t.Fatalf("ASSERT_ADR0011_PREINVOKE_MISSING_NO_CALL: receipt=%+v err=%v calls=%d", got, e, methodCalls)
	}
	for _, bad := range []Query{
		{SessionID: started.SessionID, Generation: started.Generation, URI: uri, Line: 1, Character: 6, OccurrenceID: "sha256:" + strings.Repeat("1", 64), Revision: strings.TrimSpace(commit)},
		{SessionID: started.SessionID, Generation: started.Generation, URI: uri, Line: 1, Character: 7, OccurrenceID: occurrenceID, Revision: strings.TrimSpace(commit)},
	} {
		if got, e := owner.Acquire(context.Background(), bad); got != nil || e != ErrAcquisition || methodCalls != 0 {
			t.Fatalf("ASSERT_ADR0011_PREINVOKE_NO_CALL: receipt=%+v err=%v", got, e)
		}
	}
	if stale, staleErr := owner.Acquire(context.Background(), Query{SessionID: started.SessionID, Generation: started.Generation + 1, URI: uri, Line: 1, Character: 6, OccurrenceID: occurrenceID, Revision: strings.TrimSpace(commit)}); stale != nil || staleErr != ErrAcquisition || keyGuardReached || ownerReadReached || rawPayloadReached || targetResultReached || targetRecordReached {
		t.Fatalf("ASSERT_ADR0011_SOURCE_GENERATION_SUBSTITUTION: receipt=%+v err=%v", stale, staleErr)
	}
	// The private path uses the same managed fixture, but its independent pin is
	// test-only synthetic evidence, explicitly not reviewed implementation source.
	if private, e := owner.acquirePrivateFinal(context.Background(), validQuery); private != nil || e != ErrAcquisition {
		t.Fatalf("ASSERT_PRIVATE_FINAL_MISSING_PIN: receipt=%+v err=%v", private, e)
	}
	owner.finalImplementationPin = reviewedSyntheticSourcePin
	private, privateErr := owner.acquirePrivateFinal(context.Background(), validQuery)
	if privateErr != nil || private == nil || private.T != 1 || private.A != 2 || len(private.OrdinalRefs) != 2 || private.OrdinalRefs[0] == private.OrdinalRefs[1] || private.FinalRef.Selector == "" {
		t.Fatalf("ASSERT_PRIVATE_FINAL_VERIFIED_ORDINALS: receipt=%+v err=%v", private, privateErr)
	}
	if os.Getenv("ADR0011_PEER_AT_LIMIT_FRAME") == "1" {
		seenFrame := false
		readPaths, _ := filepath.Glob(filepath.Join(root.Path(), "adr0011-references-read-v1-*.bin"))
		for _, path := range readPaths {
			body, readErr := publication.ReadVerifiedBoundFile(root, filepath.Base(path), 2<<20)
			if readErr != nil {
				t.Fatalf("ASSERT_PRIVATE_AT_LIMIT_BODY_READBACK: %v", readErr)
			}
			frameBytes := len(body) + len(fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body)))
			if frameBytes == 2<<20 {
				seenFrame = true
			}
		}
		if !seenFrame {
			t.Fatalf("ASSERT_PRIVATE_AT_LIMIT_FRAME_MISSING: read_bodies=%d", len(readPaths))
		}
		for _, kind := range []string{"proposal", "candidate"} {
			paths, _ := filepath.Glob(filepath.Join(root.Path(), "adr0011-references-issuance-v1-"+kind+"-*.json"))
			if len(paths) != 1 {
				t.Fatalf("ASSERT_PRIVATE_AT_LIMIT_%s_COUNT: %d", kind, len(paths))
			}
			body, readErr := publication.ReadVerifiedBoundFile(root, filepath.Base(paths[0]), sourceRecordLimit)
			if readErr != nil || len(body) == 0 {
				t.Fatalf("ASSERT_PRIVATE_AT_LIMIT_%s_READBACK: %v", kind, readErr)
			}
		}
		finalBytes, readErr := publication.ReadVerifiedBoundFile(root, private.FinalRef.Selector, sourceRecordLimit)
		var counts struct{ T, A, P int }
		if readErr != nil || json.Unmarshal(finalBytes, &counts) != nil || counts.T != 1 || counts.A != 2 || counts.P != 2 {
			t.Fatalf("ASSERT_PRIVATE_AT_LIMIT_FINAL_READBACK: counts=%+v err=%v", counts, readErr)
		}
	}
	newPrivateOwner := func(t *testing.T) (*Owner, *publication.Root) {
		t.Helper()
		dir := filepath.Join(t.TempDir(), "root")
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
		r, err := publication.OpenRoot(dir)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { r.Close() })
		s, d, id, err := publishPreinvokeOccurrence(r, facts)
		if err != nil || id != occurrenceID {
			t.Fatalf("private declaration: %v %s", err, id)
		}
		o := newTestOwner(manager, r)
		o.captureFrames = true
		o.preinvokeSelector, o.preinvokeDigest = s, d
		return o, r
	}
	wrongOwner, _ := newPrivateOwner(t)
	wrongOwner.finalImplementationPin = func() (string, error) { return "bad", nil }
	if private, e := wrongOwner.acquirePrivateFinal(context.Background(), validQuery); private != nil || e != ErrAcquisition {
		t.Fatalf("ASSERT_PRIVATE_FINAL_WRONG_PIN: receipt=%+v err=%v", private, e)
	}
	owner.finalImplementationPin = reviewedSyntheticSourcePin
	changed := validQuery
	changed.Character++
	callsBefore := methodCalls
	if private, e := owner.acquirePrivateFinal(context.Background(), changed); private != nil || e != ErrAcquisition || methodCalls != callsBefore {
		t.Fatalf("ASSERT_PRIVATE_FINAL_PREINVOKE_SUBSTITUTION: receipt=%+v err=%v calls=%d", private, e, methodCalls-callsBefore)
	}
	preOwner, preRoot := newPrivateOwner(t)
	pinCalls := 0
	preOwner.finalImplementationPin = func() (string, error) {
		pinCalls++
		if pinCalls == 1 {
			return "sha256:" + strings.Repeat("c", 64), nil
		}
		return "sha256:" + strings.Repeat("d", 64), nil
	}
	if preReceipt, e := preOwner.acquirePrivateFinal(context.Background(), validQuery); preReceipt != nil || e != ErrAcquisition || pinCalls < 2 {
		t.Fatalf("ASSERT_PRIVATE_FINAL_PRECOMMIT_PIN_CHANGE: receipt=%+v err=%v calls=%d", preReceipt, e, pinCalls)
	}
	if paths, _ := filepath.Glob(filepath.Join(preRoot.Path(), "adr0011-references-issuance-v1-final-*.json")); len(paths) != 0 {
		t.Fatalf("ASSERT_PRIVATE_FINAL_PRECOMMIT_NO_FINAL: %v", paths)
	}
	postOwner, postRoot := newPrivateOwner(t)
	postOwner.finalImplementationPin = func() (string, error) { return "sha256:" + strings.Repeat("b", 64), nil }
	removed := false
	postOwner.finalTraceTestHook = func(event publication.BoundFileTraceEvent) {
		if !removed && event.Stage == "HARDLINK" && event.Result == "INSTALLED" {
			paths, _ := filepath.Glob(filepath.Join(postRoot.Path(), "adr0011-references-issuance-v1-final-*.json"))
			if len(paths) == 1 {
				removed = os.Remove(paths[0]) == nil
			}
		}
	}
	if postReceipt, e := postOwner.acquirePrivateFinal(context.Background(), validQuery); postReceipt != nil || e != ErrAcquisition || !removed {
		t.Fatalf("ASSERT_PRIVATE_FINAL_POSTCOMMIT_NO_ISSUANCE: receipt=%+v err=%v removed=%v", postReceipt, e, removed)
	}
	if !legacy {
		return
	}
	receipt, err := owner.Acquire(context.Background(), validQuery)
	if !rawPayloadReached {
		t.Fatalf("ASSERT_ADR0011_OWNER_RAW_PAYLOAD_CHECKPOINT: owner_read_reached=%t err=%v", ownerReadReached, err)
	}
	if !revisionReached {
		t.Fatalf("ASSERT_ADR0011_REVISION_CHECKPOINT_BEFORE_ISSUANCE: err=%v", err)
	}
	if !targetResultReached {
		t.Fatalf("ASSERT_ADR0011_TARGET_RESULT_CHECKPOINT: raw_reached=%t err=%v", rawPayloadReached, err)
	}
	if !targetRecordReached {
		t.Fatalf("ASSERT_ADR0011_TARGET_RECORD_CHECKPOINT: result_reached=%t err=%v", targetResultReached, err)
	}
	if !responseReadReached || !rawRecordReached || !scannerRecordReached || !eventsRecordReached || !policiesReached || !methodRecordReached {
		t.Fatalf("ASSERT_ADR0011_RAW_RECORD_CHECKPOINT: response=%t raw=%t scanner=%t events=%t policies=%t method=%t err=%v", responseReadReached, rawRecordReached, scannerRecordReached, eventsRecordReached, policiesReached, methodRecordReached, err)
	}
	if err != ErrAcquisition || receipt != nil {
		t.Fatalf("ASSERT_ADR0011_OWNER_PUBLIC_ISSUANCE_FAILS_CLOSED: receipt=%+v err=%v key_guard_reached=%t owner_read_reached=%t routing=%+v", receipt, err, keyGuardReached, ownerReadReached, manager.Records())
	}
	newRoot := func() *publication.Root {
		t.Helper()
		path := filepath.Join(t.TempDir(), "root")
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
		r, err := publication.OpenRoot(path)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = r.Close() })
		return r
	}
	t.Run("dirty-before", func(t *testing.T) {
		dirty := filepath.Join(workspace, "untracked.txt")
		if err := os.WriteFile(dirty, []byte("dirty"), 0600); err != nil {
			t.Fatal(err)
		}
		defer os.Remove(dirty)
		issued, err := newTestOwner(manager, newRoot()).Acquire(context.Background(), Query{SessionID: started.SessionID, Generation: started.Generation, URI: uri, Line: 1, Character: 6, OccurrenceID: "dirty", Revision: strings.TrimSpace(commit)})
		if err == nil || issued != nil {
			t.Fatalf("ASSERT_ADR0011_HOST_GIT_DIRTY_PRE_ZERO: receipt=%+v err=%v", issued, err)
		}
	})
	t.Run("revision-before", func(t *testing.T) {
		issued, err := newTestOwner(manager, newRoot()).Acquire(context.Background(), Query{SessionID: started.SessionID, Generation: started.Generation, URI: uri, Line: 1, Character: 6, OccurrenceID: "revision", Revision: strings.Repeat("f", 40)})
		if err == nil || issued != nil {
			t.Fatalf("ASSERT_ADR0011_HOST_GIT_REVISION_PRE_ZERO: receipt=%+v err=%v", issued, err)
		}
	})
	t.Run("dirty-after", func(t *testing.T) {
		dirty := filepath.Join(workspace, "after.txt")
		fixtureRoot := newRoot()
		s, d, id, e := publishPreinvokeOccurrence(fixtureRoot, facts)
		if e != nil {
			t.Fatal(e)
		}
		owner := newTestOwner(manager, fixtureRoot)
		owner.preinvokeSelector, owner.preinvokeDigest = s, d
		owner.captureFrames = true
		owner.afterCaptureTestHook = func() {
			if err := os.WriteFile(dirty, []byte("late"), 0600); err != nil {
				t.Error(err)
			}
		}
		defer os.Remove(dirty)
		issued, err := owner.Acquire(context.Background(), Query{SessionID: started.SessionID, Generation: started.Generation, URI: uri, Line: 1, Character: 6, OccurrenceID: id, Revision: strings.TrimSpace(commit)})
		if err == nil || issued != nil {
			t.Fatalf("ASSERT_ADR0011_HOST_GIT_DIRTY_POST_ZERO: receipt=%+v err=%v", issued, err)
		}
	})
	t.Run("commit-after", func(t *testing.T) {
		fixtureRoot := newRoot()
		s, d, id, e := publishPreinvokeOccurrence(fixtureRoot, facts)
		if e != nil {
			t.Fatal(e)
		}
		owner := newTestOwner(manager, fixtureRoot)
		owner.preinvokeSelector, owner.preinvokeDigest = s, d
		owner.captureFrames = true
		owner.afterCaptureTestHook = func() {
			git("-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "--allow-empty", "-qm", "late")
		}
		issued, err := owner.Acquire(context.Background(), Query{SessionID: started.SessionID, Generation: started.Generation, URI: uri, Line: 1, Character: 6, OccurrenceID: id, Revision: strings.TrimSpace(commit)})
		if err == nil || issued != nil {
			t.Fatalf("ASSERT_ADR0011_HOST_GIT_COMMIT_POST_ZERO: receipt=%+v err=%v", issued, err)
		}
	})
}
