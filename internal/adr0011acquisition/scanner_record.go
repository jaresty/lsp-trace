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

const scannerRecordRole = "REFERENCES_SCANNER_OBSERVATION_V1"
const scannerSourcePin = "sha256:2eaad298bc11a317c5ac0a7ff38f536f4dd616304264f3f62a24cf8ca9941730"
const scannerSourceLimit = 1 << 20

var errScannerRecord = errors.New("private references scanner observation not verified")

type scannerRecord struct {
	SchemaVersion               string                `json:"schema_version"`
	ResponseReadRef             targetRecordRefFields `json:"response_read_ref"`
	RawResultRef                targetRecordRefFields `json:"raw_result_ref"`
	ScannerImplementationDigest string                `json:"scanner_implementation_digest"`
	TopLevelForm                string                `json:"top_level_form"`
	KnownE                      int                   `json:"known_e"`
	WorkUnits                   int                   `json:"work_units"`
}

// The caller location belongs to this compiled package, never to the record.
// Read one byte past the bound so a truncated source cannot match the pin.
func scannerSourceBytes() ([]byte, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok || filepath.Base(file) != "scanner_record.go" {
		return nil, errScannerRecord
	}
	f, err := os.Open(filepath.Join(filepath.Dir(file), "raw_scanner.go"))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, scannerSourceLimit+1))
	if err != nil || len(b) == 0 || len(b) > scannerSourceLimit {
		return nil, errScannerRecord
	}
	return b, nil
}

func selectedScannerPin(source func() ([]byte, error)) (string, bool) {
	if source == nil {
		source = scannerSourceBytes
	}
	b, err := source()
	if err != nil || len(b) == 0 || len(b) > scannerSourceLimit || fmt.Sprintf("sha256:%x", sha256.Sum256(b)) != scannerSourcePin {
		return "", false
	}
	return scannerSourcePin, true
}

func canonicalScannerRecord(r scannerRecord) ([]byte, error) {
	if r.SchemaVersion != scannerRecordRole || r.ResponseReadRef.SchemaVersion != responseReadRole || r.RawResultRef.SchemaVersion != rawResultRole || !validPrivateDigest(r.ResponseReadRef.Digest) || !validPrivateDigest(r.RawResultRef.Digest) || r.ResponseReadRef.Selector != responseReadSelector(r.ResponseReadRef.Digest) || r.RawResultRef.Selector != rawResultSelector(r.RawResultRef.Digest) || r.ScannerImplementationDigest != scannerSourcePin || (r.TopLevelForm != "NULL" && r.TopLevelForm != "ARRAY") || r.KnownE < 0 || r.KnownE > 1000 || (r.TopLevelForm == "NULL" && r.KnownE != 0) || r.WorkUnits < 1 || r.WorkUnits > rawResultPayloadLimit {
		return nil, errScannerRecord
	}
	field := func(s string) string { return string(targetIdentityStringJSON(s)) }
	b := []byte(fmt.Sprintf(`{"known_e":%d,"raw_result_ref":%s,"response_read_ref":%s,"scanner_implementation_digest":%s,"schema_version":%s,"top_level_form":%s,"work_units":%d}`, r.KnownE, targetIdentityRefJSON(targetResultRef{r.RawResultRef.SchemaVersion, r.RawResultRef.Selector, r.RawResultRef.Digest}), targetIdentityRefJSON(targetResultRef{r.ResponseReadRef.SchemaVersion, r.ResponseReadRef.Selector, r.ResponseReadRef.Digest}), field(r.ScannerImplementationDigest), field(r.SchemaVersion), field(r.TopLevelForm), r.WorkUnits))
	if len(b) > sourceRecordLimit {
		return nil, errScannerRecord
	}
	return b, nil
}
func scannerRecordDigest(b []byte) string {
	return privateDigest(append([]byte(scannerRecordRole+"\x00"), b...))
}
func scannerRecordSelector(d string) string {
	return "adr0011-references-issuance-v1-scanner-" + strings.TrimPrefix(d, "sha256:") + ".json"
}

