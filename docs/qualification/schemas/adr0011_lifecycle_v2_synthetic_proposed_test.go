//go:build linux || darwin

package schemas_test

// Proposal-only synthetic oracles. No production publication, removal, or provider is invoked.
import (
	"bufio"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

const lifecycleProposalID = "https://jaresty.github.io/lsp-trace/schemas/adr0011-lifecycle-v2.proposed.schema.json"
const lifecycleDigest = "sha256:" + "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func lifecyclePinned(t *testing.T, path, want string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprintf("%x", sha256.Sum256(b)) != want {
		t.Fatalf("proposal pin changed: %s", path)
	}
	return b
}
func lifecycleIdentity() map[string]any {
	return map[string]any{"installation_id": lifecycleDigest, "root_uri": "file:///synthetic/root", "device": 1, "inode": 2}
}
func lifecycleRef(role string, host bool) map[string]any {
	r := map[string]any{"role_uri": lifecycleProposalID + "#/$defs/" + role, "selector": "adr0011-test-v2-" + strings.Repeat("a", 64) + ".json", "path": "file:///synthetic/root/record.json", "root_identity": lifecycleIdentity(), "digest": lifecycleDigest, "byte_length": 1, "no_follow": true, "single_link": true}
	if host {
		r["anchor_identity"] = lifecycleIdentity()
	}
	return r
}
func lifecycleVerification() map[string]any {
	return map[string]any{"no_replace": "VERIFIED", "readback": "EXACT", "directory_sync": "SUCCEEDED"}
}
func lifecycleBounds() map[string]any {
	return map[string]any{"objects": 0, "bytes": 0, "depth": 0, "work": 0, "complete": true}
}
func lifecycleRecord(name, domain string) map[string]any {
	ref := lifecycleRef("fenceIntent", false)
	host := lifecycleRef("hostHeadPrepare", true)
	r := map[string]any{"role": domain, "version": 2, "record_id": lifecycleDigest, "root_identity": lifecycleIdentity(), "epoch": 1}
	switch name {
	case "trustedHostRoot":
		r["genesis_head"] = lifecycleDigest
		r["anchor_uri"] = "file:///synthetic/anchor"
		r["anchor_identity"] = lifecycleIdentity()
		r["lock_selector"] = "adr0011-root.lock"
		r["provisioning_verification"] = lifecycleVerification()
	case "hostHeadPrepare":
		r["epoch"] = 2
		r["anchor_identity"] = lifecycleIdentity()
		r["previous_head"] = lifecycleDigest
		r["next_head"] = ref
		r["verification"] = lifecycleVerification()
	case "hostHeadCommit":
		r["epoch"] = 2
		r["anchor_identity"] = lifecycleIdentity()
		r["previous_head"] = lifecycleDigest
		r["next_head"] = ref
		r["prepare_ref"] = host
		r["transition_ref"] = ref
		r["verification"] = lifecycleVerification()
	case "headTransition":
		r["epoch"] = 2
		r["previous_head"] = lifecycleDigest
		r["next_head"] = ref
		r["successors"] = []any{}
		r["enumeration"] = lifecycleBounds()
		r["prepare_ref"] = host
		r["publication_verification"] = lifecycleVerification()
	case "closureSnapshot":
		r["anchored_head"] = lifecycleDigest
		r["decision_id"] = lifecycleDigest
		r["selected_dependencies"] = []any{lifecycleRef("fenceIntent", false)}
		r["dependents"] = []any{}
		r["pins"] = []any{}
		r["enumeration"] = lifecycleBounds()
		r["completeness_witness"] = ref
		r["verification"] = lifecycleVerification()
	case "fenceIntent":
		r["previous_head"] = lifecycleDigest
		r["decision_id"] = lifecycleDigest
		r["action"] = "PRIVACY_REVOKE"
		r["closure_ref"] = ref
		r["policy_digest"] = lifecycleDigest
		r["verification"] = lifecycleVerification()
	case "tombstoneChild":
		r["decision_id"] = lifecycleDigest
		r["fence_ref"] = ref
		r["closure_ref"] = ref
		r["dependent_ref"] = ref
		r["verification"] = lifecycleVerification()
	case "tombstoneRoot":
		r["decision_id"] = lifecycleDigest
		r["fence_ref"] = ref
		r["closure_ref"] = ref
		r["children"] = []any{ref}
		r["dependent_count"] = 1
		r["verification"] = lifecycleVerification()
	case "unlinkIntent":
		r["decision_id"] = lifecycleDigest
		r["fence_ref"] = ref
		r["closure_ref"] = ref
		r["tombstone_root_ref"] = ref
		r["target_ref"] = ref
		r["ordinal"] = 0
		r["pre_state"] = "EXACT_MATCH"
		r["syscall_target"] = ref["selector"]
		r["verification"] = lifecycleVerification()
	case "unlinkObservation":
		r["decision_id"] = lifecycleDigest
		r["intent_ref"] = ref
		r["invocation"] = "ATTRIBUTED"
		r["syscall_result"] = "UNLINKED"
		r["post_readback"] = "ABSENT"
		r["close_outcome"] = "SUCCEEDED"
		r["parent_directory_sync"] = "SUCCEEDED"
		r["verification"] = lifecycleVerification()
	case "cleanupTerminal":
		r["decision_id"] = lifecycleDigest
		r["fence_ref"] = ref
		r["closure_ref"] = ref
		r["root_ref"] = ref
		r["intents"] = []any{ref}
		r["observations"] = []any{ref}
		r["outcome"] = "REMOVED_VERIFIED"
		r["verification"] = lifecycleVerification()
	case "reconciliation":
		r["decision_id"] = lifecycleDigest
		r["fence_ref"] = ref
		r["observations"] = []any{ref}
		r["outcome"] = "KEEP_DENIAL"
		r["review_authority"] = ref
		r["verification"] = lifecycleVerification()
	}
	return r
}
func TestADR0011LifecycleV2ExactRoleShapesProposal(t *testing.T) {
	b := lifecyclePinned(t, "adr0011-lifecycle-v2.proposed.schema.json", "a8bc22e5cbf95cb317660cac115fbc7ce0f5c24c44b68af230721d45b2587801")
	lifecyclePinned(t, "../policies/adr0011-lifecycle-v2.proposed.json", "a2aef1faafde98da6b14d9596c6c727ab9c304cb638d2e9d7bde563e4ee345d0")
	lifecyclePinned(t, "../adr0011-lifecycle-v2-contract-freeze.proposed.md", "e1cc7100d13b7be236fee8dd7b65490e111c2f80a67cd6faff78295f83e5a5f2")
	var doc map[string]any
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	if doc["$schema"] != "https://json-schema.org/draft/2020-12/schema" || doc["$id"] != lifecycleProposalID {
		t.Fatal("wrong schema dialect or role namespace")
	}
	names := []string{"trustedHostRoot", "hostHeadPrepare", "hostHeadCommit", "headTransition", "closureSnapshot", "fenceIntent", "tombstoneChild", "tombstoneRoot", "unlinkIntent", "unlinkObservation", "cleanupTerminal", "reconciliation"}
	domains := []string{"TRUSTED_HOST_ROOT", "HOST_HEAD_PREPARE", "HOST_HEAD_COMMIT", "HEAD_TRANSITION", "CLOSURE_SNAPSHOT", "FENCE_INTENT", "TOMBSTONE_CHILD", "TOMBSTONE_ROOT", "UNLINK_INTENT", "UNLINK_OBSERVATION", "CLEANUP_TERMINAL", "RECONCILIATION"}
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	if err := c.AddResource(lifecycleProposalID, doc); err != nil {
		t.Fatal(err)
	}
	if len(doc["$defs"].(map[string]any)) < len(names) {
		t.Fatal("missing definitions")
	}
	for i, name := range names {
		t.Run(name, func(t *testing.T) {
			s, err := c.Compile(lifecycleProposalID + "#/$defs/" + name)
			if err != nil {
				t.Fatal(err)
			}
			r := lifecycleRecord(name, "ADR0011_"+domains[i]+"_V2")
			if err := s.Validate(r); err != nil {
				t.Fatalf("valid %s: %v", name, err)
			}
			for label, mutate := range map[string]func(map[string]any){"unknown": func(m map[string]any) { m["unexpected"] = true }, "missing": func(m map[string]any) { delete(m, "record_id") }, "wrong_role": func(m map[string]any) { m["role"] = "ADR0011_OTHER_V2" }} {
				t.Run(label, func(t *testing.T) {
					m := lifecycleRecord(name, "ADR0011_"+domains[i]+"_V2")
					mutate(m)
					if err := s.Validate(m); err == nil {
						t.Fatalf("accepted %s", label)
					}
				})
			}
			if name == "hostHeadPrepare" || name == "hostHeadCommit" {
				m := lifecycleRecord(name, "ADR0011_"+domains[i]+"_V2")
				delete(m, "anchor_identity")
				if s.Validate(m) == nil {
					t.Fatal("accepted missing host anchor")
				}
			}
		})
	}
}

