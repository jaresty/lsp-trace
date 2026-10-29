package privatedescribeprep

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"unicode/utf8"

	pilot "lsp-trace/internal/censuslineagepilot"
	"lsp-trace/internal/graph"
	"lsp-trace/internal/sourceposition"
	"lsp-trace/internal/sourceprojection"
)

type Declaration struct {
	Kind, Encoding                                 string
	StartByte, EndByte, NameStartByte, NameEndByte int
	NativeRange, SelectionRange, DeclarationRange  graph.Range
	Bytes                                          []byte
	Body                                           string
	Comments                                       string
}

// positionOffset never treats protocol character units as byte columns. Reject
// split UTF-8 runes, split surrogate pairs, absent lines and columns past EOL.
func positionOffset(src []byte, p graph.Position, encoding string) (int, bool) {
	offset, err := sourceposition.Offset(src, encoding, sourceposition.Position{Line: p.Line, Character: p.Character})
	return offset, err == nil
}
func receiverName(x ast.Expr) string {
	switch v := x.(type) {
	case *ast.Ident:
		return v.Name
	case *ast.StarExpr:
		return "*" + receiverName(v.X)
	case *ast.IndexExpr:
		return receiverName(v.X)
	case *ast.IndexListExpr:
		return receiverName(v.X)
	}
	return ""
}
func declaration(src []byte, n graph.Node, encoding string) (Declaration, string) {
	fail := func(s string) (Declaration, string) { return Declaration{}, s }
	if !utf8.Valid(src) {
		return fail("INVALID_SOURCE_UTF8")
	}
	if encoding != "utf-8" && encoding != "utf-16" && encoding != "utf-32" {
		return fail("UNAVAILABLE_POSITION_ENCODING")
	}
	if n.Kind != 12 && n.Kind != 6 {
		return fail("UNSUPPORTED_DECLARATION_KIND")
	}
	start, ok := positionOffset(src, n.SelectionRange.Start, encoding)
	if !ok {
		return fail("INVALID_SELECTION_POSITION")
	}
	end, ok := positionOffset(src, n.SelectionRange.End, encoding)
	if !ok || end <= start {
		return fail("INVALID_SELECTION_RANGE")
	}
	rstart, ok := positionOffset(src, n.Range.Start, encoding)
	if !ok {
		return fail("INVALID_ENDPOINT_RANGE")
	}
	rend, ok := positionOffset(src, n.Range.End, encoding)
	if !ok || rend < rstart || start < rstart || end > rend {
		return fail("INVALID_ENDPOINT_RANGE")
	}
	fs := token.NewFileSet()
	f, err := parser.ParseFile(fs, "retained.go", src, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return fail("UNPARSEABLE_RETAINED_GO_SOURCE")
	}
	var matches []*ast.FuncDecl
	for _, d := range f.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok {
			continue
		}
		ns, ne := fs.PositionFor(fn.Name.Pos(), false).Offset, fs.PositionFor(fn.Name.End(), false).Offset
		// Native selection must start at the identifier and stay within it. Some
		// providers select a prefix; it is not widened into a native FULL_DEFINITION.
		if ns != start || end > ne {
			continue
		}
		if fn.Recv == nil {
			if n.Kind != 12 || n.Name != fn.Name.Name {
				continue
			}
		} else {
			if n.Kind != 6 || len(fn.Recv.List) != 1 {
				continue
			}
			recv := receiverName(fn.Recv.List[0].Type)
			if recv == "" {
				continue
			}
			expected := "(" + recv + ")." + fn.Name.Name
			plain := recv + "." + fn.Name.Name
			if n.Name != expected && n.Name != plain {
				continue
			}
		}
		matches = append(matches, fn)
	}
	if len(matches) == 0 {
		return fail("MISSING_SOURCE_BINDING")
	}
	if len(matches) != 1 {
		return fail("AMBIGUOUS_SOURCE_BINDING")
	}
	fn := matches[0]
	s, e := fs.PositionFor(fn.Pos(), false).Offset, fs.PositionFor(fn.End(), false).Offset
	if s < rstart || e > rend || s < 0 || e > len(src) {
		return fail("DECLARATION_OUTSIDE_ENDPOINT_RANGE")
	}
	sp, err := sourceposition.PositionAtOffset(src, encoding, s)
	if err != nil {
		return fail("INVALID_DECLARATION_POSITION")
	}
	ep, err := sourceposition.PositionAtOffset(src, encoding, e)
	if err != nil {
		return fail("INVALID_DECLARATION_POSITION")
	}
	r := sourceprojection.Range{Start: sourceprojection.Position{Line: sp.Line, Character: sp.Character}, End: sourceprojection.Position{Line: ep.Line, Character: ep.Character}}
	// Reuse the same neutral byte/range assembly core as retainedprojection.
	// AST provenance is explicit; no FULL_DEFINITION/documentSymbol/receipt claim.
	const kind = "RETAINED_SOURCE_AST_DECLARATION"
	unitID := kind + ":" + n.ID
	projected, err := sourceprojection.Project([]sourceprojection.Candidate{{UnitID: unitID, CitationID: unitID + ":citation", Role: "ENDPOINT", GraphSubjectID: n.ID, LogicalSourceID: n.URI, Range: r, DisplayProvenance: kind, PositionEncoding: encoding, PrivacyClassification: "PRIVATE_RETAINED_POLICY"}}, map[string]sourceprojection.Source{n.URI: {LogicalSourceID: n.URI, Digest: pilot.Digest(src), ByteLength: len(src), Bytes: src, Available: true}}, sourceprojection.Policy{PolicyID: RequestType, BodyRequested: true, MaxBytes: len(src), MaxRanges: 1, MaxObjects: 1, MaxWork: 1, EnforceLimits: true})
	if err != nil || len(projected.Units) != 1 || len(projected.EmittedSpans) != 1 || projected.Units[0].BodyDisposition != "RETURNED" || !bytes.Equal([]byte(projected.Units[0].Body), src[s:e]) {
		return fail("NEUTRAL_DECLARATION_ASSEMBLY_GAP")
	}
	return Declaration{Kind: kind, Encoding: encoding, StartByte: s, EndByte: e, NameStartByte: start, NameEndByte: fs.PositionFor(fn.Name.End(), false).Offset, NativeRange: n.Range, SelectionRange: n.SelectionRange, DeclarationRange: graph.Range{Start: graph.Position{Line: sp.Line, Character: sp.Character}, End: graph.Position{Line: ep.Line, Character: ep.Character}}, Bytes: []byte(projected.Units[0].Body), Body: projected.Units[0].Body, Comments: "FuncDecl.Pos..End; leading documentation excluded"}, ""
}
