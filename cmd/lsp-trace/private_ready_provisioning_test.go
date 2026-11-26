//go:build darwin || linux

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"lsp-trace/internal/inventorysuccessor"
	"lsp-trace/internal/publication"
)

// TEST-ONLY canonical records. No production helper creates policy or hashes a
// config into approval. The real API's unchanged parser independently checks
// these domain/LF seals before the actual kernel can initialize a temporary root.
func readyCLIFixture(t *testing.T) (privateReadyRequest, privateReadyDecision, []byte, []byte) {
	t.Helper()
	p := t.TempDir()
	if e := os.Chmod(p, 0700); e != nil {
		t.Fatal(e)
	}
	hash := func(s string) string { return fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(s))) }
	h := hash("TEST-only-ready-policy")
	seal := func(schema, body string) ([]byte, string) {
		unsigned := []byte(body + "\n")
		id := hash("inventory-successor/private/v2\x00" + schema + "\x00" + string(unsigned))
		return bytes.Replace(unsigned, []byte(`"identity":""`), []byte(`"identity":"`+id+`"`), 1), id
	}
	job, jid := seal("lsp-trace.inventory-job-identity.private.v2", fmt.Sprintf(`{"schema":"lsp-trace.inventory-job-identity.private.v2","identity":"","stable_id":%q,"application":%q,"scope":%q,"source_authorization":%q,"revision_policy":%q,"evidence_policies":[%q],"provider_policies":[%q],"semantic_policies":[%q],"privacy":%q,"retention":%q,"resource_rules":%q,"origin":"NEW_EMPTY"}`, h, h, h, h, h, h, h, h, h, h, h))
	counter := func(n int) string {
		return fmt.Sprintf(`{"logical_bytes":%d,"read_hash_bytes":%d,"write_bytes":%d,"records":%d,"parser_bytes":%d,"provider_requests":%d,"provider_messages":%d,"work":%d,"active_ns":%d,"calls":%d,"request_bytes":%d,"response_bytes":%d,"wire_bytes":%d,"output_bytes":%d}`, n, n, n, n, n, n, n, n, n, n, n, n, n, n)
	}
	auth, aid := seal("lsp-trace.inventory-job-authorization.private.v2", fmt.Sprintf(`{"schema":"lsp-trace.inventory-job-authorization.private.v2","identity":"","job":%q,"revision":1,"previous_authorization":"NONE","verified_resource_head":"NONE","charged_snapshot":%s,"currency_ceiling":%s,"max_windows":1,"expires_unix_nano":%d,"max_tranche":%s,"max_window_ns":1,"caller_decision_reference":%q,"identity_assurance":"CALLER_ASSERTED_NOT_AUTHENTICATED"}`, jid, counter(0), counter(8), time.Now().Add(time.Minute).UnixNano(), counter(1), h))
	return privateReadyRequest{privateReadyRequestSchema, p, base64.StdEncoding.EncodeToString(job), base64.StdEncoding.EncodeToString(auth), privateReadyVerification{400, 4 << 20, 400}}, privateReadyDecision{privateReadyDecisionSchema, jid, aid, h}, job, auth
}
func readyCLIEncode(t *testing.T, v any) []byte {
	t.Helper()
	b, e := privateReadyCanonical(v)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func readyCLIInvoke(t *testing.T, args []string, request, decision []byte, out io.Writer) (int, string) {
	t.Helper()
	f, e := os.CreateTemp(t.TempDir(), "decision")
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	if _, e = f.Write(decision); e != nil {
		t.Fatal(e)
	}
	if _, e = f.Seek(0, 0); e != nil {
		t.Fatal(e)
	}
	fd, e := syscall.Dup(int(f.Fd()))
	if e != nil {
		t.Fatal(e)
	}
	// The dispatcher owns the duplicate, never f; no descriptor double-Close.
	invocation := []string{"initialize-ready", "--decision-fd", strconv.Itoa(fd)}
	if args != nil {
		invocation = args
		defer syscall.Close(fd)
	}
	var stderr bytes.Buffer
	code := runPrivateHostApplication(invocation, bytes.NewReader(request), out, &stderr)
	return code, stderr.String()
}
func TestReadyProvisioningRecordBoundPredecode(t *testing.T) {
	for _, n := range []int{privateReadyRecordBytes - 1, privateReadyRecordBytes, privateReadyRecordBytes + 1, privateReadyRecordBytes + 2} {
		encoded := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte("x"), n))
		raw, err := privateReadyRecord(encoded)
		if n <= privateReadyRecordBytes {
			if err != nil || len(raw) != n {
				t.Fatalf("ASSERT_READY_RECORD_BOUND_ACCEPTS_CAP: n=%d err=%v", n, err)
			}
		} else if err == nil || err.Error() != "record decoded bound" || raw != nil {
			t.Fatalf("ASSERT_READY_RECORD_BOUND_BEFORE_DECODE: n=%d err=%v", n, err)
		}
	}
	t.Log("ASSERT_READY_RECORD_BOUND_ACCEPTS_CAP PASS; ASSERT_READY_RECORD_BOUND_BEFORE_DECODE PASS")
}

