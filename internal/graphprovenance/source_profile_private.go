package graphprovenance

import (
	"encoding/json"
	"errors"
	"reflect"
)

// Internal profile views are constructed only after validating a versioned
// policy. They are not wire DTOs or interchangeable source capabilities.
type sourceMemberPrivate struct{ URI, SHA256, Role string }
type sourceProfilePrivate struct {
	version, policy, workspace, session string
	generation                          uint64
	members                             []sourceMemberPrivate
}

func sourceProfileV1Private(p SourcePolicyPrivateV1) (*sourceProfilePrivate, error) {
	if err := p.validate(); err != nil {
		return nil, err
	}
	v := &sourceProfilePrivate{version: VersionSourceWithholdingPrivateV1, policy: PolicySourceWithholdingPrivateV1, workspace: p.WorkspaceURI, session: p.SessionID, generation: p.Generation}
	for _, m := range p.Members {
		v.members = append(v.members, sourceMemberPrivate{m.URI, m.SHA256, m.Role})
	}
	return v, nil
}
func (p *sourceProfilePrivate) member(uri string) *sourceMemberPrivate {
	for i := range p.members {
		if p.members[i].URI == uri {
			return &p.members[i]
		}
	}
	return nil
}
func (p *sourceProfilePrivate) withholds(workspace, uri string) bool {
	_, s := RelativeURI(workspace, uri)
	return s == "" && p.member(uri) == nil
}
func (p *sourceProfilePrivate) withheld(workspace, uri string) Receipt {
	name, _ := RelativeURI(workspace, uri)
	r := Receipt{URI: uri, Path: name, Classification: PostTraversal, AnalyzedVersion: Unverified, Status: PolicyWithheldPrivateV1}
	raw, _ := json.Marshal(r)
	r.ID = digest(p.version+":withholding", raw)
	return r
}
func validateSourceProfilePrivate(s EvidenceV2, p *sourceProfilePrivate, withheldCount int) error {
	c := s.Acquisition.Request.Context
	if s.WorkspaceURI != p.workspace || c.SessionID != p.session || c.Generation != p.generation {
		return errors.New("private source policy context")
	}
	if err := validateV2ProfilePrivate(s, p); err != nil {
		return err
	}
	withheld := 0
	for _, r := range s.Captures {
		if p.withholds(s.WorkspaceURI, r.URI) {
			withheld++
			continue
		}
		if m := p.member(r.URI); m != nil && r.Status == "READABLE" && sourceHashPrivateV1(r.Content) != m.SHA256 {
			return errors.New("private source captured content drift")
		}
	}
	for _, r := range s.Supplies {
		if r.Receipt != nil {
			m := p.member(r.Receipt.URI)
			if m == nil || sourceHashPrivateV1(r.Receipt.Content) != m.SHA256 {
				return errors.New("private source supply outside policy/drift")
			}
		}
	}
	if withheld != withheldCount {
		return errors.New("private source withholding count")
	}
	return nil
}
func (p *sourceProfilePrivate) validWithheld(workspace string, r Receipt) bool {
	return reflect.DeepEqual(r, p.withheld(workspace, r.URI))
}
