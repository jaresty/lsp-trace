//go:build darwin || linux

package inventorysuccessor_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	cc "lsp-trace/internal/censuscontinuation"
	"lsp-trace/internal/describerequest"
	"lsp-trace/internal/describeworker"
	inv "lsp-trace/internal/inventorysuccessor"
	sc "lsp-trace/internal/inventorysuccessor/sourcecontinuation"
	so "lsp-trace/internal/inventorysuccessor/sourceoperator"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/lspwire"
	host "lsp-trace/internal/privatedescribeprep"
	pc "lsp-trace/internal/programc"
	"lsp-trace/internal/programcadmission"
	"lsp-trace/internal/publication"
	rg "lsp-trace/internal/referencegrouping"
	"lsp-trace/internal/retainedprojection"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/internal/sourceobject"
	"lsp-trace/internal/sourceprojection"
	tp "lsp-trace/internal/targetpacket"
	"lsp-trace/internal/v5sourcesnapshotv3"
	"lsp-trace/sessionruntime"
)

// Source acquisition subprocesses use only this controlled existing Go test binary.
// TestSuppliedHost additionally runs the existing Python byte publisher, no operator.
// The only acquisition subprocess is this controlled existing Go test binary. References /
// definitions deliberately return a real observed symbol selection coordinate.
// Foreign/snippet modes exercise source gaps, not alternate source authority.
// TestSuppliedHost keeps the original seven-mode source fixture and 8 MiB /
// 1 MiB contract. Only ONE admitted request crosses the actual file mailbox.
func TestSuppliedHost(t *testing.T) {
	input, h, contract, q := sourceStageFixture(t, "positive", false)
	inv.ControlledSourceProjectionNoReadmitForTest(t, input)
	ctx, cancel := context.WithDeadline(context.Background(), q.Deadline)
	defer cancel()
	store, e := sc.NewRootlessSourceHostStore(8 << 20)
	if e != nil {
		t.Fatal(e)
	}
	pause := sc.Run(ctx, sc.Request{PrivateAllMode: &h, PrivateSources: &q, Workspace: qWorkspace(q), Contract: contract, Store: store, StopAfter: sc.StopAfterPrivateSourceRequests})
	if pause.Err != nil || pause.PrivateSources == nil {
		t.Fatal("ASSERT_HOST_SOURCE_SETUP", pause.Err)
	}
	out := pause.PrivateSources
	if out.Counts.Admitted != 32 || out.Counts.Unresolved != 24 || out.Counts.Requests != 2 || out.Counts.Intents != 56 {
		t.Fatal("ASSERT_HOST_ALL7", out.Counts)
	}
	r := out.Requests[0]
	resume := sc.ResumeRequest{PrivateSources: &sc.PrivateAllModeSourcesResume{Version: sc.PrivateAllModeSourcesVersion, Expected: out.Expected, Originals: q.Originals, Definitions: q.Definitions, ExpectedHandoff: h}, Workspace: qWorkspace(q), Store: store, StopAfter: sc.StopAfterPrivateSourceRequests, Selector: out.CheckpointID}
	request := sc.SourceAcquisitionRequest{Resume: resume, Record: r, Attempt: sc.SuppliedSourceAttempt{ID: "controlled-host-file-attempt", CheckpointID: out.CheckpointID, SourceUnitID: r.Envelope.AdmissionID, RequestRecordID: r.RecordID}}
	home, e := os.UserHomeDir()
	if e != nil {
		t.Fatal(e)
	}
	root, e := os.MkdirTemp(home, ".source-host-upper-")
	if e != nil {
		t.Fatal(e)
	}
	defer os.RemoveAll(root)
	newConfig := func(name string, scope string) host.SourceHostConfig {
		dir := filepath.Join(root, name)
		if e := os.Mkdir(dir, 0700); e != nil {
			t.Fatal(e)
		}
		c := host.SourceHostConfig{Protocol: host.SourceHostProtocol, Mailbox: host.AutomaticHandoffConfigPrivateV2{Enabled: true, Directory: dir, Invocation: "controlled-source-host", InterpreterRole: "canned-role-not-HOST", ReviewerRole: "unused-reviewer", ScopeSHA256: scope, TimeoutMS: 2000, OverallTimeoutMS: 2000, PollMS: 1, Limits: host.AutomaticLimitsPrivateV2{MaxCalls: 1, MaxRequestBytes: 64 << 10, MaxResponseBytes: 16 << 10, MaxTotalBytes: 256 << 10, MaxOutputBytes: 64 << 10}}}
		c.Mailbox.ApprovalSHA256 = host.AutomaticHandoffApprovalPrivateV2(c.Mailbox)
		return c
	}
	limits := describeworker.SuppliedResponseLimits{MaxAdapterBytes: 4 << 10, MaxSemanticBytes: 1 << 10}
	cfg := newConfig("mailbox", h.ID())
	// Genuine opaque originals/expected/cache, not a caller-supplied bound DTO.
	// The observation must do no regeneration, Store IO or mailbox publication.
	phaseBeforeGets, phaseBeforePuts := store.OperationCounts()
	phaseProbe := &earlyWorkContext{Context: ctx}
	phases, phaseErr := out.HostPhaseBounds(phaseProbe, cfg, request, limits)
	phaseAfterGets, phaseAfterPuts := store.OperationCounts()
	phaseFiles, phaseFileErr := os.ReadDir(cfg.Mailbox.Directory)
	if phaseErr != nil || phases.Source == 0 || phases.Lower == 0 || phases.Binder == 0 || phaseBeforeGets != phaseAfterGets || phaseBeforePuts != phaseAfterPuts || phaseProbe.regenerated || phaseProbe.plan || phaseFileErr != nil || len(phaseFiles) != 0 {
		t.Fatal("ASSERT_HOST_GENUINE_PHASE_METADATA_ONLY", phases, phaseErr, phaseProbe, phaseFileErr)
	}
	added := []byte("controlled additionally retained store bytes")
	if _, e := store.Put(ctx, added); e != nil {
		t.Fatal(e)
	}
	next, phaseErr := out.HostPhaseBounds(ctx, cfg, request, limits)
	if phaseErr != nil || next.Source != phases.Source+uint64(len(added)) || next.Lower != phases.Lower+uint64(len(added)) || next.Binder != phases.Binder+uint64(len(added)) {
		t.Fatal("ASSERT_HOST_PHASE_CURRENT_STORE", phases, next, phaseErr)
	}
	if _, e := (&sc.PrivateAllModeSourcesResult{}).HostPhaseBounds(ctx, cfg, request, limits); e == nil {
		t.Fatal("ASSERT_HOST_PHASE_DTO_CANNOT_MINT")
	}
	t.Logf("ASSERT_HOST_GENUINE_PHASE_METADATA_ONLY: PASS; ASSERT_HOST_PHASE_CURRENT_STORE: PASS; ASSERT_HOST_PHASE_DTO_CANNOT_MINT: PASS; genuine peaks source=%d lower=%d binder=%d", phases.Source, phases.Lower, phases.Binder)
	for name, change := range map[string]func(*sc.SourceAcquisitionRequest){
		"record":  func(v *sc.SourceAcquisitionRequest) { v.Record.Envelope.Prompt += "foreign" },
		"unit":    func(v *sc.SourceAcquisitionRequest) { v.Attempt.SourceUnitID = out.Requests[1].Envelope.AdmissionID },
		"attempt": func(v *sc.SourceAcquisitionRequest) { v.Attempt.RequestRecordID = "foreign" },
		"expected": func(v *sc.SourceAcquisitionRequest) {
			p := *v.Resume.PrivateSources
			p.Expected = sc.PrivateAllModeSourcesExpected{}
			v.Resume.PrivateSources = &p
		},
		"workspace": func(v *sc.SourceAcquisitionRequest) { v.Resume.Workspace = "file:///foreign" },
		"original": func(v *sc.SourceAcquisitionRequest) {
			p := *v.Resume.PrivateSources
			p.Originals = nil
			v.Resume.PrivateSources = &p
		},
		"resealed": func(v *sc.SourceAcquisitionRequest) {
			u := out.Units[0]
			u.Body += "foreign"
			x, e := describerequest.FormatPrivateSourceRequestCandidate(u, contract.DescribeDeadlineMS(), 1<<20)
			if e != nil {
				t.Fatal(e)
			}
			v.Record = x
			v.Attempt.RequestRecordID = x.RecordID
			v.Attempt.SourceUnitID = u.ID
		},
	} {
		bad := request
		change(&bad)
		if p, e := sc.PrepareSourceHost(ctx, cfg, bad, limits); e == nil || p.Eligible() {
			p.Close()
			t.Fatal("ASSERT_HOST_EXACT_JOIN", name, e)
		}
		entries, e := os.ReadDir(cfg.Mailbox.Directory)
		if e != nil || len(entries) != 0 {
			t.Fatal("ASSERT_HOST_NO_MAILBOX_BEFORE_JOIN", name, e)
		}
	}
	t.Log("ASSERT_HOST_EXACT_JOIN: PASS; ASSERT_HOST_NO_MAILBOX_BEFORE_JOIN: PASS")
	// Genuine oversized READY handoff: unchanged source limits, seeds and source
	// payload. Four absent-source READY inputs retain larger preexisting payloads.
	absent, _, _, _ := sourceStageFixture(t, "positive", false)
	baseSlots, baseEntries, _ := sourceStageEntries(t, input.Input, []string{"RD", "C", "CRD", "D", "CD", "R", "CR"}, 0, []byte("source selection\n"), "CONTROLLED_SOURCE_MODE")
	slots, entries, _ := sourceStageEntries(t, absent.Input, []string{"RD", "RD", "RD", "RD"}, 7, bytes.Repeat([]byte("s"), 1<<16), strings.Repeat("p", 4096))
	large, e := cc.NewPrivateAllModeHandoff(qWorkspace(q), "source-controlled-revision", append(baseSlots, slots...), append(baseEntries, entries...))
	if e != nil {
		t.Fatal(e)
	}
	largeStore, e := sc.NewRootlessSourceHostStore(8 << 20)
	if e != nil {
		t.Fatal(e)
	}
	largePause := sc.Run(ctx, sc.Request{PrivateAllMode: &large, PrivateSources: &q, Workspace: qWorkspace(q), Contract: contract, Store: largeStore, StopAfter: sc.StopAfterPrivateSourceRequests})
	if largePause.Err != nil || largePause.PrivateSources == nil {
		t.Fatal("ASSERT_HOST_LARGE_SETUP", largePause.Err)
	}
	lo := largePause.PrivateSources
	lr := lo.Requests[0]
	bad := sc.SourceAcquisitionRequest{Resume: sc.ResumeRequest{PrivateSources: &sc.PrivateAllModeSourcesResume{Version: sc.PrivateAllModeSourcesVersion, Expected: lo.Expected, Originals: q.Originals, Definitions: q.Definitions, ExpectedHandoff: large}, Workspace: qWorkspace(q), Store: largeStore, StopAfter: sc.StopAfterPrivateSourceRequests, Selector: lo.CheckpointID}, Record: lr, Attempt: sc.SuppliedSourceAttempt{ID: request.Attempt.ID, CheckpointID: lo.CheckpointID, SourceUnitID: lr.Envelope.AdmissionID, RequestRecordID: lr.RecordID}}
	bigCfg := newConfig("large-mailbox", large.ID())
	largeGets, largePuts := largeStore.OperationCounts()
	probe := &earlyWorkContext{Context: ctx}
	p, e := sc.PrepareSourceHost(probe, bigCfg, bad, limits)
	p.Close()
	files, fe := os.ReadDir(bigCfg.Mailbox.Directory)
	afterLargeGets, afterLargePuts := largeStore.OperationCounts()
	if e == nil || p.Eligible() || afterLargeGets != largeGets || afterLargePuts != largePuts || probe.regenerated || probe.plan || fe != nil || len(files) != 0 {
		t.Fatal("ASSERT_HOST_EARLY_AGGREGATE", e, afterLargeGets-largeGets, afterLargePuts-largePuts, probe.regenerated, fe)
	}
	t.Log("ASSERT_HOST_EARLY_AGGREGATE: PASS")
	// Lower frame/output limits fail early too, before any source StoreGet.
	smallCfg := cfg
	smallCfg.Mailbox.Limits.MaxRequestBytes = 10
	smallCfg.Mailbox.ApprovalSHA256 = host.AutomaticHandoffApprovalPrivateV2(smallCfg.Mailbox)
	beforeGets, beforePuts := store.OperationCounts()
	p, e = sc.PrepareSourceHost(ctx, smallCfg, request, limits)
	afterGets, afterPuts := store.OperationCounts()
	if e == nil || p.Eligible() || afterGets != beforeGets || afterPuts != beforePuts {
		p.Close()
		t.Fatal("ASSERT_HOST_EARLY_FRAME", e)
	}
	t.Log("ASSERT_HOST_EARLY_FRAME: PASS")
	eligible, e := sc.PrepareSourceHost(ctx, cfg, request, limits)
	if e != nil || !eligible.Eligible() {
		t.Fatal("ASSERT_HOST_ELIGIBLE", e)
	}
	defer eligible.Close()
	semantic := `{"verdict":"ABSTAINED","target_role":"controlled source fixture","consumer_need":{"status":"UNRESOLVED","value":""},"provided_behavior":{"value":"fixture body only","consumer_relative":false},"boundary_contribution":"unknown","limitations":["canned fake role, not HOST output"]}`
	payload := describeworker.EncodeAdapterResultV2(semantic)
	payloadPath := filepath.Join(root, "canned-response.txt")
	if e = os.WriteFile(payloadPath, payload, 0600); e != nil {
		t.Fatal(e)
	}
	// Existing helper respond reads caller-owned raw bytes and publishes exactly
	// one response frame. Never launch/supervise an operator or a model client.
	_, testFile, _, _ := runtime.Caller(0)
	helper := filepath.Join(filepath.Dir(testFile), "..", "..", "scripts", "private-variable-v2-host-handoff.py")
	done := make(chan error, 1)
	go func() {
		until := time.Now().Add(2 * time.Second)
		for time.Now().Before(until) {
			if _, e := os.Stat(filepath.Join(cfg.Mailbox.Directory, "request-000001.json")); e == nil {
				break
			}
			time.Sleep(time.Millisecond)
		}
		cmd := exec.CommandContext(ctx, "python3", helper, "respond", "--mailbox", cfg.Mailbox.Directory, "--sequence", "1", "--payload", payloadPath)
		raw, e := cmd.CombinedOutput()
		if e != nil {
			done <- fmt.Errorf("controlled publisher: %w: %s", e, raw)
			return
		}
		done <- nil
	}()
	result, e := sc.CompleteSourceHost(ctx, eligible)
	publisherErr := <-done // drain before Fatal
	if e != nil || publisherErr != nil || result.Supplied().ID() == "" || !result.Delivery().Delivered() {
		t.Fatal("ASSERT_HOST_CONNECTED", e, publisherErr)
	}
	// Actual opaque delivered value, not a zero DTO: deterministic close-error
	// finalization witness. Physical os.Root Close failure is not injected here.
	closeFailure := errors.New("controlled first mailbox close failure")
	failed, ce := result.AfterClose(closeFailure)
	if !errors.Is(ce, closeFailure) || failed.Supplied().ID() != "" || failed.Delivery().Delivered() {
		t.Fatal("ASSERT_HOST_CLOSE_ERROR_WITHHOLDS_DELIVERY", ce)
	}
	preserved, ce := result.AfterClose(nil)
	if ce != nil || preserved.Supplied().ID() != result.Supplied().ID() || !preserved.Delivery().Delivered() {
		t.Fatal("ASSERT_HOST_CLOSE_SUCCESS_CONTROL", ce)
	}
	suppliedUpper, deliveryUpper, semanticUpper, be := result.NativePayloadBounds()
	bound, _ := result.Supplied().Bytes()
	receipt, _ := result.Delivery().Bytes()
	if be != nil || suppliedUpper != uint64(len(bound)) || deliveryUpper < uint64(len(receipt)) || semanticUpper == 0 {
		t.Fatal("ASSERT_NATIVE_OPAQUE_ACTUAL_LENGTHS", suppliedUpper, deliveryUpper, semanticUpper, be)
	}
	t.Log("ASSERT_HOST_CLOSE_ERROR_WITHHOLDS_DELIVERY: PASS; ASSERT_HOST_CLOSE_SUCCESS_CONTROL: PASS; ASSERT_NATIVE_OPAQUE_ACTUAL_LENGTHS: PASS; LIMITED: actual-value finalization helper plus finite actual-call AST, not physical close-fault execution")
	if !bytes.Contains(bound, []byte(`"origin":"SUPPLIED_TERMINAL_RESPONSE_ONLY"`)) || !bytes.Contains(bound, []byte(`"execution":"NO_PROCESS_OR_HOST_EXECUTION_VERIFIED"`)) || !bytes.Contains(receipt, []byte("HOST_OBSERVED_NOT_AUTHENTICATED")) {
		t.Fatal("ASSERT_HOST_SUPPLIED_CEILING")
	}
	var response host.SourceHostResponse
	responseRaw, e := os.ReadFile(filepath.Join(cfg.Mailbox.Directory, "response-000001.json"))
	if e != nil || json.Unmarshal(responseRaw, &response) != nil || !bytes.Equal(response.Payload, payload) {
		t.Fatal("ASSERT_HOST_HELPER_EXACT_BYTES", e)
	}
	if _, e = sc.CompleteSourceHost(ctx, eligible); e == nil {
		t.Fatal("ASSERT_HOST_UPPER_ONE_SHOT")
	}
	var public sc.SourceHostHandoff
	raw, _ := json.Marshal(eligible)
	if json.Unmarshal(raw, &public) != nil || public.Eligible() {
		t.Fatal("ASSERT_HOST_PUBLIC_NONADMITTING")
	}
	_, finalPuts := store.OperationCounts()
	if finalPuts != beforePuts {
		t.Fatal("ASSERT_HOST_NO_PUBLICATION")
	}
	// After the original positive delivery, exercise the real scalar refusal with
	// more currently retained store bytes. This is NOT an operator phase-separating
	// acquisition fixture, and must not be reported as one.
	if _, e := store.Put(ctx, bytes.Repeat([]byte("s"), 1<<20)); e != nil {
		t.Fatal(e)
	}
	refusalCfg := newConfig("phase-store-refusal", h.ID())
	phaseBeforeGets, phaseBeforePuts = store.OperationCounts()
	refusalProbe := &earlyWorkContext{Context: ctx}
	_, refusalErr := out.HostPhaseBounds(refusalProbe, refusalCfg, request, limits)
	phaseAfterGets, phaseAfterPuts = store.OperationCounts()
	refusalFiles, refusalFileErr := os.ReadDir(refusalCfg.Mailbox.Directory)
	if refusalErr == nil || phaseAfterGets != phaseBeforeGets || phaseAfterPuts != phaseBeforePuts || refusalProbe.regenerated || refusalProbe.plan || refusalFileErr != nil || len(refusalFiles) != 0 {
		t.Fatal("ASSERT_HOST_GENUINE_PHASE_EARLY_REFUSAL", refusalErr, refusalProbe, refusalFileErr)
	}
	t.Log("ASSERT_HOST_GENUINE_PHASE_EARLY_REFUSAL: PASS (current-store bound method, not connected operator separation)")
	t.Log("ASSERT_HOST_CONNECTED: PASS; ASSERT_HOST_HELPER_EXACT_BYTES: PASS; ASSERT_HOST_SUPPLIED_CEILING: PASS; ASSERT_HOST_UPPER_ONE_SHOT: PASS; ASSERT_HOST_PUBLIC_NONADMITTING: PASS; ASSERT_HOST_ALL7: PASS; original caps unchanged; source=32 unresolved=24 requests=2 exchanges=1 sandbox/model starts=0 actual_HOST=0")
}

// A genuine bounded filesystem store with immutable digest keys and cloned bytes.
// Its root contains the mailbox; correct source checkpoints do not prove disjointness.
type sourceHostFSStore struct {
	root       string
	gets, puts int
}

