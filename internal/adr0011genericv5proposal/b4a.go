package adr0011genericv5proposal

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// B4aClaim is a claimant envelope accompanied by separately supplied original
// and selector-preimage bytes. Neither a descriptor nor a private_ref retrieves bytes.
type B4aClaim struct {
	Role     string
	Envelope []byte
	Original []byte
	Artifact []byte
}

// B4aInput is held outside the claimant transaction. In particular the three
// originals and completed WRITE cannot be inferred from the claimant envelopes.
type B4aInput struct {
	Session, Transaction, Workspace, Method string
	Generation                              uint64
	URI, Version, Encoding                  string
	Line, Character                         uint64
	Source                                  []byte
	QueryOriginal, ApplicabilityOriginal    []byte
	Custody                                 string
	Language, NotebookType, NotebookURI     *string
	NotebookSourceSelector                  string
	RequestFrame, RequestParams             []byte
	RequestID                               []byte
	CompletedKey                            string
	CompletedOrdinal                        uint64
	WriteCompleted                          bool
	Claims                                  []B4aClaim
}

type b4aEnvelope struct {
	Role     string `json:"role"`
	Identity struct {
		Session     string `json:"session"`
		Generation  uint64 `json:"generation"`
		Transaction string `json:"transaction"`
	} `json:"identity"`
	Original     b4aBytes        `json:"original"`
	Predecessors []b4aDependency `json:"predecessors"`
	Payload      json.RawMessage `json:"payload"`
}
type b4aBytes struct {
	Length int    `json:"length"`
	SHA    string `json:"sha256"`
	Ref    string `json:"private_ref"`
}
type b4aDependency struct{ Role, Selector, Digest string }

