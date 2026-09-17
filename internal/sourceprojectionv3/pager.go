package sourceprojectionv3

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
)

const CursorVersion = 1

var (
	ErrInvalidRequest = errors.New("sourceprojectionv3: invalid paging request")
	ErrInvalidCursor  = errors.New("sourceprojectionv3: invalid cursor")
	ErrResourceLimit  = errors.New("sourceprojectionv3: resource limit")
)

// Record is one indivisible projection record. Data must contain one canonical
// JSON value. Objects, ranges, source bytes, and work are cumulative costs.
type Record struct {
	Kind        string          `json:"kind"`
	Key         string          `json:"key"`
	Data        json.RawMessage `json:"data"`
	Objects     uint64          `json:"objects"`
	Ranges      uint64          `json:"ranges"`
	SourceBytes uint64          `json:"source_bytes"`
	Work        uint64          `json:"work"`
}

type Binding struct {
	RequestDigest    string `json:"request_digest"`
	CustodyMode      string `json:"custody_mode"`
	CustodyDigest    string `json:"custody_digest"`
	ProjectionDigest string `json:"projection_digest"`
}

type Limits struct {
	MaxPageBytes     uint64 `json:"max_page_bytes"`
	MaxPages         uint64 `json:"max_pages"`
	MaxResponseBytes uint64 `json:"max_response_bytes"`
	MaxObjects       uint64 `json:"max_objects"`
	MaxRanges        uint64 `json:"max_ranges"`
	MaxSourceBytes   uint64 `json:"max_source_bytes"`
	MaxWork          uint64 `json:"max_work"`
}

type Accounting struct {
	Pages         uint64 `json:"pages"`
	ResponseBytes uint64 `json:"response_bytes"`
	Objects       uint64 `json:"objects"`
	Ranges        uint64 `json:"ranges"`
	SourceBytes   uint64 `json:"source_bytes"`
	Work          uint64 `json:"work"`
}

type Header struct {
	SchemaVersion        string `json:"schema_version"`
	Authority            int    `json:"authority"`
	SourceGraphComplete  string `json:"source_graph_complete"`
	GraphFactsAdded      int    `json:"graph_facts_added"`
	CustodyMode          string `json:"custody_mode"`
	PhysicalProjectionID string `json:"physical_projection_id"`
	RequestPolicyID      string `json:"request_policy_id"`
	Status               string `json:"status"`
}

type Request struct {
	Header  Header
	Binding Binding
	Limits  Limits
	Cursor  string
}

type Page struct {
	Header
	Records    []Record   `json:"records"`
	Accounting Accounting `json:"accounting"`
	NextCursor string     `json:"next_cursor,omitempty"`
	Complete   bool       `json:"complete"`
}

type cursorPayload struct {
	Version    int        `json:"version"`
	Header     Header     `json:"header"`
	Binding    Binding    `json:"binding"`
	Limits     Limits     `json:"limits"`
	Next       uint64     `json:"next"`
	Accounting Accounting `json:"accounting"`
}

type cursorEnvelope struct {
	Payload json.RawMessage `json:"payload"`
	Digest  string          `json:"digest"`
}

func Paginate(records []Record, request Request) (Page, error) {
	if err := validateRequest(records, request); err != nil {
		return Page{}, err
	}
	state := cursorPayload{Version: CursorVersion, Header: request.Header, Binding: request.Binding, Limits: request.Limits}
	if request.Cursor != "" {
		decoded, err := decodeCursor(request.Cursor)
		if err != nil || decoded.Header != request.Header || decoded.Binding != request.Binding || decoded.Limits != request.Limits || decoded.Next > uint64(len(records)) {
			return Page{}, ErrInvalidCursor
		}
		state = decoded
	}
	if state.Accounting.Pages >= request.Limits.MaxPages {
		return Page{}, ErrResourceLimit
	}

	page := Page{Header: request.Header, Records: make([]Record, 0)}
	pageBytes := uint64(0)
	for state.Next < uint64(len(records)) {
		record := records[state.Next]
		raw, _ := json.Marshal(record)
		recordBytes := uint64(len(raw))
		if recordBytes > request.Limits.MaxPageBytes {
			return Page{}, ErrResourceLimit
		}
		combinedPageBytes, ok := checkedAdd(pageBytes, recordBytes)
		if !ok {
			return Page{}, ErrResourceLimit
		}
		if len(page.Records) > 0 && combinedPageBytes > request.Limits.MaxPageBytes {
			break
		}
		next := state.Accounting
		if next.Objects, ok = checkedAdd(next.Objects, record.Objects); !ok {
			return Page{}, ErrResourceLimit
		}
		if next.Ranges, ok = checkedAdd(next.Ranges, record.Ranges); !ok {
			return Page{}, ErrResourceLimit
		}
		if next.SourceBytes, ok = checkedAdd(next.SourceBytes, record.SourceBytes); !ok {
			return Page{}, ErrResourceLimit
		}
		if next.Work, ok = checkedAdd(next.Work, record.Work); !ok {
			return Page{}, ErrResourceLimit
		}
		if exceeds(next.Objects, request.Limits.MaxObjects) || exceeds(next.Ranges, request.Limits.MaxRanges) || exceeds(next.SourceBytes, request.Limits.MaxSourceBytes) || exceeds(next.Work, request.Limits.MaxWork) {
			return Page{}, ErrResourceLimit
		}
		page.Records = append(page.Records, cloneRecord(record))
		pageBytes = combinedPageBytes
		state.Accounting = next
		state.Next++
	}
	state.Accounting.Pages++
	page.Complete = state.Next == uint64(len(records))
	if err := finalizeResponseAccounting(&page, &state, request.Limits.MaxResponseBytes); err != nil {
		return Page{}, err
	}
	return page, nil
}

