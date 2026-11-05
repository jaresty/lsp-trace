package adr0011methodresult

import (
	"context"
	"encoding/json"
	"fmt"
	"lsp-trace/internal/inventorysuccessor"
	"lsp-trace/internal/publication"
	"lsp-trace/sessionruntime"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// These fixed canonical TEST specimens were produced by the existing kernel's
// fixtureV2/sealV2/initialize path (TestPrivateManagedInputControlledSpecimensV2).
// Assembly below only removes repeated literal bytes; the kernel revalidates all
// identities on open. This is NOT another ledger or a production grant factory.
const groupingControlledJob = `{"schema":"lsp-trace.inventory-job-identity.private.v2","identity":"sha256:cfd1d773e48a5c4c13ee5534538fc8ad5800996f95c62db3fde8991e79d376cd","stable_id":"sha256:975a8adf2993cf97097506237528078940e20a0af849c6bd7d8725a1ad4eb0c0","application":"sha256:a172cedcae47474b615c54d510a5d84a8dea3032e958587430b413538be3f333","scope":"sha256:5f161c9149882e0e10124bc5dd5c11f0fbe8ec452edd52bcec76b01e9252cb33","source_authorization":"sha256:5f84fb87eb9f89631329da5335b51b987e717800bd6b2f64046f75d2cde37a8a","revision_policy":"sha256:20e9e5e3e0effffb5343d0e5fa952cc88879d07303b3ff748b615429312a57a4","evidence_policies":["sha256:ee8250fb76e094b34b471f13a73dbbe51d1ae142e9df59d7c0d31ec20f0a0a8e"],"provider_policies":["sha256:5c4c1964340aca5b65393bbe9d3249cdd71be26665b3320ad694f034f2743283"],"semantic_policies":["sha256:3784070fe3e7e3de5f0ec08eadfa10acbaa0f543916b1ab2c68f371924ff7db3"],"privacy":"sha256:a4cc6bc01a927e2a78fd3bec51e865ac0d85e4daab6f988d5d33d056e125b1c3","retention":"sha256:8e220124401da6e1a0288a86b48eb316d254cd2a9b9399cfc9a07a2fbc0253e8","resource_rules":"sha256:6c621d1a05138a7888d37d9269a9da8e2e11e4aced2f6cfd24b05ab1b9e61bb0","origin":"NEW_EMPTY"}`
const groupingControlledHeadID = "sha256:5d1a8be6a7e76928860c8b5e2273ae07d30a98a27e330d40b74c42a91be5865f"
const groupingControlledAuthID = "sha256:ffe4ef7e843a2659c6881228af4287bd5ea4eb77e8ef1640e621f675874f0326"
const groupingControlledStableID = "975a8adf2993cf97097506237528078940e20a0af849c6bd7d8725a1ad4eb0c0"

func groupingControlledCounters(n string) string {
	fields := []string{"logical_bytes", "read_hash_bytes", "write_bytes", "records", "parser_bytes", "provider_requests", "provider_messages", "work", "active_ns", "calls", "request_bytes", "response_bytes", "wire_bytes", "output_bytes"}
	rows := make([]string, len(fields))
	for i, k := range fields {
		rows[i] = fmt.Sprintf("%q:%s", k, n)
	}
	return "{" + strings.Join(rows, ",") + "}"
}
func groupingResourceOwner(t *testing.T) (*inventorysuccessor.PrivateManagedInputOwnerV2, *publication.Root) {
	t.Helper()
	path := t.TempDir()
	if err := os.Chmod(path, 0700); err != nil {
		t.Fatal(err)
	}
	zero, ceiling := groupingControlledCounters("0"), groupingControlledCounters("1000000000000")
	as := "inventory-successor/authorizations/private/v2/" + strings.TrimPrefix(groupingControlledAuthID, "sha256:") + ".json"
	a := fmt.Sprintf(`{"schema":"lsp-trace.inventory-job-authorization.private.v2","identity":%q,"job":"sha256:cfd1d773e48a5c4c13ee5534538fc8ad5800996f95c62db3fde8991e79d376cd","revision":1,"previous_authorization":"NONE","verified_resource_head":"NONE","charged_snapshot":%s,"currency_ceiling":%s,"max_windows":100,"expires_unix_nano":4102444800000000000,"max_tranche":%s,"max_window_ns":15000000000,"caller_decision_reference":"sha256:86ae35d58a6aa3b5742df94ef9d7162219f0106a911ae1954c1f0604aaec805d","identity_assurance":"CALLER_ASSERTED_NOT_AUTHENTICATED"}`+"\n", groupingControlledAuthID, zero, ceiling, ceiling)
	h := fmt.Sprintf(`{"schema":"lsp-trace.inventory-job-head.private.v2","identity":%q,"job":%s,"sequence":1,"predecessor_resource_head":"NONE","authorization_identity":%q,"authorization_selector":%q,"reservation_prefix_selector":"NONE","cumulative_charged":%s,"consumed_windows":0,"unresolved_attempt":"NONE","disposition":"READY","event":"INITIALIZE","at_unix_nano":10}`+"\n", groupingControlledHeadID, groupingControlledJob, groupingControlledAuthID, as, zero)
	for name, raw := range map[string]string{as: a, "inventory-successor/heads/private/v2/" + strings.TrimPrefix(groupingControlledHeadID, "sha256:") + ".json": h, "inventory-successor/heads/private/v2/initial/" + groupingControlledStableID + ".json": h, "inventory-successor/heads/private/v2/current/" + groupingControlledStableID + ".json": h} {
		p := filepath.Join(path, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(raw), 0600); err != nil {
			t.Fatal(err)
		}
	}
	root, err := publication.OpenRoot(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { root.Close() })
	owner, err := inventorysuccessor.OpenPrivateManagedInputOwnerV2(context.Background(), root, []byte(groupingControlledJob+"\n"), groupingControlledHeadID, inventorysuccessor.PrivateManagedInputVerificationV2{Records: 4096, Bytes: 64 << 20, Work: 16384})
	if err != nil {
		t.Fatal("CONTROLLED_EXISTING_KERNEL_ROOT:", err)
	}
	return owner, root
}
func groupingPermit(t *testing.T, m *sessionruntime.Manager, q PrivateManagedGroupingRequest, mode string, h sessionruntime.PrivateGroupingOwnedHandoff) PrivateManagedGroupingRequest {
	t.Helper()
	b, err := PrivateManagedGroupingOperationBinding(m, q, mode, h)
	if err != nil {
		return q
	}
	owner, _ := groupingResourceOwner(t)
	q.Operation, err = owner.Reserve(context.Background(), privateGroupingDigest([]byte("controlled-only-attempt")), b)
	// Invalid/expired request tests intentionally retain zero permission on refusal.
	if err != nil {
		q.Operation = inventorysuccessor.PrivateManagedInputOperationV2{}
	}
	return q
}
func groupingAcquire(t *testing.T, ctx context.Context, m *sessionruntime.Manager, q PrivateManagedGroupingRequest) (PrivateManagedGroupingInput, error) {
	return AcquirePrivateManagedGroupingInput(ctx, m, groupingPermit(t, m, q, "ACQUIRE", sessionruntime.PrivateGroupingOwnedHandoff{}))
}
func groupingIssue(t *testing.T, ctx context.Context, m *sessionruntime.Manager, h sessionruntime.PrivateGroupingOwnedHandoff, q PrivateManagedGroupingRequest) (PrivateManagedGroupingInput, error) {
	return IssuePrivateManagedGroupingInput(ctx, m, h, groupingPermit(t, m, q, "ISSUE", h))
}
func groupingReplay(t *testing.T, in PrivateManagedGroupingInput, root *publication.Root) error {
	if root != in.ReplayOperationBinding().Root {
		return ErrPrivateManagedGrouping
	}
	owner, _ := groupingResourceOwner(t)
	p, err := owner.Reserve(context.Background(), privateGroupingDigest([]byte("controlled-replay")), in.ReplayOperationBinding())
	if err != nil {
		return err
	}
	return in.ReplayReserved(context.Background(), p)
}
func TestPrivateManagedGroupingControlledSpecimenShape(t *testing.T) {
	owner, root := groupingResourceOwner(t)
	if owner == nil || root == nil {
		t.Fatal("CONTROLLED_ROOT_NOT_OPENED")
	}
	var j map[string]any
	if json.Unmarshal([]byte(groupingControlledJob), &j) != nil {
		t.Fatal("CONTROLLED_JOB")
	}
}

func groupingReserve(t *testing.T, owner *inventorysuccessor.PrivateManagedInputOwnerV2, m *sessionruntime.Manager, q PrivateManagedGroupingRequest, mode, attempt string, h sessionruntime.PrivateGroupingOwnedHandoff) PrivateManagedGroupingRequest {
	t.Helper()
	b, err := PrivateManagedGroupingOperationBinding(m, q, mode, h)
	if err != nil {
		t.Fatal(err)
	}
	q.Operation, err = owner.Reserve(context.Background(), privateGroupingDigest([]byte(attempt)), b)
	if err != nil {
		t.Fatal(err)
	}
	return q
}
func groupingAccount(t *testing.T, root *publication.Root) (uint64, map[string]uint64, string) {
	t.Helper()
	raw, err := root.ReadSelector("inventory-successor/heads/private/v2/current/"+groupingControlledStableID+".json", 64<<10)
	if err != nil {
		t.Fatal(err)
	}
	var h struct {
		Windows     uint64            `json:"consumed_windows"`
		Charged     map[string]uint64 `json:"cumulative_charged"`
		Disposition string            `json:"disposition"`
	}
	if json.Unmarshal(raw, &h) != nil {
		t.Fatal("accounting snapshot")
	}
	return h.Windows, h.Charged, h.Disposition
}
func TestPrivateManagedGroupingReservationBinding(t *testing.T) {
	for _, mode := range []string{"manager", "session", "generation", "workspace", "uri", "query", "revision", "privacy", "root", "deadline", "kind", "attempt-reuse"} {
		t.Run(mode, func(t *testing.T) {
			m, q, p := groupingFixture(t, true, true)
			owner, root := groupingResourceOwner(t)
			q = groupingReserve(t, owner, m, q, "ACQUIRE", "binding", sessionruntime.PrivateGroupingOwnedHandoff{})
			candidate := q
			switch mode {
			case "manager":
				m = new(sessionruntime.Manager)
			case "session":
				candidate.Session.SessionID += "x"
			case "generation":
				candidate.Session.Generation++
			case "workspace":
				candidate.Session.Workspace += "x"
			case "uri":
				candidate.URI += "x"
			case "query":
				candidate.Character++
			case "revision":
				candidate.Revision += "x"
			case "privacy":
				candidate.Privacy += "x"
			case "root":
				_, other, _ := groupingFixture(t, true, true)
				candidate.Root = other.Root
			case "deadline":
				candidate.Deadline = candidate.Deadline.Add(time.Second)
			case "kind":
				candidate.Kinds = candidate.Kinds[:1]
			case "attempt-reuse":
				if _, err := AcquirePrivateManagedGroupingInput(context.Background(), m, q); err != nil {
					t.Fatal(err)
				}
			}
			beforeM, beforeP := groupingMEDEffects(t, p)
			got, err := AcquirePrivateManagedGroupingInput(context.Background(), m, candidate)
			afterM, afterP := groupingMEDEffects(t, p)
			if err == nil || got.Input.ByteLength() != 0 || beforeM != afterM || beforeP != afterP {
				t.Fatalf("ASSERT_OPERATION_BINDING_BEFORE_EFFECTS: %s err=%v effects=%d/%d,%d/%d", mode, err, beforeM, afterM, beforeP, afterP)
			}
			windows, charge, _ := groupingAccount(t, root)
			if windows != 1 || charge["calls"] == 0 {
				t.Fatal("ASSERT_FAILED_USE_NO_REFUND")
			}
			t.Log("ASSERT_OPERATION_BINDING_BEFORE_EFFECTS: PASS")
		})
	}
}
func TestPrivateManagedGroupingPrepaidOnceAndReplay(t *testing.T) {
	m, q, p := groupingFixture(t, true, true)
	owner, root := groupingResourceOwner(t)
	q = groupingReserve(t, owner, m, q, "ACQUIRE", "whole-operation", sessionruntime.PrivateGroupingOwnedHandoff{})
	_, before, _ := groupingAccount(t, root)
	got, err := AcquirePrivateManagedGroupingInput(context.Background(), m, q)
	if err != nil {
		t.Fatal(err)
	}
	windows, after, disposition := groupingAccount(t, root)
	if windows != 1 || disposition != "OBSERVED" || fmt.Sprint(before) != fmt.Sprint(after) || after["provider_requests"] != 7 {
		t.Fatal("ASSERT_ACQUIRE_INITIAL_REPLAY_PREPAID_ONCE")
	}
	if got.Replay(q.Root) == nil {
		t.Fatal("ASSERT_LATER_REPLAY_NOT_FREE")
	}
	beforeM, beforeP := groupingMEDEffects(t, p)
	replay, err := owner.Reserve(context.Background(), privateGroupingDigest([]byte("later-replay")), got.ReplayOperationBinding())
	if err != nil {
		t.Fatal(err)
	}
	if err = got.ReplayReserved(context.Background(), replay); err != nil {
		t.Fatal(err)
	}
	if err = got.ReplayReserved(context.Background(), replay); err == nil {
		t.Fatal("ASSERT_REPLAY_PERMISSION_ONE_SHOT")
	}
	afterM, afterP := groupingMEDEffects(t, p)
	windows, after, _ = groupingAccount(t, root)
	if windows != 2 || after["provider_requests"] != 7 || after["calls"] <= before["calls"] || beforeM != afterM || beforeP != afterP {
		t.Fatal("ASSERT_LATER_REPLAY_SEPARATE_CHARGE_NO_CAPTURE")
	}
	t.Log("ASSERT_ACQUIRE_INITIAL_REPLAY_PREPAID_ONCE: PASS")
	t.Log("ASSERT_LATER_REPLAY_SEPARATE_CHARGE_NO_CAPTURE: PASS")
}
func TestPrivateManagedGroupingDirectIssueReservation(t *testing.T) {
	m, q, _ := groupingFixture(t, true, true)
	owner, root := groupingResourceOwner(t)
	if err := m.WithReadySessionLease(context.Background(), q.Session, func(ctx context.Context) error {
		h := groupingCapture(t, ctx, m, q)
		if _, err := IssuePrivateManagedGroupingInput(ctx, m, h, q); err == nil {
			t.Fatal("ASSERT_DIRECT_ISSUE_REQUIRES_RESERVATION")
		}
		q = groupingReserve(t, owner, m, q, "ISSUE", "direct-issue", h)
		got, err := IssuePrivateManagedGroupingInput(ctx, m, h, q)
		if err != nil || got.Input.ByteLength() == 0 {
			t.Fatalf("direct issuance: %v", err)
		}
		w, c, d := groupingAccount(t, root)
		if w != 1 || c["provider_requests"] != 0 || c["request_bytes"] != 0 || c["calls"] == 0 || d != "OBSERVED" {
			t.Fatal("ASSERT_DIRECT_ISSUE_OWN_CHARGE_WITHOUT_RECAPTURE")
		}
		original := string(got.Input.CanonicalBytes())
		replay, err := owner.Reserve(ctx, privateGroupingDigest([]byte("after-direct-issue")), got.ReplayOperationBinding())
		if err != nil {
			t.Fatal(err)
		}
		if err = got.ReplayReserved(ctx, replay); err != nil {
			t.Fatal(err)
		}
		w, c, d = groupingAccount(t, root)
		if w != 2 || d != "OBSERVED" || c["provider_requests"] != 0 || string(got.Input.CanonicalBytes()) != original {
			t.Fatal("ASSERT_DIRECT_ISSUE_NEXT_PAID_REPLAY_EXACT")
		}
		t.Log("ASSERT_DIRECT_ISSUE_NEXT_PAID_REPLAY_EXACT: PASS")
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	t.Log("ASSERT_DIRECT_ISSUE_OWN_CHARGE_WITHOUT_RECAPTURE: PASS")
}
func TestPrivateManagedGroupingReservedCancellation(t *testing.T) {
	m, q, p := groupingFixture(t, true, true)
	owner, root := groupingResourceOwner(t)
	entered := make(chan struct{}, 1)
	p.mu.Lock()
	p.dropMethod = "textDocument/references"
	p.entered = entered
	p.mu.Unlock()
	q = groupingReserve(t, owner, m, q, "ACQUIRE", "cancel-retains", sessionruntime.PrivateGroupingOwnedHandoff{})
	_, before, _ := groupingAccount(t, root)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := AcquirePrivateManagedGroupingInput(ctx, m, q); done <- err }()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("ASSERT_RESERVED_CANCEL_NATIVE_REACHED")
	}
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("ASSERT_RESERVED_CANCEL_REFUSED")
		}
	case <-time.After(4 * time.Second):
		t.Fatal("ASSERT_RESERVED_CANCEL_PROPAGATED")
	}
	w, after, d := groupingAccount(t, root)
	if w != 1 || fmt.Sprint(before) != fmt.Sprint(after) || d != "PENDING" {
		t.Fatal("ASSERT_RESERVED_CANCEL_NO_REFUND_STICKY")
	}
	t.Log("ASSERT_RESERVED_CANCEL_NO_REFUND_STICKY: PASS")
}
func TestPrivateManagedGroupingConcurrentPermit(t *testing.T) {
	m, q, p := groupingFixture(t, true, true)
	owner, root := groupingResourceOwner(t)
	q = groupingReserve(t, owner, m, q, "ACQUIRE", "concurrent-permit", sessionruntime.PrivateGroupingOwnedHandoff{})
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := AcquirePrivateManagedGroupingInput(context.Background(), m, q)
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	wins := 0
	for err := range results {
		if err == nil {
			wins++
		}
	}
	w, _, _ := groupingAccount(t, root)
	methods, _ := groupingMEDEffects(t, p)
	if wins != 1 || w != 1 || methods != 4 {
		t.Fatalf("ASSERT_CONCURRENT_PERMISSION_ONE_WINNER: wins=%d windows=%d methods=%d", wins, w, methods)
	}
	t.Log("ASSERT_CONCURRENT_PERMISSION_ONE_WINNER: PASS")
}

