//go:build lsp_trace_controlled_describe_fixture

package inventorysuccessor

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"lsp-trace/internal/programcadmission"
	"lsp-trace/internal/publication"
	"lsp-trace/sessionruntime"
)

// All positive custody comes from the actual mixed producer and R preparation.
// The fixed adapter supplies only literal pipe traffic, not source authority.
func preparedDescribePipeFixtureV1(t *testing.T) (preparedDescribeTestFixtureV1, PrivateSourceRPreparationOperationV1, PreparedSourceRV1) {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	fixture, err := sessionruntime.NewControlledDescribePipeFixture(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := fixture.Close(ctx); err != nil {
			t.Error("fixture cleanup", err)
		}
	})
	owner, initial := preparedDescribeFixtureOwnerV1(t, dir)
	deadline := time.Now().Add(12 * time.Second)
	q := PrivateMixedInputRequestV2{Manager: fixture.Manager(), Root: owner.k.root, Start: fixture.StartRequest(deadline), URI: fixture.URI(), OccurrenceID: "Describe-literal-query", Revision: "literal-revision", Privacy: "private-controlled", OccurrencePolicy: "ordinal", CompositionPolicy: "identity", Line: 1, Character: 5, Kinds: []programcadmission.RelationKind{programcadmission.TypedReferencesSymbol}}
	mixed, err := owner.ReserveMixedInputV2(context.Background(), hashLabelV2("pipe-capture"), q)
	if err != nil {
		t.Fatal("SETUP_MIXED_RESERVE", err)
	}
	input, err := ExecuteMixedInputV2(context.Background(), mixed, q)
	if err != nil {
		t.Fatal("SETUP_MIXED_EXECUTE", err)
	}
	if input.replayBinding.Mode != "REPLAY" || !input.sourceConfirmed || input.sourceEvidence.SourceByteLength() == 0 || fixture.Manager().Census().Workers != 0 {
		t.Fatal("ASSERT_PIPE_NATIVE_COMPLETION")
	}
	t.Log("ASSERT_PIPE_NATIVE_COMPLETION: PASS")
	pq := PrivateSourceRPreparationRequestV1{Input: input, Root: owner.k.root, ExpectedHead: owner.head, Attempt: hashLabelV2("pipe-preparation"), Workspace: dir, Revision: q.Revision, RunID: "pipe-run", SeedNamespace: "pipe-seed", Seed: 7, Deadline: deadline}
	prep, err := owner.ReserveRPreparationV1(context.Background(), pq)
	if err != nil {
		t.Fatal("SETUP_R_RESERVE", err)
	}
	prepared, err := ExecuteRPreparationV1(context.Background(), prep, pq)
	if err != nil {
		t.Fatal("SETUP_R_EXECUTE", err)
	}
	now := time.Now()
	request := PreparedDescribePersistRequestV1{Attempt: hashLabelV2("pipe-persistence"), Nonce: hashLabelV2("pipe-nonce"), RootPath: dir, InitialHead: initial, Decision: hashLabelV2("decision"), Question: []byte("Explain Query"), Request: []byte("fixed synthetic request"), ResponseSchema: []byte("fixed synthetic schema"), Deadline: deadline, Expires: now.Add(5 * time.Minute), RetentionUntil: now.Add(10 * time.Minute), RecordLimit: 16 << 20}
	return preparedDescribeTestFixtureV1{owner: owner, initial: initial, q: request}, prep, prepared
}

