package censuslineagepilot

import (
	"path/filepath"
	"strings"

	"lsp-trace/internal/runtimeprofile"
)

const ScopedRehearsalConfigLimitPrivate = 32 << 10
const scopedRehearsalArtifactPrivate = 2 << 20
const scopedRehearsalTotalPrivate = 16 << 20
const scopedRehearsalObjectsPrivate = 8

func ScopedRehearsalBudgetFixedPrivate() ScopedRehearsalBudgetPrivate {
	return ScopedRehearsalBudgetPrivate{scopedRehearsalArtifactPrivate, scopedRehearsalTotalPrivate, scopedRehearsalObjectsPrivate, 2 * scopedRehearsalTotalPrivate, 16 * scopedRehearsalTotalPrivate}
}
func ScopedRehearsalBudgetPartialV2Private() ScopedRehearsalBudgetPrivate {
	b := ScopedRehearsalBudgetFixedPrivate()
	b.ArtifactBytes = 4 << 20
	return b
}
func (c ScopedRehearsalConfigPrivate) fixedBudget() ScopedRehearsalBudgetPrivate {
	if c.Schema == "lsp-trace.scoped-rehearsal.private.v4" {
		return ScopedRehearsalBudgetPartialV2Private()
	}
	return ScopedRehearsalBudgetFixedPrivate()
}
func (c ScopedRehearsalConfigPrivate) executionPolicy() (scopedExecutionPolicyPrivate, error) {
	switch c.Schema {
	case "lsp-trace.scoped-rehearsal.private.v1":
		return scopedExecutionPolicyFixedPrivate(), nil
	case "lsp-trace.scoped-rehearsal.private.v2":
		return scopedExecutionPolicyV2Private(), nil
	case "lsp-trace.scoped-rehearsal.private.v3":
		return scopedExecutionPartialInspectionPolicyPrivate(), nil
	case "lsp-trace.scoped-rehearsal.private.v4":
		return scopedExecutionPartialInspectionPolicyV2Private(), nil
	default:
		return scopedExecutionPolicyPrivate{}, ErrInvalid
	}
}

// ValidateScopedRehearsalConfigPurePrivate is the shared producer/retained-reader
// contract. Paths are syntax/comparison data only: no filesystem reads, profile
// resolution, executable lookup, Manager or runtime capability construction.
// The producer separately applies its existing outside-temporary-root rule.
func ValidateScopedRehearsalConfigPurePrivate(c ScopedRehearsalConfigPrivate) error {
	if _, err := c.executionPolicy(); err != nil {
		return err
	}
	if !c.Locator.valid() || !featureScopeRevision.MatchString(c.CodeHEAD) || c.Session.Validate() != nil || c.Budget != c.fixedBudget() || len(c.Args) > 16 || len(c.Environment) == 0 || len(c.Environment) > 32 {
		return ErrInvalid
	}
	for _, p := range append(append([]string{c.Workspace, c.Server}, c.InputRoots[:]...), c.OutputRoots[:]...) {
		if !filepath.IsAbs(p) || filepath.Clean(p) != p || len(p) > 4096 {
			return ErrInvalid
		}
	}
	for _, id := range c.OutputIdentities {
		if id.Inode == 0 {
			return ErrInvalid
		}
	}
	v, e := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: c.TrustDomain, Workspace: c.Workspace, Profile: c.Profile, EnvironmentReference: c.EnvironmentReference})
	if e != nil || v.Workspace().String() != c.Workspace {
		return ErrInvalid
	}
	previous := ""
	for _, entry := range c.Environment {
		name, _, ok := strings.Cut(entry, "=")
		if !ok || name == "" || name <= previous || strings.ContainsRune(entry, 0) || len(entry) > 4096 {
			return ErrInvalid
		}
		for i, ch := range []byte(name) {
			if !((ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') || ch == '_' || (i > 0 && ch >= '0' && ch <= '9')) {
				return ErrInvalid
			}
		}
		previous = name
	}
	for _, arg := range c.Args {
		if len(arg) > 4096 || strings.ContainsRune(arg, 0) {
			return ErrInvalid
		}
	}
	return nil
}
