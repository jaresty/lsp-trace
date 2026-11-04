package inventorysuccessor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"sort"
	"sync"

	"lsp-trace/internal/censuscontinuation"
	"lsp-trace/internal/censussourcecontinuation"
	"lsp-trace/internal/programcrepresentative"
	"lsp-trace/internal/publication"
)

type MultiSeedOperationKindV2 string

const (
	MultiSeedDenominatorOperationV2 MultiSeedOperationKindV2 = "DENOMINATOR"
	MultiSeedLineageOperationV2     MultiSeedOperationKindV2 = "LINEAGE"
	MultiSeedSuccessorOperationV2   MultiSeedOperationKindV2 = "SUCCESSOR"
)

var ErrMultiSeedConflictV2 = errors.New("CONFLICT: multiseed checkpoint CAS not committed")
var ErrMultiSeedIndeterminateV2 = errors.New("INDETERMINATE_EXECUTION: multiseed checkpoint CAS commit unknown")

type MultiSeedOperationResultV2 struct {
	Outcome      Outcome
	Head         censussourcecontinuation.HeadRevision
	Artifact     programcrepresentative.ArtifactBindingDTO
	GapLedger    []byte
	Observations MultiSeedOperationObservationsV2
}

// Each operation object owns one explicitly selected CAS boundary. No method
// chains a following operation, retries a failed CAS, or clears a typed stop.
type MultiSeedCheckpointOperationV2 struct {
	mu                                             sync.Mutex
	kind                                           MultiSeedOperationKindV2
	workflowKey, headKey                           string
	identity                                       WorkflowIdentity
	q                                              censuscontinuation.VerifiedQualificationV2
	objects                                        *MultiSeedObjectsV2
	denominatorHeads, lineageHeads, successorHeads censussourcecontinuation.HeadStore
	stop                                           error
	observations                                   MultiSeedOperationObservationsV2
}
type MultiSeedOperationObservationsV2 struct {
	CASAttempts              uint64
	ImmutablePublishAttempts uint64
}