func (s *sourceHostFSStore) MaxObjectBytes() int64 { return 8 << 20 }
func (s *sourceHostFSStore) Put(ctx context.Context, b []byte) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if len(b) > 8<<20 {
		return "", errors.New("fixture object ceiling")
	}
	id := fmt.Sprintf("sha256:%x", sha256.Sum256(b))
	name := filepath.Join(s.root, strings.TrimPrefix(id, "sha256:"))
	s.puts++
	if prior, err := os.ReadFile(name); err == nil {
		if !bytes.Equal(prior, b) {
			return "", errors.New("fixture replacement")
		}
		return id, nil
	}
	f, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return "", err
	}
	_, err = f.Write(bytes.Clone(b))
	closeErr := f.Close()
	if err != nil {
		return "", err
	}
	return id, closeErr
}
func (s *sourceHostFSStore) Get(ctx context.Context, id string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.gets++
	if len(id) != 71 || !strings.HasPrefix(id, "sha256:") || strings.ContainsAny(id[7:], "/\\\\") {
		return nil, errors.New("fixture key")
	}
	b, err := os.ReadFile(filepath.Join(s.root, id[7:]))
	if len(b) > 8<<20 {
		return nil, errors.New("fixture object ceiling")
	}
	return bytes.Clone(b), err
}
func TestSuppliedHostFSStore(t *testing.T) {
	input, h, contract, q := sourceStageFixture(t, "positive", false)
	inv.ControlledSourceProjectionNoReadmitForTest(t, input)
	ctx, cancel := context.WithDeadline(context.Background(), q.Deadline)
	defer cancel()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	root, err := os.MkdirTemp(home, ".source-host-fs-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)
	store := &sourceHostFSStore{root: root}
	pause := sc.Run(ctx, sc.Request{PrivateAllMode: &h, PrivateSources: &q, Workspace: qWorkspace(q), Contract: contract, Store: store, StopAfter: sc.StopAfterPrivateSourceRequests})
	if pause.Err != nil || pause.PrivateSources == nil {
		t.Fatal("ASSERT_HOST_FS_SETUP", pause.Err)
	}
	out := pause.PrivateSources
	r := out.Requests[0]
	resume := sc.ResumeRequest{PrivateSources: &sc.PrivateAllModeSourcesResume{Version: sc.PrivateAllModeSourcesVersion, Expected: out.Expected, Originals: q.Originals, Definitions: q.Definitions, ExpectedHandoff: h}, Workspace: qWorkspace(q), Store: store, StopAfter: sc.StopAfterPrivateSourceRequests, Selector: out.CheckpointID}
	// Prove the filesystem store actually serves the correct paused source artifacts.
	if resumed := sc.Resume(ctx, resume); resumed.Err != nil {
		t.Fatal("ASSERT_HOST_FS_RESUME_SETUP", resumed.Err)
	}
	mailbox := filepath.Join(root, "mailbox")
	if err := os.Mkdir(mailbox, 0700); err != nil {
		t.Fatal(err)
	}
	cfg := host.SourceHostConfig{Protocol: host.SourceHostProtocol, Mailbox: host.AutomaticHandoffConfigPrivateV2{Enabled: true, Directory: mailbox, Invocation: "controlled-fs", InterpreterRole: "canned", ReviewerRole: "unused", ScopeSHA256: h.ID(), TimeoutMS: 1000, OverallTimeoutMS: 1000, PollMS: 1, Limits: host.AutomaticLimitsPrivateV2{MaxCalls: 1, MaxRequestBytes: 64 << 10, MaxResponseBytes: 16 << 10, MaxTotalBytes: 256 << 10, MaxOutputBytes: 64 << 10}}}
	cfg.Mailbox.ApprovalSHA256 = host.AutomaticHandoffApprovalPrivateV2(cfg.Mailbox)
	request := sc.SourceAcquisitionRequest{Resume: resume, Record: r, Attempt: sc.SuppliedSourceAttempt{ID: "controlled-fs-attempt", CheckpointID: out.CheckpointID, SourceUnitID: r.Envelope.AdmissionID, RequestRecordID: r.RecordID}}
	store.gets, store.puts = 0, 0
	eligible, err := sc.PrepareSourceHost(ctx, cfg, request, describeworker.SuppliedResponseLimits{MaxAdapterBytes: 4 << 10, MaxSemanticBytes: 1 << 10})
	eligible.Close()
	files, readErr := os.ReadDir(mailbox)
	if err == nil || eligible.Eligible() || store.gets != 0 || store.puts != 0 || readErr != nil || len(files) != 0 {
		t.Fatal("ASSERT_HOST_FS_STORE_REFUSED_BEFORE_IO", err, store.gets, store.puts, len(files), readErr)
	}
	t.Log("ASSERT_HOST_FS_STORE_REFUSED_BEFORE_IO: PASS")
}

// Observe real completed preparation at the existing upper liveness boundary.
// Waiting targets its published absolute deadline; caller/source remain live.
// The pure minimum and closed-call guards separately avoid relying on timing.
type sourceHostEligibleContext struct {
	context.Context
	dir      string
	seen     bool
	deadline time.Time
}

func (c *sourceHostEligibleContext) Err() error {
	if err := c.Context.Err(); err != nil {
		return err
	}
	for n := 1; n < 10; n++ {
		pc, _, _, ok := runtime.Caller(n)
		if !ok || !strings.HasSuffix(runtime.FuncForPC(pc).Name(), ".sourceHostPublicationLive") {
			continue
		}
		var prep struct{ Protocol, Deadline string }
		raw, err := os.ReadFile(filepath.Join(c.dir, "preparation.json"))
		if err != nil || json.Unmarshal(raw, &prep) != nil || prep.Protocol != host.SourceHostProtocol {
			break
		}
		deadline, err := time.Parse(time.RFC3339Nano, prep.Deadline)
		if err != nil {
			break
		}
		c.seen, c.deadline = true, deadline
		timer := time.NewTimer(time.Until(deadline))
		<-timer.C
		timer.Stop()
		break
	}
	return c.Context.Err()
}
func TestSuppliedHostLatePreparation(t *testing.T) {
	input, h, contract, q := sourceStageFixture(t, "positive", false)
	inv.ControlledSourceProjectionNoReadmitForTest(t, input)
	ctx, cancel := context.WithDeadline(context.Background(), q.Deadline)
	defer cancel()
	originalDeadline := q.Deadline
	store, err := sc.NewRootlessSourceHostStore(8 << 20)
	if err != nil {
		t.Fatal(err)
	}
	pause := sc.Run(ctx, sc.Request{PrivateAllMode: &h, PrivateSources: &q, Workspace: qWorkspace(q), Contract: contract, Store: store, StopAfter: sc.StopAfterPrivateSourceRequests})
	if pause.Err != nil || pause.PrivateSources == nil {
		t.Fatal("ASSERT_HOST_LATE_PREPARATION_SOURCE", pause.Err)
	}
	out := pause.PrivateSources
	if out.Counts.Admitted != 32 || out.Counts.Unresolved != 24 || out.Counts.Requests != 2 {
		t.Fatal("ASSERT_HOST_LATE_PREPARATION_SOURCE_COUNTS", out.Counts)
	}
	r := out.Requests[0]
	resume := sc.ResumeRequest{PrivateSources: &sc.PrivateAllModeSourcesResume{Version: sc.PrivateAllModeSourcesVersion, Expected: out.Expected, Originals: q.Originals, Definitions: q.Definitions, ExpectedHandoff: h}, Workspace: qWorkspace(q), Store: store, StopAfter: sc.StopAfterPrivateSourceRequests, Selector: out.CheckpointID}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	dir, err := os.MkdirTemp(home, ".ep-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	cfg := host.SourceHostConfig{Protocol: host.SourceHostProtocol, Mailbox: host.AutomaticHandoffConfigPrivateV2{Enabled: true, Directory: dir, Invocation: "late-prep", InterpreterRole: "canned", ReviewerRole: "unused", ScopeSHA256: h.ID(), TimeoutMS: 100, OverallTimeoutMS: 500, PollMS: 1, Limits: host.AutomaticLimitsPrivateV2{MaxCalls: 1, MaxRequestBytes: 64 << 10, MaxResponseBytes: 16 << 10, MaxTotalBytes: 256 << 10, MaxOutputBytes: 64 << 10}}}
	cfg.Mailbox.ApprovalSHA256 = host.AutomaticHandoffApprovalPrivateV2(cfg.Mailbox)
	request := sc.SourceAcquisitionRequest{Resume: resume, Record: r, Attempt: sc.SuppliedSourceAttempt{ID: "late-prep", CheckpointID: out.CheckpointID, SourceUnitID: r.Envelope.AdmissionID, RequestRecordID: r.RecordID}}
	stage := &sourceHostEligibleContext{Context: ctx, dir: dir}
	eligible, err := sc.PrepareSourceHost(stage, cfg, request, describeworker.SuppliedResponseLimits{MaxAdapterBytes: 4 << 10, MaxSemanticBytes: 1 << 10})
	eligible.Close()
	if !stage.seen || stage.deadline.IsZero() || time.Now().Before(stage.deadline) || !stage.deadline.Before(originalDeadline) || ctx.Err() != nil || !time.Now().Before(originalDeadline) || q.Deadline != originalDeadline {
		t.Fatal("ASSERT_HOST_LATE_PREPARATION_READBACK_LIVE_SOURCE", stage.seen, stage.deadline, ctx.Err())
	}
	t.Log("ASSERT_HOST_LATE_PREPARATION_READBACK_LIVE_SOURCE: PASS; actual original mailbox deadline elapsed, source/caller unchanged and live")
	if err == nil || eligible.Eligible() {
		t.Fatal("ASSERT_HOST_ELIGIBLE_MAILBOX_EARLIER_DEADLINE", err, eligible.Eligible())
	}
	t.Log("ASSERT_HOST_ELIGIBLE_MAILBOX_EARLIER_DEADLINE: PASS; no exchange or actual_HOST invocation")
}

func TestPrivateSourceSELFResponder(t *testing.T) {
	at := -1
	for n, a := range os.Args {
		if a == "private-source-self" {
			at = n
			break
		}
	}
	if at < 0 {
		return
	}
	root := os.Args[at+1]
	mode := "positive"
	if len(os.Args) > at+2 {
		mode = os.Args[at+2]
	}
	uri := (&url.URL{Scheme: "file", Path: filepath.Join(root, "root.go")}).String()
	display := lsp.Range{Start: lsp.Position{Line: 1}, End: lsp.Position{Line: 1, Character: 22}}
	selection := lsp.Range{Start: lsp.Position{Line: 1, Character: 5}, End: lsp.Position{Line: 1, Character: 9}}
	site := lsp.Range{Start: lsp.Position{Line: 1, Character: 13}, End: lsp.Position{Line: 1, Character: 18}}
	item := lsp.CallHierarchyItem{Name: "Root", URI: uri, Kind: 12, Range: display, SelectionRange: selection}
	otherSelection := lsp.Range{Start: lsp.Position{Line: 2, Character: 5}, End: lsp.Position{Line: 2, Character: 10}}
	other := lsp.CallHierarchyItem{Name: "Other", URI: uri, Kind: 12, Range: lsp.Range{Start: lsp.Position{Line: 2}, End: lsp.Position{Line: 2, Character: 22}}, SelectionRange: otherSelection}
	reader := lspwire.NewReader(os.Stdin, lspwire.DefaultLimits())
	for {
		msg, err := reader.Read()
		if err != nil || msg.Method == "exit" {
			os.Exit(0)
		}
		if len(msg.ID) == 0 {
			continue
		}
		var result any
		switch msg.Method {
		case "initialize":
			result = map[string]any{"serverInfo": map[string]string{"name": "controlled-source-self", "version": "v1"}, "capabilities": map[string]any{"positionEncoding": "utf-16", "callHierarchyProvider": true, "documentSymbolProvider": true, "referencesProvider": true, "definitionProvider": true}}
		case "textDocument/documentSymbol":
			otherName := "Other"
			if mode == "same-name" {
				otherName = "Root" // native names cannot disambiguate these symbols
			}
			result = []any{map[string]any{"name": "Root", "kind": 12, "range": display, "selectionRange": selection}, map[string]any{"name": otherName, "kind": 12, "range": other.Range, "selectionRange": other.SelectionRange}}
		case "textDocument/references", "textDocument/definition":
			targetURI, r := uri, otherSelection
			if mode == "foreign" {
				targetURI = (&url.URL{Scheme: "file", Path: filepath.Join(root, "foreign.go")}).String()
			}
			if mode == "snippet" {
				r = site
			}
			result = []any{map[string]any{"uri": targetURI, "range": r}}
			count := map[string]int{"native-large": 32, "host-phase-8": 8, "host-phase-16": 16, "host-phase-24": 24}[mode]
			if count > 0 && msg.Method == "textDocument/references" {
				items := make([]any, count)
				for i := range items {
					items[i] = map[string]any{"uri": targetURI, "range": r}
				}
				result = items
			}
			if strings.HasPrefix(mode, "link") && msg.Method == "textDocument/definition" {
				full := other.Range
				if mode == "link-envelope" {
					full.End.Character = 21
				}
				if mode == "link-selection" {
					r.Start.Character++
					r.End.Character++
				}
				if mode == "link-foreign" {
					targetURI = (&url.URL{Scheme: "file", Path: filepath.Join(root, "foreign.go")}).String()
				}
				result = []any{map[string]any{"targetUri": targetURI, "targetRange": full, "targetSelectionRange": r}}
			}
		case "textDocument/prepareCallHierarchy":
			result = []lsp.CallHierarchyItem{item}
		case "callHierarchy/incomingCalls":
			otherSite := lsp.Range{Start: lsp.Position{Line: 2, Character: 14}, End: lsp.Position{Line: 2, Character: 18}}
			result = []lsp.CallHierarchyIncomingCall{{From: other, FromRanges: []lsp.Range{otherSite, otherSite}}}
		case "callHierarchy/outgoingCalls":
			result = []lsp.CallHierarchyOutgoingCall{{To: other, FromRanges: []lsp.Range{site, site}}}
		}
		raw, _ := json.Marshal(result)
		body := fmt.Sprintf(`{"jsonrpc":"2.0","id":%s,"result":%s}`, msg.ID, raw)
		if _, err = fmt.Fprintf(os.Stdout, "Content-Length: %d\r\n\r\n%s", len(body), body); err != nil {
			os.Exit(72)
		}
	}
}

type sourceStageStore struct {
	*cc.MemoryStore
	puts    int
	gets    int
	fail    bool
	corrupt string
}

func (s *sourceStageStore) MaxObjectBytes() int64 { return 8 << 20 }
func (s *sourceStageStore) Put(c context.Context, b []byte) (string, error) {
	s.puts++
	if s.fail {
		return "", errors.New("controlled source Put failure")
	}
	return s.MemoryStore.Put(c, b)
}
func (s *sourceStageStore) Get(c context.Context, id string) ([]byte, error) {
	s.gets++
	b, e := s.MemoryStore.Get(c, id)
	if id == s.corrupt && len(b) > 0 {
		b[0] ^= 1
	}
	return b, e
}

type sourceStageWorker struct{ calls int }

func (w *sourceStageWorker) Run(context.Context, describerequest.Record, string) (describeworker.RunResult, error) {
	w.calls++
	return describeworker.RunResult{}, errors.New("source stage must never execute")
}

func TestSuppliedOpPreparationControls(t *testing.T) {
	for _, control := range []string{"paid", "paid-same-name", "shortage", "scope", "seed", "attempt", "input", "root", "stale", "cancel", "deadline", "uncertain"} {
		t.Run(control, func(t *testing.T) {
			owner, root := inv.ControlledRPrepCanonicalOwnerForTest(t)
			if control == "shortage" {
				owner, root = inv.ControlledRPrepShortOwnerForTest(t)
			}
			m, start, uri := mixedStartFixtureV2(t)
			body := []byte("package fixture\nfunc Root(){ Other() }\nfunc Other(){ Root() }\n")
			if e := os.WriteFile(filepath.Join(start.Profile.Workspace().String(), "root.go"), body, 0644); e != nil {
				t.Fatal(e)
			}
			mode := "positive"
			if control == "paid-same-name" {
				mode = "same-name"
			}
			start.Process.Args = []string{"-test.run=^TestPrivateSourceSELFResponder$", "--", "private-source-self", start.Profile.Workspace().String(), mode}
			capture := inv.PrivateMixedInputRequestV2{Manager: m, Root: root, Start: start, URI: uri, OccurrenceID: "prep-control", Revision: "source-controlled-revision", Privacy: "private-source-controlled", OccurrencePolicy: "ordinal", CompositionPolicy: "identity", Line: 1, Character: 6, Kinds: []programcadmission.RelationKind{programcadmission.TypedReferencesSymbol}}
			ctx := context.Background()
			op, e := owner.ReserveMixedInputV2(ctx, fmt.Sprintf("sha256:%x", sha256.Sum256([]byte("capture-control"))), capture)
			if e != nil {
				t.Fatal(e)
			}
			input, e := inv.ExecuteMixedInputV2(ctx, op, capture)
			if e != nil {
				t.Fatal(e)
			}
			q := inv.PrivateSourceRPreparationRequestV1{Input: input, Root: root, ExpectedHead: owner.PreparationHeadV1(), Attempt: fmt.Sprintf("sha256:%x", sha256.Sum256([]byte("prep-control"))), Workspace: start.Profile.Workspace().String(), Revision: capture.Revision, RunID: "prep-control", SeedNamespace: "source-original-schedule", Seed: 7, Deadline: input.SourceDeadlineV1()}
			_, before := inv.ControlledRPrepDebitForTest(t, owner)
			if control == "scope" {
				q.RunID = string([]byte{0xff})
			}
			if control == "stale" {
				q.ExpectedHead = "sha256:" + strings.Repeat("0", 64)
			}
			operation, e := owner.ReserveRPreparationV1(ctx, q)
			if control == "shortage" || control == "stale" || control == "scope" {
				if e == nil {
					t.Fatal("ASSERT_RPREP_NO_RESERVATION", control)
				}
				_, after := inv.ControlledRPrepDebitForTest(t, owner)
				if before != after {
					t.Fatal("ASSERT_RPREP_NO_CONSTRUCTION_DEBIT", control)
				}
				if p, e := inv.ExecuteRPreparationV1(ctx, operation, q); e == nil {
					_, _, _, _, _, ve := p.Values()
					t.Fatal("ASSERT_RPREP_NO_USABLE", ve)
				}
				t.Log("ASSERT_RPREP_NO_RESERVATION: PASS", control)
				return
			}
			if e != nil {
				t.Fatal(e)
			}
			disposition, afterReserve := inv.ControlledRPrepDebitForTest(t, owner)
			if disposition != "PENDING" || afterReserve[0] <= before[0] || afterReserve[13] <= before[13] {
				t.Fatal("ASSERT_RPREP_UPFRONT_DEBIT", disposition, before, afterReserve)
			}
			expected := inv.ControlledRPrepExpectedChargeForTest(t, q)
			for i, n := range expected {
				if i == 8 {
					continue
				}
				if afterReserve[i]-before[i] != n {
					t.Fatal("ASSERT_RPREP_EXACT_DEBIT", i, n, afterReserve[i]-before[i])
				}
			}
			bad := q
			switch control {
			case "seed":
				bad.Seed++
			case "attempt":
				bad.Attempt = "sha256:" + strings.Repeat("0", 64)
			case "input":
				bad.Input = inv.PrivateManagedGroupingInput{}
			case "root":
				bad.Root = nil
			case "deadline":
				bad.Deadline = time.Now().Add(-time.Second)
			case "cancel":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			case "uncertain":
				inv.ControlledMixedObserveUncertainForTest(owner)
			}
			prepared, e := inv.ExecuteRPreparationV1(ctx, operation, bad)
			if strings.HasPrefix(control, "paid") {
				if e != nil {
					t.Fatal("ASSERT_RPREP_PAID", e)
				}
				_, _, _, _, _, e = prepared.Values()
				if e != nil {
					t.Fatal(e)
				}
				retained, ok := prepared.RetainedPayloadUpper()
				if !ok || retained <= uint64(input.Input.ByteLength()) || retained > 8<<20 {
					t.Fatal("ASSERT_RPREP_ACTUAL_RETAINED_METADATA", retained, ok)
				}
				t.Logf("ASSERT_RPREP_ACTUAL_RETAINED_METADATA: PASS bound=%d input=%d", retained, input.Input.ByteLength())
				sourceOriginalTargetChecks(t, prepared, q)
			} else {
				if e == nil {
					t.Fatal("ASSERT_RPREP_BAD_USABLE", control)
				}
				if _, _, _, _, _, ve := prepared.Values(); ve == nil {
					t.Fatal("ASSERT_RPREP_BAD_USABLE", control)
				}
			}
			if _, e = inv.ExecuteRPreparationV1(context.Background(), operation, q); e == nil {
				t.Fatal("ASSERT_RPREP_ONE_SHOT", control)
			}
			t.Logf("ASSERT_RPREP_UPFRONT_DEBIT: PASS; ASSERT_RPREP_ONE_SHOT: PASS control=%s before=%v reserved=%v", control, before, afterReserve)
		})
	}
}

// Genuine paid preparation, post-Observe originals, canonical admissions and
// exact later HOST join. Public DTO perturbations never stand in for this control.
func sourceOriginalTargetChecks(t *testing.T, prepared inv.PreparedSourceRV1, q inv.PrivateSourceRPreparationRequestV1) {
	t.Helper()
	inv.ControlledOriginalTargetRetentionForTest(t, prepared)
	input, policy, frozen, result, originals, e := prepared.Values()
	if e != nil {
		t.Fatal(e)
	}
	modes := []string{"RD", "C", "CRD", "D", "CD", "R", "CR"}
	slots := make([]cc.PrivateAllModeSlot, 7)
	entries := make([]cc.PrivateAllModeEntry, 7)
	for i, mode := range modes {
		run := q.RunID + "/" + mode
		if mode == "R" {
			run = q.RunID
		}
		slots[i] = cc.PrivateAllModeSlot{Ordinal: i, Mode: mode, RunID: run, SeedNamespace: q.SeedNamespace, Seed: q.Seed}
		entries[i] = cc.PrivateAllModeEntry{Slot: slots[i], Disposition: cc.PrivateAllModeDeclaredMissing, Lineage: q.RunID, MissingReason: "NOT_CAPTURED_BY_R_ONLY_OPERATOR;UNRESOLVED"}
		if mode == "R" {
			entries[i] = cc.PrivateAllModeEntry{Slot: slots[i], Disposition: cc.PrivateAllModeReady, Lineage: q.RunID, Input: input, Policy: policy, Case: frozen, Result: result}
		}
	}
	h, e := cc.NewPrivateAllModeHandoff(q.Workspace, q.Revision, slots, entries)
	if e != nil {
		t.Fatal(e)
	}
	objects, e := prepared.SourceObjects()
	if e != nil {
		t.Fatal(e)
	}
	contract := sourceStageContract(t)
	sources := sc.PrivateAllModeSourcesRequest{PolicyID: contract.PacketSourcePolicy().PolicyID, Limits: tp.PrivateSourceLimits{MaxDocumentBytes: 1 << 20, MaxTotalBytes: 8 << 20, MaxWork: 10000, MaxObjects: 1000}, Deadline: q.Deadline, Originals: []inv.PrivateProducerOriginals{originals}, Objects: objects}
	store, e := sc.NewRootlessSourceHostStore(8 << 20)
	if e != nil {
		t.Fatal(e)
	}
	out := sc.Run(context.Background(), sc.Request{PrivateAllMode: &h, PrivateSources: &sources, Workspace: q.Workspace, Contract: contract, Store: store, StopAfter: sc.StopAfterPrivateSourceRequests})
	if out.Err != nil || out.PrivateSources == nil {
		t.Fatal("ASSERT_ORIGINAL_CANONICAL_ADMISSION", out.Err)
	}
	s := out.PrivateSources
	before, _ := json.Marshal(s)
	if s.Counts.Intents != 8 || s.Counts.Admitted != 2 || s.Counts.Unresolved != 6 || s.Counts.Requests != 2 || len(s.Units) != 2 {
		t.Fatal("ASSERT_ORIGINAL_NOMINATIONS_RETAINED", s.Counts)
	}
	selectionGets, selectionPuts := store.OperationCounts()
	var original, other tp.PrivateSourceUnit
	for _, u := range s.Units {
		if prepared.MatchesOriginalQueryTarget(u) {
			original = u
		} else {
			other = u
		}
	}
	if original.ID == "" || other.ID == "" || original.URI != other.URI || original.SymbolID == other.SymbolID || original.SelectionRange == other.SelectionRange {
		t.Fatal("ASSERT_ORIGINAL_SAME_FILE_DISTINCT_SYMBOLS")
	}
	for _, units := range [][]tp.PrivateSourceUnit{{other, original}, {original, other}} {
		id, err := prepared.OriginalQueryTargetID(units)
		if err != nil || id != original.ID {
			t.Fatal("ASSERT_ORIGINAL_OTHER_BEFORE_QUERY", err)
		}
	}
	if id, err := prepared.OriginalQueryTargetID([]tp.PrivateSourceUnit{other}); err == nil || id != "" {
		t.Fatal("ASSERT_ORIGINAL_MISSING_REFUSED")
	}
	for name, change := range map[string]func(*tp.PrivateSourceUnit){
		"symbol":       func(u *tp.PrivateSourceUnit) { u.SymbolID = other.SymbolID },
		"selection":    func(u *tp.PrivateSourceUnit) { u.SelectionRange = other.SelectionRange },
		"display":      func(u *tp.PrivateSourceUnit) { u.DisplayRange = other.DisplayRange },
		"uri":          func(u *tp.PrivateSourceUnit) { u.URI += "other" },
		"source":       func(u *tp.PrivateSourceUnit) { u.SourceDigest = "sha256:" + strings.Repeat("0", 64) },
		"session":      func(u *tp.PrivateSourceUnit) { u.SessionID += "other" },
		"generation":   func(u *tp.PrivateSourceUnit) { u.Generation++ },
		"version":      func(u *tp.PrivateSourceUnit) { u.DocumentVersion += "other" },
		"encoding":     func(u *tp.PrivateSourceUnit) { u.PositionEncoding = "utf-8" },
		"revision":     func(u *tp.PrivateSourceUnit) { u.Revision += "other" },
		"privacy":      func(u *tp.PrivateSourceUnit) { u.Privacy += "other" },
		"workspace":    func(u *tp.PrivateSourceUnit) { u.Workspace += "other" },
		"input":        func(u *tp.PrivateSourceUnit) { u.InputID = "foreign" },
		"unit-version": func(u *tp.PrivateSourceUnit) { u.Version = "foreign" },
		"ceiling":      func(u *tp.PrivateSourceUnit) { u.ClaimCeiling = tp.PrivateSourceCandidateCeiling },
		"id":           func(u *tp.PrivateSourceUnit) { u.ID = "" },
	} {
		bad := original
		change(&bad)
		if prepared.MatchesOriginalQueryTarget(bad) {
			t.Fatal("ASSERT_ORIGINAL_EXACT_METADATA", name)
		}
		if id, err := prepared.OriginalQueryTargetID([]tp.PrivateSourceUnit{other, bad}); err == nil || id != "" {
			t.Fatal("ASSERT_ORIGINAL_EXACT_METADATA", name)
		}
		t.Log("ASSERT_ORIGINAL_EXACT_METADATA: PASS", name)
	}
	conflict := other
	conflict.ID = original.ID
	duplicate := original
	duplicate.ID += "other"
	for _, units := range [][]tp.PrivateSourceUnit{{original, original}, {original, duplicate}, {original, conflict}, nil} {
		if id, err := prepared.OriginalQueryTargetID(units); err == nil || id != "" {
			t.Fatal("ASSERT_ORIGINAL_AMBIGUITY_REFUSED")
		}
	}
	gets, puts := store.OperationCounts()
	if gets != selectionGets || puts != selectionPuts {
		t.Fatal("ASSERT_ORIGINAL_SELECTION_NO_STORE_EFFECTS")
	}
	t.Logf("ASSERT_ORIGINAL_CANONICAL_ORDER: first_other=%t; requests=%d units=%d; all retained", s.Requests[0].Envelope.AdmissionID == other.ID, len(s.Requests), len(s.Units))
	t.Log("ASSERT_ORIGINAL_SELECTION_NO_STORE_EFFECTS: PASS (including genuine no-original-match refusal; no HOST exchange)")
	if (inv.PreparedSourceRV1{}).MatchesOriginalQueryTarget(original) {
		t.Fatal("ASSERT_ORIGINAL_ZERO_NO_MATCH")
	}
	// A public matching DTO is intentionally NOT admission: altering its body
	// leaves the scalar predicate true but the exact full-record HOST join refuses.
	fake := original
	fake.Body += "\ncaller-invented-body"
	if !prepared.MatchesOriginalQueryTarget(fake) {
		t.Fatal("ASSERT_ORIGINAL_METADATA_NOT_ADMISSION_CONTROL")
	}
	r, e := describerequest.FormatPrivateSourceRequestCandidate(fake, contract.DescribeDeadlineMS(), 1<<20)
	if e != nil {
		t.Fatal(e)
	}
	dir, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	mailbox := filepath.Join(dir, "mailbox")
	if e = os.Mkdir(mailbox, 0700); e != nil {
		t.Fatal(e)
	}
	cfg := host.SourceHostConfig{Protocol: host.SourceHostProtocol, Mailbox: host.AutomaticHandoffConfigPrivateV2{Enabled: true, Directory: mailbox, Invocation: "original-target-control", InterpreterRole: "canned-not-HOST", ReviewerRole: "unused", ScopeSHA256: h.ID(), TimeoutMS: 2000, OverallTimeoutMS: 2000, PollMS: 1, Limits: host.AutomaticLimitsPrivateV2{MaxCalls: 1, MaxRequestBytes: 64 << 10, MaxResponseBytes: 16 << 10, MaxTotalBytes: 256 << 10, MaxOutputBytes: 64 << 10}}}
	cfg.Mailbox.ApprovalSHA256 = host.AutomaticHandoffApprovalPrivateV2(cfg.Mailbox)
	resume := sc.ResumeRequest{PrivateSources: &sc.PrivateAllModeSourcesResume{Version: sc.PrivateAllModeSourcesVersion, Expected: s.Expected, Originals: sources.Originals, ExpectedHandoff: h}, Workspace: q.Workspace, Store: store, StopAfter: sc.StopAfterPrivateSourceRequests, Selector: s.CheckpointID}
	request := sc.SourceAcquisitionRequest{Resume: resume, Record: r, Attempt: sc.SuppliedSourceAttempt{ID: "original-target-control", CheckpointID: s.CheckpointID, SourceUnitID: r.Envelope.AdmissionID, RequestRecordID: r.RecordID}}
	limits := describeworker.SuppliedResponseLimits{MaxAdapterBytes: 4 << 10, MaxSemanticBytes: 1 << 10}
	p, err := sc.PrepareSourceHost(context.Background(), cfg, request, limits)
	p.Close()
	files, fe := os.ReadDir(mailbox)
	if err == nil || p.Eligible() || fe != nil || len(files) != 0 {
		t.Fatal("ASSERT_ORIGINAL_METADATA_CANNOT_ADMIT", err, fe)
	}
	// The actual canonical original record still passes unchanged Resume admission.
	for _, record := range s.Requests {
		if record.Envelope.AdmissionID == original.ID {
			request.Record = record
			request.Attempt.RequestRecordID = record.RecordID
		}
	}
	p, err = sc.PrepareSourceHost(context.Background(), cfg, request, limits)
	if err != nil || !p.Eligible() {
		p.Close()
		t.Fatal("ASSERT_ORIGINAL_GENUINE_HOST_PREPARE", err)
	}
	p.Close()
	after, _ := json.Marshal(s)
	if !bytes.Equal(before, after) {
		t.Fatal("ASSERT_ORIGINAL_NOMINATIONS_RETAINED")
	}
	for i, entry := range entries {
		if entry.Slot.Seed != 7 || entry.Slot.Mode != modes[i] || (entry.Disposition == cc.PrivateAllModeReady) != (i == 5) {
			t.Fatal("ASSERT_ORIGINAL_SEVEN_MODE_SEED")
		}
	}
	t.Log("ASSERT_ORIGINAL_OTHER_BEFORE_QUERY: PASS; ASSERT_ORIGINAL_MISSING_REFUSED: PASS; ASSERT_ORIGINAL_AMBIGUITY_REFUSED: PASS; ASSERT_ORIGINAL_ZERO_NO_MATCH: PASS; ASSERT_ORIGINAL_METADATA_CANNOT_ADMIT: PASS; ASSERT_ORIGINAL_GENUINE_HOST_PREPARE: PASS; ASSERT_ORIGINAL_NOMINATIONS_RETAINED: PASS; ASSERT_ORIGINAL_SEVEN_MODE_SEED: PASS")
}

// TestSuppliedOp connects actual controlled owner, R SELF acquisition, paid
// preparation, source Resume, file-role exchange and Go-owned native publication.
func TestSuppliedOp(t *testing.T) {
	home, e := os.UserHomeDir()
	if e != nil {
		t.Fatal(e)
	}
	dir, e := os.MkdirTemp(home, ".source-op-cli-")
	if e != nil {
		t.Fatal(e)
	}
	defer os.RemoveAll(dir)
	binary := filepath.Join(dir, "lsp-trace")
	_, file, _, _ := runtime.Caller(0)
	build := exec.Command("go", "build", "-o", binary, "./cmd/lsp-trace")
	build.Dir = filepath.Join(filepath.Dir(file), "..", "..")
	raw, e := build.CombinedOutput()
	if e != nil {
		t.Fatalf("CLI build is not RED: %v %s", e, raw)
	}
	t.Run("api", func(t *testing.T) { testSuppliedOp(t, "") })
	t.Run("cli", func(t *testing.T) { testSuppliedOp(t, binary) })
}

type sourceOpCLIConfig struct {
	Schema, Mode, Pin                                                     string
	AccountingRoot                                                        string
	CanonicalJob                                                          []byte
	ExpectedHead                                                          string
	Verification                                                          inv.PrivateManagedInputVerificationV2
	Profile                                                               runtimeprofile.Selector
	LanguageID, Provider                                                  string
	ProcessPath                                                           string
	ProcessArgs                                                           []string
	Workspace, Revision, Privacy, URI, OccurrenceID                       string
	Line, Character                                                       uint32
	CaptureAttempt, PreparationAttempt, RoleAttempt, RunID, SeedNamespace string
	Seed                                                                  uint64
	Deadline                                                              time.Time
	Contract                                                              cc.ContractInput
	SourceLimits                                                          tp.PrivateSourceLimits
	Host                                                                  host.SourceHostConfig
	ResponseLimits                                                        describeworker.SuppliedResponseLimits
	OutputRoots                                                           [2]string
}

// Finite source-level supplement for physical close-fault paths which are not
// deterministically injectable through the closed production mailbox. Reads only
// the two authorized concrete implementations, not a compiler closure.
func TestSourceOperatorMailboxCloseBinding(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	readBody := func(path, name, receiver string) string {
		t.Helper()
		raw, e := os.ReadFile(path)
		if e != nil {
			t.Fatal(e)
		}
		fs := token.NewFileSet()
		f, e := parser.ParseFile(fs, path, raw, 0)
		if e != nil {
			t.Fatal(e)
		}
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Name.Name != name {
				continue
			}
			if receiver != "" {
				if fn.Recv == nil {
					continue
				}
				star, ok := fn.Recv.List[0].Type.(*ast.StarExpr)
				if !ok {
					continue
				}
				id, ok := star.X.(*ast.Ident)
				if !ok || id.Name != receiver {
					continue
				}
			}
			return string(raw[fs.Position(fn.Body.Pos()).Offset:fs.Position(fn.Body.End()).Offset])
		}
		t.Fatal("function absent", name)
		return ""
	}
	lower := readBody(filepath.Join(filepath.Dir(file), "..", "privatedescribeprep", "source_handoff_private.go"), "Close", "SourceHostMailbox")
	if !strings.Contains(lower, "return m.closeErr") || strings.Count(lower, "return m.closeErr") != 2 || !strings.Contains(lower, "m.closeErr = m.io.root.Close()") || !strings.Contains(lower, "m.io == nil || m.io.closed") || strings.Index(lower, "m.io.closed = true") > strings.Index(lower, "m.closeErr = m.io.root.Close()") {
		t.Fatal("ASSERT_MAILBOX_FIRST_CLOSE_STICKY_BINDING")
	}
	upper := readBody(filepath.Join(filepath.Dir(file), "sourcecontinuation", "host_handoff.go"), "CompleteSourceHost", "")
	// The named return is finalized by the actual deferred first close. Timing
	// validation must run AFTER that close, and BOTH errors must erase custody.
	// This finite implementation supplement does not inject a physical close fault.
	deferAt := strings.Index(upper, "defer func() {")
	closeAt := strings.Index(upper, "closeErr := s.mailbox.Close()")
	liveAt := strings.Index(upper, "if !tailDeadline.IsZero() && (retainedCtx == nil || retainedCtx.Err() != nil || !sourceclock.Now(retainedCtx, \"host-close\").Before(tailDeadline))")
	lateAt := strings.Index(upper, "closeErr = errors.Join(closeErr, context.DeadlineExceeded)")
	joinAt := strings.Index(upper, "out, err = out.AfterClose(errors.Join(err, closeErr))")
	endAt := strings.Index(upper, "}()")
	exchangeAt := strings.Index(upper, "s.mailbox.Exchange")
	if deferAt < 0 || closeAt <= deferAt || liveAt <= closeAt || lateAt <= liveAt || joinAt <= lateAt || endAt <= joinAt || exchangeAt <= endAt || strings.Count(upper, "s.mailbox.Close()") != 1 || strings.Count(upper, "out.AfterClose") != 1 {
		t.Fatal("ASSERT_HOST_ACTUAL_CLOSE_JOIN_BINDING")
	}
	t.Log("ASSERT_MAILBOX_FIRST_CLOSE_STICKY_BINDING: PASS; ASSERT_HOST_ACTUAL_CLOSE_JOIN_BINDING: PASS; LIMITED: finite AST supplement, not physical descriptor close-fault execution")
}

// Genuine funded reservation-scope regression: canonical owner and SELF capture;
// no real provider, HOST, acquisition, kernel override, or forged custody. During
// cases use only the existing tagged clock's cancellation boundary; no time shift.
func TestHOSTTimingReservationScopeV2(t *testing.T) {
	for _, mode := range []string{"HOST60", "legacy", "local"} {
		for _, phase := range []string{"live", "before", "during", "caller-during"} {
			t.Run(fmt.Sprintf("%s/%s", mode, phase), func(t *testing.T) {
				canceled := phase != "live"
				if (phase == "during" || phase == "caller-during") && controlledTimingClock == nil {
					t.Skip("covered by ordinary wrapper tagged child")
				}
				m, start, uri := mixedStartFixtureV2(t)
				body := []byte("package fixture\nfunc Root(){ Other() }\nfunc Other(){ Root() }\n")
				if e := os.WriteFile(filepath.Join(start.Profile.Workspace().String(), "root.go"), body, 0644); e != nil {
					t.Fatal(e)
				}
				start.Process.Args = []string{"-test.run=^TestPrivateSourceSELFResponder$", "--", "private-source-self", start.Profile.Workspace().String(), "positive"}
				var owner *inv.PrivateManagedInputOwnerV2
				var root *publication.Root
				var timing *inv.PrivateSourceTimingFrameV2
				deadline := start.Deadline
				if mode == "HOST60" {
					p := inv.PrivateSourceTimingV2{Mode: inv.PrivateSourceHOST60V2, CaptureMS: 15000, PreparationMS: 15000, HOSTMS: 60000, PublicationCleanupMS: 5000}
					env, e := m.PrivateProviderWindowEnvelope(start, sessionruntime.PrivateProviderWindowPlan{References: true})
					if e != nil {
						t.Fatal(e)
					}
					owner, root = inv.ControlledHOSTTimingOwnerForTest(t, p, env.Bounds())
					f, e := p.Bind(time.Now())
					if e != nil {
						t.Fatal(e)
					}
					timing = &f
					start.Deadline = f.CaptureDeadline
					deadline = f.TotalDeadline
				} else if mode == "local" {
					owner, root = inv.ControlledLocalOwnerForTest(t)
				} else {
					owner, root = inv.ControlledRPrepCanonicalOwnerForTest(t)
				}
				A, stopA := context.WithDeadline(context.Background(), deadline)
				defer stopA()
				var duringCancel context.CancelFunc
				fired := false
				if phase == "during" || phase == "caller-during" {
					A = controlledTimingClock(A, func(boundary string) time.Time {
						if boundary == "r-preparation-before-observe" {
							fired = true
							duringCancel()
						}
						return time.Now()
					})
				}
				digest := func(s string) string { return fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(s))) }
				capture := inv.PrivateMixedInputRequestV2{Manager: m, Root: root, Start: start, URI: uri, OccurrenceID: "scope-probe", Revision: "source-controlled-revision", Privacy: "private-source-controlled", OccurrencePolicy: "ordinal", CompositionPolicy: "identity", Line: 1, Character: 6, Kinds: []programcadmission.RelationKind{programcadmission.TypedReferencesSymbol}, Timing: timing}
				mop, e := owner.ReserveMixedInputV2(A, digest("scope-probe-M"), capture)
				if e != nil {
					t.Fatalf("M reserve: %v", e)
				}
				input, e := inv.ExecuteMixedInputV2(A, mop, capture)
				if e != nil {
					t.Fatalf("M execute: %v", e)
				}
				_, md, mdebit, mw, e := inv.ControlledHOSTTimingSnapshotForTest(owner)
				if e != nil || md != "OBSERVED" || mw != 1 {
					t.Fatalf("M original observation missing: %s windows=%d err=%v", md, mw, e)
				}
				q := inv.PrivateSourceRPreparationRequestV1{Input: input, Root: root, ExpectedHead: owner.PreparationHeadV1(), Attempt: digest("scope-probe-P"), Workspace: start.Profile.Workspace().String(), Revision: capture.Revision, RunID: "scope-probe", SeedNamespace: "source-original-schedule", Seed: 7, Deadline: input.SourceDeadlineV1()}
				ioEffects := 0
				if mode == "local" {
					base, e := filepath.EvalSymlinks(t.TempDir())
					if e != nil {
						t.Fatal(e)
					}
					path := filepath.Join(base, "tiny")
					data := []byte("tiny-controlled-artifact\n")
					if e = os.WriteFile(path, data, 0600); e != nil {
						t.Fatal(e)
					}
					pin := describeworker.FilePin{Path: path, SHA256: digest(string(data))}
					c := inv.PrivateSourceLocalConfigV1{Config: describeworker.Config{Worker: pin, Model: pin, Library: pin, SandboxExecutable: pin, SandboxProfile: pin, Grammar: pin, RuntimeIdentity: "controlled-only", AdapterIdentity: "controlled-only", ModelIdentity: "not-a-model", ResponseVersion: describeworker.ResponseVersionV2, Limits: describeworker.Limits{TimeoutMS: 1000, MaxTokens: 1, ContextTokens: 1, StdoutBytes: 8192, StderrBytes: 1024, WorkBytes: 4096, TempBytes: 4096}}}
					for i := range c.Lengths {
						c.Lengths[i] = uint64(len(data))
					}
					c = inv.ControlledLocalIOForTest(c, func(string) error { ioEffects++; return nil })
					q.Local = &c
				}
				B, stopB := context.WithCancel(A)
				defer stopB()
				executionCaller := A
				duringCancel = stopB
				if phase == "caller-during" {
					var stopCaller context.CancelFunc
					executionCaller, stopCaller = context.WithCancel(A)
					defer stopCaller()
					duringCancel = stopCaller
				}
				bd, bok := B.Deadline()
				if !bok || bd.Before(q.Deadline) {
					t.Fatal("B fails to cover original total")
				}
				pop, e := owner.ReserveRPreparationV1(B, q)
				if e != nil {
					t.Fatalf("P reserve: %v", e)
				}
				ph, pd, pdebit, pw, e := inv.ControlledHOSTTimingSnapshotForTest(owner)
				if e != nil || pd != "PENDING" || pw != 2 || pdebit == mdebit {
					t.Fatalf("genuine full P debit missing: %s windows=%d err=%v", pd, pw, e)
				}
				t.Logf("RESERVED M=%s/%d P=%s/%d debit=%v BcoversTotal=%t", md, mw, pd, pw, pdebit, !bd.Before(q.Deadline))
				if phase == "before" {
					stopB()
				}
				if A.Err() != nil || (phase == "before" && B.Err() != context.Canceled) || (phase != "before" && B.Err() != nil) {
					t.Fatal("scope control invalid")
				}
				usable := false
				if mode == "local" {
					prepared, err := inv.ExecuteLocalRPreparationV1(executionCaller, pop, q)
					e = err
					if err == nil {
						_, _, _, _, _, ve := prepared.Values()
						usable = ve == nil
					}
				} else {
					prepared, err := inv.ExecuteRPreparationV1(executionCaller, pop, q)
					e = err
					if err == nil {
						_, _, _, _, _, ve := prepared.Values()
						usable = ve == nil
					}
				}
				ah, ad, adebit, aw, se := inv.ControlledHOSTTimingSnapshotForTest(owner)
				if (phase == "during" || phase == "caller-during") && !fired {
					t.Fatal("ASSERT_P_DURING_BOUNDARY_NOT_REACHED")
				}
				t.Logf("RESULT A=%v B=%v executeErr=%v usable=%t before=%s after=%s headSame=%t debitSame=%t windows=%d ioEffects=%d snapshotErr=%v", A.Err(), B.Err(), e, usable, pd, ad, ah == ph, adebit == pdebit, aw, ioEffects, se)
				if A.Err() != nil || se != nil || adebit != pdebit || aw != pw {
					t.Fatal("ASSERT_ACCOUNTING_CONTROL")
				}
				if canceled {
					if e == nil || usable || ad != "PENDING" || ah != ph || ioEffects != 0 {
						t.Errorf("ASSERT_P_RESERVATION_SCOPE: canceled reservation/caller must refuse under live A before effects/Observe; error=%v usable=%t disposition=%s headSame=%t ioEffects=%d", e, usable, ad, ah == ph, ioEffects)
					}
				} else {
					want := "OBSERVED"
					if mode == "local" {
						want = "PENDING"
					}
					if e != nil || !usable || ad != want {
						t.Errorf("ASSERT_LIVE_B_CONTROL: error=%v usable=%t disposition=%s want=%s", e, usable, ad, want)
					}
				}
				if e != nil {
					if mode == "local" {
						_, again := inv.ExecuteLocalRPreparationV1(A, pop, q)
						if again == nil {
							t.Fatal("ASSERT_P_RETIRED")
						}
					} else {
						_, again := inv.ExecuteRPreparationV1(A, pop, q)
						if again == nil {
							t.Fatal("ASSERT_P_RETIRED")
						}
					}
				}
			})
		}
	}
}