func TestPreparedDescribePipePublicReplay(t *testing.T) {
	f, prep, prepared := preparedDescribePipeFixtureV1(t)
	op, err := f.owner.ReservePreparedDescribeV1(context.Background(), prep, prepared, f.q)
	if err != nil || op.state == nil {
		t.Fatal("ASSERT_PREPARED_PUBLIC_REPLAY", err)
	}
	t.Log("ASSERT_PREPARED_PUBLIC_REPLAY: PASS")
	persisted, err := CommitPreparedDescribeV1(context.Background(), op)
	if err != nil {
		t.Fatal("public Commit", err)
	}
	completion := preparedDescribeFixtureCompletionV1(t, f, persisted.Anchors())
	opening, err := completion.ReservePreparedDescribeOpenV1(context.Background(), preparedDescribeOpenRequestV1(f, persisted.Anchors()))
	if err != nil {
		t.Fatal(err)
	}
	opened, err := OpenPreparedDescribeV1(context.Background(), opening)
	if err != nil || string(opened.Definition()) != "func Query() {}" {
		t.Fatal("ASSERT_PREPARED_PIPE_EXACT_OPEN", err, string(opened.Definition()))
	}
	t.Log("ASSERT_PREPARED_PIPE_EXACT_OPEN: PASS")
	config := preparedDescribeTestConfigV1{sealTestV2(t, &completion.job), completion.head, persisted.Anchors(), f.q.Expires, opened.Provenance()}
	raw, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(f.q.RootPath, "pipe-open-config.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	childCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(childCtx, os.Args[0], "-test.run=^TestPreparedDescribePipeFreshChild$", "-test.v", "-test.timeout=15s")
	cmd.Env = append(os.Environ(), "PREPARED_DESCRIBE_PIPE_OPEN_ROOT="+f.q.RootPath)
	output, err := cmd.CombinedOutput()
	if err != nil || !bytes.Contains(output, []byte("ASSERT_PREPARED_PIPE_FRESH_OPEN: PASS")) {
		t.Fatalf("ASSERT_PREPARED_PIPE_FRESH_OPEN: %v %s", err, output)
	}
	t.Logf("fresh child output:\n%s", output)
	t.Log("ASSERT_PREPARED_PIPE_FRESH_OPEN: PASS")
}

func TestPreparedDescribePipeFreshChild(t *testing.T) {
	dir := os.Getenv("PREPARED_DESCRIBE_PIPE_OPEN_ROOT")
	if dir == "" {
		t.Skip("fresh-process helper")
	}
	raw, err := os.ReadFile(filepath.Join(dir, "pipe-open-config.json"))
	if err != nil {
		t.Fatal(err)
	}
	var config preparedDescribeTestConfigV1
	if err = json.Unmarshal(raw, &config); err != nil {
		t.Fatal(err)
	}
	root, err := publication.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := root.Close(); err != nil {
			t.Error(err)
		}
	}()
	owner, err := OpenPrivateManagedInputOwnerV2(context.Background(), root, config.Job, config.Head, PrivateManagedInputVerificationV2{4096, 64 << 20, 16384})
	if err != nil {
		t.Fatal(err)
	}
	op, err := owner.ReservePreparedDescribeOpenV1(context.Background(), PreparedDescribeOpenRequestV1{Attempt: hashLabelV2("pipe-fresh-open"), Decision: hashLabelV2("decision"), Anchors: config.Anchors, Deadline: time.Now().Add(10 * time.Second), Expires: config.Expires, RecordLimit: 16 << 20})
	if err != nil {
		t.Fatal(err)
	}
	opened, err := OpenPreparedDescribeV1(context.Background(), op)
	if err != nil || string(opened.Definition()) != "func Query() {}" || !bytes.Equal(opened.Provenance(), config.Provenance) {
		t.Fatal("ASSERT_PREPARED_PIPE_FRESH_OPEN", err)
	}
	t.Log("ASSERT_PREPARED_PIPE_FRESH_OPEN: PASS")
}

