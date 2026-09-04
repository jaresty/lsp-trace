package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
)

const CollectorRequestSchema = "lsp-trace.provider-collector-request.v1"

// Executor is the transport-only surface consumed by Collector.
type Executor interface {
	Execute(context.Context, string, json.RawMessage, Limits) Receipt
}

// Admission is a host-authored decision. Its provider identity and limits never
// come from the operation request.
type Admission struct {
	ProviderID string
	AdapterID  string
	Limits     Limits
}

type Selection struct {
	Relations  []string
	Languages  []string
	Frameworks []string
	Providers  []string
	Adapters   json.RawMessage
}

type AdmissionResolver interface {
	Admit(context.Context, Selection) (Admission, error)
}

// SemanticAdapter owns validation and interpretation of provider output.
type SemanticAdapter interface {
	Adapt(context.Context, StrictCollectorRequest, Receipt) (json.RawMessage, error)
}

type Collector struct {
	runtime  Executor
	admitter AdmissionResolver
	adapter  SemanticAdapter
}

func NewCollector(runtime Executor, admitter AdmissionResolver, adapter SemanticAdapter) (*Collector, error) {
	if runtime == nil || admitter == nil || adapter == nil {
		return nil, errors.New("provider collector requires runtime, admission resolver, and semantic adapter")
	}
	return &Collector{runtime: runtime, admitter: admitter, adapter: adapter}, nil
}

type ManagedSessionCustody struct {
	SessionID  string `json:"session_id"`
	Generation uint64 `json:"generation"`
}

type SeedCustody struct {
	URI       string  `json:"uri"`
	Line      *uint32 `json:"line,omitempty"`
	Character *uint32 `json:"character,omitempty"`
	Symbol    string  `json:"symbol,omitempty"`
	StartMode string  `json:"start_mode,omitempty"`
}

type DocumentCustody struct {
	OriginalURI       string          `json:"original_uri"`
	WorkspaceRevision json.RawMessage `json:"workspace_revision,omitempty"`
	FailOnUnknown     bool            `json:"fail_on_unknown_revision"`
}

type CollectorLimits struct {
	MaxDepth         int   `json:"max_depth,omitempty"`
	DownDepth        int   `json:"down_depth,omitempty"`
	UpDepth          int   `json:"up_depth,omitempty"`
	MaxNodes         int   `json:"max_nodes"`
	MaxMessages      int   `json:"max_messages,omitempty"`
	MaxBytes         int   `json:"max_bytes,omitempty"`
	TimeoutMS        int64 `json:"timeout_ms"`
	RequestTimeoutMS int64 `json:"request_timeout_ms"`
}

type StrictCollectorRequest struct {
	SchemaVersion string                `json:"schema_version"`
	ProviderID    string                `json:"provider_id"`
	AdapterID     string                `json:"adapter_id"`
	Session       ManagedSessionCustody `json:"session"`
	Seed          SeedCustody           `json:"seed"`
	Relations     []string              `json:"relations"`
	Documents     DocumentCustody       `json:"document_custody"`
	Limits        CollectorLimits       `json:"limits"`
}

type operationInput struct {
	SessionID             string          `json:"session_id"`
	Generation            uint64          `json:"generation"`
	URI                   string          `json:"uri"`
	Line                  *uint32         `json:"line"`
	Character             *uint32         `json:"character"`
	Symbol                string          `json:"symbol"`
	StartMode             string          `json:"start_mode"`
	MaxDepth              int             `json:"max_depth"`
	DownDepth             int             `json:"down_depth"`
	UpDepth               int             `json:"up_depth"`
	MaxNodes              int             `json:"max_nodes"`
	MaxMessages           int             `json:"max_messages"`
	MaxBytes              int             `json:"max_bytes"`
	TimeoutMS             int64           `json:"timeout_ms"`
	RequestTimeoutMS      int64           `json:"request_timeout_ms"`
	Relations             *[]string       `json:"relations"`
	Languages             []string        `json:"languages"`
	Frameworks            []string        `json:"frameworks"`
	Adapters              json.RawMessage `json:"adapters"`
	Providers             []string        `json:"providers"`
	WorkspaceRevision     json.RawMessage `json:"workspace_revision"`
	FailOnUnknownRevision bool            `json:"fail_on_unknown_revision"`
}

func (c *Collector) CollectRelations(ctx context.Context, relations []string, raw json.RawMessage) (json.RawMessage, error) {
	if c == nil || c.runtime == nil || c.admitter == nil || c.adapter == nil {
		return nil, errors.New("relation collector unavailable")
	}
	if len(relations) == 0 {
		return nil, errors.New("relation collector requires admitted relations")
	}
	var in operationInput
	if err := decodeCollectorInput(raw, &in); err != nil {
		return nil, fmt.Errorf("strict collector input: %w", err)
	}
	selected := append([]string(nil), relations...)
	sort.Strings(selected)
	admission, err := c.admitter.Admit(ctx, Selection{
		Relations: append([]string(nil), selected...), Languages: append([]string(nil), in.Languages...),
		Frameworks: append([]string(nil), in.Frameworks...), Providers: append([]string(nil), in.Providers...),
		Adapters: append(json.RawMessage(nil), in.Adapters...),
	})
	if err != nil {
		return nil, err
	}
	if admission.ProviderID == "" || admission.AdapterID == "" {
		return nil, errors.New("provider admission omitted identity")
	}
	request := StrictCollectorRequest{
		SchemaVersion: CollectorRequestSchema,
		ProviderID:    admission.ProviderID,
		AdapterID:     admission.AdapterID,
		Session:       ManagedSessionCustody{SessionID: in.SessionID, Generation: in.Generation},
		Seed:          SeedCustody{URI: in.URI, Line: in.Line, Character: in.Character, Symbol: in.Symbol, StartMode: in.StartMode},
		Relations:     selected,
		Documents:     DocumentCustody{OriginalURI: in.URI, WorkspaceRevision: append(json.RawMessage(nil), in.WorkspaceRevision...), FailOnUnknown: in.FailOnUnknownRevision},
		Limits:        CollectorLimits{MaxDepth: in.MaxDepth, DownDepth: in.DownDepth, UpDepth: in.UpDepth, MaxNodes: in.MaxNodes, MaxMessages: in.MaxMessages, MaxBytes: in.MaxBytes, TimeoutMS: in.TimeoutMS, RequestTimeoutMS: in.RequestTimeoutMS},
	}
	encoded, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	receipt := c.runtime.Execute(ctx, admission.ProviderID, encoded, admission.Limits)
	adapted, err := c.adapter.Adapt(ctx, request, cloneReceipt(receipt))
	if err != nil {
		return nil, err
	}
	return append(json.RawMessage(nil), adapted...), nil
}

func decodeCollectorInput(raw []byte, target any) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(target); err != nil {
		return err
	}
	if err := d.Decode(&struct{}{}); err != io.EOF {
		return errors.New("one JSON value required")
	}
	return nil
}

func cloneReceipt(r Receipt) Receipt {
	r.Response = append(json.RawMessage(nil), r.Response...)
	r.Stderr.Bytes = append([]byte(nil), r.Stderr.Bytes...)
	if r.Failure != nil {
		copyFailure := *r.Failure
		r.Failure = &copyFailure
	}
	return r
}
