package adr0011acquisition

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"lsp-trace/internal/adr0011methodresult"
	"lsp-trace/internal/adr0011requestkey"
	"lsp-trace/internal/publication"
)

// proposalContextInputs is selected by the test owner, not populated from a
// claimant proposal. Its occurrence is synthetic and never admits a live query.
// ImplementationDigest is an explicit independent TEST-ONLY owner pin.
type proposalContextInputs struct {
	TargetInputs                                             targetRecordInputs
	ResponseInputs                                           responseReadInputs
	MethodInputs                                             methodRecordInputs
	Method, Target, ResponseRead, RawResult, Scanner, Events privateBodyPublication
	Policies                                                 [4]policyPublication
	Payload, Journal                                         []byte
	ScannerObservation                                       rawScannerObservation
	Evaluation                                               *adr0011methodresult.ReferenceEvaluation
	Git                                                      hostGitExpectation
	SourceBytes                                              []byte
	SchemaDigest, ImplementationDigest, OccurrenceID         string
}

var errProposalContext = errors.New("private references proposal context not verified")

// These are context indexing aliases, not $defs names, selector roles or schema IDs.
var proposalContextSchemaRoles = [...]string{
	"candidate", "events", "final", "host-git", "method-record", "owner-read", "policy", "prepared-source",
	"proposal", "raw-result", "response-read", "revision-identity", "scanner", "source-identity", "target-record", "target-result",
}

const proposalContextSchemaDigest = "sha256:" + reviewedSuccessorSchemaDigest

// canonicalProposalContext constructs only $defs/context. It neither publishes
// records nor qualifies an occurrence. Every ref is derived from replayed state.
func canonicalProposalContext(root *publication.Root, x proposalContextInputs, read func(*publication.Root, string, int64) ([]byte, error)) ([]byte, error) {
	fail := func() ([]byte, error) { return nil, errProposalContext }
	t, r, m := x.TargetInputs, x.ResponseInputs, x.MethodInputs
	if root == nil || x.SchemaDigest != reviewedSuccessorSchemaDigest || !validPrivateDigest(x.ImplementationDigest) ||
		!targetResultValidDigest(x.OccurrenceID) || x.OccurrenceID != t.Query.OccurrenceID ||
		!bytes.Equal(x.SourceBytes, t.Original) || x.Git != t.Git || t.Workspace != x.Git.Root ||
		r.TargetInputs.SchemaDigest != x.SchemaDigest || t.SchemaDigest != x.SchemaDigest || r.SchemaDigest != x.SchemaDigest ||
		m.ResponseInputs.SchemaDigest != x.SchemaDigest || m.ResponseInputs.TargetInputs.SchemaDigest != x.SchemaDigest ||
		r.Target != x.Target || m.ResponseRead != x.ResponseRead || m.RawResult != x.RawResult || m.ResponseInputs.Target != x.Target ||
		r.Query != t.Query || m.ResponseInputs.Query != t.Query ||
		!sameProposalTarget(t, r.TargetInputs) || !sameProposalTarget(t, m.ResponseInputs.TargetInputs) ||
		!reflect.DeepEqual(r.Pair, m.ResponseInputs.Pair) || r.Binding != m.ResponseInputs.Binding ||
		r.OwnerRead != m.ResponseInputs.OwnerRead || r.Payload != m.ResponseInputs.Payload ||
		!reflect.DeepEqual(r.Frames, m.ResponseInputs.Frames) ||
		t.Prepared.stage != "VERIFIED" || t.Source.stage != "VERIFIED" || t.Revision.stage != "VERIFIED" ||
		t.Before.stage != "VERIFIED" || t.After.stage != "VERIFIED" || t.OwnerRead.stage != "VERIFIED" || t.Result.stage != "VERIFIED" ||
		r.OwnerRead.stage != "VERIFIED" || r.Payload.stage != "VERIFIED" ||
		x.Method.stage != "VERIFIED" || x.Target.stage != "VERIFIED" || x.ResponseRead.stage != "VERIFIED" ||
		x.RawResult.stage != "VERIFIED" || x.Scanner.stage != "VERIFIED" || x.Events.stage != "VERIFIED" {
		return fail()
	}
	if read == nil {
		read = publication.ReadVerifiedBoundFile
	}
	if !replayPreparedSource(root, t.Prepared, t.Source, x.SourceBytes, t.Binding.URI, t.Binding.Version, t.Workspace, x.SchemaDigest, read) ||
		!replayRevisionIdentity(root, t.Revision, t.Before, t.After, x.Git, x.SchemaDigest, read) ||
		!replayTargetRecord(root, x.Target, t, read) || !replayResponseRead(root, x.ResponseRead, r, read) ||
		!replayRawResult(root, x.RawResult, x.ResponseRead, r.Payload, r, read) ||
		!replayScannerRecord(root, x.Scanner, x.ResponseRead, x.RawResult, r.Payload, r, read, nil) ||
		!replayMethodRecord(root, x.Method, m, read) {
		return fail()
	}
	for i, p := range x.Policies {
		if !replayPolicy(root, policySelections[i], p, nil, read) {
			return fail()
		}
	}
	response, ok := responseReadExpected(root, r, read)
	if !ok {
		return fail()
	}
	tx, err := referencesTransactionID(r.Pair.SessionID, r.Pair.Generation, r.Pair.Key, r.Frames.invocation, x.OccurrenceID,
		targetRecordRef(sourceIdentityRole, t.Source), targetRecordRef(revisionIdentityRole, t.Revision))
	if err != nil || tx != response.TransactionID || !validProposalEvaluation(x.Evaluation, r, x.ScannerObservation, x.Payload) ||
		!replayEvents(root, x.Events, x.ResponseRead, x.RawResult, x.ScannerObservation, r, tx, x.Payload, x.Journal, read) {
		return fail()
	}
	observed, err := observeRawScanner(root, r.Payload, read)
	if err != nil || observed != x.ScannerObservation || !validTargetIdentityString(r.Pair.SessionID) || !validTargetIdentityString(r.Frames.invocation) {
		return fail()
	}
	retained, err := read(root, r.Payload.selector, rawResultPayloadLimit)
	if err != nil || !bytes.Equal(x.Payload, retained) {
		return fail()
	}
	field := func(s string) string { return string(targetIdentityStringJSON(s)) }
	ref := func(role string, p privateBodyPublication) string {
		return string(targetIdentityRefJSON(targetRecordRef(role, p)))
	}
	schemas := make([]string, len(proposalContextSchemaRoles))
	for i, role := range proposalContextSchemaRoles {
		schemas[i] = fmt.Sprintf(`{"digest":%s,"role":%s}`, field(proposalContextSchemaDigest), field(role))
	}
	// ASCII keys are in UTF-16/JCS order; the shared encoder handles NFC strings.
	values := []string{
		`"admission_policy_ref":` + ref(admissionPolicyRole, x.Policies[1].Record),
		`"generation":` + fmt.Sprint(r.Pair.Generation),
		`"implementation_digest":` + field(x.ImplementationDigest),
		`"invocation_id":` + field(r.Frames.invocation),
		`"method_policy_ref":` + ref(methodPolicyRole, x.Policies[0].Record),
		`"method_ref":` + ref(methodRecordVersion, x.Method),
		`"privacy_policy_ref":` + ref(privacyPolicyRole, x.Policies[2].Record),
		`"query_occurrence_id":` + field(x.OccurrenceID),
		`"request_key":` + field(adr0011requestkey.Encode(r.Pair.Key)),
		`"retention_policy_ref":` + ref(retentionPolicyRole, x.Policies[3].Record),
		`"revision_identity_ref":` + ref(revisionIdentityRole, t.Revision),
		`"schema_digests":[` + strings.Join(schemas, ",") + `]`,
		`"session_id":` + field(r.Pair.SessionID),
		`"source_identity_ref":` + ref(sourceIdentityRole, t.Source),
		`"target_ref":` + ref(targetRecordVersion, x.Target),
		`"transaction_id":` + field(tx),
	}
	result := []byte("{" + strings.Join(values, ",") + "}")
	if len(result) > sourceRecordLimit {
		return fail()
	}
	return result, nil
}