func (o *MultiSeedCheckpointOperationV2) ObservationsV2() MultiSeedOperationObservationsV2 {
	if o == nil {
		return MultiSeedOperationObservationsV2{}
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.observations
}
func NewMultiSeedCheckpointOperationV2(root *publication.Root, kind MultiSeedOperationKindV2, workflowKey, headKey string, identity WorkflowIdentity, q censuscontinuation.VerifiedQualificationV2) (*MultiSeedCheckpointOperationV2, error) {
	if kind != MultiSeedDenominatorOperationV2 && kind != MultiSeedLineageOperationV2 && kind != MultiSeedSuccessorOperationV2 || workflowKey == "" || headKey == "" || !validIdentity(identity) {
		return nil, ErrInvalid
	}
	s, err := q.SubjectDTOv2()
	if err != nil {
		return nil, err
	}
	expected := programcrepresentative.QualificationWorkflowIdentityDTO{Profile: identity.Profile, Contract: identity.Contract, CensusReceiptSHA256: identity.CensusReceiptSHA256, AvailabilityLedgerSHA256: identity.AvailabilityLedgerSHA256, SourcePinSHA256: identity.SourcePinSHA256, ProgramCPlanSHA256: identity.ProgramCPlanSHA256, ProgramCBindingsSHA256: identity.ProgramCBindingsSHA256, PrivacyPolicySHA256: identity.PrivacyPolicySHA256, RetentionPolicySHA256: identity.RetentionPolicySHA256, ResourcePolicySHA256: identity.ResourcePolicySHA256, ReplayPolicySHA256: identity.ReplayPolicySHA256, QualificationPolicySHA256: identity.QualificationPolicySHA256, Provenance: "PREDECESSOR_RETAINED_CALLER_ASSERTED"}
	if s.WorkflowIdentity != expected {
		return nil, ErrInvalid
	}
	objects, err := NewMultiSeedObjectsV2(root)
	if err != nil {
		return nil, err
	}
	dh, err := NewMultiSeedEvidenceHeadsV2(root, "denominator")
	if err != nil {
		return nil, err
	}
	lh, err := NewMultiSeedEvidenceHeadsV2(root, "lineage")
	if err != nil {
		return nil, err
	}
	sh, err := NewMultiSeedSuccessorHeadsV2(root)
	if err != nil {
		return nil, err
	}
	operation := &MultiSeedCheckpointOperationV2{kind: kind, workflowKey: workflowKey, headKey: headKey, identity: identity, q: q, objects: objects, denominatorHeads: dh, lineageHeads: lh, successorHeads: sh}
	objects.onPublish = func() { operation.observations.ImmutablePublishAttempts++ }
	return operation, nil
}
func (o *MultiSeedCheckpointOperationV2) Execute(ctx context.Context) (MultiSeedOperationResultV2, error) {
	if o == nil {
		return MultiSeedOperationResultV2{}, ErrInvalid
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.stop != nil {
		return MultiSeedOperationResultV2{}, o.stop
	}
	if ctx == nil {
		o.stop = ErrInvalid
		return MultiSeedOperationResultV2{}, o.stop
	}
	if err := ctx.Err(); err != nil {
		o.stop = err
		return MultiSeedOperationResultV2{}, err
	}
	var result MultiSeedOperationResultV2
	var err error
	switch o.kind {
	case MultiSeedDenominatorOperationV2:
		result, err = o.commitDenominator(ctx)
	case MultiSeedLineageOperationV2:
		result, err = o.commitLineage(ctx)
	case MultiSeedSuccessorOperationV2:
		result, err = o.commitSuccessor(ctx)
	default:
		err = ErrInvalid
	}
	if err != nil {
		o.stop = err
		return MultiSeedOperationResultV2{}, err
	}
	result.Observations = o.observations
	return result, nil
}
func (o *MultiSeedCheckpointOperationV2) cas(ctx context.Context, heads censussourcecontinuation.HeadStore, expected censussourcecontinuation.HeadRevision, raw []byte) error {
	if o.stop != nil {
		return o.stop
	}
	if ctx == nil {
		o.stop = ErrInvalid
		return o.stop
	}
	if err := ctx.Err(); err != nil {
		o.stop = err
		return err
	}
	o.observations.CASAttempts++
	result := heads.CompareAndSwap(ctx, o.headKey, expected, raw)
	switch result.Outcome {
	case censussourcecontinuation.CASCommitted:
		if result.Err == nil {
			return nil
		}
		o.stop = errors.Join(ErrMultiSeedIndeterminateV2, result.Err)
	case censussourcecontinuation.CASNotCommitted:
		o.stop = errors.Join(ErrMultiSeedConflictV2, result.Err)
	default:
		o.stop = errors.Join(ErrMultiSeedIndeterminateV2, result.Err)
	}
	return o.stop
}
func (o *MultiSeedCheckpointOperationV2) checkWorkflow(ctx context.Context) ([]byte, error) {
	raw, err := o.successorHeads.Load(ctx, o.headKey)
	if err != nil {
		return nil, err
	}
	s, err := o.q.SubjectDTOv2()
	if err != nil {
		return nil, err
	}
	revision, err := ParseMultiSeedSuccessorHeadRevisionV2(raw)
	if err != nil || revision.Generation == 0 {
		return nil, ErrInvalid
	}
	if revision.Generation == 1 {
		var v head
		if strict(raw, &v) != nil || v.WorkflowKey != o.workflowKey || v.Identity != o.identity || v.PredecessorHead != s.TerminalProgramCHead.Selector {
			return nil, ErrInvalid
		}
	} else {
		v, err := parseMultiSeedSuccessorV2(raw)
		if err != nil || v.WorkflowKey != o.workflowKey || WorkflowIdentity(v.Identity) != o.identity || v.PredecessorHead != s.TerminalProgramCHead.Selector {
			return nil, ErrInvalid
		}
		if _, err = o.objects.ResolveSuccessorPredecessorV2(ctx, v.PriorSuccessor, o.workflowKey, o.headKey, o.identity, v.PredecessorHead); err != nil {
			return nil, err
		}
	}
	return raw, nil
}
func (o *MultiSeedCheckpointOperationV2) retainAndVerify(ctx context.Context, objects map[programcrepresentative.ArtifactBindingDTO][]byte) error {
	keys := make([]programcrepresentative.ArtifactBindingDTO, 0, len(objects))
	for b := range objects {
		keys = append(keys, b)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].SchemaVersion != keys[j].SchemaVersion {
			return keys[i].SchemaVersion < keys[j].SchemaVersion
		}
		return keys[i].SHA256 < keys[j].SHA256
	})
	for _, b := range keys {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := o.objects.Retain(ctx, b, objects[b]); err != nil {
			return err
		}
	}
	return o.verifyObjects(ctx, objects)
}
func (o *MultiSeedCheckpointOperationV2) verifyObjects(ctx context.Context, objects map[programcrepresentative.ArtifactBindingDTO][]byte) error {
	for b, expected := range objects {
		raw, err := o.objects.Resolve(ctx, b)
		if err != nil || !bytes.Equal(raw, expected) {
			return errors.Join(ErrInvalid, err)
		}
	}
	return ctx.Err()
}
func (o *MultiSeedCheckpointOperationV2) evidenceHead(kind string, artifact, denominator programcrepresentative.ArtifactBindingDTO) (programcrepresentative.EvidenceHeadV2, []byte, error) {
	sb, err := o.q.SubjectBindingV2()
	if err != nil {
		return programcrepresentative.EvidenceHeadV2{}, nil, err
	}
	eb, err := o.q.EnvelopeBindingV2()
	if err != nil {
		return programcrepresentative.EvidenceHeadV2{}, nil, err
	}
	v := programcrepresentative.EvidenceHeadV2{SchemaVersion: multiSeedEvidenceHeadVersionV2(kind), WorkflowKey: o.workflowKey, HeadKey: o.headKey, Generation: 1, Subject: sb, QualificationEnvelope: eb, Artifact: artifact, Denominator: denominator, ClaimCeiling: programcrepresentative.FixedClaimCeilingV2()}
	raw, err := encodeMultiSeedEvidenceHeadV2(v)
	if err != nil {
		return v, nil, err
	}
	v, err = parseMultiSeedEvidenceHeadV2(raw, kind)
	return v, raw, err
}
func (o *MultiSeedCheckpointOperationV2) loadDenominator(ctx context.Context) (programcrepresentative.VerifiedDenominatorV2, []byte, error) {
	zero := programcrepresentative.VerifiedDenominatorV2{}
	raw, err := o.denominatorHeads.Load(ctx, o.headKey)
	if err != nil {
		return zero, nil, err
	}
	head, err := parseMultiSeedEvidenceHeadV2(raw, "denominator")
	if err != nil {
		return zero, nil, err
	}
	body, err := o.objects.Resolve(ctx, head.Artifact)
	if err != nil {
		return zero, nil, err
	}
	d, err := o.q.VerifyDenominatorArtifactV2(body)
	if err != nil {
		return zero, nil, err
	}
	b, err := d.BindingV2()
	if err != nil {
		return zero, nil, err
	}
	_, expected, err := o.evidenceHead("denominator", b, b)
	if err != nil || !bytes.Equal(raw, expected) {
		return zero, nil, ErrInvalid
	}
	objects, err := o.q.RetainedQualificationArtifactsV2()
	if err != nil {
		return zero, nil, err
	}
	if err = o.verifyObjects(ctx, objects); err != nil {
		return zero, nil, err
	}
	return d, raw, nil
}
func (o *MultiSeedCheckpointOperationV2) loadLineage(ctx context.Context, d programcrepresentative.VerifiedDenominatorV2) (programcrepresentative.VerifiedLineageV2, []byte, error) {
	zero := programcrepresentative.VerifiedLineageV2{}
	raw, err := o.lineageHeads.Load(ctx, o.headKey)
	if err != nil {
		return zero, nil, err
	}
	head, err := parseMultiSeedEvidenceHeadV2(raw, "lineage")
	if err != nil {
		return zero, nil, err
	}
	body, err := o.objects.Resolve(ctx, head.Artifact)
	if err != nil {
		return zero, nil, err
	}
	l, err := o.q.VerifyLineageArtifactV2(ctx, body, d, o.objects)
	if err != nil {
		return zero, nil, err
	}
	b, err := l.BindingV2()
	if err != nil {
		return zero, nil, err
	}
	db, err := d.BindingV2()
	if err != nil {
		return zero, nil, err
	}
	_, expected, err := o.evidenceHead("lineage", b, db)
	if err != nil || !bytes.Equal(raw, expected) {
		return zero, nil, ErrInvalid
	}
	return l, raw, nil
}
func (o *MultiSeedCheckpointOperationV2) commitDenominator(ctx context.Context) (MultiSeedOperationResultV2, error) {
	zero := MultiSeedOperationResultV2{}
	if _, err := o.checkWorkflow(ctx); err != nil {
		return zero, err
	}
	d, err := o.q.BuildDenominatorArtifactV2()
	if err != nil {
		return zero, err
	}
	b, err := d.BindingV2()
	if err != nil {
		return zero, err
	}
	head, wanted, err := o.evidenceHead("denominator", b, b)
	if err != nil {
		return zero, err
	}
	current, err := o.denominatorHeads.Load(ctx, o.headKey)
	if err != nil {
		return zero, err
	}
	if len(current) > 0 {
		if !bytes.Equal(current, wanted) {
			return zero, ErrInvalid
		}
		if _, _, err = o.loadDenominator(ctx); err != nil {
			return zero, err
		}
		return MultiSeedOperationResultV2{Outcome: OutcomeWaitingEvidence, Head: censussourcecontinuation.HeadRevision{Generation: 1, Selector: head.Selector}, Artifact: b}, nil
	}
	objects, err := o.q.RetainedQualificationArtifactsV2()
	if err != nil {
		return zero, err
	}
	body, err := d.CanonicalBytesV2()
	if err != nil {
		return zero, err
	}
	objects[b] = body
	if err = o.retainAndVerify(ctx, objects); err != nil {
		return zero, err
	}
	if err = o.cas(ctx, o.denominatorHeads, censussourcecontinuation.HeadRevision{}, wanted); err != nil {
		return zero, err
	}
	return MultiSeedOperationResultV2{Outcome: OutcomeWaitingEvidence, Head: censussourcecontinuation.HeadRevision{Generation: 1, Selector: head.Selector}, Artifact: b}, nil
}
func (o *MultiSeedCheckpointOperationV2) commitLineage(ctx context.Context) (MultiSeedOperationResultV2, error) {
	zero := MultiSeedOperationResultV2{}
	if _, err := o.checkWorkflow(ctx); err != nil {
		return zero, err
	}
	d, denominatorRaw, err := o.loadDenominator(ctx)
	if err != nil {
		return zero, err
	}
	current, err := o.lineageHeads.Load(ctx, o.headKey)
	if err != nil {
		return zero, err
	}
	if len(current) > 0 {
		l, raw, err := o.loadLineage(ctx, d)
		if err != nil {
			return zero, err
		}
		b, _ := l.BindingV2()
		h, err := parseMultiSeedEvidenceHeadV2(raw, "lineage")
		if err != nil {
			return zero, err
		}
		return MultiSeedOperationResultV2{Outcome: OutcomeWaitingEvidence, Head: censussourcecontinuation.HeadRevision{Generation: 1, Selector: h.Selector}, Artifact: b}, nil
	}
	l, err := o.q.BuildLineageArtifactV2(d)
	if err != nil {
		return zero, err
	}
	b, err := l.BindingV2()
	if err != nil {
		return zero, err
	}
	db, _ := d.BindingV2()
	head, wanted, err := o.evidenceHead("lineage", b, db)
	if err != nil {
		return zero, err
	}
	objects, err := l.ArtifactsV2()
	if err != nil {
		return zero, err
	}
	body, err := l.CanonicalBytesV2()
	if err != nil {
		return zero, err
	}
	objects[b] = body
	if err = o.retainAndVerify(ctx, objects); err != nil {
		return zero, err
	}
	if _, err = o.q.VerifyLineageArtifactV2(ctx, body, d, o.objects); err != nil {
		return zero, err
	}
	now, err := o.denominatorHeads.Load(ctx, o.headKey)
	if err != nil || !bytes.Equal(now, denominatorRaw) {
		return zero, errors.Join(ErrMultiSeedConflictV2, err)
	}
	if err = o.cas(ctx, o.lineageHeads, censussourcecontinuation.HeadRevision{}, wanted); err != nil {
		return zero, err
	}
	return MultiSeedOperationResultV2{Outcome: OutcomeWaitingEvidence, Head: censussourcecontinuation.HeadRevision{Generation: 1, Selector: head.Selector}, Artifact: b}, nil
}
func (o *MultiSeedCheckpointOperationV2) commitSuccessor(ctx context.Context) (MultiSeedOperationResultV2, error) {
	zero := MultiSeedOperationResultV2{}
	d, denominatorRaw, err := o.loadDenominator(ctx)
	if err != nil {
		return zero, err
	}
	l, lineageRaw, err := o.loadLineage(ctx, d)
	if err != nil {
		return zero, err
	}
	proof, err := o.q.ProjectCheckpointV2(d, l)
	if err != nil {
		return zero, err
	}
	current, err := o.checkWorkflow(ctx)
	if err != nil {
		return zero, err
	}
	revision, err := ParseMultiSeedSuccessorHeadRevisionV2(current)
	if err != nil {
		return zero, err
	}
	if revision.Generation == 2 {
		v, err := parseMultiSeedSuccessorV2(current)
		if err != nil || v.Denominator != proof.Denominator || v.Lineage != proof.Lineage {
			return zero, ErrInvalid
		}
		ledger, err := gapLedgerForMultiSeedSuccessorV2(v, proof.D)
		if err != nil {
			return zero, err
		}
		return MultiSeedOperationResultV2{Outcome: OutcomeWaitingEvidence, Head: revision, Artifact: proof.Lineage, GapLedger: ledger}, nil
	}
	prior, err := o.objects.RetainSuccessorPredecessorV2(ctx, current, o.workflowKey, o.headKey, o.identity, proof.TerminalProgramCHead.Selector)
	if err != nil {
		return zero, err
	}
	var successor programcrepresentative.SuccessorHeadV2
	if err = json.Unmarshal(current, &successor); err != nil {
		return zero, err
	}
	successor.SchemaVersion = multiSeedSuccessorVersionV2
	successor.Generation = 2
	successor.MissingEvidence = []string{string(GapRetainedSourceClosure), string(GapFullDefinitionBindings), string(GapCallerOccurrenceV3Custody)}
	successor.PriorSuccessor = prior
	successor.Denominator = proof.Denominator
	successor.Lineage = proof.Lineage
	wanted, err := encodeMultiSeedSuccessorV2(successor)
	if err != nil {
		return zero, err
	}
	if _, err = parseMultiSeedSuccessorV2(wanted); err != nil {
		return zero, err
	}
	now, err := o.denominatorHeads.Load(ctx, o.headKey)
	if err != nil || !bytes.Equal(now, denominatorRaw) {
		return zero, errors.Join(ErrMultiSeedConflictV2, err)
	}
	now, err = o.lineageHeads.Load(ctx, o.headKey)
	if err != nil || !bytes.Equal(now, lineageRaw) {
		return zero, errors.Join(ErrMultiSeedConflictV2, err)
	}
	if err = o.cas(ctx, o.successorHeads, revision, wanted); err != nil {
		return zero, err
	}
	committed, err := o.checkWorkflow(ctx)
	if err != nil || !bytes.Equal(committed, wanted) {
		return zero, errors.Join(ErrMultiSeedIndeterminateV2, err)
	}
	v, err := parseMultiSeedSuccessorV2(committed)
	if err != nil {
		return zero, err
	}
	ledger, err := gapLedgerForMultiSeedSuccessorV2(v, proof.D)
	if err != nil {
		return zero, err
	}
	return MultiSeedOperationResultV2{Outcome: OutcomeWaitingEvidence, Head: censussourcecontinuation.HeadRevision{Generation: 2, Selector: v.Selector}, Artifact: proof.Lineage, GapLedger: ledger}, nil
}
