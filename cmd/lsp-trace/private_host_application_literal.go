//go:build darwin || linux

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	pilot "lsp-trace/internal/censuslineagepilot"
	product "lsp-trace/internal/privatehostproductv3"
)

// This is a bounded optimization of an already-approved include domain, NOT a
// workspace census. Basenames match at ANY depth in native selection; mixed,
// escaped, glob and noncanonical rules must retain general traversal.
func applicationQualifiedLiteralIncludes(includes []string) ([]string, bool) {
	if len(includes) == 0 || !applicationLiteralObservedPaths {
		return nil, false
	}
	bytes := 0
	for _, p := range includes {
		bytes += len(p) + 1
		if bytes > 64<<10 || len(p) > 4096 || p != strings.TrimSpace(p) || strings.ContainsAny(p, "*?[]\\") || strings.HasPrefix(p, "/") || !strings.Contains(p, "/") || path.Clean(p) != p {
			return nil, false
		}
		if len(strings.Split(p, "/")) > 64 {
			return nil, false
		}
		for _, part := range strings.Split(p, "/") {
			if part == "" || part == "." || part == ".." || len(part) > 255 {
				return nil, false
			}
		}
	}
	return includes, true
}

type applicationLiteralDiscovery struct {
	RootIdentity                                                                  string
	LookupOpens, CanonicalNames                                                   uint64
	Parent                                                                        string
	DirectoryReservations, CandidatePages, CandidateNames, SourceStats, RootOpens uint64
	DirectoryOpens, DirectoryReads                                                uint64 // always zero: no source-directory scans
	MaxPageNames                                                                  int
}
type applicationLiteralDirectory struct{ Path, Next, Identity, Parent string }
type applicationLiteralAncestor struct{ Path, Identity, Parent string }

// Bounded by the <=64KiB configuration, never by source-directory cardinality.
func applicationLiteralNames(includes []string, directory string) []string {
	prefix := ""
	if directory != "." {
		prefix = filepath.ToSlash(directory) + "/"
	}
	seen := map[string]bool{}
	names := []string{}
	for _, p := range includes {
		if !strings.HasPrefix(p, prefix) {
			continue
		}
		rest := strings.TrimPrefix(p, prefix)
		name, _, _ := strings.Cut(rest, "/")
		if name != "" && !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}
	sort.Strings(names) // ONLY the bounded caller config; never collect/sort sources
	return names
}

