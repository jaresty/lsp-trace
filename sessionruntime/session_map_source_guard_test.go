package sessionruntime

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// This deliberately finite source guard is not a control-flow proof or runtime
// ownership qualification. Enumerate every direct manager-map mutation, then
// bind each reviewed site to its existing enclosing ownership lifecycle.
func sessionMapSourceGuard(source []byte) error {
	fs := token.NewFileSet()
	file, err := parser.ParseFile(fs, "sessionruntime.go", source, 0)
	if err != nil {
		return err
	}
	// Remove comments as a source of false protection matches; normalize formatting.
	var buf bytes.Buffer
	if err := format.Node(&buf, fs, file); err != nil {
		return err
	}
	source = buf.Bytes()
	fs = token.NewFileSet()
	file, err = parser.ParseFile(fs, "sessionruntime.go", source, 0)
	if err != nil {
		return err
	}
	compact := func(s string) string { return strings.Join(strings.Fields(s), "") }
	text := func(n ast.Node) string {
		return compact(string(source[fs.Position(n.Pos()).Offset:fs.Position(n.End()).Offset]))
	}
	isMap := func(e ast.Expr) bool {
		s, ok := e.(*ast.SelectorExpr)
		if !ok || s.Sel.Name != "sessions" {
			return false
		}
		id, ok := s.X.(*ast.Ident)
		return ok && id.Name == "m"
	}
	type protection struct{ ancestors, before, after []string }
	reviewed := map[string]protection{
		"start/install/id/1": {before: []string{
			"m.mu.Lock()", "if current := m.sessions[id]; current != nil", "m.mu.Lock()", "defer m.mu.Unlock()",
			"if m.sessions[id] != nil || len(m.sessions) >= m.limits.MaxSessions",
			"if admission := m.algebra.Admit(id, 0); admission.Kind != session.AdmissionFree",
			"return m.failTransientIdentityStartLocked(id, r, session.ResourceExhausted, observed)",
			"return m.failTransientIdentityStartLocked(id, r, identityFailure, observed)",
			"r.transientIdentity = transientIdentity", "r.diagnosticGeneration = m.newDiagnosticGeneration(attemptID, id, 1)",
		}},
		"failTransientIdentityStartLocked/install/id/1": {ancestors: []string{"private"}, before: []string{
			"intent := m.algebra.Lifecycle", "privateChild, private := r.process.(*privateProviderWindowChild)",
			"r.record.State, r.protocolOwned = session.Poisoned, true",
		}, after: []string{"privateChild.window.ownedRecord = r", "m.mu.Unlock()", "teardown := r.process.Teardown(context.Background())", "resources := r.process.Close()"}},
		"failTransientIdentityStartLocked/remove/id/1": {ancestors: []string{
			"cleanup.Failure == \"\" && death && resources.Kind == managedprocess.ResourcesClosed && m.algebra.ReleaseStopped(id, 1)", "private",
		}, before: []string{
			"if private { m.mu.Lock()", "if m.sessions[id] != r", "return result",
			"death := teardown.Death.Reap.Kind == managedprocess.ReapComplete", "joined := !private || privateChild.cleanupJoined()",
			"cleanup = m.algebra.CompleteLifecycleObserved", "NoContainedSurvivors: death && joined", "StderrDrainComplete: joined",
		}},
		"failTransientIdentityStartLocked/install/id/2": {before: []string{
			"if private { m.mu.Lock()", "if m.sessions[id] != r", "result.transientFailure.Cleanup = session.LifecycleConflict", "return result",
			"if cleanup.Failure == \"\" && death && resources.Kind == managedprocess.ResourcesClosed && m.algebra.ReleaseStopped(id, 1)",
			"result.transientFailure.Cleanup = cleanup.Failure", "r.record.State = session.Poisoned",
		}},
		"runLifecycle/remove/operation.SessionID/1": {ancestors: []string{"!operation.Restart"}, before: []string{
			"m.mu.Lock()", "completed := m.algebra.CompleteLifecycleObserved(operation.SessionID, operation.ID, observed)",
			"if completed.Failure != \"\" || !death || resources.Kind != managedprocess.ResourcesClosed || !privateJoined",
			"if !m.algebra.ReleaseStopped(operation.SessionID, operation.Generation)",
			"join := m.retirePrivateB4StoppedLocked(operation.SessionID, operation.Generation, r)",
			"m.mu.Unlock()", "join.wait()", "genericOwner.waitReleased()", "m.mu.Lock()",
		}, after: []string{"privateChild.window.lifecycleJoined = true", "m.finishOperation(operation)", "m.mu.Unlock()"}},
	}
	seen := map[string]int{}
	found := map[string]bool{}
	var failure error
	admissionFound := false
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		if fn.Name.Name == "terminateWithShutdownResponsePrivateV2" {
			if admissionFound {
				return fmt.Errorf("SG2 lifecycle admission duplicate function")
			}
			admissionFound = true
			// These are deliberately exact, finite reviewed statement shapes, not
			// equivalent control-flow patterns. Only direct statements of this
			// function can supply a condition AND its rejecting body. A nested
			// decoy, return elsewhere, or duplicate candidate cannot satisfy it.
			for _, spec := range []struct{ name, condition, body string }{
				{"conflict", "m.closed || r.leaseCount != 0 || (r.protocolOwned && !r.lifecycleOwned)", "{ return session.LifecycleResult{State: r.record.State, Generation: r.record.Generation, Failure: session.LifecycleConflict} }"},
				{"capacity", "m.workers >= m.limits.MaxChildren || !m.canReserveOperation()", "{ return session.LifecycleResult{Failure: session.ResourceExhausted} }"},
				{"reservation", "!m.reserveOperation(intent.IntentID)", "{ m.workers--\n return session.LifecycleResult{Failure: session.ResourceExhausted} }"},
			} {
				var candidate *ast.IfStmt
				count := 0
				for _, stmt := range fn.Body.List {
					branch, ok := stmt.(*ast.IfStmt)
					if ok && text(branch.Cond) == compact(spec.condition) {
						count++
						candidate = branch
					}
				}
				if count != 1 {
					return fmt.Errorf("SG2 lifecycle admission %s direct branch count %d", spec.name, count)
				}
				if candidate.Init != nil || candidate.Else != nil || text(candidate.Body) != compact(spec.body) {
					return fmt.Errorf("SG2 lifecycle admission %s body", spec.name)
				}
			}
			body := text(fn.Body)
			at := 0
			for _, guard := range []string{"m.mu.Lock()", "if m.closed || r.leaseCount != 0 || (r.protocolOwned && !r.lifecycleOwned)", "if m.workers >= m.limits.MaxChildren || !m.canReserveOperation()", "intent := m.algebra.Lifecycle", "if !m.reserveOperation(intent.IntentID)", "r.protocolOwned, r.lifecycleOwned = true, true", "go m.runLifecycle(snapshot, r.process, r.pending, r.spec, r.retired)"} {
				i := strings.Index(body[at:], compact(guard))
				if i < 0 {
					return fmt.Errorf("SG2 lifecycle admission missing ordered protection %q", guard)
				}
				at += i + len(compact(guard))
			}
		}
		var stack []ast.Node
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			if n == nil {
				stack = stack[:len(stack)-1]
				return true
			}
			stack = append(stack, n)
			kind, key := "", ""
			switch v := n.(type) {
			case *ast.AssignStmt:
				for _, lhs := range v.Lhs {
					if ix, ok := lhs.(*ast.IndexExpr); ok && isMap(ix.X) {
						kind, key = "install", text(ix.Index)
						if len(v.Lhs) != 1 || len(v.Rhs) != 1 || v.Tok != token.ASSIGN || text(v.Rhs[0]) != "r" {
							kind = "unreviewed-assignment"
						}
					}
					if isMap(lhs) {
						kind, key = "replace", "map"
					}
				}
			case *ast.IncDecStmt:
				if ix, ok := v.X.(*ast.IndexExpr); ok && isMap(ix.X) {
					kind, key = "increment", text(ix.Index)
				}
			case *ast.CallExpr:
				id, ok := v.Fun.(*ast.Ident)
				if ok && len(v.Args) > 0 && isMap(v.Args[0]) {
					if id.Name == "delete" && len(v.Args) == 2 {
						kind, key = "remove", text(v.Args[1])
					} else if id.Name == "clear" {
						kind, key = "clear", "map"
					}
				}
			}
			if kind == "" {
				return true
			}
			base := fn.Name.Name + "/" + kind + "/" + key
			seen[base]++
			name := fmt.Sprintf("%s/%d", base, seen[base])
			spec, ok := reviewed[name]
			if !ok {
				failure = fmt.Errorf("SG1 unknown mutation %s", name)
				return true
			}
			found[name] = true
			before := compact(string(source[fs.Position(fn.Body.Pos()).Offset:fs.Position(n.Pos()).Offset]))
			after := compact(string(source[fs.Position(n.End()).Offset:fs.Position(fn.Body.End()).Offset]))
			for _, section := range []struct {
				value    string
				required []string
			}{{before, spec.before}, {after, spec.after}} {
				at := 0
				for _, guard := range section.required {
					i := strings.Index(section.value[at:], compact(guard))
					if i < 0 {
						failure = fmt.Errorf("SG2 %s missing ordered protection %q", name, guard)
						break
					}
					at += i + len(compact(guard))
				}
			}
			for _, condition := range spec.ancestors {
				matched := false
				for _, ancestor := range stack[:len(stack)-1] {
					if branch, ok := ancestor.(*ast.IfStmt); ok && text(branch.Cond) == compact(condition) {
						matched = true
					}
				}
				if !matched {
					failure = fmt.Errorf("SG2 %s missing enclosing condition %q", name, condition)
				}
			}
			return true
		})
	}
	if failure != nil {
		return failure
	}
	if !admissionFound {
		return fmt.Errorf("SG2 lifecycle admission missing")
	}
	for name := range reviewed {
		if !found[name] {
			return fmt.Errorf("SG1 missing reviewed mutation %s", name)
		}
	}
	return nil
}