func TestSourceOperatorActualAggregateRefusal(t *testing.T) {
	testSuppliedOp(t, "native-aggregate-refusal")
}

func TestSourceOperatorPhaseFixtures(t *testing.T) {
	for _, mode := range []string{"host-phase-8", "host-phase-16", "host-phase-24"} {
		t.Run(mode, func(t *testing.T) { testSuppliedOp(t, mode) })
	}
}

func TestSourceOperatorOriginalQueryTarget(t *testing.T) {
	testSuppliedOp(t, "original-target")
}

func TestSourceOperatorLocalUnfunded(t *testing.T) { testSuppliedOp(t, "local-invalid") }

func TestConnectedProviderStartSourceV2(t *testing.T) { testSuppliedOp(t, "connected-provider-start") }

type timingHeadObservation struct {
	Head, Disposition string
	Debit             [14]uint64
	Windows           uint64
}

var controlledTimingClock func(context.Context, func(string) time.Time) context.Context

func testSuppliedOp(t *testing.T, binary string) {
	timingCase := ""
	if strings.HasPrefix(binary, "host-timing-") {
		timingCase, binary = strings.TrimPrefix(binary, "host-timing-"), ""
	}
	var timingProfile inv.PrivateSourceTimingV2
	var timingFrame inv.PrivateSourceTimingFrameV2
	var timingShift atomic.Int64
	var wireRemaining atomic.Int64
	var timingFence atomic.Pointer[time.Time]
	var tailFence atomic.Pointer[time.Time]
	var beforeHOST atomic.Pointer[timingHeadObservation]
	connectedStart := binary == "connected-provider-start"
	if connectedStart {
		binary = ""
	}
	localInvalid := binary == "local-invalid"
	if localInvalid {
		binary = ""
	}
	originalTarget := binary == "original-target"
	if originalTarget {
		binary = ""
	}
	phaseMode := ""
	if strings.HasPrefix(binary, "host-phase-") {
		phaseMode, binary = binary, ""
	}
	large := binary == "native-aggregate-refusal"
	if large {
		binary = ""
	}
	var owner *inv.PrivateManagedInputOwnerV2
	var root *publication.Root
	m, start, uri := mixedStartFixtureV2(t)
	if timingCase != "" {
		timingProfile = inv.PrivateSourceTimingV2{Mode: inv.PrivateSourceHOST60V2, CaptureMS: 15000, PreparationMS: 15000, HOSTMS: 60000, PublicationCleanupMS: 5000}
		envelope, ee := m.PrivateProviderWindowEnvelope(start, sessionruntime.PrivateProviderWindowPlan{References: true})
		if ee != nil {
			t.Fatal(ee)
		}
		short := -1
		if timingCase == "future" {
			short = -2
		}
		if strings.HasPrefix(timingCase, "underfund-") {
			if _, ee = fmt.Sscanf(timingCase, "underfund-%d", &short); ee != nil {
				t.Fatal(ee)
			}
		}
		owner, root = inv.ControlledHOSTTimingOwnerForTest(t, timingProfile, envelope.Bounds(), short)
	} else {
		owner, root = inv.ControlledRPrepCanonicalOwnerForTest(t)
	}
	body := []byte("package fixture\nfunc Root(){ Other() }\nfunc Other(){ Root() }\n")
	if e := os.WriteFile(filepath.Join(start.Profile.Workspace().String(), "root.go"), body, 0644); e != nil {
		t.Fatal(e)
	}
	mode := "positive"
	if large {
		mode = "native-large"
	} else if phaseMode != "" {
		mode = phaseMode
	}
	start.Process.Args = []string{"-test.run=^TestPrivateSourceSELFResponder$", "--", "private-source-self", start.Profile.Workspace().String(), mode}
	capture := inv.PrivateMixedInputRequestV2{Manager: m, Root: root, Start: start, URI: uri, OccurrenceID: "source-op", Revision: "source-controlled-revision", Privacy: "private-source-controlled", OccurrencePolicy: "ordinal", CompositionPolicy: "identity", Line: 1, Character: 6, Kinds: []programcadmission.RelationKind{programcadmission.TypedReferencesSymbol}}
	home, e := os.UserHomeDir()
	if e != nil {
		t.Fatal(e)
	}
	dir, e := os.MkdirTemp(home, ".source-op-")
	if e != nil {
		t.Fatal(e)
	}
	defer os.RemoveAll(dir)
	mailbox := filepath.Join(dir, "mailbox")
	outputs := [2]string{filepath.Join(dir, "a"), filepath.Join(dir, "b")}
	for _, p := range []string{mailbox, outputs[0], outputs[1]} {
		if e = os.Mkdir(p, 0700); e != nil {
			t.Fatal(e)
		}
	}
	cfg := host.SourceHostConfig{Protocol: host.SourceHostProtocol, Mailbox: host.AutomaticHandoffConfigPrivateV2{Enabled: true, Directory: mailbox, Invocation: "source-op", InterpreterRole: "canned-role-not-HOST", ReviewerRole: "unused-reviewer", TimeoutMS: 2000, OverallTimeoutMS: 2000, PollMS: 1, Limits: host.AutomaticLimitsPrivateV2{MaxCalls: 1, MaxRequestBytes: 64 << 10, MaxResponseBytes: 16 << 10, MaxTotalBytes: 256 << 10, MaxOutputBytes: 64 << 10}}}
	payload := describeworker.EncodeAdapterResultV2(`{"verdict":"ABSTAINED","target_role":"controlled R source fixture","consumer_need":{"status":"UNRESOLVED","value":""},"provided_behavior":{"value":"fixture body only","consumer_relative":false},"boundary_contribution":"unknown","limitations":["canned role, not authenticated HOST output"]}`)
	payloadPath := filepath.Join(dir, "payload")
	if e = os.WriteFile(payloadPath, payload, 0600); e != nil {
		t.Fatal(e)
	}
	_, file, _, _ := runtime.Caller(0)
	helper := filepath.Join(filepath.Dir(file), "..", "..", "scripts", "private-variable-v2-host-handoff.py")
	parentDeadline := start.Deadline
	if timingCase != "" {
		timingFrame, e = timingProfile.Bind(time.Now())
		if e != nil {
			t.Fatal(e)
		}
		capture.Start.Deadline = timingFrame.CaptureDeadline
		parentDeadline = timingFrame.TotalDeadline
	}
	if connectedStart {
		parentDeadline = time.Now().Add(30 * time.Second)
	} // setup bound only; SOURCE stays15s below
	ctx, cancel := context.WithDeadline(context.Background(), parentDeadline)
	defer cancel()
	if timingCase != "" {
		if controlledTimingClock == nil {
			t.Fatal("controlled clock requires explicit test build tag")
		}
		ctx = controlledTimingClock(ctx, func(boundary string) time.Time {
			if (timingCase == "tail" && boundary == "terminal") || (timingCase == "mailbox-tail" && boundary == "host-close") {
				if fence := tailFence.Load(); fence != nil {
					return *fence
				}
			}
			if timingCase == "equal" && boundary == "host-acceptance" {
				if fence := timingFence.Load(); fence != nil {
					return *fence
				}
			}
			if boundary == "host-complete" {
				now := time.Now().Add(time.Duration(timingShift.Load()))
				tail := now.Add(5 * time.Second)
				tailFence.Store(&tail)
				return now
			}
			if timingCase == "preparation" && boundary == "preparation-complete" {
				return timingFrame.Started.Add(30 * time.Second)
			}
			if timingCase == "capture" && boundary == "capture-complete" {
				return timingFrame.CaptureDeadline
			}
			return time.Now().Add(time.Duration(timingShift.Load()))
		})
	}
	publisherCtx, publisherCancel := context.WithCancel(ctx)
	defer publisherCancel()
	done := make(chan error, 1)
	connectedEntry := make(chan struct{})
	go func() {
		if connectedStart {
			select {
			case <-publisherCtx.Done():
				done <- publisherCtx.Err()
				return
			case <-connectedEntry:
			}
			capPath := filepath.Join(dir, "startup-capability.json")
			capRaw, re := os.ReadFile(capPath)
			if re != nil {
				done <- re
				return
			}
			pin := fmt.Sprintf("sha256:%x", sha256.Sum256(capRaw))
			cmd := exec.CommandContext(publisherCtx, "python3", helper, "connected-wait", "--control", filepath.Join(dir, "control"), "--capability", capPath, "--capability-sha256", pin)
			if raw, re := cmd.CombinedOutput(); re != nil {
				done <- fmt.Errorf("ASSERT_AUTOMATED_CONNECTED_HANDLER: %w %s", re, raw)
				return
			}
		}
		for {
			if _, e := os.Stat(filepath.Join(mailbox, "request-000001.json")); e == nil {
				break
			}
			select {
			case <-publisherCtx.Done():
				done <- publisherCtx.Err()
				return
			case <-time.After(time.Millisecond):
			}
		}
		if timingCase != "" {
			h, d, v, w, se := inv.ControlledHOSTTimingSnapshotForTest(owner)
			if se != nil || d != "OBSERVED" || w != 2 {
				done <- fmt.Errorf("ASSERT_HOST_TIMING_P_OBSERVED_BEFORE_ROLE %s %d %v", d, w, se)
				return
			}
			beforeHOST.Store(&timingHeadObservation{h, d, v, w})
			raw, re := os.ReadFile(filepath.Join(mailbox, "request-000001.json"))
			if re != nil {
				done <- re
				return
			}
			var frame struct{ Binding struct{ Deadline string } }
			if re = json.Unmarshal(raw, &frame); re != nil {
				done <- re
				return
			}
			deadline, re := time.Parse(time.RFC3339Nano, frame.Binding.Deadline)
			if re != nil {
				done <- re
				return
			}
			wireRemaining.Store(int64(time.Until(deadline)))
			timingFence.Store(&deadline)
			delay := 6 * time.Second
			if timingCase == "late" {
				delay = 61 * time.Second
			}
			timingShift.Store(int64(delay))
			if timingCase == "cancel" {
				cancel()
				done <- nil
				return
			}
		}
		cmd := exec.CommandContext(publisherCtx, "python3", helper, "respond", "--mailbox", mailbox, "--sequence", "1", "--payload", payloadPath)
		raw, e := cmd.CombinedOutput()
		if e != nil {
			done <- fmt.Errorf("publisher: %w: %s", e, raw)
			return
		}
		done <- nil
	}()
	q := so.Request{Owner: owner, Capture: capture, CaptureAttempt: fmt.Sprintf("sha256:%x", sha256.Sum256([]byte("op-capture"))), PreparationAttempt: fmt.Sprintf("sha256:%x", sha256.Sum256([]byte("op-prepare"))), RoleAttempt: "source-op-role", RunID: "source-op-run", SeedNamespace: "source-original-schedule", Seed: 7, Contract: sourceStageContract(t), SourceLimits: tp.PrivateSourceLimits{MaxDocumentBytes: 1 << 20, MaxTotalBytes: 8 << 20, MaxWork: 10000, MaxObjects: 1000}, Host: cfg, ResponseLimits: describeworker.SuppliedResponseLimits{MaxAdapterBytes: 4 << 10, MaxSemanticBytes: 1 << 10}, OutputRoots: outputs}
	if connectedStart || timingCase != "" {
		q.SourceLimits = tp.PrivateSourceLimits{MaxDocumentBytes: 16 << 10, MaxTotalBytes: 64 << 10, MaxWork: 4096, MaxObjects: 16}
		q.ResponseLimits = describeworker.SuppliedResponseLimits{MaxAdapterBytes: 8192, MaxSemanticBytes: 4096}
		q.Host.Mailbox.TimeoutMS = 5000
		q.Host.Mailbox.OverallTimeoutMS = 5000
		q.Host.Mailbox.PollMS = 10
		q.Host.Mailbox.Limits.MaxTotalBytes = 147456
		cl := q.Contract.CaptureLimits()
		cl.MaxSourceBytes = 16384
		cl.MaxTotalSourceBytes = 65536
		cl.MaxBindings = 64
		cl.MaxWork = 4096
		policy := q.Contract.PacketSourcePolicy()
		policy.MaxBytes = 65536
		policy.MaxRanges = 64
		policy.MaxObjects = 16
		policy.MaxWork = 4096
		d := "sha256:" + strings.Repeat("1", 64)
		q.Contract, e = cc.NewContinuationContract(cc.ContractInput{ContinuationID: d, ProfileID: d, WorkerPinID: d, ProgramCSeed: 7, CaptureLimits: cl, PacketSourcePolicy: policy, PacketResolveLimits: retainedprojection.ResolveLimits{MaxDistinctObjects: 16, MaxUniqueSourceBytes: 65536, MaxLogicalSelections: 64}, PacketMaxResponseBytes: 131072, DescribeDeadlineMS: 5000, RetryPolicy: cc.RetryPolicy{MaxAttemptsPerRequest: 1}})
		if e != nil {
			t.Fatal(e)
		}
	}
	if timingCase != "" {
		q.Timing = &timingFrame
		if timingCase == "forged" {
			timingFrame.TotalDeadline = timingFrame.TotalDeadline.Add(time.Nanosecond)
		}
		if timingCase == "incompatible" {
			timingProfile.PublicationCleanupMS++
			timingFrame, e = timingProfile.Bind(timingFrame.Started)
			if e != nil {
				t.Fatal(e)
			}
		}
		q.SourceLimits = tp.PrivateSourceLimits{MaxDocumentBytes: 16 << 10, MaxTotalBytes: 64 << 10, MaxWork: 4096, MaxObjects: 16}
		q.Host.Mailbox.TimeoutMS = 60000
		q.Host.Mailbox.OverallTimeoutMS = 60000
		if timingCase == "legacy" {
			q.Timing = nil
			q.Host.Mailbox.TimeoutMS = 5000
			q.Host.Mailbox.OverallTimeoutMS = 5000
		}
	}
	if localInvalid {
		q.Local = &inv.PrivateSourceLocalConfigV1{}
	}
	_, beforeRefusals := inv.ControlledRPrepDebitForTest(t, owner)
	badAlias := q
	badAlias.OutputRoots[1] = badAlias.OutputRoots[0]
	if _, e := so.Run(ctx, badAlias); e == nil {
		publisherCancel()
		<-done
		t.Fatal("ASSERT_OP_ALIAS_BEFORE_CAPTURE")
	}
	cancelled, cancelledStop := context.WithCancel(ctx)
	cancelledStop()
	if _, e := so.Run(cancelled, q); e == nil {
		publisherCancel()
		<-done
		t.Fatal("ASSERT_OP_CANCEL_BEFORE_CAPTURE")
	}
	_, afterRefusals := inv.ControlledRPrepDebitForTest(t, owner)
	if beforeRefusals != afterRefusals || m.Census().Sessions != 0 {
		publisherCancel()
		<-done
		t.Fatal("ASSERT_OP_REFUSAL_ZERO_EFFECT")
	}
	if timingCase == "replacement" || timingCase == "scope-cancel" {
		q.Capture.Timing = q.Timing
		op, err := owner.ReserveMixedInputV2(ctx, q.CaptureAttempt, q.Capture)
		if err == nil {
			drive := ctx
			if timingCase == "replacement" {
				drive = context.WithValue(ctx, struct{}{}, true)
			}
			input, xe := inv.ExecuteMixedInputV2(drive, op, q.Capture)
			err = xe
			if timingCase == "scope-cancel" && xe == nil {
				cancel()
				fresh, stop := context.WithDeadline(context.Background(), timingFrame.TotalDeadline)
				prep := inv.PrivateSourceRPreparationRequestV1{Input: input, Root: root, ExpectedHead: owner.PreparationHeadV1(), Attempt: q.PreparationAttempt, Workspace: start.Profile.Workspace().String(), Revision: q.Capture.Revision, RunID: q.RunID, SeedNamespace: q.SeedNamespace, Seed: q.Seed, Deadline: input.SourceDeadlineV1()}
				_, err = owner.ReserveRPreparationV1(fresh, prep)
				stop()
			}
		}
		publisherCancel()
		<-done
		_, d, v, w, se := inv.ControlledHOSTTimingSnapshotForTest(owner)
		want := "PENDING"
		if timingCase == "scope-cancel" {
			want = "OBSERVED"
		}
		if err == nil || se != nil || d != want || w != 1 || v[0] == 0 || m.Census().Workers != 0 || (timingCase == "replacement" && m.Census().Sessions != 0) {
			t.Fatal("ASSERT_HOST_TIMING_ORIGINAL_SCOPE", timingCase, err, se, d, w, m.Census())
		}
		t.Log("ASSERT_HOST_TIMING_ORIGINAL_SCOPE PASS", timingCase, "actual head", d, "windows", w)
		return
	}
	var out so.Outcome
	var runErr error
	if binary == "" {
		if connectedStart {
			if e = os.Mkdir(filepath.Join(dir, "control"), 0700); e != nil {
				t.Fatal(e)
			}
			pending, providerPath, controlOwner := inv.ControlledProviderLaunchForTest(t, dir, owner)
			// The test peer carries only the live startup packet; genuine CONTROL
			// custody and the actual Local=nil SOURCE owner remain in this Go
			// invocation. No parent test writes START or resumes the HOST handler.
			digest := func(raw []byte) string { return fmt.Sprintf("sha256:%x", sha256.Sum256(raw)) }
			d := digest([]byte("controlled-start-config"))
			launchDeadline := parentDeadline.UTC().Format(time.RFC3339Nano)
			ready, _ := json.Marshal(map[string]any{"Schema": "private-connected-source-ready.v2", "Invocation": cfg.Mailbox.Invocation, "ConfigPin": d, "SourceTemplatePin": d, "ControlReceipt": "fixed-provider-control/v2 /Users/schwa/.local/bin/gopls v0.23.0 " + digest([]byte("controlled transport head")), "LaunchDeadline": launchDeadline})
			ready = append(ready, '\n')
			if e = pending.PublishStartReadyV2(ready); e != nil {
				t.Fatal(e)
			}
			config, _ := json.Marshal(map[string]any{"Schema": "private-source-start.v2", "Invocation": cfg.Mailbox.Invocation, "Base": dir, "Pin": d, "LaunchDeadline": launchDeadline, "ProducerSession": "controlled-producer", "ReviewerSession": "controlled-reviewer", "Source": map[string]any{"Host": cfg}, "Ready": string(ready)})
			configPath := filepath.Join(dir, "startup-config.json")
			if e = os.WriteFile(configPath, config, 0600); e != nil {
				t.Fatal(e)
			}
			cap, _ := json.Marshal(map[string]any{"Schema": "private-source-start-capability.v1", "ConfigSHA256": digest(config), "ConfigPin": d, "Invocation": cfg.Mailbox.Invocation, "ProducerSession": "controlled-producer", "ReviewerSession": "controlled-reviewer", "RoleID": cfg.Mailbox.InterpreterRole, "Mailbox": mailbox, "Control": filepath.Join(dir, "control"), "SourceTotalMS": 15000, "Nonce": strings.Repeat("a", 64), "LaunchDeadline": launchDeadline, "ApproveParentStart": true})
			capPath := filepath.Join(dir, "startup-capability.json")
			if e = os.WriteFile(capPath, cap, 0600); e != nil {
				t.Fatal(e)
			}
			peer := filepath.Join(dir, "controlled-startup-peer")
			peerRaw := []byte(`#!/usr/bin/env python3
import json,os,sys,time
c=json.load(open(sys.argv[3]));w=int(sys.argv[5]);os.write(w,c['Ready'].encode());os.close(w)
end=time.monotonic()+30
while not os.path.exists(c['Base']+'/operator-finished.json') and time.monotonic()<end:time.sleep(.001)
assert os.path.exists(c['Base']+'/operator-finished.json')
`)
			if e = os.WriteFile(peer, peerRaw, 0700); e != nil {
				t.Fatal(e)
			}
			bridge := filepath.Join(filepath.Dir(helper), "..", ".pi", "skills", "private-host-streaming-application", "scripts", "activation-bridge.py")
			command := exec.CommandContext(ctx, "python3", bridge, "source", peer, configPath, digest(config), capPath, digest(cap))
			var bridgeOut, bridgeErr bytes.Buffer
			command.Stdout, command.Stderr = &bridgeOut, &bridgeErr
			if e = command.Start(); e != nil {
				t.Fatal(e)
			}
			finished := false
			finishBridge := func() {
				if finished {
					return
				}
				finished = true
				_ = os.WriteFile(filepath.Join(dir, "operator-finished.json"), []byte("{}"), 0600)
				if re := command.Wait(); re != nil {
					t.Errorf("ASSERT_AUTOMATED_CONNECTED_OWNER: %v %s %s", re, bridgeOut.Bytes(), bridgeErr.Bytes())
				}
			}
			defer finishBridge()
			// Wait only for the transport endpoint before the ONE role launch;
			// this is test orchestration, not parent readiness/START coordination.
			for i := 0; i < 10000; i++ {
				if _, re := os.Stat(filepath.Join(dir, "control", "source-ack.fifo")); re == nil {
					break
				}
				if ctx.Err() != nil {
					t.Fatal(ctx.Err())
				}
				time.Sleep(time.Millisecond)
			}
			close(connectedEntry)
			var got []byte
			var re error
			for i := 0; i < 10000; i++ {
				got, re = pending.ReadStartMetadataV2("control/start.json")
				if re == nil {
					break
				}
				if !errors.Is(re, os.ErrNotExist) {
					t.Fatal("ASSERT_CONNECTED_SIGNAL", re)
				}
				select {
				case <-ctx.Done():
					t.Fatal(ctx.Err())
				case <-time.After(time.Millisecond):
				}
			}
			if re != nil {
				t.Fatal("ASSERT_CONNECTED_SIGNAL", re)
			}
			var actualStart struct {
				Operation, Invocation                                   string
				ProducerWaiterActive                                    bool
				DriverReadySHA256, ParentReadySHA256, SourceTemplatePin string
			}
			parent, re := os.ReadFile(filepath.Join(dir, "control", "parent-ready.json"))
			if re != nil || json.Unmarshal(got, &actualStart) != nil || actualStart.Operation != "START_CONFIRM" || actualStart.Invocation != cfg.Mailbox.Invocation || !actualStart.ProducerWaiterActive || actualStart.DriverReadySHA256 != digest(ready) || actualStart.ParentReadySHA256 != digest(parent) || actualStart.SourceTemplatePin != d {
				t.Fatal("ASSERT_AUTOMATED_CONNECTED_TYPED_JOIN", re)
			}
			t.Log("ASSERT_AUTOMATED_CONNECTED_TYPED_JOIN PASS; one handler invocation, no manual parent START")
			firstStart := time.Now() // fixed before consume; no re-invocation clock
			if e = pending.ConsumeStartV2(got); e != nil {
				t.Fatal(e)
			}
			expiry, re := pending.SourceAuthorizationExpiryV2(ctx, owner)
			if re != nil {
				t.Fatal(re)
			}
			deadline := firstStart.Add(15 * time.Second)
			if expiry.Before(deadline) {
				deadline = expiry
			}
			if !deadline.After(firstStart) || q.Local != nil {
				t.Fatal("ASSERT_CONNECTED_ORIGINAL_EXPIRY")
			}
			q.Capture.Start.Deadline = deadline
			sourceCtx, sourceCancel := context.WithDeadline(ctx, deadline)
			defer sourceCancel()
			ctx = sourceCtx
			t.Log("ASSERT_CONNECTED_ORIGINAL_CLOCK PASS; chosen once min(start+15s, original expiry)")
			if e = pending.FinishProviderLaunchV2(ctx, providerPath); e != nil {
				t.Fatal("ASSERT_CONNECTED_PAID_FRESHNESS", e)
			}
			if e = pending.ConsumeStartV2(got); e == nil {
				t.Fatal("ASSERT_CONNECTED_NO_SECOND_START")
			}
			disp, _ := inv.ControlledRPrepDebitForTest(t, controlOwner)
			if disp != "OBSERVED" {
				t.Fatal("ASSERT_CONNECTED_CONTROL_OBSERVED", disp)
			}
			t.Log("ASSERT_CONNECTED_CONTROL_SIGNAL_FRESHNESS PASS; tiny provider never executed; next actual SELF and canned mailbox")
		}
		out, runErr = so.Run(ctx, q)
	} else {
		job, head := inv.ControlledRPrepCLIJobForTest(t, owner)
		d := "sha256:" + strings.Repeat("1", 64)
		cfg := sourceOpCLIConfig{Schema: "private-source-host-application.v1", Mode: "EXECUTE", AccountingRoot: root.Path(), CanonicalJob: job, ExpectedHead: head, Verification: inv.PrivateManagedInputVerificationV2{Records: 4096, Bytes: 64 << 20, Work: 16384}, Profile: runtimeprofile.Selector{TrustDomain: "test", Workspace: start.Profile.Workspace().String(), Profile: "go", EnvironmentReference: "local"}, LanguageID: "go", Provider: "lsp", ProcessPath: start.Process.Path, ProcessArgs: start.Process.Args, Workspace: start.Profile.Workspace().String(), Revision: capture.Revision, Privacy: capture.Privacy, URI: uri, OccurrenceID: capture.OccurrenceID, Line: capture.Line, Character: capture.Character, CaptureAttempt: q.CaptureAttempt, PreparationAttempt: q.PreparationAttempt, RoleAttempt: q.RoleAttempt, RunID: q.RunID, SeedNamespace: q.SeedNamespace, Seed: q.Seed, Deadline: start.Deadline, Contract: cc.ContractInput{ContinuationID: d, ProfileID: d, WorkerPinID: d, ProgramCSeed: 9, CaptureLimits: q.Contract.CaptureLimits(), PacketSourcePolicy: q.Contract.PacketSourcePolicy(), PacketResolveLimits: q.Contract.PacketResolveLimits(), PacketMaxResponseBytes: q.Contract.PacketMaxResponseBytes(), DescribeDeadlineMS: q.Contract.DescribeDeadlineMS(), RetryPolicy: cc.RetryPolicy{MaxAttemptsPerRequest: 1}}, SourceLimits: q.SourceLimits, Host: q.Host, ResponseLimits: q.ResponseLimits, OutputRoots: outputs}
		raw, _ := json.Marshal(cfg)
		var fields map[string]json.RawMessage
		_ = json.Unmarshal(raw, &fields)
		raw, _ = json.Marshal(fields)
		cfg.Pin = fmt.Sprintf("sha256:%x", sha256.Sum256(raw))
		raw, _ = json.Marshal(cfg)
		config := filepath.Join(dir, "config.json")
		if e = os.WriteFile(config, raw, 0600); e != nil {
			publisherCancel()
			<-done
			t.Fatal(e)
		}
		command := exec.CommandContext(ctx, binary, "private-host-application", "source-describe", config)
		var stdout, stderr bytes.Buffer
		command.Stdout = &stdout
		command.Stderr = &stderr
		runErr = command.Run()
		if runErr != nil {
			runErr = fmt.Errorf("%w stderr=%s stdout=%s", runErr, stderr.Bytes(), stdout.Bytes())
		} else {
			runErr = json.Unmarshal(stdout.Bytes(), &out)
		}
	}
	publisherCancel()
	publisherErr := <-done // always drain before Fatal
	if timingCase != "" {
		disposition, debit := inv.ControlledRPrepDebitForTest(t, owner)
		if pre := beforeHOST.Load(); pre != nil {
			h, d, v, w, se := inv.ControlledHOSTTimingSnapshotForTest(owner)
			if se != nil || h != pre.Head || d != pre.Disposition || v != pre.Debit || w != pre.Windows {
				t.Fatal("ASSERT_HOST_TIMING_TERMINAL_LEDGER_TRUTH", se, d, w)
			}
			t.Log("ASSERT_HOST_TIMING_TERMINAL_LEDGER_TRUTH PASS; genuine pre-HOST OBSERVED head/debit/windows2 unchanged")
		}
		if timingCase == "forged" || timingCase == "incompatible" || timingCase == "future" || strings.HasPrefix(timingCase, "underfund-") {
			if runErr == nil || disposition != "READY" || debit != [14]uint64{} || out.Published != [2]bool{} || m.Census().Workers != 0 || m.Census().Sessions != 0 {
				t.Fatal("ASSERT_HOST_TIMING_PREFLIGHT", timingCase, out, runErr, disposition, debit, m.Census())
			}
			t.Log("ASSERT_HOST_TIMING_PREFLIGHT PASS", timingCase)
			return
		}
		if timingCase == "preparation" {
			_, d, _, w, se := inv.ControlledHOSTTimingSnapshotForTest(owner)
			if runErr == nil || se != nil || d != "OBSERVED" || w != 2 || out.Published != [2]bool{} || beforeHOST.Load() != nil || m.Census().Workers != 0 {
				t.Fatal("ASSERT_HOST_TIMING_PREPARATION", out, runErr, d, w)
			}
			t.Log("ASSERT_HOST_TIMING_PREPARATION PASS; P observed, no HOST publication")
			return
		}
		if timingCase == "capture" {
			if runErr == nil || disposition != "PENDING" || debit[0] == 0 || out.Published != [2]bool{} || m.Census().Workers != 0 {
				t.Fatal("ASSERT_HOST_TIMING_CAPTURE", out, runErr, disposition, debit)
			}
			t.Log("ASSERT_HOST_TIMING_CAPTURE PASS")
			return
		}
		if timingCase == "mailbox-tail" {
			if !errors.Is(runErr, context.DeadlineExceeded) || disposition != "OBSERVED" || out.Published != [2]bool{} || out.Readback != [2]bool{} || m.Census().Workers != 0 {
				t.Fatal("ASSERT_HOST_MAILBOX_CLOSE_TAIL", out, runErr, disposition)
			}
			t.Log("ASSERT_HOST_MAILBOX_CLOSE_TAIL PASS; actual deferred close equality refuses custody, genuine P remains OBSERVED")
			return
		}
		if timingCase == "legacy" || timingCase == "late" || timingCase == "equal" || timingCase == "cancel" {
			if runErr == nil || disposition != "OBSERVED" || debit[0] == 0 || out.Published != [2]bool{} || m.Census().Workers != 0 {
				t.Fatal("ASSERT_HOST_TIMING_REJECTION", timingCase, out, runErr, disposition, debit)
			}
			t.Log("ASSERT_HOST_TIMING_REJECTION PASS", timingCase)
			return
		}
		if timingCase == "tail" {
			if runErr == nil || disposition != "OBSERVED" || out.Disposition != "FAILED_RETAINED" || out.Published != [2]bool{true, true} || out.Readback != [2]bool{true, true} || m.Census().Workers != 0 {
				t.Fatal("ASSERT_HOST_TIMING_TAIL", out, runErr, disposition)
			}
			t.Log("ASSERT_HOST_TIMING_TAIL PASS")
			return
		}
		if runErr != nil || publisherErr != nil || wireRemaining.Load() < int64(50*time.Second) || disposition != "OBSERVED" {
			t.Fatal("ASSERT_HOST_TIMING_CONNECTED", out, runErr, publisherErr, time.Duration(wireRemaining.Load()), disposition)
		}
		t.Log("ASSERT_HOST_TIMING_CONNECTED PASS; controlled 6s HOST delay, original lifetime retained, actual native pair below")
	}
	if localInvalid {
		if runErr == nil || out.NativeID != "" || out.Published != [2]bool{} {
			t.Fatal("ASSERT_LOCAL_BRANCH_REFUSES_BEFORE_DELIVERY", out, runErr)
		}
		t.Log("ASSERT_LOCAL_BRANCH_REFUSES_BEFORE_DELIVERY: PASS")
		return
	}
	if phaseMode != "" && runErr != nil {
		if !errors.Is(runErr, so.ErrNativeAggregate) && !strings.Contains(runErr.Error(), "host-phase=") {
			t.Fatal("ASSERT_HOST_PHASE_FIXTURE_BOUNDED_FAILURE", phaseMode, runErr)
		}
		if out.NativeID != "" || out.Published != [2]bool{} || out.Readback != [2]bool{} || m.Census().Workers != 0 {
			t.Fatal("ASSERT_HOST_PHASE_FIXTURE_NO_OUTPUT", phaseMode, out, m.Census())
		}
		if _, e := os.Stat(filepath.Join(mailbox, "preparation.json")); !os.IsNotExist(e) {
			t.Fatal("ASSERT_HOST_PHASE_FIXTURE_BEFORE_MAILBOX", e)
		}
		if errors.Is(runErr, so.ErrNativeAggregate) {
			t.Logf("LIMITED_PHASE_FIXTURE: %s native bound refuses first; no connected HOST phase separation; %v", phaseMode, runErr)
		} else {
			t.Logf("ASSERT_HOST_PHASE_FIXTURE_BEFORE_MAILBOX: PASS; genuine paid opaque R; %s; %v", phaseMode, runErr)
		}
		return
	}
	if large {
		if !errors.Is(runErr, so.ErrNativeAggregate) || out.NativeID != "" || out.Published != [2]bool{} || out.Readback != [2]bool{} {
			t.Fatal("ASSERT_NATIVE_ACTUAL_UPSTREAM_REFUSAL", out, runErr, publisherErr)
		}
		if _, e := os.Stat(filepath.Join(mailbox, "preparation.json")); !os.IsNotExist(e) {
			t.Fatal("ASSERT_NATIVE_BEFORE_HOST", e)
		}
		for _, p := range outputs {
			entries, e := os.ReadDir(p)
			if e != nil || len(entries) != 0 {
				t.Fatal("ASSERT_NATIVE_NO_PUBLICATION", e)
			}
		}
		disposition, _ := inv.ControlledRPrepDebitForTest(t, owner)
		if disposition != "OBSERVED" || out.Counts.Intents == 0 || m.Census().Workers != 0 {
			t.Fatal("ASSERT_NATIVE_ACTUAL_PAID_R", disposition, out, m.Census())
		}
		t.Logf("ASSERT_NATIVE_ACTUAL_UPSTREAM_REFUSAL: PASS; ASSERT_NATIVE_BEFORE_HOST: PASS; ASSERT_NATIVE_NO_PUBLICATION: PASS; ASSERT_NATIVE_ACTUAL_PAID_R: PASS; same original caps/source/seed; genuine paid opaque multi-occurrence R input; %v", runErr)
		return
	}
	if runErr != nil || publisherErr != nil {
		t.Fatal("ASSERT_OP_CONNECTED", out, runErr, publisherErr)
	}
	if originalTarget {
		raw, readErr := os.ReadFile(filepath.Join(mailbox, "request-000001.json"))
		if readErr != nil || !bytes.Contains(raw, []byte("func Root(){ Other() }")) || bytes.Contains(raw, []byte("func Other(){ Root() }")) {
			t.Fatal("ASSERT_ORIGINAL_QUERY_HOST_BODY_ONLY", readErr, string(raw))
		}
		t.Log("ASSERT_ORIGINAL_QUERY_HOST_BODY_ONLY: PASS (actual paid R SELF and canned mailbox; not real HOST)")
	}
	if phaseMode != "" {
		t.Logf("LIMITED_PHASE_FIXTURE: %s all original phase/native bounds fit; no connected HOST phase separation", phaseMode)
	}
	if out.Disposition != "SOURCE_PROVISIONAL_DELIVERED_PAIR" || out.NativeID == "" || out.Published != [2]bool{true, true} || out.Readback != [2]bool{true, true} || m.Census().Workers != 0 {
		t.Fatal("ASSERT_OP_NATIVE_PAIR", out)
	}
	a, e := os.ReadFile(filepath.Join(outputs[0], out.NativeID))
	if e != nil {
		t.Fatal(e)
	}
	b, e := os.ReadFile(filepath.Join(outputs[1], out.NativeID))
	if e != nil || !bytes.Equal(a, b) {
		t.Fatal("ASSERT_OP_EXACT_PAIR", e)
	}
	var native map[string]json.RawMessage
	if e = json.Unmarshal(a, &native); e != nil || string(native["schema"]) != `"SOURCE_NATIVE_V1"` || len(native["machine"]) == 0 || len(native["human_inventory"]) == 0 {
		t.Fatal("ASSERT_OP_NATIVE_SCHEMA", e)
	}
	if string(native["origin"]) != `"SUPPLIED_TERMINAL_RESPONSE_ONLY"` || string(native["execution"]) != `"NO_PROCESS_OR_HOST_EXECUTION_VERIFIED"` || string(native["host_identity"]) != `"HOST_OBSERVED_NOT_AUTHENTICATED"` || string(native["authority"]) != "0" || string(native["accepted"]) != "false" || string(native["identity"]) != `"UNRESOLVED"` || string(native["completeness"]) != `"UNKNOWN"` || string(native["absence"]) != `"PROHIBITED"` {
		t.Fatal("ASSERT_OP_NATIVE_CEILING")
	}
	disposition, debit := inv.ControlledRPrepDebitForTest(t, owner)
	if disposition != "OBSERVED" || debit[0] == 0 || debit[5] == 0 {
		t.Fatal("ASSERT_OP_CUMULATIVE_DEBIT", disposition, debit)
	}
	t.Logf("ASSERT_OP_CONNECTED: PASS; ASSERT_OP_EXACT_PAIR: PASS; original seven-mode schedule R-only; counts=%+v debit=%v native=%s", out.Counts, debit, out.NativeID)
}

