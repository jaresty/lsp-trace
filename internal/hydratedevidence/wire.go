package hydratedevidence

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
	"strings"
	"sync"
)

//go:embed schema.json
var ownedSchema []byte
var schemaOnce sync.Once
var compiledSchema *jsonschema.Schema
var schemaError error

func Schema() []byte { return append([]byte(nil), ownedSchema...) }
func structure(raw []byte) error {
	if err := preflight(raw); err != nil {
		return err
	}
	schemaOnce.Do(func() {
		doc, e := jsonschema.UnmarshalJSON(bytes.NewReader(ownedSchema))
		if e != nil {
			schemaError = e
			return
		}
		c := jsonschema.NewCompiler()
		const uri = "urn:lsp-trace:hydrated-evidence:internal:v1"
		if e = c.AddResource(uri, doc); e != nil {
			schemaError = e
			return
		}
		compiledSchema, schemaError = c.Compile(uri)
	})
	if schemaError != nil {
		return schemaError
	}
	doc, e := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if e != nil {
		return e
	}
	return compiledSchema.Validate(doc)
}
func ValidateJSON(input Input, r Request, raw []byte) error {
	if e := checkRequest(r); e != nil {
		return e
	}
	if len(raw) > r.Policy.MaxOutputBytes {
		return errors.New("output byte budget")
	}
	if e := structure(raw); e != nil {
		return e
	}
	var b Bundle
	if e := decode(raw, &b); e != nil {
		return e
	}
	return Validate(input, r, b)
}

// Text is an escaped, validated human view; it shares the exact opt-in policy.
func Text(input Input, r Request, b Bundle) (string, error) {
	if e := Validate(input, r, b); e != nil {
		return "", e
	}
	return text(r, b, nil)
}

func text(r Request, b Bundle, selectedSources map[string]bool) (string, error) {
	var out strings.Builder
	fmt.Fprintf(&out, "Retained context %s\nOrigins: %d; spans: %d; requested context complete: %t\nNo analyzed-version, source-completeness or independent-support claim.\n", b.Digest, b.TotalOrigins, b.TotalSpans, b.Complete)
	optional := func(s *string) string {
		if s == nil {
			return "UNAVAILABLE"
		}
		return *s
	}
	for _, s := range b.Sources {
		if selectedSources != nil && !selectedSources[s.ID] {
			continue
		}
		fmt.Fprintf(&out, "Source %q uri=%q receipt=%q version=%q revision=%q hash=%q state=%s authority=%s/%s acquisition=%s analyzed=%s\n", s.ID, s.URI, s.ReceiptReference, s.VersionReference, optional(s.Revision), optional(s.ContentHash), s.State, s.Authority, s.Qualification, s.Classification, s.AnalyzedVersion)
		if out.Len() > r.Policy.MaxOutputBytes {
			return "", errors.New("text output byte budget")
		}
	}
	for _, o := range b.Origins {
		authority, qualification := "UNAVAILABLE", "UNAVAILABLE"
		if o.Record != nil {
			authority = o.Record.Authority
			qualification = o.Record.Qualification
		}
		fmt.Fprintf(&out, "Origin %q record=%q source=%q disposition=%s authority=%s/%s original_coordinates=%s coordinate_authority=%s range=%v bytes=%v spans=%q\n", o.Selection.ID, o.Selection.RecordID, o.Selection.SourceID, o.Status, authority, qualification, o.OriginalCoordinatesStatus, o.CoordinateAuthority, o.OriginalRange, o.Bytes, o.SpanIDs)
		if o.Record != nil {
			fmt.Fprintf(&out, "  Retained record native_id=%q pointer=%q kind=%q anchor_status=%q retained_position_encoding=%q\n", o.Record.NativeID, o.Record.Pointer, o.Record.Kind, o.Record.AnchorStatus, o.Record.Encoding)
			fmt.Fprintf(&out, "  Accompanying relationship references (not independent support): %q\n", o.Record.RelationshipReferences)
		}
		if out.Len() > r.Policy.MaxOutputBytes {
			return "", errors.New("text output byte budget")
		}
	}
	for _, s := range b.Spans {
		fmt.Fprintf(&out, "Span %q source=%q bytes=[%d,%d) encoding=%s origins=%q content=%q\n", s.ID, s.SourceID, s.Bytes.Start, s.Bytes.End, s.Encoding, s.OriginIDs, string(s.Content))
		if out.Len() > r.Policy.MaxOutputBytes {
			return "", errors.New("text output byte budget")
		}
	}
	return out.String(), nil
}
