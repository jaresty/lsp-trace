//go:build darwin || linux

package main

import (
	"encoding/json"
	"testing"
)

func TestCensusReviewModeSettingsV3(t *testing.T) {
	c := callerHostSettingsFixture(t)
	c.Action = "run" // Discovery intentionally does not compare attempt policy.
	request, err := censusCallerHostObject(c.Request)
	if err != nil {
		t.Fatal(err)
	}
	inventory := censusCallerHostMap(request, "Inventory")
	inventory["ReviewMode"] = "SAME_THREAD"
	c.Request, _ = json.Marshal(request)
	saved, err := censusCallerHostStable(c)
	if err != nil {
		t.Fatal("ASSERT_REVIEW_MODE_SETTINGS", err)
	}
	if got := censusCallerHostMap(saved.RequestPolicy, "Inventory")["ReviewMode"]; got != "SAME_THREAD" {
		t.Fatal("ASSERT_REVIEW_MODE_SETTINGS", "policy dropped mode", got)
	}
	if !censusCallerHostSettingsMatch(saved, c) {
		t.Fatal("ASSERT_REVIEW_MODE_SETTINGS", "matching policy refused")
	}
	for _, replacement := range []string{"SUBAGENTS", ""} {
		inventory["ReviewMode"] = replacement
		if replacement == "" {
			delete(inventory, "ReviewMode")
		}
		c.Request, _ = json.Marshal(request)
		if censusCallerHostSettingsMatch(saved, c) {
			t.Fatal("ASSERT_REVIEW_MODE_SETTINGS", "mismatched mode accepted", replacement)
		}
	}
	t.Log("ASSERT_REVIEW_MODE_SETTINGS: PASS")
}
