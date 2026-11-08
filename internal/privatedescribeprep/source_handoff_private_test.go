//go:build darwin || linux

package privatedescribeprep

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	pilot "lsp-trace/internal/censuslineagepilot"
	"lsp-trace/internal/describerequest"
	"lsp-trace/internal/targetpacket"
)

func sourceHostTest(t *testing.T) (SourceHostConfig, describerequest.Record, SourceHostContext) {
	t.Helper()
	home, e := os.UserHomeDir()
	if e != nil {
		t.Fatal(e)
	}
	dir, e := os.MkdirTemp(home, ".source-host-test-")
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	d := "sha256:" + strings.Repeat("1", 64)
	u := targetpacket.PrivateSourceUnit{Version: targetpacket.PrivateSourceVersion, ClaimCeiling: targetpacket.PrivateSourceCeiling, ID: d, SourceDigest: d, Body: "func F() {}", URI: "file:///fixture.go", Workspace: "file:///fixture", Revision: "fixture"}
	r, e := describerequest.FormatPrivateSourceRequestCandidate(u, 1000, 64<<10)
	if e != nil {
		t.Fatal(e)
	}
	c := SourceHostConfig{Protocol: SourceHostProtocol, Mailbox: AutomaticHandoffConfigPrivateV2{Enabled: true, Directory: dir, Invocation: "canned-source", InterpreterRole: "canned-interpreter", ReviewerRole: "unused-reviewer", ScopeSHA256: d, TimeoutMS: 1000, OverallTimeoutMS: 1000, PollMS: 1, Limits: AutomaticLimitsPrivateV2{MaxCalls: 1, MaxRequestBytes: 64 << 10, MaxResponseBytes: 16 << 10, MaxTotalBytes: 256 << 10, MaxOutputBytes: 64 << 10}}}
	c.Mailbox.ApprovalSHA256 = AutomaticHandoffApprovalPrivateV2(c.Mailbox)
	s := SourceHostContext{d, d, u.ID, d, r.RecordID, "canned-attempt", t.TempDir(), time.Now().Add(time.Second)}
	return c, r, s
}

// Controlled in-process file role: it only frames canned bytes and invokes the
// unchanged mailbox publish operation. It is NOT a HOST/model execution.
func sourceHostCanned(m *SourceHostMailbox, payload []byte, change func(*SourceHostResponse)) <-chan error {
	done := make(chan error, 1)
	go func() {
		var raw []byte
		var err error
		for time.Now().Before(m.io.deadline) {
			raw, err = os.ReadFile(filepath.Join(m.io.cfg.Directory, "request-000001.json"))
			if err == nil {
				break
			}
			time.Sleep(time.Millisecond)
		}
		if err != nil {
			done <- err
			return
		}
		var q SourceHostRequest
		if err = json.Unmarshal(raw, &q); err != nil {
			done <- err
			return
		}
		response := SourceHostResponse{q.Binding, pilot.Digest(payload), len(payload), payload}
		if change != nil {
			change(&response)
		}
		raw, err = json.Marshal(response)
		if err == nil {
			err = m.io.publish("response-000001.json", raw)
		}
		done <- err
	}()
	return done
}

func TestSourceHostRoundTrip(t *testing.T) {
	c, r, s := sourceHostTest(t)
	m, e := NewSourceHostMailbox(context.Background(), c, r, s)
	if e != nil {
		t.Fatal(e)
	}
	defer m.Close()
	payload := []byte{'{', '\n', 0, 0xff, '}'}
	done := sourceHostCanned(m, payload, nil)
	got, receipt, e := m.Exchange(context.Background())
	roleErr := <-done
	if e != nil || roleErr != nil || !bytes.Equal(got, payload) || !receipt.Delivered() {
		t.Fatal("ASSERT_SOURCE_HOST_EXACT_BYTES", e, roleErr)
	}
	raw, _ := receipt.Bytes()
	if !bytes.Contains(raw, []byte("HOST_OBSERVED_NOT_AUTHENTICATED")) || !bytes.Contains(raw, []byte(`"Authority":0`)) {
		t.Fatal("ASSERT_SOURCE_HOST_NO_AUTH")
	}
	if _, _, e = m.Exchange(context.Background()); e == nil {
		t.Fatal("ASSERT_SOURCE_HOST_SUCCESS_ONE_SHOT")
	}
	if _, e = (SourceHostDelivery{}).Bytes(); e == nil {
		t.Fatal("ASSERT_SOURCE_HOST_PUBLIC_RECEIPT")
	}
	t.Log("ASSERT_SOURCE_HOST_EXACT_BYTES: PASS; ASSERT_SOURCE_HOST_NO_AUTH: PASS; ASSERT_SOURCE_HOST_SUCCESS_ONE_SHOT: PASS; ASSERT_SOURCE_HOST_PUBLIC_RECEIPT: PASS")
}