func TestADR0011LifecycleV2ZeroDependentRevokeProposal(t *testing.T) {
	b := lifecyclePinned(t, "adr0011-lifecycle-v2.proposed.schema.json", "a8bc22e5cbf95cb317660cac115fbc7ce0f5c24c44b68af230721d45b2587801")
	var doc map[string]any
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	if err := c.AddResource(lifecycleProposalID, doc); err != nil {
		t.Fatal(err)
	}
	rootSchema, err := c.Compile(lifecycleProposalID + "#/$defs/tombstoneRoot")
	if err != nil {
		t.Fatal(err)
	}
	root := lifecycleRecord("tombstoneRoot", "ADR0011_TOMBSTONE_ROOT_V2")
	root["children"] = []any{}
	root["dependent_count"] = 0
	if err := rootSchema.Validate(root); err != nil {
		t.Fatalf("zero-dependent verified root rejected: %v", err)
	}
	closureSchema, err := c.Compile(lifecycleProposalID + "#/$defs/closureSnapshot")
	if err != nil {
		t.Fatal(err)
	}
	closure := lifecycleRecord("closureSnapshot", "ADR0011_CLOSURE_SNAPSHOT_V2")
	closure["selected_dependencies"] = []any{lifecycleRef("fenceIntent", false)}
	if err := closureSchema.Validate(closure); err != nil {
		t.Fatalf("selected dependency with no dependents rejected: %v", err)
	}
	closure["selected_dependencies"] = []any{}
	if err := closureSchema.Validate(closure); err == nil {
		t.Fatal("unowned empty closure admitted")
	}
	state := lifecycleGood()
	state.dependentCount, state.childCount = 0, 0
	if got := lifecycleDecision(state); got != "REMOVED_VERIFIED" {
		t.Fatalf("complete zero-dependent model: %s", got)
	}
}