// Reconstruct only from independently verified predecessors and retained bytes.
func scannerRecordExpected(root *publication.Root, responseRead, rawResult, payload privateBodyPublication, x responseReadInputs, read func(*publication.Root, string, int64) ([]byte, error), source func() ([]byte, error)) (scannerRecord, bool) {
	var zero scannerRecord
	pin, ok := selectedScannerPin(source)
	if !ok || root == nil || responseRead.stage != "VERIFIED" || rawResult.stage != "VERIFIED" || payload.stage != "VERIFIED" || payload != x.Payload {
		return zero, false
	}
	if read == nil {
		read = publication.ReadVerifiedBoundFile
	}
	if !replayRawResult(root, rawResult, responseRead, payload, x, read) {
		return zero, false
	}
	observed, err := observeRawScanner(root, payload, read)
	if err != nil || (observed.form != "NULL" && observed.form != "ARRAY") || observed.workUnits < 1 || observed.workUnits > rawResultPayloadLimit {
		return zero, false
	}
	return scannerRecord{scannerRecordRole, targetRecordRefFields{responseRead.selector, responseRead.digest, responseReadRole}, targetRecordRefFields{rawResult.selector, rawResult.digest, rawResultRole}, pin, observed.form, observed.knownE, observed.workUnits}, true
}

func replayScannerRecord(root *publication.Root, state, responseRead, rawResult, payload privateBodyPublication, x responseReadInputs, read func(*publication.Root, string, int64) ([]byte, error), source func() ([]byte, error)) bool {
	return replayScannerRecordInternal(root, state, responseRead, rawResult, payload, x, read, source, false)
}
func replayScannerRecordInternal(root *publication.Root, state, responseRead, rawResult, payload privateBodyPublication, x responseReadInputs, read func(*publication.Root, string, int64) ([]byte, error), source func() ([]byte, error), promoting bool) bool {
	if state.stage != "VERIFIED" && !(promoting && state.stage == "COMMITTED_UNVERIFIED") {
		return false
	}
	if read == nil {
		read = publication.ReadVerifiedBoundFile
	}
	b, err := read(root, state.selector, sourceRecordLimit)
	if err != nil || state.byteCount != len(b) || state.digest != scannerRecordDigest(b) || state.selector != scannerRecordSelector(state.digest) || strictjson.RejectDuplicates(b) != nil {
		return false
	}
	var record scannerRecord
	decoder := json.NewDecoder(bytes.NewReader(b))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&record) != nil {
		return false
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return false
	}
	expected, ok := scannerRecordExpected(root, responseRead, rawResult, payload, x, read, source)
	if !ok {
		return false
	}
	canonical, err := canonicalScannerRecord(expected)
	return err == nil && bytes.Equal(b, canonical)
}

func publishScannerRecord(root *publication.Root, responseRead, rawResult, payload privateBodyPublication, x responseReadInputs, read func(*publication.Root, string, int64) ([]byte, error), source func() ([]byte, error)) (privateBodyPublication, error) {
	absent := privateBodyPublication{stage: "ABSENT"}
	if read == nil {
		read = publication.ReadVerifiedBoundFile
	}
	record, ok := scannerRecordExpected(root, responseRead, rawResult, payload, x, read, source)
	if !ok {
		return absent, errScannerRecord
	}
	b, err := canonicalScannerRecord(record)
	if err != nil {
		return absent, errScannerRecord
	}
	digest := scannerRecordDigest(b)
	selector := scannerRecordSelector(digest)
	precommit, uncertain := false, false
	receipt, err := publication.PublishBoundFileWithTrace(root, selector, b, func(got []byte) error {
		if !bytes.Equal(got, b) {
			return errScannerRecord
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
	if err != nil || receipt == nil {
		if precommit && !uncertain {
			return absent, errScannerRecord
		}
		return privateBodyPublication{selector: selector, digest: digest, byteCount: len(b), stage: "COMMITTED_UNVERIFIED"}, errScannerRecord
	}
	state := privateBodyPublication{selector: selector, digest: digest, byteCount: len(b), stage: "COMMITTED_UNVERIFIED"}
	if !verifiedTargetPublication(receipt, selector, privateDigest(b), len(b)) || !replayScannerRecordInternal(root, state, responseRead, rawResult, payload, x, read, source, true) {
		return state, errScannerRecord
	}
	state.stage = "VERIFIED"
	return state, nil
}