func TestSourceHostBindings(t *testing.T) {
	changes := map[string]func(*SourceHostResponse){
		"nonce": func(v *SourceHostResponse) { v.Binding.OwnerNonce = "foreign" }, "role": func(v *SourceHostResponse) { v.Binding.RoleID = "foreign" }, "sequence": func(v *SourceHostResponse) { v.Binding.Sequence++ }, "request": func(v *SourceHostResponse) { v.Binding.RequestSHA256 = "foreign" }, "preparation": func(v *SourceHostResponse) { v.Binding.PreparationSHA256 = "foreign" }, "source": func(v *SourceHostResponse) { v.Binding.Source.SourceDigest = "foreign" }, "attempt": func(v *SourceHostResponse) { v.Binding.Source.AttemptID = "foreign" }, "unit": func(v *SourceHostResponse) { v.Binding.Source.SourceUnitID = "foreign" }, "length": func(v *SourceHostResponse) { v.PayloadByteLength++ }, "digest": func(v *SourceHostResponse) { v.PayloadSHA256 = "foreign" }, "family": func(v *SourceHostResponse) { v.Binding.Stage = "INTERPRET" }, "protocol": func(v *SourceHostResponse) { v.Binding.Protocol = AutomaticHandoffProtocolPrivateV2 }, "oversized-frame": func(v *SourceHostResponse) {
			v.Payload = bytes.Repeat([]byte("x"), 15<<10)
			v.PayloadByteLength = len(v.Payload)
			v.PayloadSHA256 = pilot.Digest(v.Payload)
		},
	}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			c, r, s := sourceHostTest(t)
			m, e := NewSourceHostMailbox(context.Background(), c, r, s)
			if e != nil {
				t.Fatal(e)
			}
			defer m.Close()
			done := sourceHostCanned(m, []byte("canned"), change)
			got, receipt, e := m.Exchange(context.Background())
			roleErr := <-done
			if e == nil || got != nil || receipt.Delivered() || roleErr != nil {
				t.Fatal("ASSERT_SOURCE_HOST_BAD_FRAME", name, e, roleErr)
			}
			if _, _, e = m.Exchange(context.Background()); e == nil {
				t.Fatal("ASSERT_SOURCE_HOST_FAILURE_ONE_SHOT")
			}
			t.Log("ASSERT_SOURCE_HOST_BAD_FRAME: PASS; ASSERT_SOURCE_HOST_FAILURE_ONE_SHOT: PASS")
		})
	}
}

func TestSourceHostEarlyAndFiles(t *testing.T) {
	for _, mode := range []string{"early", "root", "preparation", "symlink", "hardlink", "cancel", "expired"} {
		t.Run(mode, func(t *testing.T) {
			c, r, s := sourceHostTest(t)
			m, e := NewSourceHostMailbox(context.Background(), c, r, s)
			if e != nil {
				t.Fatal(e)
			}
			defer m.Close()
			ctx := context.Background()
			switch mode {
			case "early":
				e = m.io.publish("response-000001.json", []byte("early"))
			case "root":
				old := c.Mailbox.Directory + "-old"
				e = os.Rename(c.Mailbox.Directory, old)
				if e == nil {
					e = os.Mkdir(c.Mailbox.Directory, 0700)
				}
				defer os.RemoveAll(old)
			case "preparation":
				e = os.Remove(filepath.Join(c.Mailbox.Directory, "preparation.json"))
				if e == nil {
					e = os.WriteFile(filepath.Join(c.Mailbox.Directory, "preparation.json"), []byte("substituted"), 0600)
				}
			case "symlink":
				e = os.Symlink("preparation.json", filepath.Join(c.Mailbox.Directory, "response-000001.json"))
			case "hardlink":
				e = os.Link(filepath.Join(c.Mailbox.Directory, "preparation.json"), filepath.Join(c.Mailbox.Directory, "alias"))
			case "cancel":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			case "expired":
				m.io.deadline = time.Now().Add(-time.Second)
			}
			if e != nil {
				t.Fatal(e)
			}
			if got, receipt, e := m.Exchange(ctx); e == nil || got != nil || receipt.Delivered() {
				t.Fatal("ASSERT_SOURCE_HOST_UNSAFE_OR_EXPIRED", mode, e)
			}
			if _, _, e = m.Exchange(context.Background()); e == nil {
				t.Fatal("ASSERT_SOURCE_HOST_TERMINAL")
			}
			if mode == "cancel" {
				if _, e := os.Stat(filepath.Join(c.Mailbox.Directory, "request-000001.json")); !errors.Is(e, os.ErrNotExist) {
					t.Fatal("ASSERT_SOURCE_HOST_FAILURE_NO_REPLAY_IO", e)
				}
			}
			t.Log("ASSERT_SOURCE_HOST_UNSAFE_OR_EXPIRED: PASS; ASSERT_SOURCE_HOST_TERMINAL: PASS; ASSERT_SOURCE_HOST_FAILURE_NO_REPLAY_IO: PASS")
		})
	}
}

