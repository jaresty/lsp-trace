package inventoryworkflow

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"lsp-trace/internal/censuscontinuation"
	"lsp-trace/internal/inventorysuccessor"
	p "lsp-trace/internal/programcrepresentative"
	"lsp-trace/internal/programctransitionhost"
)

// This opt-in harness is bound to one genuinely reviewed CONTROLLED subject.
// It does not acquire application evidence or create a replacement corpus.
// Parent runs each mode in a separate Go process after independently reviewing
// this source. No mode retries an uncertain invocation or chains the three CASes.
const genuineEvidenceV2 = "/Users/schwa/.local/state/lsp-trace-evidence/multiseed-genuine-qualification-513puh_b"
const genuineCorpusV2 = genuineEvidenceV2 + "/corpus-parent-subject-v1"
const genuineNativeV2 = genuineEvidenceV2 + "/native-review-evidence"
const genuineExecutionV2 = genuineEvidenceV2 + "/execution-evidence"

func genuineSubjectBindingV2() p.ArtifactBindingDTO {
	return p.ArtifactBindingDTO{SchemaVersion: p.QualificationSubjectSchemaVersionV2, Identity: "sha256:c122e54294879172ae81c7b556309d3f049ab48ea8ca8d011211dee76fe7afb2", SHA256: "sha256:38d6b1b692d10e398acb13973a87fa259281bb3044a8ef1e641027c2d7160323", ByteLength: 14600}
}
func genuineScopeV2() []string {
	return []string{"SUBJECT_CANONICAL_BYTES", "SUBJECT_EXPECTED_CONTEXT_RECONSTRUCTION", "IMPLEMENTATION_SEAM_MANIFEST", "STAGE6_POLICY", "STAGE6_LIMITS", "STAGE6_FINALIZATION", "CORPUS_AND_WORKFLOW_BINDINGS", "COMPLETE_RECEIPT_INVENTORY", "TERMINAL_PROGRAM_C_HEAD", "CLAIM_CEILINGS", "ACYCLICITY", "MULTISEED_ORDERED_BIJECTION", "GROUP_LOCAL_TERMINALS", "D_AND_G_CONSERVATION", "BOUNDED_TYPED_SIZE_ADMISSION"}
}
func genuineDecodeV2(raw []byte, dst any, canonical bool) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(dst); err != nil {
		return err
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("GENUINE_SINGLE_JSON_VALUE")
	}
	if canonical {
		again, err := json.Marshal(dst)
		if err != nil || !bytes.Equal(raw, again) {
			return fmt.Errorf("GENUINE_CANONICAL_JSON")
		}
	}
	return nil
}

// Pure check: no retention, owner capability, or namespace access is possible.
func genuineProjectionV2(raw []byte, b p.ReviewedHarnessActorBindingV2, subject p.ArtifactBindingDTO, verdict string) (p.ReviewAssignmentV2, p.ReviewTerminalV2, error) {
	var a p.ReviewAssignmentV2
	var z p.ReviewTerminalV2
	fail := func() (p.ReviewAssignmentV2, p.ReviewTerminalV2, error) {
		return a, z, fmt.Errorf("GENUINE_PROJECTION_BEFORE_WRITES")
	}
	if b.Actor.Projection != nil || b.Projection.SchemaVersion != "lsp-trace.program-c-representative-review-transcript-jsonl.private.v2" || b.Projection.Identity != "" || p.VerifyRawArtifactBindingV2(b.Projection, raw) != nil {
		return fail()
	}
	lines := bytes.Split(raw, []byte{'\n'})
	if len(lines) != 3 || len(lines[0]) == 0 || len(lines[1]) == 0 || len(lines[2]) != 0 {
		return fail()
	}
	if genuineDecodeV2(lines[0], &a, true) != nil || genuineDecodeV2(lines[1], &z, true) != nil {
		return fail()
	}
	if a.SchemaVersion != "lsp-trace.program-c-representative-review-assignment.private.v2" || z.SchemaVersion != "lsp-trace.program-c-representative-review-terminal.private.v2" || a.ActorKind != b.Actor.Kind || z.ActorKind != b.Actor.Kind || a.HarnessActorID != b.Actor.ID || z.HarnessActorID != b.Actor.ID || a.ParentActorID != b.Actor.ParentID || z.ParentActorID != b.Actor.ParentID || b.Actor.IsolationKind != b.Actor.Kind || a.Subject != subject || z.Subject != subject || z.Verdict != verdict || !reflect.DeepEqual(a.Scope, genuineScopeV2()) || !reflect.DeepEqual(z.Scope, genuineScopeV2()) {
		return fail()
	}
	return a, z, nil
}

