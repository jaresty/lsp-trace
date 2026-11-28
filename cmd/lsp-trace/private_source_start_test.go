//go:build darwin || linux

package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
	"lsp-trace/internal/describeworker"
	inv "lsp-trace/internal/inventorysuccessor"
	"lsp-trace/internal/publication"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/internal/targetpacket"
)

func startConfigFixtureV2(t *testing.T) privateSourceStartConfig {
	t.Helper()
	d := "sha256:" + strings.Repeat("1", 64)
	now := time.Now()
	c := privateSourceStartConfig{Schema: "private-source-start.v2", Invocation: privateStartInvocation, Base: filepath.Join(t.TempDir(), privateStartInvocation), ControlRoot: filepath.Join(t.TempDir(), "control-accounting"), CanonicalControlJob: []byte("unissued test declaration"), ControlHead: d, ControlVerification: inv.PrivateManagedInputVerificationV2{Records: 400, Bytes: 4 << 20, Work: 400}, ControlAttempt: d, LaunchAttempt: d, ControlDeadline: now.Add(5 * time.Second), LaunchDeadline: now.Add(30 * time.Second), ProducerSession: privateStartProducerSession, ReviewerSession: privateStartReviewerSession, ProducerReadySHA256: d, ReviewerReadySHA256: d, FrozenProducerSHA256: d, FrozenReviewerSHA256: d}
	s := privateSourceHostConfig{Schema: "private-source-host-application.v1", Mode: "EXECUTE", AccountingRoot: filepath.Join(t.TempDir(), "source-accounting"), CanonicalJob: []byte("unissued source"), ExpectedHead: d, Verification: inv.PrivateManagedInputVerificationV2{Records: 400, Bytes: 4 << 20, Work: 400}, ProcessPath: privateStartProvider, Workspace: "/controlled", Provider: "gopls", Profile: runtimeprofile.Selector{Workspace: "/controlled"}, Seed: 7, SeedNamespace: "source-original-schedule", SourceLimits: targetpacket.PrivateSourceLimits{MaxDocumentBytes: 16384, MaxTotalBytes: 65536, MaxWork: 4096, MaxObjects: 16}, ResponseLimits: describeworker.SuppliedResponseLimits{MaxAdapterBytes: 8192, MaxSemanticBytes: 4096}}
	s.Host.Mailbox.Invocation = c.Invocation
	s.Host.Mailbox.InterpreterRole = "host-source-producer-1"
	s.Host.Mailbox.ReviewerRole = "host-source-reviewer-1"
	s.Host.Mailbox.TimeoutMS = 5000
	s.Host.Mailbox.OverallTimeoutMS = 5000
	s.Host.Mailbox.PollMS = 10
	s.Host.Mailbox.Limits.MaxCalls = 1
	s.Host.Mailbox.Limits.MaxRequestBytes = 65536
	s.Host.Mailbox.Limits.MaxResponseBytes = 16384
	s.Host.Mailbox.Limits.MaxTotalBytes = 147456
	s.Host.Mailbox.Limits.MaxOutputBytes = 65536
	c.Source = s
	c.Pin = privateStartConfigPin(c)
	return c
}
func TestConnectedStartConfigV2(t *testing.T) {
	for _, mode := range []string{"valid", "wrong-invocation", "prebound-deadline", "prebound-pin", "provider", "args", "same-root", "same-role", "wrong-producer-session", "wrong-reviewer-session", "source-cap", "host-cap", "seed", "unknown", "duplicate", "noncanonical"} {
		t.Run(mode, func(t *testing.T) {
			c := startConfigFixtureV2(t)
			switch mode {
			case "wrong-invocation":
				c.Invocation = "another"
			case "prebound-deadline":
				c.Source.Deadline = time.Now()
			case "prebound-pin":
				c.Source.Pin = "prebound"
			case "provider":
				c.Source.ProcessPath = "/arbitrary"
			case "args":
				c.Source.ProcessArgs = []string{"--arbitrary"}
			case "same-root":
				c.ControlRoot = c.Source.AccountingRoot
			case "same-role":
				c.ReviewerSession = c.ProducerSession
			case "wrong-producer-session":
				c.ProducerSession = "not-the-frozen-producer"
			case "wrong-reviewer-session":
				c.ReviewerSession = "not-the-frozen-reviewer"
			case "source-cap":
				c.Source.SourceLimits.MaxDocumentBytes++
			case "host-cap":
				c.Source.Host.Mailbox.TimeoutMS++
			case "seed":
				c.Source.Seed++
			}
			c.Pin = privateStartConfigPin(c)
			raw, _ := json.Marshal(c)
			switch mode {
			case "unknown":
				raw = append([]byte(`{"Unknown":true,`), raw[1:]...)
			case "duplicate":
				raw = append([]byte(`{"Schema":"private-source-start.v2",`), raw[1:]...)
			case "noncanonical":
				raw = append([]byte(" "), raw...)
			}
			_, e := decodePrivateStartConfig(raw)
			if (e == nil) != (mode == "valid") {
				t.Fatalf("ASSERT_CONNECTED_CONFIG: %s %v", mode, e)
			}
			t.Log("ASSERT_CONNECTED_CONFIG PASS", mode)
		})
	}
}
func TestConnectedParentStartBindingsV2(t *testing.T) {
	for _, mode := range []string{"valid", "malformed", "wrong-invocation", "unready", "wrong-session", "wrong-driver", "wrong-parent", "wrong-source", "unknown", "duplicate"} {
		t.Run(mode, func(t *testing.T) {
			c := startConfigFixtureV2(t)
			ready := []byte("exact-generated-driver-ready\n")
			dh := privateStartDigest(ready)
			parent := privateParentStartReady{"private-parent-start-ready.v2", c.Invocation, dh, c.ProducerSession, c.ReviewerSession, true}
			if mode == "unready" {
				parent.ProducerWaiterActive = false
			}
			if mode == "wrong-session" {
				parent.ProducerSession = "foreign"
			}
			pr, _ := json.Marshal(parent)
			s := privateParentStartSignal{"START_CONFIRM", c.Invocation, true, dh, privateStartDigest(pr), privateSourceConfigPin(c.Source)}
			switch mode {
			case "wrong-invocation":
				s.Invocation = "foreign"
			case "wrong-driver":
				s.DriverReadySHA256 = "foreign"
			case "wrong-parent":
				s.ParentReadySHA256 = "foreign"
			case "wrong-source":
				s.SourceTemplatePin = "foreign"
			}
			raw, _ := json.Marshal(s)
			switch mode {
			case "malformed":
				raw = []byte("{")
			case "unknown":
				raw = append([]byte(`{"extra":true,`), raw[1:]...)
			case "duplicate":
				raw = append([]byte(`{"Operation":"START_CONFIRM",`), raw[1:]...)
			}
			_, e := validatePrivateParentStart(raw, pr, ready, c)
			if (e == nil) != (mode == "valid") {
				t.Fatalf("ASSERT_PARENT_START_BINDING: %s %v", mode, e)
			}
			t.Log("ASSERT_PARENT_START_BINDING PASS", mode)
		})
	}
}

