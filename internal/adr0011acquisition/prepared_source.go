package adr0011acquisition

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"lsp-trace/internal/publication"
	"lsp-trace/internal/strictjson"
	"lsp-trace/sessionruntime"
)

const preparedSourceRole = "REFERENCES_PREPARED_SOURCE_V1"
const sourceIdentityRole = "REFERENCES_SOURCE_IDENTITY_V1"
const sourceRecordLimit = 1500000

var errPreparedSource = errors.New("private prepared source not verified")

type preparedSourceRecord struct {
	SchemaVersion  string `json:"schema_version"`
	URI            string `json:"uri"`
	Version        int    `json:"version"`
	Encoding       string `json:"encoding"`
	TextDigest     string `json:"text_digest"`
	TextByteLength int    `json:"text_byte_length"`
}
type sourceIdentityRecord struct {
	SchemaVersion     string          `json:"schema_version"`
	PreparedURI       string          `json:"prepared_uri"`
	PreparedVersion   int             `json:"prepared_version"`
	PreparedDigest    string          `json:"prepared_digest"`
	PositionEncoding  string          `json:"position_encoding"`
	PreparedSourceRef targetResultRef `json:"prepared_source_ref"`
}

func sourceRoleDigest(role string, b []byte) string {
	h := sha256.New()
	h.Write([]byte(role))
	h.Write([]byte{0})
	h.Write(b)
	return fmt.Sprintf("sha256:%x", h.Sum(nil))
}
func sourceRecordSelector(role, digest string) string {
	return "adr0011-references-issuance-v1-" + role + "-" + strings.TrimPrefix(digest, "sha256:") + ".json"
}
func preparedTextSelector(digest string) string {
	return "adr0011-references-prepared-text-v1-" + strings.TrimPrefix(digest, "sha256:") + ".bin"
}

