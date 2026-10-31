//go:build darwin || linux

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	pilot "lsp-trace/internal/censuslineagepilot"
)

// Err observes exact held directory-FD identity at existing IO checkpoints.
// There is no polling goroutine, timing window, blocking FIFO open, or IO hook.
type applicationDirectoryFDObserverPrivate struct {
	context.Context
	device int32
	inode  uint64
	seen   bool
}

func (c *applicationDirectoryFDObserverPrivate) Err() error {
	for fd := 0; fd < 1024; fd++ {
		var st syscall.Stat_t
		if syscall.Fstat(fd, &st) == nil && int32(st.Dev) == c.device && uint64(st.Ino) == c.inode {
			c.seen = true
		}
	}
	return c.Context.Err()
}
func TestCensusApplicationRepairNoOutsideOpenPrivate(t *testing.T) {
	s, r, _, _ := applicationMockCompletedPrivate(t)
	outside := filepath.Join(t.TempDir(), "outside")
	if e := os.Mkdir(outside, 0700); e != nil {
		t.Fatal(e)
	}
	trap := filepath.Join(outside, "trap.json")
	if e := syscall.Mkfifo(trap, 0600); e != nil {
		t.Fatal(e)
	}
	var st syscall.Stat_t
	if e := syscall.Stat(outside, &st); e != nil {
		t.Fatal(e)
	}
	observer := &applicationDirectoryFDObserverPrivate{Context: context.Background(), device: int32(st.Dev), inode: uint64(st.Ino)}
	s.store.ctx = observer
	// Positive sensitivity control: the same bounded external adapter opens the
	// owned outside directory, opens FIFO nonblocking, then refuses nonregular IO.
	_, e := s.store.external(trap, 1024)
	assertInventoryPrivate(t, "ASSERT_REPAIR_OUTSIDE_FD_CONTROL", e != nil && observer.seen)
	observer.seen = false
	r.FilePages = append([]censusInventoryPagePrivate(nil), r.FilePages...)
	rel, e := filepath.Rel(r.AttemptDirectory, trap)
	if e != nil {
		t.Fatal(e)
	}
	r.FilePages[0].Selector = rel
	e = s.verifyTerminalPages(r.AttemptDirectory, r)
	assertInventoryPrivate(t, "ASSERT_REPAIR_NO_OUTSIDE_OPEN", e != nil && strings.Contains(e.Error(), "terminal denominator") && !observer.seen)
}