type genuineDerivationActorV2 struct {
	Role           string               `json:"role"`
	Kind           string               `json:"kind"`
	ID             string               `json:"id"`
	ParentID       string               `json:"parent_id"`
	IsolationKind  string               `json:"isolation_kind"`
	Projection     p.ArtifactBindingDTO `json:"projection"`
	ProjectionPath string               `json:"projection_path"`
}
type genuineDerivationV2 struct {
	Schema           string                     `json:"schema"`
	Subject          p.ArtifactBindingDTO       `json:"subject"`
	Scope            []string                   `json:"scope"`
	Writer           string                     `json:"writer_session"`
	Reviewer         string                     `json:"reviewer_task"`
	Parent           string                     `json:"reviewer_parent_session"`
	Isolated         bool                       `json:"native_launch_isolated"`
	Inherit          bool                       `json:"native_launch_inherit_context"`
	WriterAssignment string                     `json:"native_writer_assignment_event"`
	WriterTerminal   string                     `json:"native_writer_terminal_event"`
	ReviewerTerminal string                     `json:"native_reviewer_terminal_event"`
	Actors           []genuineDerivationActorV2 `json:"actors"`
	Note             string                     `json:"note"`
	Issued           bool                       `json:"qualification_envelope_issued"`
}
type genuineInputsV2 struct {
	descriptor  preparedQualificationV2
	request     Request
	source      preparedSourceReaderV2
	subject     p.QualificationSubjectV2
	subjectRaw  []byte
	actors      []p.ReviewedHarnessActorBindingV2
	projections [][]byte
}

