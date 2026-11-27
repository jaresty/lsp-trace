package managedprocess

// This is an injectable accounting engine, NOT Linux launch admission. No native
// adapter or lifecycle dispatch exists here. The quote covers census only, not
// startup, stderr, wait, or containment. Work units are policy weights, not time.
import (
	"bytes"
	"encoding/binary"
	"errors"
	"time"
)

const (
	linuxCensusRecords = 1024
	linuxCensusDirs    = 32
	linuxCensusBuffer  = 4096
	linuxCensusMembers = 64
	linuxCensusStorage = 65536
	linuxProcMagic     = 0x9fa0
	linuxCensusAbsence = "BOUNDED_LINUX_GROUP_ABSENCE"
	linuxCensusSample  = "BOUNDED_LINUX_MEMBER_SAMPLE"
)

// One additional numeric-self record per observation. Each record has two
// opens, two reads (4096+1 requested bytes), and two reserved closes. VerifyProcFS
// is a separate operation; OpenRoot must not hide it or perform other probes.
// Future funding must prepay raw against BOTH ResponseBytes and WireBytes, plus
// retained and work separately. This quote does not change existing funding.
type linuxCensusQuote struct {
	operations, raw, retained, work, clocks, entries, records, reads, directories, members, observations int64
}

func fixedLinuxCensusQuote() linuxCensusQuote {
	const n, r, d = 2, linuxCensusRecords + 1, linuxCensusDirs
	a := int64(n * (2 + 2 + 2 + 1 + 6*r + d))
	closes := int64(n * (1 + 2*r))
	c := 2*(a-closes) + 2*n
	j := int64(n * d * ((linuxCensusBuffer + 23) / 24))
	q := int64(n * (d*linuxCensusBuffer + r*(linuxCensusBuffer+1)))
	return linuxCensusQuote{a, q, linuxCensusStorage, 32*(q+linuxCensusStorage) + 64*a + 256*j + c,
		c, j, n * r, 2 * n * r, n * d, n * linuxCensusMembers, n}
}

// The future adapter must normalize errors to bounded errno values, not retain
// arbitrary errors/strings. Each method except Now represents exactly one native
// attempt, no retries or hidden work. Read/ReadDir report actual zero-byte EOF as
// (0,0); a positive short read is NOT EOF. FD >= 0 transfers custody even on error.
// Calls are serialized and must not reenter the owning private window.
type linuxCensusErrno uint32

const (
	linuxEPERM linuxCensusErrno = 1
	linuxESRCH linuxCensusErrno = 3
)

type linuxCensusAdapter interface {
	Now() time.Time
	ProbeGroup(pgid int32) linuxCensusErrno
	SelfPID() (int32, linuxCensusErrno)
	SelfPGID(pid int32) (int32, linuxCensusErrno)
	OpenRoot() (int, linuxCensusErrno)
	VerifyProcFS(fd int) (uint64, linuxCensusErrno)
	OpenPID(root int, decimal []byte) (int, linuxCensusErrno)
	OpenStat(pidFD int) (int, linuxCensusErrno)
	Read(fd int, dst []byte) (int, linuxCensusErrno)
	ReadDir(root int, dst []byte) (int, linuxCensusErrno)
	Close(fd int) linuxCensusErrno
}

type linuxCensusPhase uint8

const (
	linuxTerminal linuxCensusPhase = iota
	linuxReconciliation
)

type linuxCensusStage uint8

const (
	linuxStageAdmission linuxCensusStage = iota
	linuxStageProbe
	linuxStageRoot
	linuxStageProcFS
	linuxStageSelfPID
	linuxStageSelfPGID
	linuxStagePID
	linuxStageStat
	linuxStageRead
	linuxStageDir
	linuxStageClose
)
const (
	linuxFailBinding uint64 = 1 << iota
	linuxFailPhase
	linuxFailBudget
	linuxFailDeadline
	linuxFailAdapter
	linuxFailProcFS
	linuxFailStat
	linuxFailDir
	linuxFailIncomplete
	linuxFailMembers
	linuxFailNotAbsent
	linuxFailClose
)

var errLinuxCensus = errors.New("managedprocess: bounded Linux census uncertain")

type linuxCensusFailure struct {
	bits  uint64
	stage linuxCensusStage
	errno linuxCensusErrno
}

func (f *linuxCensusFailure) add(bit uint64, stage linuxCensusStage, errno linuxCensusErrno) {
	if f.bits == 0 {
		f.stage, f.errno = stage, errno
	}
	f.bits |= bit
}