func sourceStageContract(t *testing.T) cc.ContinuationContract {
	return sourceStageResponseContract(t, 1<<20)
}
func sourceStageResponseContract(t *testing.T, responseCap int) cc.ContinuationContract {
	t.Helper()
	d := "sha256:" + strings.Repeat("1", 64)
	c, e := cc.NewContinuationContract(cc.ContractInput{ContinuationID: d, ProfileID: d, WorkerPinID: d, ProgramCSeed: 9, CaptureLimits: v5sourcesnapshotv3.Limits{MaxArtifactBytes: 8 << 20, MaxParentBytes: 8 << 20, MaxReceipts: 100, MaxSourceBytes: 1 << 20, MaxTotalSourceBytes: 8 << 20, MaxBindings: 1000, MaxWork: 10000}, PacketSourcePolicy: sourceprojection.Policy{PolicyID: "controlled-source", BodyRequested: true, MaxBytes: 1 << 20, MaxRanges: 100, MaxObjects: 100, MaxWork: 1000, EnforceLimits: true}, PacketResolveLimits: retainedprojection.ResolveLimits{MaxDistinctObjects: 100, MaxUniqueSourceBytes: 8 << 20, MaxLogicalSelections: 100}, PacketMaxResponseBytes: responseCap, DescribeDeadlineMS: 1000, RetryPolicy: cc.RetryPolicy{MaxAttemptsPerRequest: 1}})
	if e != nil {
		t.Fatal(e)
	}
	return c
}
func sourceStageFixture(t *testing.T, mode string, uncertain bool) (inv.PrivateManagedGroupingInput, cc.PrivateAllModeHandoff, cc.ContinuationContract, sc.PrivateAllModeSourcesRequest) {
	t.Helper()
	owner, root := inv.ControlledManagedInputOwnerForTest(t, false)
	m, start, uri := mixedStartFixtureV2(t)
	if e := os.WriteFile(filepath.Join(start.Profile.Workspace().String(), "root.go"), []byte("package fixture\nfunc Root(){ Other() }\nfunc Other(){ Root() }\n"), 0644); e != nil {
		t.Fatal(e)
	}
	start.Process.Args = []string{"-test.run=^TestPrivateSourceSELFResponder$", "--", "private-source-self", start.Profile.Workspace().String(), mode}
	q := inv.PrivateMixedInputRequestV2{Manager: m, Root: root, Start: start, URI: uri, OccurrenceID: "source-controlled", Revision: "source-controlled-revision", Privacy: "private-source-controlled", OccurrencePolicy: "ordinal", CompositionPolicy: "identity", Line: 1, Character: 6, Kinds: []programcadmission.RelationKind{programcadmission.TypedCalls, programcadmission.TypedReferencesSymbol, programcadmission.TypedResolvesToDefinition}}
	op, e := owner.ReserveMixedInputV2(context.Background(), fmt.Sprintf("sha256:%x", sha256.Sum256([]byte("source-"+mode))), q)
	if e != nil {
		t.Fatal(e)
	}
	if uncertain {
		inv.ControlledMixedObserveUncertainForTest(owner)
	}
	input, e := inv.ExecuteMixedInputV2(context.Background(), op, q)
	if uncertain {
		if _, proofErr := input.SourceOriginalsV1(); proofErr == nil {
			t.Fatal("ASSERT_SOURCE_UNCERTAIN_NO_CAP")
		}
		if e == nil {
			t.Fatal("ASSERT_SOURCE_UNCERTAIN_PRODUCER")
		}
		t.Log("ASSERT_SOURCE_UNCERTAIN_NO_CAP: PASS")
		return input, cc.PrivateAllModeHandoff{}, cc.ContinuationContract{}, sc.PrivateAllModeSourcesRequest{}
	}
	if e != nil {
		t.Fatal("ASSERT_SOURCE_ACTUAL_PRODUCER", e)
	}
	if m.Census().Workers != 0 {
		t.Fatal("ASSERT_SOURCE_PRODUCER_JOINED")
	}
	originals, e := input.SourceOriginalsV1()
	if e != nil {
		t.Fatal("ASSERT_SOURCE_CONFIRMED_ORIGINALS", e)
	}
	slots, entries, _ := sourceStageEntries(t, input.Input, []string{"RD", "C", "CRD", "D", "CD", "R", "CR"}, 0, []byte("source selection\n"), "CONTROLLED_SOURCE_MODE")
	h, e := cc.NewPrivateAllModeHandoff(start.Profile.Workspace().String(), q.Revision, slots, entries)
	if e != nil {
		t.Fatal(e)
	}
	raw := []byte("package fixture\nfunc Root(){ Other() }\nfunc Other(){ Root() }\n")
	request := sc.PrivateAllModeSourcesRequest{PolicyID: "controlled-source", Limits: tp.PrivateSourceLimits{MaxDocumentBytes: 1 << 20, MaxTotalBytes: 8 << 20, MaxWork: 10000, MaxObjects: 1000}, Deadline: input.SourceDeadlineV1(), Originals: []inv.PrivateProducerOriginals{originals}, Objects: []sourceobject.Object{{Identity: sourceobject.Identity{Digest: fmt.Sprintf("sha256:%x", sha256.Sum256(raw)), ByteLength: uint64(len(raw))}, Bytes: raw}}}
	return input, h, sourceStageContract(t), request
}

