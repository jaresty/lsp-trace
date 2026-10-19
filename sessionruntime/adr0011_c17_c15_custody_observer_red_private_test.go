package sessionruntime

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"math"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/manageddiagnostic"
	"lsp-trace/internal/session"
)

// This compile boundary deliberately names the production contract. It must not
// be satisfied by func(any), reflection, JSON decoding, or an unchecked test hook.
// The checked installers are part of the contract because observer refusal must
// propagate before the mapped custody effect.
type privateC15CheckedAccountObserverHost interface {
	installPrivateB4C15BatchObserverChecked(privateB4C15BatchObserver) session.Failure
}

type privateC15CheckedManagerObserverHost interface {
	installPrivateB4C15BatchObserverChecked(string, uint64, privateB4C15BatchObserver) session.Failure
}

func privateC15RequireConcreteObserverContract(
	account *privateB4ByteAccountV2,
	manager *Manager,
	observer privateB4C15BatchObserver,
	event privateB4C15Event,
	operation privateB4C15Operation,
	disposition privateB4C15Disposition,
) {
	var accountHost privateC15CheckedAccountObserverHost = account
	var managerHost privateC15CheckedManagerObserverHost = manager
	_ = accountHost
	_ = managerHost
	_ = observer
	_ = event
	_ = operation
	_ = disposition
}

// privateB4C15Observer is a test-only compatibility shape for older collectors.
// Production Manager paths install only privateB4C15BatchObserver.
type privateB4C15Observer func(privateB4C15Event) error

type privateC15CustodyObserverHost interface {
	installPrivateB4C15BatchObserverChecked(privateB4C15BatchObserver) session.Failure
}

func privateC15BatchObserverFromLegacy(observer privateB4C15Observer) privateB4C15BatchObserver {
	if observer == nil {
		return nil
	}
	return func(events []privateB4C15Event) error {
		for _, event := range events {
			if err := observer(event); err != nil {
				return err
			}
		}
		return nil
	}
}

type privateC15ObservedEvent struct {
	TransactionID string
	ObjectID      string
	SiteID        string
	Kind          string
	Sequence      uint64
	Terminal      bool
}

type privateC15ObservedLedger struct {
	transaction string
	last        uint64
	terminal    map[string]int
	live        map[string]bool
	events      []privateC15ObservedEvent
}

func newPrivateC15ObservedLedger() *privateC15ObservedLedger {
	return &privateC15ObservedLedger{terminal: make(map[string]int), live: make(map[string]bool)}
}

func (l *privateC15ObservedLedger) observe(event privateB4C15Event) error {
	e, err := privateC15DecodeObservedEvent(event)
	if err != nil {
		return err
	}
	if l.transaction == "" {
		l.transaction = e.TransactionID
	}
	if e.TransactionID == "" || e.TransactionID != l.transaction || e.ObjectID == "" || e.SiteID == "" {
		return errors.New("unstable or missing transaction/object/site identity")
	}
	if e.Sequence == 0 || e.Sequence <= l.last {
		return errors.New("sequence is not strictly monotonic")
	}
	l.last = e.Sequence
	switch e.Kind {
	case "reserve", "allocation", "copy":
		if l.live[e.ObjectID] {
			return errors.New("duplicate object creation")
		}
		l.live[e.ObjectID] = true
	case "alias":
		if !l.live[e.ObjectID] {
			return errors.New("alias without live authority")
		}
	case "transfer":
		if !l.live[e.ObjectID] {
			return errors.New("transfer without live object")
		}
	case "release", "terminal", "settlement":
		if !l.live[e.ObjectID] {
			return errors.New("settlement without live object")
		}
		if !e.Terminal {
			return errors.New("settlement must be terminal")
		}
		l.terminal[e.ObjectID]++
		delete(l.live, e.ObjectID)
	case "refusal", "non-escape":
		return errors.New("refusal/non-escape is not an object lifecycle state")
	default:
		return errors.New("unknown custody operation kind")
	}
	l.events = append(l.events, e)
	return nil
}

func privateC15DecodeObservedEvent(event privateB4C15Event) (privateC15ObservedEvent, error) {
	v := reflect.ValueOf(event)
	if !v.IsValid() || v.Kind() == reflect.Pointer || v.Kind() != reflect.Struct {
		return privateC15ObservedEvent{}, errors.New("observer record must be a non-pointer struct value")
	}
	if privateC15ContainsAuthority(v) {
		return privateC15ObservedEvent{}, errors.New("observer record contains mutable/reference custody authority")
	}
	stringField := func(name string) string {
		f := v.FieldByName(name)
		if f.IsValid() && f.Kind() == reflect.String {
			return f.String()
		}
		return ""
	}
	uintField := func(name string) uint64 {
		f := v.FieldByName(name)
		if f.IsValid() && f.Kind() >= reflect.Uint && f.Kind() <= reflect.Uint64 {
			return f.Uint()
		}
		return 0
	}
	boolField := func(name string) bool {
		f := v.FieldByName(name)
		return f.IsValid() && f.Kind() == reflect.Bool && f.Bool()
	}
	return privateC15ObservedEvent{
		TransactionID: stringField("TransactionID"), ObjectID: stringField("ObjectID"), SiteID: stringField("SiteID"),
		Kind: stringField("Kind"), Sequence: uintField("Sequence"), Terminal: boolField("Terminal"),
	}, nil
}

func privateC15ContainsAuthority(v reflect.Value) bool {
	for i := 0; i < v.NumField(); i++ {
		switch f := v.Field(i); f.Kind() {
		case reflect.Pointer, reflect.Map, reflect.Slice, reflect.Func, reflect.Interface, reflect.Chan, reflect.UnsafePointer:
			return true
		case reflect.Struct:
			if privateC15ContainsAuthority(f) {
				return true
			}
		}
	}
	return false
}

func privateC15InstallObserver(t *testing.T, account *privateB4ByteAccountV2, observer privateB4C15Observer) privateC15CustodyObserverHost {
	t.Helper()
	host, ok := any(account).(privateC15CustodyObserverHost)
	if !ok {
		t.Fatal("ASSERT_C17_C15_OBSERVER_CHECKED_OWNER_SEAM_ABSENT")
	}
	if failure := host.installPrivateB4C15BatchObserverChecked(privateC15BatchObserverFromLegacy(observer)); failure != "" {
		t.Fatalf("ASSERT_C17_C15_OBSERVER_CHECKED_OWNER_INSTALL failure=%s", failure)
	}
	account.mu.Lock()
	if account.c15 != nil {
		account.c15.batchOwnershipOnly = false
	}
	account.mu.Unlock()
	return host
}

type privateC15ExpectedEvent struct {
	siteID   string
	kind     string
	terminal bool
}

type privateC15RuntimeFixture struct {
	name              string
	diagnostic        bool
	targetSites       map[string]int
	extraOperations   []privateC15ExpectedEvent
	expectedOrder     []privateC15ExpectedEvent
	derivedLifecycle  func() map[privateC15ExpectedEvent]int
	expectedCreated   int
	unreachedSiteKeys []string
	run               func(*testing.T, privateB4C15Observer)
}

func privateC15ExpectedSiteCounts(siteIDs ...string) map[string]int {
	counts := make(map[string]int, len(siteIDs))
	for _, siteID := range siteIDs {
		counts[siteID]++
	}
	return counts
}

func privateC15ExpectedOperation(siteID, kind string) privateC15ExpectedEvent {
	return privateC15ExpectedEvent{siteID: siteID, kind: kind}
}

func privateC15ExpectedLifecycle(fixture privateC15RuntimeFixture) map[privateC15ExpectedEvent]int {
	if fixture.derivedLifecycle != nil {
		return fixture.derivedLifecycle()
	}
	operations := append([]privateC15ExpectedEvent(nil), fixture.extraOperations...)
	for siteID := range fixture.targetSites {
		kind := "creation"
		switch {
		case siteID == "c15.owner.08.source-descriptor-transfer":
			kind = "transfer"
		case strings.HasPrefix(siteID, "c15.copy."):
			kind = "copy"
		case strings.HasPrefix(siteID, "c15.alias."):
			kind = "alias"
		}
		operations = append(operations, privateC15ExpectedOperation(siteID, kind))
	}
	counts := make(map[privateC15ExpectedEvent]int, len(operations)*2)
	for _, event := range operations {
		counts[event]++
		if event.kind == "creation" || event.kind == "copy" {
			counts[privateC15ExpectedEvent{siteID: event.siteID, kind: "terminal", terminal: true}]++
		}
	}
	return counts
}

