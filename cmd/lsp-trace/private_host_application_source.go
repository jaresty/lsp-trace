//go:build darwin || linux

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"lsp-trace/internal/censusacquisition"
	pilot "lsp-trace/internal/censuslineagepilot"
	"lsp-trace/internal/manageddiagnostic"
	product "lsp-trace/internal/privatehostproductv3"
	"lsp-trace/sessionruntime"
)

// Match native source.prunedDirectory: workspace-relative prefix, never basename.
func applicationPrunedDirectory(prune, relative string) bool {
	relative = filepath.ToSlash(relative)
	return relative == prune || strings.HasPrefix(relative, prune+"/")
}

func applicationFileIdentity(st os.FileInfo) string {
	v, ok := st.Sys().(*syscall.Stat_t)
	if !ok {
		return ""
	}
	return pilot.Digest([]byte(fmt.Sprintf("%d/%d/%d/%d/%d", v.Dev, v.Ino, st.Size(), st.ModTime().UnixNano(), st.Mode())))
}

func advanceApplicationDiscovery(ctx context.Context, r hostApplicationRequest, a *product.ApplicationJournal, s *applicationState) error {
	return advanceApplicationDiscoveryWithHandoff(ctx, r, a, s, true)
}

// The distinct discover operation drains the catalog; run yields pending work.
// Neither operation rewrites the approved selection or configuration.
func advanceApplicationDiscoveryWithHandoff(ctx context.Context, r hostApplicationRequest, a *product.ApplicationJournal, s *applicationState, allowHandoff bool) error {
	o, e := parseCensusCLIOptions(r.Template.CensusArguments)
	if e != nil {
		return e
	}
	if literals, ok := applicationQualifiedLiteralIncludes(o.Includes); ok {
		return advanceApplicationLiteralDiscovery(ctx, r, o, literals, a, s, allowHandoff)
	}
	for !s.DiscoveryFinished {
		if e = ctx.Err(); e != nil {
			return e
		}
		path := filepath.Join(r.Configuration.SourceRoot, s.Directory)
		info, e := os.Lstat(path)
		if e != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || applicationFileIdentity(info) != s.DirectoryIdentity {
			return product.ErrInvalid
		}
		// A restart reopens this directory from the beginning under NEW charged
		// claims. No OS directory offset is treated as a portable checkpoint.
		if _, e = a.ReserveOperation(pilot.Digest([]byte(fmt.Sprintf("%s/directory/%s/%d", s.ApplicationSHA256, s.Directory, a.Head().Sequence))), "directory_acquisition", product.ApplicationBounds{ReadHashBytes: 65536, Records: 64, ParserBytes: 65536, Work: 64, ActiveNS: uint64(time.Second)}, time.Now()); e != nil {
			return e
		}
		dir, e := os.Open(path)
		if e != nil {
			return e
		}
		// Pending work is returned only after a fresh charged directory observation.
		if allowHandoff {
			if e = applicationDiscoveryDirectoryCheck(ctx, path, s.DirectoryIdentity); e != nil {
				dir.Close()
				return e
			}
			if s.Selected != "" {
				return applicationDiscoveryDirectoryHandoff(ctx, dir, path, s.DirectoryIdentity)
			}
		}
		for {
			if e = ctx.Err(); e != nil {
				dir.Close()
				return e
			}
			paid, pe := a.ReserveOperation(pilot.Digest([]byte(fmt.Sprintf("%s/names/%d", s.ApplicationSHA256, a.Head().Sequence))), "directory_page", product.ApplicationBounds{ReadHashBytes: 1 << 20, Records: 128, ParserBytes: 1 << 20, Work: 1024, ActiveNS: uint64(3 * time.Second)}, time.Now())
			if pe != nil {
				dir.Close()
				return pe
			}
			names, re := dir.Readdirnames(32)
			keys := []string{}
			entries := []product.ApplicationCatalogEntry{}
			working := *s
			flush := func() error {
				if len(entries) == 0 {
					return nil
				}
				raw, human, e := product.ApplicationCatalogEncoding(entries, s.ApplicationSHA256)
				if e != nil {
					return e
				}
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
			for _, name := range names {
				if len(name) > 255 || name == "." || name == ".." {
					dir.Close()
					return product.ErrInvalid
				}
				relative := filepath.Join(s.Directory, name)
				if len(relative) > 4096 || strings.Count(relative, string(os.PathSeparator)) > r.Template.Policy.MaxPathDepth {
					dir.Close()
					return product.ErrInvalid
				}
				key := pilot.Digest([]byte(s.ApplicationSHA256 + "/entry/" + relative))
				seen, e := a.HasIndexInPage(key, paid)
				if e != nil {
					dir.Close()
					return e
				}
				if seen {
					continue
				}
				st, e := os.Lstat(filepath.Join(r.Configuration.SourceRoot, relative))
				if e != nil {
					dir.Close()
					return e
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
						node := applicationDirectory{relative, working.DirectoryStack, entry.Identity}
						id, e := a.PutRecord(node)
						if e != nil {
							dir.Close()
							return e
						}
						working.DirectoryStack = id
						entry.Disposition = "DIRECTORY"
					}
				} else {
					working.Entries++
					selected := !matchesAnyDiscoveryPattern(o.Excludes, relative) && (len(o.Includes) == 0 || matchesAnyDiscoveryPattern(o.Includes, relative))
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
						dir.Close()
						return e
					}
					if allowHandoff && s.Selected != "" {
						if re != nil && re != io.EOF {
							dir.Close()
							return re
						}
						return applicationDiscoveryDirectoryHandoff(ctx, dir, path, s.DirectoryIdentity)
					}
				}
			}
			if e = flush(); e != nil {
				dir.Close()
				return e
			}
			if allowHandoff && s.Selected != "" {
				if re != nil && re != io.EOF {
					dir.Close()
					return re
				}
				return applicationDiscoveryDirectoryHandoff(ctx, dir, path, s.DirectoryIdentity)
			}
			if re == io.EOF {
				break
			}
			if re != nil {
				dir.Close()
				return re
			}
		}
		dir.Close()
		last, e := os.Lstat(path)
		if e != nil || applicationFileIdentity(last) != s.DirectoryIdentity {
			return product.ErrInvalid
		}
		if s.DirectoryStack == "" {
			s.DiscoveryFinished = true
			s.Directory = ""
		} else {
			var next applicationDirectory
			if e = a.GetRecord(s.DirectoryStack, &next); e != nil {
				return e
			}
			s.Directory = next.Path
			s.DirectoryIdentity = next.Identity
			s.DirectoryStack = next.Next
		}
		if e = saveApplicationState(a, *s); e != nil {
			return e
		}
	}
	return nil
}

