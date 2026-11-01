//go:build darwin || linux

package privatehostproductv3

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	pilot "lsp-trace/internal/censuslineagepilot"
	prep "lsp-trace/internal/privatedescribeprep"
)

func testConfig(t *testing.T) Config {
	t.Helper()
	root, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	c := Config{Profile: Profile, Application: pilot.Digest([]byte("application")), ScopeSHA256: pilot.Digest([]byte("scope")), ProducerConfigSHA256: pilot.Digest([]byte("producer")), Store: filepath.Join(root, "app"), Requested: 20, AllowanceRevision: 1, Exchange: pilot.PageCostPrivateV3{LogicalBytes: 1 << 30, ReadHashBytes: 1 << 30, WriteBytes: 1 << 30, Work: 1 << 30}, Seeds: []uint64{3, 7}, MaxCalls: 40, MaxRequestBytes: 1 << 20, MaxResponseBytes: 64 << 10, MaxTotalBytes: 8 << 20}
	c.Preparation = prep.Limits{MaxTargets: 64, MaxPartBytes: 64 << 10, MaxTotalBytes: 8 << 20, MaxParts: 256, MaxSourceBytes: 2 << 20}
	c.RolePromptSHA256 = [2]string{PromptDigest("interpreter"), PromptDigest("reviewer")}
	c.ApprovalSHA256 = ConfigDigest(c)
	return c
}
func TestHostProductConfigPinsAndNoUnlimited(t *testing.T) {
	c := testConfig(t)
	if ValidateConfig(c) != nil {
		t.Fatal("ASSERT_CONFIG_VALID")
	}
	for _, mutate := range []func(*Config){func(c *Config) { c.ScopeSHA256 = "changed" }, func(c *Config) { c.MaxCalls = 0 }, func(c *Config) { c.Exchange = pilot.PageCostPrivateV3{} }, func(c *Config) { c.Profile = "PRIVATE_CENSUS_AST_V2" }, func(c *Config) { c.MaxRequestBytes = 2 << 20 }} {
		changed := c
		mutate(&changed)
		if ValidateConfig(changed) == nil {
			t.Fatal("ASSERT_CONFIG_MUTATION_REFUSED")
		}
	}
	c.HumanReceiptSHA256 = "separate"
	if ConfigDigest(c) == c.ApprovalSHA256 {
		t.Fatal("ASSERT_HUMAN_RECEIPT_NOT_APPROVAL_FIELD")
	}
}
func TestHostProductProductionOpenRejectsForeignScope(t *testing.T) {
	c := testConfig(t)
	if e := Initialize(c); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(c.Store, "scope.json"), []byte("{}"), 0600); e != nil {
		t.Fatal(e)
	}
	if e := Initialize(c); e == nil {
		t.Fatal("ASSERT_PRODUCTION_OPEN_FOREIGN_SCOPE_REFUSED")
	}
}
func TestHostProductOriginalClaimCreateOnly(t *testing.T) {
	c := testConfig(t)
	if Initialize(c) != nil || ClaimOriginal(c) != nil {
		t.Fatal("ASSERT_ORIGINAL_FIRST_CLAIM")
	}
	if ClaimOriginal(c) == nil {
		t.Fatal("ASSERT_ORIGINAL_REPLAY_REFUSED")
	}
}
func TestHostProductPhysicalMailboxOwnRoleAndParts(t *testing.T) {
	dir, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	if e = os.Chmod(dir, 0700); e != nil {
		t.Fatal(e)
	}
	for _, role := range []string{"interpreter", "reviewer"} {
		if MarkReady(dir, Ready{Protocol: Protocol, Role: role, LaunchID: role, PromptSHA256: PromptDigest(role)}) != nil {
			t.Fatal("ready")
		}
	}
	body := strings.Repeat("a", 19999) + "🧭" + strings.Repeat("b", 22000) + "尾"
	task := Task{Protocol: Protocol, Role: "reviewer", Sequence: 1, RequestDigest: pilot.Digest([]byte("request")), Body: body}
	mailbox := Mailbox{Directory: dir, MaxRequestBytes: 1 << 20, MaxResponseBytes: 64 << 10, ExchangeTimeout: 2 * time.Second}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { _, e := mailbox.Exchange(ctx, task); done <- e }()
	bundle, e := WaitBundle(ctx, dir, "reviewer", 0)
	if e != nil {
		t.Fatal(e)
	}
	if len(bundle.Parts) < 3 {
		t.Fatal("ASSERT_ALL_ORDERED_PARTS_EXPORTED")
	}
	got, e := ReadBundle(bundle)
	if e != nil || got.Body != body {
		t.Fatal("ASSERT_UTF8_TAIL_EXACT", e)
	}
	own, stop := context.WithTimeout(ctx, 30*time.Millisecond)
	defer stop()
	if _, e = WaitBundle(own, dir, "interpreter", 0); e == nil {
		t.Fatal("ASSERT_ROLE_METADATA_NOT_PARITY")
	}
	payload, _ := json.Marshal(Response{Protocol: Protocol, RequestDigest: task.RequestDigest})
	if Submit(dir, "interpreter", 1, payload) == nil {
		t.Fatal("ASSERT_FOREIGN_ROLE_RESPONSE_REFUSED")
	}
	if Submit(dir, "reviewer", 1, append(payload, []byte("{}")...)) == nil {
		t.Fatal("ASSERT_TRAILING_PAYLOAD_REFUSED")
	}
	legacyResponse, _ := json.Marshal(Response{Protocol: "PRIVATE_COHORT_V2", RequestDigest: task.RequestDigest})
	if Submit(dir, "reviewer", 1, legacyResponse) == nil {
		t.Fatal("ASSERT_NEW_PHYSICAL_REFUSES_LEGACY_RESPONSE")
	}
	if e = Submit(dir, "reviewer", 1, payload); e != nil {
		t.Fatal(e)
	}
	if e = <-done; e != nil {
		t.Fatal(e)
	}
	if Submit(dir, "reviewer", 1, payload) == nil {
		t.Fatal("ASSERT_LATE_RESPONSE_REPLAY_REFUSED")
	}
	legacy := task
	legacy.Protocol = "PRIVATE_COHORT_V2"
	legacy.Sequence = 2
	if _, e = mailbox.Exchange(ctx, legacy); e == nil {
		t.Fatal("ASSERT_NEW_REFUSES_OLD_PROTOCOL")
	}
}
func TestHostProductPairedReportNoReplace(t *testing.T) {
	c := testConfig(t)
	if Initialize(c) != nil {
		t.Fatal("init")
	}
	r, e := DeliverEarlyFailure(c, "HOST_LAUNCH", "HOST_TOOL_UNAVAILABLE")
	if e != nil || r.Completed != 0 || r.Pending != 20 {
		t.Fatal(e)
	}
	a := filepath.Join(c.Store, "result/report-00000001.json")
	b := filepath.Join(c.Store, "result/report-00000002.json")
	ai, e := os.Stat(a)
	if e != nil {
		t.Fatal(e)
	}
	bi, e := os.Stat(b)
	if e != nil || os.SameFile(ai, bi) {
		t.Fatal("ASSERT_DISTINCT_PAIRED_REPORT_INODES")
	}
	before, _ := os.ReadFile(a)
	if _, e = DeliverEarlyFailure(c, "HOST_LAUNCH", "SECOND_FAILURE"); e != nil {
		t.Fatal(e)
	}
	after, _ := os.ReadFile(a)
	if !bytes.Equal(before, after) {
		t.Fatal("ASSERT_PRIOR_REPORT_BYTES_IMMUTABLE")
	}
	human, _ := os.ReadFile(filepath.Join(c.Store, "result/inventory.md"))
	if !bytes.Contains(human, []byte("does not establish absence")) {
		t.Fatal("ASSERT_NO_LEADS_NOT_NO_FEATURES")
	}
}
func TestHostProductHeaderAdmissionBeforeEncoding(t *testing.T) {
	task := Task{Protocol: Protocol, Body: strings.Repeat("<", 1<<20)}
	if admitTask(task, 1<<20) {
		t.Fatal("ASSERT_OVERSIZE_FIELD_GRAPH_REFUSED")
	}
	if admitResponse(Response{Explanation: strings.Repeat("<", 64<<10)}, 64<<10) {
		t.Fatal("ASSERT_RESPONSE_FIELD_GRAPH_REFUSED")
	}
}