func TestPreparedDescribePipeCanceledProducerNoGrant(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	fixture, err := sessionruntime.NewControlledDescribePipeFixture(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := fixture.Close(ctx); err != nil {
			t.Error(err)
		}
	})
	owner, _ := preparedDescribeFixtureOwnerV1(t, dir)
	deadline := time.Now().Add(10 * time.Second)
	q := PrivateMixedInputRequestV2{Manager: fixture.Manager(), Root: owner.k.root, Start: fixture.StartRequest(deadline), URI: fixture.URI(), OccurrenceID: "canceled-query", Revision: "literal-revision", Privacy: "private-controlled", OccurrencePolicy: "ordinal", CompositionPolicy: "identity", Line: 1, Character: 5, Kinds: []programcadmission.RelationKind{programcadmission.TypedReferencesSymbol}}
	op, err := owner.ReserveMixedInputV2(context.Background(), hashLabelV2("canceled-pipe-capture"), q)
	if err != nil {
		t.Fatal(err)
	}
	before, err := owner.k.reopen(context.Background(), owner.job, owner.limits)
	if err != nil {
		t.Fatal(err)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	input, err := ExecuteMixedInputV2(canceled, op, q)
	if err == nil || input.sourceConfirmed || input.sourceEvidence.SourceByteLength() != 0 {
		t.Fatal("ASSERT_PIPE_CANCELED_PRODUCER_ZERO", err)
	}
	prep, err := owner.ReserveRPreparationV1(context.Background(), PrivateSourceRPreparationRequestV1{Input: input, Root: owner.k.root, ExpectedHead: owner.head, Attempt: hashLabelV2("canceled-prep"), Workspace: dir, Revision: q.Revision, RunID: "canceled-run", SeedNamespace: "pipe-seed", Seed: 7, Deadline: deadline})
	if err == nil || prep.state != nil {
		t.Fatal("canceled producer admitted R preparation")
	}
	after, err := owner.k.reopen(context.Background(), owner.job, owner.limits)
	if err != nil || before.Charged != after.Charged || before.Windows != after.Windows {
		t.Fatal("canceled charge changed", err)
	}
	t.Log("ASSERT_PIPE_CANCELED_PRODUCER_ZERO: PASS; no source/R authority; charges retained")
}

func TestPreparedDescribePipePublicRejects(t *testing.T) {
	for _, fault := range []string{"ACQUIRE", "local", "foreign", "foreign-source", "query-mismatch"} {
		t.Run(fault, func(t *testing.T) {
			f, prep, prepared := preparedDescribePipeFixtureV1(t)
			q := prep.state.request
			metadata := *prepared.state
			switch fault {
			case "ACQUIRE":
				q.Input.replayBinding.Mode = "ACQUIRE"
				metadata.input.replayBinding.Mode = "ACQUIRE"
			case "local":
				q.Local = &PrivateSourceLocalConfigV1{}
			case "foreign-source":
				_, foreign, _ := preparedDescribePipeFixtureV1(t)
				q.Input.sourceEvidence = foreign.state.request.Input.sourceEvidence
			case "query-mismatch":
				q.Input.replayBinding.URI += ".foreign"
			}
			// Negative-only corruptions of existing opaque values; never used to mint
			// a successful source, completion, R preparation or Describe result.
			bad := PrivateSourceRPreparationOperationV1{state: &privateSourceRPreparationStateV1{owner: f.owner, used: true, request: q, head: prep.state.head}}
			badPrepared := PreparedSourceRV1{state: &metadata}
			if fault == "foreign" {
				_, bad, badPrepared = preparedDescribePipeFixtureV1(t)
			}
			before, err := f.owner.k.reopen(context.Background(), f.owner.job, f.owner.limits)
			if err != nil {
				t.Fatal(err)
			}
			op, err := f.owner.ReservePreparedDescribeV1(context.Background(), bad, badPrepared, f.q)
			after, e := f.owner.k.reopen(context.Background(), f.owner.job, f.owner.limits)
			if err == nil || op.state != nil || e != nil || after.Identity != before.Identity || after.Charged != before.Charged {
				t.Fatal("ASSERT_PREPARED_PIPE_PUBLIC_REJECT", fault, err, e)
			}
			t.Log("ASSERT_PREPARED_PIPE_PUBLIC_REJECT: PASS", fault)
		})
	}
}
