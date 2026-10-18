package censuscontrolflow

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"math"
	"os"
	"path/filepath"
	"sync"
)

const (
	SchemaVersion     = "lsp-trace.census-control-flow-diagnostic.v1"
	DefaultMaxBytes   = int64(4096)
	DefaultMaxRecords = 1
)

type FirstCause string
type CorePhase string
type BatchRecord string

const (
	CallbackNotEntered     FirstCause  = "CALLBACK_NOT_ENTERED"
	CallbackRuntimeInvalid FirstCause  = "CALLBACK_RUNTIME_INVALID"
	CoreFailed             FirstCause  = "CORE_FAILED"
	PostCoreRuntimeInvalid FirstCause  = "POST_CORE_RUNTIME_INVALID"
	FinalNonzeroFallback   FirstCause  = "FINAL_NONZERO_FALLBACK"
	PhaseNotEntered        CorePhase   = "NOT_ENTERED"
	PhaseInput             CorePhase   = "INPUT"
	PhaseDiscovery         CorePhase   = "DISCOVERY"
	PhasePlanning          CorePhase   = "PLANNING"
	PhaseBatchAcquire      CorePhase   = "BATCH_ACQUIRE"
	PhaseBatchAdmission    CorePhase   = "BATCH_ADMISSION"
	PhaseManifest          CorePhase   = "MANIFEST"
	PhaseCompleted         CorePhase   = "COMPLETED"
	BatchNotAttempted      BatchRecord = "NOT_ATTEMPTED"
	BatchAccepted          BatchRecord = "ACCEPTED"
	BatchRejected          BatchRecord = "REJECTED"
)

type Record struct {
	SchemaVersion string      `json:"schema_version"`
	FirstCause    FirstCause  `json:"first_cause"`
	CorePhase     CorePhase   `json:"core_phase"`
	BatchRecord   BatchRecord `json:"batch_record"`
}

type Recorder struct {
	path       string
	maxBytes   int64
	maxRecords int
	mu         sync.Mutex
}

func NewRecorder(path string, maxBytes int64, maxRecords int) (*Recorder, error) {
	if path == "" || !filepath.IsAbs(path) || filepath.Clean(path) != path || maxBytes <= 0 || maxBytes >= math.MaxInt64 || maxRecords <= 0 {
		return nil, errors.New("invalid control-flow diagnostic recorder configuration")
	}
	if err := custody(path); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return nil, errors.New("control-flow diagnostic storage unavailable")
	}
	if err = f.Close(); err != nil {
		return nil, err
	}
	if _, err = read(path, maxBytes, maxRecords, true); err != nil {
		return nil, err
	}
	return &Recorder{path: path, maxBytes: maxBytes, maxRecords: maxRecords}, nil
}

func (r *Recorder) Record(v Record) error {
	if r == nil {
		return errors.New("control-flow diagnostic recorder unavailable")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := custody(r.path); err != nil {
		return err
	}
	line, err := Marshal(v)
	if err != nil {
		return err
	}
	old, err := read(r.path, r.maxBytes, r.maxRecords, true)
	if err != nil {
		return err
	}
	if int64(len(old))+int64(len(line)) > r.maxBytes {
		return errors.New("control-flow diagnostic byte limit")
	}
	if bytes.Count(old, []byte{'\n'}) >= r.maxRecords {
		return errors.New("control-flow diagnostic record limit")
	}
	before, err := os.Lstat(r.path)
	if err != nil || !before.Mode().IsRegular() || before.Mode()&os.ModeSymlink != 0 || before.Mode().Perm() != 0600 {
		return errors.New("control-flow diagnostic storage unavailable")
	}
	f, err := os.OpenFile(r.path, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return errors.New("control-flow diagnostic storage unavailable")
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !opened.Mode().IsRegular() || opened.Mode().Perm() != 0600 || !os.SameFile(before, opened) {
		return errors.New("control-flow diagnostic storage unavailable")
	}
	if n, err := f.Write(line); err != nil || n != len(line) {
		return errors.New("control-flow diagnostic storage unavailable")
	}
	if err := f.Sync(); err != nil {
		return errors.New("control-flow diagnostic storage unavailable")
	}
	return nil
}

func Marshal(v Record) ([]byte, error) {
	if err := validate(v); err != nil {
		return nil, err
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

func Parse(line []byte) (Record, error) {
	var v Record
	dec := json.NewDecoder(bytes.NewReader(line))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&v); err != nil {
		return v, errors.New("invalid control-flow diagnostic record")
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return v, errors.New("invalid control-flow diagnostic record")
	}
	canonical, err := json.Marshal(v)
	if err != nil || !bytes.Equal(canonical, line) {
		return v, errors.New("noncanonical control-flow diagnostic record")
	}
	if err := validate(v); err != nil {
		return v, err
	}
	return v, nil
}

func ReadLast(path string) (Record, error) {
	var zero Record
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return zero, errors.New("invalid control-flow diagnostic path")
	}
	if err := custody(path); err != nil {
		return zero, err
	}
	raw, err := read(path, DefaultMaxBytes, DefaultMaxRecords, false)
	if err != nil {
		return zero, err
	}
	lines := bytes.Split(bytes.TrimSuffix(raw, []byte{'\n'}), []byte{'\n'})
	return Parse(lines[len(lines)-1])
}

func validate(v Record) error {
	causes := map[FirstCause]bool{CallbackNotEntered: true, CallbackRuntimeInvalid: true, CoreFailed: true, PostCoreRuntimeInvalid: true, FinalNonzeroFallback: true}
	phases := map[CorePhase]bool{PhaseNotEntered: true, PhaseInput: true, PhaseDiscovery: true, PhasePlanning: true, PhaseBatchAcquire: true, PhaseBatchAdmission: true, PhaseManifest: true, PhaseCompleted: true}
	batches := map[BatchRecord]bool{BatchNotAttempted: true, BatchAccepted: true, BatchRejected: true}
	if v.SchemaVersion != SchemaVersion || !causes[v.FirstCause] || !phases[v.CorePhase] || !batches[v.BatchRecord] {
		return errors.New("invalid control-flow diagnostic record")
	}
	return nil
}

func custody(path string) error {
	info, err := os.Lstat(filepath.Dir(path))
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != 0700 {
		return errors.New("control-flow diagnostic parent unavailable")
	}
	return nil
}

func read(path string, max int64, records int, empty bool) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != 0600 || info.Size() > max || (!empty && info.Size() == 0) {
		return nil, errors.New("control-flow diagnostic unavailable")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, max+1))
	if err != nil || int64(len(raw)) != info.Size() || int64(len(raw)) > max {
		return nil, errors.New("control-flow diagnostic unavailable")
	}
	if len(raw) == 0 {
		return raw, nil
	}
	if raw[len(raw)-1] != '\n' {
		return nil, errors.New("control-flow diagnostic ledger incomplete")
	}
	lines := bytes.Split(bytes.TrimSuffix(raw, []byte{'\n'}), []byte{'\n'})
	if len(lines) > records {
		return nil, errors.New("control-flow diagnostic record limit")
	}
	for _, line := range lines {
		if _, err := Parse(line); err != nil {
			return nil, errors.New("control-flow diagnostic ledger invalid")
		}
	}
	return raw, nil
}