// Derived from fixture actions and semantic keys, never from observer output.
// The setup performs params ingress, the owned pair, caller-frame transfer, and
// header/body successor reads, each with a distinct consume-scratch object.
func privateC15ProductionFixtureLifecycle() map[privateC15ExpectedEvent]int {
	created := map[string]int{
		"c15.owner.01.v2-transaction-table": 1, "c15.copy.02.params-ingress": 1,
		"c15.copy.06.source-lease-handle-slice": 1, "c15.copy.37.persistent-capture-params": 1,
		"c15.owner.09.request-frame-capture-charge": 1, "c15.owner.10.request-frame-retained-charge": 1,
		"c15.owner.11.caller-request-frame": 1, "c15.owner.14.successor-prefetch": 1,
		"c15.owner.04.manager-source-ingress-table": 1, "c15.owner.05.per-source-ingress-state": 1,
		"c15.copy.03.prepared-source-content": 1, "c15.copy.07.transaction-source-bytes": 1,
		"c15.owner.25.diagnostic-event-backing": 1, "c15.copy.26.diagnostic-metadata-strings": 1,
		"c15.owner.13.outbound-canonical-frame": 1, "c15.copy.12.encoded-request-body": 1,
		"c15.owner.15.successor-header": 1, "c15.owner.16.successor-consume-scratch": 2,
		"c15.owner.17.successor-body": 1, "c15.copy.18.original-response-frame": 1,
		"c15.owner.22.result-token-replacement": 1, "c15.copy.23.decoded-message-clone": 1,
		"c15.copy.34.owned-pair-params": 1, "c15.copy.35.owned-pair-result": 1,
		"c15.owner.27.fixed-diagnostic-history": 1, "c15.copy.39.final-capture-request-frame": 1,
		"c15.copy.40.final-capture-request-params": 1, "c15.copy.41.final-capture-response-frame": 1,
		"c15.copy.42a.final-capture-target-slice": 1, "c15.copy.42b.final-capture-source-bytes": 1,
	}
	events := make(map[privateC15ExpectedEvent]int, len(created)*2+4)
	for siteID, count := range created {
		kind := "creation"
		if strings.HasPrefix(siteID, "c15.copy.") {
			kind = "copy"
		}
		events[privateC15ExpectedEvent{siteID: siteID, kind: kind}] = count
		events[privateC15ExpectedEvent{siteID: siteID, kind: "terminal", terminal: true}] = count
	}
	// Transfers and aliases preserve object identity and add no denominator object.
	events[privateC15ExpectedEvent{siteID: "c15.owner.08.source-descriptor-transfer", kind: "transfer"}] = 1
	events[privateC15ExpectedEvent{siteID: "c15.owner.11.caller-request-frame", kind: "transfer"}] = 1
	events[privateC15ExpectedEvent{siteID: "c15.alias.43.borrowed-lexical-result", kind: "alias"}] = 1
	events[privateC15ExpectedEvent{siteID: "c15.alias.44.borrowed-capture-values", kind: "alias"}] = 1
	return events
}

func privateC15NormalizeExpectedEvent(event privateC15ObservedEvent) privateC15ExpectedEvent {
	kind := event.Kind
	switch kind {
	case "reserve", "allocation":
		kind = "creation"
	case "release", "settlement", "terminal":
		kind = "terminal"
	}
	return privateC15ExpectedEvent{siteID: event.SiteID, kind: kind, terminal: event.Terminal}
}

func TestADR0011C17C15ObserverAccepted44RuntimeFixtureMatrix(t *testing.T) {
	fixtures := []privateC15RuntimeFixture{
		{
			name: "account owner copy alias transfer terminal",
			targetSites: privateC15ExpectedSiteCounts(
				"c15.copy.02.params-ingress",
			),
			extraOperations: []privateC15ExpectedEvent{
				privateC15ExpectedOperation("c15.copy.02.params-ingress", "alias"),
				privateC15ExpectedOperation("c15.copy.02.params-ingress", "transfer"),
			},
			expectedOrder: []privateC15ExpectedEvent{
				privateC15ExpectedOperation("c15.copy.02.params-ingress", "copy"),
				privateC15ExpectedOperation("c15.copy.02.params-ingress", "alias"),
				privateC15ExpectedOperation("c15.copy.02.params-ingress", "transfer"),
				{siteID: "c15.copy.02.params-ingress", kind: "terminal", terminal: true},
			},
			run: func(t *testing.T, observer privateB4C15Observer) {
				account := mustPrivateB4ByteAccountV2(t)
				privateC15InstallObserver(t, account, observer)
				copied, lease, failure := copyPrivateB4Params(account, []byte(`{"fixture":true}`))
				if failure != "" || len(copied) == 0 || !lease.lease.active {
					t.Fatalf("BLOCKED_NOT_RED account fixture failure=%s copied=%q lease=%+v", failure, copied, lease)
				}
				alias := lease.alias()
				moved := lease.transfer()
				alias.release()
				moved.release()
				account.requestTerminalRelease()
			},
		},
		{
			name:        "retained notification owner and callback alias",
			targetSites: privateC15ExpectedSiteCounts("c15.copy.28.retained-notification-set", "c15.alias.29.notifications-callback"),
			run: func(t *testing.T, observer privateB4C15Observer) {
				account := mustPrivateB4ByteAccountV2(t)
				privateC15InstallObserver(t, account, observer)
				owner, failure := newPrivateB4RetainedNotificationOwner(account, notificationFixture())
				if failure != "" || owner == nil {
					t.Fatalf("BLOCKED_NOT_RED notification fixture failure=%s", failure)
				}
				lease := RetainedNotificationLease{owner: owner}
				if err := lease.WithNotifications(func([]lspwire.Message) error { return nil }); err != nil || !lease.Release() {
					t.Fatalf("BLOCKED_NOT_RED notification callback err=%v", err)
				}
				account.requestTerminalRelease()
			},
		},
		{
			name:        "retained response owner and callback alias",
			targetSites: privateC15ExpectedSiteCounts("c15.copy.30.retained-response-set", "c15.alias.31.responses-callback"),
			run: func(t *testing.T, observer privateB4C15Observer) {
				account := mustPrivateB4ByteAccountV2(t)
				privateC15InstallObserver(t, account, observer)
				owner, failure := newPrivateB4RetainedResponseOwner(account, retainedResponseFixture())
				if failure != "" || owner == nil {
					t.Fatalf("BLOCKED_NOT_RED response fixture failure=%s", failure)
				}
				lease := RetainedResponseLease{owner: owner}
				if err := lease.WithResponses(func([]lspwire.Message) error { return nil }); err != nil || !lease.Release() {
					t.Fatalf("BLOCKED_NOT_RED response callback err=%v", err)
				}
				account.requestTerminalRelease()
			},
		},
		{
			name:        "server error owner and callback alias",
			targetSites: privateC15ExpectedSiteCounts("c15.copy.32.server-error-message-data", "c15.alias.33.server-error-callback"),
			run: func(t *testing.T, observer privateB4C15Observer) {
				account := mustPrivateB4ByteAccountV2(t)
				privateC15InstallObserver(t, account, observer)
				owner, failure := newPrivateB4ServerErrorOwner(account, serverErrorFixture())
				if failure != "" || owner == nil {
					t.Fatalf("BLOCKED_NOT_RED server-error fixture failure=%s", failure)
				}
				lease := ServerErrorLease{owner: owner}
				if err := lease.WithServerError(func(ServerErrorView) error { return nil }); err != nil || !lease.Release() {
					t.Fatalf("BLOCKED_NOT_RED server-error callback err=%v", err)
				}
				account.requestTerminalRelease()
			},
		},
		{
			name:        "owned method pair copies and callback alias",
			targetSites: privateC15ExpectedSiteCounts("c15.copy.34.owned-pair-params", "c15.copy.35.owned-pair-result", "c15.alias.36.owned-pair-callback"),
			run: func(t *testing.T, observer privateB4C15Observer) {
				account := mustPrivateB4ByteAccountV2(t)
				privateC15InstallObserver(t, account, observer)
				owner, failure := newPrivateB4OwnedMethodPairOwner(account, OwnedMethodPair{Params: []byte("params"), Result: []byte("result")})
				if failure != "" || owner == nil {
					t.Fatalf("BLOCKED_NOT_RED pair fixture failure=%s", failure)
				}
				lease := OwnedMethodPairLease{owner: owner}
				if err := lease.WithPair(func(OwnedMethodPairView) error { return nil }); err != nil || !lease.Release() {
					t.Fatalf("BLOCKED_NOT_RED pair callback err=%v", err)
				}
				account.requestTerminalRelease()
			},
		},
		{
			name:       "production private transaction, source, successor, capture, diagnostic, and server-request paths",
			diagnostic: true,
			targetSites: privateC15ExpectedSiteCounts(
				"c15.owner.01.v2-transaction-table", "c15.owner.04.manager-source-ingress-table", "c15.owner.05.per-source-ingress-state", "c15.owner.08.source-descriptor-transfer",
				"c15.owner.09.request-frame-capture-charge", "c15.owner.10.request-frame-retained-charge", "c15.owner.11.caller-request-frame",
				"c15.owner.13.outbound-canonical-frame", "c15.owner.14.successor-prefetch", "c15.owner.15.successor-header",
				"c15.owner.16.successor-consume-scratch", "c15.owner.17.successor-body", "c15.owner.22.result-token-replacement",
				"c15.owner.25.diagnostic-event-backing", "c15.owner.27.fixed-diagnostic-history",
				"c15.alias.43.borrowed-lexical-result", "c15.alias.44.borrowed-capture-values", "c15.alias.45a.server-request-id-token",
				"c15.copy.03.prepared-source-content", "c15.copy.06.source-lease-handle-slice", "c15.copy.07.transaction-source-bytes",
				"c15.copy.12.encoded-request-body", "c15.copy.18.original-response-frame", "c15.copy.21.exact-result-token",
				"c15.copy.23.decoded-message-clone", "c15.copy.26.diagnostic-metadata-strings", "c15.copy.37.persistent-capture-params",
				"c15.copy.39.final-capture-request-frame", "c15.copy.40.final-capture-request-params", "c15.copy.41.final-capture-response-frame",
				"c15.copy.42a.final-capture-target-slice", "c15.copy.42b.final-capture-source-bytes",
				"c15.copy.45b.unsupported-response-body", "c15.copy.45c.unsupported-response-header",
			),
			derivedLifecycle: privateC15ProductionFixtureLifecycle,
			expectedCreated:  31,
			unreachedSiteKeys: []string{
				"c15.alias.45a.server-request-id-token", "c15.copy.21.exact-result-token",
				"c15.copy.45b.unsupported-response-body", "c15.copy.45c.unsupported-response-header",
			},
			run: func(t *testing.T, observer privateB4C15Observer) {
				f := newFullP1Fixture(t)
				f.m.diagnostics = manageddiagnostic.NewStore(manageddiagnostic.Bounds{MaxRecords: 4, MaxBytes: 1 << 20})
				f.req.CaptureOwnedMethodPair = true
				if failure := installPrivateB4C15BatchObserverForTest(f.m, f.req.SessionID, f.req.Generation, privateC15BatchObserverFromLegacy(observer)); failure != "" {
					t.Fatalf("BLOCKED_NOT_RED manager observer install failure=%s", failure)
				}
				result, managerLease, selection := f.transact(t)
				requestLease, _ := result.PrivateB4RequestFrameLease()
				responseLease, _ := result.PrivateB4ResponseFrameLease()
				resultLease, _ := result.PrivateB4ResultLease()
				pairLease, _ := result.PrivateOwnedMethodPairLease()
				_, _ = consumePrivateB4SnapshotForTest(f.m, managerLease, selection)
				requestLease.Release()
				responseLease.Release()
				resultLease.Release()
				pairLease.Release()
			},
		},
	}

	mapped := make(map[string]bool, len(privateB4C15ProductionSites))
	for _, row := range privateB4C15ProductionSites {
		mapped[row.siteID] = true
	}
	expectedUnion := make(map[string]bool, len(mapped))
	for _, fixture := range fixtures {
		for siteID, count := range fixture.targetSites {
			if !mapped[siteID] || count != 1 {
				t.Fatalf("ASSERT_C17_C15_RUNTIME_MATRIX_UNKNOWN_OR_NONEXACT_TARGET_SITE fixture=%s site=%s count=%d", fixture.name, siteID, count)
			}
			expectedUnion[siteID] = true
		}
	}
	if len(expectedUnion) != 44 || len(expectedUnion) != len(mapped) {
		t.Fatalf("ASSERT_C17_C15_RUNTIME_MATRIX_EXACT_44 fixtures=%d expected=%d mapped=%d", len(fixtures), len(expectedUnion), len(mapped))
	}

	for _, fixture := range fixtures {
		fixture := fixture
		t.Run(fixture.name, func(t *testing.T) {
			if fixture.diagnostic {
				requireC15DiagnosticRepresentation(t)
			}
			ledger := newPrivateC15ObservedLedger()
			fixture.run(t, ledger.observe)
			expectedEvents := privateC15ExpectedLifecycle(fixture)
			actualEvents := make(map[privateC15ExpectedEvent]int)
			actualOrder := make([]privateC15ExpectedEvent, 0, len(ledger.events))
			for _, event := range ledger.events {
				normalized := privateC15NormalizeExpectedEvent(event)
				actualEvents[normalized]++
				actualOrder = append(actualOrder, normalized)
			}
			if !reflect.DeepEqual(actualEvents, expectedEvents) {
				t.Errorf("ASSERT_C17_C15_RUNTIME_MATRIX_COMPLETE_EVENT_COUNTS fixture=%s expected=%v actual=%v events=%+v", fixture.name, expectedEvents, actualEvents, ledger.events)
			}
			if fixture.expectedOrder != nil && !reflect.DeepEqual(actualOrder, fixture.expectedOrder) {
				t.Errorf("ASSERT_C17_C15_RUNTIME_MATRIX_EXACT_EVENT_ORDER fixture=%s expected=%v actual=%v events=%+v", fixture.name, fixture.expectedOrder, actualOrder, ledger.events)
			}
			privateC15RequireExactReferenceLedger(t, ledger, fixture.targetSites, expectedEvents)
			if fixture.expectedCreated != 0 && len(ledger.terminal) != fixture.expectedCreated {
				t.Errorf("ASSERT_C17_C15_FIXTURE_CREATED_DENOMINATOR expected=%d terminal_objects=%d", fixture.expectedCreated, len(ledger.terminal))
			}
			for _, siteID := range fixture.unreachedSiteKeys {
				for _, event := range ledger.events {
					if event.SiteID == siteID {
						t.Errorf("ASSERT_C17_C15_FIXTURE_NON_REACHABILITY site=%s event=%+v", siteID, event)
					}
				}
			}
		})
	}
}

