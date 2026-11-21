package scopejob

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"lsp-trace/internal/publication"
)

// Explicit offline metadata-only probe. Never starts a provider or reads source
// contents. Artifacts are retained outside the workspace, not t.TempDir cleanup.
func TestScopeJobRepositoryMetadataProbe(t *testing.T) {
	workspace := os.Getenv("LSP_TRACE_SCOPE_JOB_PROBE_ROOT")
	directory := os.Getenv("LSP_TRACE_SCOPE_JOB_PROBE_REPORT")
	if workspace == "" || directory == "" {
		t.Skip("explicit metadata-only probe not selected")
	}
	store := filepath.Join(directory, "scope-store")
	if e := os.Mkdir(store, 0700); e != nil {
		t.Fatal(e)
	}
	root, e := publication.OpenRoot(store)
	if e != nil {
		t.Fatal(e)
	}
	defer root.Close()
	started := time.Now()
	r, e := Start(context.Background(), root, Config{Workspace: workspace, Roots: []string{"."}, Profile: "gopls", Language: "go", Budget: DefaultBudget()}, Hooks{})
	if e != nil {
		t.Fatal(e)
	}
	raw, e := json.MarshalIndent(r, "", "  ")
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(directory, "scope-report.json"), raw, 0600); e != nil {
		t.Fatal(e)
	}
	reopened, e := Inspect(root, r.JobID, time.Now().Add(48*time.Hour))
	if e != nil {
		t.Fatal(e)
	}
	if reopened.Spent != r.Spent || reopened.Counts != r.Counts || reopened.Expired {
		t.Fatal("reopen renewed spending or expired default job")
	}
	t.Logf("elapsed=%s job=%s state=%s entries=%d regular=%d directories=%d selected=%d retained=%d steps=%d logical=%d work=%d packets=%d", time.Since(started), r.JobID, r.State, r.Counts.Entries, r.Counts.Regular, r.Counts.Directories, r.Counts.Selected, r.RetainedBytes, r.Spent.Steps, r.Spent.LogicalBytes, r.Spent.Work, r.Spent.Packets)
	if r.State != "SCOPE_READY" {
		t.Fatalf("honest partial result retained: %s", r.Reason)
	}
}
