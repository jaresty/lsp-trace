//go:build darwin || linux

package inventorysuccessor_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"lsp-trace/internal/describeworker"
	inv "lsp-trace/internal/inventorysuccessor"
	so "lsp-trace/internal/inventorysuccessor/sourceoperator"
	host "lsp-trace/internal/privatedescribeprep"
	"lsp-trace/internal/programcadmission"
	"lsp-trace/internal/publication"
	tp "lsp-trace/internal/targetpacket"
)

const managedSemanticFixtureCeiling = 2048
const localWorkerSemanticFixture = `{"verdict":"ABSTAINED","target_role":"controlled source fixture","consumer_need":{"status":"UNRESOLVED","value":""},"provided_behavior":{"value":"fixture body only","consumer_relative":false},"boundary_contribution":"unknown","limitations":["controlled fake worker; not model qualification"]}`

// Pure parser control: no managed owner, shell, SELF responder or worker starts.
func TestSourceOperatorManagedSemanticFixtureParser(t *testing.T) {
	limits := describeworker.SuppliedResponseLimits{MaxAdapterBytes: 8192, MaxSemanticBytes: managedSemanticFixtureCeiling}
	payload := describeworker.EncodeAdapterResultV2(localWorkerSemanticFixture)
	if managedSemanticFixtureCeiling != 2048 || len(localWorkerSemanticFixture) > limits.MaxSemanticBytes || len(payload) > limits.MaxAdapterBytes {
		t.Fatal("ASSERT_MANAGED_FIXTURE_PAYLOAD_FITS", len(localWorkerSemanticFixture), len(payload), limits)
	}
	candidate, err := describeworker.ParseSuppliedTerminalResponse(payload, limits)
	if err != nil || candidate.RawAdapterDigest() == "" {
		t.Fatal("ASSERT_MANAGED_FIXTURE_POSITIVE_PARSE", err)
	}
	// Exactly the original malformed worker stdout, including its final newline.
	malformed := []byte("{\"status\":\"COMPLETE\",\"text\":\"{}\"}\n")
	if len(malformed) > limits.MaxAdapterBytes || len("{}") > limits.MaxSemanticBytes {
		t.Fatal("ASSERT_MANAGED_MALFORMED_UNDER_LIMIT")
	}
	_, err = describeworker.ParseSuppliedTerminalResponse(malformed, limits)
	var failure *describeworker.Failure
	if !errors.As(err, &failure) || failure.Stage() != describeworker.StageOutput || failure.Code() != describeworker.CodeOutputInvalid || (failure.Subcode() != "OUTER_PROTOCOL_OR_CANONICALITY" && failure.Subcode() != "SEMANTIC_SCHEMA_MISMATCH") {
		t.Fatal("ASSERT_MANAGED_MALFORMED_PROTOCOL_NOT_BYTES", err)
	}
	t.Log("ASSERT_MANAGED_FIXTURE_PAYLOAD_FITS: PASS; ASSERT_MANAGED_FIXTURE_POSITIVE_PARSE: PASS; ASSERT_MANAGED_MALFORMED_UNDER_LIMIT: PASS; ASSERT_MANAGED_MALFORMED_PROTOCOL_NOT_BYTES: PASS")
}

// Controlled shell fixtures are not models, sandbox qualification, or inference.
// AdapterIdentity explicitly selects the private SOURCE local adapter; ordinary A
// configurations keep their delivery-disabled behavior.
func TestSourceOperatorLocalWorkerInvocation(t *testing.T) { runLocalWorkerFixture(t, "invocation") }

// This must traverse the real operator, not a separate artifact evaluator.
func TestSourceOperatorLocalManagedReuse(t *testing.T) { runLocalWorkerFixture(t, "managed-reuse") }
func TestSourceOperatorLocalManagedMetadata(t *testing.T) {
	for _, mode := range []string{"managed-stat-base", "managed-stat-dependency", "managed-stat-control"} {
		t.Run(strings.TrimPrefix(mode, "managed-stat-"), func(t *testing.T) { runLocalWorkerFixture(t, mode) })
	}
}
func TestSourceOperatorLocalManagedPipes(t *testing.T) {
	for _, mode := range []string{"managed-pipe-early", "managed-pipe-error", "managed-pipe-control"} {
		t.Run(strings.TrimPrefix(mode, "managed-pipe-"), func(t *testing.T) { runLocalWorkerFixture(t, mode) })
	}
}
func TestSourceOperatorLocalManagedRefusal(t *testing.T) {
	for _, mode := range []string{"managed-revoked", "managed-zero", "managed-missing", "managed-underfunded", "managed-malformed", "managed-active-revoke", "managed-revoke-before-start", "managed-cleanup-failure"} {
		t.Run(mode, func(t *testing.T) { runLocalWorkerFixture(t, mode) })
	}
}
func TestSourceOperatorLocalWorkerDelivery(t *testing.T) {
	for _, mode := range []string{"success", "success-spaced", "underfunded", "zero-config", "badlimits", "pin-mismatch", "failed-exit", "timeout", "deadline", "cancel", "oversize", "stderr", "fragmented", "inherited", "detached", "malformed", "scratch", "cleanup-observer-failure"} {
		t.Run(mode, func(t *testing.T) { runLocalWorkerFixture(t, mode) })
	}
}
func TestSourceOperatorLocalReviewRepairs(t *testing.T) {
	for _, mode := range []string{"wire-html", "stage-too-large", "profile-too-large", "policy-permissive", "policy-comment", "policy-override", "untrusted-executor", "swap-worker", "swap-model", "swap-library", "swap-executor", "swap-profile", "swap-grammar"} {
		t.Run(mode, func(t *testing.T) { runLocalWorkerFixture(t, mode) })
	}
}

func TestSourceOperatorLocalProtectedStaging(t *testing.T) {
	for _, mode := range []string{"unsafe-parent", "unsafe-ancestor", "stage-mode-drift", "cleanup-mode-drift"} {
		t.Run(mode, func(t *testing.T) { runLocalWorkerFixture(t, mode) })
	}
}

