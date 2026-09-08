// Package graphprovenance binds unchanged graph bytes to bounded historical
// observations. It never authenticates analyzed source or server consumption.
package graphprovenance

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"unicode/utf8"

	"lsp-trace/internal/custodyevidence"
	"lsp-trace/internal/schema"
	"lsp-trace/internal/source"
	"lsp-trace/internal/strictjson"
	"lsp-trace/sessionruntime"
)

const Version = "lsp-trace.graph-provenance.v1"
const Family = "graph-provenance"
const Unverified = "ANALYZED_VERSION_UNVERIFIED"
const PostTraversal = "POST_TRAVERSAL_CAPTURE"
const Supplied = "LSP_SUPPLIED"
const MaxGraphBytes = 8 << 20
const MaxFileBytes = 1 << 20
const MaxTotalBytes = 4 << 20
const MaxFiles = 64
const MaxEnvelopeBytes = 48 << 20

type SupplyMetadata struct {
	SessionID  string          `json:"session_id"`
	Generation uint64          `json:"generation"`
	Version    int             `json:"document_version"`
	Method     string          `json:"method"`
	Params     json.RawMessage `json:"params"`
}
type Receipt struct {
	ID               string          `json:"id"`
	URI              string          `json:"uri"`
	Path             string          `json:"path"`
	Classification   string          `json:"classification"`
	AnalyzedVersion  string          `json:"analyzed_version"`
	Status           string          `json:"status"`
	Content          []byte          `json:"content"`
	CanonicalReceipt []byte          `json:"canonical_receipt"`
	Supply           *SupplyMetadata `json:"supply,omitempty"`
}
type Binding struct {
	Pointer     string   `json:"pointer"`
	URI         string   `json:"uri"`
	Attribution string   `json:"attribution"`
	ReceiptIDs  []string `json:"receipt_ids"`
}
type Evidence struct {
	SchemaVersion          string    `json:"schema_version"`
	GraphBytes             []byte    `json:"graph_bytes"`
	GraphDigest            string    `json:"graph_digest"`
	WorkspaceURI           string    `json:"workspace_uri"`
	SeedURI                string    `json:"seed_uri"`
	SessionID              string    `json:"session_id"`
	Generation             uint64    `json:"generation"`
	AnalyzedVersion        string    `json:"analyzed_version"`
	DependencyCompleteness string    `json:"dependency_completeness"`
	SupplyStatus           string    `json:"supply_status"`
	Supply                 *Receipt  `json:"supply,omitempty"`
	Captures               []Receipt `json:"captures"`
	Bindings               []Binding `json:"bindings"`
}

func digest(domain string, raw []byte) string {
	return fmt.Sprintf("sha256:%x", sha256.Sum256(append(append([]byte(domain), 0), raw...)))
}
func seal(r Receipt) Receipt {
	r.ID = ""
	raw, _ := json.Marshal(r)
	r.ID = digest(Version+":receipt", raw)
	return r
}
func receipt(uri, name, class, status string, content []byte, supply *SupplyMetadata) Receipt {
	a := source.Acquisition{Status: source.Readable, Provenance: source.Provenance{Mechanism: class + "/" + status, Locator: uri}}
	if status != "READABLE" {
		a.Status = source.Unreadable
		a.Failure = &source.AcquisitionFailure{Reason: status}
		content = nil
	}
	_, content, canonical, _ := source.CanonicalizeReceipt(source.DiscoveredItem{ID: uri, Locator: uri}, a, content)
	return seal(Receipt{URI: uri, Path: name, Classification: class, AnalyzedVersion: Unverified, Status: status, Content: content, CanonicalReceipt: canonical, Supply: supply})
}