// This is a deliberately independent model of the frozen policy, NOT a schema or production replay.
type lifecycleState struct {
	epoch, cachedEpoch                                                                                               int
	anchor, previous, next                                                                                           string
	prepares, commits, transitions                                                                                   int
	fence, childComplete, rootComplete, intent, observation, targetAbsent, lockVerified, inodeVerified, syncVerified bool
	dependentCount, childCount                                                                                       int
	action                                                                                                           string
}

func lifecycleDecision(s lifecycleState) string {
	if !s.lockVerified || !s.inodeVerified || s.cachedEpoch != s.epoch || s.prepares != 1 || s.commits != 1 || s.transitions != 1 || s.previous != s.anchor || s.next == s.anchor || s.epoch != 2 {
		return "DENY_HEAD_OR_LOCK"
	}
	if !s.fence {
		return "DENY_NO_FENCE"
	}
	if s.action == "REVOKE" && (!s.childComplete || !s.rootComplete || s.childCount != s.dependentCount) {
		return "DENY_PARTIAL_CUTOVER"
	}
	if !s.intent {
		return "DENY_NO_INTENT"
	}
	if !s.observation {
		return "EFFECT_UNCERTAIN_NO_AUTO_ABORT"
	}
	if !s.targetAbsent {
		return "DENY_UNEXPLAINED_STATE"
	}
	if !s.syncVerified {
		return "DENY_UNCERTAIN_DURABILITY"
	}
	return "REMOVED_VERIFIED"
}
func lifecycleGood() lifecycleState {
	return lifecycleState{epoch: 2, cachedEpoch: 2, anchor: "old", previous: "old", next: "new", prepares: 1, commits: 1, transitions: 1, fence: true, childComplete: true, rootComplete: true, intent: true, observation: true, targetAbsent: true, lockVerified: true, inodeVerified: true, syncVerified: true, dependentCount: 1, childCount: 1, action: "REVOKE"}
}
func TestADR0011LifecycleV2SyntheticNegativeOracleProposal(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*lifecycleState)
		want   string
	}{
		{"stale_epoch", func(s *lifecycleState) { s.cachedEpoch = 1 }, "DENY_HEAD_OR_LOCK"},
		{"fork", func(s *lifecycleState) { s.commits = 2 }, "DENY_HEAD_OR_LOCK"},
		{"gap", func(s *lifecycleState) { s.epoch = 3; s.cachedEpoch = 3 }, "DENY_HEAD_OR_LOCK"},
		{"missing_host_commit", func(s *lifecycleState) { s.commits = 0 }, "DENY_HEAD_OR_LOCK"},
		{"partial_child", func(s *lifecycleState) { s.childCount = 0 }, "DENY_PARTIAL_CUTOVER"},
		{"partial_root", func(s *lifecycleState) { s.rootComplete = false }, "DENY_PARTIAL_CUTOVER"},
		{"intent_without_observation_present", func(s *lifecycleState) { s.observation = false; s.targetAbsent = false }, "EFFECT_UNCERTAIN_NO_AUTO_ABORT"},
		{"intent_without_observation_absent", func(s *lifecycleState) { s.observation = false }, "EFFECT_UNCERTAIN_NO_AUTO_ABORT"},
		{"failed_directory_sync", func(s *lifecycleState) { s.syncVerified = false }, "DENY_UNCERTAIN_DURABILITY"},
		{"unknown_directory_sync", func(s *lifecycleState) { s.syncVerified = false }, "DENY_UNCERTAIN_DURABILITY"},
		{"unexplained_absence", func(s *lifecycleState) { s.intent = false }, "DENY_NO_INTENT"},
		{"no_mutex_fallback", func(s *lifecycleState) { s.lockVerified = false }, "DENY_HEAD_OR_LOCK"},
	}
	if got := lifecycleDecision(lifecycleGood()); got != "REMOVED_VERIFIED" {
		t.Fatalf("valid model: %s", got)
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := lifecycleGood()
			tc.mutate(&s)
			if got := lifecycleDecision(s); got != tc.want {
				t.Fatalf("got %s want %s", got, tc.want)
			}
		})
	}
}

