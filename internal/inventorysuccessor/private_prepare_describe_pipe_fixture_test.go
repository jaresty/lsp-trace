//go:build lsp_trace_controlled_describe_fixture

package inventorysuccessor

import (
	"bytes"
	"context"
	"encoding/json"
	"lsp-trace/internal/programcadmission"
	"lsp-trace/internal/publication"
	"lsp-trace/sessionruntime"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func prepareDescribeNativeFixtureV1(t *testing.T, unfunded int) (PrepareDescribeRequestV1, *sessionruntime.ControlledDescribePipeFixture) {
	t.Helper()
	dir, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	if e = os.Chmod(dir, 0700); e != nil {
		t.Fatal(e)
	}
	fixture, e := sessionruntime.NewControlledDescribePipeFixture(dir)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if e := fixture.Close(ctx); e != nil {
			t.Error(e)
		}
	})
	root, e := publication.OpenRoot(dir)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		if e := root.Close(); e != nil {
			t.Error(e)
		}
	})
	j, a, d := fixtureV2(t)
	var amounts [14]uint64
	for i := range amounts {
		amounts[i] = 1e14
	}
	if unfunded >= 0 {
		amounts[unfunded] = 0
	}
	a.Ceiling = countersFromV2(amounts)
	a.MaxTranche = a.Ceiling
	a.MaxWindows = 100
	a.Expires = time.Now().Add(time.Hour).UnixNano()
	a.MaxWindowNS = uint64(15 * time.Second)
	sealTestV2(t, &a)
	d.Authorization = a.Identity
	k := &kernelV2{root: root}
	limits := verificationLimitsV2{4096, 64 << 20, 16384}
	initial, e := k.initialize(context.Background(), j, a, d, time.Now().UnixNano(), limits)
	if e != nil {
		t.Fatal(e)
	}
	owner, e := OpenPrivateManagedInputOwnerV2(context.Background(), root, sealTestV2(t, &j), initial.Identity, PrivateManagedInputVerificationV2{4096, 64 << 20, 16384})
	if e != nil {
		t.Fatal(e)
	}
	now := time.Now()
	deadline := now.Add(12 * time.Second)
	capture := PrivateMixedInputRequestV2{Manager: fixture.Manager(), Root: root, Start: fixture.StartRequest(deadline), URI: fixture.URI(), OccurrenceID: "unit2-query", Revision: "literal-revision", Privacy: "private-controlled", OccurrencePolicy: "ordinal", CompositionPolicy: "identity", Line: 1, Character: 5, Kinds: []programcadmission.RelationKind{programcadmission.TypedReferencesSymbol}}
	persist := PreparedDescribePersistRequestV1{Attempt: hashLabelV2("unit2-persist"), Nonce: hashLabelV2("unit2-nonce"), RootPath: dir, InitialHead: initial.Identity, Decision: a.Decision, Question: []byte("Explain Query"), Request: []byte("exact request"), ResponseSchema: []byte("exact schema"), Deadline: deadline, Expires: now.Add(5 * time.Minute), RetentionUntil: now.Add(10 * time.Minute), RecordLimit: 16 << 20}
	return PrepareDescribeRequestV1{Owner: owner, Capture: capture, CaptureAttempt: hashLabelV2("unit2-capture"), PreparationAttempt: hashLabelV2("unit2-preparation"), PacketAttempt: hashLabelV2("unit2-packet"), RunID: "unit2-run", SeedNamespace: "unit2-seed", Seed: 7, Persist: persist, PacketLimit: 1 << 20}, fixture
}