type linuxCensusResult struct {
	Group             GroupCensusObservation
	GroupAbsent       bool
	ResourceUncertain bool
	Sampled           int
	Failure           linuxCensusFailure
}

func (r linuxCensusResult) cleanupReady() bool {
	return r.GroupAbsent && !r.ResourceUncertain && r.Failure.bits == 0 && r.Group.Err == nil && !r.Group.Truncated
}
func (r *linuxCensusResult) fail(bit uint64, stage linuxCensusStage, errno linuxCensusErrno) {
	r.Failure.add(bit, stage, errno)
	r.Group.Err, r.Group.Truncated = errLinuxCensus, true
}

type linuxCensusBinding struct {
	window   *privateProviderWindow
	manager  *Manager
	profile  PrivateProviderWindowProfile
	quote    linuxCensusQuote
	deadline time.Time
	pgid     int32
}
type linuxCensusFD struct {
	fd   int
	held bool
}
type linuxStatIdentity struct {
	pid, pgid int32
	start     uint64
}
type linuxCensusEngine struct {
	binding      linuxCensusBinding
	adapter      linuxCensusAdapter
	remaining    linuxCensusQuote
	used         [2]bool
	history      [2]linuxCensusResult
	absence      int // -1 until an accepted historical absence, never fresh authority
	closeFailure linuxCensusFailure
	fds          [3]linuxCensusFD
	lastClock    time.Time
	storagePaid  bool
	dir          [linuxCensusBuffer]byte
	stat         [linuxCensusBuffer + 1]byte
	decimal      [16]byte
	members      [linuxCensusMembers]linuxStatIdentity
}

func newLinuxCensusEngine(b *privateProviderWindow, pgid int32, a linuxCensusAdapter) *linuxCensusEngine {
	q := fixedLinuxCensusQuote()
	e := &linuxCensusEngine{binding: linuxCensusBinding{b, b.manager, b.profile, q, b.deadline, pgid},
		adapter: a, remaining: q, absence: -1, storagePaid: true}
	// Prepay the entire retained envelope and its initialization work once.
	e.remaining.retained = 0
	e.remaining.work -= 32 * q.retained
	return e
}

// observe is the sole engine entry. It neither resets a residual nor creates a
// context/deadline. The private owner serializes adapter calls and history.
func (e *linuxCensusEngine) observe(binding linuxCensusBinding, phase linuxCensusPhase) linuxCensusResult {
	b := e.binding.window
	b.mu.Lock()
	defer b.mu.Unlock()
	base := linuxCensusResult{Group: GroupCensusObservation{ProcessGroupID: int(e.binding.pgid), Limit: linuxCensusMembers, Bounded: true, Evidence: linuxCensusSample}}
	if binding != e.binding || b.linuxCensus != e || b.manager != binding.manager || b.profile != binding.profile ||
		binding.profile != binding.manager.PrivateProviderWindowProfile() || b.deadline != binding.deadline || !e.storagePaid {
		base.fail(linuxFailBinding, linuxStageAdmission, 0)
		return base
	}
	if phase > linuxReconciliation || phase == linuxReconciliation && !e.used[linuxTerminal] {
		base.fail(linuxFailPhase, linuxStageAdmission, 0)
		return base
	}
	// Resource uncertainty is applied BEFORE returning any cache or fast absence.
	if e.used[phase] {
		return e.view(e.history[phase])
	}
	if e.absence >= 0 {
		return e.view(e.history[e.absence])
	}
	e.used[phase] = true
	r := linuxCensusRun{e: e, result: base, end: binding.deadline}
	if !e.spend(linuxCensusQuote{observations: 1, clocks: 2, work: 2}) {
		r.result.fail(linuxFailBudget, linuxStageAdmission, 0)
	} else {
		now := r.clock()
		if d := now.Add(100 * time.Millisecond); d.Before(r.end) {
			r.end = d
		}
		if r.result.Failure.bits == 0 {
			r.run()
		}
		// Completion sample was reserved at entry, including failure paths.
		r.clock()
	}
	r.result = e.view(r.result)
	e.history[phase] = r.result
	if r.result.cleanupReady() {
		e.absence = int(phase)
	}
	return r.result
}
func (e *linuxCensusEngine) view(r linuxCensusResult) linuxCensusResult {
	if e.closeFailure.bits != 0 {
		r.ResourceUncertain = true
		r.fail(linuxFailClose, e.closeFailure.stage, e.closeFailure.errno)
	}
	return r
}
func (e *linuxCensusEngine) spend(c linuxCensusQuote) bool {
	r := &e.remaining
	if c.operations > r.operations || c.raw > r.raw || c.work > r.work || c.clocks > r.clocks || c.entries > r.entries ||
		c.records > r.records || c.reads > r.reads || c.directories > r.directories || c.members > r.members || c.observations > r.observations {
		return false
	}
	r.operations -= c.operations
	r.raw -= c.raw
	r.work -= c.work
	r.clocks -= c.clocks
	r.entries -= c.entries
	r.records -= c.records
	r.reads -= c.reads
	r.directories -= c.directories
	r.members -= c.members
	r.observations -= c.observations
	return true
}

