//go:build darwin || linux

package privatehostproductv3

import pilot "lsp-trace/internal/censuslineagepilot"

// Current configuration pins are checked as metadata BEFORE a source Freeze.
// They bind bytes, not authenticated history or restored capture ownership.
func ValidateTransfer(c Config, t Transfer) error {
	var m pilot.RetainedPageCheckpointManifestPrivateV3
	if ValidateConfig(c) != nil || len(t.Manifest) == 0 || len(t.Manifest) > 4096 || strict(t.Manifest, &m) != nil {
		return ErrInvalid
	}
	if m.Application != c.Application || m.ScopeSHA256 != c.ScopeSHA256 || m.Revision.Policy.ConfigSHA256 != c.ProducerConfigSHA256 || m.Targets != uint64(c.Requested) {
		return ErrInvalid
	}
	if len(t.Spec.Members) > 16 {
		return ErrInvalid
	}
	for _, member := range t.Spec.Members {
		if len(member.CanonicalRelativePath) > 4096 || len(member.ExpectedSHA256) > 128 || len(member.LanguageID) > 128 {
			return ErrInvalid
		}
	}
	if len(t.Spec.Root) > 4096 || len(t.Spec.RevisionAssertion) > 128 {
		return ErrInvalid
	}
	spec := mustJSON(t.Spec)
	if len(spec) > 128<<10 || pilot.Digest(spec) != m.SpecSHA256 {
		return ErrInvalid
	}
	return nil
}
