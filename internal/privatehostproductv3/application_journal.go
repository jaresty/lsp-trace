//go:build darwin || linux

package privatehostproductv3

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"syscall"
	"time"
)

// This namespace deliberately does not use openStore, load, resultNumbers, or
// recursive admission. A current fixed-size slot and exact hash-keyed objects
// are read; historical journal verification is NOT claimed by this admission.
// Same-user storage observations and checksums are not authentication.
type applicationJournal struct {
	root                *os.Root
	file                *os.File
	config              ApplicationConfig
	head                ApplicationHead
	io                  ApplicationIO
	closed, poisoned    bool
	baseRead, baseWrite uint64
	activePage          string
	pageLookups         int
	pendingDerivation   *applicationDerivationContract
}
type applicationRecord struct {
	Head                        ApplicationHead
	Event, Kind, PreviousSHA256 string
}
type applicationObject struct {
	Name string
	Raw  []byte
}
type applicationObjectRef struct {
	Name, SHA256 string
	Bytes        int
}
type applicationTransaction struct {
	Record  applicationRecord
	Objects []applicationObjectRef
}
type applicationUnitLedger struct {
	Binding                UnitBinding
	Allowance              ApplicationBounds
	Retention              ApplicationBounds
	ActiveDeadlineUnixNano int64
	Clock                  applicationClockStamp
	Contract               *applicationDerivationContract
}

// First-return retention has its own prepaid processing envelope. It is not
// a second semantic grant and is not charged again when the bytes return.
func applicationRetentionEnvelope() ApplicationBounds {
	return ApplicationBounds{ReadHashBytes: 4 * ApplicationRecordLimit, WriteBytes: 8 * ApplicationRecordLimit, Records: 32, ParserBytes: 4 * ApplicationRecordLimit, Work: 64}
}

type applicationExchange struct {
	UnitID, RequestSHA256 string
	LocalSequence         uint64
	Reservation           OperationReservation
}

