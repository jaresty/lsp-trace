package managedprocess

import (
	"context"
	"encoding/binary"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
	"unsafe"
)

// All adapters below are in-memory. These tests never construct a Process,
// command, native provider, live proc reader, or SELF helper.
type linuxFixture struct {
	now            time.Time
	calls          [12]int
	clocks         int
	probes         []linuxCensusErrno
	magic          uint64
	selfGroup      int32
	pid            int32
	readNo         int
	batches        [][]byte
	dirNo          int
	primary        func(int32) []byte
	readErr        linuxCensusErrno
	dirErr         linuxCensusErrno
	closeFD        int
	closeErr       linuxCensusErrno
	more           int
	openErr        linuxCensusErrno
	badRead        int
	expireOnRead   bool
	requests       int64
	errs           [10]linuxCensusErrno
	badDir         int
	advanceOnProbe time.Duration
	live           [3]bool
	peak           int
}

const (
	fProbe = iota
	fSelf
	fGroup
	fRoot
	fVerify
	fPID
	fStat
	fRead
	fDir
	fClose
)

func (f *linuxFixture) Now() time.Time { f.clocks++; return f.now }
func (f *linuxFixture) ProbeGroup(p int32) linuxCensusErrno {
	if p != 99 {
		panic("not stored target group")
	}
	n := f.calls[fProbe]
	f.calls[fProbe]++
	f.now = f.now.Add(f.advanceOnProbe)
	if n < len(f.probes) {
		return f.probes[n]
	}
	return linuxEPERM
}
func (f *linuxFixture) SelfPID() (int32, linuxCensusErrno) { f.calls[fSelf]++; return 7, f.errs[fSelf] }
func (f *linuxFixture) acquire(slot int) {
	if f.live[slot] {
		panic("descriptor slot reused while held")
	}
	f.live[slot] = true
	n := 0
	for _, held := range f.live {
		if held {
			n++
		}
	}
	if n > f.peak {
		f.peak = n
	}
}
func (f *linuxFixture) SelfPGID(p int32) (int32, linuxCensusErrno) {
	if p != 7 {
		panic("not numeric self")
	}
	f.calls[fGroup]++
	return f.selfGroup, f.errs[fGroup]
}
func (f *linuxFixture) OpenRoot() (int, linuxCensusErrno) {
	f.calls[fRoot]++
	f.acquire(0)
	return 10, f.openErr
}
func (f *linuxFixture) VerifyProcFS(fd int) (uint64, linuxCensusErrno) {
	if fd != 10 {
		panic("wrong root")
	}
	f.calls[fVerify]++
	return f.magic, f.errs[fVerify]
}
func (f *linuxFixture) OpenPID(fd int, name []byte) (int, linuxCensusErrno) {
	if fd != 10 {
		panic("wrong root")
	}
	p, err := strconv.ParseInt(string(name), 10, 32)
	if err != nil || p <= 0 {
		panic("non-numeric PID")
	}
	f.pid = int32(p)
	f.calls[fPID]++
	f.acquire(1)
	return 20, f.errs[fPID]
}
func (f *linuxFixture) OpenStat(fd int) (int, linuxCensusErrno) {
	if fd != 20 {
		panic("wrong pid fd")
	}
	f.calls[fStat]++
	f.readNo = 0
	f.acquire(2)
	return 30, f.errs[fStat]
}
func (f *linuxFixture) Read(fd int, dst []byte) (int, linuxCensusErrno) {
	if fd != 30 {
		panic("wrong stat fd")
	}
	f.calls[fRead]++
	f.requests += int64(len(dst))
	f.readNo++
	if f.expireOnRead {
		f.now = f.now.Add(time.Second)
	}
	if f.badRead != 0 {
		return f.badRead, 0
	}
	if f.readErr != 0 {
		return 0, f.readErr
	}
	if f.readNo == 2 {
		return f.more, 0
	}
	b := fixtureStat(f.pid, 99, "worker", 55)
	if f.pid == 7 {
		b = fixtureStat(7, 8, "supervisor", 22)
	}
	if f.primary != nil {
		b = f.primary(f.pid)
	}
	return copy(dst, b), 0
}
func (f *linuxFixture) ReadDir(fd int, dst []byte) (int, linuxCensusErrno) {
	if fd != 10 {
		panic("wrong root fd")
	}
	f.calls[fDir]++
	f.requests += int64(len(dst))
	if f.dirErr != 0 {
		return 0, f.dirErr
	}
	if f.badDir != 0 {
		return f.badDir, 0
	}
	i := f.dirNo
	f.dirNo++
	if i >= len(f.batches) {
		return 0, 0
	}
	return copy(dst, f.batches[i]), 0
}
func (f *linuxFixture) Close(fd int) linuxCensusErrno {
	f.calls[fClose]++
	if fd == f.closeFD && f.closeErr != 0 {
		return f.closeErr
	}
	f.live[fd/10-1] = false
	return 0
}
func fixtureStat(pid, pgid int32, comm string, start uint64) []byte {
	return []byte(fmt.Sprintf("%d (%s) S 1 %d 1 0 -1 0 0 0 0 0 0 0 -1 -1 20 0 1 0 %d\n", pid, comm, pgid, start))
}
func fixtureDir(names ...string) []byte {
	var out []byte
	for _, name := range names {
		n := (19 + len(name) + 1 + 7) &^ 7
		b := make([]byte, n)
		binary.LittleEndian.PutUint16(b[16:18], uint16(n))
		b[18] = 4
		copy(b[19:], name)
		out = append(out, b...)
	}
	return out
}
func newLinuxFixture(t *testing.T) (*linuxCensusEngine, *linuxFixture) {
	t.Helper()
	f := &linuxFixture{now: time.Unix(1000, 0), magic: linuxProcMagic, selfGroup: 8, closeFD: -1}
	m := &Manager{}
	b := &privateProviderWindow{ctx: context.Background(), manager: m, profile: m.PrivateProviderWindowProfile(), deadline: f.now.Add(time.Second)}
	e, err := b.bindLinuxCensus(99, f)
	if err != nil {
		t.Fatal(err)
	}
	return e, f
}
func observeLinux(e *linuxCensusEngine, p linuxCensusPhase) linuxCensusResult {
	return e.observe(e.binding, p)
}
func noLinuxCleanup(t *testing.T, r linuxCensusResult) {
	t.Helper()
	if r.cleanupReady() || r.Group.Err == nil && r.Group.Members == 0 && !r.Group.Truncated {
		t.Fatalf("ASSERT_NO_FALSE_CLEANUP: %+v", r)
	}
}

