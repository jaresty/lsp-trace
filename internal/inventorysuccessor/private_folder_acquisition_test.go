package inventorysuccessor

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"lsp-trace/internal/publication"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/sessionruntime"
)

func TestLocalFolderOwnedCaptureBinding(t *testing.T) {
	for _, mode := range []string{"methods", "directions", "members"} {
		t.Run(mode, func(t *testing.T) {
			path := t.TempDir()
			if err := os.Chmod(path, 0700); err != nil {
				t.Fatal(err)
			}
			root, err := publication.OpenRoot(path)
			if err != nil {
				t.Fatal(err)
			}
			defer root.Close()
			selector, err := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: "test", Workspace: path, Profile: "go", EnvironmentReference: "offline"})
			if err != nil {
				t.Fatal(err)
			}
			deadline := time.Now().Add(10 * time.Second)
			limits := sessionruntime.PrivateDiscoveryLimits{SourceBytes: 16 * (1<<20 + 1), RequestBytes: 32 << 20, ResponseBytes: 16 << 20, WireBytes: 48 << 20, RetainedBytes: 512 << 20, Work: 1 << 32, Messages: 256, Requests: 128, FrameBytes: 1 << 20, HeaderBytes: 8192}
			c := sessionruntime.PrivateDiscoveryConstruction{Start: sessionruntime.StartRequest{Profile: runtimeprofile.Resolve(selector), Deadline: deadline}, Members: []sessionruntime.PrivateDiscoveryMember{{Path: "a.go", URI: "file://" + path + "/a.go", LanguageID: "go"}}, Limits: limits, Deadline: deadline}
			if mode == "directions" {
				c.CaptureOwnedMethods = true
			}
			members, runtime := PrivateDiscoveryConstructionIdentityV2(c)
			b := PrivateDiscoveryBindingV2{Revision: "e1b9c3b13ace77cd015ebe0f6c4bfe63d970ba73", PlanIdentity: hashLabelV2("plan"), DenominatorDigest: hashLabelV2("prepared"), MemberDigest: members, Workspace: path, RuntimeFingerprint: runtime, Deadline: deadline, Limits: limits}
			op, err := OpenLocalFolderDiscoveryV1(context.Background(), root, b, hashLabelV2("owned binding"))
			if err != nil {
				t.Fatal(err)
			}
			c.CaptureOwnedMethods = true
			if mode == "directions" {
				c.CaptureCallDirections = sessionruntime.PrivateCallsOutgoing
			}
			if mode == "members" {
				c.CaptureOwnedMethods = false // change membership only
				c.Members = append(c.Members, sessionruntime.PrivateDiscoveryMember{Path: "excluded.py", URI: "file://" + path + "/excluded.py", LanguageID: "python"})
			}
			changedMembers, changed := PrivateDiscoveryConstructionIdentityV2(c)
			if mode == "members" {
				changed, runtime = changedMembers, members
				if op.state.binding.DenominatorDigest != b.DenominatorDigest || op.state.owner.job.SourceAuthorization != b.DenominatorDigest {
					t.Fatal("full preparation source authorization replaced by selected membership")
				}
			}
			if changed == runtime || changed == "" {
				t.Fatal("ASSERT_OWNED_BOUND_BEFORE_EFFECT: retention mode absent from identity")
			}
			starts := 0
			c.NewStarter = func() (sessionruntime.Starter, error) { starts++; return nil, errors.New("must not construct") }
			if err := op.Execute(context.Background(), c, func(context.Context, *sessionruntime.PrivateDiscoverySession) error { return nil }); err == nil || starts != 0 {
				t.Fatal("ASSERT_OWNED_BOUND_BEFORE_EFFECT: mode retrofitted", err, starts)
			}
			if op.OwnedMethods().Available() {
				t.Fatal("ASSERT_OWNED_ACCOUNTING_BARRIER: output before completion")
			}
			if err := op.Complete(context.Background()); err == nil || op.OwnedMethods().Available() {
				t.Fatal("ASSERT_OWNED_ACCOUNTING_BARRIER: failed operation released originals", err)
			}
			t.Log("ASSERT_OWNED_BOUND_BEFORE_EFFECT: PASS; ASSERT_OWNED_ACCOUNTING_BARRIER: PASS")
		})
	}
}