func TestPrivateManagedGroupingConcurrentReservationWinner(t *testing.T) {
	m, q, _ := groupingFixture(t, true, true)
	owner, root := groupingResourceOwner(t)
	b, err := PrivateManagedGroupingOperationBinding(m, q, "ACQUIRE", sessionruntime.PrivateGroupingOwnedHandoff{})
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	permits := make(chan inventorysuccessor.PrivateManagedInputOperationV2, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			p, err := owner.Reserve(context.Background(), privateGroupingDigest([]byte(fmt.Sprint("concurrent-reserve", i))), b)
			if err == nil {
				permits <- p
			}
		}(i)
	}
	wg.Wait()
	close(permits)
	count := 0
	for p := range permits {
		count++
		q.Operation = p
	}
	if count != 1 {
		t.Fatalf("ASSERT_CONCURRENT_RESERVE_SINGLE_WINNER: %d", count)
	}
	got, err := AcquirePrivateManagedGroupingInput(context.Background(), m, q)
	if err != nil || got.Input.ByteLength() == 0 {
		t.Fatalf("ASSERT_CONCURRENT_RESERVE_WINNER_REMAINS_USABLE: %v", err)
	}
	w, _, d := groupingAccount(t, root)
	if w != 1 || d != "OBSERVED" {
		t.Fatal("ASSERT_CONCURRENT_RESERVE_CHARGED_ONCE")
	}
	t.Log("ASSERT_CONCURRENT_RESERVE_WINNER_REMAINS_USABLE: PASS")
}

