//go:build darwin || linux

package privatehostproductv3

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	pilot "lsp-trace/internal/censuslineagepilot"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

// This is deterministic IPC, not a host runtime or inference API. The existing
// host Agent tool owns role launch and semantic payload authorship.
type Ready struct{ Protocol, Role, LaunchID, PromptSHA256 string }
type Part struct {
	Path, SHA256 string
	Bytes        int
}
type Bundle struct {
	Event, Directory              string
	Protocol, Role, RequestDigest string
	Sequence                      uint64
	Bytes                         int
	Parts                         []Part
	ReceiptSHA256                 string
}
type Mailbox struct {
	Directory                         string
	MaxRequestBytes, MaxResponseBytes int
	ExchangeTimeout                   time.Duration
	Observation                       Lifecycle
}

func (m Mailbox) Lifecycle() Lifecycle {
	r := m.Observation
	if r.RoleCleanup == "" {
		r.RoleCleanup = "UNOBSERVED_HOST_ROLE_RETURN"
	}
	return r
}

func PrepareMailbox(path string) error {
	if e := os.Mkdir(path, 0700); e != nil && !os.IsExist(e) {
		return e
	}
	return privateDir(path)
}
func mailboxStore(path string) (*store, error) {
	if privateDir(path) != nil {
		return nil, ErrInvalid
	}
	r, e := os.OpenRoot(path)
	if e != nil {
		return nil, e
	}
	return &store{root: r}, nil
}
func MarkReady(path string, r Ready) error {
	if r.Protocol != Protocol || (r.Role != "interpreter" && r.Role != "reviewer") || r.LaunchID == "" || r.PromptSHA256 != PromptDigest(r.Role) {
		return ErrInvalid
	}
	s, e := mailboxStore(path)
	if e != nil {
		return e
	}
	defer s.close()
	return s.write("ready-"+r.Role+".json", mustJSON(r))
}
func (m Mailbox) CheckReady() error {
	if m.ExchangeTimeout <= 0 || m.MaxRequestBytes < 1 || m.MaxRequestBytes > 1<<20 || m.MaxResponseBytes < 1 || m.MaxResponseBytes > 64<<10 {
		return ErrInvalid
	}
	s, e := mailboxStore(m.Directory)
	if e != nil {
		return e
	}
	defer s.close()
	var ids [2]string
	for i, role := range []string{"interpreter", "reviewer"} {
		b, e := s.read("ready-"+role+".json", 4096)
		var r Ready
		if e != nil || strict(b, &r) != nil || r.Protocol != Protocol || r.Role != role || r.LaunchID == "" || r.PromptSHA256 != PromptDigest(role) {
			return ErrInvalid
		}
		ids[i] = r.LaunchID
	}
	if ids[0] == ids[1] {
		return ErrInvalid
	}
	return nil
}
func (m Mailbox) Exchange(ctx context.Context, t Task) (Response, error) {
	var zero Response
	if e := m.CheckReady(); e != nil {
		return zero, e
	}
	if t.Protocol != Protocol || (t.Role != "interpreter" && t.Role != "reviewer") || t.Sequence == 0 {
		return zero, ErrInvalid
	}
	raw := mustJSON(t)
	if len(raw) > m.MaxRequestBytes {
		return zero, ErrPaused
	}
	s, e := mailboxStore(m.Directory)
	if e != nil {
		return zero, e
	}
	defer s.close()
	bundle := Bundle{Event: "NeedsHost", Directory: m.Directory, Protocol: Protocol, Role: t.Role, RequestDigest: t.RequestDigest, Sequence: t.Sequence, Bytes: len(raw), Parts: []Part{}}
	for start, index := 0, 0; start < len(raw); index++ {
		end := start + 20000
		if end > len(raw) {
			end = len(raw)
		}
		for end < len(raw) && !utf8.RuneStart(raw[end]) {
			end--
		}
		if end == start {
			return zero, ErrInvalid
		}
		name := fmt.Sprintf("part-%08d-%04d.json", t.Sequence, index)
		part := raw[start:end]
		if e = s.write(name, part); e != nil {
			return zero, e
		}
		bundle.Parts = append(bundle.Parts, Part{Path: filepath.Join(m.Directory, name), SHA256: hash(part), Bytes: len(part)})
		start = end
	}
	bundle.ReceiptSHA256 = hash(mustJSON(bundle))
	metadata := mustJSON(bundle)
	if len(metadata) >= 48000 {
		return zero, ErrPaused
	}
	if e = s.write(fmt.Sprintf("request-%08d.json", t.Sequence), metadata); e != nil {
		return zero, e
	}
	wait, cancel := context.WithTimeout(ctx, m.ExchangeTimeout)
	defer cancel()
	timer := time.NewTicker(10 * time.Millisecond)
	defer timer.Stop()
	for {
		b, e := s.read(fmt.Sprintf("reply-%08d.json", t.Sequence), m.MaxResponseBytes)
		if e == nil {
			var r Response
			if strict(b, &r) != nil || r.Protocol != Protocol || r.RequestDigest != t.RequestDigest {
				return zero, ErrInvalid
			}
			return r, nil
		}
		if !os.IsNotExist(e) {
			return zero, e
		}
		select {
		case <-wait.Done():
			return zero, wait.Err()
		case <-timer.C:
		}
	}
}
func hash(raw []byte) string { return pilot.Digest(raw) }