// Helper A owns flock until killed. Helper B is an independent executable process.
func TestADR0011LifecycleV2FlockHelperProposal(t *testing.T) {
	if os.Getenv("ADR0011_LIFECYCLE_HELPER") != "1" {
		return
	}
	path := os.Getenv("ADR0011_LIFECYCLE_LOCK")
	f, err := os.OpenFile(path, os.O_RDWR|syscall.O_NOFOLLOW, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil || !st.Mode().IsRegular() {
		t.Fatal("not regular")
	}
	stat := st.Sys().(*syscall.Stat_t)
	if stat.Nlink != 1 {
		t.Fatal("not single link")
	}
	mode := os.Getenv("ADR0011_LIFECYCLE_MODE")
	if mode != "local" && os.Getenv("ADR0011_LIFECYCLE_RED_LOCAL") != "1" {
		err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	}
	if err != nil {
		fmt.Println("LOCK_DENIED")
		return
	}
	if mode == "A" {
		fmt.Println("LOCK_HELD")
		io.Copy(io.Discard, os.Stdin)
		return
	}
	b, err := os.ReadFile(os.Getenv("ADR0011_LIFECYCLE_FENCE"))
	if err != nil {
		t.Fatal(err)
	}
	if string(b) == "VERIFIED_FENCE\n" {
		fmt.Println("REACQUIRED_FENCE_DENIES_USE_PIN")
		return
	}
	fmt.Println("LOCK_ACQUIRED_NO_FENCE")
}
func TestADR0011LifecycleV2TwoProcessFlockProposal(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "adr0011-root.lock")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		t.Fatal(err)
	}
	pinned, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}
	pinnedInode := pinned.Sys().(*syscall.Stat_t).Ino
	f.Close()
	fence := filepath.Join(root, "fence")
	if err := os.WriteFile(fence, []byte("VERIFIED_FENCE\n"), 0600); err != nil {
		t.Fatal(err)
	}
	helper := func(mode string) *exec.Cmd {
		c := exec.Command(os.Args[0], "-test.run=^TestADR0011LifecycleV2FlockHelperProposal$")
		c.Env = append(os.Environ(), "ADR0011_LIFECYCLE_HELPER=1", "ADR0011_LIFECYCLE_LOCK="+path, "ADR0011_LIFECYCLE_FENCE="+fence, "ADR0011_LIFECYCLE_MODE="+mode)
		return c
	}
	a := helper("A")
	stdout, err := a.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdin, err := a.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err = a.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { stdin.Close(); a.Process.Kill(); a.Wait() }()
	ready := make(chan string, 1)
	go func() {
		s := bufio.NewScanner(stdout)
		if s.Scan() {
			ready <- s.Text()
		} else {
			ready <- "NO_READY"
		}
	}()
	select {
	case line := <-ready:
		if line != "LOCK_HELD" {
			t.Fatalf("A: %s", line)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("A readiness timeout")
	}
	b := helper("B")
	out, err := b.CombinedOutput()
	if err != nil {
		t.Fatalf("B: %s: %v", out, err)
	}
	if !strings.Contains(string(out), "LOCK_DENIED") {
		t.Fatalf("ASSERT_B_DENIED_WHILE_A_HOLDS: %s", out)
	}
	if err := a.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	stdin.Close()
	a.Wait()
	b = helper("B")
	out, err = b.CombinedOutput()
	if err != nil {
		t.Fatalf("B after kill: %s: %v", out, err)
	}
	if !strings.Contains(string(out), "REACQUIRED_FENCE_DENIES_USE_PIN") {
		t.Fatalf("ASSERT_REACQUIRED_FENCE_DENIES_USE_PIN: %s", out)
	}
	// Replace only this disposable lock path; a reacquired lock on a new inode is not the provisioned lock.
	if err := os.Rename(path, path+".old"); err != nil {
		t.Fatal(err)
	}
	replacement, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		t.Fatal(err)
	}
	changed, err := replacement.Stat()
	replacement.Close()
	if err != nil {
		t.Fatal(err)
	}
	if changed.Sys().(*syscall.Stat_t).Ino == pinnedInode {
		t.Fatal("ASSERT_CHANGED_LOCK_INODE_DENIED: replacement reused pinned inode")
	}
	for name, mutate := range map[string]func(*lifecycleState){"stale_epoch": func(s *lifecycleState) { s.cachedEpoch = 1 }, "changed_inode": func(s *lifecycleState) { s.inodeVerified = false }, "incomplete_root": func(s *lifecycleState) { s.rootComplete = false }} {
		t.Run(name, func(t *testing.T) {
			s := lifecycleGood()
			mutate(&s)
			if got := lifecycleDecision(s); got == "REMOVED_VERIFIED" {
				t.Fatal("synthetic replay admitted")
			}
		})
	}
}