func TestPrivateManagedGroupingExpiredPermissionNoEffects(t *testing.T) {
	m, q, p := groupingFixture(t, true, true)
	owner, root := groupingResourceOwner(t)
	q.Deadline = time.Now().Add(100 * time.Millisecond)
	q = groupingReserve(t, owner, m, q, "ACQUIRE", "expired-permit", sessionruntime.PrivateGroupingOwnedHandoff{})
	time.Sleep(time.Until(q.Deadline) + time.Millisecond)
	beforeM, beforeP := groupingMEDEffects(t, p)
	got, err := AcquirePrivateManagedGroupingInput(context.Background(), m, q)
	afterM, afterP := groupingMEDEffects(t, p)
	if err == nil || got.Input.ByteLength() != 0 || beforeM != afterM || beforeP != afterP {
		t.Fatal("ASSERT_EXPIRED_PERMISSION_NO_EFFECTS")
	}
	w, c, d := groupingAccount(t, root)
	if w != 1 || c["calls"] == 0 || d != "PENDING" {
		t.Fatal("ASSERT_EXPIRED_RESERVATION_NO_REFUND")
	}
	t.Log("ASSERT_EXPIRED_PERMISSION_NO_EFFECTS: PASS")
}

func TestPrivateManagedGroupingReservationRequired(t *testing.T) {
	m, q, p := groupingFixture(t, true, true)
	// This request deliberately has no resource authorization. It is otherwise a
	// valid existing managed transaction, not a malformed fixture or missing API.
	beforeM, beforeP := groupingMEDEffects(t, p)
	got, err := AcquirePrivateManagedGroupingInput(context.Background(), m, q)
	afterM, afterP := groupingMEDEffects(t, p)
	if err == nil || got.Input.ByteLength() != 0 || beforeM != afterM || beforeP != afterP {
		t.Fatalf("ASSERT_RESERVATION_BEFORE_MANAGED_EFFECTS: err=%v methods=%d/%d publications=%d/%d", err, beforeM, afterM, beforeP, afterP)
	}
	t.Log("ASSERT_RESERVATION_BEFORE_MANAGED_EFFECTS: PASS")
}
