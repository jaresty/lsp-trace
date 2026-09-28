package adr0011acquisition

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"

	"lsp-trace/internal/publication"
	"lsp-trace/internal/strictjson"
)

const revisionIdentityRole = "REFERENCES_REVISION_IDENTITY_V1"

var errRevisionIdentity = errors.New("private revision identity not verified")

type revisionIdentityRecord struct {
	SchemaVersion    string          `json:"schema_version"`
	RootURI          string          `json:"root_uri"`
	Commit           string          `json:"commit"`
	Custody          string          `json:"custody"`
	HostGitBeforeRef targetResultRef `json:"host_git_before_ref"`
	HostGitAfterRef  targetResultRef `json:"host_git_after_ref"`
}

func revisionHostRef(state privateBodyPublication) targetResultRef {
	return targetResultRef{SchemaVersion: hostGitRole, Selector: state.selector, Digest: state.digest}
}

func canonicalRevisionIdentity(r revisionIdentityRecord) ([]byte, error) {
	if r.SchemaVersion != revisionIdentityRole || r.Custody != "CALLER_ASSERTED" ||
		r.HostGitBeforeRef.SchemaVersion != hostGitRole || r.HostGitAfterRef.SchemaVersion != hostGitRole ||
		!targetResultValidDigest(r.HostGitBeforeRef.Digest) || !targetResultValidDigest(r.HostGitAfterRef.Digest) ||
		r.HostGitBeforeRef.Selector != hostGitRecordSelector(r.HostGitBeforeRef.Digest) ||
		r.HostGitAfterRef.Selector != hostGitRecordSelector(r.HostGitAfterRef.Digest) {
		return nil, errRevisionIdentity
	}
	var b bytes.Buffer
	b.WriteString(`{"commit":`)
	writeJCSString(&b, r.Commit)
	b.WriteString(`,"custody":`)
	writeJCSString(&b, r.Custody)
	writeRef := func(ref targetResultRef) {
		b.WriteString(`{"digest":`)
		writeJCSString(&b, ref.Digest)
		b.WriteString(`,"schema_version":`)
		writeJCSString(&b, ref.SchemaVersion)
		b.WriteString(`,"selector":`)
		writeJCSString(&b, ref.Selector)
		b.WriteByte('}')
	}
	b.WriteString(`,"host_git_after_ref":`)
	writeRef(r.HostGitAfterRef)
	b.WriteString(`,"host_git_before_ref":`)
	writeRef(r.HostGitBeforeRef)
	b.WriteString(`,"root_uri":`)
	writeJCSString(&b, r.RootURI)
	b.WriteString(`,"schema_version":`)
	writeJCSString(&b, r.SchemaVersion)
	b.WriteByte('}')
	if b.Len() > sourceRecordLimit {
		return nil, errRevisionIdentity
	}
	return b.Bytes(), nil
}

func readRevisionHost(root *publication.Root, state privateBodyPublication, x hostGitExpectation, read func(*publication.Root, string, int64) ([]byte, error)) (hostGitRecord, bool) {
	var r hostGitRecord
	if state.stage != "VERIFIED" || !replayHostGit(root, state, x, reviewedSuccessorSchemaDigest, read) {
		return r, false
	}
	b, err := read(root, state.selector, sourceRecordLimit)
	if err != nil {
		return r, false
	}
	return decodeHostGit(b)
}