func sameProposalTarget(a, b targetRecordInputs) bool {
	return reflect.DeepEqual(a.Frames, b.Frames) && reflect.DeepEqual(a.Pair, b.Pair) && a.Binding == b.Binding && a.Query == b.Query &&
		bytes.Equal(a.Original, b.Original) && a.Workspace == b.Workspace && a.Git == b.Git && a.SchemaDigest == b.SchemaDigest &&
		a.Prepared == b.Prepared && a.Source == b.Source && a.Revision == b.Revision && a.Before == b.Before &&
		a.After == b.After && a.OwnerRead == b.OwnerRead && a.Result == b.Result
}

// This tests the evaluation's independent, non-admitting shape against the
// scanner and replayed attached journal. It cannot promote T or A.
func validProposalEvaluation(e *adr0011methodresult.ReferenceEvaluation, r responseReadInputs, s rawScannerObservation, payload []byte) bool {
	if e == nil || e.Key != r.Pair.Key || e.RawDigest != privateDigest(payload) || e.E != s.knownE || e.P != s.knownE ||
		e.EB != s.knownE || e.ET != s.knownE || e.N != 1 || e.B != 1 || e.T != 0 || e.A != 0 || e.FailureOrdinal != -1 ||
		len(e.Events) != 2*s.knownE || len(e.Items) != s.knownE {
		return false
	}
	for i, event := range e.Events {
		transition := "BEGIN"
		if i%2 != 0 {
			transition = "TERMINAL"
		}
		if event.Ordinal != i/2 || event.Transition != transition {
			return false
		}
	}
	disposition, outcome := "ITEMS", "COMPLETE"
	if s.knownE == 0 {
		disposition, outcome = "EMPTY", "COMPLETE_EMPTY"
	}
	return e.Disposition == disposition && e.Outcome == outcome
}