// WaitBundle selects own Role and monotone Sequence from CLI-produced metadata;
// filenames and sequence parity never bind a role. It returns the COMPLETE
// bounded receipt+manifest or refuses; it never truncates it.
func WaitBundle(ctx context.Context, path, role string, after uint64) (Bundle, error) {
	var out Bundle
	if role != "interpreter" && role != "reviewer" {
		return out, ErrInvalid
	}
	s, e := mailboxStore(path)
	if e != nil {
		return out, e
	}
	defer s.close()
	timer := time.NewTicker(10 * time.Millisecond)
	defer timer.Stop()
	for {
		entries, e := os.ReadDir(path)
		if e != nil {
			return out, e
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
		for _, entry := range entries {
			if !strings.HasPrefix(entry.Name(), "request-") || !strings.HasSuffix(entry.Name(), ".json") {
				continue
			}
			b, e := s.read(entry.Name(), 47999)
			if os.IsNotExist(e) {
				continue
			}
			var bundle Bundle
			if e != nil || strict(b, &bundle) != nil {
				return out, ErrInvalid
			}
			if bundle.Protocol != Protocol || bundle.Directory != path || bundle.Event != "NeedsHost" {
				return out, ErrInvalid
			}
			if bundle.Role != role || bundle.Sequence <= after {
				continue
			}
			receipt := bundle.ReceiptSHA256
			bundle.ReceiptSHA256 = ""
			if receipt != hash(mustJSON(bundle)) {
				return out, ErrInvalid
			}
			bundle.ReceiptSHA256 = receipt
			if bundle.Bytes < 1 || bundle.Bytes > 1<<20 || len(bundle.Parts) == 0 || len(bundle.Parts) > 128 {
				return out, ErrInvalid
			}
			return bundle, nil
		}
		if b, e := s.read("terminal.json", stateLimit); e == nil && len(b) > 0 {
			return out, ioTerminal
		}
		select {
		case <-ctx.Done():
			return out, ctx.Err()
		case <-timer.C:
		}
	}
}

var ioTerminal = errors.New("TERMINAL_PRIVATE_HOST_PRODUCT_V3")

// ReadBundle is the fixture/CLI exact-byte verifier; a model role must still read
// ALL exported parts through ordinary read tools, in parallel grouped turns.
func ReadBundle(b Bundle) (Task, error) {
	var t Task
	var raw []byte
	if b.Protocol != Protocol || b.Event != "NeedsHost" || b.Bytes < 1 || b.Bytes > 1<<20 {
		return t, ErrInvalid
	}
	s, e := mailboxStore(b.Directory)
	if e != nil {
		return t, e
	}
	registered, e := s.read(fmt.Sprintf("request-%08d.json", b.Sequence), 47999)
	s.close()
	if e != nil || !bytes.Equal(registered, mustJSON(b)) {
		return t, ErrInvalid
	}
	for index, part := range b.Parts {
		if filepath.Dir(part.Path) != b.Directory || filepath.Base(part.Path) != fmt.Sprintf("part-%08d-%04d.json", b.Sequence, index) {
			return t, ErrInvalid
		}
		if part.Bytes < 1 || part.Bytes > 20000 {
			return t, ErrInvalid
		}
		s, e := mailboxStore(filepath.Dir(part.Path))
		if e != nil {
			return t, e
		}
		data, e := s.read(filepath.Base(part.Path), 20000)
		s.close()
		if e != nil || len(data) != part.Bytes || hash(data) != part.SHA256 {
			return t, ErrInvalid
		}
		if len(raw) > b.Bytes-len(data) {
			return t, ErrInvalid
		}
		raw = append(raw, data...)
	}
	if len(raw) != b.Bytes || strict(raw, &t) != nil || t.Role != b.Role || t.Sequence != b.Sequence || t.Protocol != Protocol || t.RequestDigest != b.RequestDigest {
		return t, ErrInvalid
	}
	return t, nil
}

// Submit canonicalizes model-authored payload JSON; it never authors semantics.
// It binds the exact CLI-produced own-role request before create-only response.
func Submit(path, role string, sequence uint64, payload []byte) error {
	if len(payload) > 64<<10 {
		return ErrPaused
	}
	s, e := mailboxStore(path)
	if e != nil {
		return e
	}
	defer s.close()
	b, e := s.read(fmt.Sprintf("request-%08d.json", sequence), 47999)
	var bundle Bundle
	if e != nil || strict(b, &bundle) != nil || bundle.Role != role || bundle.Sequence != sequence || bundle.Protocol != Protocol {
		return ErrInvalid
	}
	r, e := decodeResponse(payload)
	if e != nil || r.Protocol != Protocol || r.RequestDigest != bundle.RequestDigest {
		return ErrInvalid
	}
	return s.write(fmt.Sprintf("reply-%08d.json", sequence), mustJSON(r))
}

// Response is a closed, flat scalar schema. Inspect decoded object keys BEFORE
// canonicalization: encoding/json's struct decoder otherwise accepts duplicate
// fields and case aliases with last-value-wins semantics. Nested review/citation
// objects are not this protocol and cannot be laundered into a clean Response.
// The existing 64 KiB wire cap bounds this pass; no recursive object walk or map
// of arbitrary caller keys is introduced. Escaped keys are compared decoded.
func decodeResponse(payload []byte) (Response, error) {
	var r Response
	if len(payload) > 64<<10 || !utf8.Valid(payload) {
		return r, ErrInvalid
	}
	d := json.NewDecoder(bytes.NewReader(payload))
	opening, e := d.Token()
	if e != nil || opening != json.Delim('{') {
		return r, ErrInvalid
	}
	seen := make(map[string]bool, 11)
	for d.More() {
		key, e := d.Token()
		if e != nil {
			return Response{}, ErrInvalid
		}
		name, ok := key.(string)
		if !ok || seen[name] {
			return Response{}, ErrInvalid
		}
		var field any
		switch name {
		case "Protocol":
			field = &r.Protocol
		case "RequestDigest":
			field = &r.RequestDigest
		case "Label":
			field = &r.Label
		case "Explanation":
			field = &r.Explanation
		case "Alternative":
			field = &r.Alternative
		case "RejectionCondition":
			field = &r.RejectionCondition
		case "Quote":
			field = &r.Quote
		case "Start":
			field = &r.Start
		case "End":
			field = &r.End
		case "Supported":
			field = &r.Supported
		default:
			return Response{}, ErrInvalid
		}
		seen[name] = true
		if d.Decode(field) != nil {
			return Response{}, ErrInvalid
		}
	}
	closing, e := d.Token()
	if e != nil || closing != json.Delim('}') || d.Decode(new(any)) != io.EOF {
		return Response{}, ErrInvalid
	}
	return r, nil
}

func (m Mailbox) Terminal(r Report) error {
	s, e := mailboxStore(m.Directory)
	if e != nil {
		return e
	}
	defer s.close()
	return s.write("terminal.json", mustJSON(r))
}