// Every API attempt reserves this explicit conservative owned-IO envelope
// BEFORE indexed object reads, including failed and duplicate attempts. These
// processing-only reservations do not multiply semantic entitlements.
func applicationOverhead() ApplicationBounds {
	return ApplicationBounds{ReadHashBytes: 16 * ApplicationRecordLimit, WriteBytes: 16 * ApplicationRecordLimit, Records: 64, ParserBytes: 16 * ApplicationRecordLimit, Work: 128}
}
func applicationAdd(a, b ApplicationBounds) (ApplicationBounds, bool) {
	out := a
	av := reflect.ValueOf(&out).Elem()
	bv := reflect.ValueOf(b)
	for n := 0; n < av.NumField(); n++ {
		x, y := av.Field(n).Uint(), bv.Field(n).Uint()
		if y > math.MaxUint64-x {
			return ApplicationBounds{}, false
		}
		av.Field(n).SetUint(x + y)
	}
	return out, true
}
func applicationTimes(a ApplicationBounds, n uint64) (ApplicationBounds, bool) {
	out := a
	v := reflect.ValueOf(&out).Elem()
	for i := 0; i < v.NumField(); i++ {
		x := v.Field(i).Uint()
		if n != 0 && x > math.MaxUint64/n {
			return ApplicationBounds{}, false
		}
		v.Field(i).SetUint(x * n)
	}
	return out, true
}
func applicationWithin(a, b ApplicationBounds) bool {
	av, bv := reflect.ValueOf(a), reflect.ValueOf(b)
	for n := 0; n < av.NumField(); n++ {
		if av.Field(n).Uint() > bv.Field(n).Uint() {
			return false
		}
	}
	return true
}
func applicationNoSemantics(b ApplicationBounds) bool {
	return b.Calls == 0 && b.RequestBytes == 0 && b.ResponseBytes == 0 && b.WireBytes == 0 && b.OutputBytes == 0
}
func applicationCanonicalPath(p string) bool {
	return filepath.IsAbs(p) && filepath.Clean(p) == p && len(p) <= 4096
}
func validateApplication(c ApplicationConfig, now time.Time) error {
	if c.Schema != "private-host-application-config.v1" || c.Profile != ApplicationProfile || !applicationCanonicalPath(c.Store) || !applicationCanonicalPath(c.SourceRoot) || c.Store == c.SourceRoot || len(c.RevisionAssertion) == 0 || len(c.RevisionAssertion) > 4096 || c.ApprovalSHA256 != ApplicationConfigDigest(c) {
		return ErrInvalid
	}
	for _, d := range []string{c.SelectionRuleSHA256, c.UnitPolicySHA256, c.ProducerConfigSHA256, c.ProducerExecutableSHA256, c.StorageIdentitySHA256, c.RolePromptSHA256[0], c.RolePromptSHA256[1]} {
		if !digestValue(d) {
			return ErrInvalid
		}
	}
	if c.RolePromptSHA256 != [2]string{hash([]byte(CohortRolePrompt("interpreter"))), hash([]byte(CohortRolePrompt("reviewer")))} {
		return ErrInvalid
	}
	g := c.Grant
	b := g.Ceiling
	if g.Revision != 1 || g.PreviousGrantSHA256 != "" || g.ExpiresUnixNano <= now.UnixNano() || g.BootstrapAttempts < 1 || g.BootstrapAttempts > 1024 || !applicationNoSemantics(g.BootstrapEach) || g.BootstrapEach.ReadHashBytes < 4*ApplicationRecordLimit || g.BootstrapEach.WriteBytes < ApplicationRecordLimit || g.BootstrapEach.ParserBytes < 3*ApplicationRecordLimit || g.BootstrapEach.Records < 8 || g.BootstrapEach.Work < 32 || g.BootstrapEach.ActiveNS == 0 || b.Calls > 16 || b.RequestBytes > 1<<20 || b.ResponseBytes > 64<<10 || b.WireBytes > 8<<20 || b.OutputBytes > 4<<20 || b.ActiveNS > uint64(240*time.Second) {
		return ErrInvalid
	}
	if _, ok := applicationTimes(g.BootstrapEach, g.BootstrapAttempts); !ok {
		return ErrInvalid
	}
	return nil
}
func applicationOpenRoot(path string) (*applicationJournal, error) {
	if privateDir(path) != nil {
		return nil, ErrInvalid
	}
	root, e := os.OpenRoot(path)
	if e != nil {
		return nil, e
	}
	a := &applicationJournal{root: root}
	f, e := root.OpenFile("journal", os.O_RDWR|os.O_APPEND|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0600)
	if e != nil {
		root.Close()
		return nil, e
	}
	a.file = f
	st, e := f.Stat()
	if e != nil || !st.Mode().IsRegular() || st.Mode().Perm() != 0600 || st.Sys().(*syscall.Stat_t).Nlink != 1 || syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB) != nil {
		a.close()
		return nil, ErrInvalid
	}
	return a, nil
}
func createApplication(c ApplicationConfig, now time.Time) (*ApplicationJournal, error) {
	if validateApplication(c, now) != nil {
		return nil, ErrInvalid
	}
	boot, ok := applicationTimes(c.Grant.BootstrapEach, c.Grant.BootstrapAttempts)
	if !ok {
		return nil, ErrInvalid
	}
	reserved, ok := applicationAdd(boot, applicationOverhead())
	if !ok || !applicationWithin(reserved, c.Grant.Ceiling) {
		return nil, ErrPaused
	}
	// Create-only root: an interrupted initialization never becomes a new grant.
	if e := os.Mkdir(c.Store, 0700); e != nil {
		return nil, e
	}
	if privateDir(c.Store) != nil {
		return nil, ErrInvalid
	}
	root, e := os.OpenRoot(c.Store)
	if e != nil {
		return nil, e
	}
	f, e := root.OpenFile("journal", os.O_CREATE|os.O_EXCL|os.O_RDWR|os.O_APPEND|syscall.O_NOFOLLOW, 0600)
	if e != nil {
		root.Close()
		return nil, e
	}
	a := &applicationJournal{root: root, file: f, config: c}
	fail := func(e error) (*ApplicationJournal, error) { a.close(); return nil, e }
	if e = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); e != nil {
		return fail(e)
	}
	for _, dir := range []string{"objects"} {
		if e = root.Mkdir(dir, 0700); e != nil {
			return fail(e)
		}
	}
	// Fixed, create-only root-policy lane. It is read before the larger header.
	policy := make([]byte, 8)
	binary.BigEndian.PutUint64(policy, c.Grant.BootstrapAttempts)
	if e = a.write("bootstrap-policy", policy); e != nil {
		return fail(e)
	}
	if e = a.write("header.json", mustJSON(c)); e != nil {
		return fail(e)
	}
	h := ApplicationHead{Schema: "private-host-application-head.v1", ApplicationSHA256: ApplicationConfigDigest(c), Reserved: reserved, FeatureIdentity: "UNRESOLVED", SourceCompleteness: "UNKNOWN", AbsenceInference: "PROHIBITED"}
	if e = a.append(applicationRecord{Head: h, Kind: "INITIAL_PREPAID_BOOTSTRAP_LANE"}); e != nil {
		return fail(e)
	}
	if e = a.putObject(applicationObject{applicationFrontierName(h.ApplicationSHA256, 0), mustJSON(h)}); e != nil {
		return fail(e)
	}
	if e = a.consumeBootstrap(); e != nil {
		return fail(e)
	}
	if e = a.syncRoot(); e != nil {
		return fail(e)
	}
	return &ApplicationJournal{implementation: a}, nil
}
func openApplication(c ApplicationConfig, now time.Time) (*ApplicationJournal, error) {
	if !applicationCanonicalPath(c.Store) {
		return nil, ErrInvalid
	}
	a, e := applicationOpenRoot(c.Store)
	if e != nil {
		return nil, e
	}
	fail := func(e error) (*ApplicationJournal, error) { a.close(); return nil, e }
	// Charge the prepaid bounded lane BEFORE header parsing, including malformed
	// headers and stale caller attempts. Corrupt root-policy bytes refuse here.
	if e = a.consumeBootstrap(); e != nil {
		return fail(e)
	}
	raw, e := a.read("header.json", ApplicationRecordLimit)
	if e != nil || strict(raw, &a.config) != nil {
		return fail(ErrInvalid)
	}
	policy, e := a.read("bootstrap-policy", 8)
	if e != nil || len(policy) != 8 || a.config.Store != c.Store || binary.BigEndian.Uint64(policy) != a.config.Grant.BootstrapAttempts {
		return fail(ErrInvalid)
	}
	if _, e = a.root.Lstat("pending.json"); e == nil || !os.IsNotExist(e) {
		return fail(ErrApplicationAmbiguous)
	}
	if e = a.loadLast(); e != nil {
		return fail(e)
	}
	if validateApplication(c, now) != nil || !bytes.Equal(raw, mustJSON(c)) || a.head.ApplicationSHA256 != ApplicationConfigDigest(c) || !applicationWithin(a.head.Reserved, c.Grant.Ceiling) {
		return fail(ErrInvalid)
	}
	a.baseRead = a.head.Observed.ReadHashBytes
	a.baseWrite = a.head.Observed.WriteBytes
	return &ApplicationJournal{implementation: a}, nil
}
func (a *applicationJournal) close() error {
	if a == nil || a.closed {
		return nil
	}
	a.closed = true
	var e error
	if a.file != nil {
		e = errors.Join(syscall.Flock(int(a.file.Fd()), syscall.LOCK_UN), a.file.Close())
	}
	if a.root != nil {
		e = errors.Join(e, a.root.Close())
	}
	return e
}
func (a *applicationJournal) syncRoot() error {
	d, e := a.root.Open(".")
	if e != nil {
		return e
	}
	return errors.Join(d.Sync(), d.Close())
}
func (a *applicationJournal) consumeBootstrap() error {
	policy, e := a.read("bootstrap-policy", 8)
	if e != nil || len(policy) != 8 {
		return ErrApplicationAmbiguous
	}
	limit := binary.BigEndian.Uint64(policy)
	if limit < 1 || limit > 1024 {
		return ErrApplicationAmbiguous
	}
	f, e := a.root.OpenFile("bootstrap", os.O_CREATE|os.O_RDWR|os.O_APPEND|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0600)
	if e != nil {
		return e
	}
	defer f.Close()
	st, e := f.Stat()
	if e != nil || !st.Mode().IsRegular() || st.Mode().Perm() != 0600 || st.Sys().(*syscall.Stat_t).Nlink != 1 || st.Size() < 0 || uint64(st.Size()) >= limit {
		return ErrPaused
	}
	n, e := f.Write([]byte{1})
	if e != nil || n != 1 {
		return ErrApplicationAmbiguous
	}
	a.io.WriteBytes++
	return errors.Join(f.Sync(), a.syncRoot())
}
func (a *applicationJournal) read(name string, limit int) ([]byte, error) {
	a.io.RecordLookups++
	b, e := (&store{root: a.root}).read(name, limit)
	a.io.ReadBytes += uint64(len(b))
	if uint64(len(b)) > a.io.MaxReadBuffer {
		a.io.MaxReadBuffer = uint64(len(b))
	}
	return b, e
}
func (a *applicationJournal) write(name string, b []byte) error {
	if len(b) > ApplicationRecordLimit {
		return ErrInvalid
	}
	e := (&store{root: a.root}).write(name, b)
	if e == nil {
		a.io.WriteBytes += uint64(len(b))
	}
	return e
}
func (a *applicationJournal) loadLast() error {
	st, e := a.file.Stat()
	if e != nil || st.Size() < applicationSlotBytes || st.Size()%applicationSlotBytes != 0 {
		return ErrApplicationAmbiguous
	}
	slot := make([]byte, applicationSlotBytes)
	n, e := a.file.ReadAt(slot, st.Size()-applicationSlotBytes)
	if e != nil || n != len(slot) {
		return ErrApplicationAmbiguous
	}
	a.io.ReadBytes += uint64(n)
	if uint64(n) > a.io.MaxReadBuffer {
		a.io.MaxReadBuffer = uint64(n)
	}
	a.io.RecordLookups++
	size := int(binary.BigEndian.Uint32(slot[:4]))
	if size < 1 || size > len(slot)-36 {
		return ErrApplicationAmbiguous
	}
	sum := sha256.Sum256(slot[:len(slot)-32])
	if !bytes.Equal(sum[:], slot[len(slot)-32:]) {
		return ErrApplicationAmbiguous
	}
	for _, b := range slot[4+size : len(slot)-32] {
		if b != 0 {
			return ErrApplicationAmbiguous
		}
	}
	var r applicationRecord
	if strict(slot[4:4+size], &r) != nil || r.Head.Sequence != uint64(st.Size()/applicationSlotBytes-1) || r.Head.Schema != "private-host-application-head.v1" || r.Head.Authority != 0 || r.Head.Accepted || r.Head.FeatureIdentity != "UNRESOLVED" || r.Head.SourceCompleteness != "UNKNOWN" || r.Head.AbsenceInference != "PROHIBITED" {
		return ErrApplicationAmbiguous
	}
	// No higher immutable selector may be silently forgotten by a complete-slot
	// rewind. Direct lookup is O(1) records, independent of application history.
	selector, e := a.read(applicationFrontierName(r.Head.ApplicationSHA256, r.Head.Sequence), ApplicationRecordLimit)
	if e != nil || !bytes.Equal(selector, mustJSON(r.Head)) {
		return ErrApplicationAmbiguous
	}
	if r.Head.Sequence == math.MaxUint64 {
		return ErrApplicationAmbiguous
	}
	if _, e = a.root.Lstat(applicationFrontierName(r.Head.ApplicationSHA256, r.Head.Sequence+1)); !os.IsNotExist(e) {
		return ErrApplicationAmbiguous
	}
	a.head = r.Head
	return nil
}
func (a *applicationJournal) append(r applicationRecord) error {
	raw := mustJSON(r)
	if len(raw) > applicationSlotBytes-36 {
		return ErrInvalid
	}
	slot := make([]byte, applicationSlotBytes)
	binary.BigEndian.PutUint32(slot[:4], uint32(len(raw)))
	copy(slot[4:], raw)
	sum := sha256.Sum256(slot[:len(slot)-32])
	copy(slot[len(slot)-32:], sum[:])
	n, e := a.file.Write(slot)
	a.io.WriteBytes += uint64(n)
	if e != nil || n != len(slot) {
		a.poisoned = true
		return ErrApplicationAmbiguous
	}
	if e = a.file.Sync(); e != nil {
		a.poisoned = true
		return e
	}
	a.head = r.Head
	return nil
}
func applicationObjectName(kind, id string) string {
	return "objects/" + id[7:9] + "/" + kind + "-" + id + ".json"
}
func applicationFrontierName(app string, seq uint64) string {
	return applicationObjectName("frontier", hash([]byte(app+":frontier:"+strconv.FormatUint(seq, 10))))
}
func (a *applicationJournal) putObject(o applicationObject) error {
	if e := a.root.Mkdir(filepath.Dir(o.Name), 0700); e != nil && !os.IsExist(e) {
		return e
	}
	return a.write(o.Name, o.Raw)
}
func (a *applicationJournal) object(kind, id string, v any) error {
	if !digestValue(id) {
		return ErrInvalid
	}
	raw, e := a.read(applicationObjectName(kind, id), ApplicationRecordLimit)
	if e != nil {
		return e
	}
	return strict(raw, v)
}
func (a *applicationJournal) exists(kind, id string) bool {
	_, e := a.root.Lstat(applicationObjectName(kind, id))
	return !os.IsNotExist(e)
}
func (a *applicationJournal) commit(kind, event string, h ApplicationHead, objects ...applicationObject) error {
	if a.closed || a.poisoned || h.Sequence == math.MaxUint64 {
		return ErrApplicationAmbiguous
	}
	h.Sequence = a.head.Sequence + 1
	// Instrumented completed owned IO is a durable lower bound. The current
	// commit cannot authenticate or exactly self-account its own future writes.
	if a.io.ReadBytes > math.MaxUint64-a.baseRead || a.io.WriteBytes > math.MaxUint64-a.baseWrite {
		return ErrApplicationAmbiguous
	}
	h.Observed.ReadHashBytes = a.baseRead + a.io.ReadBytes
	h.Observed.WriteBytes = a.baseWrite + a.io.WriteBytes
	r := applicationRecord{Head: h, Event: event, Kind: kind, PreviousSHA256: hash(mustJSON(a.head))}
	objects = append(objects, applicationObject{applicationFrontierName(h.ApplicationSHA256, h.Sequence), mustJSON(h)})
	if len(objects) > 64 {
		return ErrInvalid
	}
	refs := make([]applicationObjectRef, 0, len(objects))
	for _, o := range objects {
		if len(o.Raw) > ApplicationRecordLimit {
			return ErrInvalid
		}
		refs = append(refs, applicationObjectRef{o.Name, hash(o.Raw), len(o.Raw)})
	}
	tx := applicationTransaction{Record: r, Objects: refs}
	raw, encodingErr := applicationMarshal(tx, ApplicationRecordLimit)
	if encodingErr != nil {
		return encodingErr
	}
	// If any later step fails, pending remains a durable execution refusal. No
	// automatic completion/replay infers a missing source owner or semantic reply.
	if e := a.write("pending.json", raw); e != nil {
		a.poisoned = true
		return e
	}
	a.poisoned = true
	if e := a.append(r); e != nil {
		return e
	}
	for _, o := range objects {
		if e := a.putObject(o); e != nil {
			return e
		}
	}
	if e := a.root.Remove("pending.json"); e != nil {
		return e
	}
	if e := a.syncRoot(); e != nil {
		return e
	}
	a.poisoned = false
	return nil
}
func (a *applicationJournal) charge(kind string, b ApplicationBounds, now time.Time) error {
	if a == nil || a.closed || a.poisoned {
		return ErrApplicationAmbiguous
	}
	if now.UnixNano() >= a.config.Grant.ExpiresUnixNano {
		return ErrPaused
	}
	next, ok := applicationAdd(a.head.Reserved, b)
	if !ok || !applicationWithin(next, a.config.Grant.Ceiling) {
		return ErrPaused
	}
	h := a.head
	h.Reserved = next
	return a.commit(kind, "", h)
}
func (a *applicationJournal) attempt(kind string, now time.Time) error {
	return a.charge("ATTEMPT_"+kind, applicationOverhead(), now)
}
func (a *applicationJournal) reserveOperation(id, kind string, b ApplicationBounds, now time.Time) (OperationReservation, error) {
	if !digestValue(id) || len(kind) == 0 || len(kind) > 64 || !applicationNoSemantics(b) {
		return OperationReservation{}, ErrInvalid
	}
	if e := a.attempt("OPERATION", now); e != nil {
		return OperationReservation{}, e
	}
	if a.exists("claim", id) {
		return OperationReservation{}, ErrApplicationSpent
	}
	total, ok := applicationAdd(a.head.Reserved, b)
	if !ok || !applicationWithin(total, a.config.Grant.Ceiling) {
		return OperationReservation{}, ErrPaused
	}
	r := OperationReservation{EventID: id, Kind: kind, Bounds: b, Sequence: a.head.Sequence + 1}
	h := a.head
	h.Reserved = total
	e := a.commit("RESERVE_OPERATION", id, h, applicationObject{applicationObjectName("claim", id), mustJSON(r)})
	if e == nil && kind == "directory_page" {
		a.activePage = id
		a.pageLookups = 32
	}
	return r, e
}
func validApplicationUnit(c ApplicationConfig, u UnitBinding) bool {
	for _, d := range []string{u.ApplicationSHA256, u.UnitID, u.InputSHA256, u.RevisionSHA256, u.SourceFreezeSHA256, u.ScopeSHA256, u.ProducerConfigSHA256, u.PolicySHA256, u.RootIdentitySHA256} {
		if !digestValue(d) {
			return false
		}
	}
	return (u.CheckpointSHA256 == "" || digestValue(u.CheckpointSHA256)) && u.ApplicationSHA256 == ApplicationConfigDigest(c) && u.ProducerConfigSHA256 == c.ProducerConfigSHA256 && u.PolicySHA256 == c.UnitPolicySHA256
}
func (a *applicationJournal) admitUnit(u UnitBinding, b ApplicationBounds, now time.Time) error {
	if !validApplicationUnit(a.config, u) || b.Calls > 16 || b.ActiveNS == 0 || b.ActiveNS > uint64(240*time.Second) {
		return ErrInvalid
	}
	first, e := applicationClockNow()
	if e != nil {
		return e
	}
	if e := a.attempt("ADMIT_UNIT", now); e != nil {
		return e
	}
	if a.exists("unit", u.UnitID) {
		return ErrApplicationSpent
	}
	// Entitlement is irrevocably reserved here, before a child ledger can exist.
	retention, ok := applicationTimes(applicationRetentionEnvelope(), b.Calls)
	if !ok {
		return ErrInvalid
	}
	prepaid, ok := applicationAdd(b, retention)
	if !ok {
		return ErrInvalid
	}
	total, ok := applicationAdd(a.head.Reserved, prepaid)
	if !ok || !applicationWithin(total, a.config.Grant.Ceiling) {
		return ErrPaused
	}
	h := a.head
	h.Reserved = total
	stamp, e := applicationClockNow()
	if e != nil {
		return e
	}
	deadline, e := applicationAdmissionDeadline(now, b.ActiveNS, a.config.Grant.ExpiresUnixNano, first, stamp)
	if e != nil {
		return e
	}
	// Store the actual sample unchanged. The shortened deadline encodes only
	// remaining prepaid time; wall skew never moves the kernel start forward.
	return a.commit("PREPAY_CHILD", u.UnitID, h, applicationObject{applicationObjectName("unit", u.UnitID), mustJSON(applicationUnitLedger{Binding: u, Allowance: b, Retention: retention, ActiveDeadlineUnixNano: deadline, Clock: stamp, Contract: a.pendingDerivation})})
}
func applicationExchangeID(app, unit string, seq uint64) string {
	return hash([]byte(app + ":" + unit + ":" + strconv.FormatUint(seq, 10)))
}
func (a *applicationJournal) reserveExchange(unit string, seq uint64, digest string, b ApplicationBounds, now time.Time) (OperationReservation, error) {
	if !digestValue(unit) || !digestValue(digest) || seq < 1 || seq > 16 || b.Calls != 1 || b.RequestBytes < 1 || b.RequestBytes > 1<<20 || b.ResponseBytes < 1 || b.ResponseBytes > 64<<10 || b.WireBytes < b.RequestBytes+b.ResponseBytes || b.ActiveNS > uint64(60*time.Second) {
		return OperationReservation{}, ErrInvalid
	}
	if e := a.attempt("EXCHANGE", now); e != nil {
		return OperationReservation{}, e
	}
	var child applicationUnitLedger
	if e := a.object("unit", unit, &child); e != nil {
		return OperationReservation{}, e
	}
	id := applicationExchangeID(a.head.ApplicationSHA256, unit, seq)
	if a.exists("exchange", id) {
		return OperationReservation{}, ErrApplicationSpent
	}
	var used ApplicationBounds
	for n := uint64(1); n < seq; n++ {
		previous := applicationExchangeID(a.head.ApplicationSHA256, unit, n)
		var p applicationExchange
		if a.object("exchange", previous, &p) != nil || !a.exists("return", previous) {
			return OperationReservation{}, ErrApplicationAmbiguous
		}
		var ok bool
		used, ok = applicationAdd(used, p.Reservation.Bounds)
		if !ok {
			return OperationReservation{}, ErrInvalid
		}
	}
	used, ok := applicationAdd(used, b)
	if !ok || !applicationWithin(used, child.Allowance) {
		return OperationReservation{}, ErrPaused
	}
	r := OperationReservation{EventID: id, Kind: "exchange", UnitID: unit, Bounds: b, Sequence: a.head.Sequence + 1}
	claim := applicationExchange{unit, digest, seq, r}
	e := a.commit("SUBORDINATE_EXCHANGE", id, a.head, applicationObject{applicationObjectName("exchange", id), mustJSON(claim)})
	return r, e
}
func (a *applicationJournal) recordReturned(event string, o ObservedWork) error {
	if !digestValue(event) || o.EventID != event || !digestValue(o.ResponseSHA256) || o.ResponseBytes > 64<<10+1 || o.OutputBytes > 4<<20 || o.Accounting != "UNKNOWN_HISTORY_KNOWN_RETURNED_BYTES" && o.Accounting != "UNKNOWN_LOCAL_PREVIOUS_OBSERVATION" {
		return ErrInvalid
	}
	// The first return draws its bounded retention cost from PREPAY_CHILD, not
	// from remaining parent allowance. It grants no execution after expiry.
	if a.closed || a.poisoned {
		return ErrApplicationAmbiguous
	}
	var claim applicationExchange
	if e := a.object("exchange", event, &claim); e != nil {
		return e
	}
	if o.RequestBytes != claim.Reservation.Bounds.RequestBytes {
		return ErrInvalid
	}
	if a.exists("return", event) {
		// Further duplicate/conflict admission is a new processing attempt; a
		// refusal cannot erase the already-durable first observation.
		if e := a.attempt("REPEAT_RETURN_OBSERVATION", time.Now()); e != nil {
			return e
		}
		var old ObservedWork
		if a.object("return", event, &old) != nil || !bytes.Equal(mustJSON(old), mustJSON(o)) {
			return ErrApplicationAmbiguous
		}
		return nil
	}
	h := a.head
	actual := ApplicationBounds{Calls: 1, RequestBytes: o.RequestBytes, ResponseBytes: o.ResponseBytes, OutputBytes: o.OutputBytes, Records: o.Records, ActiveNS: o.ElapsedNS}
	if o.RequestBytes > math.MaxUint64-o.ResponseBytes {
		return ErrInvalid
	}
	actual.WireBytes = o.RequestBytes + o.ResponseBytes
	var ok bool
	h.Observed, ok = applicationAdd(h.Observed, actual)
	if !ok {
		return ErrInvalid
	}
	return a.commit("RETURNED_BYTES_NOT_SEMANTIC_ADMISSION", event, h, applicationObject{applicationObjectName("return", event), mustJSON(o)})
}