func privateC15RequireExactReferenceLedger(t *testing.T, ledger *privateC15ObservedLedger, targetSites map[string]int, expectedEvents map[privateC15ExpectedEvent]int) {
	t.Helper()
	if ledger.transaction == "" || len(ledger.events) == 0 {
		t.Errorf("ASSERT_C17_C15_REFERENCE_LEDGER_EMPTY target_sites=%v expected_events=%v", targetSites, expectedEvents)
		return
	}
	created := make(map[string]privateC15ObservedEvent)
	terminal := make(map[string]int)
	last := uint64(0)
	for _, event := range ledger.events {
		if event.TransactionID != ledger.transaction || event.ObjectID == "" || event.SiteID == "" || event.Sequence != last+1 {
			t.Errorf("ASSERT_C17_C15_REFERENCE_LEDGER_IDENTITY_SEQUENCE transaction=%q last=%d event=%+v", ledger.transaction, last, event)
		}
		last = event.Sequence
		switch event.Kind {
		case "reserve", "allocation", "copy":
			if _, exists := created[event.ObjectID]; exists {
				t.Errorf("ASSERT_C17_C15_REFERENCE_LEDGER_DUPLICATE_OBJECT object=%s", event.ObjectID)
			}
			created[event.ObjectID] = event
		case "alias", "transfer":
			if _, exists := created[event.ObjectID]; !exists {
				t.Errorf("ASSERT_C17_C15_REFERENCE_LEDGER_SHARED_AUTHORITY_WITHOUT_OBJECT object=%s kind=%s", event.ObjectID, event.Kind)
			}
		case "release", "terminal", "settlement":
			if event.Terminal {
				terminal[event.ObjectID]++
			}
		}
	}
	expectedTerminalCount := 0
	for event, count := range expectedEvents {
		if event.kind == "terminal" && event.terminal {
			expectedTerminalCount += count
		}
	}
	if expectedTerminalCount != len(created) {
		t.Errorf("ASSERT_C17_C15_REFERENCE_LEDGER_EXPECTED_TERMINALS_EQUAL_CREATED expected_terminals=%d created=%d target_sites=%v expected_events=%v", expectedTerminalCount, len(created), targetSites, expectedEvents)
	}
	for objectID, creation := range created {
		if terminal[objectID] != 1 {
			t.Errorf("ASSERT_C17_C15_REFERENCE_LEDGER_ONE_TERMINAL object=%s site=%s terminals=%d", objectID, creation.SiteID, terminal[objectID])
		}
	}
	for objectID := range terminal {
		if _, exists := created[objectID]; !exists {
			t.Errorf("ASSERT_C17_C15_REFERENCE_LEDGER_TERMINAL_WITHOUT_CREATION object=%s", objectID)
		}
	}
}

