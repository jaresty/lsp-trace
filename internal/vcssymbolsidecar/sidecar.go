package vcssymbolsidecar

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
)

const SchemaVersion = "lsp-trace.vcs-symbol-churn-sidecar.v2"

type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}
type ChangedLine struct {
	Side string `json:"side"`
	Path string `json:"path"`
	Line int    `json:"line"`
}
type Symbol struct {
	Side  string `json:"side,omitempty"`
	Path  string `json:"path"`
	Name  string `json:"name"`
	Kind  int    `json:"kind"`
	Range Range  `json:"range"`
}
type LineAttribution struct {
	Side        string `json:"side"`
	Path        string `json:"path"`
	Line        int    `json:"line"`
	Outcome     string `json:"outcome"`
	SymbolName  string `json:"symbol_name,omitempty"`
	SymbolKind  int    `json:"symbol_kind,omitempty"`
	SymbolRange *Range `json:"symbol_range,omitempty"`
}
type Result struct {
	SchemaVersion         string            `json:"schema_version"`
	GraphArtifactDigest   string            `json:"graph_artifact_digest,omitempty"`
	FromRevision          string            `json:"from_revision,omitempty"`
	ToRevision            string            `json:"to_revision,omitempty"`
	OldAcquisition        []FileOutcome     `json:"old_acquisition,omitempty"`
	NewAcquisition        []FileOutcome     `json:"new_acquisition,omitempty"`
	Authority             int               `json:"authority"`
	SourceGraphComplete   string            `json:"source_graph_complete"`
	Attribution           string            `json:"attribution"`
	CrossRevisionIdentity string            `json:"cross_revision_identity"`
	LineCount             int               `json:"line_count"`
	AttributedLineCount   int               `json:"attributed_line_count"`
	AmbiguousLineCount    int               `json:"ambiguous_line_count"`
	UnmatchedLineCount    int               `json:"unmatched_line_count"`
	Lines                 []LineAttribution `json:"lines"`
}

type BuildRequest struct {
	Repository   string
	FromRevision string
	ToRevision   string
	Paths        []string
	LanguageID   string
}
type DiffSource interface {
	Collect(string, []string, string, string) (DiffCollection, error)
}

func BuildV2(ctx context.Context, raw []byte, req BuildRequest, diffs DiffSource, workspaces WorkspaceProvider, sessions SessionProvider) (Result, error) {
	if len(raw) == 0 || diffs == nil {
		return Result{}, errors.New("symbol sidecar requires exact graph bytes and diff source")
	}
	diff, err := diffs.Collect(req.Repository, req.Paths, req.FromRevision, req.ToRevision)
	if err != nil {
		return Result{}, err
	}
	oldResult, err := AcquireRevision(ctx, RevisionRequest{Repository: req.Repository, Revision: diff.FromRevision, Paths: req.Paths, LanguageID: req.LanguageID}, workspaces, sessions)
	if err != nil {
		return Result{}, err
	}
	newResult, err := AcquireRevision(ctx, RevisionRequest{Repository: req.Repository, Revision: diff.ToRevision, Paths: req.Paths, LanguageID: req.LanguageID}, workspaces, sessions)
	if err != nil {
		return Result{}, err
	}
	if oldResult.Revision != diff.FromRevision || newResult.Revision != diff.ToRevision {
		return Result{}, errors.New("historical symbol revision disagrees with Git diff")
	}
	symbols := []Symbol{}
	appendSide := func(side string, rows []FileOutcome) {
		for _, row := range rows {
			for _, symbol := range row.Symbols {
				symbol.Side = side
				symbols = append(symbols, symbol)
			}
		}
	}
	appendSide("OLD", oldResult.Outcomes)
	appendSide("NEW", newResult.Outcomes)
	result, err := Attribute(diff.Lines, symbols)
	if err != nil {
		return Result{}, err
	}
	sum := sha256.Sum256(raw)
	result.GraphArtifactDigest = fmt.Sprintf("sha256:%x", sum)
	result.FromRevision = diff.FromRevision
	result.ToRevision = diff.ToRevision
	result.OldAcquisition = oldResult.Outcomes
	result.NewAcquisition = newResult.Outcomes
	if err = ValidateComposed(result, len(req.Paths)); err != nil {
		return Result{}, err
	}
	return result, nil
}
func ValidateComposed(r Result, pathCount int) error {
	if err := Validate(r); err != nil {
		return err
	}
	if len(r.GraphArtifactDigest) != 71 || r.GraphArtifactDigest[:7] != "sha256:" || !validSymbolCommit(r.FromRevision) || !validSymbolCommit(r.ToRevision) || len(r.OldAcquisition) != pathCount || len(r.NewAcquisition) != pathCount {
		return errors.New("invalid composed symbol sidecar binding")
	}
	oldPaths, err := validateAcquisition(r.OldAcquisition)
	if err != nil {
		return err
	}
	newPaths, err := validateAcquisition(r.NewAcquisition)
	if err != nil {
		return err
	}
	for i := range oldPaths {
		if oldPaths[i] != newPaths[i] {
			return errors.New("historical acquisition path sets disagree")
		}
	}
	return nil
}

