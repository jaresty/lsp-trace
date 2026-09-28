package adr0011acquisition

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"lsp-trace/internal/publication"
)

// This codec is synthetic and private. In particular a digest supplied by the
// caller is not an implementation-source review or a qualification.
const finalRecordRole = "REFERENCES_FINAL_ISSUANCE_V1"

var errFinalRecord = errors.New("private references final issuance not verified")

type finalDeclaration struct {
	Selector string
	Digest   string
	Facts    preinvokeFacts
}

// finalImplementationPin must select source independently of the claimant and
// of proposalContextInputs. Production passes nil and fails closed.
type finalImplementationPin func() (string, error)

func finalRecordDigest(b []byte) string {
	return privateDigest(append([]byte(finalRecordRole+"\x00"), b...))
}
func finalRecordSelector(d string) string {
	return "adr0011-references-issuance-v1-final-" + strings.TrimPrefix(d, "sha256:") + ".json"
}

func canonicalFinalRecord(root *publication.Root, candidate, proposal privateBodyPublication, x proposalContextInputs, declaration finalDeclaration, pin finalImplementationPin, read func(*publication.Root, string, int64) ([]byte, error)) ([]byte, error) {
	if read == nil {
		read = publication.ReadVerifiedBoundFile
	}
	if pin == nil {
		return nil, errFinalRecord
	}
	independentlySelected, err := pin()
	if err != nil || !validPrivateDigest(independentlySelected) || independentlySelected != x.ImplementationDigest || !replayCandidateRecord(root, candidate, proposal, x, read) {
		return nil, errFinalRecord
	}
	t, r := x.TargetInputs, x.ResponseInputs
	f := declaration.Facts
	if f.SessionID != r.Pair.SessionID || f.Generation != r.Pair.Generation || f.Workspace != t.Workspace || f.URI != t.Query.URI || f.Line != t.Query.Line || f.Character != t.Query.Character || f.Version != t.Binding.Version || f.SourceDigest != privateDigest(x.SourceBytes) || !bytes.Equal(f.Source, x.SourceBytes) || f.GitRoot != x.Git.Root || f.GitCommit != x.Git.Commit || f.Executable != x.Git.Executable || !verifyPreinvokeOccurrence(root, declaration.Selector, declaration.Digest, f, x.OccurrenceID) {
		return nil, errFinalRecord
	}
	context, err := canonicalProposalContext(root, x, read)
	if err != nil {
		return nil, errFinalRecord
	}
	proposalBytes, err := read(root, proposal.selector, sourceRecordLimit)
	if err != nil {
		return nil, errFinalRecord
	}
	var proposed struct {
		KnownE      int    `json:"known_e"`
		P           int    `json:"whole_result_p"`
		Outcome     string `json:"proposed_outcome"`
		Disposition string `json:"proposed_disposition"`
	}
	if json.Unmarshal(proposalBytes, &proposed) != nil || proposed.KnownE < 0 || proposed.KnownE > 1000 || proposed.P != proposed.KnownE || (proposed.KnownE == 0 && (proposed.Outcome != "COMPLETE_EMPTY" || proposed.Disposition != "EMPTY")) || (proposed.KnownE > 0 && (proposed.Outcome != "COMPLETE" || proposed.Disposition != "ITEMS")) {
		return nil, errFinalRecord
	}
	// The candidate is recalculated from exact retained raw bytes, not from its
	// claimant's occurrence array or a count inferred from a self-consistent final.
	expectedCandidate, err := canonicalCandidateRecord(root, proposal, x, read)
	if err != nil {
		return nil, errFinalRecord
	}
	var candidateFields struct {
		Occurrences []json.RawMessage `json:"occurrences"`
		P           int               `json:"proposed_p"`
	}
	if json.Unmarshal(expectedCandidate, &candidateFields) != nil || candidateFields.Occurrences == nil || len(candidateFields.Occurrences) != proposed.P || candidateFields.P != proposed.P {
		return nil, errFinalRecord
	}
	refs := []targetResultRef{
		targetRecordRef(candidateRecordRole, candidate), targetRecordRef(proposalRecordRole, proposal), targetRecordRef(eventsRole, x.Events), targetRecordRef(scannerRecordRole, x.Scanner), targetRecordRef(rawResultRole, x.RawResult), targetRecordRef(responseReadRole, x.ResponseRead), targetRecordRef(methodRecordVersion, x.Method), targetRecordRef(targetRecordVersion, x.Target), targetRecordRef(targetResultRole, t.Result), targetRecordRef(ownerReadRole, t.OwnerRead), targetRecordRef(ownerReadRole, r.OwnerRead), targetRecordRef(preparedSourceRole, t.Prepared), targetRecordRef(sourceIdentityRole, t.Source), targetRecordRef(revisionIdentityRole, t.Revision), targetRecordRef(hostGitRole, t.Before), targetRecordRef(hostGitRole, t.After),
	}
	for i, p := range x.Policies {
		refs = append(refs, targetRecordRef(policySelections[i].role, p.Record))
	}
	refs = append(refs, targetResultRef{"QUERY_OCCURRENCE_TEST_V1", declaration.Selector, declaration.Digest})
	if len(refs) != 21 || len(refs) > 64 {
		return nil, errFinalRecord
	}
	encoded := make([]string, 0, len(refs))
	seen := make(map[string]bool, len(refs))
	for _, ref := range refs {
		if ref.Selector == "" || !validPrivateDigest(ref.Digest) || ref.SchemaVersion == finalRecordRole {
			return nil, errFinalRecord
		}
		b := string(targetIdentityRefJSON(ref))
		if seen[b] {
			return nil, errFinalRecord
		}
		seen[b] = true
		encoded = append(encoded, b)
	}
	sort.Strings(encoded)
	q := func(s string) string { return string(targetIdentityStringJSON(s)) }
	body := []byte(fmt.Sprintf(`{"a":%d,"b":1,"candidate_ref":%s,"context":%s,"dependency_refs":[%s],"e":%d,"e_b":%d,"e_t":%d,"issued_disposition":%s,"issued_outcome":%s,"n":1,"p":%d,"proposal_ref":%s,"schema_version":%s,"t":1}`, proposed.P, targetIdentityRefJSON(refs[0]), context, strings.Join(encoded, ","), proposed.KnownE, proposed.KnownE, proposed.KnownE, q(proposed.Disposition), q(proposed.Outcome), proposed.P, targetIdentityRefJSON(refs[1]), q(finalRecordRole)))
	if len(body) > sourceRecordLimit {
		return nil, errFinalRecord
	}
	return body, nil
}

