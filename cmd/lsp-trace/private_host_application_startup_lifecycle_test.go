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
	"strings"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
	pilot "lsp-trace/internal/censuslineagepilot"
	product "lsp-trace/internal/privatehostproductv3"
)

func TestHostApplicationStartupDescriptorLifetime(t *testing.T) {
	for _, kind := range []string{"valid", "closed-reader", "full-pipe", "cancelled", "read-end", "standard", "named-fifo", "unlinked-fifo", "oversized-field"} {
		t.Run(kind, func(t *testing.T) {
			read, write, e := os.Pipe()
			if e != nil {
				t.Fatal(e)
			}
			defer read.Close()
			defer write.Close()
			inherited, e := unix.Dup(int(write.Fd()))
			if e != nil {
				t.Fatal(e)
			}
			write.Close()
			value := fmt.Sprint(inherited)
			if kind == "read-end" {
				value = fmt.Sprint(read.Fd())
			}
			if kind == "standard" {
				value = "1"
			}
			if kind == "named-fifo" || kind == "unlinked-fifo" {
				path := filepath.Join(t.TempDir(), "fifo")
				if e = unix.Mkfifo(path, 0600); e != nil {
					t.Fatal(e)
				}
				fd, er := unix.Open(path, unix.O_RDWR|unix.O_NONBLOCK, 0)
				if er != nil {
					t.Fatal(er)
				}
				defer unix.Close(fd)
				value = fmt.Sprint(fd)
				if kind == "unlinked-fifo" {
					writeFD, er := unix.Open(path, unix.O_WRONLY|unix.O_NONBLOCK, 0)
					if er != nil {
						t.Fatal(er)
					}
					defer unix.Close(writeFD)
					if er = os.Remove(path); er != nil {
						t.Fatal(er)
					}
					value = fmt.Sprint(writeFD)
				}
			}
			n, e := openApplicationStartupNotifier(value)
			invalid := kind == "read-end" || kind == "standard" || kind == "named-fifo" || kind == "unlinked-fifo"
			if invalid {
				unix.Close(inherited)
				if e == nil {
					n.close()
					t.Fatal("ASSERT_STARTUP_INVALID_DESCRIPTOR A-fail")
				}
				t.Log("ASSERT_STARTUP_INVALID_DESCRIPTOR A-pass")
				return
			}
			if e != nil {
				t.Fatal(e)
			}
			flags, e := unix.FcntlInt(n.file.Fd(), unix.F_GETFD, 0)
			if e != nil || flags&unix.FD_CLOEXEC == 0 {
				t.Fatal("CLOEXEC missing")
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if kind == "closed-reader" {
				read.Close()
			}
			if kind == "full-pipe" {
				fd := int(n.file.Fd())
				unix.SetNonblock(fd, true)
				for {
					_, er := unix.Write(fd, make([]byte, 512))
					if er == unix.EAGAIN {
						break
					}
					if er != nil {
						t.Fatal(er)
					}
				}
			}
			if kind == "cancelled" {
				cancel()
			}
			d := pilot.Digest([]byte("binding"))
			p := applicationStartupPacket{"private-application-startup.v1", "ACTIVATION_READY", d, d, d, d, 1}
			if kind == "oversized-field" {
				p.RunID = strings.Repeat("a", 513)
			}
			start := time.Now()
			e = n.emit(ctx, p)
			if (e == nil) != (kind == "valid") || n.file != nil || time.Since(start) > time.Second {
				t.Fatalf("ASSERT_STARTUP_FINITE_ONCE_CLOSE A-fail kind=%s error=%v", kind, e)
			}
			if e = n.emit(context.Background(), p); e != nil {
				t.Fatal("second activation retried notification", e)
			}
			if kind == "valid" {
				raw, er := io.ReadAll(io.LimitReader(read, 513))
				if er != nil || len(raw) > 512 || bytes.Count(raw, []byte("ACTIVATION_READY")) != 1 {
					t.Fatal("duplicate or missing packet")
				}
			}
			t.Logf("ASSERT_STARTUP_FINITE_ONCE_CLOSE A-pass kind=%s", kind)
		})
	}
}

func TestHostApplicationRawEndpointClassifier(t *testing.T) {
	wrapper, err := filepath.Abs("../../.pi/skills/private-host-streaming-application/scripts/activation-host.py")
	if err != nil {
		t.Fatal(err)
	}
	script := `import ctypes,errno,importlib.util,os,socket,sys,tempfile
spec=importlib.util.spec_from_file_location("owned_wrapper",sys.argv[1]);m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m)
r,w=os.pipe()
assert m.anonymous_pipe(r,os.O_RDONLY) and m.anonymous_pipe(w,os.O_WRONLY), "ASSERT_RAW_ANONYMOUS"
assert not m.anonymous_pipe(r,os.O_WRONLY), "ASSERT_RAW_ACCESS"
with tempfile.TemporaryDirectory() as d:
 p=os.path.join(d,"fifo");os.mkfifo(p,0o600);keeper=os.open(p,os.O_RDWR|os.O_NONBLOCK)
 rd=os.open(p,os.O_RDONLY|os.O_NONBLOCK);wr=os.open(p,os.O_WRONLY|os.O_NONBLOCK);os.unlink(p)
 for fd,access in ((keeper,os.O_RDONLY),(keeper,os.O_WRONLY),(rd,os.O_RDONLY),(wr,os.O_WRONLY)):
  assert not m.anonymous_pipe(fd,access), "ASSERT_RAW_NAMED_DOMAIN"
 for fd in (keeper,rd,wr):os.close(fd)
 with open(os.path.join(d,"regular"),"w+b") as f:assert not m.anonymous_pipe(f.fileno(),os.O_RDONLY), "ASSERT_RAW_REGULAR"
a,b=socket.socketpair();assert not m.anonymous_pipe(a.fileno(),os.O_RDONLY), "ASSERT_RAW_SOCKET";a.close();b.close()
with open(os.devnull,"rb") as f:assert not m.anonymous_pipe(f.fileno(),os.O_RDONLY), "ASSERT_RAW_DEVICE"
original=m.sys.platform;m.sys.platform="unsupported";assert not m.anonymous_pipe(r,os.O_RDONLY), "ASSERT_RAW_PLATFORM";m.sys.platform=original
class Query:
 def __call__(self,*args):ctypes.set_errno(errno.EPERM);return -1
class Lib:
 fstatfs=Query()
original=m.ctypes.CDLL;m.ctypes.CDLL=lambda *a,**k:Lib()
assert not m.anonymous_pipe(r,os.O_RDONLY), "ASSERT_RAW_QUERY_FAIL_CLOSED"
m.ctypes.CDLL=original;os.close(r);os.close(w)
print("ASSERT_RAW_CLASSIFIER A-pass Darwin actual FIFO/pipe; unsupported/query fail closed")
`
	cmd := exec.Command("python3", "-c", script, wrapper)
	raw, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ASSERT_RAW_CLASSIFIER A-fail %v %s", err, raw)
	}
	t.Log(string(raw))
}