type linuxCensusRun struct {
	e      *linuxCensusEngine
	result linuxCensusResult
	end    time.Time
}

func (r *linuxCensusRun) clock() time.Time {
	n := r.e.adapter.Now()
	if n.Before(r.e.lastClock) || !n.Before(r.end) || r.e.binding.window.ctx.Err() != nil {
		r.result.fail(linuxFailDeadline, linuxStageAdmission, 0)
	}
	r.e.lastClock = n
	return n
}
func (r *linuxCensusRun) before(stage linuxCensusStage, c linuxCensusQuote) bool {
	if r.result.Failure.bits != 0 {
		return false
	}
	c.operations++
	c.clocks += 2
	c.work += 64*c.operations + 32*c.raw + 256*c.entries + 2
	if !r.e.spend(c) {
		r.result.fail(linuxFailBudget, stage, 0)
		return false
	}
	r.clock()
	return r.result.Failure.bits == 0
}
func (r *linuxCensusRun) after(stage linuxCensusStage, errno linuxCensusErrno) bool {
	r.clock()
	if errno != 0 {
		r.result.fail(linuxFailAdapter, stage, errno)
	}
	return r.result.Failure.bits == 0
}
func (r *linuxCensusRun) probe() linuxCensusErrno {
	if !r.before(linuxStageProbe, linuxCensusQuote{}) {
		return 0
	}
	code := r.e.adapter.ProbeGroup(r.e.binding.pgid)
	r.after(linuxStageProbe, 0)
	if code != 0 && code != linuxEPERM && code != linuxESRCH {
		r.result.fail(linuxFailAdapter, linuxStageProbe, code)
	}
	return code
}
func (r *linuxCensusRun) absent() {
	r.result.GroupAbsent = true
	r.result.Group.Members = 0
	r.result.Group.Evidence = linuxCensusAbsence
}
func (r *linuxCensusRun) run() {
	code := r.probe()
	if r.result.Failure.bits != 0 {
		return
	}
	if code == linuxESRCH {
		r.absent()
		return
	}
	if r.e.closeFailure.bits != 0 {
		r.result = r.e.view(r.result)
		return
	}
	r.scan()
	if r.result.Failure.bits != 0 {
		return
	}
	code = r.probe()
	if r.result.Failure.bits != 0 {
		return
	}
	if code == linuxESRCH {
		r.absent()
		return
	}
	if r.result.Sampled == 0 {
		r.result.fail(linuxFailNotAbsent, linuxStageProbe, code)
	}
}

