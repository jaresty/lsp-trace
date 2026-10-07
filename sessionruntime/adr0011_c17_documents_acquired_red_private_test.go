package sessionruntime

import (
	"context"
	"errors"
	"os"
	"reflect"
	"sync"
	"testing"
	"time"

	"lsp-trace/internal/adr0011work"
	"lsp-trace/internal/session"
)

func c17DocumentsContext(identity string) (context.Context, *adr0011work.Account) {
	account := adr0011work.New(identity)
	return adr0011work.WithAccount(context.Background(), account), account
}

func c17DocumentsCount(t *testing.T, account *adr0011work.Account, want uint64) {
	t.Helper()
	got := account.Snapshot()
	if got.DocumentsAcquired != want {
		t.Fatalf("ASSERT_DOCUMENTS_ACQUIRED_COUNT got=%d want=%d snapshot=%+v", got.DocumentsAcquired, want, got)
	}
}

func TestADR0011C17DocumentsAcquiredDefaultOffExactResultParity(t *testing.T) {
	content := []byte("package parity\n")
	m, req, _, _ := supplyFixture(t, content)

	plain := m.prepareDocument(context.Background(), req, false)
	public := m.PrepareDocument(context.Background(), req)
	if !reflect.DeepEqual(plain, public) {
		t.Fatalf("ASSERT_DOCUMENTS_ACQUIRED_DEFAULT_OFF_RESULT_PARITY plain=%+v public=%+v", plain, public)
	}
}

