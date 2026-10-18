package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/session"
	"lsp-trace/sessionruntime"
)

const (
	privateProviderProbeSchema   = "lsp-trace.private-provider-probe.v1"
	privateProviderProbeSchemaV2 = "lsp-trace.private-provider-probe.v2"
)

type privateProviderProbeRecord struct {
	Sequence uint64 `json:"sequence"`
	Phase    string `json:"phase"`
	Class    string `json:"class"`
	Cause    string `json:"cause"`
}
type privateProviderProbeCount struct {
	Cause string `json:"cause"`
	Count uint64 `json:"count"`
}
type privateProviderProbeArtifact struct {
	Schema          string                       `json:"schema"`
	Status          string                       `json:"status"`
	FileDenominator uint64                       `json:"file_denominator"`
	CauseCounts     []privateProviderProbeCount  `json:"cause_counts"`
	Records         []privateProviderProbeRecord `json:"records"`
	RerunCount      uint64                       `json:"rerun_count"`
	Work            uint64                       `json:"work"`
}

type privateProviderProbeExhaustionV2 struct {
	ObservedMessages      int    `json:"observed_messages"`
	ObservedBytes         int64  `json:"observed_bytes"`
	ConfiguredMaxMessages int    `json:"configured_max_messages"`
	ConfiguredMaxBytes    int64  `json:"configured_max_bytes"`
	Boundary              string `json:"boundary"`
}
type privateProviderProbeRecordV2 struct {
	Sequence   uint64                            `json:"sequence"`
	Phase      string                            `json:"phase"`
	Class      string                            `json:"class"`
	Cause      string                            `json:"cause"`
	Exhaustion *privateProviderProbeExhaustionV2 `json:"exhaustion,omitempty"`
}
type privateProviderProbeArtifactV2 struct {
	Schema          string                         `json:"schema"`
	Status          string                         `json:"status"`
	FileDenominator uint64                         `json:"file_denominator"`
	CauseCounts     []privateProviderProbeCount    `json:"cause_counts"`
	Records         []privateProviderProbeRecordV2 `json:"records"`
	RerunCount      uint64                         `json:"rerun_count"`
	Work            uint64                         `json:"work"`
}
type privateProviderProbe struct {
	mu                    sync.Mutex
	path, version         string
	sequence, files, work uint64
	counts                map[string]uint64
	records               []privateProviderProbeRecord
	recordsV2             []privateProviderProbeRecordV2
	incomplete            bool
}