func TestSourceHostPreflight(t *testing.T) {
	c, r, s := sourceHostTest(t)
	for _, change := range []func(*SourceHostConfig, *describerequest.Record){func(c *SourceHostConfig, _ *describerequest.Record) { c.Protocol = AutomaticHandoffProtocolPrivateV2 }, func(c *SourceHostConfig, _ *describerequest.Record) { c.Mailbox.Limits.MaxRequestBytes = 10 }, func(_ *SourceHostConfig, r *describerequest.Record) { r.Envelope.Prompt = strings.Repeat("x", 1<<20) }} {
		bad, q := c, r
		change(&bad, &q)
		if _, e := NewSourceHostMailbox(context.Background(), bad, q, s); e == nil {
			t.Fatal("ASSERT_SOURCE_HOST_PREFLIGHT")
		}
		entries, e := os.ReadDir(c.Mailbox.Directory)
		if e != nil || len(entries) != 0 {
			t.Fatal("ASSERT_SOURCE_HOST_PREFLIGHT_NO_IO", e)
		}
	}
	t.Log("ASSERT_SOURCE_HOST_PREFLIGHT: PASS; ASSERT_SOURCE_HOST_PREFLIGHT_NO_IO: PASS")
}

type sourceHostLateContext struct {
	context.Context
	cancel   context.CancelFunc
	seam     string
	deadline time.Time
	seen     bool
}