func TestLocalFolderAcquisitionFreshBoundAdmission(t *testing.T) {
	path := t.TempDir()
	if err := os.Chmod(path, 0700); err != nil {
		t.Fatal(err)
	}
	root, err := publication.OpenRoot(path)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	selector, err := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: "test", Workspace: path, Profile: "go", EnvironmentReference: "offline"})
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(10 * time.Second)
	limits := sessionruntime.PrivateDiscoveryLimits{SourceBytes: 16 * (1<<20 + 1), RequestBytes: 32 << 20, ResponseBytes: 16 << 20, WireBytes: 48 << 20, RetainedBytes: 512 << 20, Work: 1 << 32, Messages: 256, Requests: 128, FrameBytes: 1 << 20, HeaderBytes: 8192}
	c := sessionruntime.PrivateDiscoveryConstruction{Start: sessionruntime.StartRequest{Profile: runtimeprofile.Resolve(selector), Deadline: deadline}, Members: []sessionruntime.PrivateDiscoveryMember{{Path: "a.go", URI: "file://" + path + "/a.go", LanguageID: "go"}}, Limits: limits, Deadline: deadline}
	members, runtime := PrivateDiscoveryConstructionIdentityV2(c)
	b := PrivateDiscoveryBindingV2{Revision: "e1b9c3b13ace77cd015ebe0f6c4bfe63d970ba73", PlanIdentity: hashLabelV2("plan"), DenominatorDigest: hashLabelV2("verified preparation metadata"), MemberDigest: members, Workspace: path, RuntimeFingerprint: runtime, Deadline: deadline, Limits: limits}
	attempt := hashLabelV2("fresh local invocation")
	// Enlarging even one existing currency is not a local-consent operation.
	tooLarge := b
	tooLarge.Limits.Requests++
	if _, err := OpenLocalFolderDiscoveryV1(context.Background(), root, tooLarge, attempt); err == nil {
		t.Fatal("ASSERT_FOLDER_FIXED_CEILING: enlarged request ceiling admitted")
	}
	entries, err := os.ReadDir(path)
	if err != nil || len(entries) != 0 {
		t.Fatal("ASSERT_FOLDER_FIXED_CEILING: rejection had accounting effects", err)
	}
	op, err := OpenLocalFolderDiscoveryV1(context.Background(), root, b, attempt)
	if err != nil {
		t.Fatal(err)
	}
	owner := op.state.owner
	pending, err := owner.k.reopen(context.Background(), owner.job, owner.limits)
	if err != nil || pending.Disposition != "PENDING" || pending.Windows != 1 || pending.Charged.ProviderRequests != 128 {
		t.Fatal("ASSERT_FOLDER_SEPARATE_RESERVATION: missing exact pending charge", err)
	}
	starts := 0
	c.NewStarter = func() (sessionruntime.Starter, error) {
		starts++
		head, err := owner.k.reopen(context.Background(), owner.job, owner.limits)
		if err != nil || head.Identity != pending.Identity || head.Charged != pending.Charged {
			t.Error("ASSERT_FOLDER_READBACK_BEFORE_EFFECT: pending changed", err)
		}
		return nil, errors.New("controlled constructor failure; no provider started")
	}
	if err := op.Execute(context.Background(), c, func(context.Context, *sessionruntime.PrivateDiscoverySession) error { return nil }); err == nil || starts != 1 {
		t.Fatal("ASSERT_FOLDER_CONSTRUCTOR_BOUNDARY", starts, err)
	}
	if err := op.Execute(context.Background(), c, func(context.Context, *sessionruntime.PrivateDiscoverySession) error { return nil }); err == nil || starts != 1 {
		t.Fatal("ASSERT_FOLDER_NO_REPLAY", starts, err)
	}
	if err := op.Complete(context.Background()); err == nil {
		t.Fatal("ASSERT_FOLDER_FAILED_EXECUTION_NOT_OBSERVED")
	}
	spent, err := owner.k.reopen(context.Background(), owner.job, owner.limits)
	if err != nil || spent.Charged != pending.Charged || spent.Windows != 1 || spent.Disposition == "OBSERVED" {
		t.Fatal("ASSERT_FOLDER_NO_REFUND", err)
	}
	if _, err := OpenLocalFolderDiscoveryV1(context.Background(), root, b, attempt); err == nil {
		t.Fatal("ASSERT_FOLDER_NO_REINITIALIZATION")
	}
	t.Log("ASSERT_FOLDER_FIXED_CEILING: PASS; ASSERT_FOLDER_SEPARATE_RESERVATION: PASS; ASSERT_FOLDER_READBACK_BEFORE_EFFECT: PASS; ASSERT_FOLDER_NO_REPLAY: PASS; ASSERT_FOLDER_NO_REFUND: PASS; ASSERT_FOLDER_NO_REINITIALIZATION: PASS")
}
