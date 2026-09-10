// Package hydratedinspection owns the offline public focused inspection contract.
// Input strings are exact JSON bytes, never host paths or source locators.
package hydratedinspection

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	"lsp-trace/internal/boundedanalysis"
	he "lsp-trace/internal/hydratedevidence"
)

const Version = "lsp-trace.inspect-hydrated.v1"
const SchemaID = "https://jaresty.github.io/lsp-trace/mcp/schemas/output-inspect-hydrated.v1.schema.json"
const InputSchemaID = "https://jaresty.github.io/lsp-trace/mcp/schemas/input-inspect-hydrated.v1.schema.json"

// Public transport caps supplement, never enlarge, the core policy.
const MaxRequestBytes = 4 << 20
const MaxArtifactBytes = 1 << 20
const MaxResponseBytes = 1 << 20

type Request struct {
	Input    string   `json:"input"`
	Sidecars []string `json:"sidecars,omitempty"`
	he.FocusRequest
	Page   bool   `json:"page,omitempty"`
	Cursor string `json:"cursor,omitempty"`
}

type View struct {
	SchemaID      string           `json:"$id"`
	SchemaVersion string           `json:"schema_version"`
	Focus         he.FocusRequest  `json:"focus_request"`
	Manifest      he.FocusManifest `json:"manifest"`
	Request       he.Request       `json:"request"`
	Delivery      string           `json:"delivery"`
	Bundle        *he.Bundle       `json:"bundle,omitempty"`
	Page          *he.Page         `json:"page,omitempty"`
	NextCursor    string           `json:"next_cursor"`
}

type continuation struct {
	Focus string `json:"focus"`
	Core  string `json:"core"`
}

func DefaultRequest() Request {
	f := he.DefaultFocusRequest()
	f.NodeIDs = []string{}
	f.RelationIDs = []string{}
	f.SiblingRelationIDs = []string{}
	f.SidecarRecordIDs = []string{}
	return Request{FocusRequest: f}
}

// Decode performs exact closed schema validation before typed decoding. No loose
// float64 conversion or case-folded field aliases enter the semantic request.
func Decode(raw []byte) (Request, error) {
	r := DefaultRequest()
	if err := ValidateInputJSON(raw); err != nil {
		return r, err
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return r, err
	}
	return r, Check(r)
}

// Check is input-only, including selection policy, before any artifact admission.
// The CLI calls it with a placeholder input before reading explicit files.
func Check(r Request) error {
	if err := he.CheckFocusRequest(r.FocusRequest); err != nil {
		return err
	}
	if r.CorePolicy.AllowCallerBoundaries || r.CorePolicy.IncludeBodies {
		return errors.New("core source flags must remain false; use include_bodies")
	}
	if r.Cursor != "" && !r.Page {
		return errors.New("cursor requires page")
	}
	if len(r.Cursor) > 2048 {
		return errors.New("cursor byte limit")
	}
	if len(r.Sidecars) > 64 {
		return errors.New("sidecar count limit")
	}
	total := len(r.Input)
	for _, s := range r.Sidecars {
		total += len(s)
	}
	if total == 0 || total > MaxArtifactBytes || total > r.CorePolicy.MaxInputBytes {
		return errors.New("combined explicit input byte limit")
	}
	return nil
}
func (r Request) InputBytes() he.Input {
	in := he.Input{Artifact: []byte(r.Input)}
	for _, s := range r.Sidecars {
		in.Sidecars = append(in.Sidecars, []byte(s))
	}
	return in
}
func encodeCursor(manifest, core string) string {
	if core == "" {
		return ""
	}
	raw, _ := json.Marshal(continuation{manifest, core})
	return base64.RawURLEncoding.EncodeToString(raw)
}
func decodeCursor(manifest, token string) (string, error) {
	if token == "" {
		return "", nil
	}
	if len(token) > 2048 {
		return "", errors.New("cursor byte limit")
	}
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return "", errors.New("invalid focus cursor")
	}
	var c continuation
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err = d.Decode(&c); err != nil || c.Focus != manifest || c.Core == "" || encodeCursor(c.Focus, c.Core) != token {
		return "", errors.New("stale or invalid focus cursor")
	}
	return c.Core, nil
}

// Inspect rehydrates statelessly from the same original bytes on every request.
// Both producer outputs are validated; no hidden session/cache is cursor authority.
func Inspect(r Request) (View, error) {
	return inspectWithProducer(r, he.HydrateFocused)
}