func newPrivateProviderProbe(path string) (*privateProviderProbe, error) {
	return newPrivateProviderProbeVersion(path, "v1")
}
func newPrivateProviderProbeVersion(path, version string) (*privateProviderProbe, error) {
	if version != "v1" && version != "v2" {
		return nil, errors.New("invalid private provider probe version")
	}
	if path == "" || !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return nil, errors.New("invalid private provider probe path")
	}
	info, err := os.Lstat(filepath.Dir(path))
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != 0700 {
		return nil, errors.New("private provider probe parent unavailable")
	}
	if info, err = os.Lstat(path); err == nil {
		if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != 0600 {
			return nil, errors.New("private provider probe unavailable")
		}
	} else if !os.IsNotExist(err) {
		return nil, errors.New("private provider probe unavailable")
	}
	return &privateProviderProbe{path: path, version: version, counts: make(map[string]uint64)}, nil
}
func checkedProbeAdd(a *uint64, n uint64) bool {
	if math.MaxUint64-*a < n {
		return false
	}
	*a += n
	return true
}
func (p *privateProviderProbe) observe(phase, class, cause string, file bool) {
	if p == nil || cause == "" {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if !checkedProbeAdd(&p.work, 1) || !checkedProbeAdd(&p.sequence, 1) {
		p.incomplete = true
		return
	}
	if file && !checkedProbeAdd(&p.files, 1) {
		p.incomplete = true
		return
	}
	if math.MaxUint64-p.counts[cause] < 1 {
		p.incomplete = true
		return
	}
	p.counts[cause]++
	want := phase == "DOCUMENT_SYMBOL" && !hasProbePhase(p.records, phase) || phase == "DOCUMENT_SUPPLY" && hasProbePhase(p.records, "DOCUMENT_SYMBOL") && !hasProbePhase(p.records, phase)
	if want && len(p.records) < 2 {
		p.records = append(p.records, privateProviderProbeRecord{p.sequence, phase, class, cause})
		if p.version == "v2" {
			p.recordsV2 = append(p.recordsV2, privateProviderProbeRecordV2{Sequence: p.sequence, Phase: phase, Class: class, Cause: cause})
		}
	}
}
func hasProbePhase(records []privateProviderProbeRecord, phase string) bool {
	for _, r := range records {
		if r.Phase == phase {
			return true
		}
	}
	return false
}
func privateProviderProbeBoundary(observedMessages int, observedBytes int64, configuredMaxMessages int, configuredMaxBytes int64) (string, error) {
	if observedMessages < 0 || observedBytes < 0 {
		return "", errors.New("invalid private provider probe exhaustion")
	}
	if configuredMaxMessages <= 0 || configuredMaxBytes <= 0 {
		return "UNKNOWN", nil
	}
	if observedBytes > configuredMaxBytes {
		return "BYTE_LIMIT_EXCEEDED", nil
	}
	if observedMessages >= configuredMaxMessages {
		return "MESSAGE_CEILING_OBSERVED", nil
	}
	return "NEITHER_BOUNDARY_OBSERVED", nil
}
func (p *privateProviderProbe) observeExhaustion(phase, class, cause string, configuredMaxMessages int, configuredMaxBytes int64, result sessionruntime.RoundTripResult) {
	if p == nil || cause == "" {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if !checkedProbeAdd(&p.work, 1) || !checkedProbeAdd(&p.sequence, 1) {
		p.incomplete = true
		return
	}
	if math.MaxUint64-p.counts[cause] < 1 {
		p.incomplete = true
		return
	}
	p.counts[cause]++
	want := phase == "DOCUMENT_SYMBOL" && !hasProbePhase(p.records, phase) || phase == "DOCUMENT_SUPPLY" && hasProbePhase(p.records, "DOCUMENT_SYMBOL") && !hasProbePhase(p.records, phase)
	if !want || len(p.records) >= 2 {
		return
	}
	p.records = append(p.records, privateProviderProbeRecord{p.sequence, phase, class, cause})
	boundary, err := privateProviderProbeBoundary(result.Messages, result.Bytes, configuredMaxMessages, configuredMaxBytes)
	if err != nil {
		p.incomplete = true
		return
	}
	p.recordsV2 = append(p.recordsV2, privateProviderProbeRecordV2{
		Sequence: p.sequence, Phase: phase, Class: class, Cause: cause,
		Exhaustion: &privateProviderProbeExhaustionV2{result.Messages, result.Bytes, configuredMaxMessages, configuredMaxBytes, boundary},
	})
}
func (p *privateProviderProbe) observeSuccess(file bool) {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if !checkedProbeAdd(&p.work, 1) || !checkedProbeAdd(&p.sequence, 1) || file && !checkedProbeAdd(&p.files, 1) {
		p.incomplete = true
	}
}
func (p *privateProviderProbe) artifact() privateProviderProbeArtifact {
	p.mu.Lock()
	defer p.mu.Unlock()
	causes := make([]string, 0, len(p.counts))
	for c := range p.counts {
		causes = append(causes, c)
	}
	sort.Strings(causes)
	counts := make([]privateProviderProbeCount, 0, len(causes))
	for _, c := range causes {
		counts = append(counts, privateProviderProbeCount{c, p.counts[c]})
	}
	status := "COMPLETE"
	if p.incomplete {
		status = "INCOMPLETE"
	}
	return privateProviderProbeArtifact{privateProviderProbeSchema, status, p.files, counts, append([]privateProviderProbeRecord(nil), p.records...), 0, p.work}
}
func (p *privateProviderProbe) artifactV2() privateProviderProbeArtifactV2 {
	base := p.artifact()
	p.mu.Lock()
	records := append([]privateProviderProbeRecordV2(nil), p.recordsV2...)
	p.mu.Unlock()
	return privateProviderProbeArtifactV2{privateProviderProbeSchemaV2, base.Status, base.FileDenominator, base.CauseCounts, records, base.RerunCount, base.Work}
}
func (p *privateProviderProbe) finalize() error {
	if p == nil {
		return nil
	}
	var value any = p.artifact()
	if p.version == "v2" {
		value = p.artifactV2()
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	info, err := os.Lstat(filepath.Dir(p.path))
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != 0700 {
		return errors.New("private provider probe parent unavailable")
	}
	f, err := os.OpenFile(p.path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return errors.New("private provider probe unavailable")
	}
	defer f.Close()
	if err = f.Chmod(0600); err != nil {
		return err
	}
	if n, e := f.Write(raw); e != nil || n != len(raw) {
		return errors.New("private provider probe unavailable")
	}
	return f.Sync()
}
func parsePrivateProviderProbe(raw []byte) (privateProviderProbeArtifact, error) {
	var a privateProviderProbeArtifact
	line := bytes.TrimSuffix(raw, []byte{'\n'})
	d := json.NewDecoder(bytes.NewReader(line))
	d.DisallowUnknownFields()
	if d.Decode(&a) != nil {
		return a, errors.New("invalid private provider probe")
	}
	if d.Decode(&struct{}{}) != io.EOF {
		return a, errors.New("invalid private provider probe")
	}
	canonical, _ := json.Marshal(a)
	if !bytes.Equal(canonical, line) || a.Schema != privateProviderProbeSchema || (a.Status != "COMPLETE" && a.Status != "INCOMPLETE") || len(a.Records) > 2 || a.RerunCount != 0 || a.FileDenominator > a.Work {
		return a, errors.New("invalid private provider probe")
	}
	previousCause := ""
	for _, count := range a.CauseCounts {
		if count.Cause == "" || count.Count == 0 || previousCause >= count.Cause {
			return a, errors.New("invalid private provider probe")
		}
		previousCause = count.Cause
	}
	var previousSequence uint64
	for i, record := range a.Records {
		if record.Sequence == 0 || record.Sequence <= previousSequence || record.Sequence > a.Work || record.Cause == "" {
			return a, errors.New("invalid private provider probe")
		}
		if i == 0 && record.Phase != "DOCUMENT_SYMBOL" || i == 1 && record.Phase != "DOCUMENT_SUPPLY" {
			return a, errors.New("invalid private provider probe")
		}
		validClass := record.Class == "TYPED_SESSION_RUNTIME_FAILURE" || record.Class == "JSON_RPC_CODE_BUCKET" || record.Class == "DECODE_FAILURE" || record.Class == "DOCUMENT_FAILURE" || record.Class == "ADAPTER_IDENTITY_MISMATCH"
		if !validClass {
			return a, errors.New("invalid private provider probe")
		}
		previousSequence = record.Sequence
	}
	return a, nil
}

func validPrivateProviderProbePhaseClass(phase, class string) bool {
	switch phase {
	case "DOCUMENT_SYMBOL":
		return class == "TYPED_SESSION_RUNTIME_FAILURE" || class == "JSON_RPC_CODE_BUCKET" || class == "DECODE_FAILURE"
	case "DOCUMENT_SUPPLY":
		return class == "DOCUMENT_FAILURE" || class == "ADAPTER_IDENTITY_MISMATCH"
	}
	return false
}

func validPrivateProviderProbeCause(class, cause string) bool {
	switch class {
	case "TYPED_SESSION_RUNTIME_FAILURE":
		switch session.Failure(cause) {
		case session.SessionNotFound, session.StaleGeneration, session.LifecycleConflict, session.SessionStopping, session.SessionPoisoned, session.ResourceExhausted, session.SessionCrashed, session.RequestCancelled, session.RequestTimeout:
			return true
		}
		return cause == "OTHER_TYPED_FAILURE"
	case "JSON_RPC_CODE_BUCKET":
		switch cause {
		case "PARSE_ERROR", "INVALID_REQUEST", "METHOD_NOT_FOUND", "INVALID_PARAMS", "INTERNAL_ERROR", "SERVER_ERROR", "OTHER":
			return true
		}
	case "DECODE_FAILURE":
		return cause == "DECODE_FAILURE"
	case "DOCUMENT_FAILURE":
		switch session.Failure(cause) {
		case session.RequestCancelled, session.RequestTimeout, session.StaleGeneration, session.LifecycleConflict, session.ResourceExhausted, session.SessionPoisoned, sessionruntime.DocumentSupplyUnavailable, sessionruntime.DocumentURIUnavailable, sessionruntime.DocumentOutsideWorkspace, sessionruntime.DocumentSourceUnavailable, sessionruntime.LanguageIDUnavailable:
			return true
		}
		return cause == "OTHER_TYPED_FAILURE"
	case "ADAPTER_IDENTITY_MISMATCH":
		return cause == "ADAPTER_IDENTITY_MISMATCH"
	}
	return false
}

func knownPrivateProviderProbeCause(cause string) bool {
	for _, class := range []string{"TYPED_SESSION_RUNTIME_FAILURE", "JSON_RPC_CODE_BUCKET", "DECODE_FAILURE", "DOCUMENT_FAILURE", "ADAPTER_IDENTITY_MISMATCH"} {
		if validPrivateProviderProbeCause(class, cause) {
			return true
		}
	}
	return false
}

func parsePrivateProviderProbeV2(raw []byte) (privateProviderProbeArtifactV2, error) {
	var a privateProviderProbeArtifactV2
	if len(raw) == 0 || raw[len(raw)-1] != '\n' || len(raw) > 1 && raw[len(raw)-2] == '\n' {
		return a, errors.New("invalid private provider probe")
	}
	line := bytes.TrimSuffix(raw, []byte{'\n'})
	d := json.NewDecoder(bytes.NewReader(line))
	d.DisallowUnknownFields()
	if d.Decode(&a) != nil {
		return a, errors.New("invalid private provider probe")
	}
	if d.Decode(&struct{}{}) != io.EOF {
		return a, errors.New("invalid private provider probe")
	}
	if a.CauseCounts == nil || len(a.Records) == 0 && a.Records != nil {
		return a, errors.New("invalid private provider probe")
	}
	canonical, _ := json.Marshal(a)
	if !bytes.Equal(canonical, line) || a.Schema != privateProviderProbeSchemaV2 || (a.Status != "COMPLETE" && a.Status != "INCOMPLETE") || len(a.Records) > 2 || a.RerunCount != 0 || a.FileDenominator > a.Work {
		return a, errors.New("invalid private provider probe")
	}
	previousCause := ""
	causeCounts := make(map[string]uint64, len(a.CauseCounts))
	var totalCount uint64
	for _, count := range a.CauseCounts {
		if !knownPrivateProviderProbeCause(count.Cause) || count.Count == 0 || count.Count > a.Work || previousCause >= count.Cause || math.MaxUint64-totalCount < count.Count {
			return a, errors.New("invalid private provider probe")
		}
		totalCount += count.Count
		if totalCount > a.Work {
			return a, errors.New("invalid private provider probe")
		}
		causeCounts[count.Cause] = count.Count
		previousCause = count.Cause
	}
	retainedCounts := make(map[string]uint64, len(a.Records))
	var previousSequence, retainedSymbolCount, retainedSupplyCount uint64
	for i, record := range a.Records {
		if record.Sequence == 0 || record.Sequence <= previousSequence || record.Sequence > a.Work || record.Cause == "" {
			return a, errors.New("invalid private provider probe")
		}
		if i == 0 && record.Phase != "DOCUMENT_SYMBOL" || i == 1 && record.Phase != "DOCUMENT_SUPPLY" {
			return a, errors.New("invalid private provider probe")
		}
		if !validPrivateProviderProbePhaseClass(record.Phase, record.Class) || !validPrivateProviderProbeCause(record.Class, record.Cause) {
			return a, errors.New("invalid private provider probe")
		}
		if record.Phase == "DOCUMENT_SYMBOL" {
			retainedSymbolCount++
		} else {
			retainedSupplyCount++
		}
		retainedCounts[record.Cause]++
		if retainedCounts[record.Cause] > causeCounts[record.Cause] {
			return a, errors.New("invalid private provider probe")
		}
		wantExhaustion := record.Class == "TYPED_SESSION_RUNTIME_FAILURE" && record.Cause == string(session.ResourceExhausted)
		if wantExhaustion != (record.Exhaustion != nil) {
			return a, errors.New("invalid private provider probe")
		}
		if record.Exhaustion != nil {
			e := record.Exhaustion
			boundary, err := privateProviderProbeBoundary(e.ObservedMessages, e.ObservedBytes, e.ConfiguredMaxMessages, e.ConfiguredMaxBytes)
			if err != nil || e.Boundary != boundary {
				return a, errors.New("invalid private provider probe")
			}
		}
		previousSequence = record.Sequence
	}
	if retainedSymbolCount > a.Work-a.FileDenominator || retainedSupplyCount > a.FileDenominator {
		return a, errors.New("invalid private provider probe")
	}
	return a, nil
}

func privateSessionFailureCause(f session.Failure) (string, string) {
	switch f {
	case session.SessionNotFound, session.StaleGeneration, session.LifecycleConflict, session.SessionStopping, session.SessionPoisoned, session.ResourceExhausted, session.SessionCrashed, session.RequestCancelled, session.RequestTimeout:
		return "TYPED_SESSION_RUNTIME_FAILURE", string(f)
	default:
		return "TYPED_SESSION_RUNTIME_FAILURE", "OTHER_TYPED_FAILURE"
	}
}
func privateDocumentFailureCause(f session.Failure) (string, string) {
	switch f {
	case session.RequestCancelled, session.RequestTimeout, session.StaleGeneration, session.LifecycleConflict, session.ResourceExhausted, session.SessionPoisoned, sessionruntime.DocumentSupplyUnavailable, sessionruntime.DocumentURIUnavailable, sessionruntime.DocumentOutsideWorkspace, sessionruntime.DocumentSourceUnavailable, sessionruntime.LanguageIDUnavailable:
		return "DOCUMENT_FAILURE", string(f)
	default:
		return "DOCUMENT_FAILURE", "OTHER_TYPED_FAILURE"
	}
}
func privateRPCErrorCause(e *lspwire.RPCError) (string, string) {
	c := "OTHER"
	if e.Code == -32700 {
		c = "PARSE_ERROR"
	} else if e.Code == -32600 {
		c = "INVALID_REQUEST"
	} else if e.Code == -32601 {
		c = "METHOD_NOT_FOUND"
	} else if e.Code == -32602 {
		c = "INVALID_PARAMS"
	} else if e.Code == -32603 {
		c = "INTERNAL_ERROR"
	} else if e.Code >= -32099 && e.Code <= -32000 {
		c = "SERVER_ERROR"
	}
	return "JSON_RPC_CODE_BUCKET", c
}
func privateProbePhase(method string) string {
	if method == "textDocument/documentSymbol" {
		return "DOCUMENT_SYMBOL"
	}
	return "PROVIDER_REQUEST"
}
