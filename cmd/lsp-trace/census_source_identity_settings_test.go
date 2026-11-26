//go:build darwin || linux

package main

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestCensusFreshIdentitySettingsV3(t *testing.T) {
	const a = "ASSERT_FRESH_IDENTITY_SETTINGS"
	c := callerHostSettingsFixture(t)
	var q map[string]any
	if json.Unmarshal(c.Request, &q) != nil {
		t.Fatal("SETUP_SETTINGS_IDENTITY")
	}
	config := q["Configuration"].(map[string]any)
	inventory := q["Inventory"].(map[string]any)
	owner := inventory["NativeOwners"].([]any)[0].(map[string]any)
	config["RevisionAssertion"] = "fresh-attempt-revision"
	inventory["SourceRevision"] = "fresh-attempt-revision"
	inventory["SourceIdentity"] = map[string]any{"Schema": "inventory-source-identity.v1", "DeclaredRevision": "fresh-attempt-declaration"}
	owner["Revision"] = "fresh-attempt-revision"
	c.Request, _ = json.Marshal(q)
	saved, e := censusCallerHostStable(c)
	raw, _ := json.Marshal(saved)
	if e != nil || bytes.Contains(raw, []byte("fresh-attempt-")) {
		t.Fatal(a, "FAIL fresh state saved as policy", e)
	}
	// Refreshing only identity leaves reusable settings unchanged. This does not
	// admit either request or assert that stored headers are fresh source evidence.
	config["RevisionAssertion"] = "another-fresh-revision"
	inventory["SourceRevision"] = "another-fresh-revision"
	inventory["SourceIdentity"] = map[string]any{"DeclaredRevision": "another-fresh-declaration"}
	owner["Revision"] = "another-fresh-revision"
	c.Request, _ = json.Marshal(q)
	again, e := censusCallerHostStable(c)
	if e != nil || !censusCallerHostSame(saved, again) {
		t.Fatal(a, "FAIL identity refresh changes reusable policy", e)
	}
	t.Log(a + ": PASS")
}
