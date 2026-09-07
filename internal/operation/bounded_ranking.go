package operation

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"lsp-trace/internal/boundedanalysis"
	"lsp-trace/internal/boundedranking"
)

const BoundedRetainedRanking Name = "bounded_retained_ranking"

func BoundedRetainedRankingHandler(ctx context.Context, request Request) (Result, *Failure) {
	if err := boundedanalysis.Preflight(request.Input, boundedranking.MaxBytes); err != nil {
		return Result{}, inputFailure("INPUT_INVALID", err)
	}
	// Exact spelling and presence are checked before typed defaults; JSON null is
	// not omission. Integer decoding never truncates float64 seed weights.
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(request.Input, &fields); err != nil {
		return Result{}, inputFailure("INPUT_INVALID", err)
	}
	allowed := map[string]bool{"input": true, "algorithm": true, "alpha": true, "tolerance": true, "max_iterations": true, "max_work": true, "seeds": true}
	for k, v := range fields {
		if !allowed[k] || bytes.Equal(bytes.TrimSpace(v), []byte("null")) {
			return Result{}, inputFailure("INPUT_INVALID", fmt.Errorf("unknown/null ranking member %q", k))
		}
	}
	p := boundedranking.Defaults("")
	var in struct {
		Input         json.RawMessage       `json:"input"`
		Algorithm     string                `json:"algorithm"`
		Alpha         float64               `json:"alpha"`
		Tolerance     float64               `json:"tolerance"`
		MaxIterations int                   `json:"max_iterations"`
		MaxWork       int                   `json:"max_work"`
		Seeds         []boundedranking.Seed `json:"seeds"`
	}
	in.Alpha = p.Alpha
	in.Tolerance = p.Tolerance
	in.MaxIterations = p.MaxIterations
	in.MaxWork = p.MaxWork
	d := json.NewDecoder(bytes.NewReader(request.Input))
	d.DisallowUnknownFields()
	if err := d.Decode(&in); err != nil {
		return Result{}, inputFailure("INPUT_INVALID", err)
	}
	if in.Algorithm == "PAGERANK" && fields["seeds"] != nil {
		return Result{}, inputFailure("INPUT_INVALID", fmt.Errorf("PAGERANK forbids seeds"))
	}
	// Typed Go decoders accept case aliases: reject them explicitly in seed rows.
	var seeds []map[string]json.RawMessage
	if fields["seeds"] != nil {
		if err := json.Unmarshal(fields["seeds"], &seeds); err != nil {
			return Result{}, inputFailure("INPUT_INVALID", err)
		}
		for _, s := range seeds {
			if len(s) != 2 || s["node_id"] == nil || s["weight"] == nil {
				return Result{}, inputFailure("INPUT_INVALID", fmt.Errorf("exact seed members required"))
			}
		}
	}
	var text string
	if json.Unmarshal(in.Input, &text) == nil {
		in.Input = []byte(text)
	}
	p = boundedranking.Parameters{Algorithm: in.Algorithm, Alpha: in.Alpha, Tolerance: in.Tolerance, MaxIterations: in.MaxIterations, MaxWork: in.MaxWork, Seeds: in.Seeds}
	raw, err := boundedranking.Analyze(ctx, in.Input, p)
	if err != nil {
		return Result{}, inputFailure("INPUT_INVALID", err)
	}
	if _, err = boundedranking.ValidateFor(raw, boundedranking.Family, "v1"); err != nil {
		return Result{}, inputFailure("OUTPUT_VALIDATION_FAILED", err)
	}
	return Result{Artifact: raw}, nil
}