// Duplicate into raw ownership and retire the original wrapper before EOF can
// be observed. The caller must transfer the returned FD to the owned opener.
func duplicateApplicationStartupWriter(t *testing.T, w *os.File) int {
	t.Helper()
	fd, dupErr := unix.Dup(int(w.Fd()))
	closeErr := w.Close()
	if dupErr != nil {
		t.Fatal(dupErr)
	}
	if closeErr != nil || fd < 3 {
		_ = unix.Close(fd) // this raw duplicate has not been transferred yet
		t.Fatalf("writer duplicate custody: fd=%d close=%v", fd, closeErr)
	}
	return fd
}

func TestConnectedBridgeCanonicalJoinV2(t *testing.T) {
	c := startConfigFixtureV2(t)
	if e := os.Mkdir(c.Base, 0700); e != nil {
		t.Fatal(e)
	}
	canonicalBase, e := filepath.EvalSymlinks(c.Base)
	if e != nil {
		t.Fatal(e)
	}
	c.Base = canonicalBase
	control := filepath.Join(c.Base, "control")
	if e := os.Mkdir(control, 0700); e != nil {
		t.Fatal(e)
	}
	c.Pin = privateStartConfigPin(c)
	ready, _ := json.Marshal(privateConnectedReady{"private-connected-source-ready.v2", c.Invocation, c.Pin, privateSourceConfigPin(c.Source), "fixed-provider-control/v2 /Users/schwa/.local/bin/gopls v0.23.0 " + privateStartDigest([]byte("controlled receipt epoch")), c.LaunchDeadline})
	ready = append(ready, '\n')
	if len(ready) > applicationStartupPacketLimit {
		t.Fatal("ASSERT_SOURCE_READY_PIPE_BOUND", len(ready))
	}
	for _, mode := range []string{"live", "cancelled"} {
		t.Run(mode, func(t *testing.T) {
			r, w, e := os.Pipe()
			if e != nil {
				t.Fatal(e)
			}
			defer r.Close()
			n, e := openOwnedApplicationStartupNotifier(duplicateApplicationStartupWriter(t, w))
			if e != nil {
				t.Fatal(e) // the owned opener has already consumed the duplicate
			}
			defer n.close()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if mode == "cancelled" {
				cancel()
			}
			e = notifyPrivateSourceReady(ctx, n, ready)
			got, re := io.ReadAll(r)
			if re != nil || (mode == "live" && (e != nil || !bytes.Equal(got, ready))) || (mode == "cancelled" && (e == nil || len(got) != 0)) {
				t.Fatal("ASSERT_SOURCE_READY_PIPE", mode, e, re)
			}
			if notifyPrivateSourceReady(context.Background(), n, ready) == nil {
				t.Fatal("ASSERT_SOURCE_READY_ONCE")
			}
			t.Log("ASSERT_SOURCE_READY_PIPE PASS", mode)
		})
	}
	config, _ := json.Marshal(c)
	configPath := filepath.Join(c.Base, "start-config.json")
	readyPath := filepath.Join(c.Base, "controlled-ready.json")
	if e := os.WriteFile(configPath, config, 0600); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(readyPath, ready, 0600); e != nil {
		t.Fatal(e)
	}
	bridge, e := filepath.Abs(filepath.Join("..", "..", ".pi", "skills", "private-host-streaming-application", "scripts", "activation-bridge.py"))
	if e != nil {
		t.Fatal(e)
	}
	// Test-only peer creates ACK bytes. Python's separate FIFO test uses the
	// real connected-wait handler; neither claims authenticated model output.
	program := `import importlib.util,json,sys,os
s=importlib.util.spec_from_file_location('bridge',sys.argv[1]);b=importlib.util.module_from_spec(s);s.loader.exec_module(b)
c=json.load(open(sys.argv[2]));r=open(sys.argv[3],'rb').read()
cap=dict(Invocation=c['Invocation'],ConfigPin=c['Pin'],ProducerSession=c['ProducerSession'],ReviewerSession=c['ReviewerSession'],LaunchDeadline=c['LaunchDeadline'],Nonce='a'*64)
pin=b.host.digest(b.source_encoded(cap))
a=dict(Schema='private-source-connected-ack.v1',CapabilitySHA256=pin,DriverReadySHA256=b.host.digest(r),Nonce=cap['Nonce'],ProducerWaiterActive=True)
fd=b.private_root(c['Base']+'/control')
try:b.source_parent_join(fd,c,cap,pin,r,b.source_encoded(a))
finally:os.close(fd)
`
	cmd := exec.Command("python3", "-c", program, bridge, configPath, readyPath)
	cmd.Env = append(os.Environ(), "PYTHONDONTWRITEBYTECODE=1")
	if out, e := cmd.CombinedOutput(); e != nil {
		t.Fatalf("bridge canonical join: %v %s", e, out)
	}
	parent, e := os.ReadFile(filepath.Join(control, "parent-ready.json"))
	if e != nil {
		t.Fatal(e)
	}
	start, e := os.ReadFile(filepath.Join(control, "start.json"))
	if e != nil {
		t.Fatal(e)
	}
	if _, e := validatePrivateParentStart(start, parent, ready, c); e != nil {
		t.Fatal("ASSERT_BRIDGE_TYPED_GO_JOIN", e)
	}
	t.Log("ASSERT_BRIDGE_TYPED_GO_JOIN PASS; exact canonical typed pins, no source/provider/accounting IO")
}

