//go:build lsp_trace_controlled_describe_fixture && (darwin || linux)

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	inv "lsp-trace/internal/inventorysuccessor"
	"lsp-trace/internal/publication"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func completeCLIDirV1(t *testing.T) string {
	t.Helper()
	p, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	if e = os.Chmod(p, 0700); e != nil {
		t.Fatal(e)
	}
	return p
}
func completeCLIFixtureV1(t *testing.T) (privateSourceCompleteDescribeConfig, inv.PrepareDescribePacketV1) {
	t.Helper()
	p, root, owner, fixture := prepareCLIFixture(t)
	p.Persist.Request, p.Persist.ResponseSchema, _ = inv.NewPrivateDescribeRequestV1(p.Persist.Question)
	staged, e := privateSourcePrepareDescribeOwned(context.Background(), p, root, owner, fixture.Manager(), fixture.StartRequest(p.Persist.Deadline), privatePrepareDescribeCloseReports{})
	if e != nil || fixture.Manager().Census().Workers != 0 {
		t.Fatal("SETUP_COMPLETE_CLI_NATIVE_PREPARE", e)
	}
	var packet inv.PrepareDescribePacketV1
	if json.Unmarshal(staged.Packet, &packet) != nil {
		t.Fatal("packet")
	}
	c := privateSourceCompleteDescribeConfig{Schema: "private-source-complete-describe.v1", AccountingRoot: p.AccountingRoot, Workspace: p.Workspace, Verification: p.Verification, ProtectedRoots: []string{p.AccountingRoot}, Open: inv.PreparedDescribeOpenRequestV1{Decision: p.Persist.Decision, Anchors: staged.Continuation, Deadline: time.Now().Add(12 * time.Second), Expires: p.Persist.Expires, RecordLimit: 16 << 20}}
	for i := range c.Destinations {
		path := completeCLIDirV1(t)
		r, e := publication.OpenContentRetentionRoot(context.Background(), path)
		if e != nil {
			t.Fatal(e)
		}
		id, e := r.DirectoryIdentity()
		ce := r.Close()
		if e != nil || ce != nil {
			t.Fatal(e, ce)
		}
		c.Destinations[i] = completeDescribeDestinationConfigV1{path, id}
	}
	c.SuppliedFile = filepath.Join(c.AccountingRoot, "complete-supplied.json")
	if e = os.WriteFile(c.SuppliedFile, completeCLISubmissionV1(t, packet, "A"), 0600); e != nil {
		t.Fatal(e)
	}
	c = completeCLIAdmitV1(t, c, "initial")
	t.Log("ASSERT_COMPLETE_CLI_NATIVE_PREPARE: PASS")
	return c, packet
}
func completeCLISubmissionV1(t *testing.T, p inv.PrepareDescribePacketV1, label string) []byte {
	t.Helper()
	semantic := `{"verdict":"COMPLETE","target_role":"` + label + `","consumer_need":{"status":"UNRESOLVED","value":""},"provided_behavior":{"value":"fixed synthetic description","consumer_relative":false},"boundary_contribution":"synthetic only","limitations":["not qualification"]}`
	text, _ := json.Marshal(semantic)
	adapter := []byte(`{"status":"COMPLETE","text":` + string(text) + `,"tokens":0,"load_ms":0,"run_ms":0,"cancelled":false,"decode_status":0,"grammar":"","context_tokens":0}` + "\n")
	b, e := json.Marshal(inv.CompleteDescribeSuppliedV1{Schema: inv.PrivateDescribeSuppliedSchemaV1, Bindings: inv.CompleteDescribeBindingsV1{QuestionSHA256: p.QuestionSHA256, RequestSHA256: p.RequestSHA256, ResponseSchemaSHA256: p.ResponseSchemaSHA256, DefinitionSHA256: p.DefinitionSHA256, ProvenanceSHA256: p.ProvenanceSHA256}, Adapter: adapter})
	if e != nil {
		t.Fatal(e)
	}
	return b
}