// RelativeURI performs lexical canonicalization only; actual opens are contained
// by the host's os.Root handle. Validation never opens these paths offline.
func RelativeURI(workspaceURI, uri string) (string, string) {
	u, err := url.Parse(uri)
	if err != nil || !u.IsAbs() {
		return "", "INVALID_URI"
	}
	if u.Scheme != "file" {
		return "", "VIRTUAL_URI"
	}
	if u.Host != "" || !path.IsAbs(u.Path) || path.Clean(u.Path) != u.Path || strings.ContainsAny(u.Path, "\\\x00:") || (&url.URL{Scheme: "file", Path: u.Path}).String() != uri {
		return "", "INVALID_URI"
	}
	root, err := url.Parse(workspaceURI)
	if err != nil {
		return "", "INVALID_URI"
	}
	prefix := strings.TrimSuffix(root.Path, "/") + "/"
	if !strings.HasPrefix(u.Path, prefix) {
		return "", "OUTSIDE_SCOPE"
	}
	name := strings.TrimPrefix(u.Path, prefix)
	if name == "" {
		return "", "OUTSIDE_SCOPE"
	}
	return name, ""
}

// Capture owns only graph-referenced reads through the host-selected workspace.
// A changed file is captured as a later observation, never a traversal snapshot.
func Capture(ctx context.Context, raw []byte, workspace, seedURI, sessionID string, generation uint64, supplied *sessionruntime.DocumentSupply) ([]byte, error) {
	if !filepath.IsAbs(workspace) || filepath.Clean(workspace) != workspace {
		return nil, errors.New("host workspace must be canonical absolute")
	}
	bindings, err := Census(raw)
	if err != nil {
		return nil, err
	}
	e := Evidence{SchemaVersion: Version, GraphBytes: append([]byte(nil), raw...), GraphDigest: digest(Version+":graph", raw), WorkspaceURI: (&url.URL{Scheme: "file", Path: filepath.ToSlash(workspace)}).String(), SeedURI: seedURI, SessionID: sessionID, Generation: generation, AnalyzedVersion: Unverified, DependencyCompleteness: "UNKNOWN_INCOMPLETE", SupplyStatus: "NO_NOTIFICATION_OBSERVATION", Captures: []Receipt{}, Bindings: bindings}
	if supplied != nil {
		name, status := RelativeURI(e.WorkspaceURI, supplied.URI)
		if status != "" || supplied.Classification != Supplied || supplied.URI != seedURI || supplied.SessionID != sessionID || supplied.Generation != generation {
			return nil, errors.New("supply context mismatch")
		}
		r := receipt(supplied.URI, name, Supplied, "READABLE", supplied.Content, &SupplyMetadata{SessionID: sessionID, Generation: generation, Version: supplied.DocumentVersion, Method: supplied.Method, Params: append(json.RawMessage(nil), supplied.Params...)})
		e.Supply = &r
		e.SupplyStatus = "OBSERVED_NOTIFICATION"
	}
	root, rootErr := os.OpenRoot(workspace)
	if rootErr == nil {
		defer root.Close()
	}
	uris := sourceURIs(bindings)
	used, attempts := 0, 0
	for _, uri := range uris {
		name, status := RelativeURI(e.WorkspaceURI, uri)
		var content []byte
		if status == "" {
			switch {
			case ctx.Err() != nil:
				status = "CANCELLED"
			case attempts >= MaxFiles || used >= MaxTotalBytes:
				status = "BUDGET_EXCEEDED"
			case rootErr != nil:
				status = "UNREADABLE"
			default:
				attempts++
				limit := MaxFileBytes
				if left := MaxTotalBytes - used; left < limit {
					limit = left
				}
				content, err = source.ReadRegularInputBounded(root, name, int64(limit))
				// Successful reads charge actual bytes; failed reads conservatively
				// charge the whole bound, so oversized files cannot bypass the budget.
				if err != nil {
					used += limit
					status = "UNREADABLE"
					if err.Error() == "input exceeds byte limit" {
						status = "BYTE_LIMIT_EXCEEDED"
					}
				} else {
					used += len(content)
					status = "READABLE"
				}
			}
		}
		e.Captures = append(e.Captures, receipt(uri, name, PostTraversal, status, content, nil))
	}
	bind(&e)
	encoded, err := json.Marshal(e)
	if err != nil {
		return nil, err
	}
	if _, err = ValidateFor(encoded, Family, "v1"); err != nil {
		return nil, err
	}
	return append(encoded, '\n'), nil
}
func sourceURIs(bindings []Binding) []string {
	seen := map[string]bool{}
	for _, b := range bindings {
		if b.Attribution == "SOURCE" {
			seen[b.URI] = true
		}
	}
	uris := make([]string, 0, len(seen))
	for uri := range seen {
		uris = append(uris, uri)
	}
	sort.Strings(uris)
	return uris
}
func bind(e *Evidence) {
	ids := map[string]string{}
	for _, r := range e.Captures {
		ids[r.URI] = r.ID
	}
	for i := range e.Bindings {
		b := &e.Bindings[i]
		b.ReceiptIDs = []string{}
		if b.Attribution == "SOURCE" {
			b.ReceiptIDs = append(b.ReceiptIDs, ids[b.URI])
			if e.Supply != nil && e.Supply.URI == b.URI {
				b.ReceiptIDs = append(b.ReceiptIDs, e.Supply.ID)
			}
			sort.Strings(b.ReceiptIDs)
		}
	}
}