func TestConnectedTypedDeadlineOnceV2(t *testing.T) {
	c := startConfigFixtureV2(t).Source
	start := time.Now()
	for _, d := range []time.Duration{time.Second, 15 * time.Second, 60 * time.Second} {
		expiry := start.Add(d)
		got, e := bindPrivateSourceStart(c, start, expiry)
		want := start.Add(15 * time.Second)
		if expiry.Before(want) {
			want = expiry
		}
		if e != nil || !got.Deadline.Equal(want) || got.Pin != privateSourceConfigPin(got) {
			t.Fatal("ASSERT_TYPED_DEADLINE_MIN_ONCE", e)
		}
		if _, e = bindPrivateSourceStart(got, start.Add(time.Second), expiry); e == nil {
			t.Fatal("ASSERT_DEADLINE_CANNOT_REFRESH")
		}
		raw, _ := json.Marshal(got)
		var recursive map[string]any
		_ = json.Unmarshal(raw, &recursive)
		recursive["Pin"] = ""
		sorted, _ := json.Marshal(recursive)
		if got.Pin == privateStartDigest(sorted) {
			t.Fatal("ASSERT_TYPED_NOT_RECURSIVE_SORT")
		}
	}
	if _, e := bindPrivateSourceStart(c, start, start); e == nil {
		t.Fatal("ASSERT_AUTH_EXPIRY_EQUAL_REFUSED")
	}
	t.Log("ASSERT_TYPED_DEADLINE_MIN_ONCE PASS; ASSERT_DEADLINE_CANNOT_REFRESH PASS; ASSERT_TYPED_NOT_RECURSIVE_SORT PASS")
}
func TestConnectedCLIUnissuedBeforeProviderV2(t *testing.T) {
	c := startConfigFixtureV2(t)
	if e := os.Mkdir(c.Base, 0700); e != nil {
		t.Fatal(e)
	}
	c.Pin = privateStartConfigPin(c)
	raw, _ := json.Marshal(c)
	p := filepath.Join(t.TempDir(), "start.json")
	if e := os.WriteFile(p, raw, 0600); e != nil {
		t.Fatal(e)
	}
	p, e := filepath.EvalSymlinks(p)
	if e != nil {
		t.Fatal(e)
	}
	var out, err bytes.Buffer
	if code := runPrivateHostApplication([]string{"start-source", p}, strings.NewReader(""), &out, &err); code == 0 || out.Len() != 0 || !strings.Contains(err.String(), "CONTROL_ROOT_REFUSED") {
		t.Fatalf("ASSERT_CONNECTED_CLI_UNISSUED: out=%s err=%s", out.Bytes(), err.Bytes())
	}
	if _, e := os.Stat(filepath.Join(c.Base, "control", "connected-ready.json")); !os.IsNotExist(e) {
		t.Fatal("ASSERT_UNISSUED_NO_READY")
	}
	t.Log("ASSERT_CONNECTED_CLI_UNISSUED PASS")
}

