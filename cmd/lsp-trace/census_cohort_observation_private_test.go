//go:build darwin || linux

package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	pilot "lsp-trace/internal/censuslineagepilot"
	prep "lsp-trace/internal/privatedescribeprep"
)

// Optional read-only diagnosis of original creator bytes. This never opens an
// application claim, handoff backend or original-owner continuation.
func TestCohortOriginalPublicationObservationPrivate(t *testing.T) {
	report := os.Getenv("LSP_TRACE_COHORT_OBSERVATION_REPORT")
	if report == "" {
		t.Skip("explicit original report path required")
	}
	b, e := os.ReadFile(report)
	if e != nil {
		t.Fatal(e)
	}
	var r censusInventoryReportPrivate
	if json.Unmarshal(b, &r) != nil || r.Outcome == nil {
		t.Fatal("original report encoding")
	}
	path := filepath.Join(filepath.Dir(report), "inventory-0", r.Outcome.EnvelopeSHA256)
	info, e := os.Stat(path)
	if e != nil || info.Size() > 16<<20 {
		t.Fatal("original observation bound", e)
	}
	raw, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	if pilot.Digest(raw) != r.Outcome.EnvelopeSHA256 || uint64(len(raw)) != r.Outcome.ByteLength {
		t.Fatal("original digest/length")
	}
	if _, _, e = prep.ValidateAutomaticPublicationObservationPrivateV2(context.Background(), raw, r.Configuration.Preparation, r.Configuration.Post); e != nil {
		t.Fatal(e)
	}
	t.Logf("ASSERT_ORIGINAL_COHORT_PURE_OBSERVATION: PASS bytes=%d digest=%s; no owner, claim or physical transport authentication", len(raw), pilot.Digest(raw))
}

func TestCohortApplicationReceiptPreviewPrivate(t *testing.T) {
	python, e := exec.LookPath("python3")
	if e != nil {
		t.Fatal(e)
	}
	base, _, config, args := inventoryTwoFixturePrivate(t)
	c, e := loadCensusInventoryConfigPrivate(config)
	if e != nil {
		t.Fatal(e)
	}
	c.MaxOrigins = 1
	c.Policy.MaxNominationFiles = 6
	c.Policy.Discovery.MaxFiles = 6
	directory := filepath.Join(base, "application")
	if e = os.Mkdir(directory, 0700); e != nil {
		t.Fatal(e)
	}
	c.Application = &censusApplicationConfigPrivate{censusApplicationTerminalProfilePrivate, directory, 1 << 20, 2 << 20, 128 << 20, 128}
	saveInventoryConfigPrivate(t, config, c)
	x := newTerminalProcessRunnerPrivate(t, base, args)
	run := func(preview bool) (censusInventoryReportPrivate, int, int) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		argv := append([]string{"census"}, x.args...)
		if preview {
			argv = append(argv, "--private-inventory-scope-preview")
		}
		cmd := exec.CommandContext(ctx, x.binary, argv...)
		cmd.Env = append(os.Environ(), "LSP_TRACE_OWNED_TWO_ORIGIN_SERVER=1")
		out, e := cmd.Output()
		var meta struct{ AttemptDirectory string }
		if e != nil || json.Unmarshal(out, &meta) != nil || cmd.ProcessState == nil || ctx.Err() != nil {
			t.Fatalf("controlled CLI Wait: %v %s stderr=%s", e, out, func() []byte {
				if ex, ok := e.(*exec.ExitError); ok {
					return ex.Stderr
				}
				return nil
			}())
		}
		b, e := os.ReadFile(filepath.Join(meta.AttemptDirectory, "report.json"))
		if e != nil {
			t.Fatal(e)
		}
		var r censusInventoryReportPrivate
		if json.Unmarshal(b, &r) != nil {
			t.Fatal("report")
		}
		return r, cmd.ProcessState.ExitCode(), cmd.Process.Pid
	}
	first, _, _ := run(true)
	if first.ApplicationBinding == nil || first.ApplicationBinding.Segment != 0 || first.ApplicationBinding.ClaimSHA256 != "" {
		t.Fatal("initial preview")
	}
	for _, name := range []string{"mailbox", "exports", "control", "payloads"} {
		if e = os.Mkdir(filepath.Join(base, name), 0700); e != nil {
			t.Fatal(e)
		}
	}
	cfg := censusCohortFixtureConfig(filepath.Join(base, "mailbox"), first.ScopeSHA256)
	c.CohortAutomation = &cfg
	saveInventoryConfigPrivate(t, config, c)
	helper, e := filepath.Abs(filepath.Join("..", "..", "scripts", "private-variable-v2-host-handoff.py"))
	if e != nil {
		t.Fatal(e)
	}
	done := make(chan error, 1)
	go func() { done <- cohortHostEmulatePrivate(base, cfg.Directory, helper, python) }()
	delivered, code, pid := run(false)
	hostErr := <-done
	if hostErr != nil || code != 0 || delivered.Outcome == nil || delivered.Inventory == nil || delivered.ApplicationBinding.Segment != 0 {
		t.Fatalf("controlled cohort delivery: host=%v failure=%s", hostErr, delivered.Failure)
	}
	applicationHostObservationPrivate(t, delivered, code, pid, true, false)
	c.CohortAutomation = nil
	saveInventoryConfigPrivate(t, config, c)
	next, _, _ := run(true)
	if next.Status != "SCOPE_PREVIEW_METADATA_ONLY_NOT_GENERATION" || next.ApplicationBinding == nil || next.ApplicationBinding.Segment != 1 || next.ApplicationBinding.FullPlanSHA256 != first.ApplicationBinding.FullPlanSHA256 || next.ApplicationBinding.ClaimSHA256 != "" || next.StopCalls != 0 || next.Inventory != nil || next.Outcome != nil {
		t.Fatalf("cohort receipt did not advance nonspending preview: %s %+v", next.Failure, next.ApplicationBinding)
	}
	if _, e = os.Stat(filepath.Join(directory, censusApplicationLeafPrivate(1, "claim"))); !os.IsNotExist(e) {
		t.Fatal("preview spent next claim")
	}
	t.Log("ASSERT_COHORT_RECEIPT_NEXT_SCOPE: PASS controlled stdio original CLI Wait, segment=1, no next claim/session/model; not live gopls")
}