// Existing controlled evaluation setup only. Neither the scalar accessor nor
// the binder invokes this helper, evaluation, fresh admission or a provider.
func sourceStageEntries(t *testing.T, input rg.AdmittedInput, modes []string, start int, selector []byte, rationale string) ([]cc.PrivateAllModeSlot, []cc.PrivateAllModeEntry, uint64) {
	t.Helper()
	var slots []cc.PrivateAllModeSlot
	var entries []cc.PrivateAllModeEntry
	var payloadMinimum uint64
	maxString := 256
	if len(rationale) > maxString {
		maxString = 4096
	}
	for index, mode := range modes {
		i := start + index
		var rules []rg.GroupingPolicyRule
		for n, k := range []string{"CALLS", "REFERENCES_SYMBOL", "RESOLVES_TO_DEFINITION"} {
			r := rg.GroupingPolicyRule{RelationKind: k, Action: "EXCLUDE", RationaleCode: rationale, Weight: rg.Rational{Numerator: 0, Denominator: 1}}
			if strings.ContainsRune(mode, []rune{'C', 'R', 'D'}[n]) {
				r.Action = "INCLUDE"
				r.Weight.Numerator = 1
			}
			rules = append(rules, r)
		}
		policy, e := rg.BuildGroupingPolicy(rg.GroupingPolicyRequest{PolicyID: "source-" + mode, Epoch: 1, EligibleKinds: []string{"CALLS", "REFERENCES_SYMBOL", "RESOLVES_TO_DEFINITION"}, Rules: rules, Limits: rg.GroupingPolicyLimits{MaxRules: 3, MaxCanonicalBytes: 1 << 16, MaxStringBytes: maxString, MaxAbsoluteNumerator: 1, MaxDenominator: 1, MaxCap: 1}})
		if e != nil {
			t.Fatal(e)
		}
		slot := cc.PrivateAllModeSlot{Ordinal: i, Mode: mode, RunID: fmt.Sprintf("source-run-%d", i), SeedNamespace: "source-original-schedule", Seed: 7}
		c, e := pc.NewGroupingEvaluationCasePrivate(pc.GroupingEvaluationCaseRequestPrivate{CaseID: fmt.Sprintf("source-case-%d", i), RunID: slot.RunID, SelectorID: "source-selection", SelectorBytes: selector, Seed: 7, Input: input, Policy: policy, ProjectionLimits: rg.CompositionLimits{MaxRawBytes: 1 << 20, MaxOccurrences: 128, MaxPairContributions: 128, MaxPairs: 128, MaxCanonicalBytes: 1 << 20, MaxStringBytes: 4096, MaxDenominator: 1 << 20, MaxAccumulator: 1 << 20}})
		if e != nil {
			t.Fatal(e)
		}
		result, e := pc.EvaluateGroupingCasePrivate(c)
		if e != nil {
			t.Fatal(e)
		}
		// Fixture expectation from the same already-created opaque values, not
		// serialized DTOs or CanonicalBytes clones. This is outside sizing/binding.
		binding, e := pc.BindGroupingPairPrivate(c, result, input)
		if e != nil {
			t.Fatal(e)
		}
		payloadMinimum += 2 * uint64(input.ByteLength()+policy.ByteLength()+binding.Case.ByteLength+result.ByteLength()+c.Projection().ByteLength())
		slots = append(slots, slot)
		entries = append(entries, cc.PrivateAllModeEntry{Slot: slot, Disposition: cc.PrivateAllModeReady, Lineage: fmt.Sprintf("source-lineage-%d", i), Input: input, Policy: policy, Case: c, Result: result})
	}
	return slots, entries, payloadMinimum
}

func TestSuppliedRetainedHandoff(t *testing.T) {
	served, baseH, contract, q := sourceStageFixture(t, "positive", false)
	absent, _, _, _ := sourceStageFixture(t, "positive", false)
	if served.Input.ID() == absent.Input.ID() || len(q.Originals) != 1 || q.Originals[0].InputID() != served.Input.ID() {
		t.Fatal("ASSERT_ABSENT_READY_SOURCE_SETUP")
	}
	ctx, cancel := context.WithDeadline(context.Background(), q.Deadline)
	defer cancel()
	baseSlots, baseEntries, baseMinimum := sourceStageEntries(t, served.Input, []string{"RD", "C", "CRD", "D", "CD", "R", "CR"}, 0, []byte("source selection\n"), "CONTROLLED_SOURCE_MODE")
	rebuilt, e := cc.NewPrivateAllModeHandoff(qWorkspace(q), "source-controlled-revision", baseSlots, baseEntries)
	if e != nil || rebuilt.ID() != baseH.ID() {
		t.Fatal("ASSERT_HANDOFF_IDENTITY_UNCHANGED", e)
	}
	baseSize, ok := baseH.RetainedPayloadUpper()
	if !ok || baseSize < baseMinimum {
		t.Fatal("ASSERT_HANDOFF_ALL_READY_PAYLOADS", baseSize, baseMinimum)
	}
	baseID := baseH.ID()
	again, ok := baseH.RetainedPayloadUpper()
	if !ok || again != baseSize || baseH.ID() != baseID {
		t.Fatal("ASSERT_HANDOFF_SCALAR_READONLY")
	}
	build := func(selector []byte, rationale string) (cc.PrivateAllModeHandoff, uint64) {
		slots, entries, minimum := sourceStageEntries(t, absent.Input, []string{"RD", "RD", "RD", "RD"}, 7, selector, rationale)
		h, e := cc.NewPrivateAllModeHandoff(qWorkspace(q), "source-controlled-revision", append(append([]cc.PrivateAllModeSlot(nil), baseSlots...), slots...), append(append([]cc.PrivateAllModeEntry(nil), baseEntries...), entries...))
		if e != nil {
			t.Fatal(e)
		}
		return h, baseMinimum + minimum
	}
	small, smallMinimum := build([]byte("absent source selector\n"), "CONTROLLED_SOURCE_MODE")
	large, largeMinimum := build(bytes.Repeat([]byte("s"), 1<<16), strings.Repeat("p", 4096))
	smallSize, smallOK := small.RetainedPayloadUpper()
	largeSize, largeOK := large.RetainedPayloadUpper()
	if !smallOK || !largeOK || smallSize < smallMinimum || largeSize < largeMinimum || largeSize <= smallSize || largeSize-smallSize < largeMinimum-smallMinimum {
		t.Fatal("ASSERT_ABSENT_READY_PAYLOAD_GROWTH", smallSize, largeSize, smallMinimum, largeMinimum)
	}
	t.Logf("ASSERT_ABSENT_READY_PAYLOAD_GROWTH: PASS; base=%d small=%d large=%d payload_delta=%d", baseSize, smallSize, largeSize, largeMinimum-smallMinimum)
	store := &sourceStageStore{MemoryStore: cc.NewMemoryStore()}
	pause := func(h cc.PrivateAllModeHandoff) *sc.PrivateAllModeSourcesResult {
		r := sc.Run(ctx, sc.Request{PrivateAllMode: &h, PrivateSources: &q, Workspace: qWorkspace(q), Contract: contract, Store: store, StopAfter: sc.StopAfterPrivateSourceRequests})
		if r.Err != nil || r.PrivateSources == nil {
			t.Fatal("ASSERT_ABSENT_READY_PREPARED", r.Err)
		}
		return r.PrivateSources
	}
	smallOut := pause(small)
	largeOut := pause(large)
	smallRequests, _ := describerequest.Bytes(smallOut.Requests)
	largeRequests, _ := describerequest.Bytes(largeOut.Requests)
	if smallOut.Counts != largeOut.Counts || smallOut.Counts.Admitted != 32 || smallOut.Counts.Requests != 2 || smallOut.Counts.Unresolved <= 24 || !bytes.Equal(smallRequests, largeRequests) {
		t.Fatal("ASSERT_ABSENT_READY_CONSTANT_COUNTS", smallOut.Counts, largeOut.Counts)
	}
	semantic := `{"verdict":"ABSTAINED","target_role":"controlled source","consumer_need":{"status":"UNRESOLVED","value":""},"provided_behavior":{"value":"supplied fixture","consumer_relative":false},"boundary_contribution":"unknown","limitations":["not HOST output"]}`
	bindRequest := func(h cc.PrivateAllModeHandoff, out *sc.PrivateAllModeSourcesResult) sc.SuppliedSourceResponseRequest {
		r := out.Requests[0]
		return sc.SuppliedSourceResponseRequest{Resume: sc.ResumeRequest{PrivateSources: &sc.PrivateAllModeSourcesResume{Version: sc.PrivateAllModeSourcesVersion, Expected: out.Expected, Originals: q.Originals, Definitions: q.Definitions, ExpectedHandoff: h}, Workspace: qWorkspace(q), Store: store, StopAfter: sc.StopAfterPrivateSourceRequests, Selector: out.CheckpointID}, Record: r, Attempt: sc.SuppliedSourceAttempt{ID: "absent-ready-control", CheckpointID: out.CheckpointID, SourceUnitID: r.Envelope.AdmissionID, RequestRecordID: r.RecordID}, AdapterLine: describeworker.EncodeAdapterResultV2(semantic), Limits: describeworker.SuppliedResponseLimits{MaxAdapterBytes: 512 * 1024, MaxSemanticBytes: 64 * 1024}}
	}
	before := store.puts
	good, e := sc.BindSuppliedSourceResponse(ctx, bindRequest(small, smallOut))
	if e != nil || good.ID() == "" {
		t.Fatal("ASSERT_ABSENT_READY_SMALL_FITS", e)
	}
	bad, e := sc.BindSuppliedSourceResponse(ctx, bindRequest(large, largeOut))
	if e == nil || bad.ID() != "" || !strings.Contains(e.Error(), "retained-plus-peak ceiling") {
		t.Fatal("ASSERT_ABSENT_READY_PARENT_REFUSAL", e, bad.ID(), smallSize, largeSize)
	}
	if store.puts != before {
		t.Fatal("ASSERT_ABSENT_READY_NO_PUBLICATION")
	}
	t.Log("ASSERT_HANDOFF_ALL_READY_PAYLOADS: PASS; ASSERT_HANDOFF_SCALAR_READONLY: PASS; ASSERT_HANDOFF_IDENTITY_UNCHANGED: PASS; ASSERT_ABSENT_READY_CONSTANT_COUNTS: PASS; ASSERT_ABSENT_READY_SMALL_FITS: PASS; ASSERT_ABSENT_READY_PARENT_REFUSAL: PASS", smallOut.Counts)
}

// Observation only: no producer callback, mutation or production test hook.
// Any join/metadata regeneration/Runner plan entry makes the early guard fail,
// including metadata work which can precede the first StoreGet.
type earlyWorkContext struct {
	context.Context
	regenerated, plan bool
}

func (c *earlyWorkContext) Err() error {
	for depth := 1; depth <= 12; depth++ {
		pc, _, _, ok := runtime.Caller(depth)
		if !ok {
			break
		}
		name := runtime.FuncForPC(pc).Name()
		if strings.HasSuffix(name, "sourcecontinuation.sourceResponseJoin") || strings.HasSuffix(name, "censuscontinuation.preparePrivateAllMode") {
			c.regenerated = true
		}
		if strings.HasSuffix(name, "describeworker.(*Runner).PrepareSourceAcquisition") {
			c.plan = true
		}
	}
	return c.Context.Err()
}

func TestSuppliedEarly(t *testing.T) {
	runner, marker := acquireRunner(t)
	served, _, contract, q := sourceStageFixture(t, "positive", false)
	absent, _, _, _ := sourceStageFixture(t, "positive", false)
	inv.ControlledSourceProjectionNoReadmitForTest(t, served)
	ctx, cancel := context.WithDeadline(context.Background(), q.Deadline)
	defer cancel()
	baseSlots, baseEntries, _ := sourceStageEntries(t, served.Input, []string{"RD", "C", "CRD", "D", "CD", "R", "CR"}, 0, []byte("source selection\n"), "CONTROLLED_SOURCE_MODE")
	build := func(selector []byte, rationale string) (cc.PrivateAllModeHandoff, *sc.PrivateAllModeSourcesResult, *sourceStageStore) {
		slots, entries, _ := sourceStageEntries(t, absent.Input, []string{"RD", "RD", "RD", "RD"}, 7, selector, rationale)
		h, e := cc.NewPrivateAllModeHandoff(qWorkspace(q), "source-controlled-revision", append(append([]cc.PrivateAllModeSlot(nil), baseSlots...), slots...), append(append([]cc.PrivateAllModeEntry(nil), baseEntries...), entries...))
		if e != nil {
			t.Fatal("ASSERT_EARLY_SETUP", e)
		}
		store := &sourceStageStore{MemoryStore: cc.NewMemoryStore()}
		pause := sc.Run(ctx, sc.Request{PrivateAllMode: &h, PrivateSources: &q, Workspace: qWorkspace(q), Contract: contract, Store: store, StopAfter: sc.StopAfterPrivateSourceRequests})
		if pause.Err != nil || pause.PrivateSources == nil {
			t.Fatal("ASSERT_EARLY_SETUP", pause.Err)
		}
		return h, pause.PrivateSources, store
	}
	small, smallOut, smallStore := build([]byte("absent source selector\n"), "CONTROLLED_SOURCE_MODE")
	large, largeOut, largeStore := build(bytes.Repeat([]byte("s"), 1<<16), strings.Repeat("p", 4096))
	if smallOut.Counts != largeOut.Counts || smallOut.Counts.Admitted != 32 || smallOut.Counts.Requests != 2 {
		t.Fatal("ASSERT_EARLY_CONSTANT_COUNTS", smallOut.Counts, largeOut.Counts)
	}
	smallSize, _ := small.RetainedPayloadUpper()
	largeSize, _ := large.RetainedPayloadUpper()
	if largeSize <= smallSize {
		t.Fatal("ASSERT_EARLY_PAYLOAD_GROWTH")
	}
	makeRequest := func(h cc.PrivateAllModeHandoff, out *sc.PrivateAllModeSourcesResult, store *sourceStageStore) sc.SuppliedSourceResponseRequest {
		r := out.Requests[0]
		return sc.SuppliedSourceResponseRequest{Resume: sc.ResumeRequest{PrivateSources: &sc.PrivateAllModeSourcesResume{Version: sc.PrivateAllModeSourcesVersion, Expected: out.Expected, Originals: q.Originals, Definitions: q.Definitions, ExpectedHandoff: h}, Workspace: qWorkspace(q), Store: store, StopAfter: sc.StopAfterPrivateSourceRequests, Selector: out.CheckpointID}, Record: r, Attempt: sc.SuppliedSourceAttempt{ID: "early-control", CheckpointID: out.CheckpointID, SourceUnitID: r.Envelope.AdmissionID, RequestRecordID: r.RecordID}, AdapterLine: describeworker.EncodeAdapterResultV2(`{"verdict":"ABSTAINED","target_role":"controlled source","consumer_need":{"status":"UNRESOLVED","value":""},"provided_behavior":{"value":"supplied fixture","consumer_relative":false},"boundary_contribution":"unknown","limitations":["not HOST output"]}`), Limits: describeworker.SuppliedResponseLimits{MaxAdapterBytes: 512 << 10, MaxSemanticBytes: 64 << 10}}
	}
	prep := func(r sc.SuppliedSourceResponseRequest) sc.SourceAcquisitionRequest {
		return sc.SourceAcquisitionRequest{Resume: r.Resume, Record: r.Record, Attempt: r.Attempt}
	}
	// Test-only wire check, not runtime sizing: the real successful lower result
	// carries private scalar state, but an identical JSON round trip cannot.
	metadata, err := cc.PreparePrivateAllModeSourceIntents(ctx, small, contract, smallStore, qWorkspace(q))
	if err != nil {
		t.Fatal("ASSERT_EARLY_LOWER_CACHE", err)
	}
	peak, held, valid := metadata.RegenerationPayloadUpper()
	if !valid || peak < held || held == 0 {
		t.Fatal("ASSERT_EARLY_LOWER_CACHE", peak, held, valid)
	}
	encoded, err := json.Marshal(metadata)
	if err != nil {
		t.Fatal(err)
	}
	var decoded cc.PrivateAllModeResult
	if json.Unmarshal(encoded, &decoded) != nil {
		t.Fatal("ASSERT_EARLY_PRIVATE_CACHE_WIRE")
	}
	if _, _, ok := decoded.RegenerationPayloadUpper(); ok {
		t.Fatal("ASSERT_EARLY_PRIVATE_CACHE_WIRE")
	}
	again, _ := json.Marshal(decoded)
	if !bytes.Equal(encoded, again) {
		t.Fatal("ASSERT_EARLY_PRIVATE_CACHE_WIRE")
	}
	t.Log("ASSERT_EARLY_LOWER_CACHE: PASS; ASSERT_EARLY_PRIVATE_CACHE_WIRE: PASS", peak, held)
	bad := makeRequest(large, largeOut, largeStore)
	largeStore.gets, largeStore.puts = 0, 0
	probe := &earlyWorkContext{Context: ctx}
	p, e := sc.PrepareSourceAcquisition(probe, runner, prep(bad))
	if e == nil || p.ID() != "" || largeStore.gets != 0 || largeStore.puts != 0 || probe.regenerated || probe.plan {
		t.Errorf("ASSERT_EARLY_PREPARE_ZERO_REGEN: error=%v handle=%t gets=%d puts=%d regenerated=%t plan=%t", e, p.ID() != "", largeStore.gets, largeStore.puts, probe.regenerated, probe.plan)
	} else {
		t.Log("ASSERT_EARLY_PREPARE_ZERO_REGEN: PASS")
	}
	good := makeRequest(small, smallOut, smallStore)
	eligible, e := sc.PrepareSourceAcquisition(ctx, runner, prep(good))
	if e != nil || eligible.ID() == "" {
		t.Fatal("ASSERT_EARLY_SMALL_FITS", e)
	}
	smallStore.gets, smallStore.puts = 0, 0
	largeResponse := good
	largeResponse.AdapterLine = bytes.Replace(good.AdapterLine, []byte(`"grammar":""`), []byte(`"grammar":"`+strings.Repeat("g", 500*1024)+`"`), 1)
	probe = &earlyWorkContext{Context: ctx}
	b, e := sc.CompleteSuppliedSourceAcquisition(probe, runner, eligible, largeResponse)
	if e == nil || b.ID() != "" || smallStore.gets != 0 || smallStore.puts != 0 || probe.regenerated || probe.plan {
		t.Errorf("ASSERT_EARLY_COMPLETE_ZERO_REGEN: error=%v handle=%t gets=%d puts=%d regenerated=%t plan=%t", e, b.ID() != "", smallStore.gets, smallStore.puts, probe.regenerated, probe.plan)
	} else {
		t.Log("ASSERT_EARLY_COMPLETE_ZERO_REGEN: PASS")
	}
	smallStore.gets, smallStore.puts = 0, 0
	mismatch := good
	mismatch.Limits.MaxAdapterBytes++
	if b, e := sc.CompleteSuppliedSourceAcquisition(ctx, runner, eligible, mismatch); e == nil || b.ID() != "" || smallStore.gets != 0 || smallStore.puts != 0 {
		t.Error("ASSERT_EARLY_LIMIT_ZERO_REGEN", e, smallStore.gets, smallStore.puts)
	} else {
		t.Log("ASSERT_EARLY_LIMIT_ZERO_REGEN: PASS")
	}
	bound, e := sc.CompleteSuppliedSourceAcquisition(ctx, runner, eligible, good)
	if e != nil || bound.ID() == "" {
		t.Fatal("ASSERT_EARLY_SMALL_COMPLETES", e)
	}
	if _, e := os.Stat(marker); !errors.Is(e, os.ErrNotExist) {
		t.Fatal("ASSERT_EARLY_NO_MODEL_START", e)
	}
	t.Log("ASSERT_EARLY_SMALL_FITS: PASS; ASSERT_EARLY_SMALL_COMPLETES: PASS; ASSERT_EARLY_CONSTANT_COUNTS: PASS; ASSERT_EARLY_NO_MODEL_START: PASS", smallSize, largeSize, smallOut.Counts)
}