func TestLinuxCensusAbsenceNoScan(t *testing.T) {
	e, f := newLinuxFixture(t)
	f.probes = []linuxCensusErrno{linuxESRCH}
	r := observeLinux(e, linuxTerminal)
	if !r.cleanupReady() || !r.GroupAbsent || r.Group.Evidence != linuxCensusAbsence || f.calls[fProbe] != 1 || f.calls[fRoot] != 0 || f.calls[fSelf] != 0 {
		t.Fatalf("ASSERT_FAST_ABSENCE: %+v calls=%v", r, f.calls)
	}
	spent := e.remaining
	calls := f.calls
	clocks := f.clocks
	f.now = f.now.Add(time.Hour)
	for _, p := range []linuxCensusPhase{linuxTerminal, linuxReconciliation, linuxTerminal} {
		if got := observeLinux(e, p); !reflect.DeepEqual(got, r) {
			t.Fatal("ASSERT_IMMUTABLE_HISTORICAL_CACHE")
		}
	}
	if spent != e.remaining || calls != f.calls || clocks != f.clocks {
		t.Fatal("ASSERT_CACHE_ZERO_IO")
	}
}
func TestLinuxCensusProcFSIdentity(t *testing.T) {
	for _, magic := range []uint64{0, linuxProcMagic, 0x01021994} {
		t.Run(fmt.Sprint(magic), func(t *testing.T) {
			e, f := newLinuxFixture(t)
			f.magic = magic
			f.probes = []linuxCensusErrno{0, linuxESRCH}
			r := observeLinux(e, linuxTerminal)
			if f.calls[fVerify] != 1 {
				t.Fatal("ASSERT_EXPLICIT_FSTATFS")
			}
			if magic != linuxProcMagic {
				if r.Failure.bits&linuxFailProcFS == 0 || f.calls[fSelf] != 0 || f.calls[fClose] != 1 {
					t.Fatalf("ASSERT_PROCFS_MAGIC_GUARD: %+v calls=%v", r, f.calls)
				}
				noLinuxCleanup(t, r)
			} else if !r.cleanupReady() {
				t.Fatalf("ASSERT_VALID_PROCFS: %+v", r)
			}
		})
	}
}
func TestLinuxCensusProbeAndMembers(t *testing.T) {
	for _, initial := range []linuxCensusErrno{0, linuxEPERM, 5} {
		t.Run(fmt.Sprint(initial), func(t *testing.T) {
			e, f := newLinuxFixture(t)
			f.probes = []linuxCensusErrno{initial, linuxEPERM}
			f.batches = [][]byte{fixtureDir("sys", "11", "11")}
			r := observeLinux(e, linuxTerminal)
			noLinuxCleanup(t, r)
			if initial == 5 {
				if f.calls[fRoot] != 0 || r.Failure.errno != 5 {
					t.Fatal("ASSERT_PROBE_ERROR")
				}
				return
			}
			if r.GroupAbsent || r.Group.Members != 1 || r.Sampled != 1 || r.Group.Err != nil || f.calls[fPID] != 3 {
				t.Fatalf("ASSERT_MEMBER_DEDUP_REPORT: %+v calls=%v", r, f.calls)
			}
		})
	}
	e, f := newLinuxFixture(t)
	f.probes = []linuxCensusErrno{linuxEPERM, linuxEPERM}
	r := observeLinux(e, linuxTerminal)
	noLinuxCleanup(t, r)
	if r.Failure.bits&linuxFailNotAbsent == 0 {
		t.Fatal("ASSERT_EPERM_EMPTY_NOT_ABSENT")
	}
	e, f = newLinuxFixture(t)
	f.probes = []linuxCensusErrno{0, linuxESRCH}
	f.batches = [][]byte{fixtureDir("11")}
	r = observeLinux(e, linuxTerminal)
	if !r.cleanupReady() || r.Sampled != 1 || r.Group.Members != 0 {
		t.Fatal("ASSERT_FINAL_ABSENCE_DISTINCT_FROM_SAMPLE")
	}
}
func TestLinuxCensusFailureHistoryAndTwoSlots(t *testing.T) {
	e, f := newLinuxFixture(t)
	f.probes = []linuxCensusErrno{0, linuxESRCH}
	f.dirErr = 5
	first := observeLinux(e, linuxTerminal)
	noLinuxCleanup(t, first)
	if f.calls[fProbe] != 1 {
		t.Fatal("ASSERT_SCAN_ERROR_NO_REPAIR_PROBE")
	}
	f.dirErr = 0
	second := observeLinux(e, linuxReconciliation)
	if !second.cleanupReady() || !reflect.DeepEqual(e.history[0], first) {
		t.Fatal("ASSERT_SEPARATE_OBSERVATION_RETAINS_ERROR")
	}
	if repeated := observeLinux(e, linuxTerminal); !reflect.DeepEqual(repeated, first) {
		t.Fatal("ASSERT_PHASE_SPECIFIC_HISTORY_IMMUTABLE")
	}
	calls := f.calls
	spent := e.remaining
	observeLinux(e, linuxReconciliation)
	observeLinux(e, linuxTerminal)
	if f.calls != calls || e.remaining != spent || !e.used[0] || !e.used[1] {
		t.Fatal("ASSERT_NO_THIRD_OBSERVATION")
	}
	if _, err := e.binding.window.bindLinuxCensus(99, f); err == nil {
		t.Fatal("ASSERT_NO_REBIND_RESET")
	}
}
func TestLinuxCensusClosePoisonFastAbsence(t *testing.T) {
	e, f := newLinuxFixture(t)
	f.closeFD = 30
	f.closeErr = 5
	f.probes = []linuxCensusErrno{0, linuxESRCH}
	first := observeLinux(e, linuxTerminal)
	noLinuxCleanup(t, first)
	calls := f.calls
	second := observeLinux(e, linuxReconciliation)
	if !second.GroupAbsent || !second.ResourceUncertain || second.cleanupReady() || second.Group.Err == nil {
		t.Fatalf("ASSERT_CLOSE_POISON_FAST_ABSENCE: %+v", second)
	}
	if f.calls[fRoot] != calls[fRoot] || f.calls[fProbe] != calls[fProbe]+1 || !reflect.DeepEqual(first, e.history[0]) {
		t.Fatal("ASSERT_CLOSE_HISTORY_NO_ACQUISITION")
	}
	third := observeLinux(e, linuxTerminal)
	noLinuxCleanup(t, third)
}
func TestLinuxCensusHistoricalCacheCannotDischargeClosePoison(t *testing.T) {
	e, f := newLinuxFixture(t)
	f.probes = []linuxCensusErrno{linuxESRCH}
	historical := observeLinux(e, linuxTerminal)
	if !historical.cleanupReady() {
		t.Fatal("ASSERT_CACHE_CONTROL")
	}
	// Simulate an independently retained resource failure on this same owner.
	// The historical record must remain immutable, while its view is not cleanup.
	e.closeFailure.add(linuxFailClose, linuxStageClose, 5)
	calls := f.calls
	spent := e.remaining
	clocks := f.clocks
	r := observeLinux(e, linuxReconciliation)
	if r.cleanupReady() || !r.GroupAbsent || !r.ResourceUncertain || r.Group.Err == nil {
		t.Fatalf("ASSERT_CLOSE_POISON_CACHE: %+v", r)
	}
	if f.calls != calls || e.remaining != spent || f.clocks != clocks || !reflect.DeepEqual(historical, e.history[0]) {
		t.Fatal("ASSERT_POISON_VIEW_IMMUTABLE_ZERO_IO")
	}
}
func TestLinuxCensusReadAndEOF(t *testing.T) {
	for _, kind := range []string{"valid-short", "positive-continuation", "read-error", "oversized", "negative", "partial-record", "directory-error", "directory-no-eof"} {
		t.Run(kind, func(t *testing.T) {
			e, f := newLinuxFixture(t)
			f.probes = []linuxCensusErrno{0, linuxESRCH}
			switch kind {
			case "positive-continuation":
				f.more = 1
			case "read-error":
				f.readErr = 4
			case "oversized":
				f.badRead = 4097
			case "negative":
				f.badRead = -1
			case "partial-record":
				f.primary = func(int32) []byte { return []byte("7 (part") }
			case "directory-error":
				f.dirErr = 5
			case "directory-no-eof":
				for i := 0; i < 32; i++ {
					f.batches = append(f.batches, fixtureDir("sys"))
				}
			}
			r := observeLinux(e, linuxTerminal)
			quote := fixedLinuxCensusQuote()
			if quote.raw-e.remaining.raw != f.requests || quote.entries-e.remaining.entries != int64(f.calls[fDir]*171) {
				t.Fatal("ASSERT_FAILED_SHORT_REQUESTS_NEVER_REFUNDED")
			}
			if kind == "valid-short" {
				if !r.cleanupReady() || f.calls[fRead] != 2 {
					t.Fatal("ASSERT_SHORT_READ_EXPLICIT_EOF")
				}
				return
			}
			noLinuxCleanup(t, r)
			if kind == "positive-continuation" && r.Failure.bits&linuxFailIncomplete == 0 {
				t.Fatal("ASSERT_POSITIVE_CONTINUATION_REJECTED")
			}
			if kind == "read-error" && f.requests != 4096 {
				t.Fatal("ASSERT_FAILED_READ_REQUEST_CHARGED")
			}
			if f.calls[fProbe] != 1 {
				t.Fatal("ASSERT_FAILED_SCAN_NO_ZERO_PROBE")
			}
		})
	}
}
func TestLinuxCensusMaximumLedger(t *testing.T) {
	e, f := newLinuxFixture(t)
	// 1,024 records in 31 data calls, then explicit EOF on call 32. Repeat for
	// reconciliation. Duplicate entries still charge every record and read.
	for pass := 0; pass < 2; pass++ {
		for i := 0; i < 30; i++ {
			names := make([]string, 34)
			for j := range names {
				names[j] = "11"
			}
			f.batches = append(f.batches, fixtureDir(names...))
		}
		f.batches = append(f.batches, fixtureDir("11", "11", "11", "11"), nil)
	}
	for _, p := range []linuxCensusPhase{linuxTerminal, linuxReconciliation} {
		r := observeLinux(e, p)
		if r.Group.Err != nil || r.Sampled != 1 || r.GroupAbsent {
			t.Fatalf("ASSERT_MAX_LEDGER_RESULT: %+v", r)
		}
	}
	q := fixedLinuxCensusQuote()
	if q.operations != 12378 || q.clocks != 16556 || q.raw != 8660994 || q.work != 282859372 || q.retained != 65536 || q.entries != 10944 {
		t.Fatalf("ASSERT_FIXED_QUOTE: %+v", q)
	}
	want := [12]int{4, 2, 2, 2, 2, 2050, 2050, 4100, 64, 4102, 0, 0}
	if f.calls != want || f.requests != q.raw || int64(f.clocks) != q.clocks || f.peak != 3 || f.live != [3]bool{} {
		t.Fatalf("ASSERT_EXACT_ADAPTER_LEDGER: calls=%v requests=%d clocks=%d", f.calls, f.requests, f.clocks)
	}
	r := e.remaining
	if r.operations != 0 || r.raw != 0 || r.records != 0 || r.reads != 0 || r.directories != 0 || r.entries != 0 || r.clocks != 0 || r.work != 0 || r.observations != 0 || r.retained != 0 {
		t.Fatalf("ASSERT_EXACT_SPENT_RESIDUAL: %+v", r)
	}
}
func TestLinuxCensusExhaustion(t *testing.T) {
	for _, which := range []string{"operations", "raw", "work", "clocks", "entries", "records", "reads", "directories", "observations", "storage", "members"} {
		t.Run(which, func(t *testing.T) {
			e, f := newLinuxFixture(t)
			f.batches = [][]byte{fixtureDir("11")}
			switch which {
			case "operations":
				e.remaining.operations = 0
			case "raw":
				e.remaining.raw = 0
			case "work":
				e.remaining.work = 0
			case "clocks":
				e.remaining.clocks = 0
			case "entries":
				e.remaining.entries = 0
			case "records":
				e.remaining.records = 0
			case "reads":
				e.remaining.reads = 0
			case "directories":
				e.remaining.directories = 0
			case "observations":
				e.remaining.observations = 0
			case "storage":
				e.storagePaid = false
			case "members":
				e.remaining.members = 0
			}
			r := observeLinux(e, linuxTerminal)
			noLinuxCleanup(t, r)
			switch which {
			case "operations", "work", "clocks", "observations", "storage":
				if f.calls[fProbe] != 0 {
					t.Fatal("ASSERT_EXHAUSTION_BEFORE_PROBE")
				}
			case "raw", "reads":
				if f.calls[fRead] != 0 {
					t.Fatal("ASSERT_EXHAUSTION_BEFORE_READ")
				}
			case "entries", "directories":
				if f.calls[fDir] != 0 {
					t.Fatal("ASSERT_EXHAUSTION_BEFORE_GETDENTS")
				}
			case "records":
				if f.calls[fPID] != 0 {
					t.Fatal("ASSERT_EXHAUSTION_BEFORE_RECORD")
				}
			case "members":
				if r.Group.Members != 0 {
					t.Fatal("ASSERT_EXHAUSTION_BEFORE_MEMBER_INSERT")
				}
			}
		})
	}
}
func TestLinuxCensusCloseReservationAndDeadline(t *testing.T) {
	e, f := newLinuxFixture(t)
	e.remaining.operations = 2 // probe leaves one: root needs open+close
	r := observeLinux(e, linuxTerminal)
	noLinuxCleanup(t, r)
	if f.calls[fRoot] != 0 || e.remaining.operations != 1 {
		t.Fatal("ASSERT_CLOSE_RESERVED_BEFORE_ACQUIRE")
	}
	e, f = newLinuxFixture(t)
	f.expireOnRead = true
	r = observeLinux(e, linuxTerminal)
	noLinuxCleanup(t, r)
	if r.Failure.bits&linuxFailDeadline == 0 || f.calls[fClose] != 3 {
		t.Fatalf("ASSERT_RESERVED_CLOSE_AFTER_DEADLINE: %+v %v", r, f.calls)
	}
	e, f = newLinuxFixture(t)
	e.remaining.raw = 1
	r = observeLinux(e, linuxTerminal)
	noLinuxCleanup(t, r)
	if f.calls[fClose] != 3 || f.calls[fRead] != 0 {
		t.Fatal("ASSERT_RESERVED_CLOSE_AFTER_EXHAUSTION")
	}
	e, f = newLinuxFixture(t)
	f.openErr = 5
	r = observeLinux(e, linuxTerminal)
	noLinuxCleanup(t, r)
	if f.calls[fClose] != 1 {
		t.Fatal("ASSERT_ERROR_WITH_FD_CUSTODY")
	}
}
func TestLinuxCensusBindingAndConcurrency(t *testing.T) {
	for _, kind := range []string{"manager", "profile", "deadline", "pgid", "quote", "window", "phase"} {
		t.Run(kind, func(t *testing.T) {
			e, f := newLinuxFixture(t)
			b := e.binding
			p := linuxTerminal
			switch kind {
			case "manager":
				b.manager = &Manager{}
			case "profile":
				b.profile.StderrScratch++
			case "deadline":
				b.deadline = b.deadline.Add(time.Nanosecond)
			case "pgid":
				b.pgid++
			case "quote":
				b.quote.operations++
			case "window":
				b.window = &privateProviderWindow{}
			case "phase":
				p = linuxReconciliation
			}
			before := e.remaining
			noLinuxCleanup(t, e.observe(b, p))
			if f.calls != [12]int{} || f.clocks != 0 || before != e.remaining {
				t.Fatal("ASSERT_BINDING_ZERO_IO")
			}
		})
	}
	e, f := newLinuxFixture(t)
	f.probes = []linuxCensusErrno{linuxESRCH}
	var wg sync.WaitGroup
	for i := 0; i < 64; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if !observeLinux(e, linuxTerminal).cleanupReady() {
				t.Error("ASSERT_CONCURRENT_TERMINAL")
			}
		}()
	}
	wg.Wait()
	if f.calls[fProbe] != 1 || e.remaining.observations != 1 {
		t.Fatal("ASSERT_CONCURRENT_SINGLE_SPEND")
	}
}
func TestLinuxCensusParsers(t *testing.T) {
	valid := fixtureStat(11, 99, "hostile ( )\ncomm with spaces)", ^uint64(0))
	id, ok := parseLinuxStat(valid, 11)
	if !ok || id.pid != 11 || id.pgid != 99 || id.start != ^uint64(0) {
		t.Fatal("ASSERT_LAST_CLOSING_PAREN")
	}
	for _, b := range [][]byte{
		fixtureStat(12, 99, "x", 1), fixtureStat(11, 0, "x", 1), fixtureStat(11, -1, "x", 1),
		[]byte(strings.Replace(string(valid), "18446744073709551615", "18446744073709551616", 1)),
		[]byte(strings.Replace(string(valid), "18446744073709551615", "-1", 1)),
		[]byte(strings.Replace(string(valid), "S 1 99", "S 1 2147483648", 1)),
		[]byte(strings.Replace(string(valid), "-1 -1 20", "-9223372036854775809 -1 20", 1)),
		[]byte(strings.Replace(string(valid), "S 1", "SS 1", 1)), []byte("11 (x) S"), append(valid, 0),
	} {
		if _, ok := parseLinuxStat(b, 11); ok {
			t.Fatalf("ASSERT_STAT_REJECT: %q", b)
		}
	}
	for _, name := range []string{"sys", "11", "11", "."} {
		b := fixtureDir(name)
		got, next, ok := parseLinuxDirent64(b, 0)
		if !ok || string(got) != name || next != len(b) {
			t.Fatal("ASSERT_DIRENT_LAYOUT")
		}
	}
	for _, kind := range []string{"short", "zero-length", "unaligned", "overrun", "no-nul", "empty", "slash"} {
		b := fixtureDir("11")
		switch kind {
		case "short":
			b = b[:20]
		case "zero-length":
			binary.LittleEndian.PutUint16(b[16:18], 0)
		case "unaligned":
			binary.LittleEndian.PutUint16(b[16:18], 23)
		case "overrun":
			binary.LittleEndian.PutUint16(b[16:18], 32)
		case "no-nul":
			for i := 19; i < len(b); i++ {
				b[i] = '1'
			}
		case "empty":
			b[19] = 0
		case "slash":
			b[19] = '/'
		}
		if _, _, ok := parseLinuxDirent64(b, 0); ok {
			t.Fatalf("ASSERT_DIRENT_REJECT: %s", kind)
		}
	}
}
func TestLinuxCensusLimitsAndNamespace(t *testing.T) {
	e, f := newLinuxFixture(t)
	f.selfGroup = 9
	r := observeLinux(e, linuxTerminal)
	noLinuxCleanup(t, r)
	if f.calls[fDir] != 0 || r.Failure.bits&linuxFailStat == 0 {
		t.Fatal("ASSERT_SELF_NAMESPACE_CHECK")
	}
	e, f = newLinuxFixture(t)
	names := make([]string, 65)
	for i := range names {
		names[i] = strconv.Itoa(100 + i)
	}
	f.batches = [][]byte{fixtureDir(names...)}
	r = observeLinux(e, linuxTerminal)
	noLinuxCleanup(t, r)
	if r.Sampled != 64 || r.Failure.bits&linuxFailMembers == 0 {
		t.Fatal("ASSERT_MEMBER_FIXED_CAP")
	}
	e, f = newLinuxFixture(t)
	f.batches = [][]byte{{1, 2, 3}}
	r = observeLinux(e, linuxTerminal)
	noLinuxCleanup(t, r)
	if r.Failure.bits&linuxFailDir == 0 {
		t.Fatal("ASSERT_ENGINE_MALFORMED_DIRENT")
	}
}
func TestLinuxCensusAdapterErrorBoundaries(t *testing.T) {
	for _, stage := range []int{fSelf, fGroup, fVerify, fPID, fStat} {
		t.Run(fmt.Sprint(stage), func(t *testing.T) {
			e, f := newLinuxFixture(t)
			f.errs[stage] = 5
			r := observeLinux(e, linuxTerminal)
			noLinuxCleanup(t, r)
			if r.Failure.errno != 5 || f.live != [3]bool{} || f.calls[fProbe] != 1 {
				t.Fatalf("ASSERT_NORMALIZED_ADAPTER_ERROR: %+v live=%v", r, f.live)
			}
		})
	}
	for _, fd := range []int{10, 20, 30} {
		t.Run(fmt.Sprintf("close-%d", fd), func(t *testing.T) {
			e, f := newLinuxFixture(t)
			f.closeFD, f.closeErr = fd, 4
			r := observeLinux(e, linuxTerminal)
			noLinuxCleanup(t, r)
			if !r.ResourceUncertain || e.closeFailure.errno != 4 || f.peak > 3 {
				t.Fatal("ASSERT_CLOSE_ERROR_RETAINED_NO_RETRY")
			}
			root := f.calls[fRoot]
			observeLinux(e, linuxReconciliation)
			if f.calls[fRoot] != root {
				t.Fatal("ASSERT_POISON_FORBIDS_NEW_ACQUISITION")
			}
		})
	}
	for _, n := range []int{-1, 4097} {
		e, f := newLinuxFixture(t)
		f.badDir = n
		r := observeLinux(e, linuxTerminal)
		noLinuxCleanup(t, r)
		if r.Failure.bits&linuxFailDir == 0 {
			t.Fatal("ASSERT_DIRECTORY_COUNT_BOUNDS")
		}
	}
	e, f := newLinuxFixture(t)
	f.probes = []linuxCensusErrno{0, 5}
	r := observeLinux(e, linuxTerminal)
	noLinuxCleanup(t, r)
	if r.Failure.errno != 5 || f.calls[fProbe] != 2 {
		t.Fatal("ASSERT_FINAL_PROBE_ERROR")
	}
	e, f = newLinuxFixture(t)
	f.primary = func(int32) []byte { return fixtureStat(12, 8, "replacement", 1) }
	r = observeLinux(e, linuxTerminal)
	noLinuxCleanup(t, r)
	if f.calls[fDir] != 0 || r.Failure.bits&linuxFailStat == 0 {
		t.Fatal("ASSERT_ENGINE_PID_DIRECTORY_MATCH")
	}
}

