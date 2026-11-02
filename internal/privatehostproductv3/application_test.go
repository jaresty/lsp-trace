//go:build darwin || linux

package privatehostproductv3

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func applicationFixture(t *testing.T) (ApplicationConfig, time.Time) {
	t.Helper()
	now := time.Now()
	base, canonicalErr := filepath.EvalSymlinks(t.TempDir())
	if canonicalErr != nil {
		t.Fatal(canonicalErr)
	}
	if e := os.Chmod(base, 0700); e != nil {
		t.Fatal(e)
	}
	d := hash([]byte("controlled finite fixture, not a runtime grant"))
	c := ApplicationConfig{Schema: "private-host-application-config.v1", Profile: ApplicationProfile, Store: filepath.Join(base, "application"), SourceRoot: base, RevisionAssertion: "controlled revision", SelectionRuleSHA256: d, UnitPolicySHA256: d, ProducerConfigSHA256: d, ProducerExecutableSHA256: d, StorageIdentitySHA256: d, RolePromptSHA256: [2]string{hash([]byte(CohortRolePrompt("interpreter"))), hash([]byte(CohortRolePrompt("reviewer")))}, Grant: ApplicationGrant{Revision: 1, ExpiresUnixNano: now.Add(time.Minute).UnixNano(), BootstrapAttempts: 8, BootstrapEach: ApplicationBounds{ReadHashBytes: 4 * ApplicationRecordLimit, WriteBytes: ApplicationRecordLimit, Records: 8, ParserBytes: 3 * ApplicationRecordLimit, Work: 32, ActiveNS: 1_000_000}, Ceiling: ApplicationBounds{LogicalBytes: 1 << 32, ReadHashBytes: 1 << 36, WriteBytes: 1 << 36, Records: 1 << 20, ParserBytes: 1 << 36, ProviderRequests: 100, ProviderMessages: 1000, Work: 1 << 20, ActiveNS: uint64(240 * time.Second), Calls: 16, RequestBytes: 1 << 20, ResponseBytes: 64 << 10, WireBytes: 8 << 20, OutputBytes: 4 << 20}}}
	c.ApprovalSHA256 = ApplicationConfigDigest(c)
	return c, now
}
func applicationCreate(t *testing.T, c ApplicationConfig, now time.Time) *ApplicationJournal {
	t.Helper()
	a, e := CreateApplication(c, now)
	if e != nil {
		t.Fatal("fixture create", e)
	}
	t.Cleanup(func() { a.Close() })
	return a
}
func applicationUnit(c ApplicationConfig, id string) UnitBinding {
	d := hash([]byte(id))
	return UnitBinding{ApplicationSHA256: ApplicationConfigDigest(c), UnitID: d, InputSHA256: d, RevisionSHA256: d, SourceFreezeSHA256: d, ScopeSHA256: d, ProducerConfigSHA256: c.ProducerConfigSHA256, PolicySHA256: c.UnitPolicySHA256, RootIdentitySHA256: d, CheckpointSHA256: d}
}
func applicationChild(calls uint64) ApplicationBounds {
	return ApplicationBounds{LogicalBytes: 1024, ReadHashBytes: 1 << 20, WriteBytes: 1 << 20, Records: 1024, ParserBytes: 1 << 20, Work: 1024, ActiveNS: uint64(time.Second), Calls: calls, RequestBytes: calls * 1024, ResponseBytes: calls * 1024, WireBytes: calls * 2048, OutputBytes: 1024}
}

