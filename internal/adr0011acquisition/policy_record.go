package adr0011acquisition

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"lsp-trace/internal/publication"
	"lsp-trace/internal/strictjson"
)

// These are synthetic, private successor receipts. They convey no policy authority.
const (
	methodPolicyRole    = "REFERENCES_METHOD_POLICY_V1"
	admissionPolicyRole = "REFERENCES_ADMISSION_POLICY_V1"
	privacyPolicyRole   = "LOCAL_QUALIFICATION_PRIVACY_V1"
	retentionPolicyRole = "REFERENCES_RETENTION_POLICY_V1"
	policyByteLimit     = 1048576
)

var errPolicyRecord = errors.New("private policy record not verified")

type policySelection struct{ role, name, pin, selectorRole string }

var policySelections = [4]policySelection{
	{methodPolicyRole, "method", "2fb6c0174945a92b34ccffaa751d15eaa737b7349e2178899d8ed2df00a4710f", "method-policy"},
	{admissionPolicyRole, "admission", "483a0db3ac8f56f043aacd00a7cb7e3be0361dbaf9e595ba0a6d942a245de4f3", "admission-policy"},
	{privacyPolicyRole, "privacy", "8d5f9bfba89bab65ad4a44bf89e95f4fd427b7fc2227966ef2be6468e363770f", "privacy-policy"},
	{retentionPolicyRole, "retention", "5e4a817e8dc8aed4bc6053e18e1f4dfda00a159cf909fed37e0ae39f4d1baa68", "retention-policy"},
}

type policyBytesRef struct {
	Selector string `json:"selector"`
	Digest   string `json:"digest"`
}
type policyRecord struct {
	SchemaVersion  string         `json:"schema_version"`
	PolicyBytesRef policyBytesRef `json:"policy_bytes_ref"`
}
type policyPublication struct{ Bytes, Record privateBodyPublication }

func policySourcePath(s policySelection) string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "docs", "qualification", "policies", "adr0011-references-"+s.name+"-policy-v1.proposed.json")
}

// readPolicySource is injected only by package tests; the owner selects the path and pin.
type policySourceReader func(string) ([]byte, error)