type finalCloseLedgerFixtureV2 struct {
	Base, ControlRoot, ControlHead, SourceRoot, SourceHead, Receipt string
	ControlJob, SourceJob                                           []byte
	ControlVerification, SourceVerification                         inv.PrivateManagedInputVerificationV2
	ControlDeadline, LaunchDeadline, Expiry                         time.Time
	FinalControlHead, Disposition                                   string
	Windows, SourceWindows                                          uint64
	Charged                                                         [14]uint64
}
type finalCloseClockV2 struct {
	now, after time.Time
	cancel     context.CancelFunc
	closes     int
}

func (f *finalCloseClockV2) Close(root *publication.Root) error {
	f.closes++
	e := root.Close() // actual final owned close, never a fake successful root
	f.now = f.after   // monotonic virtual advance ONLY during that real close
	if f.cancel != nil {
		f.cancel()
	}
	return e
}
func (f *finalCloseClockV2) Now() time.Time { return f.now }

func TestConnectedCLIFinalControlCloseV2(t *testing.T) {
	// A bounded test process supplies genuine private test custody, never DTO-minted
	// credit. The actual CLI consumer below runs in this process with real roots.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	fixtureBinary := filepath.Join(t.TempDir(), "controlled-final-close.test")
	build := exec.CommandContext(ctx, "go", "test", "-c", "-o", fixtureBinary, "./internal/inventorysuccessor")
	build.Dir = filepath.Join("..", "..")
	if b, be := build.CombinedOutput(); be != nil {
		t.Fatalf("compile controlled fixture: %v %s", be, b)
	}
	if e := os.Chmod(fixtureBinary, 0700); e != nil {
		t.Fatal(e)
	}
	cmd := exec.CommandContext(ctx, fixtureBinary, "-test.timeout=180s", "-test.run=^TestProviderLaunchFinalCloseCLIFixtureV2$", "-test.v")
	cmd.Env = append(os.Environ(), "LSP_TRACE_FINAL_CLOSE_FIXTURE=1")
	var childErr bytes.Buffer
	cmd.Stderr = &childErr
	input, e := cmd.StdinPipe()
	if e != nil {
		t.Fatal(e)
	}
	output, e := cmd.StdoutPipe()
	if e != nil {
		t.Fatal(e)
	}
	if e = cmd.Start(); e != nil {
		t.Fatal(e)
	}
	defer func() {
		_ = input.Close()
		if e := cmd.Wait(); e != nil {
			t.Errorf("controlled fixture: %v %s", e, childErr.String())
		}
	}()
	scan := bufio.NewScanner(output)
	scan.Buffer(make([]byte, 4096), 65536)
	read := func(stage string) finalCloseLedgerFixtureV2 {
		t.Helper()
		for scan.Scan() {
			line := scan.Text()
			if !strings.HasPrefix(line, stage+" ") {
				t.Log("controlled fixture:", line)
				continue
			}
			b, de := base64.StdEncoding.DecodeString(strings.TrimPrefix(line, stage+" "))
			var m finalCloseLedgerFixtureV2
			if de != nil || json.Unmarshal(b, &m) != nil {
				t.Fatal("fixture transport")
			}
			return m
		}
		t.Fatal("fixture missing stage", stage, scan.Err())
		return finalCloseLedgerFixtureV2{}
	}
	m := read("FINAL_CLOSE_READY")
	c := startConfigFixtureV2(t)
	c.Base, c.ControlRoot, c.CanonicalControlJob, c.ControlHead, c.ControlVerification, c.ControlDeadline, c.LaunchDeadline = m.Base, m.ControlRoot, m.ControlJob, m.ControlHead, m.ControlVerification, m.ControlDeadline, m.LaunchDeadline
	c.Source.AccountingRoot, c.Source.CanonicalJob, c.Source.ExpectedHead, c.Source.Verification = m.SourceRoot, m.SourceJob, m.SourceHead, m.SourceVerification
	c.Pin = privateStartConfigPin(c)
	configRaw, _ := json.Marshal(c)
	if _, e = decodePrivateStartConfig(configRaw); e != nil {
		t.Fatal("actual canonical CLI config", e)
	}
	ready, _ := json.Marshal(privateConnectedReady{"private-connected-source-ready.v2", c.Invocation, c.Pin, privateSourceConfigPin(c.Source), m.Receipt, c.LaunchDeadline})
	ready = append(ready, '\n')
	parent, _ := json.Marshal(privateParentStartReady{"private-parent-start-ready.v2", c.Invocation, privateStartDigest(ready), c.ProducerSession, c.ReviewerSession, true})
	start, _ := json.Marshal(privateParentStartSignal{"START_CONFIRM", c.Invocation, true, privateStartDigest(ready), privateStartDigest(parent), privateSourceConfigPin(c.Source)})
	if _, e = validatePrivateParentStart(start, parent, ready, c); e != nil {
		t.Fatal("actual positive CLI parent binding", e)
	}
	started := time.Now()
	request, _ := json.Marshal(struct {
		Ready, Parent, Start []byte
		Started              time.Time
	}{ready, parent, start, started})
	if _, e = input.Write(append(request, '\n')); e != nil {
		t.Fatal(e)
	}
	m = read("FINAL_CLOSE_OBSERVED")
	c.Source, e = bindPrivateSourceStart(c.Source, started, m.Expiry)
	if e != nil {
		t.Fatal(e)
	}
	if m.Disposition != "OBSERVED" || m.Windows != 2 || m.SourceWindows != 0 || m.Charged[1] == 0 || m.Charged[8] == 0 {
		t.Fatal("genuine pre-boundary ledger", m)
	}
	claim, e := os.ReadFile(filepath.Join(c.Base, "control", "start-consumed.json"))
	if e != nil || !bytes.Equal(claim, start) {
		t.Fatal("actual consumed start", e)
	}
	// Reopening below is test observation of existing canonical fixture custody,
	// not a new control debit or a reconstruction of its opaque launch receipt.
	checkHead := func(path string, job []byte, head string, limits inv.PrivateManagedInputVerificationV2) {
		t.Helper()
		r, re := publication.OpenRoot(path)
		if re != nil {
			t.Fatal(re)
		}
		defer r.Close()
		o, re := inv.OpenPrivateManagedInputOwnerV2(context.Background(), r, job, head, limits)
		if re != nil || o.PreparationHeadV1() != head {
			t.Fatal("ASSERT_FINAL_CLOSE_LEDGER_TRUTH", re)
		}
	}
	for _, mode := range []string{"timely", "expiry-during-close", "equality-during-close", "source-cancel"} {
		t.Run(mode, func(t *testing.T) {
			control, re := publication.OpenRoot(c.ControlRoot)
			if re != nil {
				t.Fatal(re)
			}
			source, re := publication.OpenRoot(c.Source.AccountingRoot)
			if re != nil {
				t.Fatal(re)
			}
			owner, re := inv.OpenPrivateManagedInputOwnerV2(context.Background(), source, c.Source.CanonicalJob, c.Source.ExpectedHead, c.Source.Verification)
			if re != nil {
				t.Fatal(re)
			}
			sourceCtx, sourceCancel := context.WithDeadline(context.Background(), c.Source.Deadline)
			defer sourceCancel()
			before := c.LaunchDeadline.Add(-time.Nanosecond)
			hook := &finalCloseClockV2{now: before, after: before}
			switch mode {
			case "expiry-during-close":
				hook.after = c.LaunchDeadline.Add(time.Nanosecond)
			case "equality-during-close":
				hook.after = c.LaunchDeadline
			case "source-cancel":
				hook.cancel = sourceCancel
			}
			if !before.Before(c.LaunchDeadline) || !hook.after.Before(c.Source.Deadline) || sourceCtx.Err() != nil {
				t.Fatal("coherent pre-close fixture clocks")
			}
			var out, stderr bytes.Buffer
			code := finishPrivateSourceStart(sourceCtx, c, &out, &stderr, control, source, owner, hook)
			if hook.closes != 1 || control.ValidatePrivate() == nil || source.ValidatePrivate() == nil {
				t.Fatal("actual root close ownership")
			}
			checkHead(c.ControlRoot, c.CanonicalControlJob, m.FinalControlHead, c.ControlVerification)
			checkHead(c.Source.AccountingRoot, c.Source.CanonicalJob, c.Source.ExpectedHead, c.Source.Verification)
			got, re := os.ReadFile(filepath.Join(c.Base, "control", "start-consumed.json"))
			if re != nil || !bytes.Equal(got, claim) {
				t.Fatal("ASSERT_FINAL_CLOSE_LEDGER_TRUTH consumed", re)
			}
			t.Log("ASSERT_FINAL_CLOSE_LEDGER_TRUTH PASS", mode, "OBSERVED windows2 charged unchanged; consumed; SOURCE windows0")
			entered := strings.Contains(stderr.String(), "private source application: PROFILE_REFUSED")
			if mode == "timely" {
				if code != 1 || !entered {
					t.Errorf("ASSERT_FINAL_CLOSE_ADMISSION timely: %d %s", code, stderr.String())
					return
				}
			} else {
				if code != 1 || entered || !strings.Contains(stderr.String(), "CONTROL_FINAL_CLOSE_RETAINED") {
					t.Errorf("ASSERT_FINAL_CLOSE_ADMISSION %s: %d %s", mode, code, stderr.String())
					return
				}
				if mode != "source-cancel" && sourceCtx.Err() != nil {
					t.Fatal("expiry regression must keep SOURCE context live")
				}
			}
			t.Log("ASSERT_FINAL_CLOSE_ADMISSION PASS", mode)
		})
	}
}
