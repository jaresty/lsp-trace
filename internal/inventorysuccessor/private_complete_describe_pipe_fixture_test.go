//go:build lsp_trace_controlled_describe_fixture

package inventorysuccessor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	dw "lsp-trace/internal/describeworker"
	"lsp-trace/internal/publication"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type completeFixtureV1 struct {
	prepare   PrepareDescribeRequestV1
	packet    PrepareDescribePacketV1
	anchors   PreparedDescribeAnchorsV1
	outputs   [2]*publication.ContentRetentionRoot
	paths     [2]string
	ids       [2]publication.RetentionDirectoryIdentity
	source    *publication.SourceProtectionRootPrivateV2
	protected []*publication.ObservedRoot
}

func completeDirV1(t *testing.T) string {
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
func completeFixtureNativeV1(t *testing.T, options ...string) *completeFixtureV1 {
	t.Helper()
	q, fixture := prepareDescribeNativeFixtureV1(t, -1)
	q.Persist.Request, q.Persist.ResponseSchema, _ = NewPrivateDescribeRequestV1(q.Persist.Question)
	if len(options) > 0 {
		switch options[0] {
		case "unsupported":
			q.Persist.Request = []byte("arbitrary retained request")
		case "marker":
			q.Persist.ResponseSchema = []byte("{}")
		case "duplicate":
			q.Persist.Request = append([]byte(`{"mode":"R",`), q.Persist.Request[1:]...)
		case "unknown":
			q.Persist.Request = append([]byte(`{"extra":true,`), q.Persist.Request[1:]...)
		case "trailing":
			q.Persist.Request = append(q.Persist.Request, []byte("{}")...)
		case "noncanonical":
			q.Persist.Request = append([]byte(" "), q.Persist.Request...)
		}
	}
	staged, e := PrepareDescribeV1(context.Background(), q)
	if e != nil {
		t.Fatal("SETUP_NATIVE_PREPARE", e)
	}
	if fixture.Manager().Census().Workers != 0 {
		t.Fatal("native producer not joined")
	}
	f := &completeFixtureV1{prepare: q, anchors: staged.Continuation}
	if json.Unmarshal(staged.Packet, &f.packet) != nil {
		t.Fatal("packet")
	}
	for i := range f.outputs {
		f.paths[i] = completeDirV1(t)
		f.outputs[i], e = publication.OpenContentRetentionRoot(context.Background(), f.paths[i])
		if e != nil {
			t.Fatal(e)
		}
		f.ids[i], e = f.outputs[i].DirectoryIdentity()
		if e != nil {
			t.Fatal(e)
		}
		r := f.outputs[i]
		t.Cleanup(func() {
			if e := r.Close(); e != nil {
				t.Error(e)
			}
		})
	}
	f.source, e = publication.OpenSourceProtectionRootPrivateV2(context.Background(), q.Persist.RootPath)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		if e := f.source.Close(); e != nil {
			t.Error(e)
		}
	})
	p, e := publication.OpenObservedRoot(context.Background(), q.Persist.RootPath)
	if e != nil {
		t.Fatal(e)
	}
	f.protected = []*publication.ObservedRoot{p}
	t.Cleanup(func() {
		if e := p.Close(); e != nil {
			t.Error(e)
		}
	})
	t.Log("ASSERT_COMPLETE_NATIVE_PREPARE: PASS")
	return f
}
func completeOwnerV1(t *testing.T, f *completeFixtureV1, label string, unfunded int) *PrivateManagedInputOwnerV2 {
	t.Helper()
	j, a, d := fixtureV2(t)
	j.Scope = f.anchors.CompletionScope
	j.StableID = hashLabelV2("complete-job/" + label)
	sealTestV2(t, &j)
	a.Job = j.Identity
	var funds [14]uint64
	for i := range funds {
		funds[i] = 1e14
	}
	if unfunded == 14 {
		base, e := preparedDescribeChargeV1(preparedDescribeRecordMaxV1, time.Now(), time.Now().Add(time.Second))
		if e != nil {
			t.Fatal(e)
		}
		funds[9] = base.Calls
	} else if unfunded >= 0 {
		funds[unfunded] = 0
	}
	a.Ceiling = countersFromV2(funds)
	a.MaxTranche = a.Ceiling
	a.MaxWindows = 100
	a.Expires = f.prepare.Persist.Expires.UnixNano()
	a.MaxWindowNS = uint64(15 * time.Second)
	sealTestV2(t, &a)
	d.Job = j.Identity
	d.Authorization = a.Identity
	o := f.prepare.Owner
	h, e := o.k.initialize(context.Background(), j, a, d, time.Now().UnixNano(), o.limits)
	if e != nil {
		t.Fatal(e)
	}
	opened, e := OpenPrivateManagedInputOwnerV2(context.Background(), o.k.root, sealTestV2(t, &j), h.Identity, PrivateManagedInputVerificationV2{4096, 64 << 20, 16384})
	if e != nil {
		t.Fatal(e)
	}
	return opened
}
func completeAdapterV1(t *testing.T, label string) []byte {
	t.Helper()
	semantic := `{"verdict":"COMPLETE","target_role":"` + label + `","consumer_need":{"status":"UNRESOLVED","value":""},"provided_behavior":{"value":"fixed synthetic description","consumer_relative":false},"boundary_contribution":"synthetic only","limitations":["not semantic qualification"]}`
	text, _ := json.Marshal(semantic)
	return []byte(`{"status":"COMPLETE","text":` + string(text) + `,"tokens":0,"load_ms":0,"run_ms":0,"cancelled":false,"decode_status":0,"grammar":"","context_tokens":0}` + "\n")
}
func completeSubmissionV1(t *testing.T, f *completeFixtureV1, label string) []byte {
	t.Helper()
	p := f.packet
	b, e := json.Marshal(CompleteDescribeSuppliedV1{PrivateDescribeSuppliedSchemaV1, CompleteDescribeBindingsV1{p.QuestionSHA256, p.RequestSHA256, p.ResponseSchemaSHA256, p.DefinitionSHA256, p.ProvenanceSHA256}, completeAdapterV1(t, label)})
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func completeRequestV1(f *completeFixtureV1, o *PrivateManagedInputOwnerV2, label string) CompleteDescribeRequestV1 {
	return CompleteDescribeRequestV1{Root: f.prepare.Capture.Root, ExpectedHead: o.PreparationHeadV1(), Attempt: hashLabelV2("complete/" + label), Open: PreparedDescribeOpenRequestV1{Attempt: hashLabelV2("open/" + label), Decision: f.prepare.Persist.Decision, Anchors: f.anchors, Deadline: time.Now().Add(12 * time.Second), Expires: f.prepare.Persist.Expires, RecordLimit: 16 << 20}, Outputs: f.outputs, DestinationIdentities: f.ids, Source: f.source, Protected: f.protected}
}
func completeExecuteV1(ctx context.Context, o *PrivateManagedInputOwnerV2, q CompleteDescribeRequestV1, raw []byte) (CompletedDescribeV1, error) {
	op, e := o.ReserveCompleteDescribeV1(ctx, q)
	if e != nil {
		return CompletedDescribeV1{}, e
	}
	return CompleteDescribeV1(ctx, op, raw)
}
func completeFilesV1(t *testing.T, f *completeFixtureV1) [2]int {
	t.Helper()
	var counts [2]int
	for i, p := range f.paths {
		entries, e := os.ReadDir(p)
		if e != nil {
			t.Fatal(e)
		}
		for _, v := range entries {
			if len(v.Name()) > 7 && v.Name()[:7] == "sha256:" {
				counts[i]++
			}
		}
	}
	return counts
}

func TestCompleteDescribeSingleUse(t *testing.T) {
	f := completeFixtureNativeV1(t)
	for n, label := range []string{"A", "B", "A"} {
		id := string(rune('a' + n))
		o := completeOwnerV1(t, f, id, -1)
		q := completeRequestV1(f, o, id)
		before := completeFilesV1(t, f)
		result, e := completeExecuteV1(context.Background(), o, q, completeSubmissionV1(t, f, label))
		if n == 0 {
			if e != nil || result.ObjectID == "" {
				t.Fatal("SETUP_COMPLETE_FIRST_SELECTION", e)
			}
			t.Log("ASSERT_COMPLETE_FIRST_NATIVE_PAIR: PASS")
		} else if e == nil || result.ObjectID != "" || completeFilesV1(t, f) != before {
			t.Error("ASSERT_COMPLETE_SINGLE_USE", label, e, result.ObjectID)
		} else {
			t.Log("ASSERT_COMPLETE_SINGLE_USE: PASS", label)
		}
	}
	raw, e := publication.ReadVerifiedBoundFileChecked(f.prepare.Capture.Root, completeDescribeSelectorV1(f.anchors), completeDescribeLimitV1)
	if e != nil || !bytes.Contains(raw, []byte("lsp-trace.private-complete-describe-selection.v1")) {
		t.Fatal("selection not retained", e)
	}
}

func TestCompleteDescribePreflightAndTopology(t *testing.T) {
	f := completeFixtureNativeV1(t)
	for _, fault := range []string{"head", "root", "anchor", "expiry", "deadline", "cancel", "source", "protected-missing", "same-destination", "output-inside-source", "protected-output", "closed-output", "renamed-output"} {
		t.Run(fault, func(t *testing.T) {
			o := completeOwnerV1(t, f, "preflight-"+fault, -1)
			q := completeRequestV1(f, o, "preflight-"+fault)
			ctx := context.Background()
			preflight := true
			switch fault {
			case "head":
				q.ExpectedHead = hashLabelV2("foreign-head")
			case "root":
				q.Root = nil
			case "anchor":
				q.Open.Anchors.Workflow = "LOCAL"
			case "expiry":
				q.Open.Expires = time.Now().Add(-time.Second)
			case "deadline":
				q.Open.Deadline = time.Now().Add(16 * time.Second)
			case "cancel":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			default:
				preflight = false
				switch fault {
				case "source":
					r, e := publication.OpenSourceProtectionRootPrivateV2(ctx, completeDirV1(t))
					if e != nil {
						t.Fatal(e)
					}
					q.Source = r
					t.Cleanup(func() { r.Close() })
				case "protected-missing":
					r, e := publication.OpenObservedRoot(ctx, completeDirV1(t))
					if e != nil {
						t.Fatal(e)
					}
					q.Protected = []*publication.ObservedRoot{r}
					t.Cleanup(func() { r.Close() })
				case "same-destination":
					q.Outputs[1] = q.Outputs[0]
					q.DestinationIdentities[1] = q.DestinationIdentities[0]
				case "output-inside-source":
					p := filepath.Join(f.prepare.Persist.RootPath, "output-child")
					if e := os.Mkdir(p, 0700); e != nil {
						t.Fatal(e)
					}
					r, e := publication.OpenContentRetentionRoot(ctx, p)
					if e != nil {
						t.Fatal(e)
					}
					q.Outputs[0] = r
					q.DestinationIdentities[0], e = r.DirectoryIdentity()
					if e != nil {
						t.Fatal(e)
					}
					t.Cleanup(func() { r.Close() })
				case "protected-output":
					r, e := publication.OpenObservedRoot(ctx, f.paths[0])
					if e != nil {
						t.Fatal(e)
					}
					q.Protected = append(q.Protected, r)
					t.Cleanup(func() { r.Close() })
				case "closed-output":
					r, e := publication.OpenContentRetentionRoot(ctx, f.paths[0])
					if e != nil {
						t.Fatal(e)
					}
					q.Outputs[0] = r
					if e = r.Close(); e != nil {
						t.Fatal(e)
					}
				case "renamed-output":
					p := completeDirV1(t)
					r, e := publication.OpenContentRetentionRoot(ctx, p)
					if e != nil {
						t.Fatal(e)
					}
					q.Outputs[0] = r
					q.DestinationIdentities[0], e = r.DirectoryIdentity()
					if e != nil {
						t.Fatal(e)
					}
					if e = os.Rename(p, p+"-moved"); e != nil {
						t.Fatal(e)
					}
					t.Cleanup(func() { r.Close(); os.RemoveAll(p + "-moved") })
				}
			}
			before, e := o.k.reopen(context.Background(), o.job, o.limits)
			if e != nil {
				t.Fatal(e)
			}
			out, e := completeExecuteV1(ctx, o, q, completeSubmissionV1(t, f, "A"))
			after, re := o.k.reopen(context.Background(), o.job, o.limits)
			if e == nil || out.ObjectID != "" || re != nil || completeFilesV1(t, f) != [2]int{} {
				t.Fatal("ASSERT_COMPLETE_PREFLIGHT_TOPOLOGY", fault, e, re)
			}
			if preflight && (after.Charged != before.Charged || after.Windows != before.Windows) {
				t.Fatal("ASSERT_COMPLETE_PREFLIGHT_NO_EFFECT", fault)
			}
			if completeSelectedExistsV1(t, f) {
				t.Fatal("invalid topology selected", fault)
			}
			t.Log("ASSERT_COMPLETE_PREFLIGHT_TOPOLOGY: PASS", fault)
		})
	}
}
func completeSelectedExistsV1(t *testing.T, f *completeFixtureV1) bool {
	t.Helper()
	_, e := os.Lstat(filepath.Join(f.prepare.Capture.Root.Path(), completeDescribeSelectorV1(f.anchors)))
	if e == nil {
		return true
	}
	if !os.IsNotExist(e) {
		t.Fatal(e)
	}
	return false
}
func TestCompleteDescribeCompletePhaseUnfunded(t *testing.T) {
	f := completeFixtureNativeV1(t)
	o := completeOwnerV1(t, f, "only-open-funded", 14)
	q := completeRequestV1(f, o, "only-open-funded")
	op, e := o.ReserveCompleteDescribeV1(context.Background(), q)
	h, re := o.k.reopen(context.Background(), o.job, o.limits)
	if e == nil || op.state != nil || re != nil || h.Windows != 1 || h.Charged.Calls == 0 || completeSelectedExistsV1(t, f) || completeFilesV1(t, f) != [2]int{} {
		t.Fatal("ASSERT_COMPLETE_PHASE_UNFUNDED", e, re, h.Windows)
	}
	t.Log("ASSERT_COMPLETE_PHASE_UNFUNDED: PASS (paid Open retained; Complete denied before supplied read/parse or publication)")
}
func TestCompleteDescribeExistingSelector(t *testing.T) {
	for _, kind := range []string{"invalid", "symlink", "directory"} {
		t.Run(kind, func(t *testing.T) {
			f := completeFixtureNativeV1(t)
			p := filepath.Join(f.prepare.Capture.Root.Path(), completeDescribeSelectorV1(f.anchors))
			var e error
			switch kind {
			case "invalid":
				e = os.WriteFile(p, []byte("invalid selected data"), 0600)
			case "symlink":
				e = os.Symlink("missing-target", p)
			case "directory":
				e = os.Mkdir(p, 0700)
			}
			if e != nil {
				t.Fatal(e)
			}
			before, e := os.Lstat(p)
			if e != nil {
				t.Fatal(e)
			}
			o := completeOwnerV1(t, f, "existing", -1)
			r, e := completeExecuteV1(context.Background(), o, completeRequestV1(f, o, "existing"), completeSubmissionV1(t, f, "A"))
			after, se := os.Lstat(p)
			if e == nil || r.ObjectID != "" || se != nil || !os.SameFile(before, after) || completeFilesV1(t, f) != [2]int{} {
				t.Fatal("ASSERT_COMPLETE_EXISTING_SELECTOR_REFUSED", kind, e, se)
			}
			t.Log("ASSERT_COMPLETE_EXISTING_SELECTOR_REFUSED: PASS", kind)
		})
	}
}
func TestCompleteDescribeFreshPhasePaid(t *testing.T) {
	f := completeFixtureNativeV1(t)
	o := completeOwnerV1(t, f, "paid-phase", -1)
	q := completeRequestV1(f, o, "paid-phase")
	before, e := o.k.reopen(context.Background(), o.job, o.limits)
	if e != nil {
		t.Fatal(e)
	}
	op, e := o.ReserveCompleteDescribeV1(context.Background(), q)
	if e != nil {
		t.Fatal(e)
	}
	reserved, e := o.k.reopen(context.Background(), o.job, o.limits)
	if e != nil {
		t.Fatal(e)
	}
	base, e := preparedDescribeChargeV1(preparedDescribeRecordMaxV1, time.Now(), q.Open.Deadline)
	if e != nil {
		t.Fatal(e)
	}
	for i, amount := range reserved.Charged.vector() {
		delta := amount - before.Charged.vector()[i]
		minimum := 5 * base.vector()[i]
		if delta < minimum {
			t.Fatalf("ASSERT_COMPLETE_FRESH_PHASE_PAID coordinate=%d debit=%d want-at-least=%d", i, delta, minimum)
		}
	}
	result, e := CompleteDescribeV1(context.Background(), op, []byte("bad supplied payload"))
	after, re := o.k.reopen(context.Background(), o.job, o.limits)
	if e == nil || result.ObjectID != "" || re != nil || after.Charged != reserved.Charged || after.Identity != reserved.Identity || completeSelectedExistsV1(t, f) {
		t.Fatal("ASSERT_COMPLETE_REJECTED_PARSE_RETAINS_DEBIT", e, re)
	}
	t.Log("ASSERT_COMPLETE_FRESH_PHASE_PAID: PASS; ASSERT_COMPLETE_REJECTED_PARSE_RETAINS_DEBIT: PASS")
}
func TestCompleteDescribeBindings(t *testing.T) {
	f := completeFixtureNativeV1(t)
	for _, fault := range []string{"question", "request", "schema", "definition", "provenance", "unknown", "duplicate", "trailing", "noncanonical", "adapter", "adapter-bound", "semantic-bound", "raw-bound", "destination", "expired", "cancel"} {
		t.Run(fault, func(t *testing.T) {
			o := completeOwnerV1(t, f, "bad-"+fault, -1)
			q := completeRequestV1(f, o, "bad-"+fault)
			raw := completeSubmissionV1(t, f, "A")
			var s CompleteDescribeSuppliedV1
			json.Unmarshal(raw, &s)
			wrong := hashLabelV2("wrong")
			switch fault {
			case "question":
				s.Bindings.QuestionSHA256 = wrong
			case "request":
				s.Bindings.RequestSHA256 = wrong
			case "schema":
				s.Bindings.ResponseSchemaSHA256 = wrong
			case "definition":
				s.Bindings.DefinitionSHA256 = wrong
			case "provenance":
				s.Bindings.ProvenanceSHA256 = wrong
			case "adapter":
				s.Adapter = []byte("{}\n")
			case "adapter-bound":
				s.Adapter = bytes.Repeat([]byte("x"), (512<<10)+1)
			case "semantic-bound":
				s.Adapter = completeAdapterV1(t, strings.Repeat("x", 65537))
			case "destination":
				q.DestinationIdentities[0].Inode++
			case "expired":
				q.Open.Deadline = time.Now().Add(-time.Second)
			}
			raw, _ = json.Marshal(s)
			switch fault {
			case "unknown":
				raw = append([]byte(`{"extra":1,`), raw[1:]...)
			case "duplicate":
				raw = append([]byte(`{"schema":"`+s.Schema+`",`), raw[1:]...)
			case "trailing":
				raw = append(raw, []byte("{}")...)
			case "noncanonical":
				raw = append(raw, '\n')
			case "raw-bound":
				raw = bytes.Repeat([]byte("x"), (1<<20)+1)
			}
			ctx := context.Background()
			if fault == "cancel" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			before, e := o.k.reopen(context.Background(), o.job, o.limits)
			if e != nil {
				t.Fatal(e)
			}
			out, e := completeExecuteV1(ctx, o, q, raw)
			after, reopenErr := o.k.reopen(context.Background(), o.job, o.limits)
			if e == nil || out.ObjectID != "" || reopenErr != nil || completeFilesV1(t, f) != [2]int{} {
				t.Fatal("ASSERT_COMPLETE_BINDING_REFUSED", fault, e, reopenErr)
			}
			if fault != "expired" && fault != "cancel" && (after.Windows <= before.Windows || after.Charged.ReadHashBytes <= before.Charged.ReadHashBytes) {
				t.Fatal("ASSERT_COMPLETE_REJECTION_PAID", fault)
			}
			if _, e := os.Stat(filepath.Join(q.Root.Path(), completeDescribeSelectorV1(f.anchors))); !os.IsNotExist(e) {
				t.Fatal("invalid answer selected", e)
			}
			t.Log("ASSERT_COMPLETE_BINDING_REFUSED: PASS", fault)
		})
	}
}
func TestCompleteDescribeUnsupportedRequest(t *testing.T) {
	for _, fault := range []string{"unsupported", "marker", "duplicate", "unknown", "trailing", "noncanonical"} {
		t.Run(fault, func(t *testing.T) {
			f := completeFixtureNativeV1(t, fault)
			o := completeOwnerV1(t, f, fault, -1)
			out, e := completeExecuteV1(context.Background(), o, completeRequestV1(f, o, fault), completeSubmissionV1(t, f, "A"))
			if e == nil || out.ObjectID != "" || completeFilesV1(t, f) != [2]int{} {
				t.Fatal("ASSERT_COMPLETE_UNSUPPORTED_REQUEST", fault, e)
			}
			t.Log("ASSERT_COMPLETE_UNSUPPORTED_REQUEST: PASS", fault)
		})
	}
}
func TestCompleteDescribeCurrencies(t *testing.T) {
	for coordinate := 0; coordinate < 14; coordinate++ {
		t.Run(string(rune('A'+coordinate)), func(t *testing.T) {
			f := completeFixtureNativeV1(t)
			o := completeOwnerV1(t, f, "fund", coordinate)
			q := completeRequestV1(f, o, "fund")
			before, e := o.k.reopen(context.Background(), o.job, o.limits)
			if e != nil {
				t.Fatal(e)
			}
			out, e := completeExecuteV1(context.Background(), o, q, completeSubmissionV1(t, f, "A"))
			after, reopenErr := o.k.reopen(context.Background(), o.job, o.limits)
			if reopenErr != nil {
				t.Fatal(reopenErr)
			}
			zero := coordinate == 5 || coordinate == 6 || coordinate == 10 || coordinate == 11 || coordinate == 12
			if zero {
				if e != nil || out.ObjectID == "" || after.Charged.vector()[coordinate] != 0 {
					t.Fatal("ASSERT_COMPLETE_ZERO_PROVIDER_CURRENCY", coordinate, e)
				}
			} else if e == nil || out.ObjectID != "" || after.Charged != before.Charged || after.Windows != before.Windows || completeFilesV1(t, f) != [2]int{} {
				t.Fatal("ASSERT_COMPLETE_UNFUNDED_NO_EFFECT", coordinate, e)
			}
			t.Log("ASSERT_COMPLETE_CURRENCY: PASS", coordinate, "required-zero", zero)
		})
	}
}
func completeExpectedRawV1(t *testing.T, op CompleteDescribeOperationV1, submission []byte) []byte {
	t.Helper()
	s := op.state
	q := s.request
	p := s.opened.state
	a := s.opened.originals
	var supplied CompleteDescribeSuppliedV1
	if json.Unmarshal(submission, &supplied) != nil {
		t.Fatal("submission")
	}
	candidate, e := dw.ParseSuppliedTerminalResponse(supplied.Adapter, dw.SuppliedResponseLimits{MaxAdapterBytes: 512 << 10, MaxSemanticBytes: 64 << 10})
	if e != nil {
		t.Fatal(e)
	}
	v := CompleteDescribeObjectV1{Schema: "lsp-trace.private-complete-describe-object.v1", FeatureIdentity: "UNRESOLVED", Completeness: "UNKNOWN", AbsenceInference: "PROHIBITED", HOSTCost: "UNKNOWN_NOT_FREE", Origin: dw.SuppliedTerminalResponseOnly, Execution: dw.NoProcessOrHostExecutionVerified, RoleLabels: [2]string{"SOURCE_PROVIDER_ATTRIBUTION_ONLY", "EVALUATOR_ATTRIBUTION_ONLY"}, Anchors: q.Open.Anchors, CompletionJob: s.owner.job.Identity, Reservation: s.reservation, Destinations: q.DestinationIdentities, Bindings: supplied.Bindings, Question: a.question, Request: a.request, ResponseSchema: a.responseSchema, Definition: p.definition, Provenance: p.provenance, Adapter: supplied.Adapter, Semantic: candidate.CanonicalSemantic(), AdapterSHA256: candidate.RawAdapterDigest(), SemanticSHA256: candidate.CanonicalSemanticDigest()}
	raw, e := preparedDescribeEncodeV1(v, completeDescribeLimitV1)
	if e != nil {
		t.Fatal(e)
	}
	return raw
}

type completeChildConfigV1 struct {
	Barrier    string // test-only cross-process start, never production configuration
	Root       string
	Job        []byte
	Head       string
	Open       PreparedDescribeOpenRequestV1
	Attempt    string
	Paths      [2]string
	IDs        [2]publication.RetentionDirectoryIdentity
	Submission []byte
	ReadID     string
	Expected   []byte
	ResultPath string
}

func completeChildConfigForV1(t *testing.T, f *completeFixtureV1, label string) completeChildConfigV1 {
	t.Helper()
	o := completeOwnerV1(t, f, label, -1)
	q := completeRequestV1(f, o, label)
	return completeChildConfigV1{Root: q.Root.Path(), Job: sealTestV2(t, &o.job), Head: o.head, Open: q.Open, Attempt: q.Attempt, Paths: f.paths, IDs: f.ids, Submission: completeSubmissionV1(t, f, label)}
}
func completeChildAdmitV1(t *testing.T, c completeChildConfigV1) (*PrivateManagedInputOwnerV2, CompleteDescribeRequestV1) {
	t.Helper()
	root, e := publication.OpenRoot(c.Root)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		if e := root.Close(); e != nil {
			t.Error(e)
		}
	})
	o, e := OpenPrivateManagedInputOwnerV2(context.Background(), root, c.Job, c.Head, PrivateManagedInputVerificationV2{4096, 64 << 20, 16384})
	if e != nil {
		t.Fatal(e)
	}
	q := CompleteDescribeRequestV1{Root: root, ExpectedHead: c.Head, Open: c.Open, Attempt: c.Attempt, DestinationIdentities: c.IDs}
	for i, p := range c.Paths {
		r, e := publication.OpenContentRetentionRoot(context.Background(), p)
		if e != nil {
			t.Fatal(e)
		}
		q.Outputs[i] = r
		t.Cleanup(func() {
			if e := r.Close(); e != nil {
				t.Error(e)
			}
		})
	}
	q.Source, e = publication.OpenSourceProtectionRootPrivateV2(context.Background(), c.Root)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		if e := q.Source.Close(); e != nil {
			t.Error(e)
		}
	})
	r, e := publication.OpenObservedRoot(context.Background(), c.Root)
	if e != nil {
		t.Fatal(e)
	}
	q.Protected = []*publication.ObservedRoot{r}
	t.Cleanup(func() {
		if e := r.Close(); e != nil {
			t.Error(e)
		}
	})
	return o, q
}
func TestCompleteDescribeConcurrency(t *testing.T) {
	for _, mode := range []string{"threads", "processes"} {
		t.Run(mode, func(t *testing.T) {
			f := completeFixtureNativeV1(t)
			configs := []completeChildConfigV1{completeChildConfigForV1(t, f, "A"), completeChildConfigForV1(t, f, "B")}
			successes := 0
			if mode == "threads" {
				type admitted struct {
					op  CompleteDescribeOperationV1
					raw []byte
				}
				var a []admitted
				for _, c := range configs {
					o, q := completeChildAdmitV1(t, c)
					op, e := o.ReserveCompleteDescribeV1(context.Background(), q)
					if e != nil {
						t.Fatal("SETUP_BOTH_PAID_NOT_REACHED", e)
					}
					a = append(a, admitted{op, c.Submission})
				}
				var wg sync.WaitGroup
				results := make(chan bool, 2)
				start := make(chan struct{})
				for _, v := range a {
					wg.Add(1)
					go func(v admitted) {
						defer wg.Done()
						<-start
						r, e := CompleteDescribeV1(context.Background(), v.op, v.raw)
						results <- e == nil && r.ObjectID != ""
					}(v)
				}
				close(start)
				wg.Wait()
				close(results)
				for ok := range results {
					if ok {
						successes++
					}
				}
			} else {
				var cmds []*exec.Cmd
				for i, c := range configs {
					c.ResultPath = filepath.Join(c.Root, "child-result-"+string(rune('A'+i))+".json")
					c.Barrier = filepath.Join(c.Root, "both-paid-start")
					configs[i] = c
					raw, _ := json.Marshal(c)
					path := filepath.Join(c.Root, "child-config-"+string(rune('A'+i))+".json")
					if e := os.WriteFile(path, raw, 0600); e != nil {
						t.Fatal(e)
					}
					cmd := exec.Command(os.Args[0], "-test.run=^TestCompleteDescribeChild$", "-test.v", "-test.timeout=30s")
					cmd.Env = append(os.Environ(), "UNIT3_CHILD_CONFIG="+path)
					cmds = append(cmds, cmd)
				}
				var wg sync.WaitGroup
				outputs := make([][]byte, 2)
				errs := make([]error, 2)
				for i, cmd := range cmds {
					wg.Add(1)
					go func(i int, cmd *exec.Cmd) { defer wg.Done(); outputs[i], errs[i] = cmd.CombinedOutput() }(i, cmd)
				}
				ready := false
				until := time.Now().Add(8 * time.Second)
				for time.Now().Before(until) {
					ready = true
					for _, c := range configs {
						if _, e := os.Stat(c.ResultPath + ".ready"); e != nil {
							ready = false
						}
					}
					if ready {
						break
					}
					time.Sleep(5 * time.Millisecond)
				}
				if e := os.WriteFile(configs[0].Barrier, []byte("both paid; execute now"), 0600); e != nil {
					t.Fatal(e)
				}
				wg.Wait()
				if !ready {
					t.Fatalf("SETUP_BOTH_PAID_NOT_REACHED %s %s", outputs[0], outputs[1])
				}
				t.Log("ASSERT_COMPLETE_BOTH_CHILDREN_PAID_BEFORE_RELEASE: PASS")
				for i, c := range configs {
					t.Logf("child %d: %s", i, outputs[i])
					if errs[i] != nil {
						t.Fatal(errs[i])
					}
					raw, e := os.ReadFile(c.ResultPath)
					if e != nil {
						t.Fatal(e)
					}
					var r struct{ Success bool }
					if json.Unmarshal(raw, &r) != nil {
						t.Fatal("result")
					}
					if r.Success {
						successes++
					}
				}
			}
			if successes != 1 || completeFilesV1(t, f) != [2]int{1, 1} {
				t.Fatal("ASSERT_COMPLETE_ATOMIC_ONE_WINNER", mode, successes, completeFilesV1(t, f))
			}
			raw, e := publication.ReadVerifiedBoundFileChecked(f.prepare.Capture.Root, completeDescribeSelectorV1(f.anchors), completeDescribeLimitV1)
			if e != nil {
				t.Fatal(e)
			}
			var selection completeDescribeSelectionV1
			if json.Unmarshal(raw, &selection) != nil {
				t.Fatal("selection")
			}
			for _, r := range f.outputs {
				got, e := r.Read(context.Background(), selection.ObjectID, int64(selection.ObjectBytes))
				if e != nil || digest(got) != selection.ObjectID {
					t.Fatal("winner pair", e)
				}
			}
			t.Log("ASSERT_COMPLETE_ATOMIC_ONE_WINNER: PASS", mode)
		})
	}
}
func TestCompleteDescribeChild(t *testing.T) {
	path := os.Getenv("UNIT3_CHILD_CONFIG")
	if path == "" {
		t.Skip("fresh-process helper")
	}
	raw, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	var c completeChildConfigV1
	if json.Unmarshal(raw, &c) != nil {
		t.Fatal("child config")
	}
	o, q := completeChildAdmitV1(t, c)
	if c.ReadID != "" {
		// Test-only independently funded read admission; no new production Inspect API.
		now := time.Now()
		charge, e := completeDescribeChargeV1(now, now.Add(10*time.Second))
		if e != nil {
			t.Fatal(e)
		}
		h, e := o.k.reserve(context.Background(), o.job, o.head, c.Attempt, charge, now.Add(10*time.Second).UnixNano(), now.UnixNano(), o.limits)
		if e != nil {
			t.Fatal(e)
		}
		for _, r := range q.Outputs {
			got, e := r.Read(context.Background(), c.ReadID, completeDescribeLimitV1)
			if e != nil || !bytes.Equal(got, c.Expected) {
				t.Fatal("ASSERT_COMPLETE_FRESH_READ", e)
			}
		}
		if publication.CheckPostPreparationObjectsDistinctPrivateV2(context.Background(), q.Outputs, c.ReadID) != nil {
			t.Fatal("not distinct")
		}
		if _, e = o.k.observe(context.Background(), o.job, h.Identity, c.Attempt, "OBSERVED", time.Now().UnixNano(), o.limits); e != nil {
			t.Fatal(e)
		}
		t.Log("ASSERT_COMPLETE_FRESH_READ: PASS")
		return
	}
	op, e := o.ReserveCompleteDescribeV1(context.Background(), q)
	if e != nil {
		t.Fatal("SETUP_CHILD_PAID_RESERVE", e)
	}
	if c.Barrier != "" {
		if e = os.WriteFile(c.ResultPath+".ready", []byte("paid native Open and Complete reserve"), 0600); e != nil {
			t.Fatal(e)
		}
		released := false
		until := time.Now().Add(10 * time.Second)
		for time.Now().Before(until) {
			if _, e := os.Stat(c.Barrier); e == nil {
				released = true
				break
			}
			time.Sleep(5 * time.Millisecond)
		}
		if !released {
			t.Fatal("SETUP_CHILD_RELEASE_TIMEOUT")
		}
	}
	r, e := CompleteDescribeV1(context.Background(), op, c.Submission)
	data, _ := json.Marshal(struct {
		Success bool
		Result  CompletedDescribeV1
	}{e == nil && r.ObjectID != "", r})
	if e = os.WriteFile(c.ResultPath, data, 0600); e != nil {
		t.Fatal(e)
	}
}
func TestCompleteDescribeExactPair(t *testing.T) {
	f := completeFixtureNativeV1(t)
	o := completeOwnerV1(t, f, "exact", -1)
	q := completeRequestV1(f, o, "exact")
	submission := completeSubmissionV1(t, f, "A")
	op, e := o.ReserveCompleteDescribeV1(context.Background(), q)
	if e != nil {
		t.Fatal(e)
	}
	expected := completeExpectedRawV1(t, op, submission)
	r, e := CompleteDescribeV1(context.Background(), op, submission)
	if e != nil || r.ObjectID != digest(expected) {
		t.Fatal("ASSERT_COMPLETE_EXACT_PAIR", e)
	}
	for _, out := range f.outputs {
		raw, e := out.Read(context.Background(), r.ObjectID, completeDescribeLimitV1)
		if e != nil || !bytes.Equal(raw, expected) {
			t.Fatal("ASSERT_COMPLETE_EXACT_PAIR", e)
		}
	}
	var obj CompleteDescribeObjectV1
	if json.Unmarshal(expected, &obj) != nil {
		t.Fatal("object")
	}
	if r.Authority != 0 || r.Accepted || r.FeatureIdentity != "UNRESOLVED" || r.Completeness != "UNKNOWN" || r.AbsenceInference != "PROHIBITED" || r.HOSTCost != "UNKNOWN_NOT_FREE" || obj.Authority != 0 || obj.Accepted || obj.Origin != dw.SuppliedTerminalResponseOnly || obj.Execution != dw.NoProcessOrHostExecutionVerified {
		t.Fatal("ASSERT_COMPLETE_CEILINGS")
	}
	if repeated, re := CompleteDescribeV1(context.Background(), op, submission); re == nil || repeated.ObjectID != "" {
		t.Fatal("opaque operation reused")
	}
	t.Log("ASSERT_COMPLETE_EXACT_PAIR: PASS; ASSERT_COMPLETE_CEILINGS: PASS")
	c := completeChildConfigForV1(t, f, "fresh-read")
	c.ReadID = r.ObjectID
	c.Expected = expected
	raw, _ := json.Marshal(c)
	path := filepath.Join(c.Root, "fresh-read-config.json")
	if e = os.WriteFile(path, raw, 0600); e != nil {
		t.Fatal(e)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestCompleteDescribeChild$", "-test.v", "-test.timeout=30s")
	cmd.Env = append(os.Environ(), "UNIT3_CHILD_CONFIG="+path)
	output, e := cmd.CombinedOutput()
	t.Logf("fresh independent read: %s", output)
	if e != nil || !bytes.Contains(output, []byte("ASSERT_COMPLETE_FRESH_READ: PASS")) {
		t.Fatal("fresh read", e)
	}
}
func TestCompleteDescribeObserveFailureConsumed(t *testing.T) {
	for _, fault := range []string{"precommit", "committed-error"} {
		t.Run(fault, func(t *testing.T) {
			f := completeFixtureNativeV1(t)
			o := completeOwnerV1(t, f, "observe-fault", -1)
			q := completeRequestV1(f, o, "observe-fault")
			op, e := o.ReserveCompleteDescribeV1(context.Background(), q)
			if e != nil {
				t.Fatal(e)
			}
			before, e := o.k.reopen(context.Background(), o.job, o.limits)
			if e != nil {
				t.Fatal(e)
			}
			reached := false
			o.k.cas = func(ctx context.Context, r *publication.Root, s string, p publication.BoundFilePredecessor, b []byte, v func([]byte) error) (*publication.CompareAndReplaceReceipt, error) {
				if s == currentSelectorV2(o.job) && bytes.Contains(b, []byte(`"event":"OBSERVE"`)) {
					reached = true
					if !completeSelectedExistsV1(t, f) || completeFilesV1(t, f) != [2]int{1, 1} {
						t.Fatal("SETUP_PAIR_NOT_BEFORE_OBSERVE")
					}
					if fault == "precommit" {
						return nil, errors.New("private test precommit Observe refusal")
					}
					receipt, e := publication.CompareAndReplaceBoundFile(ctx, r, s, p, b, v)
					if e != nil || receipt == nil || !receipt.Committed {
						t.Fatal("SETUP_OBSERVE_NOT_COMMITTED", e)
					}
					return receipt, errors.New("private test reported error AFTER real Observe commit")
				}
				return publication.CompareAndReplaceBoundFile(ctx, r, s, p, b, v)
			}
			result, e := CompleteDescribeV1(context.Background(), op, completeSubmissionV1(t, f, "A"))
			after, re := o.k.reopen(context.Background(), o.job, o.limits)
			if !reached || e == nil || result.ObjectID != "" || !o.uncertain || re != nil || before.Charged != after.Charged {
				t.Fatal("ASSERT_COMPLETE_OBSERVE_ZERO_NO_REFUND", fault, e, re)
			}
			if (after.Identity != before.Identity) != (fault == "committed-error") {
				t.Fatal("observe commit boundary", fault)
			}
			for _, label := range []string{"A", "B"} {
				next := completeOwnerV1(t, f, "after-observe-"+label, -1)
				r, e := completeExecuteV1(context.Background(), next, completeRequestV1(f, next, "after-observe-"+label), completeSubmissionV1(t, f, label))
				if e == nil || r.ObjectID != "" || completeFilesV1(t, f) != [2]int{1, 1} {
					t.Fatal("ASSERT_COMPLETE_OBSERVE_NO_REPAIR", label, e)
				}
			}
			t.Log("ASSERT_COMPLETE_OBSERVE_ZERO_NO_REFUND: PASS; ASSERT_COMPLETE_OBSERVE_NO_REPAIR: PASS", fault)
		})
	}
}
func TestCompleteDescribePartialConsumed(t *testing.T) {
	for _, which := range []int{0, 1} {
		t.Run(string(rune('A'+which)), func(t *testing.T) {
			f := completeFixtureNativeV1(t)
			o := completeOwnerV1(t, f, "partial", -1)
			q := completeRequestV1(f, o, "partial")
			supplied := completeSubmissionV1(t, f, "A")
			op, e := o.ReserveCompleteDescribeV1(context.Background(), q)
			if e != nil {
				t.Fatal(e)
			}
			raw := completeExpectedRawV1(t, op, supplied)
			id := digest(raw)
			if e = os.WriteFile(filepath.Join(f.paths[which], id), []byte("conflicting preexisting object"), 0600); e != nil {
				t.Fatal(e)
			}
			before, e := o.k.reopen(context.Background(), o.job, o.limits)
			if e != nil {
				t.Fatal(e)
			}
			out, e := CompleteDescribeV1(context.Background(), op, supplied)
			after, reopenErr := o.k.reopen(context.Background(), o.job, o.limits)
			if e == nil || out.ObjectID != "" || reopenErr != nil || after.Identity != before.Identity || after.Charged != before.Charged {
				t.Fatal("ASSERT_COMPLETE_PARTIAL_ZERO", e, reopenErr)
			}
			key, e := publication.ReadVerifiedBoundFileChecked(q.Root, completeDescribeSelectorV1(f.anchors), completeDescribeLimitV1)
			if e != nil {
				t.Fatal("selection absent", e)
			}
			if which == 1 {
				got, e := f.outputs[0].Read(context.Background(), id, int64(len(raw)))
				if e != nil || !bytes.Equal(got, raw) {
					t.Fatal("first publication not reached", e)
				}
			}
			counts := completeFilesV1(t, f)
			for _, label := range []string{"A", "B"} {
				next := completeOwnerV1(t, f, "later"+label, -1)
				result, e := completeExecuteV1(context.Background(), next, completeRequestV1(f, next, "later"+label), completeSubmissionV1(t, f, label))
				if e == nil || result.ObjectID != "" || completeFilesV1(t, f) != counts {
					t.Fatal("ASSERT_COMPLETE_PARTIAL_NO_REPAIR", label, e)
				}
			}
			final, e := publication.ReadVerifiedBoundFileChecked(q.Root, completeDescribeSelectorV1(f.anchors), completeDescribeLimitV1)
			if e != nil || !bytes.Equal(key, final) {
				t.Fatal("selection changed", e)
			}
			t.Log("ASSERT_COMPLETE_PARTIAL_ZERO: PASS; ASSERT_COMPLETE_PARTIAL_NO_REPAIR: PASS", which)
		})
	}
}