func TestReadyProvisioningActualCLIAndOpener(t *testing.T) {
	q, d, jb, _ := readyCLIFixture(t)
	var out bytes.Buffer
	code, stderr := readyCLIInvoke(t, nil, readyCLIEncode(t, q), readyCLIEncode(t, d), &out)
	if code != 0 {
		t.Fatalf("ASSERT_READY_CLI_CONNECTED: %d %s", code, stderr)
	}
	var result privateReadyOutput
	if e := privateReadyDecode(bytes.NewReader(out.Bytes()), privateReadyRequestBytes, &result); e != nil || result.Schema != privateReadyOutputSchema || result.Assurance != "CALLER_ASSERTED_NOT_AUTHENTICATED" || !bytes.Equal(result.CanonicalJob, jb) {
		t.Fatalf("ASSERT_READY_CLI_EXACT_OUTPUT: %s %v", out.String(), e)
	}
	root, e := publication.OpenRoot(q.AccountingRoot)
	if e != nil {
		t.Fatal(e)
	}
	defer root.Close()
	owner, e := inventorysuccessor.OpenPrivateManagedInputOwnerV2(context.Background(), root, result.CanonicalJob, result.ConfirmedHead, inventorysuccessor.PrivateManagedInputVerificationV2{Records: q.Verification.Records, Bytes: q.Verification.Bytes, Work: q.Verification.Work})
	if e != nil || owner == nil {
		t.Fatalf("ASSERT_READY_CLI_UNCHANGED_OPENER: %v", e)
	}
	t.Log("ASSERT_READY_CLI_CONNECTED PASS; ASSERT_READY_CLI_EXACT_OUTPUT PASS; ASSERT_READY_CLI_UNCHANGED_OPENER PASS")
}
func TestReadyProvisioningStrictRefusals(t *testing.T) {
	for _, name := range []string{"unknown-request", "duplicate-request", "trailing-request", "noncanonical-request", "oversize-request", "record-text-bound", "bad-base64", "unknown-schema", "unknown-decision", "duplicate-decision", "trailing-decision", "noncanonical-decision", "oversize-decision", "missing-decision", "wrong-job", "wrong-auth", "wrong-ref", "unknown-arg", "stdio-fd", "signed-fd", "leading-zero-fd", "extra-arg"} {
		t.Run(name, func(t *testing.T) {
			q, d, _, _ := readyCLIFixture(t)
			request, decision := readyCLIEncode(t, q), readyCLIEncode(t, d)
			var args []string
			switch name {
			case "unknown-request":
				request = bytes.Replace(request, []byte(`{"schema":`), []byte(`{"unknown":0,"schema":`), 1)
			case "duplicate-request":
				request = bytes.Replace(request, []byte(`{"schema":`), []byte(`{"schema":"x","schema":`), 1)
			case "trailing-request":
				request = append(request, []byte("{}")...)
			case "noncanonical-request":
				request = append([]byte(" "), request...)
			case "oversize-request":
				request = bytes.Repeat([]byte("x"), privateReadyRequestBytes+1)
			case "record-text-bound":
				q.CanonicalJob = strings.Repeat("A", base64.StdEncoding.EncodedLen(privateReadyRecordBytes)+1)
				request = readyCLIEncode(t, q)
			case "bad-base64":
				q.CanonicalJob = "!!!!"
				request = readyCLIEncode(t, q)
			case "unknown-schema":
				q.Schema = "other"
				request = readyCLIEncode(t, q)
			case "unknown-decision":
				decision = bytes.Replace(decision, []byte(`{"schema":`), []byte(`{"unknown":0,"schema":`), 1)
			case "duplicate-decision":
				decision = bytes.Replace(decision, []byte(`{"schema":`), []byte(`{"schema":"x","schema":`), 1)
			case "trailing-decision":
				decision = append(decision, []byte("{}")...)
			case "noncanonical-decision":
				decision = append([]byte(" "), decision...)
			case "oversize-decision":
				decision = bytes.Repeat([]byte("x"), privateReadyDecisionBytes+1)
			case "missing-decision":
				decision = nil
			case "wrong-job":
				d.Job = d.Reference
				decision = readyCLIEncode(t, d)
			case "wrong-auth":
				d.Authorization = d.Reference
				decision = readyCLIEncode(t, d)
			case "wrong-ref":
				d.Reference = d.Job
				decision = readyCLIEncode(t, d)
			case "unknown-arg":
				args = []string{"initialize-ready", "--unknown", "3"}
			case "stdio-fd":
				args = []string{"initialize-ready", "--decision-fd", "0"}
			case "signed-fd":
				args = []string{"initialize-ready", "--decision-fd", "+3"}
			case "leading-zero-fd":
				args = []string{"initialize-ready", "--decision-fd", "03"}
			case "extra-arg":
				args = []string{"initialize-ready", "--decision-fd", "3", "extra"}
			}
			var out bytes.Buffer
			code, stderr := readyCLIInvoke(t, args, request, decision, &out)
			if code == 0 || out.Len() != 0 || !strings.Contains(stderr, "private READY provisioning refused") || strings.Contains(stderr, "CONFIG_REFUSED") {
				t.Fatalf("ASSERT_READY_CLI_REFUSE_EARLY: %d %s %s", code, out.String(), stderr)
			}
			entries, e := os.ReadDir(q.AccountingRoot)
			if e != nil || len(entries) != 0 {
				t.Fatalf("ASSERT_READY_CLI_REFUSE_BEFORE_WRITES: %v %v", entries, e)
			}
			t.Log("ASSERT_READY_CLI_REFUSE_EARLY PASS; ASSERT_READY_CLI_REFUSE_BEFORE_WRITES PASS")
		})
	}
}
func TestReadyProvisioningRootRefusals(t *testing.T) {
	for _, name := range []string{"relative", "unclean", "symlink", "missing", "nonprivate"} {
		t.Run(name, func(t *testing.T) {
			q, d, _, _ := readyCLIFixture(t)
			original := q.AccountingRoot
			switch name {
			case "relative":
				q.AccountingRoot = "relative-root"
			case "unclean":
				q.AccountingRoot += "/../" + filepath.Base(original)
			case "symlink":
				link := filepath.Join(t.TempDir(), "link")
				if e := os.Symlink(original, link); e != nil {
					t.Fatal(e)
				}
				q.AccountingRoot = link
			case "missing":
				q.AccountingRoot = filepath.Join(original, "missing")
			case "nonprivate":
				if e := os.Chmod(original, 0755); e != nil {
					t.Fatal(e)
				}
			}
			var out bytes.Buffer
			code, _ := readyCLIInvoke(t, nil, readyCLIEncode(t, q), readyCLIEncode(t, d), &out)
			entries, e := os.ReadDir(original)
			if code == 0 || out.Len() != 0 || e != nil || len(entries) != 0 {
				t.Fatalf("ASSERT_READY_ROOT_REFUSE_NO_CREATE: %d %v %v", code, entries, e)
			}
			if name == "nonprivate" {
				info, e := os.Stat(original)
				if e != nil || info.Mode().Perm() != 0755 {
					t.Fatal("root permissions changed")
				}
			}
			t.Log("ASSERT_READY_ROOT_REFUSE_NO_CREATE PASS")
		})
	}
}

