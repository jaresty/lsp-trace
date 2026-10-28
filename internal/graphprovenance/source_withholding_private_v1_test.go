//go:build darwin || linux

package graphprovenance

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lsp-trace/internal/acquisition"
	"lsp-trace/sessionruntime"
)

func withholdingFixturePrivateV1(t *testing.T) (acquisition.Result, SourcePolicyPrivateV1, map[string][]byte, string) {
	t.Helper()
	canonicalTemp, err := filepath.EvalSymlinks(os.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMPDIR", canonicalTemp)
	var root string
	r, root := coordinatorV2Fixture(t, func(req *acquisition.Request) {
		u, _ := url.Parse(req.Root.Locator.URI)
		root = filepath.Dir(u.Path)
		for _, name := range []string{"b.go", "c.go", "d.go", "e.go", "f.go", "z.go"} {
			target := req.Root
			target.ID = "extra" + name
			target.Locator.URI = (&url.URL{Scheme: "file", Path: filepath.Join(root, name)}).String()
			req.RequiredTargets = append(req.RequiredTargets, target)
		}
		target := req.Root
		target.ID = "outside"
		target.Locator.URI = "file:///outside-frozen-private.go"
		req.RequiredTargets = append(req.RequiredTargets, target)
	})
	p := SourcePolicyPrivateV1{SchemaVersion: VersionSourcePolicyPrivateV1, WorkspaceURI: (&url.URL{Scheme: "file", Path: root}).String(), SessionID: r.Request.Context.SessionID, Generation: r.Request.Context.Generation, Members: []SourceMemberPrivateV1{}}
	expected := map[string][]byte{}
	for i, name := range []string{"a.go", "b.go", "c.go", "d.go", "e.go", "f.go"} {
		uri := (&url.URL{Scheme: "file", Path: filepath.Join(root, name)}).String()
		b := []byte("package p\n")
		if err := os.WriteFile(filepath.Join(root, name), b, 0600); err != nil {
			t.Fatal(err)
		}
		role := "CONTEXT_ONLY"
		if i < 4 {
			role = "NOMINATION"
		}
		p.Members = append(p.Members, SourceMemberPrivateV1{uri, sourceHashPrivateV1(b), role})
		expected[uri] = b
	}
	if err := os.WriteFile(filepath.Join(root, "z.go"), []byte("must not read"), 0600); err != nil {
		t.Fatal(err)
	}
	return r, p, expected, root
}
func TestSourceWithholdingPrivateV1Capture(t *testing.T) {
	r, p, expected, root := withholdingFixturePrivateV1(t)
	reads := map[string]int{}
	raw, err := captureSourceWithholdingPrivateV1(context.Background(), r, p, expected, func(ctx context.Context, root *sourceRootPrivateV1, name string, n int64) ([]byte, error) {
		reads[name]++
		return readSourcePrivateV1(ctx, root, name, n)
	})
	if err != nil {
		t.Fatal(err)
	}
	e, err := ParseSourceWithholdingPrivateV1(raw)
	if err != nil {
		t.Fatal(err)
	}
	if reads["z.go"] != 0 || len(reads) != 6 || e.WithheldCount != 1 || len(e.Evidence.Captures) != 9 || e.Evidence.CaptureBudget.Attempts != 6 || e.Evidence.CaptureBudget.ChargedBytes != 60 {
		t.Fatalf("ASSERT_PRIVATE_WITHHOLD_ACCOUNTING: reads=%v budget=%+v rows=%d withheld=%d", reads, e.Evidence.CaptureBudget, len(e.Evidence.Captures), e.WithheldCount)
	}
	for _, row := range e.Evidence.Captures {
		t.Logf("source row %s: %s", row.URI, row.Status)
		if row.URI == p.WorkspaceURI+"/z.go" && (row.Status != PolicyWithheldPrivateV1 || row.Content != nil || row.CanonicalReceipt != nil) {
			t.Fatal("ASSERT_PRIVATE_NO_SOURCE_CLAIM")
		}
		if row.URI == "file:///outside-frozen-private.go" && row.Status != "OUTSIDE_SCOPE" {
			t.Fatal("ASSERT_PRIVATE_WORKSPACE_DISTINCTION")
		}
	}
	if !e.Partial || e.Accepted || e.Authority != 0 || e.ReplayAdmitted || e.SourceCompleteness != "UNKNOWN" {
		t.Fatal("ASSERT_PRIVATE_CEILINGS")
	}
	if _, err = ValidateFor(raw, Family, "v2"); err == nil {
		t.Fatal("ASSERT_OLD_V2_REJECTS_PRIVATE")
	}
	if err = ValidateV2(e.Evidence); err == nil {
		t.Fatal("ASSERT_OLD_TYPED_V2_REJECTS_PRIVATE")
	}
	if err = os.RemoveAll(root); err != nil {
		t.Fatal(err)
	}
	if _, err = ParseSourceWithholdingPrivateV1(raw); err != nil {
		t.Fatal("offline:", err)
	}
	e.Policy.Members[0].URI = "changed"
	e.Evidence.Captures[1].Content[0] = 'X'
	again, err := ParseSourceWithholdingPrivateV1(raw)
	if err != nil || again.Policy.Members[0].URI == "changed" || bytes.HasPrefix(again.Evidence.Captures[1].Content, []byte("X")) {
		t.Fatal("ASSERT_PRIVATE_DETACHED_OUTPUT")
	}
	t.Log("ASSERT_PRIVATE_WITHHOLD_ACCOUNTING: PASS")
}
func TestSourceWithholdingPrivateV1ClosedContract(t *testing.T) {
	r, p, expected, _ := withholdingFixturePrivateV1(t)
	raw, err := captureSourceWithholdingPrivateV1(context.Background(), r, p, expected, readSourcePrivateV1)
	if err != nil {
		t.Fatal(err)
	}
	mutations := map[string]func(*SourceWithholdingPrivateV1){
		"missing-policy":   func(e *SourceWithholdingPrivateV1) { e.Policy = SourcePolicyPrivateV1{} },
		"scope-version":    func(e *SourceWithholdingPrivateV1) { e.Policy.SchemaVersion = "v0" },
		"noncanonical-uri": func(e *SourceWithholdingPrivateV1) { e.Policy.Members[0].URI += "?x=1" },
		"reorder": func(e *SourceWithholdingPrivateV1) {
			e.Policy.Members[0], e.Policy.Members[1] = e.Policy.Members[1], e.Policy.Members[0]
		},
		"duplicate":  func(e *SourceWithholdingPrivateV1) { e.Policy.Members[1] = e.Policy.Members[0] },
		"mixed-hash": func(e *SourceWithholdingPrivateV1) { e.Policy.Members[0].SHA256 = sourceHashPrivateV1([]byte("other")) },
		"hash-case": func(e *SourceWithholdingPrivateV1) {
			e.Policy.Members[0].SHA256 = strings.ToUpper(e.Policy.Members[0].SHA256)
		},
		"outside-workspace-withheld": func(e *SourceWithholdingPrivateV1) {
			for i, r := range e.Evidence.Captures {
				if r.Status == "OUTSIDE_SCOPE" {
					e.Evidence.Captures[i] = withheldPrivateV1(e.Evidence.WorkspaceURI, r.URI)
				}
			}
		},
		"withheld-source-hash-claim": func(e *SourceWithholdingPrivateV1) {
			for i := range e.Evidence.Captures {
				if e.Evidence.Captures[i].Status == PolicyWithheldPrivateV1 {
					e.Evidence.Captures[i].CanonicalReceipt = []byte(`{"sha256":"invented"}`)
				}
			}
		},
		"policy-context": func(e *SourceWithholdingPrivateV1) { e.Policy.Generation++ },
		"missing-row": func(e *SourceWithholdingPrivateV1) {
			e.Evidence.Captures = e.Evidence.Captures[:len(e.Evidence.Captures)-1]
		},
		"charge": func(e *SourceWithholdingPrivateV1) { e.Evidence.CaptureBudget.Attempts++ },
		"count":  func(e *SourceWithholdingPrivateV1) { e.WithheldCount++ },
		"content-for-withheld": func(e *SourceWithholdingPrivateV1) {
			for i := range e.Evidence.Captures {
				if e.Evidence.Captures[i].Status == PolicyWithheldPrivateV1 {
					e.Evidence.Captures[i].Content = []byte{}
				}
			}
		},
		"unmotivated-status": func(e *SourceWithholdingPrivateV1) {
			for i := range e.Evidence.Captures {
				r := &e.Evidence.Captures[i]
				if r.Status == "READABLE" {
					*r = receiptV2(r.URI, r.Path, r.Classification, PolicyWithheldPrivateV1, nil, nil)
					break
				}
			}
		},
		"accepted":        func(e *SourceWithholdingPrivateV1) { e.Accepted = true },
		"replay":          func(e *SourceWithholdingPrivateV1) { e.ReplayAdmitted = true },
		"source-complete": func(e *SourceWithholdingPrivateV1) { e.SourceCompleteness = "COMPLETE" },
		"authority":       func(e *SourceWithholdingPrivateV1) { e.Authority = 1 },
		"partial":         func(e *SourceWithholdingPrivateV1) { e.Partial = false },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			e, err := ParseSourceWithholdingPrivateV1(raw)
			if err != nil {
				t.Fatal(err)
			}
			mutate(&e)
			bad, err := json.Marshal(e)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = ParseSourceWithholdingPrivateV1(bad); err == nil {
				t.Fatal("ASSERT_PRIVATE_REJECT_CORRUPTION:", name)
			}
		})
	}
	for name, bad := range map[string][]byte{"duplicate-key": bytes.Replace(raw, []byte(`"accepted":false`), []byte(`"accepted":false,"accepted":false`), 1), "unknown": bytes.Replace(raw, []byte(`"accepted":false`), []byte(`"accepted":false,"unknown":0`), 1), "missing": bytes.Replace(raw, []byte(`"accepted":false,`), nil, 1), "depth": []byte(strings.Repeat("[", MaxDepthV2+1) + strings.Repeat("]", MaxDepthV2+1)), "bytes": bytes.Repeat([]byte(" "), MaxSourceWithholdingPrivateV1+1)} {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseSourceWithholdingPrivateV1(bad); err == nil {
				t.Fatal("ASSERT_PRIVATE_PARSE_BOUND:", name)
			}
		})
	}
}
func TestSourceWithholdingPrivateV1ActualReadAccounting(t *testing.T) {
	r, p, expected, _ := withholdingFixturePrivateV1(t)
	for _, cancelled := range []bool{false, true} {
		t.Run(map[bool]string{false: "failed-attempt-budget", true: "cancel-order"}[cancelled], func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			calls := 0
			if cancelled {
				cancel()
			}
			raw, err := captureSourceWithholdingPrivateV1(ctx, r, p, expected, func(context.Context, *sourceRootPrivateV1, string, int64) ([]byte, error) {
				calls++
				return nil, errors.New("denied")
			})
			if err != nil {
				t.Fatal(err)
			}
			e, err := ParseSourceWithholdingPrivateV1(raw)
			if err != nil {
				t.Fatal(err)
			}
			wantCalls, wantBytes := 4, MaxTotalBytes
			if cancelled {
				wantCalls, wantBytes = 0, 0
			}
			if calls != wantCalls || e.Evidence.CaptureBudget.Attempts != wantCalls || e.Evidence.CaptureBudget.ChargedBytes != wantBytes || e.WithheldCount != 1 {
				t.Fatalf("ASSERT_PRIVATE_ACTUAL_READ_CHARGE: %d %+v", calls, e.Evidence.CaptureBudget)
			}
			if !cancelled {
				count := 0
				for _, row := range e.Evidence.Captures {
					if row.Status == "BUDGET_EXCEEDED" {
						count++
					}
				}
				if count != 2 {
					t.Fatal("ASSERT_PRIVATE_BYTE_BUDGET_ORDER")
				}
			}
		})
	}
}
func TestSourceWithholdingPrivateV1PreallocationBounds(t *testing.T) {
	for _, raw := range [][]byte{
		[]byte(`{"source_policy":{"members":[null,null,null,null,null,null,null]}}`),
		[]byte(`{"evidence":{"captures":[` + strings.Repeat(`null,`, MaxRecordsV2) + `null]}}`),
	} {
		if _, err := ParseSourceWithholdingPrivateV1(raw); err == nil || err.Error() != "private source array allocation bound" {
			t.Fatalf("ASSERT_PRIVATE_PREALLOCATION_BOUND: %v", err)
		}
	}
}

