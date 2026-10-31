//go:build darwin || linux

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"lsp-trace/internal/censusacquisition"
	pilot "lsp-trace/internal/censuslineagepilot"
	prep "lsp-trace/internal/privatedescribeprep"
)

const censusApplicationHostTrustPrivate = "SAME_USER_HOST_ATTESTED_NOT_AUTHENTICATED"

type censusApplicationClaimPrivate struct {
	Schema, FullPlanSHA256                             string
	Segment, ProcessID                                 int
	AttemptDirectory, ScopeSHA256, ConfigurationSHA256 string
}
type censusApplicationResultPrivate struct {
	Schema, ClaimSHA256, ReportSHA256, Status, Failure string
}

// A caller DTO, NEVER a native qualification or acquisition capability. The
// external host writes it only AFTER observing its exact child's Wait result.
type censusApplicationHostObservationPrivate struct {
	Schema, Trust, FullPlanSHA256                                                        string
	Segment, ProcessID, ExitCode                                                         int
	Joined, Stopped                                                                      bool
	AttemptDirectory, ClaimSHA256, ResultSHA256, ReportSHA256, ScopeSHA256, NativeSHA256 string
	NativeBytes                                                                          uint64
}
type censusApplicationCandidateRefPrivate struct {
	Namespace, OriginID, ID, Label string
	NativeIndex                    int
}
type censusApplicationOriginAccountingPrivate struct {
	Reason                                                          string `json:",omitempty"`
	Disposition                                                     string `json:",omitempty"`
	PlanOrigin                                                      int
	DiscoverySHA256                                                 string
	FileEntries, SymbolEntries, CallableTargets, NonCallableSymbols int
	DiscoveryComplete                                               bool
}
type censusApplicationVerifiedSegmentPrivate struct {
	Disposition                                                                      string `json:",omitempty"`
	Origins                                                                          []censusApplicationOriginAccountingPrivate
	Segment                                                                          int
	AttemptDirectory, ScopeSHA256, ReportSHA256, HostObservationSHA256, NativeSHA256 string
	NativeBytes                                                                      uint64
	OriginIDs                                                                        []string
	Coverage                                                                         []prep.Coverage
	Candidates                                                                       []censusApplicationCandidateRefPrivate
}
type censusApplicationBlockedPrivate struct {
	Segment                                                                                         int
	AttemptDirectory, ClaimSHA256, ResultSHA256, ClaimedReportSHA256, HostObservationSHA256, Status string
	BindingStatus                                                                                   string // explicitly UNVERIFIED; no completion credit
}
type censusApplicationAccountingPrivate struct {
	Blocked                                                                                   *censusApplicationBlockedPrivate `json:",omitempty"`
	SourceCompleteness, AbsenceInference                                                      string
	Schema, InputFamily, FullPlanSHA256, Trust, FeatureIdentity, CrossSegmentNavigation       string
	Authority                                                                                 int
	Accepted                                                                                  bool
	FileDenominator, EligibleFiles, CompletedFiles, ExcludedFiles, GapFiles, UnattemptedFiles int
	ObservedSymbolEntries, CallableTargets, NonCallableSymbols                                int
	SourceGapTargets, ResourceGapTargets                                                      int
	TerminalAccountedFiles, TerminalAccountedTargets                                          int `json:",omitempty"`
	NativePreparedTargets                                                                     int `json:",omitempty"`
	Files                                                                                     []censusInventoryFilePrivate
	FilePages                                                                                 []censusInventoryPagePrivate
	Verified                                                                                  []censusApplicationVerifiedSegmentPrivate
	Failure                                                                                   string
}
type censusApplicationProgressPrivate struct {
	store      *censusApplicationStorePrivate
	options    censusCLIOptions
	config     censusInventoryConfigPrivate
	rows       []censusInventoryFilePrivate
	pages      []censusInventoryPagePrivate
	plan       censusApplicationFullPlanPrivate
	raw        []byte
	id         string
	next       int
	verified   []censusApplicationVerifiedSegmentPrivate
	blocked    *censusApplicationBlockedPrivate
	claimed    bool
	claimSHA   string
	runtimeSHA string
	failure    string
}