// canonicalPreparedURI is a lexical admission guard; Manager independently
// acquires the prepared bytes through its descriptor-bound no-follow mode.
func canonicalPreparedURI(uri, workspace string) bool {
	u, err := url.Parse(uri)
	if err != nil || u.Scheme != "file" || u.Host != "" || u.Path == "" || u.RawQuery != "" || u.Fragment != "" || u.RawFragment != "" || u.User != nil || u.Opaque != "" || u.ForceQuery || !filepath.IsAbs(workspace) || filepath.Clean(workspace) != workspace {
		return false
	}
	path := u.Path
	if !filepath.IsAbs(path) || filepath.Clean(path) != path || !utf8.ValidString(path) || strings.ContainsRune(path, 0) {
		return false
	}
	const hex = "0123456789ABCDEF"
	var canonical strings.Builder
	canonical.WriteString("file://")
	for i := 0; i < len(path); i++ {
		c := path[i]
		if c == '/' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '.' || c == '_' || c == '~' {
			canonical.WriteByte(c)
		} else {
			canonical.WriteByte('%')
			canonical.WriteByte(hex[c>>4])
			canonical.WriteByte(hex[c&15])
		}
	}
	if uri != canonical.String() {
		return false
	}
	rel, err := filepath.Rel(workspace, path)
	return err == nil && rel != "." && rel != ".." && !filepath.IsAbs(rel) && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
func canonicalSourceRecord(v any) ([]byte, error) {
	var b bytes.Buffer
	switch r := v.(type) {
	case preparedSourceRecord:
		if r.SchemaVersion != preparedSourceRole || r.Encoding != "utf-16" || r.Version < 0 || r.TextByteLength < 0 || r.TextByteLength > sessionruntime.MaxDocumentSupplyBytes || !targetResultValidDigest(r.TextDigest) {
			return nil, errPreparedSource
		}
		b.WriteString(`{"encoding":`)
		writeJCSString(&b, r.Encoding)
		b.WriteString(`,"schema_version":`)
		writeJCSString(&b, r.SchemaVersion)
		b.WriteString(`,"text_byte_length":`)
		fmt.Fprint(&b, r.TextByteLength)
		b.WriteString(`,"text_digest":`)
		writeJCSString(&b, r.TextDigest)
		b.WriteString(`,"uri":`)
		writeJCSString(&b, r.URI)
		b.WriteString(`,"version":`)
		fmt.Fprint(&b, r.Version)
	case sourceIdentityRecord:
		if r.SchemaVersion != sourceIdentityRole || r.PositionEncoding != "utf-16" || r.PreparedVersion < 0 || !targetResultValidDigest(r.PreparedDigest) || r.PreparedSourceRef.SchemaVersion != preparedSourceRole || !targetResultValidDigest(r.PreparedSourceRef.Digest) || r.PreparedSourceRef.Selector != sourceRecordSelector("prepared-source", r.PreparedSourceRef.Digest) {
			return nil, errPreparedSource
		}
		b.WriteString(`{"position_encoding":`)
		writeJCSString(&b, r.PositionEncoding)
		b.WriteString(`,"prepared_digest":`)
		writeJCSString(&b, r.PreparedDigest)
		b.WriteString(`,"prepared_source_ref":{"digest":`)
		writeJCSString(&b, r.PreparedSourceRef.Digest)
		b.WriteString(`,"schema_version":`)
		writeJCSString(&b, r.PreparedSourceRef.SchemaVersion)
		b.WriteString(`,"selector":`)
		writeJCSString(&b, r.PreparedSourceRef.Selector)
		b.WriteString(`},"prepared_uri":`)
		writeJCSString(&b, r.PreparedURI)
		b.WriteString(`,"prepared_version":`)
		fmt.Fprint(&b, r.PreparedVersion)
		b.WriteString(`,"schema_version":`)
		writeJCSString(&b, r.SchemaVersion)
	default:
		return nil, errPreparedSource
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}
func decodeSourceRecord(raw []byte, v any) bool {
	if len(raw) == 0 || len(raw) > sourceRecordLimit || strictjson.RejectDuplicates(raw) != nil {
		return false
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if dec.Decode(v) != nil {
		return false
	}
	var extra any
	return dec.Decode(&extra) == io.EOF
}
func replayPreparedSource(root *publication.Root, prepared, sourceRef privateBodyPublication, original []byte, uri string, version int, workspace, schemaDigest string, read func(*publication.Root, string, int64) ([]byte, error)) bool {
	if root == nil || read == nil || schemaDigest != reviewedSuccessorSchemaDigest || !canonicalPreparedURI(uri, workspace) || version < 0 || len(original) == 0 || len(original) > sessionruntime.MaxDocumentSupplyBytes || prepared.stage != "VERIFIED" || sourceRef.stage == "ABSENT" {
		return false
	}
	digest := privateDigest(original)
	payload, e := read(root, preparedTextSelector(digest), sessionruntime.MaxDocumentSupplyBytes)
	if e != nil || !bytes.Equal(payload, original) {
		return false
	}
	pbytes, e := read(root, prepared.selector, sourceRecordLimit)
	if e != nil || prepared.digest != sourceRoleDigest(preparedSourceRole, pbytes) || prepared.selector != sourceRecordSelector("prepared-source", prepared.digest) || prepared.byteCount != len(pbytes) {
		return false
	}
	var p preparedSourceRecord
	if !decodeSourceRecord(pbytes, &p) {
		return false
	}
	canonical, e := canonicalSourceRecord(p)
	if e != nil || !bytes.Equal(canonical, pbytes) || p != (preparedSourceRecord{preparedSourceRole, uri, version, "utf-16", digest, len(original)}) {
		return false
	}
	sbytes, e := read(root, sourceRef.selector, sourceRecordLimit)
	if e != nil || sourceRef.digest != sourceRoleDigest(sourceIdentityRole, sbytes) || sourceRef.selector != sourceRecordSelector("source", sourceRef.digest) || sourceRef.byteCount != len(sbytes) {
		return false
	}
	var s sourceIdentityRecord
	if !decodeSourceRecord(sbytes, &s) {
		return false
	}
	canonical, e = canonicalSourceRecord(s)
	return e == nil && bytes.Equal(canonical, sbytes) && s == (sourceIdentityRecord{sourceIdentityRole, uri, version, digest, "utf-16", targetResultRef{preparedSourceRole, prepared.selector, prepared.digest}})
}
func publishPreparedSource(root *publication.Root, original []byte, uri string, version int, workspace, schemaDigest string, read func(*publication.Root, string, int64) ([]byte, error)) (privateBodyPublication, privateBodyPublication, error) {
	absent := privateBodyPublication{stage: "ABSENT"}
	if root == nil || schemaDigest != reviewedSuccessorSchemaDigest || !canonicalPreparedURI(uri, workspace) || version < 0 || len(original) == 0 || len(original) > sessionruntime.MaxDocumentSupplyBytes {
		return absent, absent, errPreparedSource
	}
	if read == nil {
		read = publication.ReadVerifiedBoundFile
	}
	digest := privateDigest(original)
	selector := preparedTextSelector(digest)
	publish := func(selector string, body []byte) (privateBodyPublication, error) {
		receipt, e := publication.PublishBoundFile(root, selector, body, func(got []byte) error {
			if !bytes.Equal(got, body) {
				return errPreparedSource
			}
			return nil
		})
		if e != nil || receipt == nil {
			return absent, errPreparedSource
		}
		state := privateBodyPublication{selector: selector, digest: privateDigest(body), byteCount: len(body), stage: "COMMITTED_UNVERIFIED"}
		if !verifiedTargetPublication(receipt, selector, state.digest, len(body)) {
			return state, errPreparedSource
		}
		got, e := read(root, selector, int64(max(len(body), 1)))
		if e != nil || !bytes.Equal(got, body) {
			return state, errPreparedSource
		}
		state.stage = "VERIFIED"
		return state, nil
	}
	payload, e := publish(selector, original)
	if e != nil {
		return payload, absent, e
	}
	p := preparedSourceRecord{preparedSourceRole, uri, version, "utf-16", digest, len(original)}
	encoded, e := canonicalSourceRecord(p)
	if e != nil {
		return payload, absent, e
	}
	pd := sourceRoleDigest(preparedSourceRole, encoded)
	pr, e := publish(sourceRecordSelector("prepared-source", pd), encoded)
	if e != nil {
		return pr, absent, e
	}
	pr.digest = pd
	s := sourceIdentityRecord{sourceIdentityRole, uri, version, digest, "utf-16", targetResultRef{preparedSourceRole, pr.selector, pr.digest}}
	encoded, e = canonicalSourceRecord(s)
	if e != nil {
		return pr, absent, e
	}
	sd := sourceRoleDigest(sourceIdentityRole, encoded)
	sr, e := publish(sourceRecordSelector("source", sd), encoded)
	if e != nil {
		return pr, sr, e
	}
	sr.digest = sd
	if !replayPreparedSource(root, pr, sr, original, uri, version, workspace, schemaDigest, read) {
		sr.stage = "COMMITTED_UNVERIFIED"
		return pr, sr, errPreparedSource
	}
	return pr, sr, nil
}
