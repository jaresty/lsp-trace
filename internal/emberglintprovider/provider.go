// Package emberglintprovider implements the host-facing provider transport and
// orchestration boundary. Ember/Glimmer semantics belong to injected analyzers.
package emberglintprovider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
)

const (
	ProviderID      = "ember-glint@1"
	RequestSchema   = "lsp-trace.provider-collector-request.v1"
	ProtocolName    = "lsp-trace.provider-observations"
	ProtocolVersion = "1"
	defaultMaxBytes = int64(4 << 20)
)

type RelationKind string

const (
	BindsArgument  RelationKind = "BINDS_ARGUMENT"
	PassesCallback RelationKind = "PASSES_CALLBACK"
	InvokesTask    RelationKind = "INVOKES_TASK"
	TriggersReload RelationKind = "TRIGGERS_RELOAD"
	UpdatesState   RelationKind = "UPDATES_STATE"
	RendersFrom    RelationKind = "RENDERS_FROM"
)

type Status string

const (
	StatusUnsupported Status = "unsupported"
	StatusUnavailable Status = "unavailable"
	StatusPartial     Status = "partial"
	StatusEmpty       Status = "empty"
	StatusFailed      Status = "failed"
)

var ErrUnavailable = errors.New("unavailable")

type Identity struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}
type Position struct {
	Line      int `json:"line,omitempty"`
	Character int `json:"character,omitempty"`
}
type Seed struct {
	URI       string `json:"uri"`
	Line      int    `json:"line,omitempty"`
	Character int    `json:"character,omitempty"`
	Symbol    string `json:"symbol,omitempty"`
	StartMode string `json:"start_mode,omitempty"`
}
type Session struct {
	SessionID  string `json:"session_id"`
	Generation int    `json:"generation"`
}
type DocumentCustody struct {
	OriginalURI           string `json:"original_uri"`
	WorkspaceRevision     any    `json:"workspace_revision,omitempty"`
	FailOnUnknownRevision bool   `json:"fail_on_unknown_revision"`
}
type Limits struct {
	MaxDepth         *int  `json:"max_depth,omitempty"`
	DownDepth        *int  `json:"down_depth,omitempty"`
	UpDepth          *int  `json:"up_depth,omitempty"`
	MaxNodes         int   `json:"max_nodes"`
	MaxMessages      int   `json:"max_messages,omitempty"`
	MaxBytes         int64 `json:"max_bytes,omitempty"`
	TimeoutMS        int   `json:"timeout_ms"`
	RequestTimeoutMS int   `json:"request_timeout_ms"`
}
type collectorRequest struct {
	SchemaVersion   string          `json:"schema_version"`
	ProviderID      string          `json:"provider_id"`
	AdapterID       string          `json:"adapter_id"`
	Session         Session         `json:"session"`
	Seed            Seed            `json:"seed"`
	Relations       []RelationKind  `json:"relations"`
	DocumentCustody DocumentCustody `json:"document_custody"`
	Limits          Limits          `json:"limits"`
}

type Document map[string]any
type Anchor struct {
	DocumentID string         `json:"document_id"`
	URI        string         `json:"uri"`
	Revision   string         `json:"revision"`
	Blob       string         `json:"blob"`
	MappingID  string         `json:"mapping_id,omitempty"`
	Range      map[string]any `json:"range"`
}
type Observation struct {
	Kind           RelationKind   `json:"kind"`
	From           map[string]any `json:"from"`
	To             map[string]any `json:"to"`
	OriginalAnchor Anchor         `json:"original_anchor"`
	VirtualAnchor  *Anchor        `json:"virtual_anchor,omitempty"`
	Supports       []any          `json:"supports"`
	DoesNotSupport []any          `json:"does_not_support"`
}
type AnalysisRequest struct {
	Session   Session
	Seed      Seed
	Relations []RelationKind
	Limits    Limits
	Documents []Document
}
type CustodyRequest struct {
	OriginalURI           string
	WorkspaceRevision     any
	FailOnUnknownRevision bool
}
type AnalysisResult struct {
	Status       Status
	Observations []Observation
	Failure      string
}
type Analyzer interface {
	QualifiedRelationKinds() []RelationKind
	Analyze(context.Context, AnalysisRequest) (AnalysisResult, error)
}
type Custody interface {
	Resolve(context.Context, CustodyRequest) ([]Document, error)
}
type Coverage struct {
	Status             Status         `json:"status"`
	QualifiedRelations []RelationKind `json:"qualified_relations"`
	RequestedRelations []RelationKind `json:"requested_relations"`
}
type Envelope struct {
	Provider     Identity      `json:"provider"`
	Protocol     Identity      `json:"protocol"`
	Adapter      Identity      `json:"adapter"`
	Authority    string        `json:"authority"`
	Coverage     Coverage      `json:"coverage"`
	Failure      string        `json:"failure,omitempty"`
	Documents    []Document    `json:"documents"`
	Observations []Observation `json:"observations"`
}
type Provider struct {
	Analyzer         Analyzer
	Custody          Custody
	MaxRequestBytes  int64
	MaxResponseBytes int64
}

