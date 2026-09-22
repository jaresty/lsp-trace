package main

import (
	"encoding/json"
	"testing"
	"time"

	"lsp-trace/internal/sourceprojectionv3"
)

func retainedContinuation(digest string, expires time.Time, bytes int64) structuralContextContinuation {
	return structuralContextContinuation{
		requestDigest: digest,
		structural:    json.RawMessage(validStructuralV2),
		expires:       expires,
		bytes:         bytes,
		page: func(string) (sourceprojectionv3.Page, error) {
			return sourceprojectionv3.Page{Header: sourceprojectionv3.Header{SchemaVersion: sourceprojectionv3.SchemaVersion}, Complete: true}, nil
		},
	}
}

func TestStructuralContextContinuationHostEvictsOldestByEntriesAndBytes(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	e := &unifiedStructuralContextV2Executor{
		continuations:          map[string]structuralContextContinuation{},
		continuationNow:        func() time.Time { return now },
		continuationTTL:        time.Minute,
		continuationMaxEntries: 2,
		continuationMaxBytes:   20,
	}
	e.continuationMu.Lock()
	if !e.retainContinuationLocked("a", retainedContinuation("a", now.Add(time.Minute), 10), now) ||
		!e.retainContinuationLocked("b", retainedContinuation("b", now.Add(time.Minute), 10), now) ||
		!e.retainContinuationLocked("c", retainedContinuation("c", now.Add(time.Minute), 10), now) {
		t.Fatal("ASSERT_CONTINUATION_HOST_ADMISSION")
	}
	e.continuationMu.Unlock()
	if _, ok := e.continuations["a"]; ok || len(e.continuations) != 2 || e.continuationBytes != 20 {
		t.Fatalf("ASSERT_CONTINUATION_HOST_OLDEST_ENTRY_EVICTION: keys=%v bytes=%d", e.continuations, e.continuationBytes)
	}
	if e.continuations["b"].sequence >= e.continuations["c"].sequence {
		t.Fatal("ASSERT_CONTINUATION_HOST_SEQUENCE_ORDER")
	}
}

func TestStructuralContextContinuationExpiryAndMismatchDoNotReacquireOrConsume(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	e := &unifiedStructuralContextV2Executor{
		continuations:          map[string]structuralContextContinuation{},
		continuationNow:        func() time.Time { return now },
		continuationTTL:        time.Minute,
		continuationMaxEntries: 4,
		continuationMaxBytes:   1 << 20,
	}
	e.continuationMu.Lock()
	if !e.retainContinuationLocked("valid", retainedContinuation("request", now.Add(time.Minute), 10), now) ||
		!e.retainContinuationLocked("expired", retainedContinuation("request", now, 10), now.Add(-time.Second)) {
		t.Fatal("ASSERT_CONTINUATION_HOST_SETUP")
	}
	e.continuationMu.Unlock()
	if _, failure, handled := e.resume("valid", "changed", 1<<20); !handled || failure == nil {
		t.Fatalf("ASSERT_CONTINUATION_MISMATCH_TYPED: handled=%t failure=%v", handled, failure)
	}
	if _, ok := e.continuations["valid"]; !ok {
		t.Fatal("ASSERT_CONTINUATION_MISMATCH_NON_CONSUMING")
	}
	now = now.Add(time.Second)
	if _, failure, handled := e.resume("expired", "request", 1<<20); !handled || failure == nil {
		t.Fatalf("ASSERT_CONTINUATION_EXPIRY_TYPED: handled=%t failure=%v", handled, failure)
	}
	if _, ok := e.continuations["expired"]; ok {
		t.Fatal("ASSERT_CONTINUATION_EXPIRY_CLEANUP")
	}
	if _, failure, handled := e.resume("valid", "request", 1<<20); !handled || failure != nil {
		t.Fatalf("ASSERT_CONTINUATION_VALID_AFTER_MISMATCH: handled=%t failure=%v", handled, failure)
	}
}