func TestADR0011C17C15ObserverRealRefusalBeforeEffectByClass(t *testing.T) {
	t.Run("owner", func(t *testing.T) {
		account := mustPrivateB4ByteAccountV2(t)
		privateC15InstallObserver(t, account, func(event privateB4C15Event) error {
			e, err := privateC15DecodeObservedEvent(event)
			if err != nil {
				return err
			}
			if e.Kind == "reserve" || e.Kind == "allocation" {
				return errors.New("refuse owner")
			}
			return nil
		})
		before := account.snapshot()
		lease, failure := account.reserve(1)
		if failure != session.ResourceExhausted || lease.lease.active || account.snapshot() != before {
			t.Errorf("ASSERT_C17_C15_OWNER_REFUSAL_BEFORE_EFFECT failure=%s lease=%+v before=%+v after=%+v", failure, lease, before, account.snapshot())
			if lease.lease.active {
				lease.release()
			}
		}
		account.requestTerminalRelease()
	})

	t.Run("alias", func(t *testing.T) {
		account := mustPrivateB4ByteAccountV2(t)
		privateC15InstallObserver(t, account, func(event privateB4C15Event) error {
			e, err := privateC15DecodeObservedEvent(event)
			if err != nil {
				return err
			}
			if e.Kind == "alias" {
				return errors.New("refuse alias")
			}
			return nil
		})
		lease, failure := account.reserve(1)
		if failure != "" {
			t.Fatalf("BLOCKED_NOT_RED alias setup failure=%s", failure)
		}
		before := account.snapshot()
		alias := lease.alias()
		if alias.lease.active || account.snapshot() != before {
			t.Errorf("ASSERT_C17_C15_ALIAS_REFUSAL_BEFORE_EFFECT alias=%+v before=%+v after=%+v", alias, before, account.snapshot())
			if alias.lease.active {
				alias.release()
			}
		}
		lease.release()
		account.requestTerminalRelease()
	})

	t.Run("copy", func(t *testing.T) {
		account := mustPrivateB4ByteAccountV2(t)
		privateC15InstallObserver(t, account, func(event privateB4C15Event) error {
			e, err := privateC15DecodeObservedEvent(event)
			if err != nil {
				return err
			}
			if e.Kind == "copy" || e.Kind == "reserve" || e.Kind == "allocation" {
				return errors.New("refuse copy")
			}
			return nil
		})
		before := account.snapshot()
		copied, lease, failure := copyPrivateB4Params(account, []byte(`{"copy":true}`))
		if failure != session.ResourceExhausted || copied != nil || lease.lease.active || account.snapshot() != before {
			t.Errorf("ASSERT_C17_C15_COPY_REFUSAL_BEFORE_EFFECT failure=%s copy=%q lease=%+v before=%+v after=%+v", failure, copied, lease, before, account.snapshot())
			if lease.lease.active {
				lease.release()
			}
		}
		account.requestTerminalRelease()
	})
}

func TestADR0011C17C15ObserverNilDefaultOffAllocationParityEveryClass(t *testing.T) {
	tests := []struct {
		name string
		run  func(*testing.T, *privateB4ByteAccountV2)
	}{
		{"owner", func(t *testing.T, account *privateB4ByteAccountV2) {
			lease, failure := account.reserve(1)
			if failure != "" {
				t.Fatalf("BLOCKED_NOT_RED owner parity failure=%s", failure)
			}
			lease.release()
		}},
		{"alias", func(t *testing.T, account *privateB4ByteAccountV2) {
			lease, failure := account.reserve(1)
			if failure != "" {
				t.Fatalf("BLOCKED_NOT_RED alias parity failure=%s", failure)
			}
			alias := lease.alias()
			alias.release()
			lease.release()
		}},
		{"copy", func(t *testing.T, account *privateB4ByteAccountV2) {
			_, lease, failure := copyPrivateB4Params(account, []byte(`{"copy":true}`))
			if failure != "" {
				t.Fatalf("BLOCKED_NOT_RED copy parity failure=%s", failure)
			}
			lease.release()
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			plain := mustPrivateB4ByteAccountV2(t)
			plainAllocs := testing.AllocsPerRun(100, func() { tc.run(t, plain) })
			plainSnapshot := plain.snapshot()
			plain.requestTerminalRelease()

			nilObserved := mustPrivateB4ByteAccountV2(t)
			privateC15InstallObserver(t, nilObserved, nil)
			nilAllocs := testing.AllocsPerRun(100, func() { tc.run(t, nilObserved) })
			nilSnapshot := nilObserved.snapshot()
			nilObserved.requestTerminalRelease()
			if nilAllocs != plainAllocs || nilSnapshot != plainSnapshot {
				t.Fatalf("ASSERT_C17_C15_NIL_DEFAULT_OFF_CLASS_PARITY class=%s plain_allocs=%f nil_allocs=%f plain=%+v nil=%+v", tc.name, plainAllocs, nilAllocs, plainSnapshot, nilSnapshot)
			}
		})
	}
}

func TestADR0011C17C15ObserverDefaultOffNilCompatibilityAndAllocationProfile(t *testing.T) {
	ordinary := mustPrivateB4ByteAccountV2(t)
	before := ordinary.snapshot()
	allocs := testing.AllocsPerRun(100, func() {
		lease, failure := ordinary.reserve(1)
		if failure != "" {
			t.Fatalf("BLOCKED_NOT_RED ordinary reserve: %s", failure)
		}
		lease.release()
	})
	if ordinary.snapshot().Live != before.Live || allocs != 0 {
		t.Fatalf("ASSERT_C17_C15_DEFAULT_OFF_PROFILE snapshot=%+v before=%+v allocs=%f", ordinary.snapshot(), before, allocs)
	}

	private := mustPrivateB4ByteAccountV2(t)
	privateC15InstallObserver(t, private, nil)
	before = private.snapshot()
	allocs = testing.AllocsPerRun(100, func() {
		lease, failure := private.reserve(1)
		if failure != "" {
			t.Fatalf("BLOCKED_NOT_RED private nil-observer reserve: %s", failure)
		}
		lease.release()
	})
	if private.snapshot().Live != before.Live || allocs != 0 {
		t.Fatalf("ASSERT_C17_C15_NIL_OBSERVER_PROFILE snapshot=%+v before=%+v allocs=%f", private.snapshot(), before, allocs)
	}
}

func TestADR0011C17C15ObserverRealOwnerCustodyGraphAndTerminalFreeze(t *testing.T) {
	account := mustPrivateB4ByteAccountV2(t)
	ledger := newPrivateC15ObservedLedger()
	privateC15InstallObserver(t, account, ledger.observe)

	lease, failure := account.reserve(11)
	if failure != "" {
		t.Fatalf("BLOCKED_NOT_RED reserve: %s", failure)
	}
	if len(ledger.events) != 1 || (ledger.events[0].Kind != "reserve" && ledger.events[0].Kind != "allocation") {
		t.Fatalf("ASSERT_C17_C15_REAL_RESERVE_EVENT events=%+v", ledger.events)
	}
	alias := lease.alias()
	if !alias.lease.active || len(ledger.events) != 2 || ledger.events[1].Kind != "alias" {
		t.Fatalf("ASSERT_C17_C15_REAL_ALIAS_EVENT events=%+v", ledger.events)
	}
	moved := lease.transfer()
	if !moved.lease.active || len(ledger.events) != 3 || ledger.events[2].Kind != "transfer" {
		t.Fatalf("ASSERT_C17_C15_REAL_TRANSFER_EVENT events=%+v", ledger.events)
	}
	alias.release()
	moved.release()
	account.requestTerminalRelease()
	if account.snapshot().Live != 0 {
		t.Fatalf("BLOCKED_NOT_RED owner not quiescent: %+v", account.snapshot())
	}
	if len(ledger.live) != 0 {
		t.Fatalf("ASSERT_C17_C15_EXACT_CUSTODY_GRAPH live=%v events=%+v", ledger.live, ledger.events)
	}
	for objectID, count := range ledger.terminal {
		if count != 1 {
			t.Fatalf("ASSERT_C17_C15_ONE_TERMINAL object=%s count=%d events=%+v", objectID, count, ledger.events)
		}
	}
	frozen := len(ledger.events)
	if _, failure := account.reserve(1); failure != session.ResourceExhausted || len(ledger.events) != frozen {
		t.Fatalf("ASSERT_C17_C15_TERMINAL_FREEZE failure=%s before=%d after=%d", failure, frozen, len(ledger.events))
	}
}

func TestADR0011C17C15ObserverCopyRefusalPrecedesIrreversibleEffect(t *testing.T) {
	account := mustPrivateB4ByteAccountV2(t)
	refusal := errors.New("observer refusal")
	privateC15InstallObserver(t, account, func(event privateB4C15Event) error {
		e, err := privateC15DecodeObservedEvent(event)
		if err != nil {
			return err
		}
		if e.Kind == "copy" || e.Kind == "reserve" || e.Kind == "allocation" {
			return refusal
		}
		return nil
	})
	params := []byte(`{"textDocument":{"uri":"file:///w/main.go"}}`)
	before := account.snapshot()
	copyValue, lease, failure := copyPrivateB4Params(account, params)
	if failure != session.ResourceExhausted || copyValue != nil || lease.lease.active {
		t.Fatalf("ASSERT_C17_C15_OBSERVER_REFUSAL_TYPED failure=%s copy=%q lease=%+v", failure, copyValue, lease)
	}
	if after := account.snapshot(); after != before {
		t.Fatalf("ASSERT_C17_C15_OBSERVER_REFUSAL_ZERO_ALLOCATION_COPY before=%+v after=%+v", before, after)
	}
}