func (p Provider) Serve(ctx context.Context, input io.Reader, output io.Writer) error {
	maxReq := p.MaxRequestBytes
	if maxReq <= 0 {
		maxReq = defaultMaxBytes
	}
	body, err := readOneFrame(input, maxReq)
	if err != nil {
		return err
	}
	var req collectorRequest
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		return fmt.Errorf("request JSON: %w", err)
	}
	if dec.Decode(&struct{}{}) != io.EOF {
		return errors.New("request must contain exactly one JSON object")
	}
	if err := validateRequest(req, int64(len(body))); err != nil {
		return err
	}

	qualified := qualifiedKinds(p.Analyzer)
	env := Envelope{Provider: Identity{Name: "ember-glint", Version: "1"}, Protocol: Identity{Name: ProtocolName, Version: ProtocolVersion}, Adapter: parseIdentity(req.AdapterID), Authority: "PROVIDER_REPORTED", Coverage: Coverage{QualifiedRelations: qualified, RequestedRelations: req.Relations}, Documents: []Document{}, Observations: []Observation{}}
	docs, custodyErr := resolveCustody(ctx, p.Custody, req)
	env.Documents = docs
	if custodyErr != nil {
		env.Coverage.Status = StatusUnavailable
		env.Failure = custodyErr.Error()
	} else if !allQualified(req.Relations, qualified) {
		env.Coverage.Status = StatusUnsupported
		env.Failure = "one or more requested relations are not qualified"
	} else if p.Analyzer == nil {
		env.Coverage.Status = StatusUnavailable
		env.Failure = "analyzer unavailable"
	} else {
		result, analyzeErr := p.Analyzer.Analyze(ctx, AnalysisRequest{Session: req.Session, Seed: req.Seed, Relations: append([]RelationKind(nil), req.Relations...), Limits: req.Limits, Documents: append([]Document(nil), docs...)})
		if analyzeErr != nil {
			if errors.Is(analyzeErr, ErrUnavailable) {
				env.Coverage.Status = StatusUnavailable
			} else {
				env.Coverage.Status = StatusFailed
			}
			env.Failure = analyzeErr.Error()
		} else {
			env.Coverage.Status = result.Status
			env.Failure = result.Failure
			env.Observations = append([]Observation(nil), result.Observations...)
			if env.Coverage.Status == "" {
				if len(env.Observations) == 0 {
					env.Coverage.Status = StatusEmpty
				} else {
					env.Coverage.Status = StatusPartial
				}
			}
			if env.Coverage.Status != StatusPartial && env.Coverage.Status != StatusEmpty && env.Coverage.Status != StatusFailed && env.Coverage.Status != StatusUnavailable {
				return fmt.Errorf("invalid analyzer status %q", env.Coverage.Status)
			}
		}
	}
	if len(env.Documents) == 0 {
		return errors.New("custody returned no immutable documents")
	}
	if err := validateObservations(env.Observations, req.Relations, qualified); err != nil {
		return err
	}
	sortObservations(env.Observations)
	payload, err := json.Marshal(env)
	if err != nil {
		return err
	}
	maxResp := p.MaxResponseBytes
	if maxResp <= 0 {
		maxResp = defaultMaxBytes
	}
	if req.Limits.MaxBytes > 0 && req.Limits.MaxBytes < maxResp {
		maxResp = req.Limits.MaxBytes
	}
	if int64(len(payload)) > maxResp {
		return fmt.Errorf("response exceeds max_bytes bound %d", maxResp)
	}
	_, err = fmt.Fprintf(output, "Content-Length: %d\r\n\r\n%s", len(payload), payload)
	return err
}