func genuineLoadV2(t *testing.T, baseline bool) genuineInputsV2 {
	t.Helper()
	var in genuineInputsV2
	for _, dir := range []string{genuineCorpusV2, genuineNativeV2} {
		if err := preparedCanonicalDirectoryV2(dir); err != nil {
			t.Fatal(err)
		}
	}
	raw, err := preparedReadV2(genuineCorpusV2, "descriptor.json", 1048576, false)
	if err != nil {
		t.Fatal(err)
	}
	if workflowDigest(raw) != "sha256:07d22d6b5aadfb55ee87591dbe9d5008373b1822ea88d77ac88ce3a72c639e9b" {
		t.Fatal("GENUINE_EXACT_REVIEWED_DESCRIPTOR")
	}
	if err = genuineDecodeV2(raw, &in.descriptor, true); err != nil {
		t.Fatal(err)
	}
	d := in.descriptor
	if d.Kind != "CONTROLLED_GRAPH_INPUT_QUALIFICATION_PREPARATION_NOT_REVIEWED" || d.CorpusRoot != genuineCorpusV2 || d.PublicationRoot != genuineCorpusV2+"/publication" || d.StateRoot != genuineCorpusV2+"/state" || d.RetainedSourceRoot != genuineCorpusV2+"/source" || d.Subject != genuineSubjectBindingV2() || len(d.Files) != 55 || d.Receipt.Path != "publication/controlled-receipt.json" || d.Ledger.Path != "publication/controlled-ledger.json" {
		t.Fatal("GENUINE_FIXED_DESCRIPTOR")
	}
	// Before first retention the original closed 55-file inventory is immutable.
	// Afterwards only the exact workflow slot may have changed among these files;
	// newly published artifacts are validated by the actual production owner.
	if baseline {
		files, e := preparedInventoryV2(genuineCorpusV2, false)
		if e != nil || !reflect.DeepEqual(files, d.Files) {
			t.Fatalf("GENUINE_BASELINE_INVENTORY: %v", e)
		}
	} else {
		for _, pin := range d.Files {
			if pin.Path == "state/inventory-successor/heads/private/v1/17748e339c492a34c99accd979e15f41b77977787e1f0f3f71c9eaf448c348eb.json" {
				continue
			}
			body, e := preparedReadV2(genuineCorpusV2, pin.Path, pin.ByteLength, true)
			if e != nil || workflowDigest(body) != pin.SHA256 {
				t.Fatalf("GENUINE_IMMUTABLE_INPUT: %s %v", pin.Path, e)
			}
		}
	}
	manifestRaw, err := preparedReadV2(genuineCorpusV2, "implementation-manifest.json", 65536, false)
	if err != nil {
		t.Fatal(err)
	}
	m, err := preparedManifestV2(manifestRaw)
	if err != nil {
		t.Fatal(err)
	}
	in.source = preparedSourceV2(d.RetainedSourceRoot, m)
	if err = p.VerifyStage6ImplementationSourceV2(m, in.source, 237568); err != nil {
		t.Fatal(err)
	}
	in.subjectRaw, err = preparedReadV2(genuineCorpusV2, "subject.json", 14600, true)
	if err != nil {
		t.Fatal(err)
	}
	if err = p.VerifyRawArtifactBindingV2(d.Subject, in.subjectRaw); err != nil {
		t.Fatal(err)
	}
	parsed, err := p.ParseProgramCRepresentativeQualificationSubjectV2(in.subjectRaw)
	if err != nil || parsed.Trusted() {
		t.Fatalf("GENUINE_STRICT_UNTRUSTED_SUBJECT: %v", err)
	}
	in.subject, err = parsed.DTO()
	if err != nil || parsed.Identity() != d.Subject.Identity || !reflect.DeepEqual(in.subject.Stage6Implementation, m) {
		t.Fatalf("GENUINE_SUBJECT_MANIFEST: %v", err)
	}
	input := func(pin preparedInputV2) censuscontinuation.ExactProgramCTransitionInput {
		body, e := preparedReadV2(genuineCorpusV2, pin.Path, pin.ByteLength, true)
		if e != nil || workflowDigest(body) != pin.SHA256 {
			t.Fatalf("GENUINE_REQUEST_PIN: %v", e)
		}
		return censuscontinuation.ExactProgramCTransitionInput{Reader: exactFileReader{path: filepath.Join(genuineCorpusV2, filepath.FromSlash(pin.Path))}, ByteLength: pin.ByteLength, SHA256: pin.SHA256}
	}
	opens := 0
	in.request = productionWorkflowRequest(t, input(d.Receipt), input(d.Ledger), d.StateRoot, 8, &opens)
	if !reflect.DeepEqual(preparedRequestMetadataOfV2(in.request), d.Request) {
		t.Fatal("GENUINE_REQUEST_IDENTITY_EQUALITY")
	}
	expectedRaw, e := preparedReadV2(genuineCorpusV2, "expected-context.json", 1048576, false)
	expectedBytes, je := json.Marshal(preparedExpectedOfV2(in.subject))
	if e != nil || je != nil || !bytes.Equal(expectedRaw, expectedBytes) {
		t.Fatal("GENUINE_EXPECTED_CONTEXT")
	}
	_, pb, pr, e := p.BuildStage6PolicyArtifactV2()
	if e != nil {
		t.Fatal(e)
	}
	_, lb, lr, e := p.BuildStage6LimitsArtifactV2()
	if e != nil {
		t.Fatal(e)
	}
	_, fb, fr, e := p.BuildStage6FinalizationArtifactV2()
	if e != nil {
		t.Fatal(e)
	}
	for i, item := range []struct {
		name string
		b    p.ArtifactBindingDTO
		raw  []byte
	}{{"policy.json", pb, pr}, {"limits.json", lb, lr}, {"finalization.json", fb, fr}} {
		body, e := preparedReadV2(genuineCorpusV2, item.name, int64(len(item.raw)), true)
		if e != nil || !bytes.Equal(body, item.raw) || []p.ArtifactBindingDTO{in.subject.Stage6Policy, in.subject.Stage6Limits, in.subject.Stage6Finalization}[i] != item.b {
			t.Fatal("GENUINE_FIXED_ARTIFACT")
		}
	}
	in.actors, err = p.ReviewedHarnessActorBindingsV2()
	if err != nil || len(in.actors) != 2 {
		t.Fatalf("GENUINE_NATIVE_REGISTRY: %v", err)
	}
	derivationRaw, err := preparedReadV2(genuineNativeV2, "PROJECTION-DERIVATION.json", 16384, false)
	if err != nil {
		t.Fatal(err)
	}
	var deriv genuineDerivationV2
	if err = genuineDecodeV2(derivationRaw, &deriv, false); err != nil {
		t.Fatal(err)
	}
	if deriv.Schema != "native-subject-review-projection-derivation.v1" || deriv.Subject != d.Subject || !reflect.DeepEqual(deriv.Scope, genuineScopeV2()) || deriv.Writer != "01a1070d-1392-7797-8214-2c7836f6d572" || deriv.Reviewer != "cf12200b-22aa-40e" || deriv.Parent != deriv.Writer || !deriv.Isolated || deriv.Inherit || deriv.Issued || len(deriv.Actors) != 2 || deriv.WriterAssignment != "d0dea19691dd8bd9dfc05c02ee6da22d69c54195b29486ce5dcf84382d7d6db0" || deriv.WriterTerminal != "daccb311e85402ab030fdbaa5dca844add83573ffe7b0802b3920a57402d14bd" || deriv.ReviewerTerminal != "bb59ce3dc2ce79d70178648ff1ddcbd4d170186f7d96af1a85123d06669da44f" {
		t.Fatal("GENUINE_NATIVE_DERIVATION")
	}
	for i, name := range []string{"writer.projection.jsonl", "reviewer.projection.jsonl"} {
		b := in.actors[i]
		a := deriv.Actors[i]
		role := []string{"writer", "reviewer"}[i]
		if a.Role != role || a.Kind != b.Actor.Kind || a.ID != b.Actor.ID || a.ParentID != b.Actor.ParentID || a.IsolationKind != b.Actor.IsolationKind || a.Projection != b.Projection || a.ProjectionPath != genuineNativeV2+"/"+name {
			t.Fatal("GENUINE_REGISTRY_NATIVE_MATCH")
		}
		projection, e := preparedReadV2(genuineNativeV2, name, int64(b.Projection.ByteLength), true)
		if e != nil {
			t.Fatal(e)
		}
		verdict := []string{"SUBJECT_PRODUCED", "ACCEPT"}[i]
		if _, _, e = genuineProjectionV2(projection, b, d.Subject, verdict); e != nil {
			t.Fatal(e)
		}
		in.projections = append(in.projections, projection)
	}
	if baseline {
		// This production call is read-only and reconstructs all expected context.
		rebuilt, e := BuildProgramCMultiSeedQualificationSubjectV2(context.Background(), in.request, in.source, 237568)
		if e != nil {
			t.Fatal(e)
		}
		body, e := rebuilt.CanonicalBytes()
		if e != nil || !rebuilt.Trusted() || !bytes.Equal(body, in.subjectRaw) {
			t.Fatal("GENUINE_RECONSTRUCTED_EXACT_SUBJECT")
		}
		genuineAccountingV2(t, in)
	}
	return in
}
func genuineAccountingV2(t *testing.T, in genuineInputsV2) {
	t.Helper()
	root, err := in.request.ProgramC.OpenState()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if e := root.Close(); e != nil {
			t.Fatal(e)
		}
	}()
	verification, err := programctransitionhost.NewCompletionVerificationRequest(context.Background(), CompletionRequest{StateRoot: root, Receipt: in.request.ProgramC.Receipt, Ledger: in.request.ProgramC.Ledger, WorkflowKey: in.request.ProgramC.WorkflowKey, HeadKey: in.request.ProgramC.HeadKey, Bindings: in.request.ProgramC.Bindings})
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := censuscontinuation.BuildProgramCMultiSeedQualificationEvidenceV2(context.Background(), verification)
	if err != nil {
		t.Fatal(err)
	}
	expected, err := evidence.QualificationSubjectExpectedContextV2()
	if err != nil {
		t.Fatal(err)
	}
	d, g := assertMultiSeedRetainedAccountingV2(t, evidence, expected)
	if d != 24 || g != 48 {
		t.Fatal("GENUINE_D24_G48_24_RUNS")
	}
}

