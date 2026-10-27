package censuslineagepilot

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"lsp-trace/internal/publication"
)

func preflightFixture(t *testing.T) (PreflightRequest, []byte, []byte) {
	t.Helper()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	home, err = filepath.EvalSymlinks(home)
	if err != nil {
		t.Fatal(err)
	}
	dir, err := os.MkdirTemp(home, ".lsp-trace-preflight-test-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(dir); err != nil {
			t.Error(err)
		}
	})
	if err = ValidateDurableRoot(dir); err != nil {
		t.Fatal("fixture root not admitted", err)
	}
	r, err := publication.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err = r.ValidatePrivate(); err != nil {
		t.Fatal(err)
	}
	if err = r.Close(); err != nil {
		t.Fatal(err)
	}
	sb, pb := inputFixture(t)
	refs := []Artifact{{Kind: SourceSelectionKind, Selector: Digest(sb), SHA256: Digest(sb), ByteLength: uint64(len(sb))}, {Kind: PartitionPlanKind, Selector: Digest(pb), SHA256: Digest(pb), ByteLength: uint64(len(pb))}}
	for i, raw := range [][]byte{sb, pb} {
		if err := os.WriteFile(filepath.Join(dir, refs[i].Selector), raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	return PreflightRequest{RootPath: dir, WorkflowKey: "workflow", LineageID: "lineage", Inputs: refs}, sb, pb
}

type diskEntry struct {
	Mode     os.FileMode
	Data     string
	Size     int64
	Modified time.Time
}

func preflightSnapshot(t *testing.T, dir string) map[string]diskEntry {
	t.Helper()
	out := map[string]diskEntry{}
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		info, e := d.Info()
		if e != nil {
			return e
		}
		data := ""
		if info.Mode().IsRegular() {
			raw, e := os.ReadFile(p)
			if e != nil {
				return e
			}
			data = string(raw)
		}
		out[p] = diskEntry{info.Mode(), data, info.Size(), info.ModTime()}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}
func TestPreflightDisk(t *testing.T) {
	req, sb, pb := preflightFixture(t)
	before := preflightSnapshot(t, req.RootPath)
	result, err := Preflight(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if result.WorkflowKey() != req.WorkflowKey || result.LineageID() != req.LineageID {
		t.Fatal("label binding")
	}
	if result.Authority() != 0 || result.Status() != "MONITORING_NOT_QUALIFIED" {
		t.Fatal("authority/status")
	}
	in := result.Inputs()
	if !bytes.Equal(in.SelectionBytes(), sb) || !bytes.Equal(in.PlanBytes(), pb) {
		t.Fatal("not actual disk bytes")
	}
	refs := in.Artifacts()
	refs[0].SHA256 = "changed"
	req.Inputs[0].SHA256 = "changed"
	changed := in.SelectionBytes()
	changed[0] = 'X'
	s := in.Selection()
	s.Files[0].Path = "changed"
	p := in.PartitionPlan()
	p.Plan.Files[0].Path = "changed"
	p.Plan.Partitions[0].Members[0].Path = "changed"
	if !bytes.Equal(result.Inputs().SelectionBytes(), sb) || !bytes.Equal(result.Inputs().PlanBytes(), pb) || result.Inputs().Artifacts()[0].SHA256 == "changed" {
		t.Fatal("alias")
	}
	if !reflect.DeepEqual(before, preflightSnapshot(t, req.RootPath)) {
		t.Fatal("preflight wrote disk")
	}
}
func TestPreflightReject(t *testing.T) {
	for _, name := range []string{"nil-context", "cancel", "deadline", "unsafe-root", "root-symlink", "file-symlink", "permissions", "missing", "truncated", "oversized", "selector", "foreign", "duplicate", "substitution", "length", "incoherent", "workflow", "lineage"} {
		t.Run(name, func(t *testing.T) {
			req, _, pb := preflightFixture(t)
			ctx := context.Background()
			path := filepath.Join(req.RootPath, req.Inputs[0].Selector)
			switch name {
			case "nil-context":
				ctx = nil
			case "cancel":
				c, cancel := context.WithCancel(ctx)
				cancel()
				ctx = c
			case "deadline":
				c, cancel := context.WithDeadline(ctx, time.Unix(0, 0))
				defer cancel()
				ctx = c
			case "unsafe-root":
				req.RootPath = "/private/tmp"
			case "root-symlink":
				link := filepath.Join(req.RootPath, "root-link")
				if e := os.Symlink(req.RootPath, link); e != nil {
					t.Fatal(e)
				}
				req.RootPath = link
			case "file-symlink":
				if e := os.Rename(path, path+".real"); e != nil {
					t.Fatal(e)
				}
				if e := os.Symlink(path+".real", path); e != nil {
					t.Fatal(e)
				}
			case "permissions":
				if e := os.Chmod(path, 0644); e != nil {
					t.Fatal(e)
				}
			case "missing":
				if e := os.Remove(path); e != nil {
					t.Fatal(e)
				}
			case "truncated":
				if e := os.WriteFile(path, []byte("{"), 0600); e != nil {
					t.Fatal(e)
				}
			case "oversized":
				if e := os.WriteFile(path, make([]byte, MaxSelectionInputBytes+1), 0600); e != nil {
					t.Fatal(e)
				}
			case "selector":
				req.Inputs[0].Selector = "../" + req.Inputs[0].Selector
			case "foreign":
				req.Inputs[0].Kind = "OTHER"
			case "duplicate":
				req.Inputs[1] = req.Inputs[0]
			case "substitution":
				if e := os.WriteFile(path, pb, 0600); e != nil {
					t.Fatal(e)
				}
			case "length":
				req.Inputs[0].ByteLength++
			case "incoherent":
				s, e := ParseSourceSelection(mustSelection(t, req))
				if e != nil {
					t.Fatal(e)
				}
				s.Workspace = "/other"
				for i := range s.Files {
					s.Files[i].URI = "file:///other/" + s.Files[i].Path
				}
				p, e := NewPartitionPlan(s, 1)
				if e != nil {
					t.Fatal(e)
				}
				raw, e := EncodePartitionPlan(p, s)
				if e != nil {
					t.Fatal(e)
				}
				req.Inputs[1] = Artifact{Kind: PartitionPlanKind, Selector: Digest(raw), SHA256: Digest(raw), ByteLength: uint64(len(raw))}
				if e := os.WriteFile(filepath.Join(req.RootPath, Digest(raw)), raw, 0600); e != nil {
					t.Fatal(e)
				}
			case "workflow":
				req.WorkflowKey = ""
			case "lineage":
				req.LineageID = ""
			}
			// Snapshot the admitted fixture's current directory, never an ambient unsafe root.
			fixtureDir := filepath.Dir(path)
			before := preflightSnapshot(t, fixtureDir)
			result, err := Preflight(ctx, req)
			if err == nil || result.Inputs().state != nil {
				t.Fatal("invalid preflight admitted", err)
			}
			if !reflect.DeepEqual(before, preflightSnapshot(t, fixtureDir)) {
				t.Fatal("failed preflight wrote disk")
			}
		})
	}
}
func mustSelection(t *testing.T, r PreflightRequest) []byte {
	t.Helper()
	b, e := os.ReadFile(filepath.Join(r.RootPath, r.Inputs[0].Selector))
	if e != nil {
		t.Fatal(e)
	}
	return b
}

type faultPreflightRoot struct {
	preflightRoot
	closeErr, errorRead error
	closed              bool
}

func (r *faultPreflightRoot) Close() error {
	r.closed = true
	return errors.Join(r.preflightRoot.Close(), r.closeErr)
}
func (r *faultPreflightRoot) ReadContentObserved(c context.Context, s string, n int64) ([]byte, error) {
	if r.errorRead != nil {
		return nil, r.errorRead
	}
	return r.preflightRoot.ReadContentObserved(c, s, n)
}
func TestPreflightReadCloseFailures(t *testing.T) {
	for _, name := range []string{"read", "close", "both"} {
		t.Run(name, func(t *testing.T) {
			req, _, _ := preflightFixture(t)
			sentinelRead := errors.New("read fault")
			sentinelClose := errors.New("root close fault")
			r, e := publication.OpenRoot(req.RootPath)
			if e != nil {
				t.Fatal(e)
			}
			f := &faultPreflightRoot{preflightRoot: r}
			t.Cleanup(func() {
				if !f.closed {
					if err := r.Close(); err != nil {
						t.Error(err)
					}
				}
			})
			if name != "read" {
				f.closeErr = sentinelClose
			}
			if name != "close" {
				f.errorRead = sentinelRead
			}
			got, e := preflightWithOpen(context.Background(), req, func(string) (preflightRoot, error) { return f, nil })
			if e == nil || got.Inputs().state != nil || !f.closed {
				t.Fatal("failure leaked state/handle", e)
			}
			if name != "read" && !errors.Is(e, sentinelClose) {
				t.Fatal("root close cause lost", e)
			}
			if name != "close" && !errors.Is(e, sentinelRead) {
				t.Fatal("read cause lost", e)
			}
		})
	}
}
