//go:build darwin

package inventorysuccessor_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDurableDescribeCompetingProcesses(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	command := func(role string) *exec.Cmd {
		cmd := exec.CommandContext(ctx, exe, "-test.run=^TestDurableDescribeProcessChild$", "-test.v", "-test.count=1")
		cmd.Env = append(os.Environ(), "DURABLE_DESCRIBE_ROLE="+role, "DURABLE_DESCRIBE_STORE="+dir)
		return cmd
	}
	if raw, err := command("A").CombinedOutput(); err != nil {
		t.Fatal(err, string(raw))
	}
	var cmds [2]*exec.Cmd
	var gates [2]io.WriteCloser
	var results [2]bytes.Buffer
	for i := range cmds {
		cmds[i] = command("compete")
		gates[i], err = cmds[i].StdinPipe()
		if err != nil {
			t.Fatal(err)
		}
		cmds[i].Stdout = &results[i]
		cmds[i].Stderr = &results[i]
		if err = cmds[i].Start(); err != nil {
			t.Fatal(err)
		}
		defer cmds[i].Process.Kill()
	}
	for _, gate := range gates {
		if _, err = gate.Write([]byte{1}); err != nil {
			t.Fatal(err)
		}
		_ = gate.Close()
	}
	wins := 0
	for i, cmd := range cmds {
		if err = cmd.Wait(); err != nil {
			t.Fatal(err, results[i].String())
		}
		if strings.Contains(results[i].String(), "ASSERT_DURABLE_PROCESS_SUBMIT: WON") {
			wins++
		}
		t.Logf("submit pid=%d: %s", cmd.ProcessState.Pid(), results[i].String())
	}
	if wins != 1 {
		t.Fatal("ASSERT_DURABLE_OS_PROCESS_ONCE: FAIL", wins)
	}
	if raw, err := command("C").CombinedOutput(); err != nil {
		t.Fatal("ASSERT_DURABLE_OS_PROCESS_REOPEN: FAIL", err, string(raw))
	}
	t.Log("ASSERT_DURABLE_OS_PROCESS_ONCE: PASS")
	t.Log("ASSERT_DURABLE_OS_PROCESS_REOPEN: PASS")
	var receipt durableDescribeTestReceipt
	raw, err := os.ReadFile(filepath.Join(dir, "test-receipt.json"))
	if err != nil || json.Unmarshal(raw, &receipt) != nil {
		t.Fatal("receipt", err)
	}
	time.Sleep(max(0, time.Until(time.Unix(0, receipt.Task.DeadlineUnixNano))) + 10*time.Millisecond)
	if raw, err := command("C").CombinedOutput(); err != nil {
		t.Fatal("ASSERT_DURABLE_EXPIRED_VERIFIED_REOPEN: FAIL", err, string(raw))
	}
	t.Log("ASSERT_DURABLE_EXPIRED_VERIFIED_REOPEN: PASS")
}

func TestDurableDescribeKilledClaim(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	command := func(role string) *exec.Cmd {
		cmd := exec.CommandContext(ctx, exe, "-test.run=^TestDurableDescribeProcessChild$", "-test.v", "-test.count=1")
		cmd.Env = append(os.Environ(), "DURABLE_DESCRIBE_ROLE="+role, "DURABLE_DESCRIBE_STORE="+dir)
		return cmd
	}
	if raw, err := command("A").CombinedOutput(); err != nil {
		t.Fatal(err, string(raw))
	}
	cmd := command("claim-and-wait")
	pipe, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	defer pipe.Close()
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer cmd.Process.Kill()
	scanner := bufio.NewScanner(stdout)
	ready := false
	for scanner.Scan() {
		if scanner.Text() == "CLAIM_SPENT_READY" {
			ready = true
			break
		}
	}
	if !ready {
		_ = cmd.Wait()
		t.Fatal("claim boundary not observed", scanner.Err())
	}
	if err = cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	if err = cmd.Wait(); err == nil {
		t.Fatal("process was not killed")
	}
	if raw, err := command("spent").CombinedOutput(); err != nil {
		t.Fatal("ASSERT_DURABLE_KILLED_CLAIM_SPENT: FAIL", err, string(raw))
	}
	t.Log("ASSERT_DURABLE_KILLED_CLAIM_SPENT: PASS")
}