type genuineReportV2 struct {
	Kind           string               `json:"kind"`
	Subject        p.ArtifactBindingDTO `json:"subject"`
	Envelope       p.ArtifactBindingDTO `json:"envelope"`
	EnvelopeBytes  []byte               `json:"envelope_bytes"`
	PriorSuccessor []byte               `json:"prior_successor"`
}

func genuineRetainV2(t *testing.T, in genuineInputsV2) {
	t.Helper()
	ctx := context.Background()
	// The new directory is also a sticky attempt boundary: a partial retention is
	// not silently retried. Orphan reuse is never a substitute for fresh review.
	if err := os.Mkdir(genuineExecutionV2, 0700); err != nil {
		t.Fatalf("GENUINE_NEW_RETENTION_ATTEMPT_ONLY: %v", err)
	}
	preparedWriteV2(t, genuineExecutionV2, "retain.attempt", []byte("one parent-selected retention attempt; no automatic retry\n"))
	before := checkpointFilesV2(t, in.descriptor.StateRoot)
	root, err := in.request.ProgramC.OpenState()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if e := root.Close(); e != nil {
			t.Fatal(e)
		}
	}()
	objects, err := inventorysuccessor.NewMultiSeedObjectsV2(root)
	if err != nil {
		t.Fatal(err)
	}
	heads, err := inventorysuccessor.NewMultiSeedSuccessorHeadsV2(root)
	if err != nil {
		t.Fatal(err)
	}
	prior, err := heads.Load(ctx, in.request.SuccessorHeadKey)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = inventorysuccessor.ParseHeadRevision(prior); err != nil {
		t.Fatal(err)
	}
	retain := func(b p.ArtifactBindingDTO, raw []byte) {
		if e := objects.Retain(ctx, b, raw); e != nil {
			t.Fatal(e)
		}
		body, e := objects.Resolve(ctx, b)
		if e != nil || !bytes.Equal(raw, body) {
			t.Fatalf("GENUINE_RETAIN_READBACK: %v", e)
		}
	}
	retain(in.descriptor.Subject, in.subjectRaw)
	_, pb, pr, err := p.BuildStage6PolicyArtifactV2()
	if err != nil {
		t.Fatal(err)
	}
	retain(pb, pr)
	_, lb, lr, err := p.BuildStage6LimitsArtifactV2()
	if err != nil {
		t.Fatal(err)
	}
	retain(lb, lr)
	_, fb, fr, err := p.BuildStage6FinalizationArtifactV2()
	if err != nil {
		t.Fatal(err)
	}
	retain(fb, fr)
	actors := make([]p.ReviewActorV2, 0, 2)
	for i, b := range in.actors {
		projection := in.projections[i]
		verdict := []string{"SUBJECT_PRODUCED", "ACCEPT"}[i]
		a, z, e := genuineProjectionV2(projection, b, in.descriptor.Subject, verdict)
		if e != nil {
			t.Fatal(e)
		}
		retain(b.Projection, projection)
		_, binding, raw, e := p.EncodeReviewTranscriptSnapshotV2(p.ReviewTranscriptSnapshotV2{SchemaVersion: "lsp-trace.program-c-representative-review-transcript-snapshot.private.v2", ActorKind: b.Actor.Kind, HarnessActorID: b.Actor.ID, ParentActorID: b.Actor.ParentID, RawTranscriptSelector: b.Projection.SHA256, RawTranscriptSHA256: b.Projection.SHA256, RawTranscriptByteLength: b.Projection.ByteLength, AssignmentSubject: a.Subject, AssignmentScope: a.Scope, TerminalSubject: z.Subject, TerminalScope: z.Scope, TerminalVerdict: z.Verdict})
		if e != nil {
			t.Fatal(e)
		}
		retain(binding, raw)
		actor := p.ReviewActorV2{IdentityKind: b.Actor.Kind, HarnessActorID: b.Actor.ID, AttributionEvidence: binding}
		size, e := p.CanonicalSizeReviewActorV2(actor)
		if e != nil {
			t.Fatal(e)
		}
		pre, e := json.Marshal(actor)
		if e != nil || uint64(len(pre)) != size {
			t.Fatal("GENUINE_ACTOR_CANONICAL_SIZE")
		}
		h := sha256.New()
		h.Write([]byte("lsp-trace:program-c-representative-review-actor:v2"))
		h.Write([]byte{0})
		h.Write(pre)
		actor.Identity = fmt.Sprintf("sha256:%x", h.Sum(nil))
		actors = append(actors, actor)
	}
	_, sb, sraw, err := p.EncodeReviewTaskSeparationV2(p.ReviewTaskSeparationV2{SchemaVersion: "lsp-trace.program-c-representative-review-task-separation.private.v2", WriterSessionID: actors[0].HarnessActorID, ReviewerTaskID: actors[1].HarnessActorID, ReviewerParentSessionID: in.actors[1].Actor.ParentID, ReviewerIsolated: in.actors[1].Actor.IsolationKind == "PI_ISOLATED_TASK", Subject: in.descriptor.Subject, ReviewScope: genuineScopeV2()})
	if err != nil {
		t.Fatal(err)
	}
	retain(sb, sraw)
	_, rb, rraw, err := p.EncodeQualificationReviewV2(p.QualificationReviewV2{SchemaVersion: p.QualificationReviewSchemaVersionV2, Profile: p.ProfileCallsOnlyFrozenCensusV1, Subject: in.descriptor.Subject, ReviewScope: genuineScopeV2(), Verdict: "ACCEPT", Writer: actors[0], Reviewer: actors[1], SeparationMethod: "DISTINCT_HARNESS_ISOLATED_TASK", SeparationEvidence: sb})
	if err != nil {
		t.Fatal(err)
	}
	retain(rb, rraw)
	_, eb, eraw, err := p.EncodeQualificationEnvelopeV2(p.QualificationEnvelopeV2{SchemaVersion: p.QualificationEnvelopeSchemaVersionV2, Profile: p.ProfileCallsOnlyFrozenCensusV1, Subject: in.descriptor.Subject, IndependentReviewReceipt: rb, Stage6Policy: pb, Stage6Limits: lb, Stage6Finalization: fb, QualificationEpoch: in.subject.TerminalProgramCHead.Generation})
	if err != nil {
		t.Fatal(err)
	}
	retain(eb, eraw)
	after := checkpointFilesV2(t, in.descriptor.StateRoot)
	// Retention may add only immutable qualification objects, never a head/CAS.
	for name, digest := range before {
		if after[name] != digest {
			t.Fatalf("GENUINE_RETAIN_NO_HEAD_CHANGE: %s", name)
		}
	}
	for name := range after {
		if _, ok := before[name]; !ok && !strings.HasPrefix(filepath.ToSlash(name), "program-c-representative/qualification/objects/private/v2/") {
			t.Fatalf("GENUINE_RETAIN_ONLY_QUALIFICATION_OBJECTS: %s", name)
		}
	}
	preparedJSONV2(t, genuineExecutionV2, "binding.json", genuineReportV2{Kind: "GENUINE_CONTROLLED_SUBJECT_RETENTION_NOT_PRODUCTION_QUALIFICATION", Subject: in.descriptor.Subject, Envelope: eb, EnvelopeBytes: eraw, PriorSuccessor: prior})
	t.Logf("GENUINE_RETENTION_READBACK_ZERO_CAS PASS envelope=%+v", eb)
}
func genuineReadReportV2(t *testing.T, in genuineInputsV2) genuineReportV2 {
	t.Helper()
	if err := preparedCanonicalDirectoryV2(genuineExecutionV2); err != nil {
		t.Fatal(err)
	}
	raw, err := preparedReadV2(genuineExecutionV2, "binding.json", 65536, false)
	if err != nil {
		t.Fatal(err)
	}
	var r genuineReportV2
	if err = genuineDecodeV2(raw, &r, true); err != nil {
		t.Fatal(err)
	}
	e, err := p.ParseQualificationEnvelopeV2(r.EnvelopeBytes)
	if err != nil || r.Kind != "GENUINE_CONTROLLED_SUBJECT_RETENTION_NOT_PRODUCTION_QUALIFICATION" || r.Subject != in.descriptor.Subject || r.Envelope.SchemaVersion != p.QualificationEnvelopeSchemaVersionV2 || r.Envelope.Identity != e.Identity || p.VerifyRawArtifactBindingV2(r.Envelope, r.EnvelopeBytes) != nil || e.Subject != r.Subject || e.Stage6Policy != in.subject.Stage6Policy || e.Stage6Limits != in.subject.Stage6Limits || e.Stage6Finalization != in.subject.Stage6Finalization || e.QualificationEpoch != in.subject.TerminalProgramCHead.Generation {
		t.Fatalf("GENUINE_REPORT_BINDING: %v", err)
	}
	if _, err = inventorysuccessor.ParseHeadRevision(r.PriorSuccessor); err != nil {
		t.Fatal(err)
	}
	return r
}
func genuineOperationV2(t *testing.T, in genuineInputsV2, r genuineReportV2, kind inventorysuccessor.MultiSeedOperationKindV2, replay bool) inventorysuccessor.MultiSeedOperationResultV2 {
	t.Helper()
	req := MultiSeedCheckpointRequestV2{Workflow: in.request, Operation: kind, Envelope: r.Envelope, ImplementationSource: in.source, MaxImplementationSourceBytes: 237568}
	// Private observer is counters-only; production owns all decisions and stores.
	var observed inventorysuccessor.MultiSeedOperationObservationsV2
	result, err := runMultiSeedCheckpointV2(context.Background(), req, func(o inventorysuccessor.MultiSeedOperationObservationsV2) { observed = o })
	if err != nil {
		t.Fatalf("GENUINE_STICKY_STOP_NO_RETRY kind=%s: %v", kind, err)
	}
	want := uint64(1)
	if replay {
		want = 0
	}
	if observed.CASAttempts != want || replay && observed.ImmutablePublishAttempts != 0 {
		t.Fatalf("GENUINE_EXACT_CAS_PUBLICATIONS kind=%s replay=%t: %+v", kind, replay, observed)
	}
	if !reflect.DeepEqual(observed, result.Observations) {
		t.Fatal("GENUINE_OBSERVER_RESULT_EQUALITY")
	}
	t.Logf("GENUINE_OPERATION PASS kind=%s replay=%t cas=%d publications=%d", kind, replay, observed.CASAttempts, observed.ImmutablePublishAttempts)
	return result
}
func genuineCheckResultV2(t *testing.T, in genuineInputsV2, r genuineReportV2, kind inventorysuccessor.MultiSeedOperationKindV2, result inventorysuccessor.MultiSeedOperationResultV2) {
	t.Helper()
	ctx := context.Background()
	root, err := in.request.ProgramC.OpenState()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if e := root.Close(); e != nil {
			t.Fatal(e)
		}
	}()
	objects, err := inventorysuccessor.NewMultiSeedObjectsV2(root)
	if err != nil {
		t.Fatal(err)
	}
	if kind == inventorysuccessor.MultiSeedSuccessorOperationV2 {
		gap, parseErr := inventorysuccessor.ParseMultiSeedGapLedgerV2(result.GapLedger)
		if parseErr != nil {
			t.Fatal(parseErr)
		}
		if result.Head.Generation != 2 || len(gap.Requirements) != 5 || gap.RepresentativeNomination.Status != "COMPUTED" || gap.RepresentativeNomination.Count == nil || *gap.RepresentativeNomination.Count != 24 || gap.CandidatePreparation.Status != "NOT_COMPUTED" || gap.CandidatePreparation.Count != nil || gap.Authority != 0 || gap.Accepted || gap.Outcome != "WAITING_EVIDENCE" {
			t.Fatal("GENUINE_FINAL_CEILINGS")
		}
		for i, g := range gap.Requirements {
			if i < 2 {
				if g.Status != "RETAINED" || g.EvidenceSelector == nil || g.EvidenceByteLength == nil {
					t.Fatal("GENUINE_FIRST_TWO_RETAINED")
				}
			} else if g.Status != "NOT_RETAINED" || g.EvidenceSelector != nil || g.EvidenceByteLength != nil {
				t.Fatal("GENUINE_SOURCE_GAPS_MISSING")
			}
		}
		heads, e := inventorysuccessor.NewMultiSeedSuccessorHeadsV2(root)
		if e != nil {
			t.Fatal(e)
		}
		raw, e := heads.Load(ctx, in.request.SuccessorHeadKey)
		if e != nil {
			t.Fatal(e)
		}
		var successor p.SuccessorHeadV2
		if e = genuineDecodeV2(raw, &successor, true); e != nil {
			t.Fatal(e)
		}
		prior, e := objects.ResolveSuccessorPredecessorV2(ctx, successor.PriorSuccessor, in.request.SuccessorWorkflowKey, in.request.SuccessorHeadKey, in.request.Identity, successor.PredecessorHead)
		if e != nil || !bytes.Equal(prior, r.PriorSuccessor) {
			t.Fatalf("GENUINE_FRESH_IMMUTABLE_PRIOR_RAW_V1: %v", e)
		}
		t.Log("GENUINE_FRESH_IMMUTABLE_PRIOR_RAW_V1 PASS; GENUINE_FINAL_CEILINGS PASS")
		return
	}
	raw, err := objects.Resolve(ctx, result.Artifact)
	if err != nil {
		t.Fatal(err)
	}
	if kind == inventorysuccessor.MultiSeedDenominatorOperationV2 {
		d, e := p.ParseDenominatorV2(raw)
		if e != nil || d.Subject != r.Subject || d.QualificationEnvelope != r.Envelope || len(d.Constituents) != 4 || len(d.Entries) != 24 || d.Accounting.TerminalRuns != 24 || d.Accounting.D != 24 || d.Accounting.G != 48 || d.Authority != 0 || d.Accepted {
			t.Fatalf("GENUINE_DENOMINATOR_D24_G48: %v", e)
		}
	} else {
		l, e := p.ParseLineageV2(raw)
		if e != nil || l.QualificationEnvelope != r.Envelope || len(l.Records) != 24 || l.Accounting.D != 24 || l.Accounting.G != 48 || l.Accounting.TerminalOuterResults != 24 || l.Accounting.UnresolvedOuterResults != 0 || l.Accounting.UnresolvedGroups != 0 || l.Authority != 0 || l.Accepted {
			t.Fatalf("GENUINE_LINEAGE_D24_G48: %v", e)
		}
	}
}

