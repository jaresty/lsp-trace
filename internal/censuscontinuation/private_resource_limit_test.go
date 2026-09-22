package censuscontinuation

import "testing"

type testPrivateResourceLimit struct{}

func (testPrivateResourceLimit) Error() string             { return "private limit" }
func (testPrivateResourceLimit) ResourceComponent() string { return "owner.component" }
func (testPrivateResourceLimit) ResourceField() string     { return "artifact_bytes" }
func (testPrivateResourceLimit) ResourceLimit() int64      { return 64 }
func (testPrivateResourceLimit) ResourceObserved() int64   { return 65 }
func (testPrivateResourceLimit) ResourceCategory() string  { return "output" }

func TestProductionCaptureLimitsAdmitRetainedNominationTier(t *testing.T) {
	limits := ProductionCaptureLimits()
	if limits.MaxReceipts != 1000 || limits.MaxBindings != 1000 || limits.MaxSourceBytes != 1<<20 || limits.MaxTotalSourceBytes != 8<<20 || limits.MaxWork != 10000 || limits.MaxArtifactBytes != 64<<20 || limits.MaxParentBytes != 64<<20 {
		t.Fatalf("ASSERT_PRODUCTION_CAPTURE_LIMIT_TIER: %+v", limits)
	}
}

func TestApplyPrivateResourceLimit(t *testing.T) {
	result := Result{}
	applyPrivateResourceLimit(&result, testPrivateResourceLimit{})
	if result.PrivateResourceComponent != "owner.component" || result.PrivateResourceLimit != 64 || result.PrivateResourceObserved != 65 || result.PrivateResourceCategory != "output" {
		t.Fatalf("ASSERT_CONTINUATION_PRIVATE_RESOURCE_ACCOUNTING: %+v", result)
	}
}