func (c *sourceHostLateContext) Err() error {
	if e := c.Context.Err(); e != nil {
		return e
	}
	for n := 1; n < 8; n++ {
		pc, _, _, ok := runtime.Caller(n)
		if ok && strings.HasSuffix(runtime.FuncForPC(pc).Name(), "."+c.seam) {
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
func TestSourceHostLateBoundaries(t *testing.T) {
	for _, seam := range []string{"sourceHostRequestLive", "sourceHostResponseLive", "sourceHostDeliveryLive"} {
		for _, expire := range []bool{false, true} {
			t.Run(seam+map[bool]string{false: "-cancel", true: "-expire"}[expire], func(t *testing.T) {
				c, r, s := sourceHostTest(t)
				if expire {
					s.Deadline = time.Now().Add(150 * time.Millisecond)
				}
				m, e := NewSourceHostMailbox(context.Background(), c, r, s)
				if e != nil {
					t.Fatal(e)
				}
				defer m.Close()
				base, cancel := context.WithCancel(context.Background())
				defer cancel()
				late := &sourceHostLateContext{Context: base, cancel: cancel, seam: seam}
				if expire {
					late.deadline = s.Deadline
				}
				var done <-chan error
				if seam != "sourceHostRequestLive" {
					done = sourceHostCanned(m, []byte("canned"), nil)
				}
				got, receipt, e := m.Exchange(late)
				var roleErr error
				if done != nil {
					roleErr = <-done
				}
				if e == nil || got != nil || receipt.Delivered() || !late.seen || roleErr != nil {
					t.Fatal("ASSERT_SOURCE_HOST_LATE_"+seam, e, late.seen, roleErr)
				}
				if seam == "sourceHostRequestLive" {
					if _, e := os.Stat(filepath.Join(c.Mailbox.Directory, "request-000001.json")); !errors.Is(e, os.ErrNotExist) {
						t.Fatal("ASSERT_SOURCE_HOST_LATE_NO_PUBLICATION", e)
					}
				}
				t.Log("ASSERT_SOURCE_HOST_LATE_" + seam + ": PASS")
			})
		}
	}
}

// Metadata syscalls do not accept a Context. A closed AST guard establishes
// check -> final live -> effect order; independent executed contexts below prove
// late (not initial) cancellation suppresses publication and positive receipts.
// This does not pretend to inject real syscall latency or add production hooks.
func sourceHostTestCall(expr ast.Expr, receiver, method string) *ast.CallExpr {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return nil
	}
	if receiver == "" {
		name, ok := call.Fun.(*ast.Ident)
		if ok && name.Name == method {
			return call
		}
	} else if selector, ok := call.Fun.(*ast.SelectorExpr); ok {
		name, ok := selector.X.(*ast.Ident)
		if ok && name.Name == receiver && selector.Sel.Name == method {
			return call
		}
	}
	return nil
}
func sourceHostTestInit(stmt ast.Stmt, receiver, method string) *ast.CallExpr {
	check, ok := stmt.(*ast.IfStmt)
	if !ok {
		return nil
	}
	assign, ok := check.Init.(*ast.AssignStmt)
	if !ok || len(assign.Rhs) != 1 {
		return nil
	}
	return sourceHostTestCall(assign.Rhs[0], receiver, method)
}
func sourceHostTestOrder(body *ast.BlockStmt, boundary string) bool {
	found, ordered := 0, 0
	seam, ctx := "sourceHost"+strings.ToUpper(boundary[:1])+boundary[1:]+"Live", "originalCtx"
	if boundary == "preparation" {
		seam, ctx = "sourceHostLive", "ctx"
	}
	ast.Inspect(body, func(node ast.Node) bool {
		block, ok := node.(*ast.BlockStmt)
		if !ok {
			return true
		}
		for i, stmt := range block.List {
			effect := false
			if boundary == "delivery" {
				if ret, ok := stmt.(*ast.ReturnStmt); ok && len(ret.Results) == 3 {
					name, ok := ret.Results[0].(*ast.Ident)
					effect = ok && name.Name == "payload"
				}
			} else if call := sourceHostTestInit(stmt, "io", "publish"); call != nil && len(call.Args) == 2 {
				name, ok := call.Args[0].(*ast.BasicLit)
				filename := map[string]string{"preparation": "preparation.json", "request": "request-000001.json"}[boundary]
				effect = ok && name.Value == "\""+filename+"\""
			}
			if !effect {
				continue
			}
			found++
			if i < 2 {
				continue
			}
			root := sourceHostTestInit(block.List[i-2], "m", "check")
			live, ok := block.List[i-1].(*ast.IfStmt)
			if root == nil || len(root.Args) != 0 || !ok {
				continue
			}
			negation, ok := live.Cond.(*ast.UnaryExpr)
			if !ok || negation.Op != token.NOT {
				continue
			}
			call := sourceHostTestCall(negation.X, "", seam)
			if call == nil || len(call.Args) != 2 {
				continue
			}
			contextName, ok := call.Args[0].(*ast.Ident)
			deadline, ok2 := call.Args[1].(*ast.Ident)
			if ok && ok2 && contextName.Name == ctx && deadline.Name == "deadline" {
				ordered++
			}
		}
		return true
	})
	return found == 1 && ordered == 1
}

type sourceHostFinalContext struct {
	context.Context
	cancel         context.CancelFunc
	seam           string
	cancelAt, seen int
}

func (c *sourceHostFinalContext) Err() error {
	if err := c.Context.Err(); err != nil {
		return err
	}
	for n := 1; n < 8; n++ {
		pc, _, _, ok := runtime.Caller(n)
		if ok && strings.HasSuffix(runtime.FuncForPC(pc).Name(), "."+c.seam) {
			c.seen++
			if c.cancelAt > 0 && c.seen == c.cancelAt {
				c.cancel()
				return c.Context.Err()
			}
			break
		}
	}
	return nil
}
func TestSourceHostFinalOrder(t *testing.T) {
	_, testFile, _, _ := runtime.Caller(0)
	file, err := parser.ParseFile(token.NewFileSet(), strings.TrimSuffix(testFile, "_test.go")+".go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	bodies := map[string]*ast.BlockStmt{}
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok {
			bodies[fn.Name.Name] = fn.Body
		}
	}
	for _, boundary := range []string{"preparation", "request", "delivery"} {
		t.Run(boundary, func(t *testing.T) {
			fn := "Exchange"
			if boundary == "preparation" {
				fn = "NewSourceHostMailbox"
			}
			if bodies[fn] == nil || !sourceHostTestOrder(bodies[fn], boundary) {
				t.Error("ASSERT_SOURCE_HOST_FINAL_" + strings.ToUpper(boundary) + "_ORDER")
			} else {
				t.Log("ASSERT_SOURCE_HOST_FINAL_" + strings.ToUpper(boundary) + "_ORDER: PASS")
			}
			for _, stop := range []bool{false, true} {
				t.Run(map[bool]string{false: "live", true: "cancel"}[stop], func(t *testing.T) {
					c, r, s := sourceHostTest(t)
					base, cancel := context.WithCancel(context.Background())
					defer cancel()
					ctx := &sourceHostFinalContext{Context: base, cancel: cancel, seam: "sourceHost" + strings.ToUpper(boundary[:1]) + boundary[1:] + "Live"}
					if stop {
						ctx.cancelAt = 1
					}
					if boundary == "preparation" {
						ctx.seam = "sourceHostLive"
						if stop {
							ctx.cancelAt = 2
						} // initial admission remains live
						m, err := NewSourceHostMailbox(ctx, c, r, s)
						if m != nil {
							m.Close()
						}
						if ctx.seen < 2 {
							t.Fatal("ASSERT_SOURCE_HOST_FINAL_PREPARATION_REACHED", ctx.seen)
						}
						if stop {
							if err == nil || m != nil {
								t.Fatal("ASSERT_SOURCE_HOST_FINAL_PREPARATION_CANCEL", err)
							}
							if _, err := os.Stat(filepath.Join(c.Mailbox.Directory, "preparation.json")); !errors.Is(err, os.ErrNotExist) {
								t.Fatal("ASSERT_SOURCE_HOST_FINAL_PREPARATION_ABSENT", err)
							}
						} else if err != nil || m == nil {
							t.Fatal("ASSERT_SOURCE_HOST_FINAL_PREPARATION_LIVE", err)
						}
						t.Log("ASSERT_SOURCE_HOST_FINAL_PREPARATION_CONTEXT: PASS")
						return
					}
					m, err := NewSourceHostMailbox(context.Background(), c, r, s)
					if err != nil {
						t.Fatal(err)
					}
					defer m.Close()
					payload := []byte{'{', 0, 0xff, '\n', '}'}
					var done <-chan error
					if boundary == "delivery" || !stop {
						done = sourceHostCanned(m, payload, nil)
					}
					got, receipt, err := m.Exchange(ctx)
					var roleErr error
					if done != nil {
						roleErr = <-done
					} // drain before any assertion
					if roleErr != nil || ctx.seen != 1 {
						t.Fatal("ASSERT_SOURCE_HOST_FINAL_BOUNDARY_REACHED", roleErr, ctx.seen)
					}
					if stop {
						if err == nil || got != nil || receipt.Delivered() {
							t.Fatal("ASSERT_SOURCE_HOST_FINAL_"+strings.ToUpper(boundary)+"_CANCEL", err)
						}
						if boundary == "request" {
							if _, err := os.Stat(filepath.Join(c.Mailbox.Directory, "request-000001.json")); !errors.Is(err, os.ErrNotExist) {
								t.Fatal("ASSERT_SOURCE_HOST_FINAL_REQUEST_ABSENT", err)
							}
						}
					} else if err != nil || !receipt.Delivered() || !bytes.Equal(got, payload) {
						t.Fatal("ASSERT_SOURCE_HOST_FINAL_LIVE_EXACT", err)
					}
					t.Log("ASSERT_SOURCE_HOST_FINAL_" + strings.ToUpper(boundary) + "_CONTEXT: PASS")
				})
			}
		})
	}
}

// Test-only stage observation: no production IO callback or mutable clock.
type sourceHostPreparedContext struct {
	context.Context
	dir                  string
	stop, seen, readback bool
	deadline             time.Time
}

func (c *sourceHostPreparedContext) Err() error {
	if err := c.Context.Err(); err != nil {
		return err
	}
	for n := 1; n < 8; n++ {
		pc, _, _, ok := runtime.Caller(n)
		if ok && strings.HasSuffix(runtime.FuncForPC(pc).Name(), ".sourceHostPreparedLive") {
			c.seen = true
			raw, err := os.ReadFile(filepath.Join(c.dir, "preparation.json"))
			var prep sourceHostPreparation
			c.readback = err == nil && json.Unmarshal(raw, &prep) == nil && prep.Protocol == SourceHostProtocol && len(prep.OwnerNonce) == 64
			c.deadline, _ = time.Parse(time.RFC3339Nano, prep.Deadline)
			if c.stop && c.readback && !c.deadline.IsZero() {
				timer := time.NewTimer(time.Until(c.deadline))
				<-timer.C // wait for the actual published deadline, not arbitrary slack
				timer.Stop()
			}
			break
		}
	}
	return nil
}
func TestSourceHostLatePreparation(t *testing.T) {
	// A live source/context does not make a past mailbox deadline live.
	ctx := context.Background()
	now := time.Now()
	source, mailbox := now.Add(time.Second), now.Add(-time.Second)
	if !sourceHostLive(ctx, source) || sourceHostPreparedLive(ctx, mailbox) || !sourceHostPreparedLive(ctx, source) {
		t.Fatal("ASSERT_SOURCE_HOST_LATE_PREPARATION_MAILBOX_DEADLINE")
	}
	t.Log("ASSERT_SOURCE_HOST_LATE_PREPARATION_MAILBOX_DEADLINE: PASS")
	_, testFile, _, _ := runtime.Caller(0)
	set := token.NewFileSet()
	file, err := parser.ParseFile(set, strings.TrimSuffix(testFile, "_test.go")+".go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	text := func(node ast.Node) string {
		var b bytes.Buffer
		if node != nil {
			_ = format.Node(&b, set, node)
		}
		return b.String()
	}
	bound := false
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "NewSourceHostMailbox" || len(fn.Body.List) < 4 {
			continue
		}
		body := fn.Body.List
		guard, ok := body[len(body)-2].(*ast.IfStmt)
		if !ok {
			continue
		}
		validation, valid := body[len(body)-3].(*ast.IfStmt)
		assign, assigned := body[len(body)-4].(*ast.AssignStmt)
		bound = valid && assigned && len(assign.Rhs) == 1 && text(assign.Rhs[0]) == "io.read(\"preparation.json\", len(encoded))" && text(validation.Cond) == "err != nil || !bytes.Equal(got, encoded)" && text(guard.Cond) == "!sourceHostPreparedLive(ctx, m.io.deadline)" && len(guard.Body.List) == 1 && text(guard.Body.List[0]) == "return fail(context.DeadlineExceeded)" && text(body[len(body)-1]) == "return m, nil"
	}
	if !bound {
		t.Error("ASSERT_SOURCE_HOST_LATE_PREPARATION_FINAL_GUARD")
	} else {
		t.Log("ASSERT_SOURCE_HOST_LATE_PREPARATION_FINAL_GUARD: PASS")
	}
	for _, stop := range []bool{false, true} {
		t.Run(map[bool]string{false: "live", true: "refuse"}[stop], func(t *testing.T) {
			c, r, s := sourceHostTest(t)
			if stop {
				c.Mailbox.TimeoutMS, c.Mailbox.OverallTimeoutMS = 100, 500
				c.Mailbox.ApprovalSHA256 = AutomaticHandoffApprovalPrivateV2(c.Mailbox)
			}
			stage := &sourceHostPreparedContext{Context: context.Background(), dir: c.Mailbox.Directory, stop: stop}
			m, err := NewSourceHostMailbox(stage, c, r, s)
			if m != nil {
				m.Close()
			}
			if !stage.seen || !stage.readback {
				t.Error("ASSERT_SOURCE_HOST_LATE_PREPARATION_READBACK_STAGE", stage.seen, stage.readback)
			}
			if stop {
				if err == nil || m != nil {
					t.Error("ASSERT_SOURCE_HOST_LATE_PREPARATION_REFUSED", err)
				}
				if !stage.seen || stage.deadline.IsZero() || time.Now().Before(stage.deadline) || !stage.deadline.Before(s.Deadline) || stage.Context.Err() != nil {
					t.Error("ASSERT_SOURCE_HOST_LATE_PREPARATION_EXPIRED_MAILBOX_LIVE_CONTEXT")
				}
			} else if err != nil || m == nil {
				t.Error("ASSERT_SOURCE_HOST_LATE_PREPARATION_LIVE", err)
			}
			if !s.Deadline.After(time.Now()) {
				t.Error("ASSERT_SOURCE_HOST_LATE_PREPARATION_SOURCE_LIVE")
			}
			if stage.seen && stage.readback {
				t.Log("ASSERT_SOURCE_HOST_LATE_PREPARATION_READBACK_STAGE: PASS")
			}
			if !t.Failed() && stop {
				t.Log("ASSERT_SOURCE_HOST_LATE_PREPARATION_REFUSED: PASS; actual mailbox expired after readback, source and caller live")
			}
		})
	}
}

func TestSourceHostRootMetadata(t *testing.T) {
	c, _, s := sourceHostTest(t)
	if _, err := observeSourceHostRoots(s.Workspace, c.Mailbox.Directory); err != nil {
		t.Fatal("ASSERT_SOURCE_HOST_COMMON_ANCESTOR_ALLOWED", err)
	}
	child := filepath.Join(c.Mailbox.Directory, "child")
	if err := os.Mkdir(child, 0700); err != nil {
		t.Fatal(err)
	}
	for name, pair := range map[string][2]string{"equal": {c.Mailbox.Directory, c.Mailbox.Directory}, "mailbox-ancestor": {child, c.Mailbox.Directory}, "workspace-ancestor": {c.Mailbox.Directory, child}} {
		t.Run(name, func(t *testing.T) {
			if _, err := observeSourceHostRoots(pair[0], pair[1]); err == nil {
				t.Fatal("ASSERT_SOURCE_HOST_BOTH_ANCESTOR_DIRECTIONS")
			}
			t.Log("ASSERT_SOURCE_HOST_BOTH_ANCESTOR_DIRECTIONS: PASS")
		})
	}
	for _, path := range []string{"/" + strings.Repeat("x", sourceHostPathMax), "/" + strings.Repeat("a/", sourceHostAncestorMax) + "z"} {
		if _, err := sourceHostAncestors(path); err == nil || !strings.Contains(err.Error(), "ceiling") && !strings.Contains(err.Error(), "path refused") {
			t.Fatal("ASSERT_SOURCE_HOST_METADATA_BOUND", err)
		}
	}
	t.Log("ASSERT_SOURCE_HOST_COMMON_ANCESTOR_ALLOWED: PASS; ASSERT_SOURCE_HOST_METADATA_BOUND: PASS")
}

func TestSourceHostRootAncestorSubstitution(t *testing.T) {
	c, r, s := sourceHostTest(t)
	base := t.TempDir()
	parent := filepath.Join(base, "parent")
	s.Workspace = filepath.Join(parent, "workspace")
	if err := os.MkdirAll(s.Workspace, 0700); err != nil {
		t.Fatal(err)
	}
	m, err := NewSourceHostMailbox(context.Background(), c, r, s)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	original, err := os.Stat(s.Workspace)
	if err != nil {
		t.Fatal(err)
	}
	retired := filepath.Join(base, "retired")
	if err := os.Rename(parent, retired); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(parent, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(retired, "workspace"), s.Workspace); err != nil {
		t.Fatal(err)
	}
	current, err := os.Stat(s.Workspace)
	if err != nil || !os.SameFile(original, current) {
		t.Fatal("ASSERT_SOURCE_HOST_SAME_ROOT_REPARENT_SETUP", err)
	}
	if got, receipt, err := m.Exchange(context.Background()); err == nil || got != nil || receipt.Delivered() {
		t.Fatal("ASSERT_SOURCE_HOST_ANCESTOR_SUBSTITUTION", err)
	}
	if _, err := os.Stat(filepath.Join(c.Mailbox.Directory, "request-000001.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("ASSERT_SOURCE_HOST_ANCESTOR_NO_REQUEST", err)
	}
	t.Log("ASSERT_SOURCE_HOST_ANCESTOR_SUBSTITUTION: PASS; ASSERT_SOURCE_HOST_ANCESTOR_NO_REQUEST: PASS")
}

func TestSourceHostRootIdentity(t *testing.T) {
	for _, mode := range []string{"equal", "workspace-ancestor", "mailbox-ancestor", "case-alias"} {
		t.Run(mode, func(t *testing.T) {
			c, r, s := sourceHostTest(t)
			switch mode {
			case "equal":
				s.Workspace = c.Mailbox.Directory
			case "workspace-ancestor", "case-alias":
				base := c.Mailbox.Directory
				workspace := filepath.Join(base, "Project")
				if e := os.Mkdir(workspace, 0700); e != nil {
					t.Fatal(e)
				}
				s.Workspace = workspace
				parent := workspace
				if mode == "case-alias" {
					parent = filepath.Join(base, "project")
					a, e := os.Stat(workspace)
					b, aliasErr := os.Stat(parent)
					if e != nil {
						t.Fatal(e)
					}
					if aliasErr != nil || !os.SameFile(a, b) {
						t.Skip("case-sensitive filesystem: actual alias unavailable")
					}
				}
				c.Mailbox.Directory = filepath.Join(parent, "mailbox")
				if e := os.Mkdir(c.Mailbox.Directory, 0700); e != nil {
					t.Fatal(e)
				}
			case "mailbox-ancestor":
				s.Workspace = filepath.Join(c.Mailbox.Directory, "workspace")
				if e := os.Mkdir(s.Workspace, 0700); e != nil {
					t.Fatal(e)
				}
			}
			c.Mailbox.ApprovalSHA256 = AutomaticHandoffApprovalPrivateV2(c.Mailbox)
			m, e := NewSourceHostMailbox(context.Background(), c, r, s)
			if m != nil {
				m.Close()
			}
			if e == nil {
				t.Fatal("ASSERT_SOURCE_HOST_ROOT_IDENTITY_REFUSED")
			}
			if _, e = os.Stat(filepath.Join(c.Mailbox.Directory, "preparation.json")); !errors.Is(e, os.ErrNotExist) {
				t.Fatal("ASSERT_SOURCE_HOST_ROOT_NO_PREPARATION", e)
			}
			t.Log("ASSERT_SOURCE_HOST_ROOT_IDENTITY_REFUSED: PASS; ASSERT_SOURCE_HOST_ROOT_NO_PREPARATION: PASS")
		})
	}
}

func TestSourceHostRootReparent(t *testing.T) {
	for _, when := range []string{"prepared", "published"} {
		t.Run(when, func(t *testing.T) {
			c, r, s := sourceHostTest(t)
			m, e := NewSourceHostMailbox(context.Background(), c, r, s)
			if e != nil {
				t.Fatal(e)
			}
			defer m.Close()
			moved := filepath.Join(c.Mailbox.Directory, "moved-workspace")
			var moveErr error
			move := func() {
				moveErr = os.Rename(s.Workspace, moved)
				if moveErr == nil {
					moveErr = os.Symlink(moved, s.Workspace)
				}
			}
			var done <-chan error
			if when == "prepared" {
				move()
			} else {
				done = sourceHostCanned(m, []byte("canned"), func(_ *SourceHostResponse) { move() })
			}
			got, receipt, exchangeErr := m.Exchange(context.Background())
			var roleErr error
			if done != nil {
				roleErr = <-done
			}
			if moveErr != nil || roleErr != nil {
				t.Fatal(moveErr, roleErr)
			}
			if exchangeErr == nil || got != nil || receipt.Delivered() {
				t.Fatal("ASSERT_SOURCE_HOST_REPARENT_REFUSED", exchangeErr)
			}
			if when == "prepared" {
				if _, e := os.Stat(filepath.Join(c.Mailbox.Directory, "request-000001.json")); !errors.Is(e, os.ErrNotExist) {
					t.Fatal("ASSERT_SOURCE_HOST_REPARENT_NO_REQUEST", e)
				}
			}
			t.Log("ASSERT_SOURCE_HOST_REPARENT_REFUSED: PASS")
		})
	}
}

// Check existing read's delayed-publication contract without altering its code.
func TestSourceHostConsumedRequest(t *testing.T) {
	c, r, s := sourceHostTest(t)
	m, e := NewSourceHostMailbox(context.Background(), c, r, s)
	if e != nil {
		t.Fatal(e)
	}
	defer m.Close()
	done := sourceHostCanned(m, []byte("canned"), func(_ *SourceHostResponse) {
		// The request is pinned before the response is consumed. Changing its mode
		// must fail the unchanged consumed-file check.
		os.Chmod(filepath.Join(c.Mailbox.Directory, "request-000001.json"), 0644)
	})
	_, receipt, e := m.Exchange(context.Background())
	roleErr := <-done
	if e == nil || receipt.Delivered() || roleErr != nil {
		t.Fatal("ASSERT_SOURCE_HOST_CONSUMED_REQUEST", e, roleErr)
	}
	if errors.Is(e, context.DeadlineExceeded) {
		t.Fatal("unexpected timeout")
	}
	t.Log("ASSERT_SOURCE_HOST_CONSUMED_REQUEST: PASS")
}