// Real, controlled pins for the existing Runner constructor. The executable is
// NEVER run; an unexpected start would leave a marker and fail the witness.
func acquireRunner(t *testing.T) (*describeworker.Runner, string) {
	t.Helper()
	if runtime.GOOS != "darwin" {
		t.Skip("Runner Preflight requires Darwin; compile-only elsewhere")
	}
	dir := t.TempDir()
	marker := filepath.Join(dir, "PROHIBITED_START")
	pin := func(name, body string, mode os.FileMode) describeworker.FilePin {
		path := filepath.Join(dir, name)
		if e := os.WriteFile(path, []byte(body), mode); e != nil {
			t.Fatal(e)
		}
		return describeworker.FilePin{Path: path, SHA256: fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(body)))}
	}
	script := "#!/bin/sh\nprintf prohibited > '" + marker + "'\nexit 97\n"
	c := describeworker.Config{Worker: pin("worker", script, 0700), Model: pin("model", "controlled model, never loaded", 0600), Library: pin("library", "controlled library, never loaded", 0600), SandboxExecutable: pin("sandbox", script, 0700), SandboxProfile: pin("profile", "(version 1)\n(deny network*)\n", 0600), Grammar: pin("grammar", "root ::= object\n", 0600), RuntimeIdentity: "controlled-runtime", AdapterIdentity: "controlled-adapter", ModelIdentity: "controlled-model", ResponseVersion: describeworker.ResponseVersionV2, Limits: describeworker.Limits{TimeoutMS: 1000, MaxTokens: 64, ContextTokens: 1024, StdoutBytes: 512 << 10, StderrBytes: 1024, WorkBytes: 1 << 20, TempBytes: 1 << 20}}
	r, e := describeworker.NewRunner(c)
	if e != nil {
		t.Fatal(e)
	}
	return r, marker
}

func TestSuppliedAcquire(t *testing.T) {
	runner, marker := acquireRunner(t)
	foreign, otherMarker := acquireRunner(t)
	input, h, c, q := sourceStageFixture(t, "positive", false)
	inv.ControlledSourceProjectionNoReadmitForTest(t, input)
	ctx, cancel := context.WithDeadline(context.Background(), q.Deadline)
	defer cancel()
	store := &sourceStageStore{MemoryStore: cc.NewMemoryStore()}
	got := sc.Run(ctx, sc.Request{PrivateAllMode: &h, PrivateSources: &q, Workspace: qWorkspace(q), Contract: c, Store: store, StopAfter: sc.StopAfterPrivateSourceRequests})
	if got.Err != nil || got.PrivateSources == nil {
		t.Fatal("ASSERT_ACQUIRE_SOURCE", got.Err)
	}
	out := got.PrivateSources
	if out.Counts.Admitted != 32 || out.Counts.Unresolved != 24 || out.Counts.Requests != 2 || out.Counts.Intents != 56 {
		t.Fatal("ASSERT_ACQUIRE_ALL7", out.Counts)
	}
	resume := sc.ResumeRequest{PrivateSources: &sc.PrivateAllModeSourcesResume{Version: sc.PrivateAllModeSourcesVersion, Expected: out.Expected, Originals: q.Originals, Definitions: q.Definitions, ExpectedHandoff: h}, Workspace: qWorkspace(q), Store: store, StopAfter: sc.StopAfterPrivateSourceRequests, Selector: out.CheckpointID}
	semantic := `{"verdict":"ABSTAINED","target_role":"controlled source fixture","consumer_need":{"status":"UNRESOLVED","value":""},"provided_behavior":{"value":"fixture body only","consumer_relative":false},"boundary_contribution":"unknown","limitations":["controlled supplied terminal text; not HOST output"]}`
	makeRequest := func(n int) sc.SuppliedSourceResponseRequest {
		r := out.Requests[n]
		return sc.SuppliedSourceResponseRequest{Resume: resume, Record: r, Attempt: sc.SuppliedSourceAttempt{ID: "controlled-acquisition", CheckpointID: out.CheckpointID, SourceUnitID: r.Envelope.AdmissionID, RequestRecordID: r.RecordID}, AdapterLine: describeworker.EncodeAdapterResultV2(semantic), Limits: describeworker.SuppliedResponseLimits{MaxAdapterBytes: 512 << 10, MaxSemanticBytes: 64 << 10}}
	}
	prepare := func(r sc.SuppliedSourceResponseRequest) sc.SourceAcquisitionRequest {
		return sc.SourceAcquisitionRequest{Resume: r.Resume, Record: r.Record, Attempt: r.Attempt}
	}
	first := makeRequest(0)
	eligible, e := sc.PrepareSourceAcquisition(ctx, runner, prepare(first))
	if e != nil || eligible.ID() == "" {
		t.Fatal("ASSERT_ACQUIRE_ELIGIBLE", e)
	}
	if _, e = sc.CompleteSuppliedSourceAcquisition(ctx, foreign, eligible, first); e == nil {
		t.Fatal("ASSERT_ACQUIRE_OWNER")
	}
	t.Log("ASSERT_ACQUIRE_OWNER: PASS")
	beforePuts := store.puts
	bindings := map[string]bool{}
	for n := range out.Requests {
		r := makeRequest(n)
		p, e := sc.PrepareSourceAcquisition(ctx, runner, prepare(r))
		if e != nil {
			t.Fatal("ASSERT_ACQUIRE_BOTH_REQUESTS", e)
		}
		b, e := sc.CompleteSuppliedSourceAcquisition(ctx, runner, p, r)
		if e != nil || b.ID() == "" || bindings[b.ID()] {
			t.Fatal("ASSERT_ACQUIRE_BOTH_REQUESTS", e)
		}
		bindings[b.ID()] = true
		ordinary, e := sc.BindSuppliedSourceResponse(ctx, r)
		bb, _ := b.Bytes()
		ob, _ := ordinary.Bytes()
		if e != nil || !bytes.Equal(bb, ob) {
			t.Fatal("ASSERT_ACQUIRE_ADOPTED_BINDER", e)
		}
		var wire map[string]json.RawMessage
		if json.Unmarshal(bb, &wire) != nil || string(wire["authority"]) != "0" || string(wire["accepted"]) != "false" || string(wire["identity"]) != `"UNRESOLVED"` || string(wire["completeness"]) != `"UNKNOWN"` || string(wire["absence"]) != `"PROHIBITED"` || string(wire["origin"]) != `"SUPPLIED_TERMINAL_RESPONSE_ONLY"` || string(wire["execution"]) != `"NO_PROCESS_OR_HOST_EXECUTION_VERIFIED"` {
			t.Fatal("ASSERT_ACQUIRE_CEILING")
		}
	}
	t.Log("ASSERT_ACQUIRE_BOTH_REQUESTS: PASS; ASSERT_ACQUIRE_ADOPTED_BINDER: PASS; ASSERT_ACQUIRE_CEILING: PASS")
	// Complete tuple negatives. Each attempt retains the same genuine upper
	// handle; mutated public values cannot cause it to bind another context.
	for name, mutate := range map[string]func(*sc.SuppliedSourceResponseRequest){
		"request":     func(r *sc.SuppliedSourceResponseRequest) { r.Record.RecordID = "foreign" },
		"prompt":      func(r *sc.SuppliedSourceResponseRequest) { r.Record.Envelope.Prompt += "foreign" },
		"full-record": func(r *sc.SuppliedSourceResponseRequest) { r.Record.Lineage.AlternativeOrdinal++ },
		"source": func(r *sc.SuppliedSourceResponseRequest) {
			r.Attempt.SourceUnitID = out.Requests[1].Envelope.AdmissionID
		},
		"attempt":         func(r *sc.SuppliedSourceResponseRequest) { r.Attempt.ID = "foreign" },
		"attempt-request": func(r *sc.SuppliedSourceResponseRequest) { r.Attempt.RequestRecordID = "foreign" },
		"checkpoint":      func(r *sc.SuppliedSourceResponseRequest) { r.Attempt.CheckpointID = "foreign" },
		"workspace":       func(r *sc.SuppliedSourceResponseRequest) { r.Resume.Workspace = "foreign" },
		"selector":        func(r *sc.SuppliedSourceResponseRequest) { r.Resume.Selector = "foreign" },
		"original": func(r *sc.SuppliedSourceResponseRequest) {
			s := *r.Resume.PrivateSources
			s.Originals = nil
			r.Resume.PrivateSources = &s
		},
		"handoff": func(r *sc.SuppliedSourceResponseRequest) {
			s := *r.Resume.PrivateSources
			s.ExpectedHandoff = cc.PrivateAllModeHandoff{}
			r.Resume.PrivateSources = &s
		},
		"expected": func(r *sc.SuppliedSourceResponseRequest) {
			s := *r.Resume.PrivateSources
			s.Expected = sc.PrivateAllModeSourcesExpected{}
			r.Resume.PrivateSources = &s
		},
		"buffer": func(r *sc.SuppliedSourceResponseRequest) { r.AdapterLine = make([]byte, (512<<10)+1) },
	} {
		r := first
		mutate(&r)
		if b, e := sc.CompleteSuppliedSourceAcquisition(ctx, runner, eligible, r); e == nil || b.ID() != "" {
			t.Fatal("ASSERT_ACQUIRE_JOIN", name)
		}
	}
	t.Log("ASSERT_ACQUIRE_JOIN: PASS")
	var public sc.SourceAcquisitionHandoff
	publicBytes, _ := json.Marshal(eligible)
	if json.Unmarshal(publicBytes, &public) != nil || public.ID() != "" {
		t.Fatal("ASSERT_ACQUIRE_OPAQUE")
	}
	if b, e := sc.CompleteSuppliedSourceAcquisition(ctx, runner, public, first); e == nil || b.ID() != "" {
		t.Fatal("ASSERT_ACQUIRE_OPAQUE")
	}
	// A genuine lower plan still cannot carry upper source eligibility via JSON.
	lowerContext := describeworker.SourceAcquisitionContext{HandoffID: h.ID(), CheckpointID: out.CheckpointID, SourceUnitID: first.Attempt.SourceUnitID, SourceDigest: "sha256:" + first.Record.Envelope.SourceRevision, Workspace: resume.Workspace, PolicyID: q.PolicyID, Deadline: q.Deadline, SourceLimits: q.Limits, MaxWireBytes: c.PacketMaxResponseBytes(), MaxParentBytes: c.CaptureLimits().MaxParentBytes}
	lower, e := runner.PrepareSourceAcquisition(ctx, first.Record, first.Attempt.ID, lowerContext)
	if e != nil {
		t.Fatal("ASSERT_ACQUIRE_LOWER_SETUP", e)
	}
	lowerBytes, _ := json.Marshal(lower)
	if json.Unmarshal(lowerBytes, &public) != nil || public.ID() != "" {
		t.Fatal("ASSERT_ACQUIRE_LOWER_NONADMITTING")
	}
	noProof := prepare(first)
	ss := *noProof.Resume.PrivateSources
	ss.Expected = sc.PrivateAllModeSourcesExpected{}
	noProof.Resume.PrivateSources = &ss
	if p, e := sc.PrepareSourceAcquisition(ctx, runner, noProof); e == nil || p.ID() != "" {
		t.Fatal("ASSERT_ACQUIRE_LOWER_NONADMITTING")
	}
	if !errors.Is(runner.RunSourceAcquisition(ctx, lower), describeworker.ErrSourceExecutionDisabled) {
		t.Fatal("ASSERT_ACQUIRE_DISABLED")
	}
	cancelled, stop := context.WithCancel(ctx)
	stop()
	gets := store.gets
	if p, e := sc.PrepareSourceAcquisition(cancelled, runner, prepare(first)); e == nil || p.ID() != "" || store.gets != gets {
		t.Fatal("ASSERT_ACQUIRE_EARLY_CANCEL")
	}
	if b, e := sc.CompleteSuppliedSourceAcquisition(cancelled, runner, eligible, first); e == nil || b.ID() != "" || store.gets != gets {
		t.Fatal("ASSERT_ACQUIRE_EARLY_CANCEL")
	}
	for _, completion := range []bool{false, true} {
		base, stop := context.WithCancel(ctx)
		late := &acquireFinalContext{Context: base, cancel: stop}
		var id string
		var e error
		if completion {
			b, err := sc.CompleteSuppliedSourceAcquisition(late, runner, eligible, first)
			id, e = b.ID(), err
		} else {
			p, err := sc.PrepareSourceAcquisition(late, runner, prepare(first))
			id, e = p.ID(), err
		}
		stop()
		phase := "prepare"
		if completion {
			phase = "completion"
		}
		if e == nil || id != "" || !late.seen {
			t.Errorf("ASSERT_ACQUIRE_FINAL_CANCEL_%s error=%v seen=%v", phase, e, late.seen)
		} else {
			t.Logf("ASSERT_ACQUIRE_FINAL_CANCEL_%s: PASS", phase)
		}
	}
	t.Log("ASSERT_ACQUIRE_EARLY_CANCEL: PASS")
	// Boundary search changes ONLY supplied grammar metadata, not any grant,
	// source limit, URI or deadline. Find an ordinary binder payload just below
	// the unchanged parent ceiling, then show this retained plan is not free.
	low, high := 0, 500*1024
	for n := 0; n < 10; n++ {
		mid := (low + high) / 2
		r := first
		r.AdapterLine = bytes.Replace(first.AdapterLine, []byte(`"grammar":""`), []byte(`"grammar":"`+strings.Repeat("g", mid)+`"`), 1)
		if _, e := sc.BindSuppliedSourceResponse(ctx, r); e == nil {
			low = mid
		} else {
			if !strings.Contains(e.Error(), "retained-plus-peak ceiling") {
				t.Fatal("ASSERT_ACQUIRE_EXTRA_CONTROL", e)
			}
			high = mid
		}
	}
	near := first
	near.AdapterLine = bytes.Replace(first.AdapterLine, []byte(`"grammar":""`), []byte(`"grammar":"`+strings.Repeat("g", low)+`"`), 1)
	if _, e := sc.BindSuppliedSourceResponse(ctx, near); e != nil {
		t.Fatal("ASSERT_ACQUIRE_EXTRA_CONTROL", e)
	}
	if b, e := sc.CompleteSuppliedSourceAcquisition(ctx, runner, eligible, near); e == nil || b.ID() != "" || !strings.Contains(e.Error(), "retained-plus-peak ceiling") {
		t.Fatal("ASSERT_ACQUIRE_EXTRA_RETAINED", e)
	}
	t.Log("ASSERT_ACQUIRE_EXTRA_RETAINED: PASS; ASSERT_ACQUIRE_EXTRA_CONTROL: PASS", low, high)
	if b, e := sc.CompleteSuppliedSourceAcquisition(ctx, runner, eligible, first); e != nil || b.ID() == "" {
		t.Fatal("ASSERT_ACQUIRE_RESTORED", e)
	}
	for _, path := range []string{marker, otherMarker} {
		if _, e := os.Stat(path); !errors.Is(e, os.ErrNotExist) {
			t.Fatal("ASSERT_ACQUIRE_NO_START", e)
		}
	}
	if store.puts != beforePuts || len(bindings) != 2 || q.Deadline != lowerContext.Deadline || q.Limits != lowerContext.SourceLimits {
		t.Fatal("ASSERT_ACQUIRE_UNCHANGED_CONTEXT")
	}
	t.Log("ASSERT_ACQUIRE_ALL7: PASS; ASSERT_ACQUIRE_OPAQUE: PASS; ASSERT_ACQUIRE_LOWER_NONADMITTING: PASS; ASSERT_ACQUIRE_DISABLED: PASS; ASSERT_ACQUIRE_NO_START: PASS; ASSERT_ACQUIRE_UNCHANGED_CONTEXT: PASS; ASSERT_ACQUIRE_RESTORED: PASS; source=32 unresolved=24 requests=2 bindings=2 describe_workers=0; no new capture/evaluator")
}

// Test-only context intercepts only the actual final upper publication helper.
// No mutable production hook or clock is introduced.
type acquireFinalContext struct {
	context.Context
	cancel   context.CancelFunc
	deadline time.Time
	seen     bool
}

func (c *acquireFinalContext) Err() error {
	if e := c.Context.Err(); e != nil {
		return e
	}
	for depth := 1; depth <= 4; depth++ {
		pc, _, _, ok := runtime.Caller(depth)
		if ok && strings.HasSuffix(runtime.FuncForPC(pc).Name(), ".sourceAcquisitionPublicationLive") {
			c.seen = true
			if !c.deadline.IsZero() {
				time.Sleep(time.Until(c.deadline) + time.Millisecond)
				return nil
			}
			c.cancel()
			return c.Context.Err()
		}
	}
	return nil
}

func TestSuppliedExpiry(t *testing.T) {
	runner, _ := acquireRunner(t)
	input, h, c, q := sourceStageFixture(t, "positive", false)
	inv.ControlledSourceProjectionNoReadmitForTest(t, input)
	for _, completion := range []bool{false, true} {
		// Shorten, never renew, the original source deadline for this separate
		// controlled source pause. Existing producer originals outlive it.
		short := q
		short.Deadline = time.Now().Add(700 * time.Millisecond)
		if !short.Deadline.Before(q.Deadline) {
			t.Fatal("ASSERT_ACQUIRE_EXPIRY_SETUP")
		}
		store := &sourceStageStore{MemoryStore: cc.NewMemoryStore()}
		got := sc.Run(context.Background(), sc.Request{PrivateAllMode: &h, PrivateSources: &short, Workspace: qWorkspace(q), Contract: c, Store: store, StopAfter: sc.StopAfterPrivateSourceRequests})
		if got.Err != nil || got.PrivateSources == nil {
			t.Fatal("ASSERT_ACQUIRE_EXPIRY_SETUP", got.Err)
		}
		out := got.PrivateSources
		r := out.Requests[0]
		resume := sc.ResumeRequest{PrivateSources: &sc.PrivateAllModeSourcesResume{Version: sc.PrivateAllModeSourcesVersion, Expected: out.Expected, Originals: q.Originals, Definitions: q.Definitions, ExpectedHandoff: h}, Workspace: qWorkspace(q), Store: store, StopAfter: sc.StopAfterPrivateSourceRequests, Selector: out.CheckpointID}
		prep := sc.SourceAcquisitionRequest{Resume: resume, Record: r, Attempt: sc.SuppliedSourceAttempt{ID: "controlled-expiry", CheckpointID: out.CheckpointID, SourceUnitID: r.Envelope.AdmissionID, RequestRecordID: r.RecordID}}
		late := &acquireFinalContext{Context: context.Background(), deadline: short.Deadline}
		if completion {
			p, e := sc.PrepareSourceAcquisition(context.Background(), runner, prep)
			if e != nil {
				t.Fatal("ASSERT_ACQUIRE_EXPIRY_SETUP", e)
			}
			semantic := `{"verdict":"ABSTAINED","target_role":"fixture","consumer_need":{"status":"UNRESOLVED","value":""},"provided_behavior":{"value":"fixture","consumer_relative":false},"boundary_contribution":"unknown","limitations":["supplied only"]}`
			b, e := sc.CompleteSuppliedSourceAcquisition(late, runner, p, sc.SuppliedSourceResponseRequest{Resume: resume, Record: r, Attempt: prep.Attempt, AdapterLine: describeworker.EncodeAdapterResultV2(semantic), Limits: describeworker.SuppliedResponseLimits{MaxAdapterBytes: 512 << 10, MaxSemanticBytes: 64 << 10}})
			if e == nil || b.ID() != "" || !late.seen {
				t.Errorf("ASSERT_ACQUIRE_FINAL_EXPIRY_completion error=%v seen=%v", e, late.seen)
			} else {
				t.Log("ASSERT_ACQUIRE_FINAL_EXPIRY_completion: PASS")
			}
		} else {
			p, e := sc.PrepareSourceAcquisition(late, runner, prep)
			if e == nil || p.ID() != "" || !late.seen {
				t.Errorf("ASSERT_ACQUIRE_FINAL_EXPIRY_prepare error=%v seen=%v", e, late.seen)
			} else {
				t.Log("ASSERT_ACQUIRE_FINAL_EXPIRY_prepare: PASS")
			}
		}
		before := store.gets
		if p, e := sc.PrepareSourceAcquisition(context.Background(), runner, prep); e == nil || p.ID() != "" || store.gets != before {
			t.Fatal("ASSERT_ACQUIRE_EARLY_EXPIRY", e)
		}
	}
	t.Log("ASSERT_ACQUIRE_EARLY_EXPIRY: PASS")
}