func sessionMapGuardSource(t *testing.T) []byte {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("source location unavailable")
	}
	b, err := os.ReadFile(filepath.Join(filepath.Dir(file), "sessionruntime.go"))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// Mutate only parseable in-memory source, locating each reviewed direct branch
// independently of the guard's implementation. Never edit the production file.
func TestSessionLeaseSourceGuardAdmissionBodies(t *testing.T) {
	original := sessionMapGuardSource(t)
	branches := []struct{ name, condition string }{
		{"conflict", "m.closed || r.leaseCount != 0 || (r.protocolOwned && !r.lifecycleOwned)"},
		{"capacity", "m.workers >= m.limits.MaxChildren || !m.canReserveOperation()"},
		{"reservation", "!m.reserveOperation(intent.IntentID)"},
	}
	for _, b := range branches {
		fs := token.NewFileSet()
		file, err := parser.ParseFile(fs, "sessionruntime.go", original, 0)
		if err != nil {
			t.Fatal(err)
		}
		var branch *ast.IfStmt
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Name.Name != "terminateWithShutdownResponsePrivateV2" {
				continue
			}
			for _, stmt := range fn.Body.List {
				v, ok := stmt.(*ast.IfStmt)
				if !ok {
					continue
				}
				var out bytes.Buffer
				if err := format.Node(&out, fs, v.Cond); err != nil {
					t.Fatal(err)
				}
				if out.String() == b.condition {
					if branch != nil {
						t.Fatal("duplicate control anchor")
					}
					branch = v
				}
			}
		}
		if branch == nil {
			t.Fatal("missing control anchor")
		}
		start, end := fs.Position(branch.Body.Pos()).Offset, fs.Position(branch.Body.End()).Offset
		body := string(original[start:end])
		wrong := "session.LifecycleConflict"
		if b.name == "conflict" {
			wrong = "session.ResourceExhausted"
		}
		failure := "session.ResourceExhausted"
		if b.name == "conflict" {
			failure = "session.LifecycleConflict"
		}
		cases := []struct{ name, body, diagnostic string }{
			{"empty-body", "{}", "body"},
			{"wrong-failure", strings.Replace(body, failure, wrong, 1), "body"},
		}
		if b.name == "reservation" {
			cases = append(cases, struct{ name, body, diagnostic string }{"missing-rollback", strings.Replace(body, "m.workers--", "", 1), "body"})
		}
		for _, c := range cases {
			t.Run(b.name+"/"+c.name, func(t *testing.T) {
				mutated := append([]byte{}, original[:start]...)
				mutated = append(mutated, []byte(c.body)...)
				mutated = append(mutated, original[end:]...)
				if _, err := parser.ParseFile(token.NewFileSet(), "mutant.go", mutated, 0); err != nil {
					t.Fatalf("invalid control: %v", err)
				}
				// Restoration is evaluated even when the mutant assertion fails.
				defer func() {
					if err := sessionMapSourceGuard(original); err != nil {
						t.Fatalf("ASSERT_ADMISSION_%s_RESTORED: %v", b.name, err)
					}
					t.Logf("ASSERT_ADMISSION_%s_RESTORED: PASS", b.name)
				}()
				want := "SG2 lifecycle admission " + b.name + " " + c.diagnostic
				err := sessionMapSourceGuard(mutated)
				if err == nil || err.Error() != want {
					t.Fatalf("ASSERT_ADMISSION_%s_%s: want %q, got %v", b.name, c.name, want, err)
				}
				t.Logf("ASSERT_ADMISSION_%s_%s: PASS; %s", b.name, c.name, err)
			})
		}
	}
}