// The catalog commit remains an unfinished cursor: no pop and no saved OS
// offset. Any unprocessed names are reobserved under new reservations on resume.
func applicationDiscoveryDirectoryCheck(ctx context.Context, path, identity string) error {
	if e := ctx.Err(); e != nil {
		return e
	}
	st, e := os.Lstat(path)
	if e != nil || !st.IsDir() || st.Mode()&os.ModeSymlink != 0 || applicationFileIdentity(st) != identity {
		return product.ErrInvalid
	}
	return ctx.Err()
}

func applicationDiscoveryDirectoryHandoff(ctx context.Context, dir *os.File, path, identity string) error {
	e := applicationDiscoveryDirectoryCheck(ctx, path, identity)
	closeErr := dir.Close()
	if e != nil {
		return e
	}
	if closeErr != nil {
		return closeErr
	}
	return ctx.Err()
}

func deriveApplicationSource(ctx context.Context, r hostApplicationRequest, entry applicationCatalogEntry, unitRoot string) (hostProductRequestV3, pilot.FrozenScopePrivateV2, error) {
	source := r.Template
	var zero pilot.FrozenScopePrivateV2
	if entry.ApplicationSHA256 != product.ApplicationConfigDigest(r.Configuration) || filepath.IsAbs(entry.Path) || filepath.Clean(entry.Path) != entry.Path || entry.Path == ".." || strings.HasPrefix(entry.Path, ".."+string(os.PathSeparator)) || len(entry.Path) > 4096 {
		return source, zero, product.ErrInvalid
	}
	st, e := os.Lstat(filepath.Join(r.Configuration.SourceRoot, entry.Path))
	if e != nil || applicationFileIdentity(st) != entry.Identity {
		return source, zero, product.ErrInvalid
	}
	args := []string{}
	for n := 0; n < len(source.CensusArguments); n++ {
		arg := source.CensusArguments[n]
		if arg == "--source" || arg == "--publication-root" {
			n++
			continue
		}
		args = append(args, arg)
	}
	pub := filepath.Join(unitRoot, "publication")
	if e = os.Mkdir(pub, 0700); e != nil && !os.IsExist(e) {
		return source, zero, e
	}
	source.CensusArguments = append(args, "--source", entry.Path, "--publication-root", pub)
	source.AuxiliaryRoot = filepath.Join(unitRoot, "auxiliary")
	source.Configuration.Store = filepath.Join(unitRoot, "original")
	source.Configuration.Application = pilot.Digest([]byte(r.ApprovalSHA256 + "/" + entry.Path))
	unitPolicy, policyErr := applicationUnitEnvelope(r)
	if policyErr != nil {
		return source, zero, policyErr
	}
	source.Configuration.MaxCalls = unitPolicy.MaxCalls
	source.Configuration.MaxRequestBytes = unitPolicy.MaxRequestBytes
	source.Configuration.MaxResponseBytes = r.UnitResponseBytes
	source.Configuration.MaxTotalBytes = unitPolicy.MaxTotalBytes
	source.Configuration.AllowanceRevision = 1
	o, e := parseCensusCLIOptions(source.CensusArguments)
	if e != nil {
		return source, zero, e
	}
	// Existing native enumeration/selection, but ONLY this bounded source unit.
	files, e := enumerateCensusPreflightContextPrivate(ctx, o)
	if e != nil || len(files) != 1 {
		return source, zero, product.ErrInvalid
	}
	f, e := pilot.FreezeCensusScopePrivateV2(ctx, o.Workspace, r.Configuration.RevisionAssertion, files, source.Policy)
	if e != nil {
		return source, zero, e
	}
	source.FileDenominator = len(files)
	source.Configuration.ScopeSHA256 = pilot.Digest(f.ManifestBytes())
	source.Configuration.ProducerConfigSHA256 = hostProductProducerDigestV3(source)
	source.Configuration.RolePromptSHA256 = [2]string{product.PromptDigest("interpreter"), product.PromptDigest("reviewer")}
	return source, f, nil
}
func applicationDerivedCohort(r hostApplicationRequest, source product.Config, unitID, unitRoot string) product.CohortConfig {
	p, e := applicationUnitEnvelope(r)
	if e != nil {
		return product.CohortConfig{}
	}
	return product.CohortConfig{Navigation: p.Navigation, RolePromptSHA256: r.Configuration.RolePromptSHA256, Requested: source.Requested, Profile: product.CohortProfile, Protocol: product.CohortProtocol, SourceConfigSHA256: product.ConfigDigest(source), ScopeSHA256: source.ScopeSHA256, Store: filepath.Join(unitRoot, "cohort"), Grant: pilot.Digest([]byte(r.ApprovalSHA256 + unitID)), MaxCalls: p.MaxCalls, MaxRequestBytes: p.MaxRequestBytes, MaxResponseBytes: r.UnitResponseBytes, MaxTotalBytes: p.MaxTotalBytes, MaxOutputBytes: p.MaxOutputBytes - (32 << 10)}
}
func acquireApplicationChild(requestPath string, r hostApplicationRequest, contractID, unitRoot string, out, errout io.Writer) int {
	var contract struct {
		Unit   product.UnitBinding
		Entry  applicationCatalogEntry
		Source hostProductRequestV3
	}
	if product.ReadApplicationRecord(r.Configuration.Store, contractID, &contract) != nil || contract.Unit.ApplicationSHA256 != product.ApplicationConfigDigest(r.Configuration) || !validApplicationDigest(contract.Unit.UnitID) || unitRoot != filepath.Join(r.Configuration.Store, "units", contract.Unit.UnitID[7:]) {
		return 1
	}
	allowance, allowanceErr := applicationChildAllowance(r, contract.Source)
	if allowanceErr != nil || product.VerifyApplicationUnitPrepayment(r.Configuration.Store, contract.Unit, allowance, contract.Source.Configuration) != nil {
		return 1
	}
	deadline, de := product.ReadApplicationUnitDeadline(r.Configuration.Store, contract.Unit.ApplicationSHA256, contract.Unit.UnitID)
	expires := time.Unix(0, r.Configuration.Grant.ExpiresUnixNano)
	if expires.Before(deadline) {
		deadline = expires
	}
	if de != nil || !time.Now().Before(deadline) {
		return 1
	}
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()
	canonical, ce := filepath.EvalSymlinks(unitRoot)
	st, se := os.Stat(unitRoot)
	if ce != nil || se != nil || canonical != unitRoot || !st.IsDir() || st.Mode().Perm() != 0700 {
		return 1
	}
	if ctx.Err() != nil || !time.Now().Before(deadline) {
		return 1
	}
	claim, _ := json.Marshal(struct{ ApplicationSHA256, UnitID, ContractSHA256 string }{contract.Unit.ApplicationSHA256, contract.Unit.UnitID, contractID})
	if applicationWriteOnce(filepath.Join(unitRoot, "application-original-claim.json"), claim) != nil {
		return 1
	}
	if ctx.Err() != nil {
		return 1
	}
	source, frozen, e := deriveApplicationSource(ctx, r, contract.Entry, unitRoot)
	expectedRaw, _ := json.Marshal(contract.Source)
	actualRaw, _ := json.Marshal(source)
	actualAllowance, allowanceErr := applicationChildAllowance(r, source)
	if e != nil || allowanceErr != nil || actualAllowance != allowance || !bytes.Equal(expectedRaw, actualRaw) || pilot.Digest(frozen.ManifestBytes()) != contract.Unit.SourceFreezeSHA256 || pilot.Digest(frozen.SpecBytes()) != contract.Unit.InputSHA256 {
		return 1
	}
	// Derivation is explicitly authorized by the application request, not by an
	// old RuntimeGrantSHA256 copied across scopes. Native discovery is not replayed.
	if hostProductServerPinV3(source) != nil {
		return 1
	}
	if e = os.Mkdir(source.AuxiliaryRoot, 0700); e != nil {
		return 1
	}
	if ctx.Err() != nil {
		return 1
	}
	deps := productionCensusRunnerDependencies()
	// Only this owned application route needs the existing startup sink. A sink
	// without the manager's bounded store would persist only UNAVAILABLE facts.
	newManager := deps.newManager
	deps.newManager = func(cfg sessionruntime.Config) (*sessionruntime.Manager, error) {
		if cfg.Diagnostics == nil {
			cfg.Diagnostics = manageddiagnostic.NewStore(manageddiagnostic.Bounds{MaxRecords: manageddiagnostic.StartupDiagnosticsMaxRecords, MaxBytes: manageddiagnostic.StartupDiagnosticsMaxBytes})
		}
		return newManager(cfg)
	}
	deps.runPrivateInventory = func(o censusCLIOptions, c censusCoreConfig, stdout, stderr io.Writer) int {
		return acquireHostProductWithHookV3(ctx, source, frozen, o, c, stdout, stderr, 2*time.Second, func(bound *hostProductRequestV3, origin pilot.EnumerationOriginPrivateV2) error {
			var frame struct {
				Batches []censusacquisition.BatchRequest `json:"target_batches"`
			}
			if json.Unmarshal(origin.Bytes(), &frame) != nil {
				return product.ErrInvalid
			}
			count := 0
			for _, batch := range frame.Batches {
				count += len(batch.Targets)
			}
			if count == 0 {
				return product.ErrInvalid
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			// Native count only establishes a lower bound: ten items/frame,
			// two mandatory roles. Reject known insufficiency before capture or
			// the original child ledger; encoded packing may need more later.
			if 2*((count+9)/10) > bound.Configuration.MaxCalls {
				return errApplicationUnitSemanticPolicy
			}
			bound.Configuration.Requested = count
			bound.Configuration.ApprovalSHA256 = product.ConfigDigest(bound.Configuration)
			if product.Initialize(bound.Configuration) != nil || product.ClaimOriginal(bound.Configuration) != nil {
				return product.ErrInvalid
			}
			raw, _ := json.Marshal(bound)
			return applicationWriteOnce(filepath.Join(unitRoot, "source.json"), raw)
		})
	}
	return runCensusWithDependencies(append(append([]string{}, source.CensusArguments...), "--private-inventory-config", requestPath), out, errout, deps)
}