func TestADR0011C17DocumentsAcquiredCacheFirstFreshRefreshCanonicalKey(t *testing.T) {
	content := []byte("package canonical\n")
	m, req, file, writer := supplyFixture(t, content)
	ctx, account := c17DocumentsContext("documents-canonical")

	preloaded := m.PrepareDocument(context.Background(), req)
	if preloaded.Failure != "" || preloaded.Supply == nil || preloaded.Supply.Method != "textDocument/didOpen" {
		t.Fatalf("BLOCKED_NOT_RED: real preload fixture failed result=%+v", preloaded)
	}
	c17DocumentsCount(t, account, 0)

	beforeCache := writer.Len()
	cached := m.PrepareDocument(ctx, req)
	if cached.Failure != "" || cached.Supply == nil || cached.Version != preloaded.Version || writer.Len() != beforeCache {
		t.Fatalf("ASSERT_DOCUMENTS_ACQUIRED_CACHE_FIRST_RESULT preloaded=%+v cached=%+v writes=%d/%d", preloaded, cached, beforeCache, writer.Len())
	}
	c17DocumentsCount(t, account, 1)

	repeated := m.PrepareDocument(ctx, req)
	if repeated.Failure != "" || repeated.Version != cached.Version || writer.Len() != beforeCache {
		t.Fatalf("ASSERT_DOCUMENTS_ACQUIRED_REPEATED_CACHE result=%+v writes=%d/%d", repeated, beforeCache, writer.Len())
	}
	c17DocumentsCount(t, account, 1)

	if err := os.WriteFile(file, []byte("package canonical\n\nvar Changed = true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	fresh := m.PrepareDocument(ctx, req)
	if fresh.Failure != "" || fresh.Supply == nil || fresh.Version <= cached.Version || writer.Len() <= beforeCache {
		t.Fatalf("BLOCKED_NOT_RED: real later fresh fixture failed cached=%+v fresh=%+v writes=%d/%d", cached, fresh, beforeCache, writer.Len())
	}
	c17DocumentsCount(t, account, 1)

	beforeRefresh := writer.Len()
	refreshed := m.RefreshDocumentSupply(ctx, req)
	if refreshed.Failure != "" || refreshed.Supply == nil || refreshed.Supply.Method != "textDocument/didChange" || writer.Len() <= beforeRefresh {
		t.Fatalf("BLOCKED_NOT_RED: real refresh fixture failed result=%+v writes=%d/%d", refreshed, beforeRefresh, writer.Len())
	}
	c17DocumentsCount(t, account, 1)
}

func TestADR0011C17DocumentsAcquiredFailedPrepareZero(t *testing.T) {
	content := []byte("package failure\n")
	cases := []struct {
		name   string
		mutate func(*DocumentRequest, string)
	}{
		{"stale-request", func(req *DocumentRequest, _ string) { req.Generation++ }},
		{"failed-source", func(_ *DocumentRequest, file string) { _ = os.Remove(file) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, req, file, writer := supplyFixture(t, content)
			ctx, account := c17DocumentsContext("documents-" + tc.name)
			tc.mutate(&req, file)
			got := m.PrepareDocument(ctx, req)
			if got.Failure == "" || got.Supply != nil || writer.Len() != 0 {
				t.Fatalf("ASSERT_DOCUMENTS_ACQUIRED_FAILED_PREPARE case=%s result=%+v writes=%d", tc.name, got, writer.Len())
			}
			c17DocumentsCount(t, account, 0)
		})
	}
}

func TestADR0011C17DocumentsAcquiredAliasFormsFollowExistingCanonicalPolicy(t *testing.T) {
	content := []byte("package aliases\n")
	for _, suffix := range []string{"?alias=1", "#alias"} {
		t.Run(suffix, func(t *testing.T) {
			m, req, _, writer := supplyFixture(t, content)
			ctx, account := c17DocumentsContext("documents-alias-" + suffix)
			req.URI += suffix
			got := m.PrepareDocument(ctx, req)
			if got.Failure != DocumentSupplyUnavailable || got.Supply != nil || writer.Len() != 0 {
				t.Fatalf("ASSERT_DOCUMENTS_ACQUIRED_ALIAS_EXISTING_POLICY suffix=%q result=%+v writes=%d", suffix, got, writer.Len())
			}
			c17DocumentsCount(t, account, 0)
		})
	}
}

func TestADR0011C17DocumentsAcquiredTransactionIdentityPreEffectAndTypedMapping(t *testing.T) {
	content := []byte("package identity\n")
	_, req, _, writer := supplyFixture(t, content)
	ctx, account := c17DocumentsContext("documents-owner")
	chosenIdentity := account.Snapshot().Identity
	if chosenIdentity != "documents-owner" {
		t.Fatalf("BLOCKED_NOT_RED: fixture account identity=%q", chosenIdentity)
	}
	if borrowed, ok := adr0011work.AccountFromContext(ctx); !ok || borrowed != account {
		t.Fatal("BLOCKED_NOT_RED: context did not retain fixture account")
	}
	called := false
	err := account.WithDocumentAcquisition(chosenIdentity+"-other", req.URI, func() (bool, error) {
		called = true
		return true, nil
	})
	if !errors.Is(err, adr0011work.ErrIdentityMismatch) || called || writer.Len() != 0 {
		t.Fatalf("ASSERT_DOCUMENTS_ACQUIRED_IDENTITY_TYPED_PRE_EFFECT err=%v called=%t writes=%d", err, called, writer.Len())
	}
	c17DocumentsCount(t, account, 0)
}

func TestADR0011C17DocumentsAcquiredConcurrentSameURIExecutesBothSourcesCountsOnce(t *testing.T) {
	content := []byte("package concurrent\n")
	m, req, _, _ := supplyFixture(t, content)
	ctx, account := c17DocumentsContext("documents-concurrent")

	entered := make(chan struct{}, 2)
	release := make(chan struct{})
	m.documentFinalHook = func() { entered <- struct{}{}; <-release }
	results := make(chan DocumentResult, 2)
	for range 2 {
		go func() { results <- m.PrepareDocument(ctx, req) }()
	}
	for range 2 {
		select {
		case <-entered:
		case <-time.After(time.Second):
			t.Fatal("BLOCKED_NOT_RED: both existing source callbacks did not execute")
		}
	}
	close(release)
	for range 2 {
		got := <-results
		if got.Failure != "" && got.Failure != session.LifecycleConflict {
			t.Fatalf("BLOCKED_NOT_RED: concurrent existing prepare behavior result=%+v", got)
		}
	}
	c17DocumentsCount(t, account, 1)
}

func TestADR0011C17DocumentsAcquiredSealWaitsAdmittedPrepare(t *testing.T) {
	content := []byte("package sealwait\n")
	m, req, _, _ := supplyFixture(t, content)
	ctx, account := c17DocumentsContext("documents-seal")
	entered := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	m.documentFinalHook = func() { once.Do(func() { close(entered) }); <-release }

	prepared := make(chan DocumentResult, 1)
	go func() { prepared <- m.PrepareDocument(ctx, req) }()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("BLOCKED_NOT_RED: real prepare was not admitted")
	}
	sealed := make(chan adr0011work.Snapshot, 1)
	go func() { sealed <- account.Seal() }()
	select {
	case got := <-sealed:
		t.Fatalf("ASSERT_DOCUMENTS_ACQUIRED_SEAL_WAITS_ADMITTED_PREPARE early=%+v", got)
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	if got := <-prepared; got.Failure != "" {
		t.Fatalf("BLOCKED_NOT_RED: admitted prepare failed result=%+v", got)
	}
	select {
	case got := <-sealed:
		if got.DocumentsAcquired != 1 || !got.Terminal {
			t.Fatalf("ASSERT_DOCUMENTS_ACQUIRED_SEAL_FINAL snapshot=%+v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("ASSERT_DOCUMENTS_ACQUIRED_SEAL_COMPLETES")
	}
}

func TestADR0011C17DocumentsAcquiredOverflowPreSource(t *testing.T) {
	t.Skip("BLOCKED_NOT_RED: public Account has no nonterminal documents_acquired max preload; foundation overflow is covered in internal/adr0011work")
}
