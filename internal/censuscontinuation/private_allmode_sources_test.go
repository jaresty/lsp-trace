package censuscontinuation

import (
	"context"
	"lsp-trace/internal/targetpacket"
	"testing"
	"time"
)

type sourceMissingStore struct {
	*MemoryStore
	puts int
}

func (s *sourceMissingStore) MaxObjectBytes() int64 { return 8 << 20 }
func (s *sourceMissingStore) Put(c context.Context, b []byte) (string, error) {
	s.puts++
	return s.MemoryStore.Put(c, b)
}
func TestPrivateSourcesLowerEntryRefused(t *testing.T) {
	h := privateMissingHandoff(t)
	c := testContract(t)
	l := c.CaptureLimits()
	q := PrivateAllModeSourcesRequest{PolicyID: c.PacketSourcePolicy().PolicyID, Limits: targetpacket.PrivateSourceLimits{MaxDocumentBytes: l.MaxSourceBytes, MaxTotalBytes: l.MaxTotalSourceBytes, MaxObjects: l.MaxBindings, MaxWork: l.MaxWork}, Deadline: time.Now().Add(5 * time.Second)}
	store := &sourceMissingStore{MemoryStore: NewMemoryStore()}
	ctx := context.Background()
	base := Run(ctx, Request{PrivateAllMode: &h, Workspace: h.wire.Workspace, Contract: c, Store: store, StopAfter: StopAfterSourceIntents})
	if base.Err != nil {
		t.Fatal(base.Err)
	}
	before := store.puts
	for _, stop := range []StopAfter{StopAfterPrivateSourceAdmissions, StopAfterPrivateSourceRequests} {
		out := Run(ctx, Request{PrivateAllMode: &h, PrivateSources: &q, Workspace: h.wire.Workspace, Contract: c, Store: store, StopAfter: stop})
		if out.Err == nil || store.puts != before {
			t.Fatal("ASSERT_LOWER_SOURCE_RUN_REFUSED_BEFORE_PUT")
		}
		resumed := Resume(ctx, ResumeRequest{PrivateSources: &PrivateAllModeSourcesResume{Version: PrivateAllModeSourcesVersion}, Selector: base.CheckpointID, Workspace: h.wire.Workspace, Store: store, StopAfter: stop})
		if resumed.Err == nil || store.puts != before {
			t.Fatal("ASSERT_LOWER_SOURCE_RESUME_REFUSED_BEFORE_PUT")
		}
	}
	old := Run(ctx, Request{PrivateAllMode: &h, Workspace: h.wire.Workspace, Contract: c, Store: store, StopAfter: StopAfterSourceIntents})
	if old.Err != nil || old.CheckpointID != base.CheckpointID {
		t.Fatal("ASSERT_SOURCE_V1_BYTES_UNCHANGED")
	}
	t.Log("ASSERT_LOWER_SOURCE_RUN_REFUSED_BEFORE_PUT: PASS; ASSERT_LOWER_SOURCE_RESUME_REFUSED_BEFORE_PUT: PASS; ASSERT_SOURCE_V1_BYTES_UNCHANGED: PASS")
}