// Controlled supplied semantic text is NOT authentic HOST output. Only the
// existing managed producer fixture executes; response binding executes nothing.
func TestPrivateSuppliedSourceResponseConnected(t *testing.T) {
	input, h, c, q := sourceStageFixture(t, "positive", false)
	inv.ControlledSourceProjectionNoReadmitForTest(t, input)
	ctx, cancel := context.WithDeadline(context.Background(), q.Deadline)
	defer cancel()
	store := &sourceStageStore{MemoryStore: cc.NewMemoryStore()}
	got := sc.Run(ctx, sc.Request{PrivateAllMode: &h, PrivateSources: &q, Workspace: qWorkspace(q), Contract: c, Store: store, StopAfter: sc.StopAfterPrivateSourceRequests})
	if got.Err != nil || got.PrivateSources == nil {
		t.Fatal("ASSERT_BIND_SOURCE_PREPARED", got.Err)
	}
	out := got.PrivateSources
	if out.Counts.Admitted != 32 || out.Counts.Unresolved != 24 || out.Counts.Requests != 2 || out.Counts.Intents != 56 {
		t.Fatal("ASSERT_BIND_ALL7_COUNTS", out.Counts)
	}
	resume := sc.ResumeRequest{PrivateSources: &sc.PrivateAllModeSourcesResume{Version: sc.PrivateAllModeSourcesVersion, Expected: out.Expected, Originals: q.Originals, Definitions: q.Definitions, ExpectedHandoff: h}, Workspace: qWorkspace(q), Store: store, StopAfter: sc.StopAfterPrivateSourceRequests, Selector: out.CheckpointID}
	semantic := `{"verdict":"ABSTAINED","target_role":"controlled source fixture","consumer_need":{"status":"UNRESOLVED","value":""},"provided_behavior":{"value":"fixture body only","consumer_relative":false},"boundary_contribution":"unknown","limitations":["controlled supplied terminal text; not HOST output"]}`
	limits := describeworker.SuppliedResponseLimits{MaxAdapterBytes: 512 * 1024, MaxSemanticBytes: 64 * 1024}
	makeRequest := func(n int) sc.SuppliedSourceResponseRequest {
		r := out.Requests[n]
		return sc.SuppliedSourceResponseRequest{Resume: resume, Record: r, Attempt: sc.SuppliedSourceAttempt{ID: "controlled-attempt", CheckpointID: out.CheckpointID, SourceUnitID: r.Envelope.AdmissionID, RequestRecordID: r.RecordID}, AdapterLine: describeworker.EncodeAdapterResultV2(semantic), Limits: limits}
	}
	first := makeRequest(0)
	// All producer leases have already joined/finished before this point. This
	// controlled context cancels ONLY at the delivered final publication check,
	// not during store retrieval or parsing, and introduces no production hook.
	lateBase, cancelLate := context.WithCancel(ctx)
	late := &sourcePublicationCancelContext{Context: lateBase, cancel: cancelLate}
	lateBound, lateErr := sc.BindSuppliedSourceResponse(late, first)
	cancelLate() // complete cleanup before any fatal assertion
	if lateErr == nil || lateBound.ID() != "" || !late.publicationSeen {
		t.Fatal("ASSERT_BIND_LATE_PUBLICATION_NO_HANDLE", lateErr, lateBound.ID(), late.publicationSeen)
	}
	t.Log("ASSERT_BIND_LATE_PUBLICATION_NO_HANDLE: PASS")

	// Original packet/parent ceilings are NOT increased. A valid, large adapter
	// with unrelated grammar metadata consumes the finite logical parser peak.
	peakRequest := first
	peakRequest.AdapterLine = bytes.Replace(first.AdapterLine, []byte(`"grammar":""`), []byte(`"grammar":"`+strings.Repeat("g", 500*1024)+`"`), 1)
	if _, e := describeworker.ParseSuppliedTerminalResponse(peakRequest.AdapterLine, limits); e != nil {
		t.Fatal("ASSERT_BIND_PEAK_CONTROL_VALID_ADAPTER", e)
	}
	if b, e := sc.BindSuppliedSourceResponse(ctx, peakRequest); e == nil || b.ID() != "" || !strings.Contains(e.Error(), "retained-plus-peak ceiling") {
		t.Fatal("ASSERT_BIND_EXISTING_PARENT_PEAK", e)
	}
	t.Log("ASSERT_BIND_EXISTING_PARENT_PEAK: PASS")

	// Separate RED control context: reduce only this contract's response ceiling
	// BEFORE source preparation. The original positive contract remains 1 MiB /
	// 8 MiB; source document/object/work/deadline caps are identical, never raised.
	tightContract := sourceStageResponseContract(t, 32*1024)
	tightStore := &sourceStageStore{MemoryStore: cc.NewMemoryStore()}
	tightRun := sc.Run(ctx, sc.Request{PrivateAllMode: &h, PrivateSources: &q, Workspace: qWorkspace(q), Contract: tightContract, Store: tightStore, StopAfter: sc.StopAfterPrivateSourceRequests})
	if tightRun.Err != nil || tightRun.PrivateSources == nil {
		t.Fatal("ASSERT_BIND_WIRE_CONTROL_SOURCE_PREPARED", tightRun.Err)
	}
	tightOut := tightRun.PrivateSources
	tightRequest := first
	tightRequest.Resume = sc.ResumeRequest{PrivateSources: &sc.PrivateAllModeSourcesResume{Version: sc.PrivateAllModeSourcesVersion, Expected: tightOut.Expected, Originals: q.Originals, Definitions: q.Definitions, ExpectedHandoff: h}, Workspace: qWorkspace(q), Store: tightStore, StopAfter: sc.StopAfterPrivateSourceRequests, Selector: tightOut.CheckpointID}
	tightRequest.Record = tightOut.Requests[0]
	tightRequest.Attempt = sc.SuppliedSourceAttempt{ID: first.Attempt.ID, CheckpointID: tightOut.CheckpointID, SourceUnitID: tightRequest.Record.Envelope.AdmissionID, RequestRecordID: tightRequest.Record.RecordID}
	tightRequest.Limits = describeworker.SuppliedResponseLimits{MaxAdapterBytes: 32 * 1024, MaxSemanticBytes: 32 * 1024}
	var large describeworker.SemanticResponseV2
	_ = json.Unmarshal([]byte(semantic), &large)
	large.TargetRole = strings.Repeat("x", 3000)
	large.Limitations = make([]string, 7)
	for n := range large.Limitations {
		large.Limitations[n] = strings.Repeat("l", 4096)
	}
	largeRaw, _ := json.Marshal(large)
	tightRequest.AdapterLine = describeworker.EncodeAdapterResultV2(string(largeRaw))
	candidate, e := describeworker.ParseSuppliedTerminalResponse(tightRequest.AdapterLine, tightRequest.Limits)
	rawRecord, _ := json.Marshal(tightRequest.Record)
	if e != nil || len(rawRecord) >= 32*1024 || len(candidate.CanonicalSemantic()) >= 32*1024 || len(rawRecord)+len(candidate.CanonicalSemantic()) <= 32*1024 {
		t.Fatal("ASSERT_BIND_WIRE_COMPONENTS_VALID", len(rawRecord), len(largeRaw), e)
	}
	if b, e := sc.BindSuppliedSourceResponse(ctx, tightRequest); e == nil || b.ID() != "" || !strings.Contains(e.Error(), "whole-wire ceiling") {
		t.Fatal("ASSERT_BIND_EXISTING_WHOLE_WIRE", e)
	}
	t.Log("ASSERT_BIND_EXISTING_WHOLE_WIRE: PASS; ASSERT_BIND_WIRE_COMPONENTS_VALID: PASS")
	puts := store.puts
	bindings := map[string]bool{}
	for n := range out.Requests {
		request := makeRequest(n)
		bound, e := sc.BindSuppliedSourceResponse(ctx, request)
		if e != nil || bound.ID() == "" || bindings[bound.ID()] {
			t.Fatal("ASSERT_BIND_ACTUAL_SOURCE_REQUEST", n, e)
		}
		bindings[bound.ID()] = true
		b, _ := bound.Bytes()
		var wire map[string]json.RawMessage
		_ = json.Unmarshal(b, &wire)
		if string(wire["authority"]) != "0" || string(wire["accepted"]) != "false" || string(wire["identity"]) != `"UNRESOLVED"` || string(wire["completeness"]) != `"UNKNOWN"` || string(wire["absence"]) != `"PROHIBITED"` || string(wire["origin"]) != `"SUPPLIED_TERMINAL_RESPONSE_ONLY"` || string(wire["execution"]) != `"NO_PROCESS_OR_HOST_EXECUTION_VERIFIED"` {
			t.Fatal("ASSERT_BIND_CLAIM_CEILING")
		}
		candidate, parseErr := describeworker.ParseSuppliedTerminalResponse(request.AdapterLine, limits)
		recordBytes, _ := json.Marshal(request.Record)
		quoted := func(v string) string { b, _ := json.Marshal(v); return string(b) }
		if parseErr != nil || string(wire["source_unit_id"]) != quoted(request.Attempt.SourceUnitID) || string(wire["source_digest"]) != quoted("sha256:"+request.Record.Envelope.SourceRevision) || string(wire["checkpoint_id"]) != quoted(out.CheckpointID) || !bytes.Equal(wire["request"], recordBytes) || string(wire["raw_adapter_digest"]) != quoted(candidate.RawAdapterDigest()) || string(wire["canonical_semantic_digest"]) != quoted(candidate.CanonicalSemanticDigest()) {
			t.Fatal("ASSERT_BIND_SERIALIZED_JOIN")
		}
		t.Log("ASSERT_BIND_SERIALIZED_JOIN: PASS")
		before, _ := bound.Bytes()
		b[0] = '!'
		s := bound.Semantic()
		s.Limitations[0] = "changed"
		after, _ := bound.Bytes()
		if !bytes.Equal(before, after) || bound.Semantic().Limitations[0] == "changed" {
			t.Fatal("ASSERT_BIND_DEFENSIVE")
		}
		var public sc.BoundSuppliedSourceResponse
		if json.Unmarshal(before, &public) != nil || public.ID() != "" {
			t.Fatal("ASSERT_BIND_PUBLIC_CANNOT_MINT")
		}
		again := sc.Resume(ctx, resume)
		if again.Err != nil {
			t.Fatal("ASSERT_BIND_RESUME", again.Err)
		}
		next, e := sc.BindSuppliedSourceResponse(ctx, request)
		if e != nil || next.ID() != bound.ID() {
			t.Fatal("ASSERT_BIND_EXACT_REPLAY_IDENTITY", e)
		}
		spaced := request
		spaced.AdapterLine = describeworker.EncodeAdapterResultV2(" \n" + semantic + "\t")
		normalized, e := sc.BindSuppliedSourceResponse(ctx, spaced)
		if e != nil || normalized.ID() == bound.ID() || normalized.Semantic().Verdict != bound.Semantic().Verdict {
			t.Fatal("ASSERT_BIND_NORMALIZED_SEMANTIC", e)
		}
		attempt := request
		attempt.Attempt.ID = "other-controlled-attempt"
		other, e := sc.BindSuppliedSourceResponse(ctx, attempt)
		if e != nil || other.ID() == bound.ID() {
			t.Fatal("ASSERT_BIND_ATTEMPT_IDENTITY", e)
		}
	}
	if len(bindings) != 2 || store.puts != puts {
		t.Fatal("ASSERT_BIND_ALL7_NO_PUBLICATION")
	}
	t.Log("ASSERT_BIND_ACTUAL_SOURCE_REQUEST: PASS; ASSERT_BIND_CLAIM_CEILING: PASS; ASSERT_BIND_DEFENSIVE: PASS; ASSERT_BIND_PUBLIC_CANNOT_MINT: PASS; ASSERT_BIND_EXACT_REPLAY_IDENTITY: PASS; ASSERT_BIND_NORMALIZED_SEMANTIC: PASS; ASSERT_BIND_ATTEMPT_IDENTITY: PASS; ASSERT_BIND_ALL7_COUNTS: PASS")
	for name, mutate := range map[string]func(*sc.SuppliedSourceResponseRequest){
		"request-id":    func(r *sc.SuppliedSourceResponseRequest) { r.Record.RecordID = "foreign" },
		"message-id":    func(r *sc.SuppliedSourceResponseRequest) { r.Record.Envelope.MessageID = "foreign" },
		"prompt":        func(r *sc.SuppliedSourceResponseRequest) { r.Record.Envelope.Prompt += "foreign" },
		"lineage":       func(r *sc.SuppliedSourceResponseRequest) { r.Record.Lineage.LineageIdentity = "foreign" },
		"ordinal":       func(r *sc.SuppliedSourceResponseRequest) { r.Record.Lineage.AlternativeOrdinal++ },
		"input":         func(r *sc.SuppliedSourceResponseRequest) { r.Record.Envelope.InputSHA256 = "foreign" },
		"source-digest": func(r *sc.SuppliedSourceResponseRequest) { r.Record.Envelope.SourceRevision = "foreign" },
		"admission": func(r *sc.SuppliedSourceResponseRequest) {
			r.Record.Envelope.AdmissionID = out.Requests[1].Envelope.AdmissionID
		},
		"attempt-source": func(r *sc.SuppliedSourceResponseRequest) {
			r.Attempt.SourceUnitID = out.Requests[1].Envelope.AdmissionID
		},
		"attempt-request":    func(r *sc.SuppliedSourceResponseRequest) { r.Attempt.RequestRecordID = out.Requests[1].RecordID },
		"attempt-checkpoint": func(r *sc.SuppliedSourceResponseRequest) { r.Attempt.CheckpointID = "foreign" },
		"attempt-empty":      func(r *sc.SuppliedSourceResponseRequest) { r.Attempt.ID = "" },
		"workspace":          func(r *sc.SuppliedSourceResponseRequest) { r.Resume.Workspace = "foreign" },
		"checkpoint":         func(r *sc.SuppliedSourceResponseRequest) { r.Resume.Selector = "foreign" },
		"stop":               func(r *sc.SuppliedSourceResponseRequest) { r.Resume.StopAfter = cc.StopAfterSourceIntents },
		"original": func(r *sc.SuppliedSourceResponseRequest) {
			s := *r.Resume.PrivateSources
			s.Originals = nil
			r.Resume.PrivateSources = &s
		},
		"expected": func(r *sc.SuppliedSourceResponseRequest) {
			s := *r.Resume.PrivateSources
			s.Expected = sc.PrivateAllModeSourcesExpected{}
			r.Resume.PrivateSources = &s
		},
		"handoff": func(r *sc.SuppliedSourceResponseRequest) {
			s := *r.Resume.PrivateSources
			s.ExpectedHandoff = cc.PrivateAllModeHandoff{}
			r.Resume.PrivateSources = &s
		},
		"version": func(r *sc.SuppliedSourceResponseRequest) {
			s := *r.Resume.PrivateSources
			s.Version = "foreign"
			r.Resume.PrivateSources = &s
		},
		"outer": func(r *sc.SuppliedSourceResponseRequest) { r.AdapterLine = append([]byte(" "), r.AdapterLine...) },
		"repeated": func(r *sc.SuppliedSourceResponseRequest) {
			r.AdapterLine = append(append([]byte(nil), r.AdapterLine...), r.AdapterLine...)
		},
		"semantic": func(r *sc.SuppliedSourceResponseRequest) {
			r.AdapterLine = describeworker.EncodeAdapterResultV2(`{"verdict":"COMPLETE"}`)
		},
		"resealed-public-unit": func(r *sc.SuppliedSourceResponseRequest) {
			u := out.Units[0]
			u.Body += "foreign"
			record, e := describerequest.FormatPrivateSourceRequestCandidate(u, c.DescribeDeadlineMS(), 1<<20)
			if e != nil {
				t.Fatal(e)
			}
			r.Record = record
			r.Attempt.RequestRecordID = record.RecordID
			r.Attempt.SourceUnitID = u.ID
		},
	} {
		t.Run(name, func(t *testing.T) {
			r := first
			mutate(&r)
			if b, e := sc.BindSuppliedSourceResponse(ctx, r); e == nil || b.ID() != "" {
				t.Fatal("ASSERT_BIND_JOIN_REFUSAL", name)
			}
			t.Log("ASSERT_BIND_JOIN_REFUSAL: PASS")
		})
	}
	original, e := input.SourceOriginalsV1()
	if e != nil {
		t.Fatal(e)
	}
	substituted := first
	s := *resume.PrivateSources
	s.Originals = []inv.PrivateProducerOriginals{original}
	substituted.Resume.PrivateSources = &s
	if _, e = sc.BindSuppliedSourceResponse(ctx, substituted); e == nil {
		t.Fatal("ASSERT_BIND_OPAQUE_ORIGINAL_SUBSTITUTION")
	}
	// Exact expected handle retained but persisted proof bytes altered.
	persisted := *out
	persisted.CheckpointID = ""
	sourceBytes, _ := json.Marshal(persisted)
	requestBytes, _ := describerequest.Bytes(out.Requests)
	for _, id := range []string{out.CheckpointID, fmt.Sprintf("sha256:%x", sha256.Sum256(sourceBytes)), fmt.Sprintf("sha256:%x", sha256.Sum256(requestBytes))} {
		store.corrupt = id
		if _, e = sc.BindSuppliedSourceResponse(ctx, first); e == nil {
			t.Fatal("ASSERT_BIND_PERSISTED_PROOFS")
		}
	}
	store.corrupt = ""
	worker := &sourceStageWorker{}
	for name, mutate := range map[string]func(*sc.SuppliedSourceResponseRequest){
		"huge":           func(r *sc.SuppliedSourceResponseRequest) { r.AdapterLine = make([]byte, 512*1024+1) },
		"worker":         func(r *sc.SuppliedSourceResponseRequest) { r.Resume.Worker = worker },
		"fresh-capture":  func(r *sc.SuppliedSourceResponseRequest) { r.Resume.FreshCapture = &cc.FreshCaptureDependencies{} },
		"invalid-limits": func(r *sc.SuppliedSourceResponseRequest) { r.Limits.MaxSemanticBytes = 64*1024 + 1 },
	} {
		t.Run(name, func(t *testing.T) {
			r := first
			mutate(&r)
			before := store.gets
			if _, e = sc.BindSuppliedSourceResponse(ctx, r); e == nil || store.gets != before || worker.calls != 0 {
				t.Fatal("ASSERT_BIND_ACTUAL_PREFLIGHT", name)
			}
			t.Log("ASSERT_BIND_ACTUAL_PREFLIGHT: PASS")
		})
	}
	stopped, stop := context.WithCancel(ctx)
	stop()
	before := store.gets
	if _, e = sc.BindSuppliedSourceResponse(stopped, first); e == nil || store.gets != before {
		t.Fatal("ASSERT_BIND_CANCEL_BEFORE_GET")
	}
	if worker.calls != 0 || store.puts != puts {
		t.Fatal("ASSERT_BIND_NO_EXECUTION")
	}
	if _, e = sc.BindSuppliedSourceResponse(ctx, first); e != nil {
		t.Fatal("ASSERT_BIND_RESTORED", e)
	}
	t.Log("ASSERT_BIND_OPAQUE_ORIGINAL_SUBSTITUTION: PASS; ASSERT_BIND_PERSISTED_PROOFS: PASS; ASSERT_BIND_CANCEL_BEFORE_GET: PASS; ASSERT_BIND_NO_EXECUTION: PASS; ASSERT_BIND_RESTORED: PASS; source=32 unresolved=24 requests=2 bindings=2 workers=0")
}

// Test-only, call-local context. runtime.Caller locates the one final liveness
// seam (including inline frames); no callback/hook/global clock enters production.
// Removing that public binder guard leaves publicationSeen false and a nonzero
// handle, so the named publication assertion fails rather than moving cancel
// earlier in the call sequence. Done/Err cancellation remains sticky and real.
type sourcePublicationCancelContext struct {
	context.Context
	cancel          context.CancelFunc
	publicationSeen bool
}

func (c *sourcePublicationCancelContext) Err() error {
	if e := c.Context.Err(); e != nil {
		return e
	}
	for depth := 1; depth <= 4; depth++ {
		pc, _, _, ok := runtime.Caller(depth)
		if ok && strings.HasSuffix(runtime.FuncForPC(pc).Name(), ".sourceResponsePublicationLive") {
			c.publicationSeen = true
			c.cancel()
			return c.Context.Err()
		}
	}
	return nil
}