func TestSourceOperatorLocalMetadataInterruption(t *testing.T) {
	for _, mode := range []string{"metadata-eintr-admit", "metadata-eintr-before-spawn", "metadata-eintr-cleanup", "metadata-eintr-cancel", "metadata-eintr-finalize"} {
		t.Run(mode, func(t *testing.T) { runLocalWorkerFixture(t, mode) })
	}
}

// Exact documented ADR7 result shape and emit option; not EncodeAdapterResultV2.
func actualADR7Wire(text string) (raw, adapter []byte) {
	v := struct {
		Status    string  `json:"status"`
		Text      string  `json:"text,omitempty"`
		Tokens    int     `json:"tokens"`
		LoadMS    float64 `json:"load_ms"`
		RunMS     float64 `json:"run_ms"`
		Cancelled bool    `json:"cancelled"`
		Decode    int32   `json:"decode_status"`
		Error     string  `json:"error,omitempty"`
		Grammar   string  `json:"grammar"`
		Context   uint32  `json:"context_tokens"`
	}{"COMPLETE", text, 32, 1.25, 2.5, false, 0, "", "llama_sampler_init_grammar", 8192}
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
	adapter, _ = json.Marshal(v)
	return b.Bytes(), append(adapter, '\n')
}
func runLocalWorkerFixture(t *testing.T, mode string) {
	isManaged := strings.HasPrefix(mode, "managed-")
	statMode := strings.HasPrefix(mode, "managed-stat-")
	metadataProbe := new(inv.ControlledMetadataProbeForTest)
	if mode == "managed-stat-base" {
		metadataProbe.Fault = "base"
	}
	if mode == "managed-stat-dependency" {
		metadataProbe.Fault = "dependency"
	}
	var admissionErr error
	joinFailure := errors.New("controlled cleanup observation failure")
	owner, root := inv.ControlledLocalOwnerForTest(t)
	if mode == "underfunded" || mode == "managed-underfunded" {
		owner, root = inv.ControlledLocalWorkerLimitedOwnerForTest(t)
	}
	m, start, uri := mixedStartFixtureV2(t)
	if e := os.WriteFile(filepath.Join(start.Profile.Workspace().String(), "root.go"), []byte("package fixture\nfunc Root(){ Other() }\nfunc Other(){ Root() }\n"), 0644); e != nil {
		t.Fatal(e)
	}
	start.Process.Args = []string{"-test.run=^TestPrivateSourceSELFResponder$", "--", "private-source-self", start.Profile.Workspace().String(), "positive"}
	if mode == "deadline" {
		start.Deadline = time.Now().Add(1500 * time.Millisecond)
	}
	ctx, cancel := context.WithDeadline(context.Background(), start.Deadline)
	defer cancel()
	// Keep the positive execution namespace below the protected checkout, not a
	// shared writable temporary ancestor. No production bypass is involved.
	cwd, e := os.Getwd()
	if e != nil {
		t.Fatal(e)
	}
	base, e := os.MkdirTemp(cwd, ".local-stage-fixture-")
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { os.RemoveAll(base) })
	base, e = filepath.EvalSymlinks(base)
	if e != nil {
		t.Fatal(e)
	}
	for _, dir := range []string{"mailbox", "a", "b", "pins"} {
		if e = os.Mkdir(filepath.Join(base, dir), 0700); e != nil {
			t.Fatal(e)
		}
	}
	if mode == "unsafe-parent" {
		if e = os.Chmod(filepath.Join(base, "pins"), 0777); e != nil {
			t.Fatal(e)
		}
	}
	if mode == "unsafe-ancestor" {
		if e = os.Chmod(base, 0777); e != nil {
			t.Fatal(e)
		}
	}
	marker := filepath.Join(base, "invocations")
	pin := func(name, content string, mode os.FileMode) (describeworker.FilePin, uint64) {
		p := filepath.Join(base, "pins", name)
		if e := os.WriteFile(p, []byte(content), mode); e != nil {
			t.Fatal(e)
		}
		return describeworker.FilePin{Path: p, SHA256: fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(content)))}, uint64(len(content))
	}
	var c inv.PrivateSourceLocalConfigV1
	semantic := localWorkerSemanticFixture
	if mode == "success-spaced" {
		semantic = " \n" + semantic + "\t"
	}
	if mode == "wire-html" {
		semantic = strings.Replace(semantic, "controlled source fixture", "A&B <T> -0 1e+09", 1)
	}
	payload := describeworker.EncodeAdapterResultV2(semantic)
	rawWorker := payload
	if mode == "wire-html" {
		rawWorker, payload = actualADR7Wire(semantic)
	}
	action := "printf '%s' '" + string(rawWorker) + "'\n"
	switch mode {
	case "failed-exit":
		action = "exit 4\n"
	case "fragmented":
		action = "printf '%s' '" + string(payload[:1]) + "'; sleep 0.1; printf '%s' '" + string(payload[1:2]) + "'; sleep 0.1; printf '%s' '" + string(payload[2:]) + "'\n"
	case "scratch":
		action = "printf unexpected > unexpected-file\n" + action
	case "timeout", "deadline":
		action = "sleep 3\n"
	case "detached":
		action = "sleep 3 >/dev/null 2>&1 &\n" + action
	case "inherited":
		action = "sleep 3 &\n" + action
	case "oversize":
		action = "i=0; while [ $i -lt 1100 ]; do printf x; i=$((i+1)); done\n"
	case "stderr":
		action = "i=0; while [ $i -lt 256 ]; do printf x >&2; i=$((i+1)); done\n"
	case "malformed", "invocation", "managed-malformed":
		action = "printf '{\"status\":\"COMPLETE\",\"text\":\"{}\"}\\n'\n"
	}
	worker := "#!/bin/sh\nprintf x >> '" + marker + "'\nprintf '%s\\n' \"$@\" > '" + filepath.Join(base, "argv") + "'\nwhile [ $# -gt 0 ]; do if [ \"$1\" = -prompt-file ]; then shift; cp \"$1\" '" + filepath.Join(base, "prompt") + "'; elif [ \"$1\" = -model ]; then shift; cp \"$1\" '" + filepath.Join(base, "model-used") + "'; elif [ \"$1\" = -grammar-file ]; then shift; cp \"$1\" '" + filepath.Join(base, "grammar-used") + "'; elif [ \"$1\" = -lib ]; then shift; cp \"$1/fake-library\" '" + filepath.Join(base, "library-used") + "'; fi; shift; done\n" + action
	if isManaged {
		worker = strings.Replace(worker, "cp \"$1/fake-library\" '"+filepath.Join(base, "library-used")+"'", "cp \"$1/fake-library\" '"+filepath.Join(base, "library-used")+"'; cp \"$1/fake-dependency\" '"+filepath.Join(base, "dependency-used")+"'", 1)
	}
	c.Config.Worker, c.Lengths[0] = pin("worker", worker, 0700)
	c.Config.Model, c.Lengths[1] = pin("fake-weights", "controlled-not-model", 0600)
	c.Config.Library, c.Lengths[2] = pin("fake-library", "controlled-not-library", 0600)
	approvedProfile := inv.ControlledLocalWorkerProfileForTest()
	if isManaged {
		approvedProfile = inv.ControlledManagedProfileForTest()
	}
	profile := approvedProfile
	switch mode {
	case "policy-permissive":
		profile = "(version 1)\n(allow default)\n"
	case "policy-comment":
		profile = "(version 1)\n; (deny network*)\n(allow default)\n"
	case "policy-override":
		profile = approvedProfile + "(allow network*)\n"
	}
	c.Config.SandboxExecutable, c.Lengths[3] = pin("sandbox", "#!/bin/sh\ncp \"$2\" '"+filepath.Join(base, "profile-used")+"'\nshift 2\nwhile [ \"$1\" = -D ]; do shift 2; done\nexec \"$@\"\n", 0700)
	c.Config.SandboxProfile, c.Lengths[4] = pin("profile", profile, 0600)
	c.Config.Grammar, c.Lengths[5] = pin("grammar", "controlled-not-grammar", 0600)
	c.Config.RuntimeIdentity = "controlled-only"
	c.Config.AdapterIdentity = "SOURCE_LOCAL_WORKER_V1"
	c.Config.ModelIdentity = "controlled-not-model"
	c.Config.ResponseVersion = describeworker.ResponseVersionV2
	c.Config.Limits = describeworker.Limits{TimeoutMS: 1000, MaxTokens: 32, ContextTokens: 8192, StdoutBytes: 1024, StderrBytes: 128, WorkBytes: 4096, TempBytes: 4096}
	var managed inv.PrivateSourceLocalManagedV1
	var lifecycle *inv.PrivateManagedInputOwnerV2
	var managedRoot string
	artifactReads, admissionReads := 0, 0
	if isManaged {
		lifecycle, _ = inv.ControlledLocalOwnerForTest(t)
		c = inv.ControlledLocalWorkerIOForTest(c, func(event string) error {
			if event == "read" {
				artifactReads++
			}
			return nil
		})
		if statMode {
			c = inv.ControlledMetadataIOForTest(c, metadataProbe)
		}
		managed, admissionErr = inv.ControlledManagedAdmitForTest(t, lifecycle, c)
		if metadataProbe.Fault == "" && admissionErr != nil {
			t.Fatal("ASSERT_MANAGED_OWNED_ADMISSION", admissionErr)
		}
		if admissionErr == nil {
			managedRoot = inv.ControlledManagedRootForTest(managed)
		}
		admissionReads = artifactReads
		if metadataProbe.Fault == "" && admissionReads != 14 {
			t.Fatal("ASSERT_MANAGED_BOUNDED_ADMISSION_READS", admissionReads)
		}
		c.Managed = &managed
		t.Cleanup(func() { inv.ControlledManagedCloseForTest(managed) })
		if mode == "managed-revoked" {
			if e := managed.Revoke(); e != nil {
				t.Fatal(e)
			}
		}
		if mode == "managed-zero" {
			c.Managed = &inv.PrivateSourceLocalManagedV1{}
		}
		if mode == "managed-missing" {
			if e := os.Remove(filepath.Join(managedRoot, "lib", "fake-dependency")); e != nil {
				t.Fatal(e)
			}
		}
	}
	pipeMode := strings.HasPrefix(mode, "managed-pipe-")
	pipeFault := errors.New("controlled owned pipe close error after actual close")
	probe := new(inv.ControlledPipeProbeForTest)
	if mode == "managed-pipe-error" {
		probe.CloseFault = pipeFault
	}
	if pipeMode {
		t.Cleanup(func() { inv.ControlledPipeDisposeForTest(probe) })
	}
	starts, joins, opens := 0, 0, 0
	namespaceAttempts, injectedAt := 0, 0
	metadataAfterPins, metadataAfterJoin, metadataFinalizing := false, false, false
	var namespacePaid [14]uint64
	var paid [14]uint64
	if mode == "deadline" {
		c.Config.Limits.TimeoutMS = 5000
	}
	if mode == "zero-config" {
		c.Lengths[0] = 0
	}
	if mode == "stage-too-large" {
		c.Config.Model, c.Lengths[1] = pin("fake-weights", strings.Repeat("x", 4096), 0600)
	}
	if mode == "profile-too-large" {
		c.Config.SandboxProfile, c.Lengths[4] = pin("profile", strings.Repeat("x", 16385), 0600)
	}
	if mode == "badlimits" {
		c.Config.Limits.ContextTokens = 8193
	}
	if mode == "pin-mismatch" {
		if e = os.WriteFile(c.Config.Worker.Path, []byte("changed"), 0700); e != nil {
			t.Fatal(e)
		}
	}
	wrap := inv.ControlledLocalWorkerIOForTest
	if mode == "untrusted-executor" {
		wrap = inv.ControlledLocalIOForTest
	}
	c = wrap(c, func(event string) error {
		if event == "namespace-cleanup" {
			metadataFinalizing = true
		}
		inject := mode == "metadata-eintr-admit" || ((mode == "metadata-eintr-before-spawn" || mode == "metadata-eintr-cancel") && metadataAfterPins) || (mode == "metadata-eintr-cleanup" && metadataAfterJoin) || (mode == "metadata-eintr-finalize" && metadataFinalizing)
		if event == "namespace-stat" {
			namespaceAttempts++ // every attempt is observed, including injected EINTR
			if namespaceAttempts > 32768 {
				t.Fatal("ASSERT_NAMESPACE_PREPAID_BOUND", namespaceAttempts)
			}
			// Avoid disk-heavy test head reopens for every raw stat. Check the real
			// P at first metadata, every injected fault and every original IO/close/
			// start/join/cleanup event; no observer call or raw attempt is bypassed.
			if namespaceAttempts > 1 && !(inject && injectedAt == 0) {
				return nil
			}
		}
		d, w, debit := inv.ControlledLocalHeadForTest(t, owner)
		if d != "PENDING" || w != 2 {
			return fmt.Errorf("ASSERT_B_PENDING: event=%s head=%s windows=%d", event, d, w)
		}
		if opens > 0 && debit != paid {
			t.Fatal("ASSERT_B_STABLE_ORIGINAL_DEBIT")
		}
		if event == "namespace-stat" {
			if namespaceAttempts == 1 {
				namespacePaid = debit
				local := uint64(65536 + 32768)
				if !isManaged {
					local, _, _ = inv.ControlledLocalReservationCallsForTest(t, owner, c)
				}
				if local < 65536+32768 {
					t.Fatal("ASSERT_B_NAMESPACE_QUOTE_JOIN", local)
				}
			}
			if namespacePaid != debit || namespaceAttempts > 32768 {
				t.Fatal("ASSERT_NAMESPACE_PREPAID_BOUND", namespaceAttempts)
			}
			if inject && injectedAt == 0 {
				injectedAt = namespaceAttempts
				if mode == "metadata-eintr-cancel" {
					cancel()
				}
				// Controlled attempt-boundary error, not a delivered kernel signal.
				// Every non-injected attempt delegates real Fstat on the same FD.
				return syscall.EINTR
			}
		}
		if event == "closed" && opens == 6 {
			metadataAfterPins = true
		}
		if event == "worker-joined" {
			metadataAfterJoin = true
		}
		if event == "open" {
			opens++
			if opens == 1 {
				paid = debit
				tranche := inv.ControlledLocalPTrancheForTest(t, owner)
				if tranche[5] != 2 || tranche[6] != 5 || tranche[10] != 135168 || tranche[11] != 17536 || tranche[12] != 148608 {
					return fmt.Errorf("ASSERT_B_TRANSPORT_CURRENCIES: got=%v", tranche)
				}
				local, _, _ := inv.ControlledLocalReservationCallsForTest(t, owner, c)
				if local < 65536+32768 && mode != "underfunded" {
					t.Fatal("ASSERT_B_NAMESPACE_QUOTE_JOIN", local)
				}
			}
		}
		if event == "closed" && strings.HasPrefix(mode, "swap-") {
			idx := map[string]int{"swap-worker": 0, "swap-model": 1, "swap-library": 2, "swap-executor": 3, "swap-profile": 4, "swap-grammar": 5}[mode]
			if opens-1 == idx {
				paths := []string{c.Config.Worker.Path, c.Config.Model.Path, c.Config.Library.Path, c.Config.SandboxExecutable.Path, c.Config.SandboxProfile.Path, c.Config.Grammar.Path}
				path := paths[idx]
				changed := []byte("REPLACED")
				if idx == 0 {
					changed = []byte("#!/bin/sh\nprintf x > '" + filepath.Join(base, "replacement") + "'\n" + strings.TrimPrefix(worker, "#!/bin/sh\n"))
				}
				if idx == 3 {
					changed = []byte("#!/bin/sh\nprintf x > '" + filepath.Join(base, "replacement") + "'\nshift 2\nexec \"$@\"\n")
				}
				if idx == 4 {
					changed = []byte("(version 1)\n(allow default)\n")
				}
				if e := os.WriteFile(path+".replacement", changed, 0700); e != nil {
					return e
				}
				if e := os.Rename(path+".replacement", path); e != nil {
					return e
				}
			}
		}
		if (mode == "stage-mode-drift" && event == "closed" && opens == 1) || (mode == "cleanup-mode-drift" && event == "worker-joined") {
			stage := filepath.Join(filepath.Dir(c.Config.Worker.Path), ".source-worker-"+strings.TrimPrefix(fmt.Sprintf("sha256:%x", sha256.Sum256([]byte("worker-preparation"))), "sha256:"))
			// Ordinary controller-owned fixture metadata only; no replacement or payload.
			if e := os.Chmod(stage, 0750); e != nil {
				return e
			}
		}
		if event == "managed-pipes-created" && pipeMode && mode != "managed-pipe-control" {
			done := make(chan error, 1)
			go func() { done <- managed.Revoke() }()
			return <-done
		}
		if event == "managed-before-start" && mode == "managed-revoke-before-start" {
			return managed.Revoke()
		}
		if event == "worker-start" {
			starts++
			if mode == "managed-active-revoke" {
				if e := managed.Revoke(); e != nil {
					return e
				}
			}
			if mode == "cancel" {
				cancel()
			}
		}
		if event == "worker-joined" {
			joins++
			if mode == "cleanup-observer-failure" || mode == "managed-cleanup-failure" {
				return joinFailure
			}
		}
		return nil
	})
	if pipeMode {
		c = inv.ControlledPipeIOForTest(c, probe)
	}
	digest := func(s string) string { return fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(s))) }
	q := so.Request{Owner: owner, Capture: inv.PrivateMixedInputRequestV2{Manager: m, Root: root, Start: start, URI: uri, OccurrenceID: "local-source-op", Revision: "source-controlled-revision", Privacy: "private-source-controlled", OccurrencePolicy: "ordinal", CompositionPolicy: "identity", Line: 1, Character: 6, Kinds: []programcadmission.RelationKind{programcadmission.TypedReferencesSymbol}}, CaptureAttempt: digest("worker-capture"), PreparationAttempt: digest("worker-preparation"), RoleAttempt: "worker-local", RunID: "worker-run", SeedNamespace: "original-schedule", Seed: 7, Contract: sourceStageResponseContract(t, 131072), SourceLimits: tp.PrivateSourceLimits{MaxDocumentBytes: 16 << 10, MaxTotalBytes: 64 << 10, MaxWork: 4096, MaxObjects: 16}, Host: host.SourceHostConfig{Protocol: host.SourceHostProtocol, Mailbox: host.AutomaticHandoffConfigPrivateV2{Enabled: true, Directory: filepath.Join(base, "mailbox"), Invocation: "worker-op", InterpreterRole: "local-worker", ReviewerRole: "isolated-reviewer", TimeoutMS: 5000, OverallTimeoutMS: 5000, PollMS: 10, Limits: host.AutomaticLimitsPrivateV2{MaxCalls: 1, MaxRequestBytes: 65536, MaxResponseBytes: 16384, MaxTotalBytes: 147456, MaxOutputBytes: 65536}}}, ResponseLimits: describeworker.SuppliedResponseLimits{MaxAdapterBytes: 8192, MaxSemanticBytes: 4096}, OutputRoots: [2]string{filepath.Join(base, "a"), filepath.Join(base, "b")}, Local: &c}
	// These later-boundary fixtures need semantic headroom, not a larger native budget.
	// Keep every other mode and all independent adapter/worker/mailbox caps intact.
	switch mode {
	case "managed-malformed", "managed-active-revoke", "managed-cleanup-failure", "managed-reuse":
		q.ResponseLimits.MaxSemanticBytes = managedSemanticFixtureCeiling
	}
	attempts := 1
	if mode == "managed-reuse" {
		attempts = 2
	}
	for iteration := 0; iteration < attempts; iteration++ {
		if iteration != 0 {
			owner, root = inv.ControlledLocalOwnerForTest(t)
			m, start, uri = mixedStartFixtureV2(t)
			if e := os.WriteFile(filepath.Join(start.Profile.Workspace().String(), "root.go"), []byte("package fixture\nfunc Root(){ Other() }\nfunc Other(){ Root() }\n"), 0644); e != nil {
				t.Fatal(e)
			}
			start.Process.Args = []string{"-test.run=^TestPrivateSourceSELFResponder$", "--", "private-source-self", start.Profile.Workspace().String(), "positive"}
			ctx, cancel = context.WithDeadline(context.Background(), start.Deadline)
			defer cancel()
			q.Owner = owner
			q.Capture.Root = root
			q.Capture.Manager = m
			q.Capture.Start = start
			q.Capture.URI = uri
			q.CaptureAttempt = digest("managed-capture-2")
			q.PreparationAttempt = digest("managed-preparation-2")
			for _, name := range []string{"mailbox-2", "a-2", "b-2"} {
				if e := os.Mkdir(filepath.Join(base, name), 0700); e != nil {
					t.Fatal(e)
				}
			}
			q.Host.Mailbox.Directory = filepath.Join(base, "mailbox-2")
			q.OutputRoots = [2]string{filepath.Join(base, "a-2"), filepath.Join(base, "b-2")}
			for _, name := range []string{"invocations", "profile-used", "model-used", "grammar-used", "library-used", "dependency-used", "argv", "prompt"} {
				if e := os.Remove(filepath.Join(base, name)); e != nil && !os.IsNotExist(e) {
					t.Fatal(e)
				}
			}
			starts, joins, opens = 0, 0, 0
			namespaceAttempts = 0
			namespacePaid = [14]uint64{}
		}
		grant := inv.ControlledLocalGrantForTest(t, owner)
		out, runErr := so.Run(ctx, q)
		switch mode {
		case "managed-malformed", "managed-active-revoke", "managed-cleanup-failure":
			if errors.Is(runErr, so.ErrNativeAggregate) {
				t.Fatal("ASSERT_MANAGED_LATER_REFUSAL_NOT_AGGREGATE", mode, runErr)
			}
		}
		if mode == "managed-cleanup-failure" && !errors.Is(runErr, joinFailure) {
			t.Fatal("ASSERT_MANAGED_INJECTED_JOIN_FAILURE", runErr)
		}
		if isManaged && runErr == nil {
			localQuote := inv.ControlledManagedLocalQuoteForTest(t, c)
			tranche := inv.ControlledLocalPTrancheForTest(t, owner)
			for i, v := range localQuote {
				if tranche[i] < v {
					t.Fatal("ASSERT_MANAGED_ORIGINAL_P_QUOTE", i, tranche[i], v)
				}
			}
			if tranche[5] != 2 || tranche[6] != 5 {
				t.Fatal("ASSERT_MANAGED_TRANSPORT_NOT_ERASED", tranche)
			}
			t.Logf("ASSERT_MANAGED_USE_QUOTE: local=%v sealed_P=%v", localQuote, tranche)
		}
		if inv.ControlledLocalGrantForTest(t, owner) != grant {
			t.Fatal("ASSERT_B_ORIGINAL_AUTHORIZATION_UNCHANGED")
		}
		count := 0
		b, e := os.ReadFile(marker)
		if e == nil {
			count = len(b)
		} else if !os.IsNotExist(e) {
			t.Fatal(e)
		}
		d, w, debit := inv.ControlledLocalHeadForTest(t, owner)
		if m.Census().Workers != 0 {
			t.Fatal("ASSERT_B_CAPTURE_JOINED", m.Census())
		}
		if statMode {
			inv.ControlledMetadataAssertForTest(t, metadataProbe)
			ld, lw, _ := inv.ControlledLocalHeadForTest(t, lifecycle)
			if metadataProbe.Fault != "" {
				if !errors.Is(admissionErr, syscall.EINTR) || starts != 0 || count != 0 || runErr == nil || out.NativeID != "" || out.Published != [2]bool{} || ld != "PENDING" || lw != 1 || d != "OBSERVED" || w != 1 {
					t.Errorf("ASSERT_METADATA_TERMINAL_NO_GENERATION_OR_OBSERVE: admission=%v source=%v starts=%d lifecycle=%s/%d source=%s/%d pair=%s", admissionErr, runErr, starts, ld, lw, d, w, out.NativeID)
				} else {
					t.Log("ASSERT_METADATA_TERMINAL_NO_GENERATION_OR_OBSERVE: PASS; no spawn/native pair; lifecycle PENDING; failed handle rejected by actual Run")
				}
			} else {
				if runErr != nil || starts != 1 || count != 1 || d != "OBSERVED" || w != 2 || out.NativeID == "" || out.Published != [2]bool{true, true} || out.Readback != [2]bool{true, true} {
					t.Fatal("ASSERT_METADATA_CONTROL_CONNECTED", runErr, out, d, w)
				}
				if e := managed.Retire(context.Background()); e != nil {
					t.Fatal(e)
				}
				t.Log("ASSERT_METADATA_CONTROL_CONNECTED: PASS")
			}
			return
		}
		if pipeMode {
			inv.ControlledPipeAssertForTest(t, probe)
			if mode == "managed-pipe-control" {
				if runErr != nil || starts != 1 || count != 1 || d != "OBSERVED" || w != 2 || out.NativeID == "" || out.Published != [2]bool{true, true} || out.Readback != [2]bool{true, true} {
					t.Fatal("ASSERT_PIPE_CONTROL_CONNECTED", runErr, out, d, w)
				}
				if e := managed.Retire(context.Background()); e != nil {
					t.Fatal(e)
				}
				t.Log("ASSERT_PIPE_CONTROL_CONNECTED: PASS")
			} else {
				if runErr == nil || starts != 0 || count != 0 || d != "PENDING" || w != 2 || out.NativeID != "" || out.Published != [2]bool{} {
					t.Fatal("ASSERT_PIPE_EARLY_REFUSAL", runErr, out, d, w, starts)
				}
				if mode == "managed-pipe-error" {
					if !errors.Is(runErr, pipeFault) {
						t.Error("ASSERT_PIPE_CLOSE_ERROR_PROPAGATED: missing actual-owned-close fault", runErr)
					} else {
						t.Log("ASSERT_PIPE_CLOSE_ERROR_PROPAGATED: PASS")
					}
				}
				t.Log("ASSERT_PIPE_EARLY_REFUSAL: PASS; after both creations, before managed check, no spawn/pair, PENDING")
			}
			return
		}
		if mode == "metadata-eintr-finalize" {
			if injectedAt == 0 || !metadataFinalizing || !errors.Is(runErr, syscall.EINTR) || starts != 1 || count != 1 || out.Disposition == "SOURCE_LOCAL_PROVISIONAL_DELIVERED_PAIR" || d != "PENDING" || w != 2 || namespaceAttempts > 32768 {
				t.Fatal("ASSERT_NAMESPACE_FINALIZE_EINTR", namespaceAttempts, starts, count, out, runErr, d, w)
			}
			t.Log("ASSERT_NAMESPACE_FINALIZE_EINTR: PASS; retained bytes are not success; original head PENDING", namespaceAttempts, out.Disposition)
			return
		}
		if strings.HasPrefix(mode, "metadata-eintr-") {
			wantStarts := 0
			if mode == "metadata-eintr-cleanup" {
				wantStarts = 1
			}
			if injectedAt == 0 || !errors.Is(runErr, syscall.EINTR) || starts != wantStarts || count != wantStarts || out.NativeID != "" || out.Published != [2]bool{} || d != "PENDING" || w != 2 || namespaceAttempts > 32768 {
				t.Fatal("ASSERT_NAMESPACE_RUN_EINTR", mode, namespaceAttempts, injectedAt, starts, count, out, runErr, d, w)
			}
			if mode == "metadata-eintr-admit" && (namespaceAttempts != 1 || opens != 0) {
				t.Fatal("ASSERT_NAMESPACE_RUN_ONE_ATTEMPT", namespaceAttempts, opens)
			}
			t.Log("ASSERT_NAMESPACE_RUN_EINTR: PASS", mode, "attempts", namespaceAttempts, "injected", injectedAt, "starts", starts, "PENDING")
			return
		}
		if mode == "stage-mode-drift" || mode == "cleanup-mode-drift" {
			wantStarts := 0
			if mode == "cleanup-mode-drift" {
				wantStarts = 1
			}
			if runErr == nil || starts != wantStarts || out.NativeID != "" || out.Published != [2]bool{} || d != "PENDING" || w != 2 {
				t.Fatal("ASSERT_PROTECTED_STAGE_CONTINUITY", mode, starts, out, runErr, d, w)
			}
			t.Log("ASSERT_PROTECTED_STAGE_CONTINUITY: PASS", mode)
			return
		}
		if mode == "unsafe-parent" || mode == "unsafe-ancestor" {
			if runErr == nil || count != 0 || starts != 0 || opens != 0 || out.NativeID != "" || out.Published != [2]bool{} || d != "PENDING" || w != 2 {
				t.Fatal("ASSERT_PROTECTED_STAGE_PRESPAWN", mode, count, starts, opens, out, runErr, d, w)
			}
			t.Log("ASSERT_PROTECTED_STAGE_PRESPAWN: PASS", mode)
			return
		}
		if strings.HasPrefix(mode, "policy-") || mode == "untrusted-executor" {
			if runErr == nil || count != 0 || starts != 0 || out.NativeID != "" || d != "PENDING" || w != 2 {
				t.Fatal("ASSERT_REVIEW_POLICY_PRESPAWN", mode, count, starts, out, runErr, d, w)
			}
			return
		}
		if mode == "zero-config" || mode == "badlimits" || mode == "pin-mismatch" || mode == "stage-too-large" || mode == "profile-too-large" {
			wantWindows := uint64(1)
			wantHead := "OBSERVED"
			if mode == "pin-mismatch" {
				wantWindows = 2
				wantHead = "PENDING"
			}
			if runErr == nil || count != 0 || starts != 0 || w != wantWindows || d != wantHead || out.NativeID != "" || out.Published != [2]bool{} || (mode != "pin-mismatch" && opens != 0) {
				t.Fatal("ASSERT_B_CONFIG_OR_PIN_REFUSAL", mode, out, runErr, count, starts, opens, d, w)
			}
			return
		}
		if mode == "managed-revoked" || mode == "managed-zero" || mode == "managed-missing" || mode == "managed-revoke-before-start" {
			wantD, wantW := "OBSERVED", uint64(1)
			if mode == "managed-missing" || mode == "managed-revoke-before-start" {
				wantD, wantW = "PENDING", 2
			}
			if runErr == nil || starts != 0 || count != 0 || out.NativeID != "" || d != wantD || w != wantW {
				t.Fatal("ASSERT_MANAGED_PREFLIGHT_REFUSAL", mode, runErr, starts, count, d, w)
			}
			t.Log("ASSERT_MANAGED_PREFLIGHT_REFUSAL: PASS", mode)
			return
		}
		if mode == "underfunded" || mode == "managed-underfunded" {
			if count != 0 || starts != 0 || opens != 0 || w != 1 || d != "OBSERVED" || runErr == nil {
				t.Fatal("ASSERT_B_PRE_EFFECT_FUNDING", count, starts, opens, w, d, runErr)
			}
			t.Log("ASSERT_B_PRE_EFFECT_FUNDING: PASS; B P refused before artifact IO and worker")
			_, retryErr := so.Run(ctx, q)
			d2, w2, b2 := inv.ControlledLocalHeadForTest(t, owner)
			if retryErr == nil || opens != 0 || starts != 0 || d2 != d || w2 != w || b2 != debit {
				t.Fatal("ASSERT_B_UNFUNDED_NO_REPLAY", retryErr, d2, w2)
			}
			return
		}
		if mode == "cancel" || mode == "managed-active-revoke" {
			if starts != 1 || count > 1 || runErr == nil || d != "PENDING" || w != 2 || out.NativeID != "" {
				t.Fatal("ASSERT_B_START_CANCEL_REFUSAL", starts, count, runErr, d, w)
			}
			if mode == "managed-active-revoke" && managed.Retire(context.Background()) == nil {
				t.Fatal("ASSERT_MANAGED_UNCERTAIN_LEASE_RETAINED")
			}
			return
		}
		if count != 1 {
			t.Fatalf("ASSERT_B_ACTUAL_WORKER_ONCE: got=%d want=1 disposition=%s head=%s windows=%d starts=%d namespace=%d error=%v", count, out.Disposition, d, w, starts, namespaceAttempts, runErr)
		}
		t.Log("ASSERT_B_ACTUAL_WORKER_ONCE: PASS")
		if mode == "success" || mode == "success-spaced" || mode == "wire-html" || mode == "managed-reuse" || strings.HasPrefix(mode, "swap-") {
			if runErr != nil || out.NativeID == "" || out.Disposition != "SOURCE_LOCAL_PROVISIONAL_DELIVERED_PAIR" || out.Published != [2]bool{true, true} || out.Readback != [2]bool{true, true} || d != "OBSERVED" || w != 2 || starts != 1 || joins != 1 {
				t.Fatal("ASSERT_B_CONNECTED_DELIVERY", out, runErr, d, w, starts, joins)
			}
			if strings.HasPrefix(mode, "swap-") {
				if _, e := os.Stat(filepath.Join(base, "replacement")); !os.IsNotExist(e) {
					t.Fatal("ASSERT_REVIEW_REPLACEMENT_NEVER_EXECUTES", mode, e)
				}
			}
			if strings.HasPrefix(mode, "swap-") || mode == "managed-reuse" {
				for name, want := range map[string]string{"model-used": "controlled-not-model", "grammar-used": "controlled-not-grammar", "library-used": "controlled-not-library", "profile-used": approvedProfile} {
					got, e := os.ReadFile(filepath.Join(base, name))
					if e != nil || string(got) != want {
						t.Fatal("ASSERT_REVIEW_VERIFIED_INPUT_BYTES", mode, name, string(got), e)
					}
				}
			}
			var request host.SourceHostRequest
			raw, e := os.ReadFile(filepath.Join(q.Host.Mailbox.Directory, "request-000001.json"))
			if e != nil || json.Unmarshal(raw, &request) != nil {
				t.Fatal("ASSERT_B_REQUEST", e)
			}
			prompt, e := os.ReadFile(filepath.Join(base, "prompt"))
			if e != nil || string(prompt) != request.Request.Envelope.Prompt || !strings.Contains(string(prompt), "func Root(){ Other() }") || strings.Contains(string(prompt), "func Other(){ Root() }") {
				t.Fatal("ASSERT_B_EXACT_SOURCE_ONLY_PROMPT", e, string(prompt))
			}
			argvRaw, e := os.ReadFile(filepath.Join(base, "argv"))
			if e != nil {
				t.Fatal(e)
			}
			argv := strings.Split(strings.TrimSuffix(string(argvRaw), "\n"), "\n")
			stagedRoot := filepath.Join(filepath.Dir(c.Config.Worker.Path), ".source-worker-"+q.PreparationAttempt[7:])
			if mode == "managed-reuse" {
				stagedRoot = managedRoot
			}
			if len(argv) != 14 || argv[0] != "-model" || argv[1] != filepath.Join(stagedRoot, "model") || argv[2] != "-lib" || argv[3] != filepath.Join(stagedRoot, "lib") || argv[4] != "-prompt-file" || filepath.Base(argv[5]) != "prompt.txt" || !filepath.IsAbs(argv[5]) || argv[6] != "-grammar-file" || argv[7] != filepath.Join(stagedRoot, "grammar.gbnf") || argv[8] != "-timeout" || argv[10] != "-max-tokens" || argv[11] != "32" || argv[12] != "-context-tokens" || argv[13] != "8192" {
				t.Fatal("ASSERT_B_EXISTING_WORKER_ARGV", argv)
			}
			remaining, e := time.ParseDuration(argv[9])
			if e != nil || remaining <= 0 || remaining > time.Second {
				t.Fatal("ASSERT_B_ORIGINAL_REMAINING_TIMEOUT", remaining, e)
			}
			var response host.SourceHostResponse
			raw, e = os.ReadFile(filepath.Join(q.Host.Mailbox.Directory, "response-000001.json"))
			if e != nil || json.Unmarshal(raw, &response) != nil || response.Binding != request.Binding || !bytes.Equal(response.Payload, payload) {
				t.Fatal("ASSERT_B_EXACT_MAILBOX_BYTES", e)
			}
			var pair [2][]byte
			for i, path := range q.OutputRoots {
				r, e := publication.OpenContentRetentionRoot(ctx, path)
				if e != nil {
					t.Fatal(e)
				}
				pair[i], e = r.ReadPostPreparationPrivateV2(ctx, out.NativeID, 1<<20)
				ce := r.Close()
				if e != nil || ce != nil {
					t.Fatal(e, ce)
				}
			}
			if !bytes.Equal(pair[0], pair[1]) || !bytes.Contains(pair[0], []byte("SOURCE_NATIVE_V1")) {
				t.Fatal("ASSERT_B_NATIVE_IDENTICAL_PAIR")
			}
			rawDigest := fmt.Sprintf("sha256:%x", sha256.Sum256(payload))
			if !bytes.Contains(pair[0], []byte(`"raw_adapter_digest":"`+rawDigest+`"`)) {
				t.Fatal("ASSERT_B_RAW_ADAPTER_DIGEST_PRESERVED")
			}
			var provenance struct {
				Worker struct {
					RawSHA     string `json:"raw_stdout_sha256"`
					RawBytes   uint64 `json:"raw_stdout_bytes"`
					AdapterSHA string `json:"adapter_sha256"`
					Contract   string `json:"executor_contract"`
				} `json:"local_worker_stdout"`
			}
			if json.Unmarshal(pair[0], &provenance) != nil || provenance.Worker.RawSHA != fmt.Sprintf("sha256:%x", sha256.Sum256(rawWorker)) || provenance.Worker.RawBytes != uint64(len(rawWorker)) || provenance.Worker.AdapterSHA != rawDigest || provenance.Worker.Contract != "CONTROLLED_TEST_EXECUTOR_NOT_OS_QUALIFIED" {
				t.Fatal("ASSERT_REVIEW_RAW_STDOUT_PROVENANCE", provenance)
			}
			if mode == "wire-html" {
				var rawText, framedText struct {
					Text string `json:"text"`
				}
				if json.Unmarshal(rawWorker, &rawText) != nil || json.Unmarshal(response.Payload, &framedText) != nil || rawText.Text != semantic || framedText.Text != semantic || bytes.Equal(rawWorker, response.Payload) {
					t.Fatal("ASSERT_REVIEW_TEXT_EXACT_NO_REPAIR")
				}
			}
			var truth struct {
				Authority    int    `json:"authority"`
				Accepted     bool   `json:"accepted"`
				Identity     string `json:"identity"`
				Completeness string `json:"completeness"`
				Absence      string `json:"absence"`
				Execution    string `json:"execution"`
				Human        string `json:"human_inventory"`
			}
			if json.Unmarshal(pair[0], &truth) != nil || truth.Authority != 0 || truth.Accepted || truth.Identity != "UNRESOLVED" || truth.Completeness != "UNKNOWN" || truth.Absence != "PROHIBITED" || truth.Execution != "NO_PROCESS_OR_HOST_EXECUTION_VERIFIED" || !strings.Contains(truth.Human, "Other six modes: DECLARED_MISSING_UNRESOLVED") {
				t.Fatal("ASSERT_B_NO_AUTHORITY_OR_QUALIFICATION", truth)
			}
			t.Log("ASSERT_B_CONNECTED_DELIVERY: PASS; pending worker start/join, exact source prompt and mailbox payload, identical Go-owned native pair, final OBSERVED")
			if mode == "managed-reuse" {
				if opens != 0 || artifactReads != admissionReads {
					t.Fatalf("ASSERT_MANAGED_NO_PER_ATTEMPT_ARTIFACT_READS: opens=%d actual reads=%d admission=%d", opens, artifactReads, admissionReads)
				}
				if dep, e := os.ReadFile(filepath.Join(base, "dependency-used")); e != nil || string(dep) != "controlled-explicit-dependency" {
					t.Fatal("ASSERT_MANAGED_EXPLICIT_DEPENDENCY_REACHED_WORKER", string(dep), e)
				}
				if _, e := os.Stat(filepath.Join(managedRoot, "model")); e != nil {
					t.Fatal("ASSERT_MANAGED_RETAINED", e)
				}
				if _, e := os.Stat(filepath.Join(filepath.Dir(c.Config.Worker.Path), ".source-worker-"+q.PreparationAttempt[7:])); !os.IsNotExist(e) {
					t.Fatal("ASSERT_MANAGED_ATTEMPT_CLEANUP", e)
				}
				t.Log("ASSERT_MANAGED_NO_PER_ATTEMPT_ARTIFACT_READS: PASS; shared generation retained; attempt namespace removed")
			}
		} else if runErr == nil || out.NativeID != "" || out.Published != [2]bool{} || d != "PENDING" || w != 2 {
			t.Fatal("ASSERT_B_FAILURE_NO_DELIVERY", mode, out, runErr, d, w)
		}
		if mode == "fragmented" && !strings.Contains(runErr.Error(), "prepaid worker descriptor read-call") {
			t.Fatal("ASSERT_B_PREPAID_DRAIN_CALLS", runErr)
		}
		before := count
		beforeOpens, beforeStarts := opens, starts
		_, retryErr := so.Run(ctx, q)
		d2, w2, b2 := inv.ControlledLocalHeadForTest(t, owner)
		b, _ = os.ReadFile(marker)
		if retryErr == nil || len(b) != before || opens != beforeOpens || starts != beforeStarts || d2 != d || w2 != w || b2 != debit {
			t.Fatal("ASSERT_B_NO_REPLAY_WINDOW", retryErr, d2, w2)
		}
	}
	if mode == "managed-reuse" {
		if err := managed.Revoke(); err != nil {
			t.Fatal(err)
		}
		if err := managed.Retire(context.Background()); err != nil {
			t.Fatal("ASSERT_MANAGED_FUNDED_RETIRE", err)
		}
		d, w, _ := inv.ControlledLocalHeadForTest(t, lifecycle)
		if d != "OBSERVED" || w != 1 {
			t.Fatal("ASSERT_MANAGED_SEPARATE_LIFECYCLE", d, w)
		}
		if _, err := os.Stat(managedRoot); !os.IsNotExist(err) {
			t.Fatal("ASSERT_MANAGED_REMOVED_ON_RETIRE", err)
		}
		t.Log("ASSERT_MANAGED_TWO_CONNECTED_ATTEMPTS: PASS; separate funded lifecycle retired after two original M/P jobs")
	}
}