func TestApplicationPrepayDurableNoRefund(t *testing.T) {
	c, now := applicationFixture(t)
	a := applicationCreate(t, c, now)
	b := ApplicationBounds{LogicalBytes: 23, ActiveNS: 123}
	if _, e := a.ReserveOperation(hash([]byte("failed bootstrap")), "bootstrap", b, now); e != nil {
		t.Fatal(e)
	}
	before := a.Head()
	a.Close()
	a, e := OpenApplication(c, now)
	if e != nil {
		t.Fatal(e)
	}
	defer a.Close()
	if before.Reserved.LogicalBytes != 23 || a.Head().Reserved.LogicalBytes != 23 || a.Head().Reserved.ActiveNS < 123 {
		t.Fatal("ASSERT_APPLICATION_PREPAY_SURVIVES_FAILED_WORK_AND_RESTART")
	}
}
func TestApplicationGlobalSixteenAcrossUnits(t *testing.T) {
	c, now := applicationFixture(t)
	a := applicationCreate(t, c, now)
	for _, id := range []string{"one", "two"} {
		if e := a.AdmitUnit(applicationUnit(c, id), applicationChild(8), now); e != nil {
			t.Fatal(e)
		}
	}
	if e := a.AdmitUnit(applicationUnit(c, "three"), applicationChild(1), now); e == nil {
		t.Fatal("ASSERT_APPLICATION_CALL17_REFUSED_BEFORE_CHILD_CREATION")
	}
	if a.Head().Reserved.Calls != 16 {
		t.Fatal("ASSERT_APPLICATION_NO_REFUND_OR_PER_UNIT_RESET")
	}
}
func TestApplicationExchangeIdentityAndReturns(t *testing.T) {
	c, now := applicationFixture(t)
	a := applicationCreate(t, c, now)
	u := applicationUnit(c, "one")
	if e := a.AdmitUnit(u, applicationChild(2), now); e != nil {
		t.Fatal(e)
	}
	r, e := a.ReserveExchange(u.UnitID, 1, hash([]byte("request")), ApplicationBounds{Calls: 1, RequestBytes: 7, ResponseBytes: 1024, WireBytes: 1031}, now)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = a.ReserveExchange(u.UnitID, 1, hash([]byte("request")), ApplicationBounds{Calls: 1}, now); e == nil {
		t.Fatal("ASSERT_APPLICATION_EXCHANGE_IDENTITY_NO_REPLAY")
	}
	observation := ObservedWork{EventID: r.EventID, RequestBytes: 7, ResponseBytes: 23, ResponseSHA256: hash([]byte("invalid physical reply")), Accounting: "UNKNOWN_HISTORY_KNOWN_RETURNED_BYTES"}
	if e = a.RecordReturnedBytes(r.EventID, observation); e != nil {
		t.Fatal(e)
	}
	if e = a.RecordReturnedBytes(r.EventID, observation); e != nil {
		t.Fatal("matching observation must not charge twice", e)
	}
	a.Close()
	a, e = OpenApplication(c, now)
	if e != nil {
		t.Fatal(e)
	}
	defer a.Close()
	if a.Head().Observed.ResponseBytes != 23 || a.Head().Observed.RequestBytes != 7 || a.Head().Reserved.Calls != 2 {
		t.Fatal("ASSERT_APPLICATION_RETURN_BYTES_DURABLE_NOT_DOUBLE_COUNTED")
	}
	observation.ResponseBytes = 24
	if e = a.RecordReturnedBytes(r.EventID, observation); e == nil {
		t.Fatal("ASSERT_APPLICATION_CONFLICTING_RETURN_REFUSED")
	}
}
func TestApplicationBootstrapAttemptLaneFinite(t *testing.T) {
	c, now := applicationFixture(t)
	c.Grant.BootstrapAttempts = 3
	c.ApprovalSHA256 = ApplicationConfigDigest(c)
	a := applicationCreate(t, c, now)
	a.Close()
	// Creation consumed attempt one; stale configurations still consume attempts.
	stale := c
	stale.RevisionAssertion = "foreign"
	stale.ApprovalSHA256 = ApplicationConfigDigest(stale)
	if bad, e := OpenApplication(stale, now); e == nil {
		bad.Close()
		t.Fatal("ASSERT_APPLICATION_STALE_CONFIG_REFUSED")
	}
	a, e := OpenApplication(c, now)
	if e != nil {
		t.Fatal(e)
	}
	a.Close()
	if bad, e := OpenApplication(c, now); e == nil {
		bad.Close()
		t.Fatal("ASSERT_APPLICATION_FAILED_BOOTSTRAP_ATTEMPTS_NEVER_RESET")
	}
}
func TestApplicationCompetingAndTornRefuse(t *testing.T) {
	c, now := applicationFixture(t)
	a := applicationCreate(t, c, now)
	if other, e := OpenApplication(c, now); e == nil {
		other.Close()
		t.Fatal("ASSERT_APPLICATION_COMPETING_COORDINATOR_REFUSED")
	}
	a.Close()
	f, e := os.OpenFile(filepath.Join(c.Store, "journal"), os.O_WRONLY|os.O_APPEND, 0600)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = f.Write([]byte("torn")); e != nil {
		t.Fatal(e)
	}
	f.Close()
	for n := 0; n < 2; n++ {
		if other, e := OpenApplication(c, now); e == nil {
			other.Close()
			t.Fatal("ASSERT_APPLICATION_TORN_RESERVATION_NEVER_RESET")
		}
	}
}
func TestApplicationIndexedTotalBeyondFourMiB(t *testing.T) {
	c, now := applicationFixture(t)
	a := applicationCreate(t, c, now)
	for n := uint64(0); n < 80; n++ {
		id := hash([]byte(time.Unix(int64(n), 0).String()))
		if _, e := a.ReserveOperation(id, "discovery", ApplicationBounds{Records: 1}, now); e != nil {
			t.Fatal(e)
		}
	}
	a.Close()
	st, e := os.Stat(filepath.Join(c.Store, "journal"))
	if e != nil {
		t.Fatal(e)
	}
	a, e = OpenApplication(c, now)
	if e != nil {
		t.Fatal(e)
	}
	defer a.Close()
	if st.Size() <= 4<<20 || a.IO().MaxReadBuffer > ApplicationRecordLimit || a.IO().ReadBytes > 4*ApplicationRecordLimit || a.IO().PriorPageReads != 0 {
		t.Fatalf("ASSERT_APPLICATION_BOUNDED_DIRECT_LOOKUP_OVER_4MIB size=%d io=%+v", st.Size(), a.IO())
	}
	h := a.Head()
	if h.Authority != 0 || h.Accepted || h.FeatureIdentity != "UNRESOLVED" || h.SourceCompleteness != "UNKNOWN" || h.AbsenceInference != "PROHIBITED" {
		t.Fatal("ASSERT_APPLICATION_PROVISIONAL_AUTHORITY")
	}
	t.Logf("INDEX_ONLY_NOT_CATALOG: total journal=%d max read buffer=%d admission bytes=%d prior page reads=%d", st.Size(), a.IO().MaxReadBuffer, a.IO().ReadBytes, a.IO().PriorPageReads)
}
func TestApplicationActiveTimeAndExpiry(t *testing.T) {
	c, now := applicationFixture(t)
	a := applicationCreate(t, c, now)
	if _, e := a.ReserveOperation(hash([]byte("timeout")), "acquisition", ApplicationBounds{ActiveNS: uint64(241 * time.Second)}, now); e == nil {
		t.Fatal("ASSERT_APPLICATION_ACTIVE_TIME_NOT_PER_UNIT")
	}
	if _, e := a.ReserveOperation(hash([]byte("expired")), "acquisition", ApplicationBounds{}, now.Add(2*time.Minute)); e == nil {
		t.Fatal("ASSERT_APPLICATION_EXPLICIT_EXPIRY")
	}
}
