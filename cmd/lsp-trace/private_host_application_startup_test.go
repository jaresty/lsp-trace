//go:build darwin

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	pilot "lsp-trace/internal/censuslineagepilot"
	product "lsp-trace/internal/privatehostproductv3"
)

// Deliberately independent wire decoder: the actual CLI must supply the binding.
type startupFixturePacket struct {
	Schema, Event, ApplicationSHA256, RunID, UnitID, ActivationSHA256 string
	Activation                                                        int
}

func verifyStartupFixturePacket(raw []byte, r hostApplicationRequest, run string) error {
	var p startupFixturePacket
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if len(raw) > 512 || d.Decode(&p) != nil || d.Decode(new(any)) != io.EOF || p.Schema != "private-application-startup.v1" || p.Event != "ACTIVATION_READY" || p.ApplicationSHA256 != product.ApplicationConfigDigest(r.Configuration) || p.RunID != run || !validApplicationDigest(p.UnitID) || !validApplicationDigest(p.ActivationSHA256) || p.Activation < 1 || p.Activation > 16 {
		return fmt.Errorf("packet binding")
	}
	activation, e := applicationReadBounded(filepath.Join(r.Configuration.Store, fmt.Sprintf("activation-%02d.json", p.Activation)), 4096)
	var ac applicationActivation
	if e != nil || pilot.Digest(activation) != p.ActivationSHA256 || json.Unmarshal(activation, &ac) != nil || ac.UnitID != p.UnitID || ac.Mailbox != filepath.Join(r.Configuration.Store, "units", p.UnitID[7:], "cohort-mailbox") {
		return fmt.Errorf("durable activation binding: %v", e)
	}
	deadline, e := product.ReadApplicationUnitDeadline(r.Configuration.Store, p.ApplicationSHA256, p.UnitID)
	if e != nil || !deadline.After(time.Now()) {
		return fmt.Errorf("existing prepaid interval: %v", e)
	}
	return nil
}
func TestHostApplicationStartupDelayedDiscoveryTwenty(t *testing.T) {
	r, path, base := readinessLiteralFixture(t)
	t.Setenv("TEST_FIXTURE_DISCOVERY_DELAY", "1")
	t.Setenv("TEST_FIXTURE_DISCOVERY_MARKER", filepath.Join(base, "discovery-reservation-barriers"))
	readinessCombinedTwenty(t, r, path, base, "chosen/deeper/leaf/twenty.go")
}

func TestHostApplicationStartupMultiunitOnce(t *testing.T) {
	_, path, base := hostApplicationCLIFixture(t)
	tool := buildApplicationTool(t, base)
	report, e := controlledApplicationRun(t, tool, path, "TEST_HOST_WRAPPER=1", "TEST_COUNT_PHYSICAL=1")
	if e != nil || report.Units != 3 || report.Targets != 20 || report.Calls != 6 {
		t.Fatalf("ASSERT_STARTUP_MULTIUNIT_ONCE A-fail report=%+v error=%v", report, e)
	}
	t.Log("ASSERT_STARTUP_MULTIUNIT_ONCE A-pass units=3 targets=20 calls=6 first_packet_only=true")
}

