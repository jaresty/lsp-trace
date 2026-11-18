package inventorysuccessor_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	app "lsp-trace/internal/applicationinventory"
	cat "lsp-trace/internal/provisionalfeaturecatalog"
	"os"
	"path/filepath"
	"testing"
)

// These use the existing genuine selected-description fixture and scripted
// reasoning callbacks. They do not establish live host or context independence.
func reviewModeRequestV3(t *testing.T, mode, reviewer string) app.Request {
	t.Helper()
	q := repairRequestV3(t, selectedFixtureV3(t))
	// A wire-bound input also lets the pre-change runtime execute this guard:
	// the old Request ignores the absent field, rather than failing compilation.
	raw, _ := json.Marshal(map[string]string{"ReviewMode": mode})
	if err := json.Unmarshal(raw, &q); err != nil {
		t.Fatal(err)
	}
	q.ReviewerIdentity = reviewer
	q.Reviewer = func(_ context.Context, raw []byte, w io.Writer) error {
		var c cat.NativeCatalogWireV3
		if err := cat.DecodeNativeV3(raw, &c); err != nil {
			return err
		}
		g := c.Groups[0]
		g.Label = "sequentially reviewed fixture coordination"
		g.Reason = "distinct review pass; context identity is not authenticated"
		return json.NewEncoder(w).Encode(cat.NativeDecisionV3{Predecessor: c.Revision, Actor: reviewer, Reason: g.Reason, Affected: []string{g.ID}, Groups: []cat.NativeGroupV3{g}})
	}
	return q
}

func TestApplicationInventoryReviewSameThreadV3(t *testing.T) {
	q := reviewModeRequestV3(t, "SAME_THREAD", "fixture-interpreter")
	calls := 0
	prepare := q.Preparer
	q.Preparer = func(ctx context.Context, lead app.Lead) (app.NativeResult, error) {
		calls++
		return prepare(ctx, lead)
	}
	r, err := app.Run(context.Background(), q)
	if err != nil || !r.Published || calls != 1 || r.SuccessfulDescriptions != 1 || r.WithMembership != 1 || r.Stages["review"].Successful != 1 {
		t.Fatal("ASSERT_REVIEW_SAME_THREAD_RUNTIME", err, calls, r)
	}
	var manifest app.PublicationManifestV3
	raw, err := os.ReadFile(filepath.Join(q.Destination, "manifest.json"))
	if err != nil || json.Unmarshal(raw, &manifest) != nil || len(manifest.Artifacts) != 4 {
		t.Fatal("ASSERT_REVIEW_SAME_THREAD_RUNTIME", err, "manifest readback")
	}
	for _, a := range manifest.Artifacts {
		raw, err := os.ReadFile(filepath.Join(q.Destination, a.Name))
		if err != nil || cat.NativeDigestV3(raw) != a.SHA256 {
			t.Fatal("ASSERT_REVIEW_SAME_THREAD_RUNTIME", err, a.Name)
		}
	}
	t.Log("ASSERT_REVIEW_SAME_THREAD_RUNTIME: PASS")
}

func TestApplicationInventoryReviewModeReportingV3(t *testing.T) {
	// Distinct-actor input reaches publication even on the old implementation,
	// so a missing mode yields an assertion failure, not a setup failure.
	for _, mode := range []string{"SAME_THREAD", "SUBAGENTS"} {
		q := reviewModeRequestV3(t, mode, "fixture-reviewer")
		r, err := app.Run(context.Background(), q)
		if err != nil || !r.Published {
			t.Fatal("ASSERT_REVIEW_MODE_REPORTING", mode, err)
		}
		for _, name := range []string{"report.json", "catalog-base.json", "catalog-proposed.json", "catalog-corrected.json"} {
			raw, err := os.ReadFile(filepath.Join(q.Destination, name))
			var wire map[string]json.RawMessage
			if err != nil || json.Unmarshal(raw, &wire) != nil {
				t.Fatal("ASSERT_REVIEW_MODE_REPORTING", name, err)
			}
			key := "review_mode"
			if name == "report.json" {
				key = "ReviewMode"
			}
			var got string
			if json.Unmarshal(wire[key], &got) != nil || got != mode {
				t.Fatal("ASSERT_REVIEW_MODE_REPORTING", name, mode, string(wire[key]))
			}
			if name != "report.json" && string(wire["review_identity"]) != `"NOT_AUTHENTICATED"` {
				t.Fatal("ASSERT_REVIEW_MODE_REPORTING", name, "identity assurance")
			}
		}
	}
	t.Log("ASSERT_REVIEW_MODE_REPORTING: PASS")
}

func TestApplicationInventoryReviewStrictV3(t *testing.T) {
	for _, mode := range []string{"", "SUBAGENTS"} {
		q := reviewModeRequestV3(t, mode, "fixture-interpreter")
		r, err := app.Run(context.Background(), q)
		if err == nil || r.Published || r.Stages["review"].Failed != 1 {
			t.Fatal("ASSERT_REVIEW_STRICT_RUNTIME", mode, err, r)
		}
		if _, err := os.Stat(q.Destination); !os.IsNotExist(err) {
			t.Fatal("ASSERT_REVIEW_STRICT_RUNTIME", mode, "unexpected publication", err)
		}
	}
	t.Log("ASSERT_REVIEW_STRICT_RUNTIME: PASS")
}

func TestApplicationInventoryReviewInvalidBeforeEffectsV3(t *testing.T) {
	q := reviewModeRequestV3(t, "invented-mode", "fixture-reviewer")
	calls := 0
	q.Preparer = func(context.Context, app.Lead) (app.NativeResult, error) {
		calls++
		return app.NativeResult{}, errors.New("must not be called")
	}
	r, err := app.Run(context.Background(), q)
	if err == nil || r.Published || calls != 0 {
		t.Fatal("ASSERT_REVIEW_MODE_BEFORE_EFFECTS", err, calls)
	}
	t.Log("ASSERT_REVIEW_MODE_BEFORE_EFFECTS: PASS")
}

func TestApplicationInventoryReviewLegacyEncodingV3(t *testing.T) {
	q := reviewModeRequestV3(t, "", "fixture-reviewer")
	r, err := app.Run(context.Background(), q)
	if err != nil || !r.Published {
		t.Fatal("ASSERT_REVIEW_LEGACY_ENCODING", err)
	}
	for _, name := range []string{"report.json", "catalog-base.json", "catalog-proposed.json", "catalog-corrected.json"} {
		raw, err := os.ReadFile(filepath.Join(q.Destination, name))
		var wire map[string]json.RawMessage
		if err != nil || json.Unmarshal(raw, &wire) != nil || wire["ReviewMode"] != nil || wire["review_mode"] != nil {
			t.Fatal("ASSERT_REVIEW_LEGACY_ENCODING", name, err)
		}
	}
	t.Log("ASSERT_REVIEW_LEGACY_ENCODING: PASS")
}