func TestCensusApplicationRepairTerminalPagesBoundaryPrivate(t *testing.T) {
	for _, kind := range []string{"control", "inline", "missing-middle", "duplicate", "reorder-pages", "extra", "entries", "unknown-row", "row-order", "future-complete", "source-classification", "empty-origin", "selected-status"} {
		t.Run(kind, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "attempt")
			if e := os.Mkdir(root, 0700); e != nil {
				t.Fatal(e)
			}
			store, e := openCensusApplicationStorePrivate(context.Background(), root, 16<<20)
			if e != nil {
				t.Fatal(e)
			}
			defer store.close()
			initial := make([]censusInventoryFilePrivate, 257)
			for i := range initial {
				initial[i].File.Path = fmt.Sprintf("file-%03d.go", i)
				initial[i].Disposition = "RESOURCE_GAP_ORIGIN_LIMIT"
				initial[i].Origin = -1
			}
			initial[5].Disposition = "EXCLUDED"
			initial[6].Disposition = "UNSUPPORTED_LANGUAGE"
			initial[7].Disposition = "READ_FAILED"
			rows := append([]censusInventoryFilePrivate(nil), initial...)
			for _, i := range []int{0, 1} {
				rows[i].Disposition = "COMPUTED_PRIVATE_PER_BATCH"
				rows[i].Origin = 0
			}
			rows[128].Disposition = "SUCCESSFUL_EMPTY"
			rows[128].Origin = 1
			pages, e := retainCensusInventoryPagesPrivate(root, "terminal-files", rows)
			if e != nil {
				t.Fatal(e)
			}
			s := &censusApplicationProgressPrivate{store: store, rows: initial, config: censusInventoryConfigPrivate{Application: &censusApplicationConfigPrivate{MaxArtifactBytes: 1 << 20}}}
			r := censusInventoryReportPrivate{FileDenominator: 257, FilePages: pages, Origins: []censusInventoryOriginPrivate{{Ordinal: 0, Files: []int{0, 1}, Status: "COMPUTED_PRIVATE_PER_BATCH"}, {Ordinal: 1, Files: []int{128}, Status: "SUCCESSFUL_EMPTY"}}}
			if e = s.verifyTerminalPages(root, r); e != nil {
				t.Fatal("valid 128/128/1 control", e)
			}
			switch kind {
			case "inline":
				r.Files = append([]censusInventoryFilePrivate(nil), rows...)
			case "missing-middle":
				r.FilePages = []censusInventoryPagePrivate{pages[0], pages[2]}
			case "duplicate":
				r.FilePages = []censusInventoryPagePrivate{pages[0], pages[0], pages[2]}
			case "reorder-pages":
				r.FilePages = []censusInventoryPagePrivate{pages[1], pages[0], pages[2]}
			case "extra":
				r.FilePages = append(r.FilePages, pages[2])
			case "entries":
				r.FilePages[1].Entries--
			case "empty-origin":
				r.Origins[1].Status = "COMPUTED_PRIVATE_PER_BATCH"
			case "selected-status":
				r.Origins[0].Status = "UNATTEMPTED"
			case "unknown-row", "row-order", "future-complete", "source-classification":
				if kind == "row-order" {
					rows[128], rows[129] = rows[129], rows[128]
				}
				if kind == "future-complete" {
					rows[200].Disposition = "COMPUTED_PRIVATE_PER_BATCH"
				}
				if kind == "source-classification" {
					rows[7].Disposition = "EXCLUDED"
				}
				// Rewrite only an owned negative fixture page, updating its claimed hashes.
				page := 1
				if kind == "source-classification" {
					page = 0
				}
				end := (page + 1) * 128
				raw, _ := json.Marshal(rows[page*128 : end])
				if kind == "unknown-row" {
					raw = []byte(strings.Replace(string(raw), `"Origin":1}`, `"Origin":1,"Unknown":null}`, 1))
				}
				if e = os.WriteFile(filepath.Join(root, pages[page].Selector), raw, 0600); e != nil {
					t.Fatal(e)
				}
				r.FilePages[page].SHA256 = pilot.Digest(raw)
				r.FilePages[page].ByteLength = len(raw)
			}
			e = s.verifyTerminalPages(root, r)
			want := kind == "control" || kind == "inline"
			assertInventoryPrivate(t, "ASSERT_REPAIR_TERMINAL_BOUNDARY_"+kind, (e == nil) == want)
		})
	}
}

func TestCensusApplicationRepairSourceSymbolOutcomePrivate(t *testing.T) {
	original, report, _, _ := applicationMockCompletedPrivate(t)
	s, r, reseal := applicationRepairClonePrivate(t, original, report)
	raw, _ := json.Marshal(r)
	var negative censusInventoryReportPrivate
	if e := json.Unmarshal(raw, &negative); e != nil {
		t.Fatal(e)
	}
	r = negative
	r.Origins[0].Status = "SUCCESSFUL_EMPTY" // actual discovery still contains targets
	for _, index := range r.Origins[0].Files {
		r.Files[index].Disposition = "SUCCESSFUL_EMPTY"
	}
	page := r.FilePages[0]
	raw, _ = json.Marshal(r.Files)
	page.SHA256 = pilot.Digest(raw)
	page.ByteLength = len(raw)
	r.FilePages[0] = page
	if e := os.WriteFile(filepath.Join(r.AttemptDirectory, page.Selector), raw, 0600); e != nil {
		t.Fatal(e)
	}
	reseal(r)
	_, e := s.verify(0)
	assertInventoryPrivate(t, "ASSERT_REPAIR_SOURCE_SYMBOL_OUTCOME", e != nil && strings.Contains(e.Error(), "original discovery"))
	t.Logf("source-symbol target observed=%v", e)
}