// ValidateFor composes legacy validators with this independently versioned
// family. Consistency is not reauthentication of public evidence structs.
func ValidateFor(data []byte, family, version string) (string, error) {
	if family != Family {
		return custodyevidence.ValidateFor(data, family, version)
	}
	if version == "v2" || version == VersionV2 {
		return validateForV2(data)
	}
	if version == "v3" || version == VersionV3 {
		return validateForV3(data)
	}
	if len(data) > MaxEnvelopeBytes {
		return "", errors.New("provenance envelope byte limit")
	}
	if err := strictjson.RejectDuplicates(data); err != nil {
		return "", err
	}
	structural, err := schema.ValidateStructure(data, family, version)
	if err != nil {
		return "", err
	}
	var e Evidence
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err = d.Decode(&e); err != nil {
		return "", err
	}
	if err = Validate(e); err != nil {
		return "", err
	}
	return structural.Version, nil
}
func Validate(e Evidence) error {
	if e.SchemaVersion != Version || e.AnalyzedVersion != Unverified || e.DependencyCompleteness != "UNKNOWN_INCOMPLETE" || e.SessionID == "" || e.Generation == 0 {
		return errors.New("invalid provenance ceiling or context")
	}
	root, err := url.Parse(e.WorkspaceURI)
	if err != nil || root.Scheme != "file" || root.Host != "" || !path.IsAbs(root.Path) || path.Clean(root.Path) != root.Path || (&url.URL{Scheme: "file", Path: root.Path}).String() != e.WorkspaceURI {
		return errors.New("invalid workspace URI")
	}
	expected, err := Census(e.GraphBytes)
	if err != nil {
		return err
	}
	if e.GraphDigest != digest(Version+":graph", e.GraphBytes) {
		return errors.New("graph digest mismatch")
	}
	var doc struct {
		Invocation struct {
			Target struct {
				URI string `json:"uri"`
			} `json:"target"`
		} `json:"invocation"`
	}
	_ = json.Unmarshal(e.GraphBytes, &doc)
	if e.SeedURI != doc.Invocation.Target.URI {
		return errors.New("seed URI mismatch")
	}
	if e.Supply == nil {
		if e.SupplyStatus != "NO_NOTIFICATION_OBSERVATION" {
			return errors.New("missing supply observation")
		}
	} else {
		r := e.Supply
		if e.SupplyStatus != "OBSERVED_NOTIFICATION" || r.Classification != Supplied || r.Status != "READABLE" || r.URI != e.SeedURI || r.Supply == nil {
			return errors.New("invalid supplied classification")
		}
		s := r.Supply
		if s.SessionID != e.SessionID || s.Generation != e.Generation || len(r.Content) > MaxFileBytes || !utf8.Valid(r.Content) {
			return errors.New("invalid supplied context/text")
		}
		if err = validateSupply(r); err != nil {
			return err
		}
		if err = validateReceipt(e.WorkspaceURI, *r); err != nil {
			return err
		}
	}
	uris := sourceURIs(expected)
	if len(uris) != len(e.Captures) {
		return errors.New("missing or extra graph source receipt")
	}
	total, readable := 0, 0
	for i, r := range e.Captures {
		if r.URI != uris[i] || r.Classification != PostTraversal || r.Supply != nil {
			return errors.New("invalid post-traversal receipt or order")
		}
		if err = validateReceipt(e.WorkspaceURI, r); err != nil {
			return err
		}
		total += len(r.Content)
		if r.Status == "READABLE" {
			readable++
		}
	}
	if readable > MaxFiles {
		return errors.New("captured file budget exceeded")
	}
	if total > MaxTotalBytes {
		return errors.New("captured content budget exceeded")
	}
	rebuilt := e
	rebuilt.Bindings = expected
	bind(&rebuilt)
	if !reflect.DeepEqual(rebuilt.Bindings, e.Bindings) {
		return errors.New("graph-derived binding census mismatch")
	}
	return nil
}
func validateReceipt(workspace string, r Receipt) error {
	name, status := RelativeURI(workspace, r.URI)
	if r.AnalyzedVersion != Unverified || r.Path != name || len(r.Content) > MaxFileBytes {
		return errors.New("receipt scope/ceiling/limit mismatch")
	}
	if status != "" {
		if r.Status != status {
			return errors.New("URI disposition mismatch")
		}
	} else {
		switch r.Status {
		case "READABLE", "UNREADABLE", "CANCELLED", "BUDGET_EXCEEDED", "BYTE_LIMIT_EXCEEDED":
		default:
			return errors.New("unknown capture outcome")
		}
	}
	if r.Status != "READABLE" && r.Content != nil {
		return errors.New("failed receipt contains bytes")
	}
	rebuilt := receipt(r.URI, r.Path, r.Classification, r.Status, r.Content, r.Supply)
	if !reflect.DeepEqual(rebuilt, r) {
		return errors.New("receipt hash/canonical bytes mismatch")
	}
	return nil
}
func validateSupply(r *Receipt) error {
	s := r.Supply
	if err := strictjson.RejectDuplicates(s.Params); err != nil {
		return err
	}
	// Exact-key and full-text validation also rejects range-based didChange and
	// post-capture reclassification without a coherent notification observation.
	var expected any
	if s.Method == "textDocument/didOpen" && s.Version == 1 {
		var p struct {
			TextDocument struct {
				LanguageID string `json:"languageId"`
			} `json:"textDocument"`
		}
		if json.Unmarshal(s.Params, &p) != nil || p.TextDocument.LanguageID == "" {
			return errors.New("missing supplied language")
		}
		expected = map[string]any{"textDocument": map[string]any{"uri": r.URI, "languageId": p.TextDocument.LanguageID, "version": s.Version, "text": string(r.Content)}}
	} else if s.Method == "textDocument/didChange" && s.Version > 1 {
		expected = map[string]any{"textDocument": map[string]any{"uri": r.URI, "version": s.Version}, "contentChanges": []any{map[string]any{"text": string(r.Content)}}}
	} else {
		return errors.New("invalid supply notification/version")
	}
	raw, _ := json.Marshal(expected)
	var want, got any
	_ = json.Unmarshal(raw, &want)
	if json.Unmarshal(s.Params, &got) != nil || !reflect.DeepEqual(want, got) {
		return errors.New("notification parameters do not match supplied bytes")
	}
	return nil
}