func advanceApplicationLiteralDiscovery(ctx context.Context, r hostApplicationRequest, o censusCLIOptions, literals []string, a *product.ApplicationJournal, s *applicationState, allowHandoff bool) error {
	if s.DiscoveryFinished {
		// Completion is not a reusable pathname capability. A restart pays for
		// a fresh fixed-path observation before returning the stored denominator.
		if _, e := a.ReserveOperation(pilot.Digest([]byte(fmt.Sprintf("%s/literal-root-restart/%d", s.ApplicationSHA256, a.Head().Sequence))), "source_root_binding", product.ApplicationBounds{ReadHashBytes: 4096, ParserBytes: 4096, Records: 4, Work: 16}, time.Now()); e != nil {
			return e
		}
		if e := ctx.Err(); e != nil {
			return e
		}
		s.LiteralDiscovery.SourceStats++
		st, e := os.Lstat(r.Configuration.SourceRoot)
		if e != nil || !st.IsDir() || st.Mode()&os.ModeSymlink != 0 || applicationFileIdentity(st) != s.LiteralDiscovery.RootIdentity {
			s.DiscoveryFinished = false
			return product.ErrInvalid
		}
		f, e := os.OpenFile(r.Configuration.SourceRoot, applicationLiteralOpenFlags, 0)
		s.LiteralDiscovery.LookupOpens++
		if e != nil {
			s.DiscoveryFinished = false
			return e
		}
		defer f.Close()
		s.LiteralDiscovery.SourceStats++
		held, e := f.Stat()
		s.LiteralDiscovery.CanonicalNames++
		observed, pe := applicationLiteralFDPath(f)
		if e != nil || pe != nil || !os.SameFile(st, held) || observed != r.Configuration.SourceRoot {
			s.DiscoveryFinished = false
			return product.ErrInvalid
		}
		return ctx.Err()
	}
	if s.LiteralDiscovery.RootIdentity == "" {
		if s.Directory != "." || s.DirectoryStack != "" {
			return product.ErrInvalid
		}
		s.LiteralDiscovery = applicationLiteralDiscovery{RootIdentity: s.DirectoryIdentity}
	}
	var root *os.Root
	var rootFile *os.File
	defer func() {
		if rootFile != nil {
			rootFile.Close()
		}
		if root != nil {
			root.Close()
		}
	}()
	// Initial and handoff root/ancestry/current checks share the directory
	// reservation; ancestor depths form a bounded staircase, not repeated
	// maximum-depth walks. Pages cover at most 32 candidates. Work 9932/5216
	// covers no-follow opens/closes, fstat, canonical-name observations and
	// approved-root Lstat checks; name bytes stay <1MiB.
	// No source-directory enumeration is performed.
	stat := func(p string) (os.FileInfo, error) {
		f, opens, e := applicationLiteralOpenAt(rootFile, p)
		s.LiteralDiscovery.LookupOpens += opens
		if e != nil {
			return nil, e
		}
		defer f.Close()
		s.LiteralDiscovery.SourceStats++
		st, e := f.Stat()
		if e != nil {
			return nil, e
		}
		s.LiteralDiscovery.CanonicalNames++
		base, e := applicationLiteralFDPath(rootFile)
		if e != nil {
			return nil, e
		}
		// Relative agreement between two moving FDs is not approved-path
		// binding. Observe the approved absolute name as well as this held FD.
		if base != r.Configuration.SourceRoot || ctx.Err() != nil {
			return nil, product.ErrInvalid
		}
		s.LiteralDiscovery.SourceStats++
		named, ne := os.Lstat(r.Configuration.SourceRoot)
		if ne != nil || !named.IsDir() || named.Mode()&os.ModeSymlink != 0 || applicationFileIdentity(named) != s.LiteralDiscovery.RootIdentity || ctx.Err() != nil {
			return nil, product.ErrInvalid
		}
		s.LiteralDiscovery.CanonicalNames++
		observed, e := applicationLiteralFDPath(f)
		if e != nil {
			return nil, e
		}
		rel, e := filepath.Rel(base, observed)
		if e != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || filepath.IsAbs(rel) {
			return nil, product.ErrInvalid
		}
		// Native enumerates names, not aliases or unique inodes. A mismatched
		// observed spelling is not a member; hardlinked real names remain distinct.
		if rel != p {
			return nil, os.ErrNotExist
		}
		return st, nil
	}
	for !s.DiscoveryFinished {
		if e := ctx.Err(); e != nil {
			return e
		}
		// Preserve the directory cost, with bounded ancestry-validation headroom.
		if _, e := a.ReserveOperation(pilot.Digest([]byte(fmt.Sprintf("%s/literal-directory/%s/%d", s.ApplicationSHA256, s.Directory, a.Head().Sequence))), "directory_acquisition", product.ApplicationBounds{ReadHashBytes: 1 << 20, Records: 128, ParserBytes: 1 << 20, Work: 1024 + 2*68*65 + 68, ActiveNS: uint64(time.Second)}, time.Now()); e != nil {
			return e
		}
		s.LiteralDiscovery.DirectoryReservations++
		if root == nil {
			// Do not turn a symlink spelling of the approved source root into a
			// literal-path capability. This check is under the directory prepayment.
			s.LiteralDiscovery.SourceStats++
			info, e := os.Lstat(r.Configuration.SourceRoot)
			if e != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || applicationFileIdentity(info) != s.LiteralDiscovery.RootIdentity {
				return product.ErrInvalid
			}
			root, e = os.OpenRoot(r.Configuration.SourceRoot)
			if e != nil {
				return e
			}
			s.LiteralDiscovery.RootOpens++
			s.LiteralDiscovery.LookupOpens++
			rootFile, e = root.OpenFile(".", applicationLiteralOpenFlags, 0)
			if e != nil {
				return e
			}
			s.LiteralDiscovery.SourceStats++
			held, e := rootFile.Stat()
			if e != nil || applicationFileIdentity(held) != s.LiteralDiscovery.RootIdentity {
				return product.ErrInvalid
			}
		}
		check := func(p, identity string) error {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			info, e := stat(p)
			if e != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || applicationFileIdentity(info) != identity {
				return product.ErrInvalid
			}
			return nil
		}
		if e := check(".", s.LiteralDiscovery.RootIdentity); e != nil {
			return e
		}
		// Validate every stored ancestor before resolving the current directory. This
		// rejects stale restart paths and intermediate symlinks, not merely escapes.
		chain := []applicationLiteralAncestor{}
		for id := s.LiteralDiscovery.Parent; id != ""; {
			if len(chain) > r.Template.Policy.MaxPathDepth || len(chain) > 64 {
				return product.ErrInvalid
			}
			var ancestor applicationLiteralAncestor
			if e := a.GetRecord(id, &ancestor); e != nil {
				return e
			}
			if ancestor.Path != "." && (!strings.HasPrefix(s.Directory, ancestor.Path+string(os.PathSeparator)) || filepath.Clean(ancestor.Path) != ancestor.Path) {
				return product.ErrInvalid
			}
			chain = append(chain, ancestor)
			id = ancestor.Parent
		}
		validateDirectory := func() error {
			if e := check(".", s.LiteralDiscovery.RootIdentity); e != nil {
				return e
			}
			for n := len(chain) - 1; n >= 0; n-- {
				if e := check(chain[n].Path, chain[n].Identity); e != nil {
					return e
				}
			}
			return check(s.Directory, s.DirectoryIdentity)
		}
		// Candidate processing and pending resumes use the charged source checks.
		// Handoff repeats full root/ancestry/current validation before closing.
		for n := len(chain) - 1; n >= 0; n-- {
			if e := check(chain[n].Path, chain[n].Identity); e != nil {
				return e
			}
		}
		if e := check(s.Directory, s.DirectoryIdentity); e != nil {
			return e
		}
		handoff := func() error {
			if e := validateDirectory(); e != nil {
				return e
			}
			e := rootFile.Close()
			rootFile = nil
			re := root.Close()
			root = nil
			if e != nil {
				return e
			}
			if re != nil {
				return re
			}
			return ctx.Err()
		}
		if allowHandoff && s.Selected != "" {
			return handoff()
		}
		parentID, e := a.PutRecord(applicationLiteralAncestor{s.Directory, s.DirectoryIdentity, s.LiteralDiscovery.Parent})
		if e != nil {
			return e
		}
		names := applicationLiteralNames(literals, s.Directory)
		// A restart starts from the finite config again, with NEW reservations and
		// the native catalog index preventing duplicate observations. No OS offsets.
		for offset := 0; offset < len(names); offset += 32 {
			if e = ctx.Err(); e != nil {
				return e
			}
			paid, e := a.ReserveOperation(pilot.Digest([]byte(fmt.Sprintf("%s/literal-names/%d", s.ApplicationSHA256, a.Head().Sequence))), "directory_page", product.ApplicationBounds{ReadHashBytes: 1 << 20, Records: 128, ParserBytes: 1 << 20, Work: 1024 + 2*32*65 + 32, ActiveNS: uint64(3 * time.Second)}, time.Now())
			if e != nil {
				return e
			}
			end := offset + 32
			if end > len(names) {
				end = len(names)
			}
			page := names[offset:end]
			s.LiteralDiscovery.CandidatePages++
			s.LiteralDiscovery.CandidateNames += uint64(len(page))
			if len(page) > s.LiteralDiscovery.MaxPageNames {
				s.LiteralDiscovery.MaxPageNames = len(page)
			}
			working := *s
			keys := []string{}
			entries := []product.ApplicationCatalogEntry{}
			flush := func() error {
				if len(entries) == 0 {
					return nil
				}
				raw, human, e := product.ApplicationCatalogEncoding(entries, s.ApplicationSHA256)
				if e != nil {
					return e
				}
				working.LiteralDiscovery = s.LiteralDiscovery
				working.CatalogBytes += uint64(len(raw))
				working.ViewBytes += uint64(len(raw) + len(human))
				if _, e = a.CommitCatalogPage(keys, entries, working); e != nil {
					return e
				}
				*s = working
				keys = nil
				entries = nil
				return nil
			}
			for _, name := range page {
				if e = ctx.Err(); e != nil {
					return e
				}
				relative := filepath.Join(s.Directory, name)
				if len(relative) > 4096 || strings.Count(relative, string(os.PathSeparator)) > r.Template.Policy.MaxPathDepth {
					return product.ErrInvalid
				}
				st, e := stat(relative)
				if os.IsNotExist(e) {
					continue
				} // absent literal is not an invented catalog member
				if e != nil {
					return e
				}
				key := pilot.Digest([]byte(s.ApplicationSHA256 + "/entry/" + relative))
				seen, e := a.HasIndexInPage(key, paid)
				if e != nil {
					return e
				}
				if seen {
					continue
				}
				entry := product.ApplicationCatalogEntry{Schema: "private-application-catalog-entry.v1", ApplicationSHA256: s.ApplicationSHA256, Path: relative, Identity: applicationFileIdentity(st), Bytes: st.Size(), ModTimeNS: st.ModTime().UnixNano(), Disposition: "EXCLUDED"}
				if st.IsDir() && st.Mode()&os.ModeSymlink == 0 {
					prune := false
					for _, p := range o.PrunedDirectories {
						if applicationPrunedDirectory(p, relative) {
							prune = true
						}
					}
					if !prune {
						id, e := a.PutRecord(applicationLiteralDirectory{relative, working.DirectoryStack, entry.Identity, parentID})
						if e != nil {
							return e
						}
						working.DirectoryStack = id
						entry.Disposition = "DIRECTORY"
					}
				} else {
					working.Entries++
					selected := !matchesAnyDiscoveryPattern(o.Excludes, relative) && matchesAnyDiscoveryPattern(o.Includes, relative)
					if selected {
						working.SelectedCount++
					}
					if selected && st.Mode().IsRegular() && st.Size() <= int64(r.Template.Policy.FileBytes) {
						entry.Disposition = "SELECTED"
						entry.Next = working.Selected
						raw, _ := json.Marshal(entry)
						working.Selected = pilot.Digest(raw)
					} else if selected {
						entry.Disposition = "SOURCE_GAP_UNSUPPORTED_OR_OVERSIZED"
						working.SourceGaps++
					} else {
						working.Excluded++
					}
				}
				keys = append(keys, key)
				entries = append(entries, entry)
				if len(entries) == 8 {
					if e = flush(); e != nil {
						return e
					}
					if allowHandoff && s.Selected != "" {
						return handoff()
					}
				}
			}
			if e = flush(); e != nil {
				return e
			}
			if allowHandoff && s.Selected != "" {
				return handoff()
			}
		}
		if e = check(s.Directory, s.DirectoryIdentity); e != nil {
			return e
		}
		if s.DirectoryStack == "" {
			s.DiscoveryFinished = true
			s.Directory = ""
		} else {
			var next applicationLiteralDirectory
			if e = a.GetRecord(s.DirectoryStack, &next); e != nil {
				return e
			}
			s.Directory = next.Path
			s.DirectoryIdentity = next.Identity
			s.DirectoryStack = next.Next
			s.LiteralDiscovery.Parent = next.Parent
		}
		if e = saveApplicationState(a, *s); e != nil {
			return e
		}
	}
	return nil
}
