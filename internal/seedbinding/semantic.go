package seedbinding

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

type lspPosition struct {
	Line      uint32 `json:"line"`
	Character uint32 `json:"character"`
}
type lspRange struct {
	Start lspPosition `json:"start"`
	End   lspPosition `json:"end"`
}
type documentSymbol struct {
	Name           string           `json:"name"`
	Detail         string           `json:"detail,omitempty"`
	Kind           int              `json:"kind"`
	Tags           []int            `json:"tags,omitempty"`
	Deprecated     bool             `json:"deprecated,omitempty"`
	Range          lspRange         `json:"range"`
	SelectionRange lspRange         `json:"selectionRange"`
	Children       []documentSymbol `json:"children,omitempty"`
}
type symbolLocation struct {
	URI   string   `json:"uri"`
	Range lspRange `json:"range"`
}
type symbolInformation struct {
	Name          string         `json:"name"`
	Kind          int            `json:"kind"`
	Tags          []int          `json:"tags,omitempty"`
	Deprecated    bool           `json:"deprecated,omitempty"`
	Location      symbolLocation `json:"location"`
	ContainerName string         `json:"containerName,omitempty"`
}

// ValidateDocumentSymbols admits exactly one declaration from the strict LSP
// array-level union. Flat SymbolInformation is qualified because LSP exposes no
// distinct name range in that representation.
func ValidateDocumentSymbols(raw []byte, manifest Manifest, source []byte, negotiatedEncoding string) ValidationResult {
	if negotiatedEncoding != manifest.Locator.Encoding {
		return ValidationResult{Status: Unavailable, PrivateDetail: "negotiated position encoding unavailable"}
	}
	var entries []json.RawMessage
	if err := decodeOne(raw, &entries); err != nil || entries == nil {
		return ValidationResult{Status: Invalid, PrivateDetail: "documentSymbol result invalid"}
	}
	shape := ""
	matches := 0
	flat := false
	var visit func(documentSymbol)
	visit = func(s documentSymbol) {
		if s.Name == manifest.ExpectedSymbol && validLSPRange(source, s.SelectionRange, negotiatedEncoding) && validLSPRange(source, s.Range, negotiatedEncoding) && containsLSP(s.SelectionRange, manifest.Locator) && containsLSP(s.Range, manifest.Locator) && equalManifestRange(s.Range, manifest.ExpectedDeclarationRange) {
			matches++
		}
		for _, child := range s.Children {
			visit(child)
		}
	}
	for _, entry := range entries {
		var discriminator struct {
			Location json.RawMessage `json:"location"`
		}
		if err := json.Unmarshal(entry, &discriminator); err != nil {
			return ValidationResult{Status: Invalid, PrivateDetail: "documentSymbol member invalid"}
		}
		memberShape := "hierarchical"
		if len(discriminator.Location) != 0 {
			memberShape = "flat"
		}
		if shape != "" && shape != memberShape {
			return ValidationResult{Status: Invalid, PrivateDetail: "mixed documentSymbol result forms"}
		}
		shape = memberShape
		if memberShape == "hierarchical" {
			var symbol documentSymbol
			if err := decodeOne(entry, &symbol); err != nil {
				return ValidationResult{Status: Invalid, PrivateDetail: "DocumentSymbol invalid"}
			}
			visit(symbol)
		} else {
			var symbol symbolInformation
			if err := decodeOne(entry, &symbol); err != nil {
				return ValidationResult{Status: Invalid, PrivateDetail: "SymbolInformation invalid"}
			}
			if symbol.Name == manifest.ExpectedSymbol && symbol.Location.URI == manifest.Locator.URI && validLSPRange(source, symbol.Location.Range, negotiatedEncoding) && containsLSP(symbol.Location.Range, manifest.Locator) && equalManifestRange(symbol.Location.Range, manifest.ExpectedDeclarationRange) {
				matches++
			}
			flat = true
		}
	}
	if matches != 1 {
		return ValidationResult{Status: Mismatch, PrivateDetail: fmt.Sprintf("semantic declaration matches=%d", matches)}
	}
	detail := "hierarchical exact name/selection/declaration range"
	if flat {
		detail = "flat SymbolInformation exact name/location range; distinct name range unavailable"
	}
	return ValidationResult{Status: Match, PrivateDetail: detail}
}

func decodeOne(raw []byte, value any) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(value); err != nil {
		return err
	}
	if err := d.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("one JSON value required")
	}
	return nil
}
func validLSPRange(source []byte, r lspRange, encoding string) bool {
	return comparePosition(r.Start.Line, r.Start.Character, r.End.Line, r.End.Character) <= 0 && validPosition(source, r.Start.Line, r.Start.Character, encoding) && validPosition(source, r.End.Line, r.End.Character, encoding)
}
func containsLSP(r lspRange, l Locator) bool {
	return comparePosition(r.Start.Line, r.Start.Character, l.Line, l.Character) <= 0 && comparePosition(l.Line, l.Character, r.End.Line, r.End.Character) <= 0
}
func equalManifestRange(r lspRange, expected Range) bool {
	return r.Start.Line == expected.StartLine && r.Start.Character == expected.StartCharacter && r.End.Line == expected.EndLine && r.End.Character == expected.EndCharacter
}
