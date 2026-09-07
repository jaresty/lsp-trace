package sliceops

import (
	"context"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"lsp-trace/internal/operation"
	"lsp-trace/internal/retainedcalls"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/sessionruntime"
)

func TestRetainedCallsFakeWireMissingRangesAndRepetition(t *testing.T) {
	for _, mode := range []string{"empty_ranges", "repeated"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			file := filepath.Join(root, "f.go")
			if err := os.WriteFile(file, []byte("package p\nfunc F() {}\n"), 0600); err != nil {
				t.Fatal(err)
			}
			uri := (&url.URL{Scheme: "file", Path: file}).String()
			profile, err := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: "wire", Workspace: root, Profile: "go", EnvironmentReference: "test"})
			if err != nil {
				t.Fatal(err)
			}
			starter := &provenanceStarter{uri: uri, callsMode: mode}
			manager, err := sessionruntime.New(sessionruntime.Config{Limits: sessionruntime.Limits{MaxSessions: 1, MaxRequests: 8, MaxChildren: 1, MaxCancels: 8, MaxTombstones: 8, MaxObservations: 64}, Starter: starter})
			if err != nil {
				t.Fatal(err)
			}
			started := manager.Start(context.Background(), sessionruntime.StartRequest{Profile: runtimeprofile.Resolve(profile)})
			ready := manager.BeginReadiness(context.Background(), started.SessionID, 1, time.Now().Add(time.Second))
			observed, _ := manager.WaitReadiness(context.Background(), ready.ID)
			if observed.State != sessionruntime.ReadinessReady {
				t.Fatal(observed)
			}
			defer func() {
				manager.Stop(context.Background(), started.SessionID, "done")
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()
				_ = manager.Shutdown(ctx)
			}()
			args, _ := json.Marshal(map[string]any{"session_id": started.SessionID, "generation": 1, "start_mode": "at", "uri": uri, "line": 1, "character": 5, "graph_provenance": true})
			result, failure := NewExecutor(manager).Execute(context.Background(), operation.Request{Name: OperationSlice, Input: args})
			if failure != nil {
				t.Fatal(failure)
			}
			if err = os.RemoveAll(root); err != nil {
				t.Fatal(err)
			}
			raw, err := retainedcalls.Export(result.Artifact)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = retainedcalls.ValidateFor(raw, retainedcalls.Family, "v1"); err != nil {
				t.Fatal(err)
			}
			var e retainedcalls.Evidence
			_ = json.Unmarshal(raw, &e)
			want := 0
			if mode == "repeated" {
				want = 2
			}
			if len(e.Tables.Groups) != 1 || len(e.Tables.Occurrences) != want || e.Tables.SupportTotal != 1 || e.Ceilings.RepeatedReports != "UNAVAILABLE" {
				t.Fatalf("ASSERT_FAKE_WIRE_RETAINED_NOT_ACQUISITION_COUNT: %+v", e.Tables.Groups)
			}
			if want == 0 && e.Tables.Groups[0].CallsiteState != "UNREPORTED" {
				t.Fatal("ASSERT_FAKE_WIRE_EMPTY_RANGE_UNREPORTED")
			}
		})
	}
}