func finalizeResponseAccounting(page *Page, state *cursorPayload, maxResponseBytes uint64) error {
	prior := state.Accounting.ResponseBytes
	total := prior
	for range 32 {
		state.Accounting.ResponseBytes = total
		page.Accounting = state.Accounting
		page.NextCursor = ""
		if !page.Complete {
			cursor, err := encodeCursor(*state)
			if err != nil {
				return err
			}
			page.NextCursor = cursor
		}
		raw, err := json.Marshal(page)
		if err != nil {
			return err
		}
		next, ok := checkedAdd(prior, uint64(len(raw)))
		if !ok || next > maxResponseBytes {
			return ErrResourceLimit
		}
		if next == total {
			return nil
		}
		total = next
	}
	return ErrResourceLimit
}

func validateRequest(records []Record, request Request) error {
	if request.Header.SchemaVersion != "lsp-trace.source-projection.v3" || request.Header.Authority != 0 || request.Header.SourceGraphComplete != "UNKNOWN" || request.Header.GraphFactsAdded != 0 || request.Header.CustodyMode == "" || request.Header.CustodyMode != request.Binding.CustodyMode || request.Header.PhysicalProjectionID == "" || request.Header.RequestPolicyID == "" || request.Header.Status == "" || request.Binding.RequestDigest == "" || request.Binding.CustodyDigest == "" || request.Binding.ProjectionDigest == "" {
		return ErrInvalidRequest
	}
	limits := request.Limits
	if limits.MaxPageBytes == 0 || limits.MaxPages == 0 || limits.MaxResponseBytes == 0 {
		return ErrResourceLimit
	}
	seen := make(map[string]struct{}, len(records))
	for _, record := range records {
		if record.Kind == "" || record.Key == "" || !json.Valid(record.Data) {
			return ErrInvalidRequest
		}
		identity := record.Kind + "\x00" + record.Key
		if _, ok := seen[identity]; ok {
			return ErrInvalidRequest
		}
		seen[identity] = struct{}{}
	}
	return nil
}

func exceeds(value, limit uint64) bool { return value > limit }

func checkedAdd(left, right uint64) (uint64, bool) {
	if ^uint64(0)-left < right {
		return 0, false
	}
	return left + right, true
}

func cloneRecord(record Record) Record {
	record.Data = append(json.RawMessage(nil), record.Data...)
	return record
}

func encodeCursor(payload cursorPayload) (string, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(raw)
	envelope, err := json.Marshal(cursorEnvelope{Payload: raw, Digest: "sha256:" + hex.EncodeToString(digest[:])})
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(envelope), nil
}

func decodeCursor(cursor string) (cursorPayload, error) {
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return cursorPayload{}, ErrInvalidCursor
	}
	var envelope cursorEnvelope
	if json.Unmarshal(raw, &envelope) != nil || !json.Valid(envelope.Payload) {
		return cursorPayload{}, ErrInvalidCursor
	}
	digest := sha256.Sum256(envelope.Payload)
	if envelope.Digest != "sha256:"+hex.EncodeToString(digest[:]) {
		return cursorPayload{}, ErrInvalidCursor
	}
	var payload cursorPayload
	if json.Unmarshal(envelope.Payload, &payload) != nil || payload.Version != CursorVersion {
		return cursorPayload{}, ErrInvalidCursor
	}
	return payload, nil
}

func RecordDigest(records []Record) (string, error) {
	raw, err := json.Marshal(records)
	if err != nil {
		return "", fmt.Errorf("sourceprojectionv3: encode records: %w", err)
	}
	digest := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(digest[:]), nil
}