// TEST ONLY canonical admission. Uses existing accepted initializer and verifier;
// not an issuer, JSON/hashes alone are not a production admission capability.
func completeCLIAdmitV1(t *testing.T, c privateSourceCompleteDescribeConfig, label string) privateSourceCompleteDescribeConfig {
	t.Helper()
	_, decision, job, auth := readyCLIFixture(t)
	var old struct {
		Identity, Scope string
		StableID        string `json:"stable_id"`
	}
	if json.Unmarshal(job, &old) != nil {
		t.Fatal("job")
	}
	job = bytes.Replace(job, []byte(`"scope":"`+old.Scope+`"`), []byte(`"scope":"`+c.Open.Anchors.CompletionScope+`"`), 1)
	job = bytes.Replace(job, []byte(`"stable_id":"`+old.StableID+`"`), []byte(`"stable_id":"`+prepareCLIHash("unit3-completion/"+label)+`"`), 1)
	seal := func(schema string, raw []byte) ([]byte, string) {
		var v struct{ Identity string }
		if json.Unmarshal(raw, &v) != nil {
			t.Fatal("seal")
		}
		u := bytes.Replace(raw, []byte(`"identity":"`+v.Identity+`"`), []byte(`"identity":""`), 1)
		id := prepareCLIHash("inventory-successor/private/v2\x00" + schema + "\x00" + string(u))
		return bytes.Replace(u, []byte(`"identity":""`), []byte(`"identity":"`+id+`"`), 1), id
	}
	job, jid := seal("lsp-trace.inventory-job-identity.private.v2", job)
	auth = bytes.Replace(auth, []byte(old.Identity), []byte(jid), 1)
	auth = bytes.ReplaceAll(auth, []byte(":8"), []byte(":100000000000000"))
	auth = bytes.ReplaceAll(auth, []byte(":1,"), []byte(":100000000000000,"))
	auth = bytes.ReplaceAll(auth, []byte(":1}"), []byte(":100000000000000}"))
	auth = bytes.Replace(auth, []byte(`"revision":100000000000000,`), []byte(`"revision":1,`), 1)
	auth = bytes.Replace(auth, []byte(`"max_windows":100000000000000,`), []byte(`"max_windows":100,`), 1)
	auth = bytes.Replace(auth, []byte(`"max_window_ns":100000000000000,`), []byte(`"max_window_ns":15000000000,`), 1)
	auth, aid := seal("lsp-trace.inventory-job-authorization.private.v2", auth)
	root, e := publication.OpenRoot(c.AccountingRoot)
	if e != nil {
		t.Fatal(e)
	}
	initialized, e := inv.InitializePrivateReadyV2(context.Background(), root, job, auth, inv.PrivateReadyDecisionAssertionV2{Job: jid, Authorization: aid, Reference: decision.Reference}, c.Verification)
	ce := root.Close()
	if e != nil || ce != nil {
		t.Fatal("completion admission", e, ce)
	}
	c.CanonicalJob = job
	c.ExpectedHead = initialized.ConfirmedHead()
	c.Attempt = prepareCLIHash("complete/" + label)
	c.Open.Attempt = prepareCLIHash("open/" + label)
	c.Open.Deadline = time.Now().Add(12 * time.Second)
	return c
}
func completeCLIOwnedV1(t *testing.T, c privateSourceCompleteDescribeConfig) (*privateCompleteDescribeOwnedV1, *os.File) {
	t.Helper()
	h := &privateCompleteDescribeOwnedV1{}
	var e error
	h.root, e = publication.OpenRoot(c.AccountingRoot)
	if e != nil {
		t.Fatal(e)
	}
	h.owner, e = inv.OpenPrivateManagedInputOwnerV2(context.Background(), h.root, c.CanonicalJob, c.ExpectedHead, c.Verification)
	if e != nil {
		t.Fatal(e)
	}
	h.source, e = publication.OpenSourceProtectionRootPrivateV2(context.Background(), c.Workspace)
	if e != nil {
		t.Fatal(e)
	}
	for _, p := range c.ProtectedRoots {
		r, e := publication.OpenObservedRoot(context.Background(), p)
		if e != nil {
			t.Fatal(e)
		}
		h.protected = append(h.protected, r)
	}
	for i, d := range c.Destinations {
		h.outputs[i], e = publication.OpenContentRetentionRoot(context.Background(), d.Path)
		if e != nil {
			t.Fatal(e)
		}
	}
	path := filepath.Join(c.AccountingRoot, "owned-config-"+strings.TrimPrefix(c.Attempt, "sha256:")+".json")
	raw, _ := json.Marshal(c)
	if e = os.WriteFile(path, raw, 0600); e != nil {
		t.Fatal(e)
	}
	config, e := openPrivateCompleteFileV1(path, 64<<10)
	if e != nil {
		t.Fatal(e)
	}
	supplied, e := openPrivateCompleteFileV1(c.SuppliedFile, 1<<20)
	if e != nil {
		t.Fatal(e)
	}
	h.data = []*os.File{config, supplied}
	t.Cleanup(func() {
		if e := h.close(); e != nil {
			t.Error(e)
		}
	})
	return h, supplied
}
func completeCLISelectedV1(t *testing.T, c privateSourceCompleteDescribeConfig) bool {
	t.Helper()
	entries, e := os.ReadDir(c.AccountingRoot)
	if e != nil {
		t.Fatal(e)
	}
	selected := 0
	for _, v := range entries {
		if strings.HasPrefix(v.Name(), ".private-complete-describe-v1-") {
			selected++
		}
	}
	return selected == 1
}
func completeCLICountsV1(t *testing.T, c privateSourceCompleteDescribeConfig) [2]int {
	t.Helper()
	var counts [2]int
	for i, d := range c.Destinations {
		v, e := os.ReadDir(d.Path)
		if e != nil {
			t.Fatal(e)
		}
		for _, entry := range v {
			if strings.HasPrefix(entry.Name(), "sha256:") {
				counts[i]++
			}
		}
	}
	return counts
}
func TestPrivateSourceCompleteDescribeCleanupGate(t *testing.T) {
	for _, fault := range []string{"data", "output", "protected", "source", "root", "cancel", "deadline", "healthy"} {
		t.Run(fault, func(t *testing.T) {
			c, packet := completeCLIFixtureV1(t)
			if fault == "deadline" {
				c.Open.Deadline = time.Now().Add(2 * time.Second)
			}
			h, supplied := completeCLIOwnedV1(t, c)
			owner := h.owner
			root := h.root
			files := append([]*os.File(nil), h.data...)
			outputs := h.outputs
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			seen := map[string]int{}
			head := ""
			h.report = func(kind string, index int) error {
				seen[kind]++
				if kind == "data" {
					if _, e := files[index].Stat(); e == nil {
						t.Fatal("data still open")
					}
					if index == 0 {
						if !completeCLISelectedV1(t, c) || completeCLICountsV1(t, c) != [2]int{1, 1} {
							t.Fatal("SETUP_COMPLETE_PAIR_NOT_REACHED")
						}
						head = owner.PreparationHeadV1()
						t.Log("ASSERT_COMPLETE_CLI_ACTUAL_PAIR: PASS")
					}
				}
				if kind == "output" {
					if _, e := outputs[index].DirectoryIdentity(); e == nil {
						t.Fatal("output still open")
					}
				}
				if kind == "root" {
					if root.ValidatePrivate() == nil {
						t.Fatal("root still open")
					}
					if fault == "cancel" {
						cancel()
					}
					if fault == "deadline" {
						time.Sleep(time.Until(c.Open.Deadline) + time.Millisecond)
					}
				}
				if kind == fault {
					return fmt.Errorf("reported %s close error AFTER actual Close", kind)
				}
				return nil
			}
			result, e := privateSourceCompleteDescribeOwned(ctx, c, h, supplied)
			if seen["data"] != 2 || seen["output"] != 2 || seen["protected"] != 1 || seen["source"] != 1 || seen["root"] != 1 || head == "" || owner.PreparationHeadV1() != head {
				t.Fatal("ASSERT_COMPLETE_ALL_CLOSES_NO_EXTRA_OBSERVE", seen)
			}
			check, err := publication.OpenRoot(c.AccountingRoot)
			if err != nil {
				t.Fatal(err)
			}
			_, verifyErr := inv.OpenPrivateManagedInputOwnerV2(context.Background(), check, c.CanonicalJob, head, c.Verification)
			ce := check.Close()
			if verifyErr != nil || ce != nil {
				t.Fatal("ASSERT_COMPLETE_RETAINED_HEAD", verifyErr, ce)
			}
			var out, diag bytes.Buffer
			status := finishPrivateSourceCompleteDescribe(ctx, c.Open.Deadline, result, e, &out, &diag)
			if fault == "healthy" {
				if e != nil || status != 0 || out.Len() == 0 || result.ObjectID == "" {
					t.Fatal("ASSERT_COMPLETE_CLI_HEALTHY", e, diag.String())
				}
			} else if e == nil || result != (inv.CompletedDescribeV1{}) || status == 0 || out.Len() != 0 {
				t.Fatal("ASSERT_COMPLETE_CLI_CLEANUP_ZERO", fault, e, result, status, out.Len())
			}
			// A fresh independently admitted job cannot repair any terminal outcome.
			for _, label := range []string{"A", "B"} {
				next := completeCLIAdmitV1(t, c, "repeat-"+label)
				if err = os.WriteFile(next.SuppliedFile, completeCLISubmissionV1(t, packet, label), 0600); err != nil {
					t.Fatal(err)
				}
				nh, nf := completeCLIOwnedV1(t, next)
				repeated, re := privateSourceCompleteDescribeOwned(context.Background(), next, nh, nf)
				if re == nil || repeated != (inv.CompletedDescribeV1{}) || completeCLICountsV1(t, c) != [2]int{1, 1} {
					t.Fatal("ASSERT_COMPLETE_CLI_NO_REPAIR", fault, label, re)
				}
			}
			if fault == "healthy" {
				t.Log("ASSERT_COMPLETE_CLI_HEALTHY: PASS")
			} else {
				t.Log("ASSERT_COMPLETE_CLI_CLEANUP_ZERO: PASS", fault)
			}
			t.Log("ASSERT_COMPLETE_CLI_NO_REPAIR: PASS (fresh same-A and B)", fault)
		})
	}
}
func TestPrivateSourceCompleteDescribeRoute(t *testing.T) {
	c, _ := completeCLIFixtureV1(t)
	raw, _ := json.Marshal(c)
	path := filepath.Join(c.AccountingRoot, "route-config.json")
	if e := os.WriteFile(path, raw, 0600); e != nil {
		t.Fatal(e)
	}
	var out, diag bytes.Buffer
	if runPrivateHostApplication([]string{"source-complete-describe", path}, nil, &out, &diag) != 0 {
		t.Fatal("ASSERT_COMPLETE_CLI_ROUTE", diag.String())
	}
	var r inv.CompletedDescribeV1
	if json.Unmarshal(out.Bytes(), &r) != nil || r.ObjectID == "" || r.Authority != 0 || r.Accepted {
		t.Fatal("ASSERT_COMPLETE_CLI_ROUTE_RECEIPT")
	}
	rawPair := []byte(nil)
	for _, d := range c.Destinations {
		p, e := os.ReadFile(filepath.Join(d.Path, r.ObjectID))
		if e != nil {
			t.Fatal(e)
		}
		if rawPair != nil && !bytes.Equal(rawPair, p) {
			t.Fatal("different pair")
		}
		rawPair = p
	}
	var obj inv.CompleteDescribeObjectV1
	if json.Unmarshal(rawPair, &obj) != nil || string(obj.Definition) != "func Query() {}" || obj.FeatureIdentity != "UNRESOLVED" || obj.Completeness != "UNKNOWN" || obj.AbsenceInference != "PROHIBITED" || obj.HOSTCost != "UNKNOWN_NOT_FREE" {
		t.Fatal("ASSERT_COMPLETE_CLI_EXACT_OBJECT")
	}
	// Same already-spent admission and fresh admission both refuse, zero output.
	out.Reset()
	diag.Reset()
	if runPrivateHostApplication([]string{"source-complete-describe", path}, nil, &out, &diag) == 0 || out.Len() != 0 {
		t.Fatal("same admission replay")
	}
	t.Log("ASSERT_COMPLETE_CLI_ROUTE: PASS; ASSERT_COMPLETE_CLI_EXACT_OBJECT: PASS")
}
func TestPrivateSourceCompleteDescribeReadFailurePaid(t *testing.T) {
	c, _ := completeCLIFixtureV1(t)
	h, supplied := completeCLIOwnedV1(t, c)
	o := h.owner
	head := o.PreparationHeadV1()
	if e := supplied.Close(); e != nil {
		t.Fatal(e)
	}
	result, e := privateSourceCompleteDescribeOwned(context.Background(), c, h, supplied)
	if e == nil || result != (inv.CompletedDescribeV1{}) || o.PreparationHeadV1() == head || completeCLISelectedV1(t, c) || completeCLICountsV1(t, c) != [2]int{} {
		t.Fatal("ASSERT_COMPLETE_CLI_READ_FAILURE_PAID", e)
	}
	t.Log("ASSERT_COMPLETE_CLI_READ_FAILURE_PAID: PASS (native Open and Complete reserved; closed supplied file produced no selection)")
}
func TestPrivateSourceCompleteDescribeCleanupFailureJoined(t *testing.T) {
	h := &privateCompleteDescribeOwnedV1{}
	f, e := os.CreateTemp(t.TempDir(), "closed")
	if e != nil {
		t.Fatal(e)
	}
	if e = f.Close(); e != nil {
		t.Fatal(e)
	}
	h.data = []*os.File{f}
	reported := false
	h.report = func(string, int) error { reported = true; return errors.New("reported after actual Close attempt") }
	if h.close() == nil || !reported {
		t.Fatal("actual failed close not joined")
	}
}