func TestPrivateSourceConnectedAllSevenAndResume(t *testing.T) {
	input, h, c, q := sourceStageFixture(t, "positive", false)
	inv.ControlledSourceProjectionNoReadmitForTest(t, input)
	store := &sourceStageStore{MemoryStore: cc.NewMemoryStore()}
	worker := &sourceStageWorker{}
	base := cc.Run(context.Background(), cc.Request{PrivateAllMode: &h, Workspace: qWorkspace(q), Contract: c, Store: store, StopAfter: cc.StopAfterSourceIntents})
	if base.Err != nil {
		t.Fatal(base.Err)
	}
	before := store.puts
	ctx, cancel := context.WithDeadline(context.Background(), q.Deadline)
	defer cancel()
	req := sc.Request{PrivateAllMode: &h, PrivateSources: &q, Workspace: qWorkspace(q), Contract: c, Store: store, Worker: worker, StopAfter: cc.StopAfterPrivateSourceRequests}
	got := sc.Run(ctx, req)
	if got.Err != nil || got.PrivateSources == nil {
		t.Fatal("ASSERT_CONNECTED_SOURCE_PAUSE", got.Err)
	}
	out := got.PrivateSources
	if got.Status != cc.StatusPaused || out.Counts.Admitted == 0 || out.Counts.Requests == 0 || out.Counts.Admitted+out.Counts.Unresolved != out.Counts.Intents || worker.calls != 0 || store.puts-before != 7 {
		t.Fatal("ASSERT_CONNECTED_SOURCE_BALANCE", out.Counts, worker.calls, store.puts-before, out.Outcomes, base.PrivateAllMode.Intents[0])
	}
	if out.ClaimCeiling != base.PrivateAllMode.ClaimCeiling || !strings.Contains(out.ClaimCeiling, "NO_DESCRIBE") {
		t.Fatal("ASSERT_SOURCE_GROUPING_CEILING_UNCHANGED")
	}
	modes := map[string]bool{}
	roles := map[string]bool{}
	for n, o := range out.Outcomes {
		if o.Intent != n {
			t.Fatal("ASSERT_SOURCE_INTENT_MULTIPLICITY_ORDER")
		}
		i := base.PrivateAllMode.Intents[o.Intent]
		if i.Seed != 7 || i.Slot < 0 || i.Slot >= 7 || i.Mode != []string{"RD", "C", "CRD", "D", "CD", "R", "CR"}[i.Slot] {
			t.Fatal("ASSERT_SOURCE_SEED_SCHEDULE")
		}
		if i.Endpoint.Disposition == "EXCLUDED" && (o.Status != "UNRESOLVED" || o.Gap != "GROUPING_ENDPOINT_EXCLUDED") {
			t.Fatal("ASSERT_SOURCE_EXCLUDED_RETAINED")
		}
		if o.Status == "SOURCE_ADMITTED" {
			i := base.PrivateAllMode.Intents[o.Intent]
			modes[i.Mode] = true
			roles[i.Endpoint.Kind+"/"+i.Endpoint.Role] = true
		}
	}
	if out.Counts.Admitted != 32 || out.Counts.Unresolved != 24 || out.Counts.Requests != 2 || out.Counts.Intents != 56 {
		t.Fatal("ASSERT_SOURCE_EXACT_ORIGINAL_COUNTS", out.Counts)
	}
	if len(modes) != 7 {
		t.Fatalf("ASSERT_SOURCE_ALL_SEVEN_POSITIVE: modes=%v outcomes=%+v", modes, out.Outcomes)
	}
	for _, role := range []string{"CALLS/CALLER_OCCURRENCE", "CALLS/CALLEE_TARGET", "REFERENCES_SYMBOL/REFERENCED_SYMBOL", "REFERENCES_SYMBOL/REFERENCING_OCCURRENCE", "RESOLVES_TO_DEFINITION/QUERY_OCCURRENCE", "RESOLVES_TO_DEFINITION/DEFINITION_TARGET"} {
		if !roles[role] {
			t.Fatal("ASSERT_SOURCE_POSITIVE_FULL_DEFINITION_ROLE", role, out.Outcomes)
		}
	}
	if describerequest.Validate(out.Requests) != nil {
		t.Fatal("ASSERT_SOURCE_EXISTING_RECORD_VALIDATE")
	}
	for _, r := range out.Requests {
		if r.Lineage.ConsumerResolution != describerequest.PrivateSourceOnly || r.Lineage.Authority != 0 || r.Lineage.Accepted || r.Envelope.SourceRevision != strings.TrimPrefix(q.Objects[0].Identity.Digest, "sha256:") || (!strings.Contains(r.Envelope.Prompt, "func Root(){ Other() }") && !strings.Contains(r.Envelope.Prompt, "func Other(){ Root() }")) || strings.Contains(r.Envelope.Prompt, "source-run-") || strings.Contains(r.Envelope.Prompt, "source-lineage-") || strings.Contains(r.Envelope.Prompt, "source-case-") {
			t.Fatal("ASSERT_SOURCE_ONLY_PROMPT_SEPARATION", r)
		}
	}
	for _, mutate := range []func(*describerequest.Record){func(r *describerequest.Record) { r.Envelope.Prompt += "substitution" }, func(r *describerequest.Record) { r.Envelope.SourceRevision = "foreign" }, func(r *describerequest.Record) { r.Envelope.AdmissionID = "foreign" }, func(r *describerequest.Record) { r.Lineage.Authority = 1 }, func(r *describerequest.Record) { r.Lineage.Accepted = true }, func(r *describerequest.Record) { r.Lineage.SourceGraphComplete = "COMPLETE" }, func(r *describerequest.Record) { r.RecordID = "foreign" }} {
		r := out.Requests[0]
		mutate(&r)
		if describerequest.ValidateRecord(r) == nil {
			t.Fatal("ASSERT_SOURCE_RECORD_FIELD_INTEGRITY")
		}
	}
	t.Log("ASSERT_SOURCE_RECORD_FIELD_INTEGRITY: PASS; ASSERT_SOURCE_GROUPING_CEILING_UNCHANGED: PASS; ASSERT_SOURCE_INTENT_MULTIPLICITY_ORDER: PASS; ASSERT_SOURCE_SEED_SCHEDULE: PASS")
	resume := sc.ResumeRequest{PrivateSources: &sc.PrivateAllModeSourcesResume{Version: sc.PrivateAllModeSourcesVersion, Expected: out.Expected, Originals: q.Originals, Definitions: q.Definitions, ExpectedHandoff: h}, Workspace: req.Workspace, Store: store, Worker: worker, StopAfter: req.StopAfter, Selector: out.CheckpointID}
	before = store.puts
	again := sc.Resume(ctx, resume)
	if again.Err != nil || again.CheckpointID != got.CheckpointID || store.puts != before || worker.calls != 0 {
		t.Fatal("ASSERT_SOURCE_EXACT_RESUME_NO_PUT", again.Err)
	}
	secondOriginals, secondErr := input.SourceOriginalsV1()
	if secondErr != nil {
		t.Fatal(secondErr)
	}
	rSub := resume
	sSub := *resume.PrivateSources
	sSub.Originals = []inv.PrivateProducerOriginals{secondOriginals}
	rSub.PrivateSources = &sSub
	if bad := sc.Resume(ctx, rSub); bad.Err == nil {
		t.Fatal("ASSERT_SOURCE_EXACT_OPAQUE_SUBSTITUTION")
	}
	t.Log("ASSERT_SOURCE_EXACT_OPAQUE_SUBSTITUTION: PASS")
	for _, mutate := range []func(*sc.ResumeRequest){func(r *sc.ResumeRequest) { r.Selector = "sha256:" + strings.Repeat("2", 64) }, func(r *sc.ResumeRequest) { r.StopAfter = cc.StopAfterSourceIntents }, func(r *sc.ResumeRequest) {
		s := *r.PrivateSources
		s.Version = cc.PrivateAllModeVersion
		r.PrivateSources = &s
	}, func(r *sc.ResumeRequest) { s := *r.PrivateSources; s.Originals = nil; r.PrivateSources = &s }, func(r *sc.ResumeRequest) {
		s := *r.PrivateSources
		s.Expected = sc.PrivateAllModeSourcesExpected{}
		r.PrivateSources = &s
	}, func(r *sc.ResumeRequest) {
		s := *r.PrivateSources
		s.ExpectedHandoff = cc.PrivateAllModeHandoff{}
		r.PrivateSources = &s
	}} {
		r := resume
		mutate(&r)
		if bad := sc.Resume(ctx, r); bad.Err == nil || store.puts != before || worker.calls != 0 {
			t.Fatal("ASSERT_SOURCE_RESUME_SUBSTITUTION")
		}
	}
	persisted := *out
	persisted.CheckpointID = ""
	sourceBytes, _ := json.Marshal(persisted)
	requestBytes, _ := describerequest.Bytes(out.Requests)
	for _, id := range []string{out.CheckpointID, base.CheckpointID, fmt.Sprintf("sha256:%x", sha256.Sum256(sourceBytes)), fmt.Sprintf("sha256:%x", sha256.Sum256(requestBytes))} {
		store.corrupt = id
		if bad := sc.Resume(ctx, resume); bad.Err == nil || store.puts != before || worker.calls != 0 {
			t.Fatal("ASSERT_SOURCE_RESUME_TAMPER", id)
		}
	}
	store.corrupt = ""
	unowned := cc.Run(ctx, cc.Request{PrivateAllMode: &h, PrivateSources: &cc.PrivateAllModeSourcesRequest{PolicyID: q.PolicyID, Limits: q.Limits, Deadline: q.Deadline}, Workspace: req.Workspace, Contract: c, Store: store, Worker: worker, StopAfter: req.StopAfter})
	if unowned.Err == nil || store.puts != before || worker.calls != 0 {
		t.Fatal("ASSERT_CONNECTED_LOWER_SOURCE_REFUSED")
	}
	cancelled, stop := context.WithCancel(ctx)
	stop()
	before = store.puts
	if bad := sc.Run(cancelled, req); bad.Err == nil || store.puts != before {
		t.Fatal("ASSERT_SOURCE_CANCEL_BEFORE_PUT")
	}
	store.fail = true
	if bad := sc.Run(ctx, req); bad.Err == nil {
		t.Fatal("ASSERT_SOURCE_STORE_FAILURE")
	}
	store.fail = false
	legacy := cc.Run(context.Background(), cc.Request{PrivateAllMode: &h, Workspace: req.Workspace, Contract: c, Store: store, StopAfter: cc.StopAfterSourceIntents})
	if legacy.Err != nil || legacy.CheckpointID != base.CheckpointID || legacy.PrivateAllMode.Counts != base.PrivateAllMode.Counts {
		t.Fatal("ASSERT_SOURCE_V1_CONTINUITY", legacy.Err)
	}
	// Every endpoint join field is independently substituted against genuine
	// original handles. Returned query coordinates are selected internally.
	prepared, prepareErr := inv.PreparePrivateProducerOriginals(q.Originals[0], q.Limits)
	if prepareErr != nil {
		t.Fatal(prepareErr)
	}
	seenEndpoint := map[tp.PrivateTypedEndpoint]bool{}
	for _, i := range base.PrivateAllMode.Intents {
		if seenEndpoint[i.Endpoint] {
			continue
		}
		seenEndpoint[i.Endpoint] = true
		if i.Endpoint.Kind == "" || i.Endpoint.Disposition == "EXCLUDED" {
			continue
		}
		a, gap := inv.AdmitPrivateEndpointSource(i, prepared, nil, q.Objects, q.Limits)
		if !a.Valid() {
			t.Fatal("ASSERT_SOURCE_DIRECT_POSITIVE", gap)
		}
		for _, mutate := range []func(*tp.PrivateSourceIntent){func(x *tp.PrivateSourceIntent) { x.InputID = "foreign" }, func(x *tp.PrivateSourceIntent) { x.Endpoint.Node = "foreign" }, func(x *tp.PrivateSourceIntent) { x.Endpoint.Occurrence = "foreign" }, func(x *tp.PrivateSourceIntent) { x.Endpoint.Role = "foreign" }, func(x *tp.PrivateSourceIntent) { x.Endpoint.Predecessor = "foreign" }, func(x *tp.PrivateSourceIntent) { x.Endpoint.Custody = "foreign" }, func(x *tp.PrivateSourceIntent) { x.Endpoint.Disposition = "EMPTY" }, func(x *tp.PrivateSourceIntent) { x.Endpoint.Ordinal++ }} {
			v := i
			mutate(&v)
			if bad, _ := inv.AdmitPrivateEndpointSource(v, prepared, nil, q.Objects, q.Limits); bad.Valid() {
				t.Fatal("ASSERT_SOURCE_ENDPOINT_SUBSTITUTION", v)
			}
		}
	}
	// Public original-query field substitutions cannot remint producer custody.
	for _, path := range []string{"Query.URI", "Query.Line", "Query.Character", "Query.SessionID", "Query.Generation", "Query.DocumentVersion", "Query.Encoding", "Query.SourceDigest", "Target.SymbolID", "Target.SymbolName", "Target.DocumentVersion", "Target.Encoding", "Target.SourceDigest", "Revision", "Privacy", "Supply.URI", "Supply.DocumentVersion", "Supply.Content", "Symbol.SessionID", "Symbol.Generation", "Symbol.RequestID"} {
		bad := input
		bad.Predecessors = append([]rg.PredecessorRecord(nil), input.Predecessors...)
		for n, p := range bad.Predecessors {
			if p.Role != "METHOD_QUERY" {
				continue
			}
			var fields map[string]any
			if e := json.Unmarshal(p.Bytes, &fields); e != nil {
				t.Fatal(e)
			}
			parts := strings.Split(path, ".")
			if len(parts) == 1 {
				fields[parts[0]] = "substitution"
			} else {
				nested := fields[parts[0]].(map[string]any)
				switch nested[parts[1]].(type) {
				case float64:
					nested[parts[1]] = float64(987)
				default:
					nested[parts[1]] = "substitution"
				}
			}
			raw, _ := json.Marshal(fields)
			bad.Predecessors[n].Bytes = append(raw, '\n')
		}
		if _, e := bad.SourceOriginalsV1(); e == nil {
			t.Fatal("ASSERT_SOURCE_ORIGINAL_FIELD_SUBSTITUTION", path)
		}
	}
	t.Log("ASSERT_SOURCE_ORIGINAL_FIELD_SUBSTITUTION: PASS; Query/Target/session/generation/version/encoding/revision/privacy/source digest/supply/native transaction fields")
	// An already-issued original handle must not retain public backing arrays.
	for _, p := range input.Predecessors {
		if p.Role == "METHOD_QUERY" {
			before := p.Bytes[0]
			p.Bytes[0] ^= 1
			_, aliasErr := inv.PreparePrivateProducerOriginals(q.Originals[0], q.Limits)
			p.Bytes[0] = before
			if aliasErr != nil {
				t.Fatal("ASSERT_SOURCE_ORIGINALS_NOT_PUBLIC_ALIAS", aliasErr)
			}
		}
	}
	t.Log("ASSERT_SOURCE_ORIGINALS_NOT_PUBLIC_ALIAS: PASS")
	// Confirm public diagnostic mutation cannot remint producer originals.
	input.Ledger[0].Ordinal++
	if _, e := input.SourceOriginalsV1(); e == nil {
		t.Fatal("ASSERT_SOURCE_ORIGINAL_GETTER_NOT_DTO")
	}
	t.Logf("ASSERT_SOURCE_ALL_SEVEN_POSITIVE: PASS; modes=%v roles=%v admitted=%d unresolved=%d requests=%d worker_calls=%d", modes, roles, out.Counts.Admitted, out.Counts.Unresolved, out.Counts.Requests, worker.calls)
	t.Log("ASSERT_SOURCE_ONLY_PROMPT_SEPARATION: PASS; ASSERT_SOURCE_EXACT_RESUME_NO_PUT: PASS; ASSERT_SOURCE_RESUME_SUBSTITUTION: PASS; ASSERT_SOURCE_ENDPOINT_SUBSTITUTION: PASS; ASSERT_SOURCE_V1_CONTINUITY: PASS")
}

// Workspace is supplied explicitly from a verified admitted unit, never from a
// path scan. The fixture's root is the fixed producer binding's exact workspace.
func qWorkspace(q sc.PrivateAllModeSourcesRequest) string { return q.Originals[0].Workspace() }

func TestPrivateSourceOriginalsUnavailableAndGaps(t *testing.T) {
	if _, e := (inv.PrivateManagedGroupingInput{}).SourceOriginalsV1(); e == nil {
		t.Fatal("ASSERT_SOURCE_ZERO_ORIGINALS")
	}
	sourceStageFixture(t, "uncertain", true)
	for _, mode := range []string{"foreign", "snippet"} {
		t.Run(mode, func(t *testing.T) {
			_, h, c, q := sourceStageFixture(t, mode, false)
			ctx, cancel := context.WithDeadline(context.Background(), q.Deadline)
			defer cancel()
			store := &sourceStageStore{MemoryStore: cc.NewMemoryStore()}
			got := sc.Run(ctx, sc.Request{PrivateAllMode: &h, PrivateSources: &q, Workspace: qWorkspace(q), Contract: c, Store: store, StopAfter: sc.StopAfterPrivateSourceRequests})
			if got.Err != nil {
				t.Fatal(got.Err)
			}
			o := got.PrivateSources
			base := cc.Run(ctx, cc.Request{PrivateAllMode: &h, Workspace: qWorkspace(q), Contract: c, Store: store, StopAfter: cc.StopAfterSourceIntents})
			if base.Err != nil {
				t.Fatal(base.Err)
			}
			for _, row := range o.Outcomes {
				i := base.PrivateAllMode.Intents[row.Intent]
				if i.Endpoint.Disposition == "EXCLUDED" {
					continue
				}
				returned := i.Endpoint.Kind == "REFERENCES_SYMBOL" && i.Endpoint.Role == "REFERENCING_OCCURRENCE" || i.Endpoint.Kind == "RESOLVES_TO_DEFINITION" && i.Endpoint.Role == "DEFINITION_TARGET"
				if returned && (row.Status != "UNRESOLVED" || row.Gap != "OWN_FULL_DEFINITION_OR_VERIFIED_SOURCE_UNAVAILABLE") {
					t.Fatal("ASSERT_SOURCE_RETURNED_FULL_DEFINITION_REQUIRED", mode, row)
				}
			}
			t.Log("ASSERT_SOURCE_RETURNED_FULL_DEFINITION_REQUIRED: PASS", mode)
			if o.Counts.Admitted == 0 || o.Counts.Unresolved == 0 || o.Counts.Intents != o.Counts.Admitted+o.Counts.Unresolved {
				t.Fatal("ASSERT_SOURCE_FOREIGN_OR_SNIPPET_GAPS", o.Counts)
			}
			for _, r := range o.Outcomes {
				if r.Status == "UNRESOLVED" && r.Gap == "" {
					t.Fatal("ASSERT_SOURCE_TYPED_GAP_REQUIRED")
				}
			}
			t.Log("ASSERT_SOURCE_FOREIGN_OR_SNIPPET_GAPS: PASS", mode, o.Counts)
		})
	}
	_, h, c, q := sourceStageFixture(t, "positive", false)
	ctx, cancel := context.WithDeadline(context.Background(), q.Deadline)
	defer cancel()
	store := &sourceStageStore{MemoryStore: cc.NewMemoryStore()}
	workspace := qWorkspace(q)
	q.Originals = nil
	got := sc.Run(ctx, sc.Request{PrivateAllMode: &h, PrivateSources: &q, Workspace: workspace, Contract: c, Store: store, StopAfter: sc.StopAfterPrivateSourceRequests})
	if got.Err != nil || got.PrivateSources.Counts.Admitted != 0 || got.PrivateSources.Counts.Unresolved != got.PrivateSources.Counts.Intents || got.RequestCount != 0 {
		t.Fatal("ASSERT_SOURCE_NO_PROOF_NOT_EMPTY", got.Err)
	}
}

func TestPrivateSourceLocationLinkSelectionAndEnvelope(t *testing.T) {
	for index, mode := range []string{"link", "link-envelope", "link-selection", "link-foreign"} {
		t.Run(fmt.Sprint(index), func(t *testing.T) {
			_, h, c, q := sourceStageFixture(t, mode, false)
			ctx, cancel := context.WithDeadline(context.Background(), q.Deadline)
			defer cancel()
			store := &sourceStageStore{MemoryStore: cc.NewMemoryStore()}
			base := cc.Run(ctx, cc.Request{PrivateAllMode: &h, Workspace: qWorkspace(q), Contract: c, Store: store, StopAfter: cc.StopAfterSourceIntents})
			got := sc.Run(ctx, sc.Request{PrivateAllMode: &h, PrivateSources: &q, Workspace: qWorkspace(q), Contract: c, Store: store, StopAfter: sc.StopAfterPrivateSourceRequests})
			if got.Err != nil || base.Err != nil {
				t.Fatal(got.Err, base.Err)
			}
			seen := 0
			for _, row := range got.PrivateSources.Outcomes {
				i := base.PrivateAllMode.Intents[row.Intent]
				if i.Endpoint.Kind != "RESOLVES_TO_DEFINITION" || i.Endpoint.Role != "DEFINITION_TARGET" || i.Endpoint.Disposition == "EXCLUDED" {
					continue
				}
				seen++
				if (row.Status == "SOURCE_ADMITTED") != (mode == "link") {
					t.Fatal("ASSERT_LOCATIONLINK_DISTINCT_SELECTION_ENVELOPE", mode, row)
				}
			}
			if seen == 0 {
				t.Fatal("ASSERT_LOCATIONLINK_NONVACUOUS")
			}
			t.Log("ASSERT_LOCATIONLINK_DISTINCT_SELECTION_ENVELOPE: PASS", mode)
		})
	}
}

func TestPrivateSourceManagerOnlyAndPendingCompletion(t *testing.T) {
	for _, pending := range []bool{false, true} {
		t.Run(fmt.Sprint(pending), func(t *testing.T) {
			m, native, window := mixedLeaseFixtureV2(t)
			owner, root := inv.ControlledManagedInputOwnerForTest(t, false)
			var unfinished inv.PrivateManagedGroupingInput
			err := m.WithReadySessionLease(window, native.Session, func(ctx context.Context) error {
				h, err := m.CapturePrivateGroupingOwned(ctx, sessionruntime.PrivateGroupingOwnedRequest{Session: native.Session, URI: native.Directional.URI, OccurrenceID: "uncompleted", Revision: native.Revision, Privacy: native.Privacy, Line: *native.Directional.Line, Character: *native.Directional.Character, References: true, Definitions: true, Deadline: native.Directional.Deadline})
				if err != nil {
					return err
				}
				if pending {
					q := inv.PrivateManagedGroupingRequest{Session: native.Session, URI: native.Directional.URI, OccurrenceID: "uncompleted", Revision: native.Revision, Privacy: native.Privacy, Line: *native.Directional.Line, Character: *native.Directional.Character, Deadline: native.Directional.Deadline, OccurrenceAdmissionPolicyID: "ordinal", CompositionPolicyID: "identity", Kinds: []programcadmission.RelationKind{programcadmission.TypedReferencesSymbol, programcadmission.TypedResolvesToDefinition}, Root: root}
					unfinished = inv.ControlledPendingSourceIssueForTest(t, ctx, m, h, q, owner)
					return nil
				}
				if err = m.ConsumePrivateGroupingOwned(ctx, h, func(sessionruntime.PrivateGroupingOwnedView) error { return nil }); err != nil {
					return err
				}
				evidence, err := m.TakePrivateGroupingSourceEvidence(ctx, h)
				if err != nil {
					return err
				}
				v, err := evidence.Snapshot(sessionruntime.PrivateGroupingQueryMaxBytes)
				if err != nil {
					return err
				}
				inv.ControlledNonProducerCandidateForTest(t, evidence, v)
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if pending {
				failed, stop := context.WithCancel(window)
				stop()
				completion, finishErr := m.FinishPrivateProviderWindow(failed)
				if finishErr == nil && completion.Completed {
					t.Fatal("ASSERT_FAILED_FINISH_CONTROL")
				}
				if _, err := unfinished.SourceOriginalsV1(); err == nil {
					t.Fatal("ASSERT_FAILED_FINISH_NO_ORIGINALS")
				}
				t.Log("ASSERT_FAILED_FINISH_NO_ORIGINALS: PASS")
			}
		})
	}
}

// These counterexamples call the actual successful legacy entries, not the
// lower candidate builder. Report only after the active lease and workers drain.
func TestPrivateSourceLegacyIssueRequiresJoinedFinish(t *testing.T) {
	privateSourceLegacyCompletionGuard(t, true)
}

func TestPrivateSourceLegacyAcquireRequiresJoinedFinish(t *testing.T) {
	privateSourceLegacyCompletionGuard(t, false)
}

func privateSourceLegacyCompletionGuard(t *testing.T, issue bool) {
	t.Helper()
	m, start, uri := mixedStartFixtureV2(t)
	// The legacy entries issue R+D, not Calls. Use their exact existing schedule;
	// a Calls window would intentionally remain incomplete at Finish.
	envelope, err := m.PrivateProviderWindowEnvelope(start, sessionruntime.PrivateProviderWindowPlan{References: true, Definitions: true})
	if err != nil {
		t.Fatal(err)
	}
	window, err := m.PrivateProviderWindowContext(context.Background(), envelope, start.Deadline)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = m.FinishPrivateProviderWindow(window) })
	started := m.Start(window, start)
	if started.Failure != "" {
		t.Fatal(started.Failure)
	}
	pending := m.BeginReadiness(window, started.SessionID, started.Generation, start.Deadline)
	ready, ok := m.WaitReadiness(window, pending.ID)
	if !ok || ready.State != sessionruntime.ReadinessReady {
		t.Fatalf("ASSERT_LEGACY_SOURCE_READY: %+v", ready)
	}
	session := sessionruntime.SessionLeaseRequest{SessionID: started.SessionID, Generation: started.Generation, Workspace: start.Profile.Workspace().String()}
	owner, root := inv.ControlledManagedInputOwnerForTest(t, false)
	q := inv.PrivateManagedGroupingRequest{Session: session, URI: uri, OccurrenceID: "legacy-source-completion", Revision: "controlled-mixed-revision", Privacy: "private-mixed-only", Line: 1, Character: 6, Deadline: start.Deadline, OccurrenceAdmissionPolicyID: "ordinal", CompositionPolicyID: "identity", Kinds: []programcadmission.RelationKind{programcadmission.TypedReferencesSymbol, programcadmission.TypedResolvesToDefinition}, Root: root}
	var input inv.PrivateManagedGroupingInput
	var originals inv.PrivateProducerOriginals
	var sourceErr error
	checked := false
	reserve := func(ctx context.Context, mode string, h sessionruntime.PrivateGroupingOwnedHandoff) error {
		binding, err := inv.PrivateManagedGroupingOperationBinding(m, q, mode, h)
		if err != nil {
			return err
		}
		q.Operation, err = owner.Reserve(ctx, fmt.Sprintf("sha256:%x", sha256.Sum256([]byte("legacy-source-"+mode))), binding)
		return err
	}
	if issue {
		err = m.WithReadySessionLease(window, session, func(ctx context.Context) error {
			h, e := m.CapturePrivateGroupingOwned(ctx, sessionruntime.PrivateGroupingOwnedRequest{Session: q.Session, URI: q.URI, OccurrenceID: q.OccurrenceID, Revision: q.Revision, Privacy: q.Privacy, Line: q.Line, Character: q.Character, References: true, Definitions: true, Deadline: q.Deadline})
			if e != nil {
				return e
			}
			if e = reserve(ctx, "ISSUE", h); e != nil {
				return e
			}
			input, e = inv.IssuePrivateManagedGroupingInput(ctx, m, h, q)
			if e != nil {
				return e
			}
			// Observe succeeded, but this callback has NOT returned its lease and
			// Finish has NOT run. Minting here is the premature-eligibility bypass.
			originals, sourceErr = input.SourceOriginalsV1()
			checked = true
			return nil
		})
	} else {
		err = reserve(window, "ACQUIRE", sessionruntime.PrivateGroupingOwnedHandoff{})
		if err == nil {
			input, err = inv.AcquirePrivateManagedGroupingInput(window, m, q)
		}
		if err == nil {
			// Acquire returned its inner lease and confirmed Observe, but did not
			// join this provider window. It is still not a source producer.
			originals, sourceErr = input.SourceOriginalsV1()
			checked = true
		}
	}
	completion, finishErr := m.FinishPrivateProviderWindow(window)
	if finishErr != nil || !completion.Completed || m.Census().Workers != 0 {
		t.Fatalf("ASSERT_LEGACY_SOURCE_CONTROL_DRAINED: completion=%+v err=%v workers=%d entry=%v", completion, finishErr, m.Census().Workers, err)
	}
	if err != nil || !checked || input.Input.ID() == "" || len(input.Ledger) == 0 {
		t.Fatalf("ASSERT_LEGACY_SOURCE_ENTRY_ACTUALLY_SUCCEEDED: issue=%t checked=%t err=%v", issue, checked, err)
	}
	assertion := "ASSERT_LEGACY_ACQUIRE_NO_ORIGINALS_WITHOUT_FINISH"
	if issue {
		assertion = "ASSERT_LEGACY_ISSUE_NO_ORIGINALS_INSIDE_ACTIVE_LEASE"
	}
	if sourceErr == nil || originals.Bound() != 0 {
		t.Fatal(assertion)
	}
	if _, e := input.SourceOriginalsV1(); e == nil {
		t.Fatal("ASSERT_LEGACY_FINISH_DOES_NOT_RETROACTIVELY_MINT")
	}
	t.Log("ASSERT_LEGACY_SOURCE_ENTRY_ACTUALLY_SUCCEEDED: PASS; confirmed Observe, typed input retained; lease and workers drained")
	t.Log(assertion + ": PASS")
}

func TestPrivateSourceVerifiedBytesAndBounds(t *testing.T) {
	_, h, c, q := sourceStageFixture(t, "positive", false)
	workspace := qWorkspace(q)
	ctx, cancel := context.WithDeadline(context.Background(), q.Deadline)
	defer cancel()
	store := &sourceStageStore{MemoryStore: cc.NewMemoryStore()}
	originals := q.Objects[0]
	foreign := append([]byte(nil), originals.Bytes...)
	foreign[0] ^= 1
	for _, object := range []sourceobject.Object{{Identity: originals.Identity, Bytes: foreign}, {Identity: sourceobject.Identity{Digest: fmt.Sprintf("sha256:%x", sha256.Sum256(foreign)), ByteLength: uint64(len(foreign))}, Bytes: foreign}, {Identity: sourceobject.Identity{Digest: originals.Identity.Digest, ByteLength: uint64(len(originals.Bytes) - 1)}, Bytes: originals.Bytes[:len(originals.Bytes)-1]}} {
		local := q
		local.Objects = []sourceobject.Object{object}
		got := sc.Run(ctx, sc.Request{PrivateAllMode: &h, PrivateSources: &local, Workspace: workspace, Contract: c, Store: store, StopAfter: sc.StopAfterPrivateSourceRequests})
		if got.Err != nil {
			t.Fatal(got.Err)
		}
		if got.PrivateSources.Counts.Admitted != 0 || got.PrivateSources.Counts.Requests != 0 {
			t.Fatal("ASSERT_SOURCE_FOREIGN_BYTES_NEVER_ADMITTED")
		}
	}
	for _, mutate := range []func(*sc.PrivateAllModeSourcesRequest){func(x *sc.PrivateAllModeSourcesRequest) { x.Limits.MaxWork = 1 }, func(x *sc.PrivateAllModeSourcesRequest) { x.PolicyID = "foreign" }, func(x *sc.PrivateAllModeSourcesRequest) { x.Originals = []inv.PrivateProducerOriginals{{}} }} {
		local := q
		mutate(&local)
		before := store.puts
		bad := sc.Run(ctx, sc.Request{PrivateAllMode: &h, PrivateSources: &local, Workspace: workspace, Contract: c, Store: store, StopAfter: sc.StopAfterPrivateSourceRequests})
		if bad.Err == nil || store.puts != before {
			t.Fatal("ASSERT_SOURCE_PREFLIGHT_ZERO_PUT", bad.Err)
		}
	}
	local := q
	local.Limits.MaxDocumentBytes = len(originals.Bytes) - 1
	before := store.puts
	got := sc.Run(ctx, sc.Request{PrivateAllMode: &h, PrivateSources: &local, Workspace: workspace, Contract: c, Store: store, StopAfter: sc.StopAfterPrivateSourceRequests})
	if got.Err == nil || store.puts != before {
		t.Fatal("ASSERT_SOURCE_BOUND_BEFORE_PUT")
	}
	if !bytes.Equal(originals.Bytes, q.Objects[0].Bytes) {
		t.Fatal("ASSERT_SOURCE_FIXTURE_BYTES_IMMUTABLE")
	}
	t.Log("ASSERT_SOURCE_FOREIGN_BYTES_NEVER_ADMITTED: PASS; ASSERT_SOURCE_BOUND_BEFORE_PUT: PASS")
}