func cohortObservationMutationsPrivate(t *testing.T, raw []byte, c censusInventoryConfigPrivate) {
	t.Helper()
	for _, kind := range []string{"max-zero", "max-64", "nil-cohort", "legacy-cohort", "frame-sha", "frame-length", "physical-length", "physical-sha", "missing-item", "duplicate-item", "swap-items", "wrong-input", "wrong-target", "wrong-attempt", "wrong-predecessor", "wrong-request", "wrong-preparation", "wrong-stage", "wrong-protocol", "raw-malformed", "cross-citation", "stale-review", "missing-frame", "extra-frame", "nav-binding"} {
		t.Run(kind, func(t *testing.T) {
			var envelope map[string]json.RawMessage
			if json.Unmarshal(raw, &envelope) != nil {
				t.Fatal("mutation fixture")
			}
			var inv prep.AutomaticInventoryPrivateV2
			if json.Unmarshal(envelope["Inventory"], &inv) != nil {
				t.Fatal("inventory fixture")
			}
			cf := inv.Cohort
			switch kind {
			case "max-zero":
				cf.MaxItems = 0
			case "max-64":
				cf.MaxItems = 64
			case "nil-cohort":
				inv.Cohort = nil
			case "legacy-cohort":
				inv.Protocol = prep.AutomaticProtocolPrivateV2
			case "frame-sha":
				cf.Frames[0].ResponseSHA256 = pilot.Digest([]byte("foreign"))
			case "frame-length":
				cf.Frames[0].ResponseByteLength++
			case "physical-length":
				cf.Frames[0].PhysicalResponseByteLength = 1
			case "physical-sha":
				cf.Frames[0].PhysicalResponseSHA256 = "bad"
			case "missing-frame":
				cf.Frames = cf.Frames[1:]
				inv.Calls--
			case "extra-frame":
				cf.Frames = append(cf.Frames, cf.Frames[0])
				inv.Calls++
			default:
				index := 0
				if kind == "stale-review" {
					index = 1
				}
				if kind == "nav-binding" {
					index = len(cf.Frames) - 1
				}
				f := &cf.Frames[index]
				if kind == "nav-binding" {
					var v prep.AutomaticReviewPrivateV2
					if json.Unmarshal(f.Response, &v) != nil {
						t.Fatal("navigation fixture")
					}
					v.Digest = pilot.Digest([]byte("stale"))
					f.Response, _ = json.Marshal(v)
				} else {
					var out prep.CohortResponsePrivateV2
					if json.Unmarshal(f.Response, &out) != nil || len(out.Items) == 0 {
						t.Fatal("frame fixture")
					}
					switch kind {
					case "missing-item":
						out.Items = out.Items[1:]
					case "duplicate-item":
						out.Items = append(out.Items, out.Items[0])
					case "swap-items":
						if len(out.Items) < 2 {
							t.Skip("requires two items")
						}
						out.Items[0], out.Items[1] = out.Items[1], out.Items[0]
					case "wrong-input":
						out.Items[0].Binding.InputSHA256 = "foreign"
					case "wrong-target":
						out.Items[0].Binding.Target = "foreign"
					case "wrong-attempt":
						out.Items[0].Binding.Attempt++
					case "wrong-predecessor":
						out.Items[0].Binding.PredecessorDigest = "foreign"
					case "wrong-request":
						out.Items[0].Binding.RequestSHA256 = "foreign"
					case "wrong-preparation":
						out.PreparationSHA256 = "foreign"
					case "wrong-stage":
						out.Stage = "REVIEW_TECHNICAL_COHORT"
					case "wrong-protocol":
						out.Protocol = prep.AutomaticProtocolPrivateV2
					case "cross-citation":
						out.Items[0].Interpretation.Claims[0].Citations[0].Handle = "foreign-body"
					case "stale-review":
						out.Items[0].Review.Digest = "stale"
					}
					f.Response, _ = json.Marshal(out)
					if kind == "raw-malformed" {
						f.Response = json.RawMessage(`[]`)
					}
					f.ItemResponseSHA256 = nil
					for _, v := range out.Items {
						b, _ := json.Marshal(v)
						f.ItemResponseSHA256 = append(f.ItemResponseSHA256, pilot.Digest(b))
					}
				}
				f.ResponseByteLength = len(f.Response)
				f.ResponseSHA256 = pilot.Digest(f.Response)
				// Physical diagnostics have no recomputable original wire here. Clear the
				// optional pair rather than claiming a forged authenticated witness.
				f.PhysicalResponseSHA256 = ""
				f.PhysicalResponseByteLength = 0
			}
			envelope["Inventory"], _ = json.Marshal(inv)
			// Keep canonical envelope FIELD ORDER, including original human render.
			var ordered struct {
				SchemaVersion string
				prep.PrivateCeilings
				Absence, Profile, PolicySHA256, PreparationSHA256 string
				Preparation                                       json.RawMessage
				Ledger                                            json.RawMessage
				Inventory                                         *prep.AutomaticInventoryPrivateV2 `json:",omitempty"`
				HumanInventory                                    string                            `json:",omitempty"`
			}
			if json.Unmarshal(raw, &ordered) != nil {
				t.Fatal("ordered fixture")
			}
			ordered.Inventory = &inv
			changed, _ := json.Marshal(ordered)
			_, _, err := prep.ValidateAutomaticPublicationObservationPrivateV2(context.Background(), changed, c.Preparation, c.Post)
			if err == nil {
				t.Fatalf("ASSERT_COHORT_OBSERVATION_REJECT_%s: accepted", kind)
			}
			// Mutations confined to diagnostics should reach the cohort checker, not
			// pass because of a noncanonical envelope or unrelated rendering failure.
			if kind != "legacy-cohort" && kind != "nil-cohort" && !strings.Contains(err.Error(), "cohort observation") {
				t.Fatalf("mutation masked: %v", err)
			}
			t.Logf("ASSERT_COHORT_OBSERVATION_REJECT_%s: PASS (%v)", kind, err)
		})
	}
}
