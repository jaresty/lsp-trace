//go:build darwin

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	pilot "lsp-trace/internal/censuslineagepilot"
	product "lsp-trace/internal/privatehostproductv3"
)

func TestHostApplicationBridgeDelayedTwenty(t *testing.T) {
	t.Setenv("TEST_FIXTURE_BRIDGE", "1")
	t.Setenv("TEST_FIXTURE_DISCOVERY_DELAY", "1")
	r, path, base := readinessLiteralFixture(t)
	t.Setenv("TEST_FIXTURE_DISCOVERY_MARKER", filepath.Join(base, "discovery-reservation-barriers"))
	readinessCombinedTwenty(t, r, path, base, "chosen/deeper/leaf/twenty.go")
}

func TestHostApplicationBridgeMultiunit(t *testing.T) {
	_, path, base := hostApplicationCLIFixture(t)
	tool := buildApplicationTool(t, base)
	report, e := controlledApplicationRun(t, tool, path, "TEST_HOST_BRIDGE=1", "TEST_COUNT_PHYSICAL=1")
	if e != nil || report.Units != 3 || report.Targets != 20 || report.Calls != 6 {
		t.Fatalf("ASSERT_BRIDGE_MULTIUNIT A-fail report=%+v error=%v", report, e)
	}
	t.Log("ASSERT_BRIDGE_MULTIUNIT A-pass units=3 targets=20 calls=6")
}

// Public deterministic bridge, controlled None_Model OS fixture only. The fake
// executable has no provider/model/source access and cannot grant authority.
func TestHostApplicationBridgePublicCancellation(t *testing.T) {
	r, path, base := hostApplicationCLIFixture(t)
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
	app := product.ApplicationConfigDigest(r.Configuration)
	unit := pilot.Digest([]byte("None_Model controlled bridge fixture"))
	activation := applicationActivation{filepath.Join(r.Configuration.Store, "units", unit[7:], "cohort-mailbox"), unit, 0}
	ac, _ := json.Marshal(activation)
	if e = os.WriteFile(filepath.Join(r.Configuration.Store, "activation-01.json"), ac, 0600); e != nil {
		t.Fatal(e)
	}
	packet, _ := json.Marshal(applicationActivationPacket(app, ticket.RunID, unit, 1, activation))
	fake := filepath.Join(base, "inert-cli.py")
	source := fmt.Sprintf("#!/usr/bin/env python3\nimport os,sys,time\nfd=int(sys.argv[-1]);os.write(fd,%q.encode());os.close(fd)\ntime.sleep(30)\n", string(packet))
	if e = os.WriteFile(fake, []byte(source), 0700); e != nil {
		t.Fatal(e)
	}
	bridge, e := filepath.Abs("../../.pi/skills/private-host-streaming-application/scripts/activation-bridge.py")
	if e != nil {
		t.Fatal(e)
	}
	if override := os.Getenv("TEST_FIXTURE_BRIDGE_OVERRIDE"); override != "" {
		bridge = override
	}
	control := filepath.Join(base, "bridge")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	invoke := func(args ...string) ([]byte, error) {
		return exec.CommandContext(ctx, "python3", append([]string{bridge}, args...)...).CombinedOutput()
	}
	prepared, e := invoke("prepare", control, fake, path, ticket.RunID, app)
	if e != nil {
		t.Fatalf("ASSERT_BRIDGE_PUBLIC_READY A-fail prepare=%s error=%v", prepared, e)
	}
	var b struct{ DeadlineNS int64 }
	json.Unmarshal(prepared, &b)
	owner := exec.CommandContext(ctx, "python3", bridge, "owner", control)
	var ownerOut, ownerErr bytes.Buffer
	owner.Stdout = &ownerOut
	owner.Stderr = &ownerErr
	neighbor := exec.Command("/bin/sleep", "30")
	if e = neighbor.Start(); e != nil {
		t.Fatal(e)
	}
	defer func() { neighbor.Process.Kill(); neighbor.Wait() }()
	if e = owner.Start(); e != nil {
		t.Fatal(e)
	}
	joined := false
	defer func() {
		if !joined {
			invoke("cancel", control)
			owner.Wait()
		}
	}()
	ready, e := invoke("await", control)
	var event struct {
		Schema, Event, RunID, ApplicationSHA256 string
		DeadlineNS                              int64
		Packet                                  startupFixturePacket
	}
	if e != nil || json.Unmarshal(ready, &event) != nil || event.Event != "ACTIVATION_READY" || event.Schema != "private-host-bridge.v1" || event.RunID != ticket.RunID || event.ApplicationSHA256 != app || event.DeadlineNS != b.DeadlineNS || event.Packet.UnitID != unit || owner.Process.Signal(syscall.Signal(0)) != nil {
		t.Fatalf("ASSERT_BRIDGE_PUBLIC_READY A-fail ready=%s error=%v owner=%s stderr=%s", ready, e, ownerOut.String(), ownerErr.String())
	}
	t.Log("ASSERT_BRIDGE_PUBLIC_READY A-pass immutable_packet_before_owner_return=true")
	cancelRaw, e := invoke("cancel", control)
	if e != nil {
		t.Fatal(e, string(cancelRaw))
	}
	e = owner.Wait()
	joined = true
	var completion struct {
		Event, Status, OwnerAgentReturn    string
		WrapperJoined, ActivationCommitted bool
	}
	if e != nil || json.Unmarshal(ownerOut.Bytes(), &completion) != nil || completion.Event != "OWNER_COMMAND_FINISHED" || !completion.WrapperJoined || !completion.ActivationCommitted || completion.OwnerAgentReturn != "UNOBSERVED" || neighbor.Process.Signal(syscall.Signal(0)) != nil {
		t.Fatalf("ASSERT_BRIDGE_PUBLIC_CANCEL A-fail result=%s stderr=%s error=%v", ownerOut.String(), ownerErr.String(), e)
	}
	t.Log("ASSERT_BRIDGE_PUBLIC_CANCEL A-pass actual_owner_command_joined=true AgentReturn=UNOBSERVED neighbor_survived=true")
}