func TestADR0011C17C15ObserverRefusalRollsBackSequenceAndObjectReservations(t *testing.T) {
	account := mustPrivateB4ByteAccountV2(t)
	refuseCopy := true
	var committed []privateB4C15Event
	privateC15InstallObserver(t, account, func(event privateB4C15Event) error {
		if event.Kind == "copy" && refuseCopy {
			return errors.New("refuse first copy")
		}
		committed = append(committed, event)
		return nil
	})
	if _, _, failure := copyPrivateB4Params(account, []byte("refused")); failure != session.ResourceExhausted {
		t.Fatalf("ASSERT_C17_C15_REFUSAL_REQUIRED failure=%s", failure)
	}
	refuseCopy = false
	_, lease, failure := copyPrivateB4Params(account, []byte("accepted"))
	if failure != "" {
		t.Fatalf("ASSERT_C17_C15_POST_REFUSAL_ACCEPTED failure=%s", failure)
	}
	if len(committed) != 1 || committed[0].Sequence != 1 || committed[0].ObjectID != "c15-object-1" {
		t.Fatalf("ASSERT_C17_C15_REFUSAL_RESERVATIONS_ROLLED_BACK events=%+v", committed)
	}
	lease.release()
	account.requestTerminalRelease()
}

func TestADR0011C17C15ObserverLookupRequiresExactGeneration(t *testing.T) {
	observer := privateB4C15BatchObserver(func([]privateB4C15Event) error { return nil })
	manager := &Manager{privateB4C15Contexts: map[string]privateB4C15ManagerContext{
		"session": {generation: 7, observer: observer, contexts: make(map[string]*privateB4C15CustodyContext)},
	}}
	context, failure := manager.privateB4C15ContextFor("session", 7, "transaction")
	if failure != "" || context == nil {
		t.Fatal("ASSERT_C17_C15_CONTEXT_EXACT_GENERATION_MISSING")
	}
	if stale, _ := manager.privateB4C15ContextFor("session", 6, "transaction"); stale != nil {
		t.Fatal("ASSERT_C17_C15_CONTEXT_NONEXACT_GENERATION_VISIBLE")
	}
}

func TestADR0011C17C15ObserverInvalidCapacityAndStateEmitNoEvents(t *testing.T) {
	account := mustPrivateB4ByteAccountV2(t)
	var events []privateB4C15Event
	privateC15InstallObserver(t, account, func(event privateB4C15Event) error {
		events = append(events, event)
		return nil
	})
	baselineEvents := len(events)
	baselineState := account.snapshot()

	lease, failure := account.reserve(privateB4MaxOwnedBytes + 1)
	if failure != session.ResourceExhausted || lease.lease.active || len(events) != baselineEvents || account.snapshot() != baselineState {
		t.Fatalf("ASSERT_C17_C15_INVALID_RESERVE_ZERO_EVENT failure=%s lease=%+v events_before=%d events_after=%d state_before=%+v state_after=%+v", failure, lease, baselineEvents, len(events), baselineState, account.snapshot())
	}

	invalid := privateB4ByteLeaseV2{account: account}
	if alias := invalid.alias(); alias.lease.active || len(events) != baselineEvents || account.snapshot() != baselineState {
		t.Fatalf("ASSERT_C17_C15_INVALID_STATE_ZERO_EVENT operation=alias alias=%+v events_before=%d events_after=%d state_before=%+v state_after=%+v", alias, baselineEvents, len(events), baselineState, account.snapshot())
	}
	if moved := invalid.transfer(); moved.lease.active || len(events) != baselineEvents || account.snapshot() != baselineState {
		t.Fatalf("ASSERT_C17_C15_INVALID_STATE_ZERO_EVENT operation=transfer moved=%+v events_before=%d events_after=%d state_before=%+v state_after=%+v", moved, baselineEvents, len(events), baselineState, account.snapshot())
	}
}

func TestADR0011C17C15ObserverSequenceOverflowRefusesBeforeEffect(t *testing.T) {
	account := mustPrivateB4ByteAccountV2(t)
	privateC15InstallObserver(t, account, func(privateB4C15Event) error { return nil })
	sequence, ok := any(account).(interface{ setPrivateC15CustodySequenceForTest(uint64) })
	if !ok {
		t.Fatal("ASSERT_C17_C15_SEQUENCE_OVERFLOW_HOOK_ABSENT")
	}
	sequence.setPrivateC15CustodySequenceForTest(math.MaxUint64)
	before := account.snapshot()
	lease, failure := account.reserve(1)
	if failure != session.ResourceExhausted || lease.lease.active || account.snapshot() != before {
		t.Fatalf("ASSERT_C17_C15_SEQUENCE_OVERFLOW_ZERO_EFFECT failure=%s lease=%+v before=%+v after=%+v", failure, lease, before, account.snapshot())
	}
}

func newPrivateC15ObserverLifecycleManager(t *testing.T, limits Limits, starter Starter, now func() time.Time) *Manager {
	t.Helper()
	algebra, err := session.NewManager(session.ManagerConfig{MaxSessions: limits.MaxSessions})
	if err != nil {
		t.Fatal(err)
	}
	if limits.MaxOperations <= 0 {
		limits.MaxOperations = limits.MaxObservations
	}
	if now == nil {
		now = time.Now
	}
	var nonce [16]byte
	if _, err := io.ReadFull(rand.Reader, nonce[:]); err != nil {
		t.Fatal(err)
	}
	sourceIngress, failure := newPrivateB4SourceIngressOwner()
	if failure != "" {
		t.Fatalf("private B4 source ingress: %s", failure)
	}
	return &Manager{
		limits: limits, starter: starter, algebra: algebra, sessions: make(map[string]*runtimeSession),
		operations: make(map[string]OperationSnapshot), readiness: make(map[string]*readinessOperation),
		readinessIDs: make(map[string]string), readinessTimeout: time.Second, now: now,
		workerDone: make(chan struct{}, 1), gitWorktreeList: boundedGitWorktreeList,
		startupAttemptNonce: nonce, transientIdentityRandom: rand.Reader, readinessHistoryRandom: rand.Reader,
		transientIdentities: make(map[string]struct{}), diagnosticOperations: make(map[DiagnosticOperationHandle]diagnosticOperation),
		privateB4SourceIngress: sourceIngress, managerDiagnosticOwner: new(managerDiagnosticOwner),
		privateDiagnosticHistory: new(privateDiagnosticHistory),
	}
}

func privateC15ObserverSourceLease(t *testing.T, m *Manager, f b4ID1Fixture, sessionID string, generation uint64) B4DefinitionSourceLease {
	t.Helper()
	sourceURI := "file:///w/target-a.go"
	sourceBytes := b4LeaseGet(t, f, "A/target-a.go")
	m.mu.Lock()
	m.sessions[sessionID].seedSources[sourceURI] = append([]byte(nil), sourceBytes...)
	m.mu.Unlock()
	prepared := m.PrepareDocument(context.Background(), DocumentRequest{SessionID: sessionID, Generation: generation, URI: sourceURI, LanguageID: "go", CaptureSupply: true})
	if prepared.Failure != "" || prepared.Supply == nil || !bytes.Equal(prepared.Supply.Content, sourceBytes) {
		t.Fatalf("BLOCKED_NOT_RED observer source supply generation=%d prepared=%+v", generation, prepared)
	}
	lease, status := m.PreparePrivateB4DefinitionSource(B4DefinitionSourceReference{SessionID: sessionID, Generation: generation, URI: sourceURI, DocumentVersion: prepared.Version})
	if status != PrivateB4Selected || lease.state == nil {
		t.Fatalf("BLOCKED_NOT_RED observer source lease generation=%d status=%s", generation, status)
	}
	return lease
}

func privateC15ObserverStageBSelected(t *testing.T, f b4ID1Fixture, observer privateB4C15Observer) (*Manager, *b4LeaseChild, B4DefinitionSelectionKey) {
	t.Helper()
	m, started, req, owner, child := b4LeaseManager(t, f, "A")
	owner.TargetSources = []B4DefinitionSourceLease{privateC15ObserverSourceLease(t, m, f, started.SessionID, started.Generation)}
	if failure := installPrivateB4C15BatchObserverForTest(m, started.SessionID, started.Generation, privateC15BatchObserverFromLegacy(observer)); failure != "" {
		t.Fatalf("BLOCKED_NOT_RED generation1 observer install=%s", failure)
	}
	var account *privateB4ByteAccountV2
	m.b4ParamsHook = func(got *privateB4ByteAccountV2) { account = got }
	result, lease := m.RoundTripPrivateB4(context.Background(), req, owner)
	if result.Failure != "" || lease == (B4DefinitionLease{}) {
		var snapshot privateB4ByteLedgerSnapshot
		if account != nil {
			snapshot = account.snapshot()
		}
		t.Fatalf("BLOCKED_NOT_RED generation1 transaction result=%+v account=%+v source_count=%d", result, snapshot, len(owner.TargetSources))
	}
	select {
	case err := <-child.observed:
		if err != nil {
			t.Fatalf("BLOCKED_NOT_RED generation1 transaction observation: %v", err)
		}
	case <-t.Context().Done():
		t.Fatalf("BLOCKED_NOT_RED generation1 transaction observation: %v", t.Context().Err())
	}
	return m, child, B4DefinitionSelectionKey{SessionID: started.SessionID, Key: result.Key, Transaction: owner.Transaction, CompletedOwnerKey: owner.CompletedOwnerKey}
}

