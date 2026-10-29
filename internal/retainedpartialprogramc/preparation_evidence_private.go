package retainedpartialprogramc

import (
	"fmt"
	pilot "lsp-trace/internal/censuslineagepilot"
	"lsp-trace/internal/graphprovenance"
)

// PreparationEvidencePrivate returns detached consistency-checked observations
// only from a constructor-sealed projection. It grants no acquisition, replay,
// ordinary Describe admission, source authentication or semantic authority.
func (p Projection) PreparationEvidencePrivate() (pilot.DerivedRetainedPartialNativeInputPrivate, graphprovenance.SourceWithholdingPrivateV1, error) {
	if p.state == nil || p.state.self != p.state {
		return pilot.DerivedRetainedPartialNativeInputPrivate{}, graphprovenance.SourceWithholdingPrivateV1{}, fmt.Errorf("sealed private projection required")
	}
	e, err := pilot.ParseDerivedRetainedPartialNativePrivate(p.state.source.raw)
	if err != nil {
		return e, graphprovenance.SourceWithholdingPrivateV1{}, err
	}
	c, err := graphprovenance.ParseSourceWithholdingPrivateV1(e.Records[5])
	return e, c, err
}
