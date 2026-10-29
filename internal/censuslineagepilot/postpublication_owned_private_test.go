//go:build darwin || linux

package censuslineagepilot_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	pilot "lsp-trace/internal/censuslineagepilot"
	prep "lsp-trace/internal/privatedescribeprep"
	"lsp-trace/internal/publication"
)

func TestOwnedPrivatePostPublication(t *testing.T) {
	ctx := context.Background()
	input, traffic := preparationOwnedInput(t)
	before, bp := traffic()
	envelope, _, e := input.Projection.PreparationEvidencePrivate()
	if e != nil {
		t.Fatal(e)
	}
	var cfg pilot.ScopedRehearsalConfigPrivate
	if e = json.Unmarshal(envelope.Records[0], &cfg); e != nil {
		t.Fatal(e)
	}
	var protected []*publication.ObservedRoot
	// Artifact roots, not the original workspace or source paths. Input ancestry
	// is allowed; every provided capability remains borrowed until after retention.
	paths := append(append([]string{}, cfg.InputRoots[:]...), cfg.OutputRoots[:]...)
	for _, path := range paths {
		r, e := publication.OpenObservedRoot(ctx, path)
		if e != nil {
			t.Fatal(e)
		}
		protected = append(protected, r)
		t.Cleanup(func() {
			if e := r.Close(); e != nil {
				t.Error(e)
			}
		})
	}
	home, e := os.UserHomeDir()
	if e != nil {
		t.Fatal(e)
	}
	base, e := os.MkdirTemp(home, ".postpublication-owned-sealed-")
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		if e := os.RemoveAll(base); e != nil {
			t.Error(e)
		}
	})
	var outputs [2]*publication.ContentRetentionRoot
	for i, name := range []string{"primary", "backup"} {
		p := filepath.Join(base, name)
		if e := os.Mkdir(p, 0700); e != nil {
			t.Fatal(e)
		}
		r, e := publication.OpenContentRetentionRoot(ctx, p)
		if e != nil {
			t.Fatal(e)
		}
		outputs[i] = r
		t.Cleanup(func() {
			if e := r.Close(); e != nil {
				t.Error(e)
			}
		})
	}
	limits := prep.Limits{MaxTargets: 64, MaxPartBytes: 64 << 10, MaxTotalBytes: 8 << 20, MaxParts: 256, MaxSourceBytes: 2 << 20}
	h, e := prep.NewPostPreparation(ctx, []prep.Input{input}, limits, ownedPostLimits())
	if e != nil {
		t.Fatal(e)
	}
	sub := ownedPostSubmission(t, h, 0)
	first, e := h.Reconcile(ctx, []prep.CallerSubmission{sub}, nil)
	if e != nil {
		t.Fatal(e)
	}
	review := prep.CallerReview{PrivateCeilings: ownedPostCeilings(), PreparationSHA256: sub.PreparationSHA256, InputSHA256: sub.InputSHA256, Target: sub.Target, SubmissionSHA256: first.Targets[0].SubmissionSHA256, ReviewerAuthorTag: "separate-caller-review", Verdict: "SUPPORT_WITH_LIMITS", Findings: "Synthetic owned declaration only; source completeness unknown"}
	out, e := prep.RetainPostPreparationPrivate(ctx, h, []prep.CallerSubmission{sub}, []prep.CallerReview{review}, outputs, protected)
	if e != nil || out.Phase != "PAIR_VERIFIED" {
		t.Fatal(e, out.Phase)
	}
	var parsed struct {
		Preparation json.RawMessage
		Ledger      prep.StageLedger
		Absence     string
	}
	if e = json.Unmarshal(out.EnvelopeBytes(), &parsed); e != nil {
		t.Fatal(e)
	}
	prepared, _ := h.PreparedBytes()
	if !bytes.Equal(prepared, parsed.Preparation) || len(parsed.Ledger.Leads) != 1 || parsed.Ledger.Coverage[0].SourceGap != 24 || out.Accepted || out.Authority != 0 || parsed.Absence != "PROHIBITED" {
		t.Fatal("sealed accounting changed")
	}
	for _, r := range outputs {
		if e := r.Close(); e != nil {
			t.Fatal(e)
		}
	}
	for _, r := range protected {
		if e := r.Close(); e != nil {
			t.Fatal(e)
		}
	}
	after, ap := traffic()
	if before != after || bp != ap {
		t.Fatal("publication performed provider traffic")
	}
}