func openCensusApplicationProgressPrivate(ctx context.Context, o censusCLIOptions, c censusInventoryConfigPrivate, rows []censusInventoryFilePrivate, pages []censusInventoryPagePrivate, attempt, runtimeSHA string) (s *censusApplicationProgressPrivate, err error) {
	a := c.Application
	if a == nil || a.validate() != nil || censusInventoryRelatedPathsPrivate(a.Directory, o.Workspace) || censusInventoryRelatedPathsPrivate(a.Directory, o.PublicationRoot) || (c.Automation != nil && censusInventoryRelatedPathsPrivate(a.Directory, c.Automation.Directory)) {
		return nil, errors.New("application roots overlap or unavailable")
	}
	store, err := openCensusApplicationStorePrivate(ctx, a.Directory, a.MaxReadBytes)
	if err != nil {
		return nil, err
	}
	s = &censusApplicationProgressPrivate{store: store, options: o, config: c, rows: append([]censusInventoryFilePrivate(nil), rows...), pages: pages, runtimeSHA: runtimeSHA}
	s.plan, s.raw, err = buildCensusApplicationFullPlanPrivate(ctx, o, c, rows, pages, runtimeSHA)
	if err != nil {
		return s, err
	}
	s.id = pilot.Digest(s.raw)
	old, e := store.read("full-source-plan.json", a.MaxPlanBytes)
	if errors.Is(e, os.ErrNotExist) {
		// Install exact denominator pages under this separate immutable source plan.
		for _, page := range pages {
			raw, e := s.store.external(filepath.Join(attempt, page.Selector), a.MaxPlanBytes)
			if e != nil {
				return s, e
			}
			if pilot.Digest(raw) != page.SHA256 || len(raw) != page.ByteLength {
				return s, errors.New("application denominator page mismatch")
			}
			prior, e := store.read(page.Selector, a.MaxPlanBytes)
			if errors.Is(e, os.ErrNotExist) {
				e = store.write(page.Selector, raw, a.MaxPlanBytes)
			} else if e == nil && !bytes.Equal(prior, raw) {
				e = errors.New("application stale denominator page")
			}
			if e != nil {
				return s, e
			}
		}
		if err = store.write("full-source-plan.json", s.raw, a.MaxPlanBytes); err != nil {
			return s, err
		}
	} else if e != nil {
		return s, e
	} else if !bytes.Equal(old, s.raw) {
		return s, errors.New("application full source/config/git/partition drift")
	}
	// Readback is required even for a first install. DTO bytes remain observations.
	old, err = store.read("full-source-plan.json", a.MaxPlanBytes)
	if err != nil || !bytes.Equal(old, s.raw) {
		return s, errors.Join(err, errors.New("application full plan readback"))
	}
	for _, page := range pages {
		b, e := store.read(page.Selector, a.MaxPlanBytes)
		if e != nil || len(b) != page.ByteLength || pilot.Digest(b) != page.SHA256 {
			return s, errors.Join(e, errors.New("application missing denominator constituent"))
		}
	}
	err = s.scan()
	return s, err
}
func (s *censusApplicationProgressPrivate) accounting(attempt string) error {
	view := censusApplicationAccountingPrivate{SourceCompleteness: "UNKNOWN", AbsenceInference: "PROHIBITED", Schema: "lsp-trace.census-application-accounting.private.v1", InputFamily: "APPLICATION_ACCOUNTING_NOT_NATIVE", FullPlanSHA256: s.id, Trust: censusApplicationHostTrustPrivate, FeatureIdentity: "UNRESOLVED", CrossSegmentNavigation: "UNREVIEWED_NO_GLOBAL_GRAPH_OR_INDEPENDENCE_CLAIM", FileDenominator: len(s.rows), Files: append([]censusInventoryFilePrivate(nil), s.rows...), Verified: s.verified, Failure: s.failure, Blocked: s.blocked}
	completed := map[int]bool{}
	terminal := map[int]bool{}
	for _, v := range s.verified {
		for _, origin := range v.Origins {
			view.ObservedSymbolEntries += origin.SymbolEntries
			view.CallableTargets += origin.CallableTargets
			view.NonCallableSymbols += origin.NonCallableSymbols
		}
		for _, g := range s.plan.groups(v.Segment) {
			for _, index := range g {
				if v.Disposition == "TERMINAL_ACCOUNTED_GAP" {
					terminal[index] = true
				} else {
					completed[index] = true
				}
			}
		}
		if v.Disposition == "TERMINAL_ACCOUNTED_GAP" {
			for _, origin := range v.Origins {
				view.TerminalAccountedTargets += origin.CallableTargets
			}
		}
		for _, c := range v.Coverage {
			if censusTerminalEnabledPrivate(s.config) && v.Disposition == "COMPLETED_NATIVE" {
				view.NativePreparedTargets += c.Prepared
			}
			view.SourceGapTargets += c.SourceGap
			view.ResourceGapTargets += c.ResourceGap
		}
	}
	spent := map[int]bool{}
	if s.failure != "" && s.blocked != nil && s.blocked.ClaimSHA256 != "" && s.blocked.Segment >= 0 && s.blocked.Segment < len(s.plan.Segments) {
		for _, g := range s.plan.groups(s.blocked.Segment) {
			for _, index := range g {
				spent[index] = true
			}
		}
	}
	for i := range view.Files {
		row := &view.Files[i]
		if row.Disposition == "UNPROCESSED" || row.Disposition == "RESOURCE_GAP_ORIGIN_LIMIT" {
			view.EligibleFiles++
			switch {
			case completed[i]:
				row.Disposition = "COMPLETED_HOST_ATTESTED"
				if censusTerminalEnabledPrivate(s.config) {
					row.Disposition = "COMPLETED_NATIVE"
				}
				view.CompletedFiles++
			case terminal[i]:
				row.Disposition = "TERMINAL_ACCOUNTED_GAP"
				view.GapFiles++
				view.TerminalAccountedFiles++
			case spent[i]:
				row.Disposition = "GAP_SPENT_OR_UNVERIFIED_SEGMENT"
				view.GapFiles++
			default:
				row.Disposition = "UNATTEMPTED"
				view.UnattemptedFiles++
			}
		} else if row.Disposition == "EXCLUDED" || row.Disposition == "UNSUPPORTED_LANGUAGE" {
			view.ExcludedFiles++
		} else {
			view.GapFiles++
		}
	}
	pages, err := retainCensusInventoryPagesPrivate(attempt, "application-accounting-files", view.Files)
	if err != nil {
		return err
	}
	view.FilePages = pages
	if len(view.Files) > 128 {
		view.Files = nil
	}
	raw, err := json.Marshal(view)
	if err != nil || len(raw) > s.config.Application.MaxArtifactBytes {
		return errors.New("application accounting artifact byte bound")
	}
	human := fmt.Sprintf("# Private application accounting\n\nPlan: %s\nTrust: %s\nAuthority: 0; accepted: false; feature identity: UNRESOLVED\nCross-segment navigation: UNREVIEWED; no global graph or independence claim.\n\nFiles: %d = completed %d + excluded %d + gap %d + unattempted %d\nSource-gap targets: %d; resource-gap targets: %d\nFailure: %s\n\n", s.id, censusApplicationHostTrustPrivate, view.FileDenominator, view.CompletedFiles, view.ExcludedFiles, view.GapFiles, view.UnattemptedFiles, view.SourceGapTargets, view.ResourceGapTargets, view.Failure)
	var text strings.Builder
	text.WriteString(human)
	if censusTerminalEnabledPrivate(s.config) {
		fmt.Fprintf(&text, "Terminal-accounted gap files: %d; targets: %d. NOT_NATIVE_DELIVERY; no absence or feature-completion inference.\n", view.TerminalAccountedFiles, view.TerminalAccountedTargets)
	}
	fmt.Fprintf(&text, "Observed symbol entries across declared file partitions: %d; callable targets: %d; non-callable symbols: %d. These counts assert no global symbol or feature identity.\n", view.ObservedSymbolEntries, view.CallableTargets, view.NonCallableSymbols)
	if s.blocked != nil {
		fmt.Fprintf(&text, "Blocked segment %d: %s; status %s; claim %s; result %s; claimed report %s; %s (no completion credit)\n", s.blocked.Segment, s.blocked.AttemptDirectory, s.blocked.Status, s.blocked.ClaimSHA256, s.blocked.ResultSHA256, s.blocked.ClaimedReportSHA256, s.blocked.BindingStatus)
	}
	for _, v := range s.verified {
		fmt.Fprintf(&text, "Segment %d: %s (%d bytes); report %s; host observation %s\n", v.Segment, v.NativeSHA256, v.NativeBytes, v.ReportSHA256, v.HostObservationSHA256)
		for _, origin := range v.Origins {
			fmt.Fprintf(&text, "  Original plan origin %d: files %d, symbol entries %d, callable targets %d, non-callables %d; discovery %s; enumeration complete %t\n", origin.PlanOrigin, origin.FileEntries, origin.SymbolEntries, origin.CallableTargets, origin.NonCallableSymbols, origin.DiscoverySHA256, origin.DiscoveryComplete)
		}
		for _, c := range v.Candidates {
			fmt.Fprintf(&text, "- %s: %s [provisional; native candidate %s]\n", c.Namespace, c.Label, c.ID)
		}
	}
	if text.Len() > s.config.Application.MaxArtifactBytes {
		return errors.New("application human accounting byte bound")
	}
	out, err := openCensusApplicationStorePrivate(s.store.ctx, attempt, s.config.Application.MaxReadBytes)
	if err != nil {
		return err
	}
	defer out.close()
	if err = out.write("application-accounting.json", raw, s.config.Application.MaxArtifactBytes); err != nil {
		return err
	}
	return out.write("application-accounting.md", []byte(text.String()), s.config.Application.MaxArtifactBytes)
}
func (s *censusApplicationProgressPrivate) binding() *censusApplicationBindingPrivate {
	b, _ := json.Marshal(s.config)
	return &censusApplicationBindingPrivate{FullPlanSHA256: s.id, Segment: s.next, SegmentCount: len(s.plan.Segments), ProcessID: os.Getpid(), ClaimSHA256: s.claimSHA, ConfigurationSHA256: pilot.Digest(b)}
}
func (s *censusApplicationProgressPrivate) scan() error {
	allowed := map[string]bool{"full-source-plan.json": true}
	for _, p := range s.pages {
		allowed[p.Selector] = true
	}
	for i := range s.plan.Segments {
		for _, kind := range []string{"claim", "result", "host"} {
			allowed[censusApplicationLeafPrivate(i, kind)] = true
		}
	}
	names, err := s.store.names(len(allowed))
	if err != nil {
		return err
	}
	present := map[string]bool{}
	for _, name := range names {
		if !allowed[name] {
			return errors.New("application foreign/duplicate receipt selector")
		}
		present[name] = true
	}
	gap := false
	for i := range s.plan.Segments {
		claim, result, host := present[censusApplicationLeafPrivate(i, "claim")], present[censusApplicationLeafPrivate(i, "result")], present[censusApplicationLeafPrivate(i, "host")]
		if host {
			if gap || !claim || !result {
				return errors.New("application out-of-order or missing receipt constituent")
			}
			v, e := s.verify(i)
			if e != nil {
				s.failure = "UNVERIFIED_OR_FAILED_HOST_OBSERVATION"
				s.blocked = s.blockedObservation(i)
				return e
			}
			s.verified = append(s.verified, v)
			s.next++
			continue
		}
		if result && !claim {
			return errors.New("application orphan result")
		}
		if claim {
			s.failure = "CLAIMED_GENERATION_SPENT_OR_AWAITING_HOST_OBSERVATION"
			s.blocked = s.blockedObservation(i)
			return errors.New(s.failure)
		}
		gap = true
	}
	return nil
}
func (s *censusApplicationProgressPrivate) blockedObservation(segment int) *censusApplicationBlockedPrivate {
	v := &censusApplicationBlockedPrivate{Segment: segment, BindingStatus: "UNVERIFIED_NO_COMPLETION_CREDIT"}
	var claim censusApplicationClaimPrivate
	raw, e := s.store.read(censusApplicationLeafPrivate(segment, "claim"), 64<<10)
	if e != nil || decodeCensusApplicationPrivate(raw, &claim) != nil || claim.Schema != "lsp-trace.census-application-claim.private.v1" || claim.FullPlanSHA256 != s.id || claim.Segment != segment || segment < 0 || segment >= len(s.plan.Segments) || claim.ProcessID < 1 || filepath.Clean(claim.AttemptDirectory) != claim.AttemptDirectory || filepath.Dir(claim.AttemptDirectory) != s.options.PublicationRoot || !strings.HasPrefix(filepath.Base(claim.AttemptDirectory), "private-census-inventory-") {
		return v
	}
	groups := s.plan.groups(segment)
	manifests := make([]json.RawMessage, len(groups))
	for j, origin := range s.plan.Segments[segment] {
		manifests[j] = s.plan.Origins[origin].Manifest
	}
	expectedScope := censusInventoryScopeBytesPrivate(s.options, s.plan.Configuration, s.pages, manifests, groups, &censusApplicationScopePrivate{s.id, segment})
	if claim.ScopeSHA256 != pilot.Digest(expectedScope) {
		return v
	}
	v.AttemptDirectory, v.ClaimSHA256 = claim.AttemptDirectory, pilot.Digest(raw)
	var result censusApplicationResultPrivate
	if b, e := s.store.read(censusApplicationLeafPrivate(segment, "result"), 64<<10); e == nil && decodeCensusApplicationPrivate(b, &result) == nil && result.ClaimSHA256 == v.ClaimSHA256 {
		v.ResultSHA256, v.ClaimedReportSHA256, v.Status = pilot.Digest(b), result.ReportSHA256, result.Status
	}
	if b, e := s.store.read(censusApplicationLeafPrivate(segment, "host"), 64<<10); e == nil {
		v.HostObservationSHA256 = pilot.Digest(b)
	}
	return v
}
func (s *censusApplicationProgressPrivate) claim(attempt, scope string) error {
	if s.next >= len(s.plan.Segments) {
		return errors.New("application already complete")
	}
	b := s.binding()
	claim := censusApplicationClaimPrivate{"lsp-trace.census-application-claim.private.v1", s.id, s.next, os.Getpid(), attempt, scope, b.ConfigurationSHA256}
	raw, _ := json.Marshal(claim)
	if err := s.store.write(censusApplicationLeafPrivate(s.next, "claim"), raw, 64<<10); err != nil {
		return err
	}
	s.claimed = true
	s.claimSHA = pilot.Digest(raw)
	return nil
}
func (s *censusApplicationProgressPrivate) result(report []byte, r censusInventoryReportPrivate) error {
	if !s.claimed {
		return nil
	}
	v := censusApplicationResultPrivate{"lsp-trace.census-application-result.private.v1", s.claimSHA, pilot.Digest(report), r.Status, r.Failure}
	raw, _ := json.Marshal(v)
	return s.store.write(censusApplicationLeafPrivate(s.next, "result"), raw, 64<<10)
}
func (s *censusApplicationProgressPrivate) revalidate(ctx context.Context, o censusCLIOptions, c censusInventoryConfigPrivate, enumerate func(context.Context, censusCLIOptions) ([]censusacquisition.SourceFile, error)) error {
	current, err := loadCensusInventoryConfigContextPrivate(ctx, o.PrivateInventoryConfigPath)
	if err != nil {
		return err
	}
	a, _ := json.Marshal(c)
	b, _ := json.Marshal(current)
	if !bytes.Equal(a, b) {
		return errors.New("application approval/config drift")
	}
	files, err := enumerate(ctx, o)
	if err != nil {
		return err
	}
	rows, _ := censusInventoryPlanPrivate(o, c, files)
	if len(rows) != len(s.rows) {
		return errors.New("application denominator drift")
	}
	for i := range rows {
		if rows[i].File != s.rows[i].File || rows[i].Disposition != s.rows[i].Disposition {
			return errors.New("application metadata drift")
		}
	}
	_, raw, err := buildCensusApplicationFullPlanPrivate(ctx, o, c, rows, s.pages, s.runtimeSHA)
	if err != nil {
		return err
	}
	if !bytes.Equal(raw, s.raw) {
		return errors.New("application full source drift")
	}
	retained, err := s.store.read("full-source-plan.json", c.Application.MaxPlanBytes)
	if err != nil || !bytes.Equal(retained, s.raw) {
		return errors.Join(err, errors.New("application retained plan drift"))
	}
	for i, prior := range s.verified {
		v, e := s.verify(i)
		if e != nil {
			return e
		}
		x, _ := json.Marshal(v)
		y, _ := json.Marshal(prior)
		if !bytes.Equal(x, y) {
			return errors.New("application prior receipt drift")
		}
	}
	return s.store.check()
}
