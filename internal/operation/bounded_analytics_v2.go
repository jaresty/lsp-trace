package operation

import (
	"context"
	"encoding/json"

	"lsp-trace/internal/normativeanalytics"
)

const (
	BoundedRetainedAnalysisV2 Name = "bounded_retained_analysis_v2"
	BoundedRetainedMetricsV2  Name = "bounded_retained_metrics_v2"
	BoundedRetainedRankingV2  Name = "bounded_retained_ranking_v2"
)

// BoundedRetainedAnalyticsV2Handler runs package-certified local synthetic
// analytics only. ClaimLevel is explicit output context, not authorization.
func BoundedRetainedAnalyticsV2Handler(_ context.Context, request Request) (Result, *Failure) {
	var input struct {
		Input      json.RawMessage `json:"input"`
		Operation  string          `json:"operation"`
		Filter     []string        `json:"filter"`
		MaxWork    int64           `json:"max_work"`
		ClaimLevel string          `json:"claim_level"`
		Provenance json.RawMessage `json:"provenance"`
	}
	if err := json.Unmarshal(request.Input, &input); err != nil {
		return Result{}, inputFailure("INPUT_INVALID", err)
	}
	var text string
	var raw []byte
	if json.Unmarshal(input.Input, &text) == nil {
		raw = []byte(text)
	} else {
		raw = append([]byte(nil), input.Input...)
	}
	graph, _, err := normativeanalytics.DecodeRetainedGraph(raw)
	if err != nil || (input.ClaimLevel != "VERIFIED" && input.ClaimLevel != "UNVERIFIED") {
		return Result{}, inputFailure("INPUT_INVALID", normativeanalytics.ErrInvalidLocalRequest)
	}
	op := normativeanalytics.Operation(input.Operation)
	want := map[Name]normativeanalytics.Operation{BoundedRetainedAnalysisV2: normativeanalytics.Analysis, BoundedRetainedMetricsV2: normativeanalytics.Metrics, BoundedRetainedRankingV2: normativeanalytics.Ranking}[request.Name]
	if op != want {
		return Result{}, inputFailure("INPUT_INVALID", normativeanalytics.ErrInvalidLocalRequest)
	}
	result, err := normativeanalytics.EvaluateLocal(normativeanalytics.LocalRequest{Operation: op, BuildRevision: graph.BuildRevision, RetainedJSON: raw, Relations: input.Filter, Policy: normativeanalytics.LocalPolicy{MaxWork: input.MaxWork}})
	if err != nil {
		return Result{}, inputFailure("INPUT_INVALID", err)
	}
	artifact, err := normativeanalytics.MarshalLocalResult(result)
	if err != nil {
		return Result{}, inputFailure("OUTPUT_VALIDATION_FAILED", err)
	}
	return Result{Artifact: artifact, LogicalDigest: result.Digest}, nil
}