// These are OS ownership fixtures, not native source/capture or model evidence.
// The real wrapper receives an actual setup/header/ticket but an owned inert
// executable supplies malformed/EOF/lifecycle schedules. No product hooks.
func TestHostApplicationStartupWrapperLifecycle(t *testing.T) {
	for _, kind := range []string{"eof", "cancel", "broken-reader", "timeout", "foreign-app", "foreign-run", "duplicate-key", "oversized", "short", "lease-data", "shell-eof", "stderr-alias", "extra-writer-alias", "named-input-rdwr", "unlinked-input-rdwr", "unlinked-input-readonly", "unlinked-output-rdwr", "unlinked-output-writeonly", "root-swap", "sigint", "sigterm", "sighup", "fork-term"} {
		t.Run(kind, func(t *testing.T) {
			r, path, base := hostApplicationCLIFixture(t)
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
			app := product.ApplicationConfigDigest(r.Configuration)
			unit := pilot.Digest([]byte("owned OS fixture only"))
			activation := applicationActivation{filepath.Join(r.Configuration.Store, "units", unit[7:], "cohort-mailbox"), unit, 0}
			ac, _ := json.Marshal(activation)
			if e = os.WriteFile(filepath.Join(r.Configuration.Store, "activation-01.json"), ac, 0600); e != nil {
				t.Fatal(e)
			}
			packet, _ := json.Marshal(applicationActivationPacket(app, setup.RunID, unit, 1, activation))
			if kind == "foreign-app" {
				packet = bytes.ReplaceAll(packet, []byte(app), []byte(pilot.Digest([]byte("foreign"))))
			}
			if kind == "foreign-run" {
				packet = bytes.ReplaceAll(packet, []byte(setup.RunID), []byte(pilot.Digest([]byte("foreign run"))))
			}
			if kind == "duplicate-key" {
				packet = append([]byte(`{"Event":"ACTIVATION_READY",`), packet[1:]...)
			}
			if kind == "oversized" {
				packet = bytes.Repeat([]byte("x"), 513)
			}
			if kind == "short" {
				packet = []byte(`{"Schema":`)
			}
			packetPath := filepath.Join(base, "packet")
			os.WriteFile(packetPath, packet, 0600)
			marker := filepath.Join(base, "owned-pid")
			fake := filepath.Join(base, "owned-cli.py")
			fakeSource := "#!/usr/bin/env python3\nimport os,sys,time,signal\nwith open(os.environ['OWNED_MARKER'],'x') as f:f.write(str(os.getpid()))\n"
			if kind == "root-swap" {
				fakeSource += "for n in range(500):\n if os.path.exists(os.environ['OWNED_MARKER']+'.release'):break\n time.sleep(.01)\n"
			}
			if kind == "fork-term" {
				fakeSource += "fd=int(sys.argv[-1]);os.close(fd)\npid=os.fork()\nif pid==0:time.sleep(30);sys.exit(0)\ndef term(s,f):\n os.waitpid(pid,0);sys.exit(0)\nsignal.signal(signal.SIGTERM,term)\nwith open(os.environ['OWNED_MARKER']+'.fork','x') as f:f.write(str(pid))\n"
			}
			if kind == "eof" {
				fakeSource += "sys.exit(7)\n"
			} else if kind != "cancel" && kind != "timeout" && kind != "lease-data" && kind != "fork-term" && kind != "sigint" && kind != "sigterm" && kind != "sighup" {
				fakeSource += "fd=int(sys.argv[-1]);os.write(fd,open(os.environ['OWNED_PACKET'],'rb').read());os.close(fd)\n"
			}
			fakeSource += "time.sleep(30)\n"
			if e = os.WriteFile(fake, []byte(fakeSource), 0700); e != nil {
				t.Fatal(e)
			}
			wrapper, e := filepath.Abs("../../.pi/skills/private-host-streaming-application/scripts/activation-host.py")
			if e != nil {
				t.Fatal(e)
			}
			if override := os.Getenv("TEST_FIXTURE_WRAPPER_OVERRIDE"); override != "" {
				wrapper = override
			}
			if kind == "timeout" {
				source, er := os.ReadFile(wrapper)
				if er != nil {
					t.Fatal(er)
				}
				needle := []byte("end = started + 270")
				if bytes.Count(source, needle) != 1 {
					t.Fatal("watchdog seam")
				}
				wrapper = filepath.Join(base, "short-watchdog.py")
				os.WriteFile(wrapper, bytes.Replace(source, needle, []byte("end = started + 3"), 1), 0600)
			}
			controlRead, controlWrite, e := os.Pipe()
			if e != nil {
				t.Fatal(e)
			}
			defer controlRead.Close()
			defer controlWrite.Close()
			read, write, e := os.Pipe()
			if e != nil {
				t.Fatal(e)
			}
			defer read.Close()
			defer write.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			logdir := filepath.Join(base, "logs")
			cmd := exec.CommandContext(ctx, "python3", wrapper, fake, path, setup.RunID, app, logdir)
			if kind == "shell-eof" || strings.HasPrefix(kind, "sig") || kind == "fork-term" {
				shellWrapper, er := filepath.Abs("../../.pi/skills/private-host-streaming-application/scripts/activation-host.sh")
				if er != nil {
					t.Fatal(er)
				}
				cmd = exec.CommandContext(ctx, "/bin/sh", shellWrapper, fake, path, setup.RunID, app, logdir)
			}
			cmd.Stdin = controlRead
			cmd.Stdout = write
			cmd.Stderr = &diag
			badFIFO := strings.HasPrefix(kind, "named-") || strings.HasPrefix(kind, "unlinked-")
			if badFIFO {
				fifo := filepath.Join(base, "raw-endpoint")
				if e = unix.Mkfifo(fifo, 0600); e != nil {
					t.Fatal(e)
				}
				keeper, er := unix.Open(fifo, unix.O_RDWR|unix.O_NONBLOCK, 0)
				if er != nil {
					t.Fatal(er)
				}
				defer unix.Close(keeper)
				flags := unix.O_RDWR
				if strings.HasSuffix(kind, "readonly") {
					flags = unix.O_RDONLY
				}
				if strings.HasSuffix(kind, "writeonly") {
					flags = unix.O_WRONLY
				}
				fd, er := unix.Open(fifo, flags|unix.O_NONBLOCK, 0)
				if er != nil {
					t.Fatal(er)
				}
				endpoint := os.NewFile(uintptr(fd), "owned-raw-fifo")
				defer endpoint.Close()
				if strings.HasPrefix(kind, "unlinked-") {
					if er = os.Remove(fifo); er != nil {
						t.Fatal(er)
					}
				}
				if strings.Contains(kind, "input") {
					cmd.Stdin = endpoint
				} else {
					cmd.Stdout = endpoint
				}
			}
			alias := kind == "stderr-alias" || kind == "extra-writer-alias" || badFIFO
			if kind == "stderr-alias" {
				cmd.Stderr = write
			}
			if kind == "extra-writer-alias" {
				cmd.ExtraFiles = []*os.File{write}
			}
			cmd.Env = append(os.Environ(), "OWNED_PACKET="+packetPath, "OWNED_MARKER="+marker)
			// A separate neighboring process must survive every owned-group cleanup.
			neighbor := exec.Command("/bin/sleep", "30")
			if e = neighbor.Start(); e != nil {
				t.Fatal(e)
			}
			defer func() { neighbor.Process.Kill(); neighbor.Wait() }()
			if kind == "broken-reader" {
				read.Close()
			}
			started := time.Now()
			if e = cmd.Start(); e != nil {
				t.Fatal(e)
			}
			write.Close()
			if alias {
				// A short bound allows the bad implementation to start its inert
				// child. Closing the lease then joins it; no timeout orphan remains.
				time.Sleep(300 * time.Millisecond)
				controlWrite.Close()
				// Bound the broken RDWR fixture without relying on its impossible EOF.
				if badFIFO {
					_ = cmd.Process.Signal(syscall.SIGTERM)
				}
				waitErr := cmd.Wait()
				_, markerErr := os.Stat(marker)
				_, receiptErr := os.Stat(filepath.Join(logdir, "process.json"))
				read.SetReadDeadline(time.Now().Add(time.Second))
				raw, readErr := io.ReadAll(io.LimitReader(read, 513))
				_, logErr := os.Stat(logdir)
				if waitErr == nil || !os.IsNotExist(markerErr) || !os.IsNotExist(receiptErr) || !os.IsNotExist(logErr) || readErr != nil || len(raw) != 0 {
					t.Fatalf("ASSERT_HOST_ALIAS_PRESTART A-fail kind=%s started=%v bytes=%d wait=%v read=%v", kind, markerErr == nil, len(raw), waitErr, readErr)
				}
				t.Logf("ASSERT_HOST_ALIAS_PRESTART A-pass kind=%s", kind)
				return
			}
			if kind == "cancel" || kind == "root-swap" || kind == "sigint" || kind == "sigterm" || kind == "sighup" || kind == "fork-term" {
				barrier := marker
				if kind == "fork-term" {
					barrier += ".fork"
				}
				for n := 0; n < 300; n++ {
					if _, er := os.Stat(barrier); er == nil {
						break
					}
					time.Sleep(10 * time.Millisecond)
				}
				if _, er := os.Stat(barrier); er != nil {
					cmd.Process.Kill()
					cmd.Wait()
					t.Fatal("owned lifecycle barrier", er)
				}
				switch kind {
				case "root-swap":
					root := r.Configuration.Store
					if e = os.Rename(root, root+".held"); e != nil {
						t.Fatal(e)
					}
					if e = os.Symlink(root+".held", root); e != nil {
						t.Fatal(e)
					}
					if e = os.WriteFile(marker+".release", []byte{1}, 0600); e != nil {
						t.Fatal(e)
					}
				case "sigint":
					cmd.Process.Signal(syscall.SIGINT)
				case "sigterm", "fork-term":
					cmd.Process.Signal(syscall.SIGTERM)
				case "sighup":
					cmd.Process.Signal(syscall.SIGHUP)
				default:
					controlWrite.Close()
				}
			}
			if kind == "lease-data" {
				controlWrite.Write([]byte{1})
			}
			var relayErr error
			var relayed []byte
			if kind == "shell-eof" {
				read.SetReadDeadline(time.Now().Add(2 * time.Second))
				relayed, relayErr = io.ReadAll(io.LimitReader(read, 513))
				controlWrite.Close()
			}
			e = cmd.Wait()
			raw, er := os.ReadFile(filepath.Join(logdir, "process.json"))
			var receipt struct {
				Status                                       string
				Joined, GroupGoneObserved, ActivationRelayed bool
				CoordinatorExit, WrapperExit                 int
			}
			if er != nil || json.Unmarshal(raw, &receipt) != nil || !receipt.Joined || !receipt.GroupGoneObserved || receipt.ActivationRelayed != (kind == "shell-eof") || e == nil || time.Since(started) > 10*time.Second || neighbor.Process.Signal(syscall.Signal(0)) != nil {
				t.Fatalf("ASSERT_HOST_LIFECYCLE A-fail kind=%s receipt=%s error=%v read=%v diagnostic=%s", kind, raw, e, er, diag.String())
			}
			if kind == "shell-eof" {
				if relayErr != nil || !bytes.Equal(relayed, packet) {
					t.Fatalf("ASSERT_HOST_SHELL_PACKET_EOF A-fail bytes=%d read=%v", len(relayed), relayErr)
				}
				t.Log("ASSERT_HOST_SHELL_PACKET_EOF A-pass before_coordinator_exit=true")
			}
			if kind == "eof" && (receipt.CoordinatorExit != 7 || receipt.WrapperExit != 7 || receipt.Status != "EOF_BEFORE_ACTIVATION") {
				t.Fatal("lost actual process status", string(raw))
			}
			if kind == "timeout" && receipt.Status != "HOST_WATCHDOG" {
				t.Fatal("watchdog not exercised", string(raw))
			}
			t.Logf("ASSERT_HOST_LIFECYCLE A-pass kind=%s receipt=%s", kind, raw)
		})
	}
}
