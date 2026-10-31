//go:build darwin || linux

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"

	"lsp-trace/internal/censusacquisition"
	pilot "lsp-trace/internal/censuslineagepilot"
)

const censusApplicationProfilePrivate = "PRIVATE_CENSUS_APPLICATION_V1"
const censusApplicationTerminalProfilePrivate = "PRIVATE_CENSUS_APPLICATION_TERMINAL_V2"

type censusApplicationConfigPrivate struct {
	Profile, Directory                                        string
	MaxPlanBytes, MaxArtifactBytes, MaxReadBytes, MaxSegments int
}

func (a censusApplicationConfigPrivate) validate() error {
	if (a.Profile != censusApplicationProfilePrivate && a.Profile != censusApplicationTerminalProfilePrivate) || a.Directory == "" || a.MaxPlanBytes < 1 || a.MaxPlanBytes > 16<<20 || a.MaxArtifactBytes < 1 || a.MaxArtifactBytes > 16<<20 || a.MaxReadBytes < 1 || a.MaxReadBytes > 256<<20 || a.MaxSegments < 1 || a.MaxSegments > 100000 {
		return errors.New("application private profile/bounds")
	}
	return nil
}

type censusApplicationFrozenOriginPrivate struct {
	Files    []int
	Manifest json.RawMessage // metadata/digests, NOT retained source or live authority
}
type censusApplicationFullPlanPrivate struct {
	Authority                                                                       int
	Accepted                                                                        bool
	FeatureIdentity, SourceCompleteness, AbsenceInference                           string
	Schema, InputFamily, Workspace, GitObservation, RevisionCustody, ControlsSHA256 string
	RuntimeConfigurationSHA256                                                      string
	Configuration                                                                   censusInventoryConfigPrivate
	Sources, Includes, Excludes, PrunedDirectories                                  []string
	Denominator                                                                     []censusInventoryPagePrivate
	Origins                                                                         []censusApplicationFrozenOriginPrivate
	Segments                                                                        [][]int
}
type censusApplicationBindingPrivate struct {
	FullPlanSHA256                   string
	Segment, SegmentCount, ProcessID int
	ClaimSHA256, ConfigurationSHA256 string
}
type censusApplicationScopePrivate struct {
	FullPlanSHA256 string
	Segment        int
}

// The git command is local metadata only and bounded by the SAME invocation
// context and 4KiB output budget. Revision assertion remains caller-asserted.
type censusApplicationGitOutputPrivate struct{ bytes.Buffer }

func (b *censusApplicationGitOutputPrivate) Write(p []byte) (int, error) {
	if len(p) > 4096-b.Len() {
		return 0, errors.New("application git metadata bound")
	}
	return b.Buffer.Write(p)
}
func censusApplicationGitPrivate(ctx context.Context, workspace string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", workspace, "rev-parse", "--show-toplevel", "--verify", "HEAD")
	out := &censusApplicationGitOutputPrivate{}
	cmd.Stdout = out
	cmd.Stderr = io.Discard
	err := cmd.Run()
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	if err != nil {
		if e, ok := err.(*exec.ExitError); ok && e.ExitCode() == 128 {
			return "UNAVAILABLE_NOT_AUTHENTICATED", nil
		}
		return "", err
	}
	return out.String(), nil
}
func buildCensusApplicationFullPlanPrivate(ctx context.Context, o censusCLIOptions, c censusInventoryConfigPrivate, rows []censusInventoryFilePrivate, pages []censusInventoryPagePrivate, runtimeSHA string) (p censusApplicationFullPlanPrivate, raw []byte, err error) {
	if c.Application == nil || c.Application.validate() != nil {
		return p, nil, errors.New("application config required")
	}
	segments, _, _, err := censusApplicationSegmentsPrivate(ctx, rows, c.MaxOrigins, c.Policy.MaxNominationFiles)
	if err != nil {
		return p, nil, err
	}
	if len(segments) > c.Application.MaxSegments {
		return p, nil, errors.New("application segment bound")
	}
	git, err := censusApplicationGitPrivate(ctx, o.Workspace)
	if err != nil {
		return p, nil, err
	}
	// Preview is an operation selector, not an execution/security/source control.
	// Keep every other option (including paths, provider args and bounds) pinned.
	controlsOptions := o
	controlsOptions.PrivateInventoryScopePreview = false
	controls, _ := json.Marshal(controlsOptions)
	clean := c
	clean.Automation = nil
	clean.CohortAutomation = nil
	p = censusApplicationFullPlanPrivate{FeatureIdentity: "UNRESOLVED", SourceCompleteness: "UNKNOWN", AbsenceInference: "PROHIBITED", Schema: "lsp-trace.census-application-source-plan.private.v1", InputFamily: "APPLICATION_SOURCE_OBSERVATIONS_NOT_AUTHORITY", Workspace: o.Workspace, GitObservation: git, RevisionCustody: "CALLER_ASSERTED_NOT_AUTHENTICATED", ControlsSHA256: pilot.Digest(controls), RuntimeConfigurationSHA256: runtimeSHA, Configuration: clean, Sources: o.Sources, Includes: o.Includes, Excludes: o.Excludes, PrunedDirectories: o.PrunedDirectories, Denominator: pages}
	observedBytes := 0
	for _, segment := range segments {
		ordinals := []int{}
		for _, origin := range segment.Origins {
			subset := make([]censusacquisition.SourceFile, len(origin.Files))
			for j, index := range origin.Files {
				subset[j] = rows[index].File
			}
			// Every call creates a validation-only owner, immediately discarded after
			// copying manifest metadata. It is never supplied to a Manager/discovery.
			f, e := pilot.FreezeCensusScopePrivateV2(ctx, o.Workspace, c.RevisionAssertion, subset, c.Policy)
			if e != nil {
				return p, nil, e
			}
			manifest := f.ManifestBytes()
			observedBytes += len(manifest)
			if observedBytes > c.Application.MaxPlanBytes {
				return p, nil, errors.New("application source-plan byte bound")
			}
			ordinals = append(ordinals, len(p.Origins))
			p.Origins = append(p.Origins, censusApplicationFrozenOriginPrivate{append([]int(nil), origin.Files...), manifest})
		}
		p.Segments = append(p.Segments, ordinals)
	}
	raw, err = json.Marshal(p)
	if err != nil || len(raw) > c.Application.MaxPlanBytes {
		return p, nil, errors.New("application source-plan byte bound")
	}
	return p, raw, ctx.Err()
}
func (p censusApplicationFullPlanPrivate) groups(segment int) [][]int {
	if segment < 0 || segment >= len(p.Segments) {
		return nil
	}
	groups := make([][]int, len(p.Segments[segment]))
	for i, ordinal := range p.Segments[segment] {
		groups[i] = append([]int(nil), p.Origins[ordinal].Files...)
	}
	return groups
}
func censusApplicationLeafPrivate(segment int, kind string) string {
	return fmt.Sprintf("segment-%06d.%s.json", segment, kind)
}
