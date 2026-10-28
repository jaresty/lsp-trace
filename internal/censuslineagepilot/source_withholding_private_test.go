//go:build darwin || linux

package censuslineagepilot

import (
	"bytes"
	"context"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"lsp-trace/internal/acquisition"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/lsp"
	"lsp-trace/sessionruntime"
)

// Disposable fake-child fixtures only. No real retained roots, server, replay,
// or production acquisition path is activated by this source-profile test.
func TestSourceWithholdingPrivateBoundCapability(t *testing.T) {
	restart, m, id, _ := adapterFixture(t)
	var bound *graphprovenance.BoundSourcePrivateV1
	var result acquisition.Result
	err := m.WithReadySessionLease(context.Background(), sessionruntime.SessionLeaseRequest{SessionID: id.SessionID, Generation: id.Generation, Workspace: restart.Scope().Selection().Workspace}, func(ctx context.Context) error {
		a, _, err := newScopeRuntimeAdapter(ctx, restart, m, id)
		if err != nil {
			return err
		}
		bound, err = graphprovenance.BindSourcePrivateV1(ctx, m, a.guard, id.SessionID, id.Generation, a.workspace)
		if err != nil {
			return err
		}
		if _, err = graphprovenance.BindSourcePrivateV1(ctx, m, a.guard, id.SessionID, id.Generation, filepath.Dir(a.workspace)); err == nil {
			t.Fatal("ASSERT_SOURCE_BOUND_WORKSPACE")
		}
		if _, err = graphprovenance.BindSourcePrivateV1(ctx, m, &sessionruntime.RetainedScopeGuardPrivate{}, id.SessionID, id.Generation, a.workspace); err == nil {
			t.Fatal("ASSERT_SOURCE_FORGED_GUARD")
		}
		// Mutate only detached guard output. A future capture must still compare the
		// original Manager-bound expected bytes, not this caller-owned copy.
		members := a.guard.MembersPrivate()
		members[0].Expected[0] ^= 1
		members[0].URI = "file:///forged.go"
		outsidePath := filepath.Join(a.workspace, "policy-withheld.go")
		if err = os.WriteFile(outsidePath, []byte("must remain unread"), 0600); err != nil {
			return err
		}
		outsideURI := (&url.URL{Scheme: "file", Path: outsidePath}).String()
		request := reviewNativeRequest(a)
		client := acquisition.NewWireClient(func(_ context.Context, wire acquisition.WireRequest) (json.RawMessage, error) {
			if wire.Method == "textDocument/prepareCallHierarchy" {
				return json.Marshal([]lsp.CallHierarchyItem{scopeAdapterItem(a.files[0].URI)})
			}
			if wire.Method == "callHierarchy/incomingCalls" {
				return json.Marshal([]lsp.CallHierarchyIncomingCall{{From: scopeAdapterItem(outsideURI), FromRanges: []lsp.Range{}}})
			}
			return json.RawMessage(`[]`), nil
		})
		result, err = acquisition.Acquire(ctx, client, request)
		if err != nil {
			return err
		}
		raw, err := graphprovenance.CaptureSourceWithholdingPrivateV1(ctx, result, bound)
		if err != nil {
			return err
		}
		e, err := graphprovenance.ParseSourceWithholdingPrivateV1(raw)
		if err != nil {
			return err
		}
		if e.WithheldCount != 1 {
			t.Fatal("ASSERT_SOURCE_BOUND_WITHHOLDING", e.WithheldCount)
		}
		if e.Evidence.CaptureBudget.Attempts != 1 {
			t.Fatal("ASSERT_SOURCE_BOUND_ATTEMPTS", e.Evidence.CaptureBudget)
		}
		t.Run("copied-capability", func(t *testing.T) {
			copied := *bound
			captured, err := graphprovenance.CaptureSourceWithholdingPrivateV1(ctx, result, &copied)
			if err == nil || captured != nil {
				t.Fatal("ASSERT_SOURCE_COPIED_CAPABILITY_REFUSED: copied real capability admitted during valid lease")
			}
			if err.Error() != "private source bound capability required" {
				t.Fatal("ASSERT_SOURCE_COPY_REFUSED_BEFORE_CAPTURE: wrong refusal boundary", err)
			}
		})
		// Pointer aliases designate the same minted object and remain valid.
		alias := bound
		if captured, err := graphprovenance.CaptureSourceWithholdingPrivateV1(ctx, result, alias); err != nil || !bytes.Equal(raw, captured) {
			t.Fatal("ASSERT_SOURCE_ORIGINAL_POINTER_REMAINS_VALID", err)
		}
		// In-policy filesystem drift cannot be converted to a normal source receipt.
		f := restart.Scope().Files()[0]
		if err = os.WriteFile(filepath.Join(a.workspace, f.Path), []byte("drift"), 0600); err != nil {
			return err
		}
		if _, err = graphprovenance.CaptureSourceWithholdingPrivateV1(ctx, result, bound); err == nil {
			t.Fatal("ASSERT_SOURCE_BOUND_EXPECTED_BYTES")
		}
		if err = os.WriteFile(filepath.Join(a.workspace, f.Path), f.Bytes, 0600); err != nil {
			return err
		}
		again, err := graphprovenance.CaptureSourceWithholdingPrivateV1(ctx, result, bound)
		if err != nil {
			return err
		}
		if !bytes.Equal(raw, again) {
			t.Fatal("ASSERT_SOURCE_BOUND_DETACHED_EXPECTATIONS")
		}
		// Replace after the actual guard/capability bind, including equal content.
		// FD-level zero-access observations live in graphprovenance reader tests.
		for _, directory := range []bool{false, true} {
			for _, equal := range []bool{false, true} {
				name := map[bool]string{false: "file", true: "directory"}[directory] + map[bool]string{false: "-different", true: "-equal"}[equal]
				t.Run("after-bind-"+name, func(t *testing.T) {
					forbiddenDir := filepath.Join(a.workspace, "source-alias-forbidden-"+name)
					if err := os.Mkdir(forbiddenDir, 0755); err != nil {
						t.Fatal(err)
					}
					forbidden := filepath.Join(forbiddenDir, filepath.Base(f.Path))
					content := []byte("different outside-six bytes")
					if equal {
						content = f.Bytes
					}
					if err := os.WriteFile(forbidden, content, 0644); err != nil {
						t.Fatal(err)
					}
					allowed := filepath.Join(a.workspace, f.Path)
					target := forbidden
					if directory {
						allowed = filepath.Dir(allowed)
						target = forbiddenDir
					}
					if allowed == a.workspace {
						t.Fatal("fixture must contain an intermediate directory")
					}
					backup := allowed + "-private-saved"
					if err := os.Rename(allowed, backup); err != nil {
						t.Fatal(err)
					}
					if err := os.Symlink(target, allowed); err != nil {
						os.Rename(backup, allowed)
						t.Fatal(err)
					}
					t.Cleanup(func() {
						os.Remove(allowed)
						if err := os.Rename(backup, allowed); err != nil {
							t.Error(err)
						}
					})
					captured, err := graphprovenance.CaptureSourceWithholdingPrivateV1(ctx, result, bound)
					if err != nil {
						t.Fatal(err)
					}
					parsed, err := graphprovenance.ParseSourceWithholdingPrivateV1(captured)
					if err != nil {
						t.Fatal(err)
					}
					seen := false
					for _, row := range parsed.Evidence.Captures {
						if row.URI == a.files[0].URI {
							seen = true
							if row.Status == "READABLE" || row.Content != nil {
								t.Fatal("ASSERT_SOURCE_BOUND_ALIAS_REFUSED")
							}
						}
					}
					if !seen {
						t.Fatal("ASSERT_SOURCE_BOUND_ALIAS_ROW_PRESERVED")
					}
					b, err := os.ReadFile(forbidden)
					if err != nil || !bytes.Equal(b, content) {
						t.Fatal("forbidden target mutated", err)
					}
				})
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = graphprovenance.CaptureSourceWithholdingPrivateV1(context.Background(), result, bound); err == nil {
		t.Fatal("ASSERT_SOURCE_EXPIRED_LEASE")
	}
}