func TestGenuineMultiSeedQualificationV2(t *testing.T) {
	mode := os.Getenv("MULTISEED_GENUINE_MODE")
	if mode == "" {
		t.Skip("parent opt-in genuine controlled qualification absent")
	}
	if mode != "validate" && mode != "retain" && mode != "denominator" && mode != "lineage" && mode != "successor" && mode != "replay" {
		t.Fatal("GENUINE_EXPLICIT_MODE_REQUIRED")
	}
	before := checkpointFilesV2(t, genuineCorpusV2)
	nativeBefore := checkpointFilesV2(t, genuineNativeV2)
	in := genuineLoadV2(t, mode == "validate" || mode == "retain")
	if mode == "validate" {
		// All negative controls are in memory; the accepted C/N bytes never change.
		for i, raw := range in.projections {
			b := in.actors[i]
			verdict := []string{"SUBJECT_PRODUCED", "ACCEPT"}[i]
			mutations := [][]byte{append(append([]byte{}, raw...), '\n'), bytes.Replace(raw, []byte(`"subject":`), []byte(`"Subject":`), 1), bytes.Replace(raw, []byte("SUBJECT_CANONICAL_BYTES"), []byte("WRONG_SCOPE"), 1)}
			for _, bad := range mutations {
				_, _, e := genuineProjectionV2(bad, b, in.descriptor.Subject, verdict)
				if e == nil {
					t.Fatal("GENUINE_BAD_PROJECTION_BEFORE_WRITES")
				}
				// Hash-correct metadata-only copies must also fail canonical or
				// subject/scope validation, not merely the original raw pin.
				copyBinding := b
				copyBinding.Projection.SHA256 = workflowDigest(bad)
				copyBinding.Projection.ByteLength = uint64(len(bad))
				if _, _, e = genuineProjectionV2(bad, copyBinding, in.descriptor.Subject, verdict); e == nil {
					t.Fatal("GENUINE_RESEALED_BAD_PROJECTION_BEFORE_WRITES")
				}
			}
			foreign := in.descriptor.Subject
			foreign.Identity = "sha256:" + strings.Repeat("0", 64)
			if _, _, e := genuineProjectionV2(raw, b, foreign, verdict); e == nil {
				t.Fatal("GENUINE_FOREIGN_SUBJECT_BEFORE_WRITES")
			}
			// Re-sealed metadata-only perturbation must still fail semantic checks.
			bad := bytes.Replace(raw, []byte("SUBJECT_CANONICAL_BYTES"), []byte("WRONG_SCOPE"), 1)
			b.Projection.SHA256 = workflowDigest(bad)
			b.Projection.ByteLength = uint64(len(bad))
			if _, _, e := genuineProjectionV2(bad, b, in.descriptor.Subject, verdict); e == nil {
				t.Fatal("GENUINE_RESEALED_SCOPE_BEFORE_WRITES")
			}
		}
		badSubject := bytes.Replace(in.subjectRaw, []byte(`"authority":0`), []byte(`"authority":1`), 1)
		if _, e := p.ParseProgramCRepresentativeQualificationSubjectV2(badSubject); e == nil {
			t.Fatal("GENUINE_MALFORMED_SUBJECT_BEFORE_TRUST")
		}
		if !reflect.DeepEqual(before, checkpointFilesV2(t, genuineCorpusV2)) || !reflect.DeepEqual(nativeBefore, checkpointFilesV2(t, genuineNativeV2)) {
			t.Fatal("GENUINE_VALIDATE_READ_ONLY")
		}
		if _, err := os.Lstat(genuineExecutionV2); !os.IsNotExist(err) {
			t.Fatal("GENUINE_VALIDATE_NO_EXECUTION_EVIDENCE_OR_ENVELOPE")
		}
		for i, b := range in.actors {
			t.Logf("GENUINE_REGISTRY_NATIVE_MATCH PASS actor=%s id=%s parent=%s sha256=%s bytes=%d", b.Actor.Kind, b.Actor.ID, b.Actor.ParentID, b.Projection.SHA256, len(in.projections[i]))
		}
		t.Log("GENUINE_VALIDATE_READ_ONLY PASS; GENUINE_PROJECTION_CANONICAL_ROUNDTRIP PASS; GENUINE_RESEALED_SCOPE_BEFORE_WRITES PASS; GENUINE_MALFORMED_SUBJECT_BEFORE_TRUST PASS; GENUINE_VALIDATE_NO_EXECUTION_EVIDENCE_OR_ENVELOPE PASS")
		return
	}
	if mode == "retain" {
		genuineRetainV2(t, in)
		return
	}
	r := genuineReadReportV2(t, in)
	kinds := []inventorysuccessor.MultiSeedOperationKindV2{inventorysuccessor.MultiSeedDenominatorOperationV2, inventorysuccessor.MultiSeedLineageOperationV2, inventorysuccessor.MultiSeedSuccessorOperationV2}
	if mode == "replay" {
		stateBefore := checkpointFilesV2(t, in.descriptor.StateRoot)
		for _, kind := range kinds {
			raw, e := preparedReadV2(genuineExecutionV2, strings.ToLower(string(kind))+".result.json", 1048576, false)
			if e != nil {
				t.Fatal(e)
			}
			var original inventorysuccessor.MultiSeedOperationResultV2
			if e = genuineDecodeV2(raw, &original, true); e != nil {
				t.Fatal(e)
			}
			replay := genuineOperationV2(t, in, r, kind, true)
			genuineCheckResultV2(t, in, r, kind, replay)
			original.Observations = inventorysuccessor.MultiSeedOperationObservationsV2{}
			replay.Observations = inventorysuccessor.MultiSeedOperationObservationsV2{}
			originalRaw, _ := json.Marshal(original)
			replayRaw, _ := json.Marshal(replay)
			if !bytes.Equal(originalRaw, replayRaw) {
				t.Fatal("GENUINE_BYTE_IDENTICAL_REPLAY_RESULT")
			}
		}
		if !reflect.DeepEqual(stateBefore, checkpointFilesV2(t, in.descriptor.StateRoot)) || !reflect.DeepEqual(before, checkpointFilesV2(t, genuineCorpusV2)) || !reflect.DeepEqual(nativeBefore, checkpointFilesV2(t, genuineNativeV2)) {
			t.Fatal("GENUINE_REPLAY_ROOT_UNCHANGED")
		}
		t.Log("GENUINE_BYTE_IDENTICAL_REPLAY_RESULT PASS; GENUINE_REPLAY_ROOT_UNCHANGED PASS")
		return
	}
	kind := kinds[0]
	if mode == "lineage" {
		kind = kinds[1]
	} else if mode == "successor" {
		kind = kinds[2]
	}
	// A failed/unknown/canceled attempt leaves this create-only stop marker. A
	// later invocation cannot use the harness to retry or chain past that stop.
	if mode != "denominator" {
		predecessor := "denominator"
		if mode == "successor" {
			predecessor = "lineage"
		}
		if _, err := preparedReadV2(genuineExecutionV2, predecessor+".result.json", 1048576, false); err != nil {
			t.Fatalf("GENUINE_SEPARATE_PRIOR_PROCESS_REQUIRED: %v", err)
		}
	}
	preparedWriteV2(t, genuineExecutionV2, mode+".attempt", []byte("one parent-selected checkpoint attempt; no automatic retry\n"))
	result := genuineOperationV2(t, in, r, kind, false)
	genuineCheckResultV2(t, in, r, kind, result)
	preparedJSONV2(t, genuineExecutionV2, mode+".result.json", result)
}