func diskPolicySource(path string) ([]byte, error) {
	f, e := os.Open(path)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	st, e := f.Stat()
	if e != nil || !st.Mode().IsRegular() || st.Size() < 1 || st.Size() > policyByteLimit {
		return nil, errPolicyRecord
	}
	b, e := io.ReadAll(io.LimitReader(f, policyByteLimit+1))
	if e != nil || len(b) < 1 || len(b) > policyByteLimit || int64(len(b)) != st.Size() {
		return nil, errPolicyRecord
	}
	return b, nil
}
func selectedPolicyBytes(s policySelection, source policySourceReader) ([]byte, error) {
	if source == nil {
		source = diskPolicySource
	}
	b, e := source(policySourcePath(s))
	if e != nil || len(b) < 1 || len(b) > policyByteLimit || bytes.ContainsAny(b, "\r\n") || fmt.Sprintf("%x", sha256.Sum256(b)) != s.pin || strictjson.RejectDuplicates(b) != nil {
		return nil, errPolicyRecord
	}
	var v map[string]json.RawMessage
	dec := json.NewDecoder(bytes.NewReader(b))
	if dec.Decode(&v) != nil || dec.Decode(new(any)) != io.EOF || len(v) < 2 {
		return nil, errPolicyRecord
	}
	version, ok := v["schema_version"]
	if !ok || !bytes.Equal(version, []byte(strconvPolicyString(s.role))) {
		return nil, errPolicyRecord
	}
	// The pinned exact bytes are the closed policy object. Re-encoding must preserve
	// JCS spelling and key order, not merely parse to an equivalent JSON value.
	canonical, e := json.Marshal(v)
	if e != nil || !bytes.Equal(canonical, b) {
		return nil, errPolicyRecord
	}
	return b, nil
}
func strconvPolicyString(v string) string { b, _ := json.Marshal(v); return string(b) }
func policyBytesSelector(d string) string {
	return "adr0011-references-policy-bytes-v1-" + strings.TrimPrefix(d, "sha256:") + ".json"
}
func policyRecordSelector(s policySelection, d string) string {
	return sourceRecordSelector(s.selectorRole, d)
}
func canonicalPolicyRecord(s policySelection, b []byte) ([]byte, error) {
	d := privateDigest(b)
	if !targetResultValidDigest(d) {
		return nil, errPolicyRecord
	}
	return []byte(`{"policy_bytes_ref":{"digest":"` + d + `","selector":"` + policyBytesSelector(d) + `"},"schema_version":"` + s.role + `"}`), nil
}
func validPolicyReceipt(r *publication.BoundFileReceipt, selector, digest string, n int) bool {
	return r != nil && r.VerificationStatus == "VERIFIED" && r.Mechanism == publication.BoundFileMechanism && r.DirectorySyncStatus == publication.DirectorySyncComplete && r.CloseStatus == publication.CloseComplete && r.FinalSelector == selector && r.Digest == digest && r.ByteLength == uint64(n) && r.NamespaceAtomic
}
func replayPolicy(root *publication.Root, s policySelection, state policyPublication, source policySourceReader, read func(*publication.Root, string, int64) ([]byte, error)) bool {
	if root == nil || state.Bytes.stage != "VERIFIED" || state.Record.stage != "VERIFIED" {
		return false
	}
	if read == nil {
		read = publication.ReadVerifiedBoundFile
	}
	original, e := selectedPolicyBytes(s, source)
	if e != nil {
		return false
	}
	bd := privateDigest(original)
	if state.Bytes.selector != policyBytesSelector(bd) || state.Bytes.digest != bd || state.Bytes.byteCount != len(original) {
		return false
	}
	got, e := read(root, state.Bytes.selector, policyByteLimit)
	if e != nil || !bytes.Equal(got, original) {
		return false
	}
	canonical, _ := canonicalPolicyRecord(s, original)
	rd := sourceRoleDigest(s.role, canonical)
	if state.Record.selector != policyRecordSelector(s, rd) || state.Record.digest != rd || state.Record.byteCount != len(canonical) {
		return false
	}
	got, e = read(root, state.Record.selector, policyByteLimit)
	if e != nil || strictjson.RejectDuplicates(got) != nil {
		return false
	}
	var actual policyRecord
	dec := json.NewDecoder(bytes.NewReader(got))
	dec.DisallowUnknownFields()
	if dec.Decode(&actual) != nil || dec.Decode(new(any)) != io.EOF || actual.SchemaVersion != s.role || actual.PolicyBytesRef != (policyBytesRef{policyBytesSelector(bd), bd}) {
		return false
	}
	return bytes.Equal(got, canonical)
}
func publishPolicy(root *publication.Root, s policySelection, source policySourceReader, read func(*publication.Root, string, int64) ([]byte, error)) (policyPublication, error) {
	absent := policyPublication{Bytes: privateBodyPublication{stage: "ABSENT"}, Record: privateBodyPublication{stage: "ABSENT"}}
	if root == nil {
		return absent, errPolicyRecord
	}
	if read == nil {
		read = publication.ReadVerifiedBoundFile
	}
	b, e := selectedPolicyBytes(s, source)
	if e != nil {
		return absent, errPolicyRecord
	}
	bd := privateDigest(b)
	bs := policyBytesSelector(bd)
	br, precommit, e := publishPolicyBound(root, bs, b)
	if e != nil || !validPolicyReceipt(br, bs, bd, len(b)) {
		return policyPublication{Bytes: policyStageOnFailure(bs, bd, len(b), precommit), Record: absent.Record}, errPolicyRecord
	}
	state := absent
	state.Bytes = privateBodyPublication{selector: bs, digest: bd, byteCount: len(b), stage: "COMMITTED_UNVERIFIED"}
	got, e := read(root, bs, policyByteLimit)
	if e != nil || !bytes.Equal(got, b) {
		return state, errPolicyRecord
	}
	state.Bytes.stage = "VERIFIED"
	// Source selection is repeated after byte publication to reject in-flight swaps.
	again, e := selectedPolicyBytes(s, source)
	if e != nil || !bytes.Equal(again, b) {
		return state, errPolicyRecord
	}
	record, _ := canonicalPolicyRecord(s, b)
	rd := sourceRoleDigest(s.role, record)
	rs := policyRecordSelector(s, rd)
	rr, precommit, e := publishPolicyBound(root, rs, record)
	if e != nil || !validPolicyReceipt(rr, rs, privateDigest(record), len(record)) {
		state.Record = policyStageOnFailure(rs, rd, len(record), precommit)
		return state, errPolicyRecord
	}
	state.Record = privateBodyPublication{selector: rs, digest: rd, byteCount: len(record), stage: "COMMITTED_UNVERIFIED"}
	got, e = read(root, rs, policyByteLimit)
	if e != nil || !bytes.Equal(got, record) {
		return state, errPolicyRecord
	}
	state.Record.stage = "VERIFIED"
	if !replayPolicy(root, s, state, source, read) {
		state.Record.stage = "COMMITTED_UNVERIFIED"
		return state, errPolicyRecord
	}
	return state, nil
}
func publishPolicyBound(root *publication.Root, selector string, content []byte) (*publication.BoundFileReceipt, bool, error) {
	precommit, uncertain := false, false
	receipt, err := publication.PublishBoundFileWithTrace(root, selector, content, func(got []byte) error {
		if !bytes.Equal(got, content) {
			return errPolicyRecord
		}
		return nil
	}, func(event publication.BoundFileTraceEvent) {
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
	return receipt, precommit && !uncertain, err
}
func policyStageOnFailure(selector, digest string, n int, precommit bool) privateBodyPublication {
	if precommit {
		return privateBodyPublication{stage: "ABSENT"}
	}
	return privateBodyPublication{stage: "COMMITTED_UNVERIFIED", selector: selector, digest: digest, byteCount: n}
}

func publishMethodPolicy(root *publication.Root) (policyPublication, error) {
	return publishPolicy(root, policySelections[0], nil, nil)
}
func publishAdmissionPolicy(root *publication.Root) (policyPublication, error) {
	return publishPolicy(root, policySelections[1], nil, nil)
}
func publishPrivacyPolicy(root *publication.Root) (policyPublication, error) {
	return publishPolicy(root, policySelections[2], nil, nil)
}
func publishRetentionPolicy(root *publication.Root) (policyPublication, error) {
	return publishPolicy(root, policySelections[3], nil, nil)
}
func replayMethodPolicy(root *publication.Root, p policyPublication) bool {
	return replayPolicy(root, policySelections[0], p, nil, nil)
}
func replayAdmissionPolicy(root *publication.Root, p policyPublication) bool {
	return replayPolicy(root, policySelections[1], p, nil, nil)
}
func replayPrivacyPolicy(root *publication.Root, p policyPublication) bool {
	return replayPolicy(root, policySelections[2], p, nil, nil)
}
func replayRetentionPolicy(root *publication.Root, p policyPublication) bool {
	return replayPolicy(root, policySelections[3], p, nil, nil)
}