func TestSessionLeaseSourceGuardNegativeControls(t *testing.T) {
	original := sessionMapGuardSource(t)
	if err := sessionMapSourceGuard(original); err != nil {
		t.Fatalf("original guard: %v", err)
	}
	cases := []struct{ name, old, new string }{
		{"lifecycle-admission", "if m.closed || r.leaseCount != 0 || (r.protocolOwned && !r.lifecycleOwned)", "if m.closed"},
		{"normal-identity", "r.transientIdentity = transientIdentity", "r.transientIdentity = \"\""},
		{"private-reservation", "r.record.State, r.protocolOwned = session.Poisoned, true", "r.record.State, r.protocolOwned = session.Poisoned, false"},
		{"exact-record", "if m.sessions[id] != r {", "if false {"},
		{"cleanup-release", "&& m.algebra.ReleaseStopped(id, 1)", ""},
		{"lifecycle-join", "genericOwner.waitReleased()", "_ = genericOwner"},
		{"extra-install", "m.sessions[id] = r", "m.sessions[id] = r\n m.sessions[\"unreviewed\"] = r"},
		{"extra-delete", "delete(m.sessions, id)", "delete(m.sessions, id)\n delete(m.sessions, \"unreviewed\")"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if !bytes.Contains(original, []byte(c.old)) {
				t.Fatal("negative-control anchor absent")
			}
			mutated := bytes.Replace(original, []byte(c.old), []byte(c.new), 1)
			err := sessionMapSourceGuard(mutated)
			if err == nil {
				t.Fatal("ASSERT_SESSION_MAP_NEGATIVE_CONTROL")
			}
			if !strings.Contains(err.Error(), "SG") {
				t.Fatalf("not assertion-specific: %v", err)
			}
			t.Logf("ASSERT_SESSION_MAP_NEGATIVE_CONTROL: PASS; %s refused: %v", c.name, err)
			if err := sessionMapSourceGuard(original); err != nil {
				t.Fatalf("restored source: %v", err)
			}
		})
	}
}