func validateAcquisition(rows []FileOutcome) ([]string, error) {
	paths := make([]string, len(rows))
	for i, row := range rows {
		if row.Path == "" || (i > 0 && rows[i-1].Path >= row.Path) {
			return nil, errors.New("invalid historical acquisition path order")
		}
		paths[i] = row.Path
		switch row.Status {
		case "COMPLETE":
			if row.Error != "" || len(row.Symbols) == 0 {
				return nil, errors.New("invalid complete historical acquisition")
			}
		case "EMPTY":
			if row.Error != "" || len(row.Symbols) != 0 {
				return nil, errors.New("invalid empty historical acquisition")
			}
		case "FAILED":
			if row.Error == "" || len(row.Symbols) != 0 {
				return nil, errors.New("invalid failed historical acquisition")
			}
		default:
			return nil, errors.New("invalid historical acquisition status")
		}
		for _, symbol := range row.Symbols {
			if symbol.Path != row.Path || symbol.Name == "" || symbol.Kind < 1 || symbol.Side != "" || symbol.Range.Start.Line < 0 || symbol.Range.End.Line < symbol.Range.Start.Line {
				return nil, errors.New("invalid historical acquisition symbol")
			}
		}
	}
	return paths, nil
}

func Attribute(changes []ChangedLine, symbols []Symbol) (Result, error) {
	r := Result{SchemaVersion: SchemaVersion, Authority: 0, SourceGraphComplete: "UNKNOWN", Attribution: "HISTORICAL_SYMBOL_RANGE", CrossRevisionIdentity: "NOT_EVALUATED", LineCount: len(changes), Lines: make([]LineAttribution, 0, len(changes))}
	for _, change := range changes {
		if change.Path == "" || change.Line < 0 || (change.Side != "OLD" && change.Side != "NEW") {
			return Result{}, errors.New("invalid changed line")
		}
		matches := make([]Symbol, 0)
		for _, symbol := range symbols {
			if symbol.Path == change.Path && (symbol.Side == "" || symbol.Side == change.Side) && containsLine(symbol.Range, change.Line) {
				matches = append(matches, symbol)
			}
		}
		line := LineAttribution{Side: change.Side, Path: change.Path, Line: change.Line + 1}
		if len(matches) == 0 {
			line.Outcome = "UNMATCHED"
			r.UnmatchedLineCount++
		} else {
			minimal := make([]Symbol, 0, len(matches))
			for i, candidate := range matches {
				containsNarrower := false
				for j, other := range matches {
					if i != j && strictlyContains(candidate.Range, other.Range) {
						containsNarrower = true
						break
					}
				}
				if !containsNarrower {
					minimal = append(minimal, candidate)
				}
			}
			if len(minimal) != 1 {
				line.Outcome = "AMBIGUOUS"
				r.AmbiguousLineCount++
			} else {
				line.Outcome = "ATTRIBUTED"
				line.SymbolName = minimal[0].Name
				line.SymbolKind = minimal[0].Kind
				rr := minimal[0].Range
				line.SymbolRange = &rr
				r.AttributedLineCount++
			}
		}
		r.Lines = append(r.Lines, line)
	}
	if err := Validate(r); err != nil {
		return Result{}, err
	}
	return r, nil
}
func containsLine(r Range, line int) bool {
	return r.Start.Line <= line && (line < r.End.Line || (line == r.End.Line && r.End.Character > 0))
}
func positionLess(a, b Position) bool {
	return a.Line < b.Line || (a.Line == b.Line && a.Character < b.Character)
}
func rangeEqual(a, b Range) bool { return a.Start == b.Start && a.End == b.End }
func strictlyContains(outer, inner Range) bool {
	startsBeforeOrEqual := !positionLess(inner.Start, outer.Start)
	endsAfterOrEqual := !positionLess(outer.End, inner.End)
	return startsBeforeOrEqual && endsAfterOrEqual && !rangeEqual(outer, inner)
}
func Validate(r Result) error {
	if r.SchemaVersion != SchemaVersion || r.Authority != 0 || r.SourceGraphComplete != "UNKNOWN" || r.Attribution != "HISTORICAL_SYMBOL_RANGE" || r.CrossRevisionIdentity != "NOT_EVALUATED" {
		return errors.New("invalid symbol sidecar claim ceiling")
	}
	if r.LineCount != len(r.Lines) || r.LineCount != r.AttributedLineCount+r.AmbiguousLineCount+r.UnmatchedLineCount {
		return errors.New("invalid symbol sidecar accounting")
	}
	a, u, m := 0, 0, 0
	for _, line := range r.Lines {
		if line.Path == "" || line.Line < 1 || (line.Side != "OLD" && line.Side != "NEW") {
			return errors.New("invalid line attribution")
		}
		switch line.Outcome {
		case "ATTRIBUTED":
			if line.SymbolName == "" || line.SymbolRange == nil {
				return errors.New("incomplete attributed line")
			}
			a++
		case "AMBIGUOUS":
			m++
		case "UNMATCHED":
			u++
		default:
			return errors.New("invalid line outcome")
		}
	}
	if a != r.AttributedLineCount || m != r.AmbiguousLineCount || u != r.UnmatchedLineCount {
		return errors.New("symbol sidecar counts do not match rows")
	}
	return nil
}