func replayFinalRecord(root *publication.Root, state, candidate, proposal privateBodyPublication, x proposalContextInputs, declaration finalDeclaration, pin finalImplementationPin, read func(*publication.Root, string, int64) ([]byte, error)) bool {
	return replayFinalRecordInternal(root, state, candidate, proposal, x, declaration, pin, read, false)
}
func replayFinalRecordInternal(root *publication.Root, state, candidate, proposal privateBodyPublication, x proposalContextInputs, declaration finalDeclaration, pin finalImplementationPin, read func(*publication.Root, string, int64) ([]byte, error), promoting bool) bool {
	if state.stage != "VERIFIED" && !(promoting && state.stage == "COMMITTED_UNVERIFIED") {
		return false
	}
	if read == nil {
		read = publication.ReadVerifiedBoundFile
	}
	got, err := read(root, state.selector, sourceRecordLimit)
	if err != nil || len(got) != state.byteCount || state.digest != finalRecordDigest(got) || state.selector != finalRecordSelector(state.digest) {
		return false
	}
	expected, err := canonicalFinalRecord(root, candidate, proposal, x, declaration, pin, read)
	return err == nil && bytes.Equal(got, expected)
}
func publishFinalRecord(root *publication.Root, candidate, proposal privateBodyPublication, x proposalContextInputs, declaration finalDeclaration, pin finalImplementationPin, read func(*publication.Root, string, int64) ([]byte, error), trace publication.BoundFileTrace) (privateBodyPublication, error) {
	absent := privateBodyPublication{stage: "ABSENT"}
	b, err := canonicalFinalRecord(root, candidate, proposal, x, declaration, pin, read)
	if err != nil {
		return absent, errFinalRecord
	}
	digest := finalRecordDigest(b)
	selector := finalRecordSelector(digest)
	precommit, uncertain := false, false
	receipt, err := publication.PublishBoundFileWithTrace(root, selector, b, func(got []byte) error {
		if !bytes.Equal(got, b) {
			return errFinalRecord
		}
		return nil
	}, func(event publication.BoundFileTraceEvent) {
		if trace != nil {
			trace(event)
		}
		if !event.OK {
			switch event.Stage {
			case "OPEN_VALIDATE", "CANDIDATE", "TEMP", "WRITE_FSYNC":
				precommit = true
			case "TARGET", "HARDLINK":
				if event.Result == "INJECTED_BEFORE_INSTALL" {
					precommit = true
				} else {
					uncertain = true
				}
			}
		}
		if event.Stage == "HARDLINK" && event.Result == "INSTALLED" {
			uncertain = true
		}
	})
	if err != nil || receipt == nil {
		if precommit && !uncertain {
			return absent, errFinalRecord
		}
		return privateBodyPublication{selector, digest, len(b), "COMMITTED_UNVERIFIED"}, errFinalRecord
	}
	state := privateBodyPublication{selector, digest, len(b), "COMMITTED_UNVERIFIED"}
	if !verifiedTargetPublication(receipt, selector, privateDigest(b), len(b)) || !replayFinalRecordInternal(root, state, candidate, proposal, x, declaration, pin, read, true) {
		return state, errFinalRecord
	}
	state.stage = "VERIFIED"
	return state, nil
}