type readyShortWriter struct{}

func (readyShortWriter) Write(b []byte) (int, error) { return len(b) - 1, nil }

type readyCancelWriter struct{ cancel context.CancelFunc }

func (w readyCancelWriter) Write(b []byte) (int, error) { w.cancel(); return len(b), nil }

type readyDeadlineWriter struct{ expiry int64 }

func (w readyDeadlineWriter) Write(b []byte) (int, error) {
	time.Sleep(time.Until(time.Unix(0, w.expiry)) + time.Millisecond)
	return len(b), nil
}

type readyErrorWriter struct{}

func (readyErrorWriter) Write([]byte) (int, error) { return 0, errors.New("TEST output failure") }
func TestReadyProvisioningFinalBarrier(t *testing.T) {
	q, d, jb, ab := readyCLIFixture(t)
	root, e := publication.OpenRoot(q.AccountingRoot)
	if e != nil {
		t.Fatal(e)
	}
	result, e := inventorysuccessor.InitializePrivateReadyV2(context.Background(), root, jb, ab, inventorysuccessor.PrivateReadyDecisionAssertionV2{Job: d.Job, Authorization: d.Authorization, Reference: d.Reference}, inventorysuccessor.PrivateManagedInputVerificationV2{Records: 400, Bytes: 4 << 20, Work: 400})
	closeErr := root.Close()
	if e != nil || closeErr != nil {
		t.Fatalf("fixture %v %v", e, closeErr)
	}
	for _, name := range []string{"close", "cancel", "late-cancel", "short", "write"} {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			var out bytes.Buffer
			var writer io.Writer = &out
			var ce error
			switch name {
			case "close":
				ce = errors.New("TEST close failure")
			case "cancel":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			case "late-cancel":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				writer = readyCancelWriter{cancel: cancel}
			case "short":
				writer = readyShortWriter{}
			case "write":
				writer = readyErrorWriter{}
			}
			if e := privateReadyWriteSuccess(ctx, result, ce, writer); e == nil {
				t.Fatalf("ASSERT_READY_FINAL_BARRIER: %s", name)
			}
			if (name == "close" || name == "cancel") && out.Len() != 0 {
				t.Fatal("ASSERT_READY_NO_OUTPUT_AFTER_CLOSE_ERROR")
			}
			t.Log("ASSERT_READY_FINAL_BARRIER PASS; fixed final barrier with genuine API result, not a physical Root.Close fault")
		})
	}
	for _, writer := range []io.Writer{readyShortWriter{}, readyErrorWriter{}} {
		q, d, _, _ := readyCLIFixture(t)
		code, _ := readyCLIInvoke(t, nil, readyCLIEncode(t, q), readyCLIEncode(t, d), writer)
		if code == 0 {
			t.Fatal("ASSERT_READY_OUTPUT_FAILURE_NONZERO")
		}
		files, e := os.ReadDir(q.AccountingRoot)
		if e != nil || len(files) == 0 {
			t.Fatal("ASSERT_READY_OUTPUT_FAILURE_RETAINS_PUBLICATION")
		}
	}
	t.Log("ASSERT_READY_OUTPUT_FAILURE_NONZERO PASS; ASSERT_READY_OUTPUT_FAILURE_RETAINS_PUBLICATION PASS")
}
func TestReadyProvisioningDuplicateCLI(t *testing.T) {
	q, d, _, _ := readyCLIFixture(t)
	var first, second bytes.Buffer
	if code, stderr := readyCLIInvoke(t, nil, readyCLIEncode(t, q), readyCLIEncode(t, d), &first); code != 0 {
		t.Fatal(stderr)
	}
	if code, _ := readyCLIInvoke(t, nil, readyCLIEncode(t, q), readyCLIEncode(t, d), &second); code == 0 || second.Len() != 0 {
		t.Fatal("ASSERT_READY_DUPLICATE_CLI_REFUSED")
	}
	t.Log("ASSERT_READY_DUPLICATE_CLI_REFUSED PASS")
}

