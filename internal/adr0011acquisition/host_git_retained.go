package adr0011acquisition

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
	"lsp-trace/internal/publication"
	"lsp-trace/internal/strictjson"
)

const hostGitRole = "REFERENCES_HOST_GIT_OBSERVATION_V1"

var errHostGit = errors.New("private host Git observation not verified")
var hostGitUTC = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d{1,9})?Z$`)
var hostGitArgv = [3][]string{{"git", "rev-parse", "--show-toplevel"}, {"git", "rev-parse", "HEAD"}, {"git", "status", "--porcelain=v1", "--untracked-files=all"}}

type hostGitOutput struct {
	Selector   string `json:"selector"`
	Digest     string `json:"digest"`
	ByteLength int    `json:"byte_length"`
}
type hostGitCommand struct {
	Argv       []string      `json:"argv"`
	ExitStatus int           `json:"exit_status"`
	Stdout     hostGitOutput `json:"stdout"`
	Stderr     hostGitOutput `json:"stderr"`
	ObservedAt string        `json:"observed_at"`
}
type hostGitRecord struct {
	SchemaVersion    string            `json:"schema_version"`
	RootURI          string            `json:"root_uri"`
	Commit           string            `json:"commit"`
	Dirty            bool              `json:"dirty"`
	ObservationPhase string            `json:"observation_phase"`
	Custody          string            `json:"custody"`
	ExecutableURI    string            `json:"executable_uri"`
	ExecutableDigest string            `json:"executable_digest"`
	CwdURI           string            `json:"cwd_uri"`
	Commands         [3]hostGitCommand `json:"commands"`
}

// hostGitExpectation must be chosen independently of the observation and its record.
type hostGitExpectation struct {
	Executable selectedGitExecutable
	Root       string
	Commit     string
	Phase      string
}

func hostGitStreamSelector(d string) string {
	return "adr0011-references-host-git-output-v1-" + strings.TrimPrefix(d, "sha256:") + ".bin"
}
func hostGitRecordSelector(d string) string { return sourceRecordSelector("host-git", d) }
func hostGitStreamRef(b []byte) hostGitOutput {
	d := privateDigest(b)
	return hostGitOutput{hostGitStreamSelector(d), d, len(b)}
}
func validHostGitPath(path string) bool {
	if !utf8.ValidString(path) || !norm.NFC.IsNormalString(path) || !strings.HasPrefix(path, "/") || strings.ContainsAny(path, "\x00\r\n") || filepath.Clean(path) != path {
		return false
	}
	for _, s := range strings.Split(path, "/") {
		if s == "." || s == ".." {
			return false
		}
	}
	return true
}
func hostGitTime(s string) (time.Time, error) {
	if !hostGitUTC.MatchString(s) {
		return time.Time{}, errHostGit
	}
	t, e := time.Parse(time.RFC3339Nano, s)
	if e != nil {
		return time.Time{}, errHostGit
	}
	return t, nil
}
func hostGitEligible(r hostGitRecord, x hostGitExpectation, streams [3][2][]byte) bool {
	if !validHostGitPath(x.Root) || x.Executable.path == "" || !validHostGitPath(x.Executable.path) || x.Executable.uri != selectedGitFileURI(x.Executable.path) || !targetResultValidDigest(x.Executable.digest) || !selectedGitCommit.MatchString(x.Commit) || (x.Phase != "BEFORE" && x.Phase != "AFTER") || r.SchemaVersion != hostGitRole || r.RootURI != selectedGitFileURI(x.Root) || r.CwdURI != r.RootURI || r.ExecutableURI != x.Executable.uri || r.ExecutableDigest != x.Executable.digest || r.Commit != x.Commit || r.Dirty || r.Custody != "HOST_OBSERVED_GIT" || r.ObservationPhase != x.Phase {
		return false
	}
	var previous time.Time
	for i, c := range r.Commands {
		if len(c.Argv) != len(hostGitArgv[i]) || c.ExitStatus != 0 || len(streams[i][0]) > maxSelectedGitStream || len(streams[i][1]) > maxSelectedGitStream || len(streams[i][1]) != 0 || c.Stdout != hostGitStreamRef(streams[i][0]) || c.Stderr != hostGitStreamRef(streams[i][1]) {
			return false
		}
		for j, a := range c.Argv {
			if a != hostGitArgv[i][j] {
				return false
			}
		}
		t, e := hostGitTime(c.ObservedAt)
		if e != nil || i > 0 && t.Before(previous) {
			return false
		}
		previous = t
	}
	root := streams[0][0]
	if len(root) < 2 || root[len(root)-1] != '\n' || !validHostGitPath(string(root[:len(root)-1])) || selectedGitFileURI(string(root[:len(root)-1])) != r.RootURI || !bytes.Equal(streams[1][0], []byte(x.Commit+"\n")) || len(streams[2][0]) != 0 {
		return false
	}
	return true
}
func canonicalHostGit(r hostGitRecord) ([]byte, error) {
	var b bytes.Buffer
	field := func(name, value string, comma bool) {
		if comma {
			b.WriteByte(',')
		}
		writeJCSString(&b, name)
		b.WriteByte(':')
		writeJCSString(&b, value)
	}
	output := func(o hostGitOutput) {
		b.WriteString(`{"byte_length":`)
		fmt.Fprint(&b, o.ByteLength)
		b.WriteString(`,"digest":`)
		writeJCSString(&b, o.Digest)
		b.WriteString(`,"selector":`)
		writeJCSString(&b, o.Selector)
		b.WriteByte('}')
	}
	b.WriteByte('{')
	field("commit", r.Commit, false)
	b.WriteString(`,"commands":[`)
	for i, c := range r.Commands {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(`{"argv":[`)
		for j, a := range c.Argv {
			if j > 0 {
				b.WriteByte(',')
			}
			writeJCSString(&b, a)
		}
		b.WriteString(`],"exit_status":`)
		fmt.Fprint(&b, c.ExitStatus)
		b.WriteString(`,"observed_at":`)
		writeJCSString(&b, c.ObservedAt)
		b.WriteString(`,"stderr":`)
		output(c.Stderr)
		b.WriteString(`,"stdout":`)
		output(c.Stdout)
		b.WriteByte('}')
	}
	b.WriteString(`],"custody":`)
	writeJCSString(&b, r.Custody)
	field("cwd_uri", r.CwdURI, true)
	b.WriteString(`,"dirty":`)
	if r.Dirty {
		b.WriteString("true")
	} else {
		b.WriteString("false")
	}
	field("executable_digest", r.ExecutableDigest, true)
	field("executable_uri", r.ExecutableURI, true)
	field("observation_phase", r.ObservationPhase, true)
	field("root_uri", r.RootURI, true)
	field("schema_version", r.SchemaVersion, true)
	b.WriteByte('}')
	if b.Len() > sourceRecordLimit || !utf8.Valid(b.Bytes()) || !norm.NFC.IsNormal(b.Bytes()) {
		return nil, errHostGit
	}
	return b.Bytes(), nil
}
func decodeHostGit(b []byte) (hostGitRecord, bool) {
	var r hostGitRecord
	if len(b) == 0 || len(b) > sourceRecordLimit || !utf8.Valid(b) || !norm.NFC.IsNormal(b) || rejectHostGitDuplicates(b) != nil {
		return r, false
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if dec.Decode(&r) != nil {
		return r, false
	}
	var extra any
	if dec.Decode(&extra) != io.EOF {
		return r, false
	}
	return r, true
}
func rejectHostGitDuplicates(b []byte) error { return strictjson.RejectDuplicates(b) }

// hostGitVerifiedStreams is owned by one private transaction, never global.
// Retain the verified no-replace receipt, not just a matching content hash.
type hostGitVerifiedStream struct {
	output  hostGitOutput
	receipt publication.BoundFileReceipt
	stage   string
}
type hostGitVerifiedStreams map[string]hostGitVerifiedStream

// replayHostGit reads the record and every stream independently. The record is
// never trusted to choose its executable, root, revision, phase, or schema.
func replayHostGit(root *publication.Root, state privateBodyPublication, x hostGitExpectation, schemaDigest string, read func(*publication.Root, string, int64) ([]byte, error)) bool {
	return replayHostGitInternal(root, state, x, schemaDigest, read, false)
}

// Only the private publisher may replay a committed record before promotion.
func replayHostGitInternal(root *publication.Root, state privateBodyPublication, x hostGitExpectation, schemaDigest string, read func(*publication.Root, string, int64) ([]byte, error), promoting bool) bool {
	if root == nil || read == nil || schemaDigest != reviewedSuccessorSchemaDigest || (state.stage != "VERIFIED" && !(promoting && state.stage == "COMMITTED_UNVERIFIED")) {
		return false
	}
	b, e := read(root, state.selector, sourceRecordLimit)
	if e != nil || len(b) != state.byteCount || state.digest != sourceRoleDigest(hostGitRole, b) || state.selector != hostGitRecordSelector(state.digest) {
		return false
	}
	r, ok := decodeHostGit(b)
	if !ok {
		return false
	}
	canonical, e := canonicalHostGit(r)
	if e != nil || !bytes.Equal(b, canonical) {
		return false
	}
	var streams [3][2][]byte
	for i, c := range r.Commands {
		for j, o := range [2]hostGitOutput{c.Stdout, c.Stderr} {
			if o.ByteLength < 0 || o.ByteLength > maxSelectedGitStream || !targetResultValidDigest(o.Digest) || o.Selector != hostGitStreamSelector(o.Digest) {
				return false
			}
			got, e := read(root, o.Selector, maxSelectedGitStream)
			if e != nil || len(got) != o.ByteLength || privateDigest(got) != o.Digest {
				return false
			}
			streams[i][j] = got
		}
	}
	return hostGitEligible(r, x, streams)
}

// publishHostGit is synthetic-only: callers supply an already observed value.
// On failure it returns the last potentially committed selector, never issuance.
func publishHostGit(root *publication.Root, verified hostGitVerifiedStreams, observed selectedGitObservation, x hostGitExpectation, schemaDigest string, read func(*publication.Root, string, int64) ([]byte, error)) (privateBodyPublication, error) {
	absent := privateBodyPublication{stage: "ABSENT"}
	if root == nil || verified == nil || schemaDigest != reviewedSuccessorSchemaDigest {
		return absent, errHostGit
	}
	if read == nil {
		read = publication.ReadVerifiedBoundFile
	}
	r := hostGitRecord{SchemaVersion: hostGitRole, RootURI: observed.rootURI, Commit: observed.commit, Dirty: observed.dirty, ObservationPhase: x.Phase, Custody: "HOST_OBSERVED_GIT", ExecutableURI: observed.executable.uri, ExecutableDigest: observed.executable.digest, CwdURI: observed.cwdURI}
	var streams [3][2][]byte
	for i, c := range observed.commands {
		streams[i] = [2][]byte{c.stdout, c.stderr}
		r.Commands[i] = hostGitCommand{append([]string(nil), c.argv...), c.exit, hostGitStreamRef(c.stdout), hostGitStreamRef(c.stderr), c.at.Format(time.RFC3339Nano)}
	}
	if !hostGitEligible(r, x, streams) {
		return absent, errHostGit
	}
	encoded, e := canonicalHostGit(r)
	if e != nil {
		return absent, errHostGit
	}
	seen := map[string]bool{}
	for _, pair := range streams {
		for _, data := range pair {
			o := hostGitStreamRef(data)
			if seen[o.Selector] {
				continue
			}
			seen[o.Selector] = true
			state := privateBodyPublication{selector: o.Selector, digest: o.Digest, byteCount: len(data), stage: "COMMITTED_UNVERIFIED"}
			if known, ok := verified[o.Selector]; ok {
				got, e := read(root, o.Selector, maxSelectedGitStream)
				if e != nil || known.stage != "VERIFIED" || known.output != o || !verifiedTargetPublication(&known.receipt, o.Selector, o.Digest, len(data)) || !bytes.Equal(got, data) {
					delete(verified, o.Selector)
					return state, errHostGit
				}
				continue
			}
			// PublishBoundFile requires a non-nil slice even for the real empty object.
			body := append([]byte{}, data...)
			receipt, e := publication.PublishBoundFile(root, o.Selector, body, func(got []byte) error {
				if !bytes.Equal(got, data) {
					return errHostGit
				}
				return nil
			})
			if e != nil || receipt == nil {
				return state, errHostGit
			}
			if !verifiedTargetPublication(receipt, o.Selector, o.Digest, len(data)) {
				return state, errHostGit
			}
			got, e := read(root, o.Selector, maxSelectedGitStream)
			if e != nil || !bytes.Equal(got, data) {
				return state, errHostGit
			}
			verified[o.Selector] = hostGitVerifiedStream{output: o, receipt: *receipt, stage: "VERIFIED"}
		}
	}
	digest := sourceRoleDigest(hostGitRole, encoded)
	selector := hostGitRecordSelector(digest)
	receipt, e := publication.PublishBoundFile(root, selector, encoded, func(got []byte) error {
		if !bytes.Equal(got, encoded) {
			return errHostGit
		}
		return nil
	})
	state := privateBodyPublication{selector: selector, digest: digest, byteCount: len(encoded), stage: "COMMITTED_UNVERIFIED"}
	if e != nil || receipt == nil {
		return state, errHostGit
	}
	if !verifiedTargetPublication(receipt, selector, privateDigest(encoded), len(encoded)) || !replayHostGitInternal(root, state, x, schemaDigest, read, true) {
		return state, errHostGit
	}
	state.stage = "VERIFIED"
	return state, nil
}
