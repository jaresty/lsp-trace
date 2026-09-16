package vcssymbolsidecar

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"math"
	"math/bits"
	"sort"
)

const (
	SchemaVersion   = "lsp-trace.vcs-symbol-churn-sidecar.v2"
	SchemaVersionV3 = "lsp-trace.vcs-symbol-churn-sidecar.v3"
)

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

type SymbolMetric struct {
	Path                    string `json:"path"`
	Name                    string `json:"name"`
	Kind                    int    `json:"kind"`
	Range                   Range  `json:"range"`
	ChangedLineCount        int    `json:"changed_line_count"`
	SymbolSpanLineCount     int    `json:"symbol_span_line_count"`
	ChurnDensityBasisPoints int    `json:"churn_density_basis_points"`
}

type ResultV3 struct {
	Result
	HistoricalSymbolMetrics []SymbolMetric `json:"historical_symbol_metrics"`
	CurrentSymbolMetrics    []SymbolMetric `json:"current_symbol_metrics"`
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

type acquiredRevisions struct {
	diff    DiffCollection
	old     RevisionSymbols
	current RevisionSymbols
	symbols []Symbol
}

func acquireRevisions(ctx context.Context, raw []byte, req BuildRequest, diffs DiffSource, workspaces WorkspaceProvider, sessions SessionProvider) (acquiredRevisions, error) {
	if len(raw) == 0 || diffs == nil {
		return acquiredRevisions{}, errors.New("symbol sidecar requires exact graph bytes and diff source")
	}
	diff, err := diffs.Collect(req.Repository, req.Paths, req.FromRevision, req.ToRevision)
	if err != nil {
		return acquiredRevisions{}, err
	}
	oldResult, err := AcquireRevision(ctx, RevisionRequest{Repository: req.Repository, Revision: diff.FromRevision, Paths: req.Paths, LanguageID: req.LanguageID}, workspaces, sessions)
	if err != nil {
		return acquiredRevisions{}, err
	}
	newResult, err := AcquireRevision(ctx, RevisionRequest{Repository: req.Repository, Revision: diff.ToRevision, Paths: req.Paths, LanguageID: req.LanguageID}, workspaces, sessions)
	if err != nil {
		return acquiredRevisions{}, err
	}
	if oldResult.Revision != diff.FromRevision || newResult.Revision != diff.ToRevision {
		return acquiredRevisions{}, errors.New("historical symbol revision disagrees with Git diff")
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
	return acquiredRevisions{diff: diff, old: oldResult, current: newResult, symbols: symbols}, nil
}

func bindResult(result *Result, raw []byte, acquired acquiredRevisions) {
	sum := sha256.Sum256(raw)
	result.GraphArtifactDigest = fmt.Sprintf("sha256:%x", sum)
	result.FromRevision = acquired.diff.FromRevision
	result.ToRevision = acquired.diff.ToRevision
	result.OldAcquisition = acquired.old.Outcomes
	result.NewAcquisition = acquired.current.Outcomes
}

func BuildV2(ctx context.Context, raw []byte, req BuildRequest, diffs DiffSource, workspaces WorkspaceProvider, sessions SessionProvider) (Result, error) {
	acquired, err := acquireRevisions(ctx, raw, req, diffs, workspaces, sessions)
	if err != nil {
		return Result{}, err
	}
	result, err := attributeV2(acquired.diff.Lines, acquired.symbols)
	if err != nil {
		return Result{}, err
	}
	bindResult(&result, raw, acquired)
	if err = ValidateComposed(result, len(req.Paths)); err != nil {
		return Result{}, err
	}
	return result, nil
}

func BuildV3(ctx context.Context, raw []byte, req BuildRequest, diffs DiffSource, workspaces WorkspaceProvider, sessions SessionProvider) (ResultV3, error) {
	acquired, err := acquireRevisions(ctx, raw, req, diffs, workspaces, sessions)
	if err != nil {
		return ResultV3{}, err
	}
	result, err := Attribute(acquired.diff.Lines, acquired.symbols)
	if err != nil {
		return ResultV3{}, err
	}
	bindResult(&result.Result, raw, acquired)
	if err = ValidateComposedV3(result, len(req.Paths)); err != nil {
		return ResultV3{}, err
	}
	return result, nil
}
func ValidateComposed(r Result, pathCount int) error {
	if err := Validate(r); err != nil {
		return err
	}
	return validateComposedBinding(r, pathCount)
}

func ValidateComposedV3(r ResultV3, pathCount int) error {
	if err := ValidateV3(r); err != nil {
		return err
	}
	return validateComposedBinding(r.Result, pathCount)
}

func validateComposedBinding(r Result, pathCount int) error {
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

func Attribute(changes []ChangedLine, symbols []Symbol) (ResultV3, error) {
	ordered := append([]ChangedLine(nil), changes...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].Path != ordered[j].Path {
			return ordered[i].Path < ordered[j].Path
		}
		if ordered[i].Side != ordered[j].Side {
			return ordered[i].Side == "OLD"
		}
		return ordered[i].Line < ordered[j].Line
	})
	base, err := attributeV2(ordered, symbols)
	if err != nil {
		return ResultV3{}, err
	}
	base.SchemaVersion = SchemaVersionV3
	historical, current, err := metricsFromLines(base.Lines)
	if err != nil {
		return ResultV3{}, err
	}
	result := ResultV3{Result: base, HistoricalSymbolMetrics: historical, CurrentSymbolMetrics: current}
	if err := ValidateV3(result); err != nil {
		return ResultV3{}, err
	}
	return result, nil
}

func attributeV2(changes []ChangedLine, symbols []Symbol) (Result, error) {
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

type symbolMetricKey struct {
	Path  string
	Name  string
	Kind  int
	Range Range
}

type metricAccumulator struct {
	metric SymbolMetric
	lines  map[int]struct{}
}

func metricsFromLines(lines []LineAttribution) ([]SymbolMetric, []SymbolMetric, error) {
	bySide := map[string]map[symbolMetricKey]*metricAccumulator{"OLD": {}, "NEW": {}}
	for _, line := range lines {
		if line.Outcome != "ATTRIBUTED" {
			continue
		}
		if line.SymbolRange == nil || line.Path == "" || line.SymbolName == "" || line.SymbolKind < 1 {
			return nil, nil, errors.New("invalid attributed symbol metric identity")
		}
		span, err := symbolSpanLineCount(*line.SymbolRange)
		if err != nil {
			return nil, nil, err
		}
		key := symbolMetricKey{Path: line.Path, Name: line.SymbolName, Kind: line.SymbolKind, Range: *line.SymbolRange}
		acc := bySide[line.Side][key]
		if acc == nil {
			acc = &metricAccumulator{metric: SymbolMetric{Path: key.Path, Name: key.Name, Kind: key.Kind, Range: key.Range, SymbolSpanLineCount: span}, lines: map[int]struct{}{}}
			bySide[line.Side][key] = acc
		}
		acc.lines[line.Line] = struct{}{}
	}
	project := func(side string) ([]SymbolMetric, error) {
		out := make([]SymbolMetric, 0, len(bySide[side]))
		for _, acc := range bySide[side] {
			acc.metric.ChangedLineCount = len(acc.lines)
			density, err := densityBasisPoints(acc.metric.ChangedLineCount, acc.metric.SymbolSpanLineCount)
			if err != nil {
				return nil, err
			}
			acc.metric.ChurnDensityBasisPoints = density
			out = append(out, acc.metric)
		}
		sort.Slice(out, func(i, j int) bool { return metricLess(out[i], out[j]) })
		return out, nil
	}
	historical, err := project("OLD")
	if err != nil {
		return nil, nil, err
	}
	current, err := project("NEW")
	if err != nil {
		return nil, nil, err
	}
	return historical, current, nil
}

func symbolSpanLineCount(r Range) (int, error) {
	if r.Start.Line < 0 || r.Start.Character < 0 || r.End.Line < 0 || r.End.Character < 0 || !positionLess(r.Start, r.End) {
		return 0, errors.New("invalid symbol range for churn density")
	}
	span := r.End.Line - r.Start.Line
	if r.End.Character > 0 {
		if span == math.MaxInt {
			return 0, errors.New("symbol range span exceeds integer limit")
		}
		span++
	}
	if span <= 0 {
		return 0, errors.New("symbol range has no density-eligible lines")
	}
	return span, nil
}

func densityBasisPoints(changed, span int) (int, error) {
	if changed < 0 || span <= 0 || changed > span {
		return 0, errors.New("invalid symbol churn density inputs")
	}
	hi, lo := bits.Mul64(uint64(changed), 10000)
	quotient, _ := bits.Div64(hi, lo, uint64(span))
	if quotient > 10000 {
		return 0, errors.New("symbol churn density exceeds basis-point bound")
	}
	return int(quotient), nil
}

func metricLess(a, b SymbolMetric) bool {
	if a.Path != b.Path {
		return a.Path < b.Path
	}
	if a.Range.Start != b.Range.Start {
		return positionLess(a.Range.Start, b.Range.Start)
	}
	if a.Range.End != b.Range.End {
		return positionLess(a.Range.End, b.Range.End)
	}
	if a.Name != b.Name {
		return a.Name < b.Name
	}
	return a.Kind < b.Kind
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
	return validateResult(r, SchemaVersion)
}

func ValidateV3(r ResultV3) error {
	if err := validateResult(r.Result, SchemaVersionV3); err != nil {
		return err
	}
	if r.HistoricalSymbolMetrics == nil || r.CurrentSymbolMetrics == nil {
		return errors.New("symbol churn V3 metric arrays must be present")
	}
	historical, current, err := metricsFromLines(r.Lines)
	if err != nil {
		return err
	}
	if !equalMetrics(r.HistoricalSymbolMetrics, historical) || !equalMetrics(r.CurrentSymbolMetrics, current) {
		return errors.New("symbol churn V3 metrics do not match attributed rows")
	}
	return nil
}

func validateResult(r Result, schemaVersion string) error {
	if r.SchemaVersion != schemaVersion || r.Authority != 0 || r.SourceGraphComplete != "UNKNOWN" || r.Attribution != "HISTORICAL_SYMBOL_RANGE" || r.CrossRevisionIdentity != "NOT_EVALUATED" {
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
			if line.SymbolName == "" || line.SymbolKind < 1 || line.SymbolRange == nil {
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

func equalMetrics(a, b []SymbolMetric) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