func TestLinuxCensusExactBoundaries(t *testing.T) {
	for _, delta := range []time.Duration{100 * time.Millisecond, -time.Nanosecond} {
		e, f := newLinuxFixture(t)
		f.advanceOnProbe = delta
		r := observeLinux(e, linuxTerminal)
		noLinuxCleanup(t, r)
		if r.Failure.bits&linuxFailDeadline == 0 || f.calls[fRoot] != 0 {
			t.Fatal("ASSERT_CLOCK_BOUNDARY")
		}
	}
	e, f := newLinuxFixture(t)
	f.now = e.binding.deadline
	r := observeLinux(e, linuxTerminal)
	noLinuxCleanup(t, r)
	if f.calls[fProbe] != 0 {
		t.Fatal("ASSERT_EXPIRED_ENTRY_ZERO_IO")
	}
	e, f = newLinuxFixture(t)
	e.remaining.operations = 3
	r = observeLinux(e, linuxTerminal)
	noLinuxCleanup(t, r)
	if f.calls[fRoot] != 1 || f.calls[fVerify] != 0 || f.calls[fClose] != 1 {
		t.Fatal("ASSERT_VERIFY_PRECHARGE_AND_CLOSE")
	}
	e, f = newLinuxFixture(t)
	e.remaining.entries = 170
	r = observeLinux(e, linuxTerminal)
	noLinuxCleanup(t, r)
	if f.calls[fDir] != 0 || e.remaining.entries != 170 {
		t.Fatal("ASSERT_ENTRY_BATCH_RESERVATION")
	}
	e, f = newLinuxFixture(t)
	for i := 0; i < 30; i++ {
		names := make([]string, 34)
		for j := range names {
			names[j] = "11"
		}
		f.batches = append(f.batches, fixtureDir(names...))
	}
	f.batches = append(f.batches, fixtureDir("11", "11", "11", "11", "11"))
	r = observeLinux(e, linuxTerminal)
	noLinuxCleanup(t, r)
	if r.Failure.bits&linuxFailBudget == 0 || f.calls[fPID] != 1025 {
		t.Fatal("ASSERT_NUMERIC_RECORD_CAP")
	}
	e, f = newLinuxFixture(t)
	observeLinux(e, linuxTerminal)
	before := e.remaining
	var wg sync.WaitGroup
	for i := 0; i < 64; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); observeLinux(e, linuxReconciliation) }()
	}
	wg.Wait()
	if f.calls[fProbe] != 4 || f.calls[fRoot] != 2 || e.remaining.observations != 0 || e.remaining.operations >= before.operations {
		t.Fatal("ASSERT_CONCURRENT_RECONCILIATION_SINGLE_SPEND")
	}
}