// AST binding witnesses complement (not replace) actual dispatcher tests and
// fixed final-barrier fault witnesses. No physical close injection is claimed.
func TestReadyProvisioningStaticBindings(t *testing.T) {
	parse := func(path, name string) *ast.FuncDecl {
		t.Helper()
		f, e := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if e != nil {
			t.Fatal(e)
		}
		for _, d := range f.Decls {
			if fn, ok := d.(*ast.FuncDecl); ok && fn.Name.Name == name {
				return fn
			}
		}
		t.Fatalf("missing function %s", name)
		return nil
	}
	fn := parse("private_ready_provisioning.go", "runPrivateReadyProvisioning")
	var closePos, writePos token.Pos
	ast.Inspect(fn, func(n ast.Node) bool {
		if a, ok := n.(*ast.AssignStmt); ok && len(a.Lhs) == 1 && len(a.Rhs) == 1 {
			if lhs, ok := a.Lhs[0].(*ast.Ident); ok && lhs.Name == "closeErr" {
				if c, ok := a.Rhs[0].(*ast.CallExpr); ok {
					if s, ok := c.Fun.(*ast.SelectorExpr); ok && s.Sel.Name == "Close" {
						if x, ok := s.X.(*ast.Ident); ok && x.Name == "root" {
							closePos = a.Pos()
						}
					}
				}
			}
		}
		if c, ok := n.(*ast.CallExpr); ok {
			if id, ok := c.Fun.(*ast.Ident); ok && id.Name == "privateReadyWriteSuccess" && len(c.Args) == 4 {
				if id, ok := c.Args[2].(*ast.Ident); ok && id.Name == "closeErr" {
					writePos = c.Pos()
				}
			}
		}
		return true
	})
	if closePos == 0 || writePos <= closePos {
		t.Fatal("ASSERT_READY_OWNED_CLOSE_BINDING")
	}
	render := func(n ast.Node) string {
		var b bytes.Buffer
		if err := format.Node(&b, token.NewFileSet(), n); err != nil {
			t.Fatal(err)
		}
		return b.String()
	}
	returns := func(body *ast.BlockStmt, target string, args []string) bool {
		if body == nil || len(body.List) != 1 {
			return false
		}
		r, ok := body.List[0].(*ast.ReturnStmt)
		if !ok || len(r.Results) != 1 {
			return false
		}
		c, ok := r.Results[0].(*ast.CallExpr)
		if !ok || render(c.Fun) != target || len(c.Args) != len(args) || c.Ellipsis.IsValid() {
			return false
		}
		for i, arg := range c.Args {
			if render(arg) != args[i] {
				return false
			}
		}
		return true
	}
	wrapper := parse("private_host_application.go", "runPrivateHostApplication")
	if !returns(wrapper.Body, "runPrivateHostApplicationWithInventoryInputsV3", []string{"args", "stdin", "stdout", "stderr", "nil"}) {
		t.Fatal("ASSERT_READY_WRAPPER_FORWARDING")
	}
	dispatch := parse("private_host_application.go", "runPrivateHostApplicationWithInventoryInputsV3")
	if dispatch.Body == nil || len(dispatch.Body.List) == 0 {
		t.Fatal("ASSERT_READY_EARLY_DISPATCH_BINDING")
	}
	early, ok := dispatch.Body.List[0].(*ast.IfStmt)
	if !ok || early.Init != nil || early.Else != nil || render(early.Cond) != `len(args) > 0 && args[0] == "initialize-ready"` || !returns(early.Body, "runPrivateReadyProvisioning", []string{"args", "stdin", "stdout", "stderr"}) {
		t.Fatal("ASSERT_READY_EARLY_DISPATCH_BINDING")
	}
	t.Log("ASSERT_READY_OWNED_CLOSE_BINDING PASS; ASSERT_READY_EARLY_DISPATCH_BINDING PASS; AST source witnesses only")
}