// Slots are fixed: root=0, PID directory=1, stat=2. Charge a future close before
// acquisition. A failed close leaves the slot uncertain and poisons new opens.
func (r *linuxCensusRun) open(slot int, stage linuxCensusStage, record bool, pid int32) bool {
	if r.e.closeFailure.bits != 0 || r.e.fds[slot].held {
		r.result.fail(linuxFailClose, stage, 0)
		return false
	}
	c := linuxCensusQuote{operations: 1} // reserved close, plus before's open
	if record {
		c.records = 1
	}
	if !r.before(stage, c) {
		return false
	}
	fd, code := -1, linuxCensusErrno(0)
	switch slot {
	case 0:
		fd, code = r.e.adapter.OpenRoot()
	case 1:
		name := linuxPIDBytes(pid, &r.e.decimal)
		fd, code = r.e.adapter.OpenPID(r.e.fds[0].fd, name)
	case 2:
		fd, code = r.e.adapter.OpenStat(r.e.fds[1].fd)
	}
	if fd >= 0 {
		r.e.fds[slot] = linuxCensusFD{fd, true}
	}
	if fd < 0 && code == 0 {
		r.result.fail(linuxFailAdapter, stage, 0)
	}
	return r.after(stage, code)
}
func (r *linuxCensusRun) close(slot int) {
	f := &r.e.fds[slot]
	if !f.held {
		return
	}
	code := r.e.adapter.Close(f.fd) // prepaid; no deadline/exhaustion veto
	if code != 0 {
		r.e.closeFailure.add(linuxFailClose, linuxStageClose, code)
		r.result.fail(linuxFailClose, linuxStageClose, code)
	} else {
		f.held = false
	}
}
func (r *linuxCensusRun) record(pid int32) (linuxStatIdentity, bool) {
	defer r.close(1)
	if !r.open(1, linuxStagePID, true, pid) {
		return linuxStatIdentity{}, false
	}
	defer r.close(2)
	if !r.open(2, linuxStageStat, false, 0) {
		return linuxStatIdentity{}, false
	}
	if !r.before(linuxStageRead, linuxCensusQuote{raw: linuxCensusBuffer, reads: 1}) {
		return linuxStatIdentity{}, false
	}
	n, code := r.e.adapter.Read(r.e.fds[2].fd, r.e.stat[:linuxCensusBuffer])
	if !r.after(linuxStageRead, code) {
		return linuxStatIdentity{}, false
	}
	if n <= 0 || n > linuxCensusBuffer {
		r.result.fail(linuxFailStat, linuxStageRead, 0)
		return linuxStatIdentity{}, false
	}
	if !r.before(linuxStageRead, linuxCensusQuote{raw: 1, reads: 1}) {
		return linuxStatIdentity{}, false
	}
	more, code := r.e.adapter.Read(r.e.fds[2].fd, r.e.stat[linuxCensusBuffer:])
	if !r.after(linuxStageRead, code) {
		return linuxStatIdentity{}, false
	}
	if more != 0 {
		r.result.fail(linuxFailIncomplete, linuxStageRead, 0)
		return linuxStatIdentity{}, false
	}
	id, ok := parseLinuxStat(r.e.stat[:n], pid)
	if !ok {
		r.result.fail(linuxFailStat, linuxStageRead, 0)
	}
	return id, ok
}
func (r *linuxCensusRun) scan() {
	defer r.close(0)
	if !r.open(0, linuxStageRoot, false, 0) {
		return
	}
	if !r.before(linuxStageProcFS, linuxCensusQuote{}) {
		return
	}
	magic, code := r.e.adapter.VerifyProcFS(r.e.fds[0].fd)
	if !r.after(linuxStageProcFS, code) {
		return
	}
	if magic != linuxProcMagic {
		r.result.fail(linuxFailProcFS, linuxStageProcFS, 0)
		return
	}
	if !r.before(linuxStageSelfPID, linuxCensusQuote{}) {
		return
	}
	self, code := r.e.adapter.SelfPID()
	if !r.after(linuxStageSelfPID, code) {
		return
	}
	if self <= 0 {
		r.result.fail(linuxFailStat, linuxStageSelfPID, 0)
		return
	}
	if !r.before(linuxStageSelfPGID, linuxCensusQuote{}) {
		return
	}
	group, code := r.e.adapter.SelfPGID(self)
	if !r.after(linuxStageSelfPGID, code) {
		return
	}
	id, ok := r.record(self)
	if !ok || r.result.Failure.bits != 0 {
		return
	}
	if group <= 0 || id.pgid != group {
		r.result.fail(linuxFailStat, linuxStageSelfPGID, 0)
		return
	}
	records := 0
	for batch := 0; batch < linuxCensusDirs; batch++ {
		if !r.before(linuxStageDir, linuxCensusQuote{raw: linuxCensusBuffer, directories: 1, entries: (linuxCensusBuffer + 23) / 24}) {
			return
		}
		n, code := r.e.adapter.ReadDir(r.e.fds[0].fd, r.e.dir[:])
		if !r.after(linuxStageDir, code) {
			return
		}
		if n < 0 || n > len(r.e.dir) {
			r.result.fail(linuxFailDir, linuxStageDir, 0)
			return
		}
		if n == 0 {
			return
		}
		for at := 0; at < n; {
			name, next, ok := parseLinuxDirent64(r.e.dir[:n], at)
			if !ok {
				r.result.fail(linuxFailDir, linuxStageDir, 0)
				return
			}
			at = next
			pid, ok := linuxPIDName(name)
			if !ok {
				continue
			}
			if records == linuxCensusRecords {
				r.result.fail(linuxFailBudget, linuxStagePID, 0)
				return
			}
			records++
			id, ok := r.record(pid)
			if !ok || r.result.Failure.bits != 0 {
				return
			}
			if id.pgid != r.e.binding.pgid {
				continue
			}
			duplicate := false
			for i := 0; i < r.result.Sampled; i++ {
				if r.e.members[i] == id {
					duplicate = true
					break
				}
			}
			if duplicate {
				continue
			}
			if r.result.Sampled == linuxCensusMembers || !r.e.spend(linuxCensusQuote{members: 1}) {
				r.result.fail(linuxFailMembers, linuxStagePID, 0)
				return
			}
			r.e.members[r.result.Sampled] = id
			r.result.Sampled++
			r.result.Group.Members = r.result.Sampled
		}
	}
	r.result.fail(linuxFailIncomplete, linuxStageDir, 0) // no explicit EOF
}

