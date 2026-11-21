//go:build darwin || linux

package main

import (
	"bytes"
	"context"
	"lsp-trace/internal/scopejob"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestScopeJobStatusContextCancellation(t *testing.T) {
	store, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	if e = os.Chmod(store, 0700); e != nil {
		t.Fatal(e)
	}
	workspace, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	root, _, e := openSelectedCensusStore("", store)
	if e != nil {
		t.Fatal(e)
	}
	r, e := scopejob.Start(context.Background(), root, scopejob.Config{Workspace: workspace, Roots: []string{"."}, Profile: "gopls", Language: "go", Budget: scopejob.DefaultBudget()}, scopejob.Hooks{})
	closeErr := root.Close()
	if e != nil || closeErr != nil {
		t.Fatal(e, closeErr)
	}
	for _, kind := range []string{"cancel", "deadline", "success"} {
		t.Run(kind, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if kind == "cancel" {
				cancel()
			}
			if kind == "deadline" {
				var stop context.CancelFunc
				ctx, stop = context.WithDeadline(context.Background(), time.Now().Add(-time.Hour))
				defer stop()
			}
			var out, diag bytes.Buffer
			code := runCensusScopeJobStatusContext(ctx, []string{r.JobID, "--publication-root", store}, &out, &diag)
			if kind == "success" {
				if code != 0 || out.Len() == 0 {
					t.Fatalf("success code=%d diag=%s", code, &diag)
				}
			} else if code == 0 || out.Len() != 0 || diag.Len() == 0 {
				t.Fatalf("cancelled CLI report: code=%d out=%s diag=%s", code, &out, &diag)
			}
		})
	}
}