func TestLinuxCensusStorageAndDarwinUnchanged(t *testing.T) {
	size := unsafe.Sizeof(linuxCensusEngine{}) + unsafe.Sizeof(linuxCensusRun{}) + unsafe.Sizeof(privateProviderWindow{})
	if size > linuxCensusStorage {
		t.Fatalf("ASSERT_FIXED_STORAGE: %d", size)
	}
	if unsafe.Sizeof(linuxCensusFailure{}) > 32 || unsafe.Sizeof(linuxCensusResult{}) > 512 || unsafe.Sizeof(linuxCensusFD{}) > 32 {
		t.Fatal("ASSERT_FIXED_RECORD_LAYOUT")
	}
	t.Logf("ENGINE_STORAGE_BYTES=%d QUOTE=%+v", size, fixedLinuxCensusQuote())
	m := &Manager{available: true}
	p := m.PrivateProviderWindowProfile()
	if p.CensusReadLimit != 4096 || p.StderrReadLimit != 65536 || p.StderrScratch != 4096 || p.LocalDarwin {
		t.Fatal("ASSERT_DARWIN_QUOTE_UNCHANGED")
	}
	if _, err := m.PrivateProviderWindowContext(context.Background(), p, time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC)); err == nil {
		t.Fatal("ASSERT_LINUX_ADMISSION_DISABLED")
	}
}