func TestReadyProvisioningFinalOriginalExpiry(t *testing.T) {
	q, d, jb, ab := readyCLIFixture(t)
	var fields struct {
		Identity string `json:"identity"`
		Expires  int64  `json:"expires_unix_nano"`
	}
	if e := json.Unmarshal(ab, &fields); e != nil {
		t.Fatal(e)
	}
	expiry := time.Now().Add(time.Second).UnixNano()
	unsigned := bytes.Replace(ab, []byte(`"identity":"`+fields.Identity+`"`), []byte(`"identity":""`), 1)
	unsigned = bytes.Replace(unsigned, []byte(strconv.FormatInt(fields.Expires, 10)), []byte(strconv.FormatInt(expiry, 10)), 1)
	id := fmt.Sprintf("sha256:%x", sha256.Sum256(append([]byte("inventory-successor/private/v2\x00lsp-trace.inventory-job-authorization.private.v2\x00"), unsigned...)))
	ab = bytes.Replace(unsigned, []byte(`"identity":""`), []byte(`"identity":"`+id+`"`), 1)
	d.Authorization = id
	root, e := publication.OpenRoot(q.AccountingRoot)
	if e != nil {
		t.Fatal(e)
	}
	result, e := inventorysuccessor.InitializePrivateReadyV2(context.Background(), root, jb, ab, inventorysuccessor.PrivateReadyDecisionAssertionV2{Job: d.Job, Authorization: d.Authorization, Reference: d.Reference}, inventorysuccessor.PrivateManagedInputVerificationV2{Records: 400, Bytes: 4 << 20, Work: 400})
	ce := root.Close()
	if e != nil || ce != nil {
		t.Fatalf("initialization before TEST expiry: %v %v", e, ce)
	}
	if e := privateReadyWriteSuccess(context.Background(), result, ce, readyDeadlineWriter{expiry: expiry}); e == nil {
		t.Fatal("ASSERT_READY_AFTER_OUTPUT_ORIGINAL_EXPIRY")
	}
	t.Log("ASSERT_READY_AFTER_OUTPUT_ORIGINAL_EXPIRY PASS")
	var out bytes.Buffer
	if e := privateReadyWriteSuccess(context.Background(), result, ce, &out); e == nil || out.Len() != 0 {
		t.Fatal("ASSERT_READY_FINAL_ORIGINAL_EXPIRY")
	}
	t.Log("ASSERT_READY_FINAL_ORIGINAL_EXPIRY PASS")
}

func TestReadyProvisioningInvalidDecisionDescriptor(t *testing.T) {
	q, _, _, _ := readyCLIFixture(t)
	var out, stderr bytes.Buffer
	if code := runPrivateHostApplication([]string{"initialize-ready", "--decision-fd", "2147483647"}, bytes.NewReader(readyCLIEncode(t, q)), &out, &stderr); code == 0 || out.Len() != 0 {
		t.Fatal("ASSERT_READY_MISSING_DESCRIPTOR")
	}
	files, e := os.ReadDir(q.AccountingRoot)
	if e != nil || len(files) != 0 {
		t.Fatal("ASSERT_READY_MISSING_DESCRIPTOR_BEFORE_WRITES")
	}
	t.Log("ASSERT_READY_MISSING_DESCRIPTOR PASS")
}