func readOneFrame(r io.Reader, max int64) ([]byte, error) {
	br := bufio.NewReader(r)
	line, err := br.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("Content-Length header: %w", err)
	}
	if !strings.HasSuffix(line, "\r\n") || !strings.HasPrefix(line, "Content-Length: ") {
		return nil, errors.New("invalid Content-Length header")
	}
	n, err := strconv.ParseInt(strings.TrimSpace(strings.TrimPrefix(line, "Content-Length: ")), 10, 64)
	if err != nil || n < 0 {
		return nil, errors.New("invalid Content-Length")
	}
	if n > max {
		return nil, fmt.Errorf("request exceeds bound %d", max)
	}
	blank, err := br.ReadString('\n')
	if err != nil || blank != "\r\n" {
		return nil, errors.New("invalid frame separator")
	}
	body := make([]byte, n)
	if _, err := io.ReadFull(br, body); err != nil {
		return nil, fmt.Errorf("short frame: %w", err)
	}
	if _, err := br.ReadByte(); err != io.EOF {
		return nil, errors.New("expected exactly one Content-Length frame")
	}
	return body, nil
}
func validateRequest(r collectorRequest, n int64) error {
	if r.SchemaVersion != RequestSchema {
		return fmt.Errorf("request schema must be %s", RequestSchema)
	}
	if r.ProviderID != ProviderID {
		return fmt.Errorf("provider identity must be %s", ProviderID)
	}
	if r.AdapterID == "" || r.Session.SessionID == "" || r.Session.Generation < 1 || r.Seed.URI == "" || r.DocumentCustody.OriginalURI == "" {
		return errors.New("request identity and custody fields must be non-empty")
	}
	if id := parseIdentity(r.AdapterID); id.Version == "unknown" {
		return errors.New("adapter identity must use name@version")
	}
	if len(r.Relations) == 0 || r.Limits.MaxNodes < 1 || r.Limits.TimeoutMS < 1 || r.Limits.RequestTimeoutMS < 1 {
		return errors.New("request bounds and relations must be positive")
	}
	if r.Limits.MaxMessages != 0 && r.Limits.MaxMessages != 1 {
		return errors.New("max_messages must permit exactly one protocol message")
	}
	if r.Limits.MaxBytes > 0 && n > r.Limits.MaxBytes {
		return fmt.Errorf("request exceeds declared max_bytes %d", r.Limits.MaxBytes)
	}
	seen := map[RelationKind]bool{}
	for _, k := range r.Relations {
		if seen[k] {
			return errors.New("relations must be unique")
		}
		seen[k] = true
	}
	return nil
}
func resolveCustody(ctx context.Context, c Custody, r collectorRequest) ([]Document, error) {
	if c == nil {
		return nil, ErrUnavailable
	}
	return c.Resolve(ctx, CustodyRequest{r.DocumentCustody.OriginalURI, r.DocumentCustody.WorkspaceRevision, r.DocumentCustody.FailOnUnknownRevision})
}
func qualifiedKinds(a Analyzer) []RelationKind {
	if a == nil {
		return []RelationKind{}
	}
	ks := append([]RelationKind(nil), a.QualifiedRelationKinds()...)
	sort.Slice(ks, func(i, j int) bool { return ks[i] < ks[j] })
	out := ks[:0]
	for _, k := range ks {
		if len(out) == 0 || out[len(out)-1] != k {
			out = append(out, k)
		}
	}
	return out
}
func allQualified(want, have []RelationKind) bool {
	m := map[RelationKind]bool{}
	for _, k := range have {
		m[k] = true
	}
	for _, k := range want {
		if !m[k] {
			return false
		}
	}
	return true
}
func parseIdentity(s string) Identity {
	p := strings.LastIndexByte(s, '@')
	if p < 1 || p == len(s)-1 {
		return Identity{Name: s, Version: "unknown"}
	}
	return Identity{Name: s[:p], Version: s[p+1:]}
}
func validateObservations(obs []Observation, requested, qualified []RelationKind) error {
	for _, o := range obs {
		if !allQualified([]RelationKind{o.Kind}, requested) || !allQualified([]RelationKind{o.Kind}, qualified) {
			return fmt.Errorf("observation kind %s is not requested and qualified", o.Kind)
		}
		if o.From == nil || o.To == nil || o.OriginalAnchor.DocumentID == "" || o.OriginalAnchor.URI == "" || o.OriginalAnchor.Revision == "" || o.OriginalAnchor.Blob == "" || o.OriginalAnchor.Range == nil || o.Supports == nil || o.DoesNotSupport == nil {
			return fmt.Errorf("observation kind %s is incomplete", o.Kind)
		}
	}
	return nil
}
func sortObservations(obs []Observation) {
	sort.SliceStable(obs, func(i, j int) bool {
		a, _ := json.Marshal(obs[i])
		b, _ := json.Marshal(obs[j])
		return bytes.Compare(a, b) < 0
	})
}
