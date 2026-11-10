//go:build darwin || linux

package inventorysuccessor

import (
	"context"
	"encoding/json"
	"errors"
	"golang.org/x/sys/unix"
	"os"
	"syscall"
	"testing"
	"time"
)

type providerFaultFixtureV2 struct {
	mode, path           string
	stats, reads, closes int
	cancel               context.CancelFunc
	deadline             time.Time
}

func (f *providerFaultFixtureV2) providerFstat(fd uintptr, st *syscall.Stat_t) error {
	f.stats++
	if f.mode == "stat-eintr" {
		return syscall.EINTR
	}
	return singleLocalDirectoryFstatV1(fd, st)
}
func (f *providerFaultFixtureV2) providerRead(fd uintptr, b []byte) (int, error) {
	f.reads++
	if f.mode == "eof-exhaustion" && f.reads == 1 && len(b) > 1 {
		b = b[:len(b)-1]
	}
	if f.mode == "call-exhaustion" && len(b) > 1 {
		b = b[:1]
	}
	return unix.Read(int(fd), b)
}
func (f *providerFaultFixtureV2) providerClose(file *os.File) error {
	f.closes++
	e := file.Close()
	if file.Name() == f.path {
		if f.mode == "close-failure" {
			e = errors.Join(e, syscall.EIO)
		}
		if f.mode == "cancel-at-close" {
			f.cancel()
		}
		if f.mode == "deadline-at-close" {
			timer := time.NewTimer(time.Until(f.deadline))
			defer timer.Stop()
			<-timer.C
		}
	}
	return e
}
func TestFixedProviderFaultBoundariesV2(t *testing.T) {
	for _, mode := range []string{"normal", "stat-eintr", "close-failure", "cancel-at-close", "deadline-at-close", "eof-exhaustion", "call-exhaustion"} {
		t.Run(mode, func(t *testing.T) {
			o, p, n := providerControlFixtureV2(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			deadline := time.Now().Add(time.Second)
			if mode == "deadline-at-close" {
				deadline = time.Now().Add(50 * time.Millisecond)
			}
			fault := &providerFaultFixtureV2{mode: mode, path: p.Path, cancel: cancel, deadline: deadline}
			r, e := o.verifyProviderControlV2(ctx, hashLabelV2(mode), deadline, p, n, fault)
			h, he := o.k.reopen(context.Background(), o.job, o.limits)
			if he != nil {
				t.Fatal(he)
			}
			if mode == "normal" {
				if e != nil || r.state == nil || h.Disposition != "OBSERVED" || fault.reads != 2 {
					t.Fatalf("ASSERT_RAW_CONTROL_SUCCESS: %v reads=%d", e, fault.reads)
				}
				raw, je := json.Marshal(r)
				var forged PrivateProviderControlV2
				if je != nil || json.Unmarshal(raw, &forged) != nil || forged.CheckCurrent(ctx) == nil {
					t.Fatal("ASSERT_CONTROL_JSON_NO_CUSTODY")
				}
			} else {
				if e == nil || r.state != nil || h.Disposition != "PENDING" || h.Windows != 1 || h.Charged.ReadHashBytes == 0 {
					t.Fatalf("ASSERT_RAW_CONTROL_RETAINED: %s %v %+v", mode, e, h)
				}
				if mode == "stat-eintr" && (fault.stats != 1 || fault.reads != 0) {
					t.Fatalf("ASSERT_SINGLE_FSTAT_EINTR: stats=%d reads=%d", fault.stats, fault.reads)
				}
				if (mode == "eof-exhaustion" || mode == "call-exhaustion") && fault.reads != 2 {
					t.Fatalf("ASSERT_RAW_READ_ATTEMPT_BOUND: %d", fault.reads)
				}
			}
			t.Logf("ASSERT_RAW_BOUNDARY PASS %s stats=%d reads=%d closes=%d", mode, fault.stats, fault.reads, fault.closes)
		})
	}
}