// Synthetic identity fixture only: this does not mint a Manager guard/lease or
// authorize any source read. Successful original capture uses the real Manager
// fixture in censuslineagepilot. Keep self minting explicit and nonproduction.
func syntheticBoundSourceIdentityPrivateV1() *BoundSourcePrivateV1 {
	b := &BoundSourcePrivateV1{manager: &sessionruntime.Manager{}, guard: &sessionruntime.RetainedScopeGuardPrivate{}, policy: SourcePolicyPrivateV1{SessionID: "identity-fixture", Generation: 1}}
	b.self = b
	return b
}

func TestSourceWithholdingPrivateV1ExactCapabilityIdentity(t *testing.T) {
	original := syntheticBoundSourceIdentityPrivateV1()
	copied := *original
	forged := &BoundSourcePrivateV1{manager: original.manager, guard: original.guard, policy: original.policy}
	for name, b := range map[string]*BoundSourcePrivateV1{"nil": nil, "zero": {}, "forged": forged, "copied": &copied} {
		t.Run(name, func(t *testing.T) {
			reads := 0
			raw, err := captureBoundSourceWithholdingPrivateV1(context.Background(), acquisition.Result{}, b, func(context.Context, *sourceRootPrivateV1, string, int64) ([]byte, error) {
				reads++
				return nil, errors.New("identity read spy reached")
			})
			if err == nil || err.Error() != "private source bound capability required" || raw != nil || reads != 0 {
				t.Fatalf("ASSERT_PRIVATE_EXACT_IDENTITY_ZERO_READS: raw=%d err=%v reads=%d", len(raw), err, reads)
			}
		})
	}
	// Exact identity is necessary, not sufficient: the existing Manager lease
	// gate still rejects this explicitly synthetic original without any reads.
	reads := 0
	raw, err := captureBoundSourceWithholdingPrivateV1(context.Background(), acquisition.Result{}, original, func(context.Context, *sourceRootPrivateV1, string, int64) ([]byte, error) { reads++; return nil, nil })
	if err == nil || err.Error() == "private source bound capability required" || raw != nil || reads != 0 {
		t.Fatalf("ASSERT_PRIVATE_IDENTITY_RETAINS_MANAGER_GATE: %v reads=%d", err, reads)
	}
}

