//go:build darwin || linux

package privatehostproductv3

import (
	"errors"
	"time"
)

// ApplicationProfile is the private prepaid application sibling. The private
// CLI owns native acquisition/orchestration; this package owns journal, indexed
// pages and concrete existing file-IPC accounting. It is not public activation,
// authentication or feature acceptance. Parent review controls adoption.
// Nothing in this package turns retained JSON into an original source owner.
const ApplicationProfile = "PRIVATE_HOST_APPLICATION_PREPAID_V1"
const ApplicationRecordLimit = 64 << 10
const applicationSlotBytes = ApplicationRecordLimit

var ErrApplicationAmbiguous = errors.New("application journal ambiguous; no execution permitted")
var ErrApplicationSpent = errors.New("application identity already spent")

// ApplicationBounds are POLICY RESERVATIONS, not measured expenditure. Each
// dimension has its own unit; parent prepayment and child usage are never added.
type ApplicationBounds struct {
	LogicalBytes, ReadHashBytes, WriteBytes, Records, ParserBytes uint64
	ProviderRequests, ProviderMessages, Work, ActiveNS            uint64
	Calls, RequestBytes, ResponseBytes, WireBytes, OutputBytes    uint64
}

type ApplicationGrant struct {
	Revision            uint64
	PreviousGrantSHA256 string
	ExpiresUnixNano     int64
	Ceiling             ApplicationBounds
	// Bootstrap attempts are prepaid in their entirety at creation. This lane
	// includes fixed header/last-slot admission before any deep child validation.
	BootstrapAttempts uint64
	BootstrapEach     ApplicationBounds
}

type ApplicationConfig struct {
	Schema, Profile, Store, SourceRoot, RevisionAssertion       string
	SelectionRuleSHA256, UnitPolicySHA256, ProducerConfigSHA256 string
	ProducerExecutableSHA256, StorageIdentitySHA256             string
	RolePromptSHA256                                            [2]string
	Grant                                                       ApplicationGrant
	ApprovalSHA256                                              string
}

type UnitBinding struct {
	ApplicationSHA256, UnitID, InputSHA256, RevisionSHA256 string
	SourceFreezeSHA256, ScopeSHA256, ProducerConfigSHA256  string
	PolicySHA256, RootIdentitySHA256, CheckpointSHA256     string
	// Source ownership is NOT represented here. The original process and typed
	// retained-owner admission must succeed separately before semantic disclosure.
}

type OperationReservation struct {
	EventID, Kind, UnitID string
	Bounds                ApplicationBounds
	Sequence              uint64
}

type ObservedWork struct {
	EventID                                                      string
	RequestBytes, ResponseBytes, OutputBytes, Records, ElapsedNS uint64
	ResponseSHA256                                               string
	Accounting                                                   string
}

type ApplicationPause struct {
	Status, Reason, Accounting    string
	SourceCompleteness            string
	LastCompletePublicationSHA256 string
}

type ApplicationHead struct {
	Schema, ApplicationSHA256                             string
	Sequence                                              uint64
	Reserved                                              ApplicationBounds
	Observed                                              ApplicationBounds
	LastCompletePublicationSHA256                         string
	CursorSHA256                                          string
	Authority                                             uint8
	Accepted                                              bool
	FeatureIdentity, SourceCompleteness, AbsenceInference string
}

type ApplicationIO struct {
	MaxReadBuffer, ReadBytes, WriteBytes, RecordLookups uint64
	// Prior records are never read when appending. This counter excludes the
	// single current fixed-size journal slot used on admission.
	PriorPageReads uint64
}

// ApplicationJournal is single-coordinator and not goroutine-safe. Its fixed
// tail/high-water checks establish local index consistency, not complete replay
// validation of every historical record or authentication. Child ownership and
// execution remain governed by the private CLI and original/retained APIs;
// publication access revalidates selected references. Unknown observations never
// become exact whole-history or production authority. No live activation here.
type ApplicationJournal struct{ implementation *applicationJournal }

func ApplicationConfigDigest(c ApplicationConfig) string {
	c.ApprovalSHA256 = ""
	return hash(mustJSON(c))
}
func ApplicationGrantDigest(g ApplicationGrant) string { return hash(mustJSON(g)) }
func UnitBindingDigest(u UnitBinding) string           { return hash(mustJSON(u)) }

func CreateApplication(c ApplicationConfig, now time.Time) (*ApplicationJournal, error) {
	return createApplication(c, now)
}
func OpenApplication(c ApplicationConfig, now time.Time) (*ApplicationJournal, error) {
	return openApplication(c, now)
}
func (a *ApplicationJournal) Close() error {
	if a == nil {
		return nil
	}
	return a.implementation.close()
}
func (a *ApplicationJournal) Head() ApplicationHead { return a.implementation.head }
func (a *ApplicationJournal) IO() ApplicationIO     { return a.implementation.io }
func (a *ApplicationJournal) ReserveOperation(id, kind string, b ApplicationBounds, now time.Time) (OperationReservation, error) {
	return a.implementation.reserveOperation(id, kind, b, now)
}
func (a *ApplicationJournal) AdmitUnit(u UnitBinding, b ApplicationBounds, now time.Time) error {
	return a.implementation.admitUnit(u, b, now)
}
func (a *ApplicationJournal) ReserveExchange(unitID string, seq uint64, digest string, b ApplicationBounds, now time.Time) (OperationReservation, error) {
	return a.implementation.reserveExchange(unitID, seq, digest, b, now)
}
func (a *ApplicationJournal) RecordReturnedBytes(event string, o ObservedWork) error {
	return a.implementation.recordReturned(event, o)
}