// Only complete framing correspondence is checked here. C's independent
// frame/message/byte/deadline budget is deliberately outside B4a.
func b4aFrame(raw []byte) (map[string]json.RawMessage, error) {
	header, body, ok := bytes.Cut(raw, []byte("\r\n\r\n"))
	if !ok {
		return nil, fmt.Errorf("incomplete framed request")
	}
	var length uint64
	seenLength := false
	for _, line := range bytes.Split(header, []byte("\r\n")) {
		name, value, found := bytes.Cut(line, []byte(":"))
		if !found || len(name) == 0 {
			return nil, fmt.Errorf("malformed framed request header")
		}
		for _, c := range name {
			if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || bytes.IndexByte([]byte("!#$%&'*+-.^_`|~"), c) >= 0) {
				return nil, fmt.Errorf("malformed framed request header")
			}
		}
		for _, c := range value {
			if c == '\n' || c == '\r' || c == 0 || (c < 0x20 && c != '\t') || c == 0x7f {
				return nil, fmt.Errorf("malformed framed request header")
			}
		}
		if bytes.EqualFold(name, []byte("Content-Length")) {
			if seenLength {
				return nil, fmt.Errorf("duplicate request Content-Length")
			}
			value = bytes.TrimSpace(value)
			if len(value) == 0 {
				return nil, fmt.Errorf("invalid request Content-Length")
			}
			for _, c := range value {
				if c < '0' || c > '9' {
					return nil, fmt.Errorf("invalid request Content-Length")
				}
			}
			var err error
			length, err = strconv.ParseUint(string(value), 10, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid request Content-Length")
			}
			seenLength = true
		}
	}
	if !seenLength || length != uint64(len(body)) {
		return nil, fmt.Errorf("request Content-Length mismatch")
	}
	return b4aFields(body)
}
func b4aDescriptor(d b4aBytes, raw []byte) bool {
	return raw != nil && d.Length == len(raw) && d.SHA == "sha256:"+a4Hash(raw) && d.Ref != ""
}
func b4aFields(raw json.RawMessage) (map[string]json.RawMessage, error) {
	if err := b4Strict(raw); err != nil {
		return nil, err
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil || m == nil {
		return nil, fmt.Errorf("invalid object: %v", err)
	}
	return m, nil
}
func b4aText(m map[string]json.RawMessage, k string) string {
	var s string
	_ = json.Unmarshal(m[k], &s)
	return s
}
func b4aUint(m map[string]json.RawMessage, k string) (uint64, error) {
	var n uint64
	err := json.Unmarshal(m[k], &n)
	return n, err
}
func b4aByteField(m map[string]json.RawMessage, k string, raw []byte) bool {
	var d b4aBytes
	return json.Unmarshal(m[k], &d) == nil && b4aDescriptor(d, raw)
}

// heldDigests contains only predecessors whose original bytes B4a independently holds.
// Other roles have A4-checked digest shape, not B4a original custody.
func b4aDeps(e b4aEnvelope, expected, heldDigests map[string]string) bool {
	if len(e.Predecessors) != len(expected) {
		return false
	}
	seen := map[string]bool{}
	for _, d := range e.Predecessors {
		want, ok := expected[d.Role]
		if !ok || seen[d.Role] || d.Selector == "" || d.Digest == "" || (want != "" && d.Selector != want) || (heldDigests[d.Role] != "" && d.Digest != heldDigests[d.Role]) {
			return false
		}
		seen[d.Role] = true
	}
	return true
}
func b4aPreimage(fields ...string) []byte {
	var out []byte
	for _, field := range fields {
		b := []byte(field)
		var size [8]byte
		binary.BigEndian.PutUint64(size[:], uint64(len(b)))
		out = append(out, size[:]...)
		out = append(out, b...)
	}
	return out
}
func b4aTransaction(in B4aInput) string {
	return "sha256:" + a4Hash(b4aPreimage("ADR0011-GENERIC-TRANSACTION/1", in.Session, strconv.FormatUint(in.Generation, 10), in.Transaction))
}
func b4aSelector(in B4aInput, role string, artifact []byte, fields ...string) (string, error) {
	return A4Selector(strings.TrimPrefix(in.Method, "textDocument/"), role, in.Session, strconv.FormatUint(in.Generation, 10), artifact, fields...)
}
func b4aOptional(m map[string]json.RawMessage, present, field string, held *string) bool {
	var flag bool
	if json.Unmarshal(m[present], &flag) != nil || flag != (held != nil) {
		return false
	}
	if held == nil {
		return string(m[field]) == "null"
	}
	return b4aByteField(m, field, []byte(*held))
}
func b4aNullOrBytes(m map[string]json.RawMessage, k string, held *string) bool {
	if held == nil {
		return string(m[k]) == "null"
	}
	return b4aByteField(m, k, []byte(*held))
}

// CheckB4a is private/offline correspondence only. A nil result is not issuance,
// admission, retention, chronology, completeness, or production authority.
func CheckB4a(in B4aInput) error {
	fail := func(s string) error { return fmt.Errorf("B4a %s", s) }
	if in.Session == "" || in.Generation == 0 || in.Transaction == "" || in.Workspace == "" || in.Source == nil || in.QueryOriginal == nil || in.ApplicabilityOriginal == nil || !in.WriteCompleted || in.CompletedKey == "" || (in.Method != "textDocument/references" && in.Method != "textDocument/definition") {
		return fail("missing independently held input")
	}
	if len(in.Claims) != 4 {
		return fail("missing or extra claimant role")
	}
	claims := map[string]B4aClaim{}
	envelopes := map[string]b4aEnvelope{}
	for _, c := range in.Claims {
		if c.Role != "QUERY" && c.Role != "SOURCE" && c.Role != "QUERY_APPLICABILITY" && c.Role != "REQUEST_WRITE" {
			return fail("extra claimant role")
		}
		if _, exists := claims[c.Role]; exists {
			return fail("duplicate claimant role")
		}
		if err := A4Validate(c.Role, c.Envelope); err != nil {
			return fmt.Errorf("B4a %s shape: %w", c.Role, err)
		}
		var e b4aEnvelope
		if err := json.Unmarshal(c.Envelope, &e); err != nil {
			return err
		}
		if e.Role != c.Role || e.Identity.Session != in.Session || e.Identity.Generation != in.Generation || e.Identity.Transaction != in.Transaction {
			return fail("cross-transaction claimant identity")
		}
		if !b4aDescriptor(e.Original, c.Original) {
			return fail(c.Role + " original descriptor")
		}
		claims[c.Role] = c
		envelopes[c.Role] = e
	}
	for _, role := range []string{"SOURCE", "QUERY", "QUERY_APPLICABILITY", "REQUEST_WRITE"} {
		if _, ok := claims[role]; !ok {
			return fail("missing " + role)
		}
	}
	src, q, app, w := claims["SOURCE"], claims["QUERY"], claims["QUERY_APPLICABILITY"], claims["REQUEST_WRITE"]
	tx := b4aTransaction(in)
	gen := strconv.FormatUint(in.Generation, 10)
	sourceArtifact := b4aPreimage("ADR0011-GENERIC-SOURCE-ARTIFACT/1", tx, in.URI, "present", in.Version, strconv.Itoa(len(in.Source)), a4Hash(in.Source), in.Custody)
	if !bytes.Equal(src.Original, in.Source) || !bytes.Equal(src.Artifact, sourceArtifact) || !b4aDeps(envelopes["SOURCE"], map[string]string{}, nil) {
		return fail("independently held SOURCE original")
	}
	sp, e := b4aFields(envelopes["SOURCE"].Payload)
	if e != nil {
		return e
	}
	if b4aText(sp, "uri") != in.URI || b4aText(sp, "version") != in.Version || b4aText(sp, "custody") != in.Custody || !b4aByteField(sp, "source_bytes", in.Source) {
		return fail("SOURCE payload correspondence")
	}
	sourceSelector, err := b4aSelector(in, "SOURCE", sourceArtifact, tx)
	if err != nil {
		return err
	}
	if !bytes.Equal(q.Original, in.QueryOriginal) || !bytes.Equal(q.Artifact, in.QueryOriginal) || !b4aDeps(envelopes["QUERY"], map[string]string{"POLICY": "", "SCHEMA": "", "SOURCE": sourceSelector}, map[string]string{"SOURCE": "sha256:" + a4Hash(in.Source)}) {
		return fail("QUERY original or SOURCE predecessor")
	}
	qp, e := b4aFields(envelopes["QUERY"].Payload)
	if e != nil {
		return e
	}
	line, le := b4aUint(qp, "line")
	character, ce := b4aUint(qp, "character")
	if le != nil || ce != nil || line != in.Line || character != in.Character || b4aText(qp, "uri") != in.URI || b4aText(qp, "version") != in.Version || b4aText(qp, "encoding") != in.Encoding || b4aText(qp, "method") != in.Method || b4aText(qp, "source") != "sha256:"+a4Hash(in.Source) {
		return fail("independently held QUERY fields")
	}
	querySelector, err := b4aSelector(in, "QUERY", in.QueryOriginal, in.URI, in.Version, a4Hash(in.Source), in.Encoding, strconv.FormatUint(in.Line, 10), strconv.FormatUint(in.Character, 10))
	if err != nil {
		return err
	}
	langPresence, lang, kind, ntPresence, nt, nuPresence, nu, ns := "absent", "", "ORDINARY", "absent", "", "absent", "", ""
	if in.Language != nil {
		langPresence = "present"
		lang = *in.Language
	}
	if in.NotebookURI != nil || in.NotebookType != nil {
		if in.NotebookURI == nil || in.NotebookType == nil || in.NotebookSourceSelector == "" {
			return fail("held notebook context")
		}
		kind = "NOTEBOOK_CELL"
		ntPresence = "present"
		nt = *in.NotebookType
		nuPresence = "present"
		nu = *in.NotebookURI
		ns = in.NotebookSourceSelector
	} else if in.NotebookSourceSelector != "" {
		return fail("held ordinary document context")
	}
	appArtifact := b4aPreimage("ADR0011-GENERIC-QUERY-APPLICABILITY-ARTIFACT/1", tx, querySelector, sourceSelector, in.URI, langPresence, lang, kind, ntPresence, nt, nuPresence, nu, ns, in.Workspace, in.Session, gen, in.Transaction)
	if !bytes.Equal(app.Original, in.ApplicabilityOriginal) || !bytes.Equal(app.Artifact, appArtifact) || !bytes.Equal(app.Original, appArtifact) || !b4aDeps(envelopes["QUERY_APPLICABILITY"], map[string]string{"QUERY": querySelector, "SOURCE": sourceSelector}, map[string]string{"QUERY": "sha256:" + a4Hash(in.QueryOriginal), "SOURCE": "sha256:" + a4Hash(in.Source)}) {
		return fail("independently held QUERY_APPLICABILITY original or predecessors")
	}
	ap, e := b4aFields(envelopes["QUERY_APPLICABILITY"].Payload)
	if e != nil {
		return e
	}
	ag, ae := b4aUint(ap, "generation")
	if ae != nil || ag != in.Generation || b4aText(ap, "session") != in.Session || b4aText(ap, "transaction") != in.Transaction || b4aText(ap, "workspace") != in.Workspace || b4aText(ap, "query_selector") != querySelector || b4aText(ap, "query_source_selector") != sourceSelector || !b4aByteField(ap, "uri_bytes", []byte(in.URI)) || !b4aOptional(ap, "language_id_present", "language_id_bytes", in.Language) || b4aText(ap, "document_kind") != kind || !b4aNullOrBytes(ap, "notebook_uri_bytes", in.NotebookURI) || !b4aNullOrBytes(ap, "notebook_type_bytes", in.NotebookType) {
		return fail("QUERY_APPLICABILITY payload correspondence")
	}
	if kind == "ORDINARY" && string(ap["notebook_source_selector"]) != "null" || kind == "NOTEBOOK_CELL" && b4aText(ap, "notebook_source_selector") != ns {
		return fail("notebook source selector")
	}
	appSelector, err := b4aSelector(in, "QUERY_APPLICABILITY", appArtifact, tx)
	if err != nil {
		return err
	}
	if !b4aDeps(envelopes["REQUEST_WRITE"], map[string]string{"QUERY": querySelector, "QUERY_APPLICABILITY": appSelector, "CAPABILITY_EVENTS": "", "POLICY": ""}, map[string]string{"QUERY": "sha256:" + a4Hash(in.QueryOriginal), "QUERY_APPLICABILITY": "sha256:" + a4Hash(in.ApplicabilityOriginal)}) && !b4aDeps(envelopes["REQUEST_WRITE"], map[string]string{"QUERY": querySelector, "QUERY_APPLICABILITY": appSelector, "CAPABILITY_EVENTS": "", "POLICY": "", "PROCESS": ""}, map[string]string{"QUERY": "sha256:" + a4Hash(in.QueryOriginal), "QUERY_APPLICABILITY": "sha256:" + a4Hash(in.ApplicabilityOriginal)}) {
		return fail("WRITE predecessors")
	}
	if !bytes.Equal(w.Original, in.RequestFrame) {
		return fail("independently held completed WRITE frame original")
	}
	wp, e := b4aFields(envelopes["REQUEST_WRITE"].Payload)
	if e != nil {
		return e
	}
	ordinal, oe := b4aUint(wp, "completed_frame_ordinal")
	if oe != nil || ordinal != in.CompletedOrdinal {
		return fail("completed WRITE ordinal")
	}
	var claimantKey string
	if json.Unmarshal(wp["actual_key"], &claimantKey) != nil || claimantKey == "" || claimantKey != in.CompletedKey || string(wp["completed"]) != "true" || !b4aByteField(wp, "frame", in.RequestFrame) || !b4aByteField(wp, "params", in.RequestParams) {
		return fail("completed WRITE payload correspondence")
	}
	request, e := b4aFrame(in.RequestFrame)
	if e != nil {
		return fail("held request frame")
	}
	if string(request["jsonrpc"]) != `"2.0"` || b4aText(request, "method") != in.Method || !bytes.Equal(request["params"], in.RequestParams) {
		return fail("held request method or params")
	}
	// Exact frame correspondence alone does not make a method-bearing message a request.
	if _, hasResult := request["result"]; hasResult {
		return fail("held request shape")
	}
	if rawError, hasError := request["error"]; hasError && string(rawError) != "null" {
		return fail("held request shape")
	}
	id, ie := parseID(request["id"])
	heldID, he := parseID(in.RequestID)
	if ie != nil || he != nil || !id.SameValue(heldID) {
		return fail("held request ID")
	}
	params, e := b4aFields(request["params"])
	if e != nil {
		return fail("held request params")
	}
	doc, e := b4aFields(params["textDocument"])
	if e != nil {
		return fail("held textDocument")
	}
	pos, e := b4aFields(params["position"])
	if e != nil {
		return fail("held position")
	}
	pl, pe := b4aUint(pos, "line")
	pc, pce := b4aUint(pos, "character")
	if b4aText(doc, "uri") != in.URI || pe != nil || pce != nil || pl != in.Line || pc != in.Character {
		return fail("held request QUERY position and URI")
	}
	return nil
}
