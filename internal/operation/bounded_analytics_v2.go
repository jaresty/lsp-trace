package operation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"

	"lsp-trace/internal/normativeanalytics"
)

const (
	BoundedRetainedAnalysisV2 Name = "bounded_retained_analysis_v2"
	BoundedRetainedMetricsV2  Name = "bounded_retained_metrics_v2"
	BoundedRetainedRankingV2  Name = "bounded_retained_ranking_v2"
)

// BoundedRetainedAnalyticsV2Handler runs package-certified local synthetic
// analytics only. Selector verification authenticates input custody without
// mutating the certified local result payload.
func BoundedRetainedAnalyticsV2Handler(_ context.Context, request Request) (Result, *Failure) {
	var input struct {
		Input               json.RawMessage `json:"input"`
		PublicationSelector *struct {
			Selector         string `json:"selector"`
			ArtifactSchemaID string `json:"artifact_schema_id"`
			ArtifactDigest   string `json:"artifact_digest"`
			ArtifactLength   uint64 `json:"artifact_byte_length"`
		} `json:"publication_selector"`
		Operation string   `json:"operation"`
		Filter    []string `json:"filter"`
		MaxWork   int64    `json:"max_work"`
	}
	if err := json.Unmarshal(request.Input, &input); err != nil {
		return Result{}, inputFailure("INPUT_INVALID", err)
	}
	var text string
	var raw []byte
	if input.PublicationSelector != nil {
		if len(input.Input) != 0 || request.PublicationRoot == nil || input.PublicationSelector.ArtifactSchemaID != "https://jaresty.github.io/lsp-trace/schemas/lsp-trace.normative-retained-graph.v1.schema.json" {
			return Result{}, inputFailure("INPUT_INVALID", normativeanalytics.ErrInvalidLocalRequest)
		}
		var err error
		raw, err = request.PublicationRoot.ReadSelector(input.PublicationSelector.Selector, normativeanalytics.MaxRetainedBytes)
		if err != nil {
			return Result{}, inputFailure("INPUT_INVALID", err)
		}
		sum := sha256.Sum256(raw)
		digest := "sha256:" + hex.EncodeToString(sum[:])
		if uint64(len(raw)) != input.PublicationSelector.ArtifactLength || digest != input.PublicationSelector.ArtifactDigest {
			return Result{}, inputFailure("INPUT_INVALID", errors.New("publication selector exact-byte verification failed"))
		}
	} else {
		if len(input.Input) == 0 {
			return Result{}, inputFailure("INPUT_INVALID", normativeanalytics.ErrInvalidLocalRequest)
		}
		if json.Unmarshal(input.Input, &text) == nil {
			raw = []byte(text)
		} else {
			raw = append([]byte(nil), input.Input...)
		}
	}
	graph, _, err := normativeanalytics.DecodeRetainedGraph(raw)
	if err != nil {
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