// replayRevisionIdentity requires independently owner-selected BEFORE and AFTER
// receipts. A readable object alone cannot promote an unverified publication.
func replayRevisionIdentity(root *publication.Root, state, before, after privateBodyPublication, expected hostGitExpectation, schemaDigest string, read func(*publication.Root, string, int64) ([]byte, error)) bool {
	if root == nil || read == nil || schemaDigest != reviewedSuccessorSchemaDigest ||
		state.stage != "VERIFIED" || before.stage != "VERIFIED" || after.stage != "VERIFIED" ||
		expected.Phase != "BEFORE" || !validHostGitPath(expected.Root) || !selectedGitCommit.MatchString(expected.Commit) ||
		finalizeSelectedGit(expected.Executable) != nil {
		return false
	}
	first, ok := readRevisionHost(root, before, expected, read)
	if !ok {
		return false
	}
	secondExpectation := expected
	secondExpectation.Phase = "AFTER"
	second, ok := readRevisionHost(root, after, secondExpectation, read)
	if !ok || first.ExecutableURI != second.ExecutableURI || first.ExecutableDigest != second.ExecutableDigest ||
		first.RootURI != second.RootURI || first.CwdURI != second.CwdURI || first.Commit != second.Commit {
		return false
	}
	lastBefore, err := hostGitTime(first.Commands[2].ObservedAt)
	if err != nil {
		return false
	}
	firstAfter, err := hostGitTime(second.Commands[0].ObservedAt)
	if err != nil || firstAfter.Before(lastBefore) {
		return false
	}
	b, err := read(root, state.selector, sourceRecordLimit)
	if err != nil || len(b) != state.byteCount || state.digest != sourceRoleDigest(revisionIdentityRole, b) || state.selector != sourceRecordSelector("revision", state.digest) || strictjson.RejectDuplicates(b) != nil {
		return false
	}
	var record revisionIdentityRecord
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if dec.Decode(&record) != nil {
		return false
	}
	var trailing any
	if dec.Decode(&trailing) != io.EOF {
		return false
	}
	canonical, err := canonicalRevisionIdentity(record)
	return err == nil && bytes.Equal(canonical, b) && record == (revisionIdentityRecord{
		SchemaVersion: revisionIdentityRole, RootURI: selectedGitFileURI(expected.Root), Commit: expected.Commit,
		Custody: "CALLER_ASSERTED", HostGitBeforeRef: revisionHostRef(before), HostGitAfterRef: revisionHostRef(after),
	})
}

// publishRevisionIdentity is a private synthetic checkpoint, not issuance.
func publishRevisionIdentity(root *publication.Root, before, after privateBodyPublication, expected hostGitExpectation, schemaDigest string, read func(*publication.Root, string, int64) ([]byte, error)) (privateBodyPublication, error) {
	absent := privateBodyPublication{stage: "ABSENT"}
	if read == nil {
		read = publication.ReadVerifiedBoundFile
	}
	if root == nil || schemaDigest != reviewedSuccessorSchemaDigest || expected.Phase != "BEFORE" || before.stage != "VERIFIED" || after.stage != "VERIFIED" || finalizeSelectedGit(expected.Executable) != nil {
		return absent, errRevisionIdentity
	}
	first, ok := readRevisionHost(root, before, expected, read)
	secondExpectation := expected
	secondExpectation.Phase = "AFTER"
	second, afterOK := readRevisionHost(root, after, secondExpectation, read)
	if !ok || !afterOK || first.RootURI != second.RootURI || first.CwdURI != second.CwdURI || first.Commit != second.Commit || first.ExecutableURI != second.ExecutableURI || first.ExecutableDigest != second.ExecutableDigest {
		return absent, errRevisionIdentity
	}
	lastBefore, e := hostGitTime(first.Commands[2].ObservedAt)
	firstAfter, ae := hostGitTime(second.Commands[0].ObservedAt)
	if e != nil || ae != nil || firstAfter.Before(lastBefore) {
		return absent, errRevisionIdentity
	}
	record := revisionIdentityRecord{revisionIdentityRole, selectedGitFileURI(expected.Root), expected.Commit, "CALLER_ASSERTED", revisionHostRef(before), revisionHostRef(after)}
	encoded, err := canonicalRevisionIdentity(record)
	if err != nil {
		return absent, errRevisionIdentity
	}
	digest := sourceRoleDigest(revisionIdentityRole, encoded)
	selector := sourceRecordSelector("revision", digest)
	receipt, err := publication.PublishBoundFile(root, selector, encoded, func(got []byte) error {
		if !bytes.Equal(got, encoded) {
			return errRevisionIdentity
		}
		return nil
	})
	state := privateBodyPublication{selector: selector, digest: digest, byteCount: len(encoded), stage: "COMMITTED_UNVERIFIED"}
	if err != nil || receipt == nil || !verifiedTargetPublication(receipt, selector, privateDigest(encoded), len(encoded)) {
		return state, errRevisionIdentity
	}
	// Only this publisher may replay its just-committed record before promotion.
	state.stage = "VERIFIED"
	if !replayRevisionIdentity(root, state, before, after, expected, schemaDigest, read) {
		state.stage = "COMMITTED_UNVERIFIED"
		return state, errRevisionIdentity
	}
	return state, nil
}
