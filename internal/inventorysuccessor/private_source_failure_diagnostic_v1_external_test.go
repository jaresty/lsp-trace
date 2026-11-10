//go:build darwin || linux

package inventorysuccessor_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"lsp-trace/internal/describeworker"
	inv "lsp-trace/internal/inventorysuccessor"
	so "lsp-trace/internal/inventorysuccessor/sourceoperator"
	host "lsp-trace/internal/privatedescribeprep"
	"lsp-trace/internal/programcadmission"
	tp "lsp-trace/internal/targetpacket"
	"lsp-trace/sessionruntime"
)

// Genuine existing test-only custody is imported through inventorysuccessor's
// external test variant. No production admission hook or grant factory is added.
func TestSourceFailureDiagnosticRunCaptureModeled(t *testing.T) {
	for _, mode := range []string{"reservation", "reservation-terminal"} {
		t.Run(mode, func(t *testing.T) {
			terminal := strings.HasSuffix(mode, "-terminal")
			if terminal && controlledTimingClock == nil {
				t.Skip("requires existing lsp_trace_controlled_clock build tag; no sleep/renewal substitute")
			}
			owner, root := inv.ControlledLocalOwnerForTest(t)
			base, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			for _, name := range []string{"mailbox", "a", "b"} {
				if err = os.Mkdir(filepath.Join(base, name), 0700); err != nil {
					t.Fatal(err)
				}
			}
			// Existing genuine manager/profile/process declaration; no Start occurs.
			manager, start, uri := mixedStartFixtureV2(t)
			deadline := start.Deadline
			if _, err := manager.PrivateProviderWindowEnvelope(start, sessionruntime.PrivateProviderWindowPlan{References: true}); err != nil {
				t.Fatal("ASSERT_RUN_FIXTURE_ENVELOPE_ADMISSIBLE")
			}
			initialHead := owner.PreparationHeadV1()
			capture := inv.PrivateMixedInputRequestV2{Manager: manager, Root: root, Start: start, URI: uri, OccurrenceID: "modeled-op", Revision: "modeled-revision", Privacy: "private-modeled", OccurrencePolicy: "ordinal", CompositionPolicy: "identity", Line: 1, Character: 6, Kinds: []programcadmission.RelationKind{programcadmission.TypedReferencesSymbol}}
			digest := func(s string) string { return fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(s))) }
			q := so.Request{Owner: owner, Capture: capture, CaptureAttempt: digest("modeled-capture"), PreparationAttempt: digest("modeled-preparation"), RoleAttempt: "modeled-role", RunID: "modeled-run", SeedNamespace: "modeled-schedule", Seed: 7, Contract: sourceStageResponseContract(t, 131072), SourceLimits: tp.PrivateSourceLimits{MaxDocumentBytes: 16 << 10, MaxTotalBytes: 64 << 10, MaxWork: 4096, MaxObjects: 16}, Host: host.SourceHostConfig{Protocol: host.SourceHostProtocol, Mailbox: host.AutomaticHandoffConfigPrivateV2{Enabled: true, Directory: filepath.Join(base, "mailbox"), Invocation: "modeled", InterpreterRole: "disabled", ReviewerRole: "disabled", TimeoutMS: 5000, OverallTimeoutMS: 5000, PollMS: 10, Limits: host.AutomaticLimitsPrivateV2{MaxCalls: 1, MaxRequestBytes: 65536, MaxResponseBytes: 16384, MaxTotalBytes: 147456, MaxOutputBytes: 65536}}}, ResponseLimits: describeworker.SuppliedResponseLimits{MaxAdapterBytes: 8192, MaxSemanticBytes: 4096}, OutputRoots: [2]string{filepath.Join(base, "a"), filepath.Join(base, "b")}}
			stage := so.FailureCaptureReserveV1
			q.CaptureAttempt = "not-a-digest"
			ctx := context.Background()
			fired := false
			if terminal {
				ctx = controlledTimingClock(ctx, func(boundary string) time.Time {
					if boundary == "terminal" {
						fired = true
						return deadline
					}
					return time.Now()
				})
			}
			// This is the actual production Run, not failV1 or synthesized metadata.
			out, runErr := so.Run(ctx, q)
			f := out.FailureDiagnosticV1()
			if !errors.Is(runErr, inv.ErrPrivateMixedInputV2) || f.Stage != stage || manager.Census().Sessions != 0 || manager.Census().Workers != 0 {
				t.Fatalf("ASSERT_RUN_POST_PUBLICATION_CAPTURE: mode=%s stage=%s mixed=%t", mode, f.Stage.String(), errors.Is(runErr, inv.ErrPrivateMixedInputV2))
			}
			if f.Class != so.ClassifyFailureV1(inv.ErrPrivateMixedInputV2) {
				t.Fatalf("ASSERT_RUN_FIRST_OPERATION_BEFORE_TERMINAL: mode=%s class=%s", mode, f.Class.String())
			}
			if f.Terminal != terminal || errors.Is(runErr, context.DeadlineExceeded) != terminal || fired != terminal || ctx.Err() != nil || f.PublicationClose || f.PendingFinalize {
				t.Fatal("ASSERT_RUN_OPERATION_TERMINAL_DISTINCT", mode)
			}
			if owner.PreparationHeadV1() != initialHead {
				t.Fatal("ASSERT_RUN_REFUSAL_NO_RESERVATION")
			}
			if out.Disposition != "FAILED_RETAINED" || out.NativeID != "" || out.Published != [2]bool{} || out.Readback != [2]bool{} || out.Counts.Requests != 0 {
				t.Fatal("ASSERT_RUN_REFUSAL_NO_OUTPUT_CREDIT", mode)
			}
			for _, path := range []string{q.Host.Mailbox.Directory, q.OutputRoots[0], q.OutputRoots[1]} {
				entries, err := os.ReadDir(path)
				if err != nil || len(entries) != 0 {
					t.Fatal("ASSERT_RUN_REFUSAL_NO_PUBLICATION", mode)
				}
			}
			t.Log("ASSERT_RUN_POST_PUBLICATION_CAPTURE: PASS; ASSERT_RUN_FIRST_OPERATION_BEFORE_TERMINAL: PASS; ASSERT_RUN_OPERATION_TERMINAL_DISTINCT: PASS; ASSERT_RUN_REFUSAL_NO_OUTPUT_CREDIT: PASS; ASSERT_RUN_REFUSAL_NO_PUBLICATION: PASS (MODELED genuine temporary owner; no child/provider/model)")
		})
	}
}