func linuxPIDBytes(pid int32, dst *[16]byte) []byte {
	i := len(dst)
	for pid > 0 {
		i--
		dst[i] = byte(pid%10) + '0'
		pid /= 10
	}
	return dst[i:]
}
func linuxPIDName(b []byte) (int32, bool) {
	if len(b) == 0 || b[0] == '0' {
		return 0, false
	}
	u, ok := linuxNumber(b, false, 31)
	return int32(u), ok && u > 0
}

// linuxNumber validates syntax and range without string conversions or slices of
// tokens. Its magnitude return is sufficient for signed fields not retained.
func linuxNumber(b []byte, signed bool, bits uint) (uint64, bool) {
	if len(b) == 0 {
		return 0, false
	}
	neg := false
	if b[0] == '-' {
		if !signed {
			return 0, false
		}
		neg = true
		b = b[1:]
	}
	if len(b) == 0 {
		return 0, false
	}
	limit := ^uint64(0)
	if bits < 64 {
		limit = (uint64(1) << bits) - 1
	}
	if signed {
		limit = (uint64(1) << (bits - 1)) - 1
		if neg {
			limit++
		}
	}
	var n uint64
	for _, c := range b {
		if c < '0' || c > '9' || n > (limit-uint64(c-'0'))/10 {
			return 0, false
		}
		n = n*10 + uint64(c-'0')
	}
	return n, true
}
func linuxSpace(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' }
func parseLinuxStat(b []byte, expectedPID int32) (linuxStatIdentity, bool) {
	var id linuxStatIdentity
	open, close := bytes.IndexByte(b, '('), bytes.LastIndexByte(b, ')')
	if open < 2 || close < open || close+3 >= len(b) || b[open-1] != ' ' || b[close+1] != ' ' || bytes.IndexByte(b, 0) >= 0 {
		return id, false
	}
	pid, ok := linuxPIDName(b[:open-1])
	if !ok || pid != expectedPID {
		return id, false
	}
	id.pid = pid
	at := close + 2
	if !((b[at] >= 'A' && b[at] <= 'Z') || (b[at] >= 'a' && b[at] <= 'z')) || !linuxSpace(b[at+1]) {
		return id, false
	}
	at++
	for field := 4; field <= 22; field++ {
		for at < len(b) && linuxSpace(b[at]) {
			at++
		}
		start := at
		for at < len(b) && !linuxSpace(b[at]) {
			at++
		}
		bits, signed := uint(64), false
		switch {
		case field >= 4 && field <= 6:
			bits = 31
		case field == 7 || field == 8:
			bits, signed = 32, true
		case field == 9:
			bits = 32
		case field >= 16 && field <= 21:
			signed = true
		}
		n, ok := linuxNumber(b[start:at], signed, bits)
		if !ok {
			return linuxStatIdentity{}, false
		}
		if field == 5 {
			if n == 0 {
				return linuxStatIdentity{}, false
			}
			id.pgid = int32(n)
		}
		if field == 22 {
			id.start = n
		}
	}
	return id, true
}

// Linux64 little-endian getdents64 only. A portable byte parser, not a syscall.
func parseLinuxDirent64(b []byte, at int) ([]byte, int, bool) {
	if at < 0 || at > len(b) || len(b)-at < 24 {
		return nil, at, false
	}
	n := int(binary.LittleEndian.Uint16(b[at+16 : at+18]))
	if n < 24 || n%8 != 0 || n > len(b)-at {
		return nil, at, false
	}
	name := b[at+19 : at+n]
	z := bytes.IndexByte(name, 0)
	if z <= 0 {
		return nil, at, false
	}
	name = name[:z]
	if bytes.IndexByte(name, '/') >= 0 {
		return nil, at, false
	}
	return name, at + n, true
}