func TestPrepareDescribeV1Pipeline(t *testing.T) {
	q, fixture := prepareDescribeNativeFixtureV1(t, -1)
	staged, e := PrepareDescribeV1(context.Background(), q)
	if e != nil || len(staged.Packet) == 0 {
		t.Fatal("ASSERT_PREPARE_NATIVE_PIPELINE", e)
	}
	if fixture.Manager().Census().Workers != 0 || q.Capture.Root.ValidatePrivate() != nil {
		t.Fatal("borrowed lifecycle changed")
	}
	var packet PrepareDescribePacketV1
	if json.Unmarshal(staged.Packet, &packet) != nil || !bytes.Equal(packet.Question, q.Persist.Question) || !bytes.Equal(packet.Request, q.Persist.Request) || !bytes.Equal(packet.ResponseSchema, q.Persist.ResponseSchema) || string(packet.Definition) != "func Query() {}" || digest(packet.Question) != packet.QuestionSHA256 || digest(packet.Request) != packet.RequestSHA256 || digest(packet.ResponseSchema) != packet.ResponseSchemaSHA256 || digest(packet.Definition) != packet.DefinitionSHA256 || digest(packet.Provenance) != packet.ProvenanceSHA256 {
		t.Error("ASSERT_PREPARE_NATIVE_EXACT_PACKET")
	}
	if packet.Authority != 0 || packet.Accepted || packet.FeatureIdentity != "UNRESOLVED" || packet.Completeness != "UNKNOWN" || packet.AbsenceInference != "PROHIBITED" || packet.HOSTCost != "UNKNOWN_NOT_FREE" || packet.RoleLabels != [2]string{"SOURCE_PROVIDER_ATTRIBUTION_ONLY", "EVALUATOR_ATTRIBUTION_ONLY"} {
		t.Error("ASSERT_PREPARE_CEILINGS")
	} else {
		t.Log("ASSERT_PREPARE_CEILINGS: PASS")
	}
	f := preparedDescribeTestFixtureV1{owner: q.Owner, initial: q.Persist.InitialHead, q: q.Persist}
	completion := preparedDescribeFixtureCompletionV1(t, f, staged.Continuation)
	config := preparedDescribeTestConfigV1{sealTestV2(t, &completion.job), completion.head, staged.Continuation, q.Persist.Expires, packet.Provenance}
	raw, e := json.Marshal(config)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(q.Persist.RootPath, "pipe-open-config.json"), raw, 0600); e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	child := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestPreparedDescribePipeFreshChild$", "-test.v", "-test.timeout=15s")
	child.Env = append(os.Environ(), "PREPARED_DESCRIBE_PIPE_OPEN_ROOT="+q.Persist.RootPath)
	output, e := child.CombinedOutput()
	if e != nil || !bytes.Contains(output, []byte("ASSERT_PREPARED_PIPE_FRESH_OPEN: PASS")) {
		t.Fatalf("ASSERT_PREPARE_NATIVE_FRESH_OPEN %v %s", e, output)
	}
	t.Logf("fresh child:\n%s", output)
	if !t.Failed() {
		t.Log("ASSERT_PREPARE_NATIVE_PIPELINE: PASS; ASSERT_PREPARE_NATIVE_EXACT_PACKET: PASS; ASSERT_PREPARE_NATIVE_FRESH_OPEN: PASS")
	}
}
func TestPrepareDescribeV1Unfunded(t *testing.T) {
	for coordinate := 0; coordinate < 14; coordinate++ {
		t.Run(string(rune('A'+coordinate)), func(t *testing.T) {
			q, fixture := prepareDescribeNativeFixtureV1(t, coordinate)
			before, e := q.Owner.k.reopen(context.Background(), q.Owner.job, q.Owner.limits)
			if e != nil {
				t.Fatal(e)
			}
			out, e := PrepareDescribeV1(context.Background(), q)
			after, reopenErr := q.Owner.k.reopen(context.Background(), q.Owner.job, q.Owner.limits)
			if e == nil || len(out.Packet) != 0 || reopenErr != nil || before.Charged != after.Charged || before.Windows != after.Windows || fixture.Manager().Census().Workers != 0 || fixture.Manager().Census().Sessions != 0 {
				t.Fatal("ASSERT_PREPARE_UNFUNDED_NO_CAPTURE", coordinate, e, reopenErr)
			}
			t.Log("ASSERT_PREPARE_UNFUNDED_NO_CAPTURE: PASS", coordinate)
		})
	}
}
func TestPrepareDescribeV1Boundaries(t *testing.T) {
	for _, fault := range []string{"cancel", "expired", "wrong-root", "wrong-initial", "wrong-kind", "extended-deadline", "packet-reject"} {
		t.Run(fault, func(t *testing.T) {
			q, fixture := prepareDescribeNativeFixtureV1(t, -1)
			ctx := context.Background()
			switch fault {
			case "cancel":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			case "expired":
				q.Persist.Deadline = time.Now().Add(-time.Second)
				q.Capture.Start.Deadline = q.Persist.Deadline
			case "wrong-root":
				q.Persist.RootPath += "/foreign"
			case "wrong-initial":
				q.Persist.InitialHead = hashLabelV2("foreign")
			case "wrong-kind":
				q.Capture.Kinds = []programcadmission.RelationKind{programcadmission.RelationKind("RESOLVES_TO_DEFINITION")}
			case "extended-deadline":
				q.Persist.Deadline = q.Capture.Start.Deadline.Add(time.Second)
			case "packet-reject":
				q.PacketLimit = 1
			}
			before, e := q.Owner.k.reopen(context.Background(), q.Owner.job, q.Owner.limits)
			if e != nil {
				t.Fatal(e)
			}
			out, e := PrepareDescribeV1(ctx, q)
			after, reopenErr := q.Owner.k.reopen(context.Background(), q.Owner.job, q.Owner.limits)
			if e == nil || len(out.Packet) != 0 || out.Continuation != (PreparedDescribeAnchorsV1{}) || reopenErr != nil || fixture.Manager().Census().Workers != 0 {
				t.Fatal("ASSERT_PREPARE_BOUNDARY_ZERO", fault, e, reopenErr)
			}
			if fault == "packet-reject" {
				if after.Windows <= before.Windows || after.Charged == before.Charged {
					t.Fatal("failed encoding refunded")
				}
			} else {
				if after.Windows != before.Windows || after.Charged != before.Charged {
					t.Fatal("preflight effects")
				}
			}
			t.Log("ASSERT_PREPARE_BOUNDARY_ZERO: PASS", fault)
		})
	}
}
