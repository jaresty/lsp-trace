//go:build darwin || linux

package privatedescribeprep

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAutomaticHandoffPendingUnsafe(t *testing.T) {
	for _, kind := range []string{"symlink", "hardlink", "mode", "empty", "cancel", "late"} {
		t.Run(kind, func(t *testing.T) {
			h, i, _ := handoffTest(t, 1)
			s := i.shared
			r := AutomaticRequestPrivateV2{Protocol: AutomaticProtocolPrivateV2, Stage: "NAVIGATE", PreparationSHA256: h.state.sha}
			raw := handoffFixtureResponse(i, r, 4096, []byte("[]"))
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan error, 1)
			var modeFile *os.File
			go func() {
				deadline := time.Now().Add(time.Second)
				for {
					if _, e := os.Stat(filepath.Join(s.cfg.Directory, "request-000001.json")); e == nil {
						break
					}
					if time.Now().After(deadline) {
						done <- fmt.Errorf("request absent")
						return
					}
					time.Sleep(time.Millisecond)
				}
				name := filepath.Join(s.cfg.Directory, "response-000001.json")
				target := filepath.Join(s.cfg.Directory, "foreign")
				var e error
				switch kind {
				case "symlink":
					e = os.WriteFile(target, raw, 0600)
					if e == nil {
						e = os.Symlink(target, name)
					}
				case "hardlink":
					e = os.WriteFile(target, raw, 0600)
					if e == nil {
						e = os.Link(target, name)
					}
				case "mode":
					modeFile, e = publishHandoffModeFixture(name, raw, 0644)
				case "empty":
					e = os.WriteFile(name, nil, 0600)
				case "cancel":
					cancel()
				case "late":
					time.Sleep(150 * time.Millisecond)
					e = os.WriteFile(name, raw, 0600)
				}
				done <- e
			}()
			_, e := i.Exchange(ctx, r, 4096)
			if he := <-done; he != nil {
				t.Fatal(he)
			}
			if modeFile != nil {
				defer modeFile.Close()
				logHandoffModeFixture(t, modeFile, raw)
				if er := verifyHandoffModeFixture(modeFile, filepath.Join(s.cfg.Directory, "response-000001.json"), 0644, len(raw)); er != nil {
					t.Fatal(er)
				}
			}
			if e == nil || !s.failed || s.sequence != 1 {
				t.Fatal("HANDOFF_PENDING_UNSAFE_TERMINAL: FAIL", kind, e)
			}
			if kind == "mode" {
				var terminal automaticHandoffFailure
				var backend AutomaticBackendErrorPrivateV2
				if !errors.As(e, &terminal) || !errors.As(e, &backend) || backend.Class != "UNAVAILABLE" {
					t.Fatalf("ASSERT_MODE_UNAVAILABLE A-fail %v", e)
				}
				t.Log("ASSERT_MODE_UNAVAILABLE A-pass")
			}
			if _, e = i.Exchange(context.Background(), r, 4096); e == nil || s.sequence != 1 {
				t.Fatal("HANDOFF_PENDING_UNSAFE_TERMINAL: FAIL subsequent send")
			}
			t.Log("HANDOFF_PENDING_UNSAFE_TERMINAL: PASS", kind)
		})
	}
}