func TestADR0011C17C15ObserverStopRestartUsesNewTransaction(t *testing.T) {
	t.Run("exact generation retirement removes only matching context", func(t *testing.T) {
		observer := privateB4C15BatchObserver(func([]privateB4C15Event) error { return nil })
		context, failure := newPrivateB4C15CustodyContext("session", 7, "transaction", observer)
		if failure != "" {
			t.Fatal(failure)
		}
		m := &Manager{privateB4C15Contexts: map[string]privateB4C15ManagerContext{
			"session": {generation: 7, observer: observer, contexts: map[string]*privateB4C15CustodyContext{"transaction": context}},
		}}
		m.retirePrivateB4C15ContextLocked("session", 6)
		if entry, present := m.privateB4C15Contexts["session"]; !present || entry.generation != 7 {
			t.Fatalf("ASSERT_C17_C15_FAILED_STOP_CONTEXT_PRESERVED present=%t generation=%d", present, entry.generation)
		}
		m.retirePrivateB4C15ContextLocked("session", 7)
		if entry, present := m.privateB4C15Contexts["session"]; present {
			t.Fatalf("ASSERT_C17_C15_STOP_CONTEXT_REMOVED generation=%d", entry.generation)
		}
		if !context.frozen {
			t.Fatal("ASSERT_C17_C15_STOP_CONTEXT_FROZEN")
		}
	})

	t.Run("successful restart executes successor private transaction", func(t *testing.T) {
		f := b4LeaseFixture(t)
		var generation1Mu sync.Mutex
		var generation1Events []privateB4C15Event
		generation1Observer := privateB4C15Observer(func(event privateB4C15Event) error {
			generation1Mu.Lock()
			generation1Events = append(generation1Events, event)
			generation1Mu.Unlock()
			return nil
		})
		generation1Snapshot := func() []privateC15ObservedEvent {
			generation1Mu.Lock()
			defer generation1Mu.Unlock()
			observed := make([]privateC15ObservedEvent, len(generation1Events))
			for i, event := range generation1Events {
				observed[i] = privateC15ObservedEvent{TransactionID: event.TransactionID, ObjectID: event.ObjectID, SiteID: event.SiteID, Kind: event.Kind, Sequence: event.Sequence, Terminal: event.Terminal}
			}
			return observed
		}
		m, old, key := privateC15ObserverStageBSelected(t, f, generation1Observer)
		successor, done := lifecycleSuccessor(t, f)
		m.mu.Lock()
		m.starter = oneChildStarter{successor}
		m.mu.Unlock()
		t.Cleanup(func() { _ = successor.Teardown(context.Background()); _ = successor.Close() })

		requestObserved := make(chan error, 1)
		allowResponse := make(chan struct{})
		served := make(chan error, 1)
		go func() {
			reader := lspwire.NewReader(old.input, lspwire.DefaultLimits())
			shutdown, err := reader.Read()
			if err == nil && (shutdown.Method != "shutdown" || string(shutdown.ID) != "2") {
				err = fmt.Errorf("unexpected shutdown frame method=%q id=%s", shutdown.Method, shutdown.ID)
			}
			requestObserved <- err
			if err != nil {
				served <- err
				return
			}
			select {
			case <-allowResponse:
			case <-t.Context().Done():
				served <- t.Context().Err()
				return
			}
			err = lspwire.NewWriter(old.output, lspwire.DefaultLimits()).Write(lspwire.Message{JSONRPC: lspwire.Version, ID: shutdown.ID, Result: []byte("null")})
			if err == nil {
				exit, readErr := reader.Read()
				err = readErr
				if err == nil && (exit.Method != "exit" || len(exit.ID) != 0) {
					err = fmt.Errorf("unexpected exit frame method=%q id=%s", exit.Method, exit.ID)
				}
			}
			served <- err
		}()

		restartResult := make(chan session.LifecycleResult, 1)
		go func() { restartResult <- m.Restart(t.Context(), key.SessionID, "c15-observer-restart") }()
		select {
		case err := <-requestObserved:
			if err != nil {
				t.Fatalf("BLOCKED_NOT_RED shutdown request observation: %v", err)
			}
		case <-t.Context().Done():
			t.Fatalf("BLOCKED_NOT_RED shutdown request observation: %v", t.Context().Err())
		}
		close(allowResponse)
		var intent session.LifecycleResult
		select {
		case intent = <-restartResult:
		case <-t.Context().Done():
			t.Fatalf("BLOCKED_NOT_RED restart admission: %v", t.Context().Err())
		}
		if intent.Failure != "" || intent.IntentID == "" {
			t.Fatalf("BLOCKED_NOT_RED restart not admitted: %+v", intent)
		}
		terminal := waitOperation(t, m, intent.IntentID, OperationComplete)
		if terminal.Failure != "" {
			t.Fatalf("BLOCKED_NOT_RED restart failed: %+v", terminal)
		}
		select {
		case err := <-served:
			if err != nil {
				t.Fatalf("BLOCKED_NOT_RED old shutdown: %v", err)
			}
		case <-t.Context().Done():
			t.Fatalf("BLOCKED_NOT_RED old shutdown not joined: %v", t.Context().Err())
		}

		generation1Frozen := generation1Snapshot()
		m.mu.Lock()
		_, generation1Present := m.privateB4C15Contexts[key.SessionID]
		m.mu.Unlock()
		generation1Terminal := false
		for i, event := range generation1Frozen {
			if event.TransactionID != key.Transaction || event.Sequence != uint64(i+1) {
				t.Fatalf("ASSERT_C17_C15_RESTART_GENERATION1_IDENTITY_SEQUENCE transaction=%q index=%d event=%+v", key.Transaction, i, event)
			}
			generation1Terminal = generation1Terminal || event.Terminal
		}
		if generation1Present || len(generation1Frozen) == 0 || !generation1Terminal {
			t.Fatalf("ASSERT_C17_C15_RESTART_GENERATION1_TERMINAL present=%t transaction=%q events=%+v", generation1Present, key.Transaction, generation1Frozen)
		}

		var generation2Mu sync.Mutex
		var generation2Events []privateB4C15Event
		generation2Observer := privateB4C15Observer(func(event privateB4C15Event) error {
			generation2Mu.Lock()
			generation2Events = append(generation2Events, event)
			generation2Mu.Unlock()
			return nil
		})
		generation2Snapshot := func() []privateC15ObservedEvent {
			generation2Mu.Lock()
			defer generation2Mu.Unlock()
			observed := make([]privateC15ObservedEvent, len(generation2Events))
			for i, event := range generation2Events {
				observed[i] = privateC15ObservedEvent{TransactionID: event.TransactionID, ObjectID: event.ObjectID, SiteID: event.SiteID, Kind: event.Kind, Sequence: event.Sequence, Terminal: event.Terminal}
			}
			return observed
		}
		if failure := installPrivateB4C15BatchObserverForTest(m, key.SessionID, 2, privateC15BatchObserverFromLegacy(generation2Observer)); failure != "" {
			t.Fatalf("BLOCKED_NOT_RED successor observer install failure=%s", failure)
		}
		req, owner := b4LeaseRequestForRestart(t, f, key.SessionID)
		req.Generation = 2
		req.Deadline = time.Now().Add(2 * time.Second)
		owner.TargetSources = []B4DefinitionSourceLease{privateC15ObserverSourceLease(t, m, f, key.SessionID, req.Generation)}
		var generation2Account *privateB4ByteAccountV2
		m.b4ParamsHook = func(got *privateB4ByteAccountV2) { generation2Account = got }
		result, successorLease := m.RoundTripPrivateB4(context.Background(), req, owner)
		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("BLOCKED_NOT_RED successor transaction: %v", err)
			}
		case <-t.Context().Done():
			t.Fatalf("BLOCKED_NOT_RED successor transaction did not complete: %v", t.Context().Err())
		}
		newKey := B4DefinitionSelectionKey{SessionID: key.SessionID, Key: result.Key, Transaction: owner.Transaction, CompletedOwnerKey: owner.CompletedOwnerKey}
		capture, status := consumePrivateB4SnapshotForTest(m, successorLease, newKey)
		generation2Frozen := generation2Snapshot()
		if result.Failure != "" || status != PrivateB4Selected || !bytes.Equal(capture.ResponseFrame, b4LeaseGet(t, f, "A/response.frame")) || len(generation2Frozen) == 0 {
			var snapshot privateB4ByteLedgerSnapshot
			if generation2Account != nil {
				snapshot = generation2Account.snapshot()
			}
			var last privateC15ObservedEvent
			if len(generation2Frozen) != 0 {
				last = generation2Frozen[len(generation2Frozen)-1]
			}
			t.Fatalf("ASSERT_C17_C15_RESTART_SUCCESSOR_PRIVATE_TRANSACTION result=%+v status=%s last_accepted=%+v events=%d account=%+v source_count=%d", result, status, last, len(generation2Frozen), snapshot, len(owner.TargetSources))
		}
		generation2Terminal := false
		for i, event := range generation2Frozen {
			if event.TransactionID != owner.Transaction || event.Sequence != uint64(i+1) {
				t.Fatalf("ASSERT_C17_C15_RESTART_GENERATION2_IDENTITY_SEQUENCE transaction=%q index=%d event=%+v", owner.Transaction, i, event)
			}
			generation2Terminal = generation2Terminal || event.Terminal
		}
		generation1After := generation1Snapshot()
		if owner.Transaction == key.Transaction || !generation2Terminal || len(generation1After) != len(generation1Frozen) {
			t.Fatalf("ASSERT_C17_C15_RESTART_GENERATION_ISOLATION generation1_transaction=%q generation2_transaction=%q generation1_before=%d generation1_after=%d generation2_terminal=%t", key.Transaction, owner.Transaction, len(generation1Frozen), len(generation1After), generation2Terminal)
		}
	})

	t.Run("failed restart emits no ledger events", func(t *testing.T) {
		clock := new(c08ScriptClock)
		starter := &sequenceStarter{children: []Child{referenceChild{}}}
		startedAt := time.Unix(1700004000, 0)
		limits := Limits{MaxSessions: 1, MaxRequests: 1, MaxChildren: 2, MaxCancels: 1, MaxTombstones: 2, MaxObservations: 64, MaxOperations: 4}
		clock.set(startedAt)
		m := newPrivateC15ObserverLifecycleManager(t, limits, starter, clock.now)
		t.Cleanup(func() { _ = m.Shutdown(context.Background()) })
		started := m.Start(context.Background(), StartRequest{Profile: profile(t)})
		if started.Failure != "" || m.ObserveInitialization(started.SessionID, started.Generation, true).State != session.Ready {
			t.Fatalf("BLOCKED_NOT_RED failed-restart start=%+v", started)
		}
		clock.set(startedAt.Add(30 * time.Second))
		ledger := newPrivateC15ObservedLedger()
		observer := privateB4C15Observer(ledger.observe)
		if failure := installPrivateB4C15BatchObserverForTest(m, started.SessionID, started.Generation, privateC15BatchObserverFromLegacy(observer)); failure != "" {
			t.Fatalf("BLOCKED_NOT_RED failed-restart observer install failure=%s", failure)
		}
		intent := m.Restart(context.Background(), started.SessionID+"-missing", "c15-observer-failed-restart")
		if intent.Failure != session.SessionNotFound || intent.IntentID != "" {
			t.Fatalf("BLOCKED_NOT_RED expected session-not-found: %+v", intent)
		}
		m.mu.Lock()
		entry, present := m.privateB4C15Contexts[started.SessionID]
		contextCount := len(m.privateB4C15Contexts)
		m.mu.Unlock()
		if !present || entry.generation != started.Generation || entry.observer == nil {
			t.Fatalf("ASSERT_C17_C15_FAILED_RESTART_CONTEXT_PRESERVED present=%t generation=%d", present, entry.generation)
		}
		if len(ledger.events) != 0 || ledger.transaction != "" || len(ledger.live) != 0 || len(ledger.terminal) != 0 || contextCount != 1 {
			t.Fatalf("ASSERT_C17_C15_FAILED_RESTART_NO_LEDGER events=%+v transaction=%q live=%v terminal=%v contexts=%d", ledger.events, ledger.transaction, ledger.live, ledger.terminal, contextCount)
		}
	})
}