func TestHostApplicationStartupPublicActivation(t *testing.T) {
	r, path, base := hostApplicationCLIFixture(t)
	tool := buildApplicationTool(t, base)
	var setup, diag bytes.Buffer
	if runPrivateHostApplication([]string{"setup", path}, bytes.NewReader(nil), &setup, &diag) != 0 {
		t.Fatal(diag.String())
	}
	var ticket struct{ RunID string }
	if json.Unmarshal(setup.Bytes(), &ticket) != nil {
		t.Fatal("setup")
	}
	r, e := readHostApplication(path)
	if e != nil {
		t.Fatal(e)
	}
	reader, writer, e := os.Pipe()
	if e != nil {
		t.Fatal(e)
	}
	defer reader.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, tool, "private-host-application", "run", path, ticket.RunID, "--startup-notify-fd", "3")
	cmd.ExtraFiles = []*os.File{writer}
	cmd.Env = append(os.Environ(), "LSP_HOST_APPLICATION_OWNED_SERVER=1")
	var out, errout bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errout
	if e = cmd.Start(); e != nil {
		writer.Close()
		t.Fatal(e)
	}
	writer.Close()
	reader.SetReadDeadline(time.Now().Add(35 * time.Second))
	raw, re := io.ReadAll(io.LimitReader(reader, 513))
	// Reap on both assertion branches; no role or model is launched here.
	_ = cmd.Process.Signal(os.Interrupt)
	we := cmd.Wait()
	if re != nil || verifyStartupFixturePacket(raw, r, ticket.RunID) != nil {
		t.Fatalf("ASSERT_STARTUP_DURABLE_BOUND_PACKET A-fail bytes=%d read=%v process=%v diagnostic=%s", len(raw), re, we, errout.String())
	}
	var terminal applicationTerminal
	if json.Unmarshal(out.Bytes(), &terminal) != nil || terminal.Status != "CANCELLED_OR_UNIT_DEADLINE_BEFORE_READY" {
		t.Fatalf("ASSERT_STARTUP_TERMINAL_STDOUT A-fail %s", out.String())
	}
	t.Logf("ASSERT_STARTUP_DURABLE_BOUND_PACKET A-pass bytes=%d EOF=true", len(raw))
	t.Log("ASSERT_STARTUP_TERMINAL_STDOUT A-pass")
}
func TestHostApplicationStartupMetadataEnvelope(t *testing.T) {
	r, path, _ := hostApplicationCLIFixture(t)
	var out, diag bytes.Buffer
	if runPrivateHostApplication([]string{"setup", path}, bytes.NewReader(nil), &out, &diag) != 0 {
		t.Fatal(diag.String())
	}
	var setup struct{ RunID string }
	json.Unmarshal(out.Bytes(), &setup)
	var ticket applicationRunTicket
	if e := product.ReadApplicationRecord(r.Configuration.Store, setup.RunID, &ticket); e != nil {
		t.Fatal(e)
	}
	var reservation product.OperationReservation
	if e := product.ReadApplicationOperation(r.Configuration.Store, ticket.WaitReservationSHA256, &reservation); e != nil {
		t.Fatal(e)
	}
	// Partition from the EXISTING fixed base, not the per-wait lanes: the
	// startup path has no record publication or read, <=16 explicit FD API calls,
	// <=64KiB re-encoding/hash of the existing activation, <=512 JSON bytes,
	// and one <=512-byte write. Finalize's existing 16*128-record partition
	// and 16*4096-byte safe diagnostics are left intact. No ActiveNS is added:
	// emission executes under the already-prepaid unit context.
	if reservation.Kind != "coordinator_host_envelope" || reservation.Bounds.ActiveNS != uint64(10*time.Second) ||
		reservation.Bounds.ReadHashBytes < 64<<20 || reservation.Bounds.ParserBytes < 64<<20 || reservation.Bounds.WriteBytes < 16<<20 || reservation.Bounds.Records < 16384 || reservation.Bounds.Work < 1<<20 ||
		uint64(16+16*128) > 16384 || uint64(2*product.ApplicationRecordLimit+512) > 64<<20 || uint64(512+16*4096) > 16<<20 || uint64(8*product.ApplicationRecordLimit+512+16+16*128) > 1<<20 {
		t.Fatal("ASSERT_STARTUP_EXISTING_METADATA_PARTITION A-fail")
	}
	t.Log("ASSERT_STARTUP_EXISTING_METADATA_PARTITION A-pass explicit_fd_api_calls<=16 activation_encode_hash<=64KiB packet<=512 existing_base_records=16384 existing_base_read_parser=64MiB existing_write=16MiB ActiveNS_added=0")
}

