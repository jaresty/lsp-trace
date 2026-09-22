package censusdiagnostic

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
	DefaultMaxBytes   int64 = 1 << 20
	DefaultMaxRecords       = 1000
)

type Record struct {
	Fingerprint       string `json:"fingerprint"`
	Category          string `json:"category"`
	Ordinal           *int   `json:"ordinal,omitempty"`
	OperationCode     string `json:"operation_code,omitempty"`
	OperationCategory string `json:"operation_category,omitempty"`
}
type Recorder struct {
	path       string
	maxBytes   int64
	maxRecords int
	mu         sync.Mutex
}

func NewRecorder(path string, maxBytes int64, maxRecords int) (*Recorder, error) {
	if path == "" || !filepath.IsAbs(path) || filepath.Clean(path) != path || maxBytes <= 0 || maxBytes >= math.MaxInt64 || maxRecords <= 0 {
		return nil, errors.New("invalid acquisition diagnostic recorder configuration")
	}
	if err := custody(path); err != nil {
		return nil, err
	}
	f, e := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if e != nil {
		return nil, errors.New("acquisition diagnostic storage unavailable")
	}
	e = f.Close()
	if e != nil {
		return nil, e
	}
	if _, e = read(path, maxBytes, maxRecords, true); e != nil {
		return nil, e
	}
	return &Recorder{path: path, maxBytes: maxBytes, maxRecords: maxRecords}, nil
}
func (r *Recorder) Record(v Record) error {
	if r == nil {
		return errors.New("acquisition diagnostic recorder unavailable")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if e := custody(r.path); e != nil {
		return e
	}
	if e := validate(v); e != nil {
		return e
	}
	old, e := read(r.path, r.maxBytes, r.maxRecords, true)
	if e != nil {
		return e
	}
	line, _ := json.Marshal(v)
	line = append(line, '\n')
	if int64(len(old))+int64(len(line)) > r.maxBytes {
		return errors.New("acquisition diagnostic byte limit")
	}
	if bytes.Count(old, []byte{'\n'}) >= r.maxRecords {
		return errors.New("acquisition diagnostic record limit")
	}
	before, e := os.Lstat(r.path)
	if e != nil || !before.Mode().IsRegular() || before.Mode().Perm() != 0600 {
		return errors.New("acquisition diagnostic storage unavailable")
	}
	f, e := os.OpenFile(r.path, os.O_APPEND|os.O_WRONLY, 0600)
	if e != nil {
		return e
	}
	defer f.Close()
	opened, e := f.Stat()
	if e != nil || !opened.Mode().IsRegular() || opened.Mode().Perm() != 0600 || !os.SameFile(before, opened) {
		return errors.New("acquisition diagnostic storage unavailable")
	}
	if n, e := f.Write(line); e != nil || n != len(line) {
		return errors.New("acquisition diagnostic storage unavailable")
	}
	if e = f.Sync(); e != nil {
		return errors.New("acquisition diagnostic storage unavailable")
	}
	return nil
}
func ReadLast(path string) (Record, error) {
	var z Record
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return z, errors.New("invalid acquisition diagnostic path")
	}
	if e := custody(path); e != nil {
		return z, e
	}
	raw, e := read(path, DefaultMaxBytes, DefaultMaxRecords, false)
	if e != nil {
		return z, e
	}
	lines := bytes.Split(bytes.TrimSuffix(raw, []byte{'\n'}), []byte{'\n'})
	return Parse(lines[len(lines)-1])
}
func Parse(line []byte) (Record, error) {
	var v Record
	d := json.NewDecoder(bytes.NewReader(line))
	d.DisallowUnknownFields()
	if e := d.Decode(&v); e != nil {
		return v, errors.New("invalid acquisition diagnostic record")
	}
	if e := d.Decode(&struct{}{}); e != io.EOF {
		return v, errors.New("invalid acquisition diagnostic record")
	}
	c, e := json.Marshal(v)
	if e != nil || !bytes.Equal(c, line) {
		return v, errors.New("noncanonical acquisition diagnostic record")
	}
	if e = validate(v); e != nil {
		return v, e
	}
	return v, nil
}
func validate(v Record) error {
	if v.Fingerprint == "" || v.Category == "" || v.Category != "INVALID_INPUT" && v.Category != "SESSION_DRIFT" && v.Category != "CANCELLED" && v.Category != "ACQUISITION" && v.Category != "ADMISSION" {
		return errors.New("invalid acquisition diagnostic category")
	}
	if v.Ordinal != nil && *v.Ordinal < 0 {
		return errors.New("invalid acquisition diagnostic ordinal")
	}
	if v.Category == "ACQUISITION" != (v.OperationCode != "" && v.OperationCategory != "") {
		return errors.New("invalid acquisition operation evidence")
	}
	return nil
}
func custody(path string) error {
	p := filepath.Dir(path)
	i, e := os.Lstat(p)
	if e != nil || !i.IsDir() || i.Mode()&os.ModeSymlink != 0 || i.Mode().Perm() != 0700 {
		return errors.New("acquisition diagnostic parent unavailable")
	}
	return nil
}
func read(path string, max int64, n int, empty bool) ([]byte, error) {
	i, e := os.Lstat(path)
	if e != nil || !i.Mode().IsRegular() || i.Mode()&os.ModeSymlink != 0 || i.Mode().Perm() != 0600 || i.Size() > max || (!empty && i.Size() == 0) {
		return nil, errors.New("acquisition diagnostic unavailable")
	}
	f, e := os.Open(path)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	raw, e := io.ReadAll(io.LimitReader(f, max+1))
	if e != nil || int64(len(raw)) != i.Size() || int64(len(raw)) > max {
		return nil, errors.New("acquisition diagnostic unavailable")
	}
	if len(raw) == 0 {
		return raw, nil
	}
	if raw[len(raw)-1] != '\n' {
		return nil, errors.New("acquisition diagnostic ledger incomplete")
	}
	lines := bytes.Split(bytes.TrimSuffix(raw, []byte{'\n'}), []byte{'\n'})
	if len(lines) > n {
		return nil, errors.New("acquisition diagnostic record limit")
	}
	for _, l := range lines {
		if _, e := Parse(l); e != nil {
			return nil, errors.New("acquisition diagnostic ledger invalid")
		}
	}
	return raw, nil
}