func TestADR0011C17C15ObserverConcurrentExactOnceDeterministic(t *testing.T) {
	account := mustPrivateB4ByteAccountV2(t)
	var mu sync.Mutex
	var events []privateC15ObservedEvent
	privateC15InstallObserver(t, account, func(event privateB4C15Event) error {
		e, err := privateC15DecodeObservedEvent(event)
		if err != nil {
			return err
		}
		mu.Lock()
		events = append(events, e)
		mu.Unlock()
		return nil
	})
	lease, failure := account.reserve(8)
	if failure != "" {
		t.Fatal(failure)
	}
	const aliases = 32
	handles := make([]privateB4ByteLeaseV2, aliases)
	for i := range handles {
		handles[i] = lease.alias()
	}
	var released atomic.Uint64
	var wg sync.WaitGroup
	for i := range handles {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			handles[i].release()
			released.Add(1)
		}(i)
	}
	wg.Wait()
	lease.release()
	if released.Load() != aliases {
		t.Fatalf("BLOCKED_NOT_RED release fixture=%d", released.Load())
	}

	mu.Lock()
	defer mu.Unlock()
	kindCounts := make(map[string]int)
	terminalCounts := make(map[string]int)
	objectKinds := make(map[string]map[string]int)
	lastSequence := uint64(0)
	for _, event := range events {
		kindCounts[event.Kind]++
		if event.Terminal {
			terminalCounts[event.ObjectID]++
		}
		if objectKinds[event.ObjectID] == nil {
			objectKinds[event.ObjectID] = make(map[string]int)
		}
		objectKinds[event.ObjectID][event.Kind]++
		if event.Sequence == 0 || event.Sequence <= lastSequence {
			t.Fatalf("ASSERT_C17_C15_CONCURRENT_SEQUENCE_STRICTLY_MONOTONIC previous=%d event=%+v", lastSequence, event)
		}
		lastSequence = event.Sequence
	}
	if kindCounts["reserve"]+kindCounts["allocation"] != 1 || kindCounts["alias"] != aliases || kindCounts["release"]+kindCounts["settlement"]+kindCounts["terminal"] != 1 {
		t.Fatalf("ASSERT_C17_C15_CONCURRENT_EXACT_EVENT_CARDINALITY counts=%v events=%+v", kindCounts, events)
	}
	if len(objectKinds) != 1 || len(terminalCounts) != 1 {
		t.Fatalf("ASSERT_C17_C15_CONCURRENT_SHARED_AUTHORITY_SETTLED objects=%d terminals=%d events=%+v", len(objectKinds), len(terminalCounts), events)
	}
	for objectID, count := range terminalCounts {
		if count != 1 {
			t.Fatalf("ASSERT_C17_C15_CONCURRENT_SETTLED_ONCE object=%s terminals=%d events=%+v", objectID, count, events)
		}
	}
}

func TestADR0011C17C15ObserverAccepted44ExactCheckedAdjacentASTGuards(t *testing.T) {
	mapping := readPrivateB4C15Mapping(t)
	repo := filepath.Clean(filepath.Join(packageDir(t), ".."))
	verification, err := privateC15VerifyPersistedMapping(repo, mapping, privateC15SemanticContracts, nil)
	if err != nil {
		t.Fatalf("ASSERT_C17_C15_PERSISTED_RESOLVED_AGREEMENT: %v", err)
	}
	if len(verification.Resolved) != 44 || verification.UniqueKeys != 44 || verification.PersistedRows != 44 {
		t.Fatalf("ASSERT_C17_C15_SEMANTIC_EXACT44 resolved=%d keys=%d persisted=%d", len(verification.Resolved), verification.UniqueKeys, verification.PersistedRows)
	}
	t.Logf("C15 raw standalone marker uses=%d (diagnostic only; semantic denominator=44; alias43/44 are distinguished by canonical role/call selectors)", verification.RawMarkerUses)
}

func privateC15HasCanonicalCheckedObserverGuard(fset *token.FileSet, src []byte, file *ast.File, row privateB4C15MappingRow) bool {
	constName := privateB4C15ConstName(row.SiteID)
	matched := 0
	ast.Inspect(file, func(n ast.Node) bool {
		block, ok := n.(*ast.BlockStmt)
		if !ok {
			return true
		}
		bindingIndex, operationIndex, observerIndex := -1, -1, -1
		for i, stmt := range block.List {
			if privateC15NodeIsExactBinding(stmt, constName) {
				if bindingIndex >= 0 {
					return true
				}
				bindingIndex = i
			}
			if astDigest(fset, src, stmt) == row.ASTDigest && enclosingFunction(file, stmt.Pos()) == row.BindingFunction {
				if operationIndex >= 0 {
					return true
				}
				operationIndex = i
			}
			if privateC15NodeHasExactCheckedObserverCall(stmt, constName, row.SiteID) {
				if observerIndex >= 0 {
					return true
				}
				observerIndex = i
			}
		}
		if bindingIndex < 0 || operationIndex < 0 || observerIndex < 0 ||
			bindingIndex == operationIndex || bindingIndex == observerIndex || operationIndex == observerIndex ||
			operationIndex-bindingIndex != row.AdjacencyIndex || adjacencyRelation(row.AdjacencyIndex) != row.AdjacencyRelation {
			return true
		}
		minimum, maximum := bindingIndex, bindingIndex
		for _, index := range []int{operationIndex, observerIndex} {
			if index < minimum {
				minimum = index
			}
			if index > maximum {
				maximum = index
			}
		}
		if maximum-minimum == 2 {
			matched++
		}
		return true
	})
	return matched == 1
}