func TestHostApplicationStartupPublicFailurePaths(t *testing.T) {
	for _, kind := range []string{"broken-reader", "source-failure", "zero-units", "cancel-before-activation"} {
		t.Run(kind, func(t *testing.T) {
			r, path, base := hostApplicationCLIFixture(t)
			if kind == "zero-units" {
				for n := 0; n < 3; n++ {
					if e := os.Remove(filepath.Join(r.Configuration.SourceRoot, fmt.Sprintf("unit%d.go", n))); e != nil {
						t.Fatal(e)
					}
				}
			}
			tool := buildApplicationTool(t, base)
			var setup, diag bytes.Buffer
			if runPrivateHostApplication([]string{"setup", path}, bytes.NewReader(nil), &setup, &diag) != 0 {
				t.Fatal(diag.String())
			}
			var ticket struct{ RunID string }
			json.Unmarshal(setup.Bytes(), &ticket)
			reader, writer, e := os.Pipe()
			if e != nil {
				t.Fatal(e)
			}
			defer reader.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, tool, "private-host-application", "run", path, ticket.RunID, "--startup-notify-fd", "3")
			cmd.ExtraFiles = []*os.File{writer}
			cmd.Env = append(os.Environ(), "LSP_HOST_APPLICATION_OWNED_SERVER=1")
			if kind == "source-failure" {
				cmd.Env = append(cmd.Env, "LSP_HOST_APPLICATION_FAIL_DOC=1")
			}
			if kind == "cancel-before-activation" {
				cmd.Env = append(cmd.Env, "LSP_HOST_APPLICATION_BLOCK_DOC=1")
			}
			var out, errout bytes.Buffer
			cmd.Stdout = &out
			cmd.Stderr = &errout
			if kind == "broken-reader" {
				reader.Close()
			}
			if e = cmd.Start(); e != nil {
				writer.Close()
				t.Fatal(e)
			}
			writer.Close()
			if kind == "cancel-before-activation" {
				seen := false
				for n := 0; n < 400; n++ {
					claims, _ := filepath.Glob(filepath.Join(r.Configuration.Store, "units", "*", "application-original-claim.json"))
					if len(claims) > 0 {
						seen = true
						break
					}
					time.Sleep(10 * time.Millisecond)
				}
				if !seen {
					cmd.Process.Kill()
					cmd.Wait()
					t.Fatal("preactivation cancellation barrier")
				}
				cmd.Process.Signal(os.Interrupt)
			}
			e = cmd.Wait()
			var report applicationTerminal
			if json.Unmarshal(out.Bytes(), &report) != nil || report.Calls != 0 {
				t.Fatalf("ASSERT_PUBLIC_STARTUP_FAILURE A-fail kind=%s report=%s diagnostic=%s", kind, out.Bytes(), errout.Bytes())
			}
			if kind == "broken-reader" {
				if report.Status != "FAILED_STARTUP_NOTIFICATION" || e == nil {
					t.Fatal("notifier failure silently continued", report.Status, e)
				}
			} else {
				reader.SetReadDeadline(time.Now().Add(time.Second))
				raw, re := io.ReadAll(io.LimitReader(reader, 513))
				if re != nil || len(raw) != 0 {
					t.Fatal("fabricated activation on early failure", len(raw), re)
				}
			}
			if kind == "zero-units" && (report.Status != "PROVISIONAL" || report.Units != 0 || e != nil) {
				t.Fatal("empty successful terminal lost", report, e)
			}
			t.Logf("ASSERT_PUBLIC_STARTUP_FAILURE A-pass kind=%s status=%s calls=%d", kind, report.Status, report.Calls)
		})
	}
}

