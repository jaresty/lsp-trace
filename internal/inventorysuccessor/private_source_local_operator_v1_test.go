//go:build darwin || linux

package inventorysuccessor_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"lsp-trace/internal/describeworker"
	inv "lsp-trace/internal/inventorysuccessor"
	so "lsp-trace/internal/inventorysuccessor/sourceoperator"
	host "lsp-trace/internal/privatedescribeprep"
	"lsp-trace/internal/programcadmission"
	tp "lsp-trace/internal/targetpacket"
)

// All modes cross actual Run -> genuine M capture/lease/window join -> original
// owner P reservation -> pending source metadata/workflow -> real pin descriptor
// IO -> close -> terminal Observe. Only existing controlled SELF acquisition runs;
// no worker/model/library is executed or provisioned and no role publisher exists.
func TestSourceOperatorLocalOwnership(t *testing.T) {
	for _, mode := range []string{"tiny", "insufficient", "overflow", "aggregate", "changed-pin", "overrun", "growth", "short-read", "symlink", "ancestor-symlink", "cancel", "deadline", "close-failure", "immutable-config"} {
		t.Run(mode, func(t *testing.T) {
			owner, root := inv.ControlledLocalOwnerForTest(t)
			m, start, uri := mixedStartFixtureV2(t)
			if e := os.WriteFile(filepath.Join(start.Profile.Workspace().String(), "root.go"), []byte("package fixture\nfunc Root(){ Other() }\nfunc Other(){ Root() }\n"), 0644); e != nil {
				t.Fatal(e)
			}
			start.Process.Args = []string{"-test.run=^TestPrivateSourceSELFResponder$", "--", "private-source-self", start.Profile.Workspace().String(), "positive"}
			if mode == "deadline" {
				start.Deadline = time.Now().Add(2 * time.Second)
			}
			ctx, cancel := context.WithDeadline(context.Background(), start.Deadline)
			defer cancel()
			base, e := filepath.EvalSymlinks(t.TempDir())
			if e != nil {
				t.Fatal(e)
			}
			for _, p := range []string{"mailbox", "a", "b", "pins"} {
				if e = os.Mkdir(filepath.Join(base, p), 0700); e != nil {
					t.Fatal(e)
				}
			}
			pinPath := filepath.Join(base, "pins", "tiny")
			data := []byte("tiny-controlled-artifact\n")
			if e = os.WriteFile(pinPath, data, 0600); e != nil {
				t.Fatal(e)
			}
			pin := describeworker.FilePin{Path: pinPath, SHA256: fmt.Sprintf("sha256:%x", sha256.Sum256(data))}
			c := inv.PrivateSourceLocalConfigV1{Config: describeworker.Config{Worker: pin, Model: pin, Library: pin, SandboxExecutable: pin, SandboxProfile: pin, Grammar: pin, RuntimeIdentity: "controlled-only", AdapterIdentity: "controlled-only", ModelIdentity: "not-a-model", ResponseVersion: describeworker.ResponseVersionV2, Limits: describeworker.Limits{TimeoutMS: 1000, MaxTokens: 1, ContextTokens: 1, StdoutBytes: 8192, StderrBytes: 1024, WorkBytes: 4096, TempBytes: 4096}}}
			for i := range c.Lengths {
				c.Lengths[i] = uint64(len(data))
			}
			switch mode {
			case "aggregate":
				c.Config.Limits.StdoutBytes = 8 << 20 // independently funded, but connected parent retention cannot fit
			case "insufficient":
				c.Lengths[1] = 1e12 // unchanged test credit; no artifact read may occur
			case "overflow":
				c.Lengths[1] = ^uint64(0)
			case "changed-pin":
				if e = os.WriteFile(pinPath, make([]byte, len(data)), 0600); e != nil {
					t.Fatal(e)
				}
			case "overrun":
				if e = os.WriteFile(pinPath, append(data, 'x'), 0600); e != nil {
					t.Fatal(e)
				}
			case "symlink":
				if e = os.Rename(pinPath, pinPath+"-real"); e != nil {
					t.Fatal(e)
				}
				if e = os.Symlink(pinPath+"-real", pinPath); e != nil {
					t.Fatal(e)
				}
			case "ancestor-symlink":
				if e = os.Rename(filepath.Join(base, "pins"), filepath.Join(base, "real")); e != nil {
					t.Fatal(e)
				}
				if e = os.Symlink(filepath.Join(base, "real"), filepath.Join(base, "pins")); e != nil {
					t.Fatal(e)
				}
			}
			opens, reads, closes := 0, 0, 0
			var debitAtOpen [14]uint64
			c = inv.ControlledLocalIOForTest(c, func(event string) error {
				disposition, windows, debit := inv.ControlledLocalHeadForTest(t, owner)
				if disposition != "PENDING" || windows != 2 {
					t.Fatalf("ASSERT_LOCAL_EFFECT_PENDING: event=%s disposition=%s windows=%d", event, disposition, windows)
				}
				if debit == [14]uint64{} {
					t.Fatal("ASSERT_LOCAL_REAL_DEBIT")
				}
				if opens > 0 && debit != debitAtOpen {
					t.Fatal("ASSERT_LOCAL_NO_THIRD_DEBIT")
				}
				switch event {
				case "open":
					opens++
					debitAtOpen = debit
					if mode == "immutable-config" {
						c.Config.Model.SHA256 = "invalid"
						c.Lengths[1] = ^uint64(0)
					}
				case "read":
					reads++
					if reads == 1 && mode == "growth" {
						if e := os.WriteFile(pinPath, append(data, 'x'), 0600); e != nil {
							return e
						}
					}
					if reads == 1 && mode == "short-read" {
						if e := os.WriteFile(pinPath, data[:1], 0600); e != nil {
							return e
						}
					}
				case "closed":
					closes++
					if mode == "cancel" {
						cancel()
					}
					if mode == "deadline" {
						<-ctx.Done()
					}
					if mode == "close-failure" {
						return errors.New("controlled actual pin close failure")
					}
				}
				return nil
			})
			capture := inv.PrivateMixedInputRequestV2{Manager: m, Root: root, Start: start, URI: uri, OccurrenceID: "local-source-op", Revision: "source-controlled-revision", Privacy: "private-source-controlled", OccurrencePolicy: "ordinal", CompositionPolicy: "identity", Line: 1, Character: 6, Kinds: []programcadmission.RelationKind{programcadmission.TypedReferencesSymbol}}
			digest := func(s string) string { return fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(s))) }
			q := so.Request{Owner: owner, Capture: capture, CaptureAttempt: digest("local-capture"), PreparationAttempt: digest("local-preparation"), RoleAttempt: "local-disabled", RunID: "local-run", SeedNamespace: "original-schedule", Seed: 7, Contract: sourceStageResponseContract(t, 131072), SourceLimits: tp.PrivateSourceLimits{MaxDocumentBytes: 16 << 10, MaxTotalBytes: 64 << 10, MaxWork: 10000, MaxObjects: 1000}, Host: host.SourceHostConfig{Protocol: host.SourceHostProtocol, Mailbox: host.AutomaticHandoffConfigPrivateV2{Enabled: true, Directory: filepath.Join(base, "mailbox"), Invocation: "local-op", InterpreterRole: "disabled", ReviewerRole: "disabled", TimeoutMS: 5000, OverallTimeoutMS: 5000, PollMS: 1, Limits: host.AutomaticLimitsPrivateV2{MaxCalls: 1, MaxRequestBytes: 65536, MaxResponseBytes: 16384, MaxTotalBytes: 147456, MaxOutputBytes: 65536}}}, ResponseLimits: describeworker.SuppliedResponseLimits{MaxAdapterBytes: 8192, MaxSemanticBytes: 4096}, OutputRoots: [2]string{filepath.Join(base, "a"), filepath.Join(base, "b")}, Local: &c}
			if mode == "tiny" {
				for _, mutate := range []func(*so.Request){
					func(r *so.Request) { r.SourceLimits.MaxDocumentBytes = (16 << 10) + 1 },
					func(r *so.Request) { r.SourceLimits.MaxTotalBytes = (64 << 10) + 1 },
					func(r *so.Request) { r.Contract = sourceStageResponseContract(t, 131073) },
					func(r *so.Request) { r.ResponseLimits.MaxAdapterBytes = 8193 },
					func(r *so.Request) { r.ResponseLimits.MaxSemanticBytes = 4097 },
					func(r *so.Request) { r.Host.Mailbox.Limits.MaxCalls = 2 },
					func(r *so.Request) { r.Host.Mailbox.Limits.MaxRequestBytes = 65537 },
					func(r *so.Request) { r.Host.Mailbox.Limits.MaxResponseBytes = 16385 },
					func(r *so.Request) { r.Host.Mailbox.Limits.MaxTotalBytes = 147457 },
					func(r *so.Request) { r.Host.Mailbox.Limits.MaxOutputBytes = 65537 },
				} {
					bad := q
					mutate(&bad)
					got, e := so.Run(ctx, bad)
					d, w, b := inv.ControlledLocalHeadForTest(t, owner)
					if e == nil || got.NativeID != "" || d != "READY" || w != 0 || b != [14]uint64{} || opens != 0 || m.Census().Sessions != 0 {
						t.Fatal("ASSERT_LOCAL_FIXED_LIMITS_PRE_CAPTURE", got, e, d, w)
					}
				}
				t.Log("ASSERT_LOCAL_FIXED_LIMITS_PRE_CAPTURE: PASS; ten one-over boundaries, zero capture/debit/artifact IO")
			}
			originalGrant := inv.ControlledLocalGrantForTest(t, owner)
			out, runErr := so.Run(ctx, q)
			if inv.ControlledLocalGrantForTest(t, owner) != originalGrant {
				t.Fatal("ASSERT_LOCAL_GRANT_UNCHANGED")
			}
			disposition, windows, finalDebit := inv.ControlledLocalHeadForTest(t, owner)
			if out.NativeID != "" || out.Published != [2]bool{} || out.Readback != [2]bool{} || runErr == nil {
				t.Fatal("ASSERT_LOCAL_NO_USABLE_OUTPUT", out, runErr)
			}
			if m.Census().Workers != 0 {
				t.Fatal("ASSERT_LOCAL_M_JOINED", m.Census())
			}
			for _, path := range []string{q.Host.Mailbox.Directory, q.OutputRoots[0], q.OutputRoots[1]} {
				entries, e := os.ReadDir(path)
				if e != nil || len(entries) != 0 {
					t.Fatal("ASSERT_LOCAL_NO_PUBLICATION", path, e)
				}
			}
			if mode == "insufficient" || mode == "overflow" {
				if opens != 0 || reads != 0 || closes != 0 || windows != 1 || disposition != "OBSERVED" {
					t.Fatal("ASSERT_LOCAL_PRE_EFFECT_REFUSAL", opens, reads, closes, windows, disposition, runErr)
				}
				t.Log("ASSERT_LOCAL_PRE_EFFECT_REFUSAL: PASS; unchanged grant; actual M OBSERVED; P never reserved")
				return
			}
			if mode == "aggregate" {
				if !errors.Is(runErr, so.ErrNativeAggregate) || opens != 0 || reads != 0 || closes != 0 || windows != 2 || disposition != "PENDING" || out.Counts.Requests == 0 {
					t.Fatal("ASSERT_LOCAL_CONNECTED_AGGREGATE_BEFORE_IO", out, disposition, windows, opens, runErr)
				}
				t.Log("ASSERT_LOCAL_CONNECTED_AGGREGATE_BEFORE_IO: PASS; funded P remains PENDING; local plus native/store/binder retention exceeds unchanged parent ceiling")
				return
			}
			if windows != 2 {
				t.Fatal("ASSERT_LOCAL_TWO_WINDOWS", windows, runErr)
			}
			if mode == "tiny" || mode == "immutable-config" {
				if !errors.Is(runErr, inv.ErrPrivateSourceLocalDeliveryDisabledV1) || out.Disposition != "LOCAL_VERIFIED_DELIVERY_DISABLED" || disposition != "OBSERVED" || opens != 6 || closes != 6 || reads != 12 || finalDebit != debitAtOpen || out.Counts.Requests == 0 {
					t.Fatal("ASSERT_LOCAL_PENDING_THROUGH_LAST_CLOSE", mode, out, disposition, opens, reads, closes, runErr)
				}
				t.Log("ASSERT_LOCAL_PENDING_THROUGH_LAST_CLOSE: PASS; six real bounded pins; final OBSERVED; exact two M/P windows; B disabled")
			} else {
				if opens == 0 {
					t.Fatal("ASSERT_LOCAL_FAILURE_IO_REACHED", mode, runErr)
				}
				if mode != "symlink" && mode != "ancestor-symlink" && closes == 0 {
					t.Fatal("ASSERT_LOCAL_FAILURE_CLOSE_REACHED", mode, runErr)
				}
				if disposition != "PENDING" || errors.Is(runErr, inv.ErrPrivateSourceLocalDeliveryDisabledV1) {
					t.Fatal("ASSERT_LOCAL_FAILURE_RETAINS_PENDING", mode, disposition, runErr)
				}
				if mode == "overrun" && reads != 0 {
					t.Fatal("ASSERT_LOCAL_LENGTH_BEFORE_READ", reads)
				}
				t.Logf("ASSERT_LOCAL_FAILURE_RETAINS_PENDING: PASS mode=%s opens=%d reads=%d closes=%d", mode, opens, reads, closes)
			}
			beforeOpens := opens
			retry, retryErr := so.Run(ctx, q)
			d2, w2, b2 := inv.ControlledLocalHeadForTest(t, owner)
			if retryErr == nil || retry.NativeID != "" || opens != beforeOpens || d2 != disposition || w2 != windows || b2 != finalDebit {
				t.Fatal("ASSERT_LOCAL_NO_RETRY_RESET_OR_THIRD_WINDOW", retryErr, d2, w2)
			}
		})
	}
}
