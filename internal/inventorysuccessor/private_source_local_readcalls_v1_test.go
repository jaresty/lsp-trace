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

	"lsp-trace/internal/describeworker"
	inv "lsp-trace/internal/inventorysuccessor"
	so "lsp-trace/internal/inventorysuccessor/sourceoperator"
	host "lsp-trace/internal/privatedescribeprep"
	"lsp-trace/internal/programcadmission"
	tp "lsp-trace/internal/targetpacket"
)

// Independent expected bound: ceil(n/32768) data calls plus the rejecting EOF
// probe. A fragmented descriptor may refuse; it cannot borrow more calls merely
// because the bytes/hash still fit. No production helper computes this oracle.
func localReadCallsExpected(n uint64) uint64 {
	q := n / 32768
	if n%32768 != 0 {
		q++
	}
	return q + 1
}
func TestSourceOperatorLocalReadCalls(t *testing.T) {
	for _, tc := range []struct {
		name     string
		length   int
		fragment bool
	}{
		{"fragmented-valid-mib", 1 << 20, true},
		{"fragmented-valid-two-byte", 2, true},
		{"zero-config-refused", 0, false},
		{"one-byte", 1, false},
		{"below-chunk", 32767, false},
		{"aligned-chunk", 32768, false},
		{"nonaligned-chunk", 32769, false},
		{"normal-mib", 1 << 20, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			owner, root := inv.ControlledLocalOwnerForTest(t)
			m, start, uri := mixedStartFixtureV2(t)
			if e := os.WriteFile(filepath.Join(start.Profile.Workspace().String(), "root.go"), []byte("package fixture\nfunc Root(){ Other() }\nfunc Other(){ Root() }\n"), 0644); e != nil {
				t.Fatal(e)
			}
			start.Process.Args = []string{"-test.run=^TestPrivateSourceSELFResponder$", "--", "private-source-self", start.Profile.Workspace().String(), "positive"}
			ctx, cancel := context.WithDeadline(context.Background(), start.Deadline)
			defer cancel()
			base, e := filepath.EvalSymlinks(t.TempDir())
			if e != nil {
				t.Fatal(e)
			}
			for _, s := range []string{"mailbox", "a", "b", "pins"} {
				if e = os.Mkdir(filepath.Join(base, s), 0700); e != nil {
					t.Fatal(e)
				}
			}
			// Six pins, each a valid private file binding to these exact unchanged bytes.
			data := make([]byte, tc.length)
			pinPath := filepath.Join(base, "pins", "valid")
			if e = os.WriteFile(pinPath, data, 0600); e != nil {
				t.Fatal(e)
			}
			pin := describeworker.FilePin{Path: pinPath, SHA256: fmt.Sprintf("sha256:%x", sha256.Sum256(data))}
			c := inv.PrivateSourceLocalConfigV1{Config: describeworker.Config{Worker: pin, Model: pin, Library: pin, SandboxExecutable: pin, SandboxProfile: pin, Grammar: pin, RuntimeIdentity: "controlled-only", AdapterIdentity: "controlled-only", ModelIdentity: "not-a-model", ResponseVersion: describeworker.ResponseVersionV2, Limits: describeworker.Limits{TimeoutMS: 1000, MaxTokens: 1, ContextTokens: 1, StdoutBytes: 8192, StderrBytes: 1024, WorkBytes: 4096, TempBytes: 4096}}}
			for i := range c.Lengths {
				c.Lengths[i] = uint64(tc.length)
			}
			var reads [6]uint64
			opens, closes := 0, 0
			var localQuote, pTranche, cumulative uint64
			var debitAtOpen [14]uint64
			observe := func(event string) error {
				// Count actual descriptor.Read entries without an expensive kernel reopen on
				// every byte. Metadata/open/close edges independently verify persisted P.
				if event == "read" {
					reads[opens-1]++
					return nil
				}
				d, w, b := inv.ControlledLocalHeadForTest(t, owner)
				if d != "PENDING" || w != 2 {
					t.Fatalf("ASSERT_LOCAL_READ_CALLS_PENDING: event=%s head=%s windows=%d", event, d, w)
				}
				if opens > 0 && b != debitAtOpen {
					t.Fatal("ASSERT_LOCAL_READ_CALLS_STABLE_DEBIT")
				}
				switch event {
				case "open":
					opens++
					if opens == 1 {
						debitAtOpen = b
						localQuote, pTranche, cumulative = inv.ControlledLocalReservationCallsForTest(t, owner, c)
					}
				case "closed":
					closes++
				}
				return nil
			}
			if tc.fragment {
				c = inv.ControlledLocalFragmentedIOForTest(c, observe)
			} else {
				c = inv.ControlledLocalIOForTest(c, observe)
			}
			digest := func(s string) string { return fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(s))) }
			q := so.Request{Owner: owner, Capture: inv.PrivateMixedInputRequestV2{Manager: m, Root: root, Start: start, URI: uri, OccurrenceID: "read-call-op", Revision: "source-controlled-revision", Privacy: "private-source-controlled", OccurrencePolicy: "ordinal", CompositionPolicy: "identity", Line: 1, Character: 6, Kinds: []programcadmission.RelationKind{programcadmission.TypedReferencesSymbol}}, CaptureAttempt: digest("read-call-M"), PreparationAttempt: digest("read-call-P"), RoleAttempt: "disabled", RunID: "read-call-run", SeedNamespace: "original-schedule", Seed: 7, Contract: sourceStageResponseContract(t, 131072), SourceLimits: tp.PrivateSourceLimits{MaxDocumentBytes: 16 << 10, MaxTotalBytes: 64 << 10, MaxWork: 10000, MaxObjects: 1000}, Host: host.SourceHostConfig{Protocol: host.SourceHostProtocol, Mailbox: host.AutomaticHandoffConfigPrivateV2{Enabled: true, Directory: filepath.Join(base, "mailbox"), Invocation: "read-call-op", InterpreterRole: "disabled", ReviewerRole: "disabled", TimeoutMS: 5000, OverallTimeoutMS: 5000, PollMS: 1, Limits: host.AutomaticLimitsPrivateV2{MaxCalls: 1, MaxRequestBytes: 65536, MaxResponseBytes: 16384, MaxTotalBytes: 147456, MaxOutputBytes: 65536}}}, ResponseLimits: describeworker.SuppliedResponseLimits{MaxAdapterBytes: 8192, MaxSemanticBytes: 4096}, OutputRoots: [2]string{filepath.Join(base, "a"), filepath.Join(base, "b")}, Local: &c}
			grant := inv.ControlledLocalGrantForTest(t, owner)
			out, runErr := so.Run(ctx, q)
			d, w, b := inv.ControlledLocalHeadForTest(t, owner)
			bound := localReadCallsExpected(uint64(tc.length))
			if tc.fragment {
				// Refuse BEFORE the next unfunded Read, whether bytes remain (MiB)
				// or the next call would be the EOF probe (two-byte fragmented pin).
				if reads[0] != bound || opens != 1 || closes != 1 || d != "PENDING" || w != 2 || runErr == nil || errors.Is(runErr, inv.ErrPrivateSourceLocalDeliveryDisabledV1) || !strings.Contains(runErr.Error(), "prepaid descriptor read-call ceiling exhausted") {
					t.Fatalf("ASSERT_LOCAL_FRAGMENT_READ_CALL_CAP: reads=%v bound=%d opens=%d closes=%d head=%s windows=%d local_quote=%d P_tranche_calls=%d cumulative_calls=%d err=%v", reads, bound, opens, closes, d, w, localQuote, pTranche, cumulative, runErr)
				}
				required := 6*uint64(tc.length) + 6
				if tc.length == 1<<20 && required <= cumulative {
					t.Fatal("ASSERT_LOCAL_FRAGMENT_EXCEEDS_RECORDED_FUNDING", required, cumulative)
				}
				t.Logf("ASSERT_LOCAL_FRAGMENT_READ_CALL_CAP: PASS reads=%v bound=%d refused_before=%d local_quote=%d P_tranche_calls=%d cumulative_calls=%d full_fragment_reads=%d", reads, bound, bound+1, localQuote, pTranche, cumulative, required)
			} else if tc.length == 0 {
				if runErr == nil || opens != 0 || closes != 0 || d != "OBSERVED" || w != 1 {
					t.Fatal("ASSERT_LOCAL_ZERO_LENGTH_PRE_IO", opens, closes, d, w, runErr)
				}
			} else {
				if !errors.Is(runErr, inv.ErrPrivateSourceLocalDeliveryDisabledV1) || d != "OBSERVED" || w != 2 || opens != 6 || closes != 6 {
					t.Fatal("ASSERT_LOCAL_NORMAL_READ_CALLS", d, w, opens, closes, runErr)
				}
				for i, n := range reads {
					if n != bound {
						t.Fatal("ASSERT_LOCAL_NORMAL_READ_CALLS", i, n, bound)
					}
				}
				t.Logf("ASSERT_LOCAL_NORMAL_READ_CALLS: PASS length=%d reads=%v per_pin_quote=%d", tc.length, reads, bound)
			}
			if tc.length > 0 {
				nonRead := uint64(64 + 6*(2*strings.Count(pinPath, "/")+8))
				if localQuote != nonRead+6*bound || pTranche < localQuote || cumulative != b[9] || b != debitAtOpen {
					t.Fatal("ASSERT_LOCAL_READ_CALL_QUOTE_JOIN", localQuote, nonRead+6*bound, pTranche, cumulative, b[9])
				}
			}
			if inv.ControlledLocalGrantForTest(t, owner) != grant || out.NativeID != "" || out.Published != [2]bool{} || out.Readback != [2]bool{} || m.Census().Workers != 0 {
				t.Fatal("ASSERT_LOCAL_READ_CALLS_NO_OUTPUT_OR_AUTH_CHANGE", out)
			}
			for _, p := range []string{q.Host.Mailbox.Directory, q.OutputRoots[0], q.OutputRoots[1]} {
				entries, e := os.ReadDir(p)
				if e != nil || len(entries) != 0 {
					t.Fatal("ASSERT_LOCAL_READ_CALLS_NO_PUBLICATION", p, e)
				}
			}
			if tc.fragment {
				before := reads
				retry, retryErr := so.Run(ctx, q)
				d2, w2, b2 := inv.ControlledLocalHeadForTest(t, owner)
				if retryErr == nil || retry.NativeID != "" || reads != before || d2 != d || w2 != w || b2 != b {
					t.Fatal("ASSERT_LOCAL_READ_CALLS_NO_THIRD_WINDOW")
				}
			}
		})
	}
}