func TestSourceWithholdingPrivateV1CapabilityAndV2Isolation(t *testing.T) {
	r, root := coordinatorV2Fixture(t, nil)
	first, err := CaptureV2(context.Background(), r, root)
	if err != nil {
		t.Fatal(err)
	}
	second, err := CaptureV2(context.Background(), r, root)
	if err != nil || !bytes.Equal(first, second) {
		t.Fatal("ASSERT_OLD_V2_BYTES")
	}
	var old EvidenceV2
	if err = json.Unmarshal(first, &old); err != nil {
		t.Fatal(err)
	}
	roundtrip, err := json.Marshal(old)
	if err != nil || !bytes.Equal(bytes.TrimSpace(first), roundtrip) {
		t.Fatal("ASSERT_OLD_V2_ROUNDTRIP_BYTES")
	}
	row := old.Captures[0]
	old.Captures[0] = receiptV2(row.URI, row.Path, row.Classification, PolicyWithheldPrivateV1, nil, nil)
	if err = ValidateV2(old); err == nil {
		t.Fatal("ASSERT_OLD_V2_UNKNOWN_STATUS")
	}
	for _, bound := range []*BoundSourcePrivateV1{nil, {}, {guard: &sessionruntime.RetainedScopeGuardPrivate{}}} {
		if _, err = CaptureSourceWithholdingPrivateV1(context.Background(), r, bound); err == nil {
			t.Fatal("ASSERT_PRIVATE_CAPABILITY_REQUIRED")
		}
	}
	if _, err = BindSourcePrivateV1(context.Background(), nil, &sessionruntime.RetainedScopeGuardPrivate{}, "session", 1, root); err == nil {
		t.Fatal("ASSERT_PRIVATE_CANNOT_FORGE_BINDING")
	}
}