func inspectWithProducer(r Request, produce func(he.Input, he.FocusRequest) (he.FocusResult, error)) (View, error) {
	if err := Check(r); err != nil {
		return View{}, err
	}
	input := r.InputBytes()
	result, err := produce(input, r.FocusRequest)
	if err != nil {
		return View{}, err
	}
	if err = he.ValidateFocused(input, r.FocusRequest, result); err != nil {
		return View{}, fmt.Errorf("producer focused validation: %w", err)
	}
	v := View{SchemaID: SchemaID, SchemaVersion: Version, Focus: r.FocusRequest, Manifest: result.Manifest, Request: result.Request, Delivery: "FULL", Bundle: &result.Bundle}
	if r.Page {
		snapshot, err := he.NewSnapshot(input, result.Request, result.Bundle)
		if err != nil {
			return View{}, err
		}
		core, err := decodeCursor(result.Manifest.Digest, r.Cursor)
		if err != nil {
			return View{}, err
		}
		p, err := snapshot.Page(core)
		if err != nil {
			return View{}, err
		}
		v.Bundle = nil
		v.Page = &p
		v.Delivery = "PAGE"
		v.NextCursor = encodeCursor(result.Manifest.Digest, p.Next)
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return View{}, err
	}
	if len(raw)+1 > MaxResponseBytes || len(raw) > r.CorePolicy.MaxOutputBytes {
		return View{}, errors.New("public output policy byte limit")
	}
	if err = ValidateOutputJSON(raw); err != nil {
		return View{}, err
	}
	return v, nil
}

// ValidateFull requires independently retained original input and focus parameters.
// Neither digests nor a presented view authenticate the original acquisition.
func ValidateFull(r Request, v View) error {
	if err := Check(r); err != nil {
		return err
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if len(raw)+1 > MaxResponseBytes || len(raw) > r.CorePolicy.MaxOutputBytes {
		return errors.New("public output policy byte limit")
	}
	if err = ValidateOutputJSON(raw); err != nil {
		return err
	}
	if r.Page || r.Cursor != "" || v.Delivery != "FULL" || v.Bundle == nil || v.Page != nil || v.NextCursor != "" || !reflect.DeepEqual(r.FocusRequest, v.Focus) {
		return errors.New("full view/request mismatch")
	}
	return he.ValidateFocused(r.InputBytes(), r.FocusRequest, he.FocusResult{Manifest: v.Manifest, Request: v.Request, Bundle: *v.Bundle})
}

// Reassemble validates every page's closed public context, exact cursor sequence,
// core canonical packing, and the reconstructed focused manifest. A single page
// is structurally checkable but cannot prove exhaustive focused semantic coverage.
func Reassemble(r Request, views []View) (he.FocusResult, error) {
	if err := Check(r); err != nil {
		return he.FocusResult{}, err
	}
	if !r.Page || r.Cursor != "" || len(views) == 0 || len(views) > r.CorePolicy.MaxPages {
		return he.FocusResult{}, errors.New("invalid complete page request")
	}
	first := views[0]
	pages := make([]he.Page, 0, len(views))
	next := ""
	total := 0
	for _, v := range views {
		raw, err := json.Marshal(v)
		if err != nil {
			return he.FocusResult{}, err
		}
		if len(raw)+1 > MaxResponseBytes || len(raw) > r.CorePolicy.MaxOutputBytes {
			return he.FocusResult{}, errors.New("public output policy byte limit")
		}
		total += len(raw)
		if total > 64<<20 {
			return he.FocusResult{}, errors.New("public aggregate byte limit")
		}
		if err = ValidateOutputJSON(raw); err != nil {
			return he.FocusResult{}, err
		}
		if v.Delivery != "PAGE" || v.Page == nil || v.Bundle != nil || !reflect.DeepEqual(v.Focus, r.FocusRequest) || !reflect.DeepEqual(v.Manifest, first.Manifest) || !reflect.DeepEqual(v.Request, first.Request) || v.NextCursor != encodeCursor(v.Manifest.Digest, v.Page.Next) {
			return he.FocusResult{}, errors.New("page context mismatch")
		}
		if len(pages) > 0 {
			core, err := decodeCursor(v.Manifest.Digest, next)
			if err != nil || core == "" {
				return he.FocusResult{}, errors.New("page cursor mismatch")
			}
		}
		next = v.NextCursor
		pages = append(pages, *v.Page)
	}
	if next != "" {
		return he.FocusResult{}, errors.New("incomplete public pages")
	}
	b, err := he.Reassemble(r.InputBytes(), first.Request, pages)
	if err != nil {
		return he.FocusResult{}, err
	}
	result := he.FocusResult{Manifest: first.Manifest, Request: first.Request, Bundle: b}
	if err = he.ValidateFocused(r.InputBytes(), r.FocusRequest, result); err != nil {
		return he.FocusResult{}, err
	}
	return result, nil
}

// Text is deliberately full-view only: a first page is not a complete human view.
func Text(r Request, v View) (string, error) {
	if err := ValidateFull(r, v); err != nil {
		return "", err
	}
	text, err := he.FocusedText(r.InputBytes(), r.FocusRequest, he.FocusResult{Manifest: v.Manifest, Request: v.Request, Bundle: *v.Bundle})
	if err == nil && len(text) > MaxResponseBytes {
		return "", errors.New("public text output byte limit")
	}
	return text, err
}

func preflight(raw []byte, limit int) error { return boundedanalysis.Preflight(raw, limit) }