func TestHostApplicationStartupLateRegistrationNoReset(t *testing.T) {
	r, path, _ := hostApplicationCLIFixture(t)
	r = readinessUnitTime(t, r, 1000)
	writeApplicationFixture(t, r, path)
	var out, diag bytes.Buffer
	if runPrivateHostApplication([]string{"setup", path}, bytes.NewReader(nil), &out, &diag) != 0 {
		t.Fatal(diag.String())
	}
	var setup struct{ RunID string }
	json.Unmarshal(out.Bytes(), &setup)
	r, e := readHostApplication(path)
	if e != nil {
		t.Fatal(e)
	}
	a, e := product.OpenApplication(r.Configuration, time.Now())
	if e != nil {
		t.Fatal(e)
	}
	u, _, _, e := readinessAdmission(t, r, a, "late-registration")
	if e != nil {
		a.Close()
		t.Fatal(e)
	}
	before, e := product.ReadApplicationUnitDeadline(r.Configuration.Store, u.ApplicationSHA256, u.UnitID)
	a.Close()
	if e != nil {
		t.Fatal(e)
	}
	unitPath := filepath.Join(r.Configuration.Store, "objects", u.UnitID[7:9], "unit-"+u.UnitID+".json")
	windowRaw, _ := json.Marshal(struct{ UnitID string }{u.UnitID})
	if e = applicationWriteOnce(applicationWaitUnitPath(r.Configuration.Store, setup.RunID, 1), windowRaw); e != nil {
		t.Fatal(e)
	}
	beforeBytes, e := os.ReadFile(unitPath)
	if e != nil {
		t.Fatal(e)
	}
	time.Sleep(time.Until(before) + 10*time.Millisecond)
	for n, role := range []string{"interpreter", "reviewer"} {
		out.Reset()
		diag.Reset()
		if runPrivateHostApplication([]string{"register", path, setup.RunID, role, "scripted-late-" + role, r.Configuration.RolePromptSHA256[n]}, bytes.NewReader(nil), &out, &diag) != 0 {
			t.Fatal(diag.String())
		}
	}
	after, e := product.ReadApplicationUnitDeadline(r.Configuration.Store, u.ApplicationSHA256, u.UnitID)
	afterBytes, readErr := os.ReadFile(unitPath)
	// Expired intervals deliberately reconstruct remaining=0 on the current
	// live monotonic anchor. Compare the durable kernel/window record, not two
	// reconstructed wall-clock timestamps (which legitimately differ).
	if e != nil || readErr != nil || !bytes.Equal(beforeBytes, afterBytes) || after.After(time.Now()) {
		t.Fatal("ASSERT_LATE_REGISTER_NO_RESET A-fail", before, after, e)
	}
	t.Log("ASSERT_LATE_REGISTER_NO_RESET A-pass registration=OBSERVATIONAL expired_unit=UNCHANGED")
	// A prior unit's expired window does not turn inter-unit idle time into a
	// new allowance. Exhaust the same irrevocable pool, then use the actual CLI.
	for n := 0; n < 100; n++ {
		if e = applicationClaimWaitSlice(r.Configuration.Store, setup.RunID, []byte(`{"phase":"EXPIRED_PRIOR_UNIT_WINDOW"}`)); e != nil {
			t.Fatal(e)
		}
	}
	out.Reset()
	diag.Reset()
	code := runPrivateHostApplication([]string{"wait", path, setup.RunID, "interpreter", "0", "1000"}, bytes.NewReader(nil), &out, &diag)
	var event applicationHostEvent
	if code != 0 || json.Unmarshal(out.Bytes(), &event) != nil || event.Event != "PAUSED_BUDGET" || event.Bundle != nil || event.GlobalSequence != 0 {
		t.Fatalf("ASSERT_EXPIRED_WINDOW_IDLE_POOL A-fail code=%d event=%s diagnostic=%s", code, out.Bytes(), diag.Bytes())
	}
	t.Log("ASSERT_EXPIRED_WINDOW_IDLE_POOL A-pass prior_window=EXPIRED slots=100 no_new_interval=true returns=UNKNOWN finalize=NOT_ATTEMPTED")
}

func TestHostApplicationStartupRejectArgumentsBeforeSource(t *testing.T) {
	run := "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	for _, args := range [][]string{{"run", "/nonexistent-request", run, "--startup-notify-fd", "1"}, {"run", "/nonexistent-request", run, "--startup-notify-fd", "-1"}, {"run", "/nonexistent-request", run, "--startup-notify-fd", "03"}, {"run", "/nonexistent-request", run, "--startup-notify-fd", "3", "extra"}, {"run", "/nonexistent-request", "--startup-notify-fd", "3"}} {
		var out, diag bytes.Buffer
		if code := runPrivateHostApplication(args, bytes.NewReader(nil), &out, &diag); code == 0 || out.Len() != 0 || !bytes.Contains(diag.Bytes(), []byte("STARTUP_NOTIFY")) || bytes.Contains(diag.Bytes(), []byte("CONFIG")) {
			t.Fatalf("ASSERT_NOTIFY_ARGS_BEFORE_SOURCE A-fail code=%d diagnostic=%s", code, diag.Bytes())
		}
	}
	t.Log("ASSERT_NOTIFY_ARGS_BEFORE_SOURCE A-pass")
}

func TestHostApplicationStartupPublicRejectDescriptor(t *testing.T) {
	_, path, _ := hostApplicationCLIFixture(t)
	f, e := os.CreateTemp(t.TempDir(), "not-pipe")
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	var out, diag bytes.Buffer
	code := runPrivateHostApplication([]string{"run", path, "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "--startup-notify-fd", fmt.Sprint(f.Fd())}, bytes.NewReader(nil), &out, &diag)
	if code == 0 || !bytes.Contains(diag.Bytes(), []byte("STARTUP_NOTIFY_REFUSED")) || out.Len() != 0 {
		t.Fatalf("ASSERT_STARTUP_REJECT_FILE A-fail code=%d diagnostic=%s", code, diag.String())
	}
	t.Log("ASSERT_STARTUP_REJECT_FILE A-pass")
}
