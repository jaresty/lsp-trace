package censuschunking

import (
	"bytes"
	"testing"
)

func TestParseCoordinatorHeadRevisionStrict(t *testing.T) {
	head := coordinatorHead{SchemaVersion: CoordinatorHeadVersion, WorkflowKey: "workflow", RootSelector: "root", PlanDigest: "plan", IndexDigest: "index", SessionID: "session", SessionGeneration: 1, WorkspaceDigest: "workspace", Generation: 3, State: headCommitted, ClaimOrdinal: -1}
	setHeadIdentity(&head)
	raw, err := encodeCoordinator(head)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ParseCoordinatorHeadRevision(raw)
	if err != nil || got != (HeadRevision{Generation: head.Generation, Selector: head.ImmutableSelector}) {
		t.Fatalf("ASSERT_HEAD_REVISION_VALID got=%+v err=%v", got, err)
	}
	cases := map[string][]byte{
		"empty":        nil,
		"malformed":    []byte(`{"bad":true}`),
		"noncanonical": append(bytes.Clone(raw), '\n'),
		"tampered":     bytes.Replace(bytes.Clone(raw), []byte(`"generation":3`), []byte(`"generation":4`), 1),
	}
	for name, candidate := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseCoordinatorHeadRevision(candidate); err == nil {
				t.Fatal("ASSERT_HEAD_REVISION_REJECTS_INVALID")
			}
		})
	}
}
