package censuscontinuation

import (
	"context"
	"errors"
	"lsp-trace/internal/targetpacket"
	"time"
)

// These lower entry shapes are refusal-only. Producer-owned source preparation
// lives in inventorysuccessor/sourcecontinuation, above the dependency boundary.
const PrivateAllModeSourcesVersion = "lsp-trace.private.allmode-source-continuation.v1"
const StopAfterPrivateSourceAdmissions StopAfter = "PRIVATE_SOURCE_ADMISSIONS"
const StopAfterPrivateSourceRequests StopAfter = "PRIVATE_SOURCE_DESCRIBE_REQUESTS"

type PrivateAllModeSourcesRequest struct {
	PolicyID string
	Limits   targetpacket.PrivateSourceLimits
	Deadline time.Time
}
type PrivateAllModeSourcesResume struct{ Version string }
type PrivateAllModeSourcesResult struct{}

func privateSourcesFailure(err error) Result {
	return Result{PrivateStage: "PRIVATE_SOURCES", PrivateCode: "PRIVATE_SOURCE_STAGE_REFUSED", Err: err}
}
func runPrivateAllModeSources(context.Context, Request) Result {
	return privateSourcesFailure(errors.New("producer-owned upper source continuation required"))
}
func resumePrivateAllModeSources(context.Context, ResumeRequest) Result {
	return privateSourcesFailure(errors.New("producer-owned upper source continuation required"))
}