func privateC15NodeIsExactBinding(n ast.Node, constName string) bool {
	a, ok := n.(*ast.AssignStmt)
	if !ok || len(a.Lhs) != 1 || len(a.Rhs) != 1 {
		return false
	}
	lhs, lok := a.Lhs[0].(*ast.Ident)
	rhs, rok := a.Rhs[0].(*ast.Ident)
	return lok && rok && lhs.Name == "_" && rhs.Name == constName
}

func privateC15NodeHasExactCheckedObserverCall(n ast.Node, constName, siteID string) bool {
	matched := false
	ast.Inspect(n, func(node ast.Node) bool {
		ifStmt, ok := node.(*ast.IfStmt)
		if !ok || ifStmt.Init == nil || ifStmt.Cond == nil {
			return true
		}
		exactSite := privateC15NodeHasIdentifier(ifStmt.Init, constName) || privateC15NodeHasSite(ifStmt.Init, siteID)
		if !privateC15NodeHasObserverCall(ifStmt.Init) || !exactSite {
			return true
		}
		matched = privateC15NodeHasIdentifier(ifStmt.Cond, "failure") || privateC15NodeHasIdentifier(ifStmt.Cond, "err")
		return !matched
	})
	return matched
}

func TestADR0011C17C15ObserverCheckedAdjacentGuardModelIsSatisfiableAndRejectsSwap(t *testing.T) {
	const siteID = "c15.copy.02.params-ingress"
	constName := privateB4C15ConstName(siteID)
	satisfying := fmt.Sprintf("package model\nconst %s = %q\nfunc target() { _ = %s; mappedOperation(); if failure := observeCustody(%s); failure != nil { return } }\n", constName, siteID, constName, constName)
	swapped := fmt.Sprintf("package model\nconst %s = %q\nfunc target() { _ = %s; if failure := observeCustody(%s); failure != nil { return }; mappedOperation() }\n", constName, siteID, constName, constName)

	parse := func(source string) (*token.FileSet, *ast.File, []byte) {
		t.Helper()
		fset := token.NewFileSet()
		bytes := []byte(source)
		file, err := parser.ParseFile(fset, "model.go", bytes, parser.AllErrors)
		if err != nil {
			t.Fatal(err)
		}
		return fset, file, bytes
	}
	fset, file, source := parse(satisfying)
	var operation ast.Node
	ast.Inspect(file, func(n ast.Node) bool {
		stmt, ok := n.(*ast.ExprStmt)
		if ok && privateC15NodeHasIdentifier(stmt, "mappedOperation") {
			operation = stmt
			return false
		}
		return operation == nil
	})
	if operation == nil {
		t.Fatal("BLOCKED_NOT_RED model operation absent")
	}
	row := privateB4C15MappingRow{SiteID: siteID, Path: "model.go", ASTDigest: astDigest(fset, source, operation), BindingFunction: "target", AdjacencyRelation: "after", AdjacencyIndex: 1}
	if !privateC15HasCanonicalCheckedObserverGuard(fset, source, file, row) {
		t.Fatal("ASSERT_C17_C15_CHECKED_ADJACENT_MODEL_SATISFIABLE")
	}
	fset, file, source = parse(swapped)
	if privateC15HasCanonicalCheckedObserverGuard(fset, source, file, row) {
		t.Fatal("ASSERT_C17_C15_CHECKED_ADJACENT_MODEL_SWAP_REJECTED")
	}
}

func privateC15NodeHasIdentifier(n ast.Node, name string) bool {
	found := false
	ast.Inspect(n, func(node ast.Node) bool {
		ident, ok := node.(*ast.Ident)
		if ok && ident.Name == name {
			found = true
		}
		return !found
	})
	return found
}

func privateC15NodeHasSite(n ast.Node, site string) bool {
	found := false
	ast.Inspect(n, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if ok && lit.Kind == token.STRING {
			value, _ := strconv.Unquote(lit.Value)
			found = found || value == site
		}
		return !found
	})
	return found
}

func privateC15NodeHasObserverCall(n ast.Node) bool {
	found := false
	ast.Inspect(n, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch fun := call.Fun.(type) {
		case *ast.Ident:
			found = strings.Contains(strings.ToLower(fun.Name), "observe") && strings.Contains(strings.ToLower(fun.Name), "custody")
		case *ast.SelectorExpr:
			found = strings.Contains(strings.ToLower(fun.Sel.Name), "observe") && strings.Contains(strings.ToLower(fun.Sel.Name), "custody")
		}
		return !found
	})
	return found
}

func TestADR0011C17C15ObserverRecordCannotMutateCustody(t *testing.T) {
	account := mustPrivateB4ByteAccountV2(t)
	var observed any
	privateC15InstallObserver(t, account, func(event privateB4C15Event) error {
		observed = event
		return nil
	})
	lease, failure := account.reserve(3)
	if failure != "" {
		t.Fatal(failure)
	}
	before := account.snapshot()
	v := reflect.ValueOf(observed)
	if !v.IsValid() || v.Kind() != reflect.Struct || privateC15ContainsAuthority(v) {
		t.Fatalf("ASSERT_C17_C15_OBSERVER_NON_AUTHORITATIVE type=%T value=%+v", observed, observed)
	}
	if strings.Contains(reflect.TypeOf(observed).String(), "Lease") || account.snapshot() != before {
		t.Fatalf("ASSERT_C17_C15_OBSERVER_CANNOT_MUTATE_CUSTODY type=%T before=%+v after=%+v", observed, before, account.snapshot())
	}
	lease.release()
}

func TestADR0011C17C15RetainedNotificationCallerLeaseSurvivesEvaluationTerminal(t *testing.T) {
	run := func(t *testing.T, observed bool) (before, published, terminal privateB4ByteLedgerSnapshot, events []privateC15ObservedEvent) {
		t.Helper()
		account := mustPrivateB4ByteAccountV2(t)
		ledger := newPrivateC15ObservedLedger()
		if observed {
			privateC15InstallObserver(t, account, ledger.observe)
		}
		before = account.snapshot()
		owner, failure := newPrivateB4RetainedNotificationOwner(account, notificationFixture())
		if failure != "" || owner == nil {
			t.Fatalf("ASSERT_C15_RETAINED_NOTIFICATION_CREATE failure=%s owner=%p", failure, owner)
		}
		published = account.snapshot()
		lease := RetainedNotificationLease{owner: owner}

		// Models successful evaluation releasing its temporary account custody while
		// the exact same owner has already been published in the returned result.
		account.requestTerminalRelease()
		if err := lease.WithNotifications(func(messages []lspwire.Message) error {
			if len(messages) != 1 || messages[0].Method != "$/progress" {
				return errors.New("wrong retained notification result")
			}
			return nil
		}); err != nil {
			t.Fatalf("ASSERT_C15_RETAINED_NOTIFICATION_CALLER_VISIBLE observed=%t err=%v owner=%p", observed, err, owner)
		}
		if !lease.Release() || lease.Release() {
			t.Fatalf("ASSERT_C15_RETAINED_NOTIFICATION_CALLER_EXACT_ONCE observed=%t owner=%p", observed, owner)
		}
		terminal = account.snapshot()
		return before, published, terminal, append([]privateC15ObservedEvent(nil), ledger.events...)
	}

	plainBefore, plainPublished, plainTerminal, _ := run(t, false)
	observedBefore, observedPublished, observedTerminal, events := run(t, true)
	if plainBefore != observedBefore || plainPublished != observedPublished || plainTerminal != observedTerminal {
		t.Fatalf("ASSERT_C15_RETAINED_NOTIFICATION_OBSERVER_PARITY plain=%+v/%+v/%+v observed=%+v/%+v/%+v", plainBefore, plainPublished, plainTerminal, observedBefore, observedPublished, observedTerminal)
	}
	if observedTerminal.Live != 0 || observedPublished.Live <= observedBefore.Live {
		t.Fatalf("ASSERT_C15_RETAINED_NOTIFICATION_FINAL_DISPOSITION before=%+v published=%+v terminal=%+v", observedBefore, observedPublished, observedTerminal)
	}
	copyObjects := map[string]int{}
	terminalObjects := map[string]int{}
	for _, event := range events {
		if event.SiteID != privateC15SiteC15Copy28RetainedNotificationSet {
			continue
		}
		switch event.Kind {
		case "copy":
			copyObjects[event.ObjectID]++
		case "release":
			terminalObjects[event.ObjectID]++
		}
	}
	if len(copyObjects) != 1 || !reflect.DeepEqual(copyObjects, terminalObjects) {
		t.Fatalf("ASSERT_C15_RETAINED_NOTIFICATION_ONE_OBJECT_ONE_CHARGE_ONE_TERMINAL copies=%v terminals=%v events=%+v", copyObjects, terminalObjects, events)
	}
}
