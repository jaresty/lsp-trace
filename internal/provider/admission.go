package provider

import (
	"context"
	"errors"
	"fmt"
	"sort"
)

// ProvisionedAdmissionResolver resolves caller selectors against immutable,
// host-owned provisioning. It never accepts process execution authority.
type ProvisionedAdmissionResolver struct {
	declarations []Declaration
	adapterID    string
}

func NewAdmissionResolver(provisioned Provisioned, adapterID string) (*ProvisionedAdmissionResolver, error) {
	if provisioned.Registry == nil || adapterID == "" {
		return nil, errors.New("provider admission requires provisioning and adapter identity")
	}
	declarations := make([]Declaration, len(provisioned.Declarations))
	for i, declaration := range provisioned.Declarations {
		declarations[i] = cloneDeclaration(declaration)
	}
	sort.Slice(declarations, func(i, j int) bool { return declarations[i].Identity < declarations[j].Identity })
	return &ProvisionedAdmissionResolver{declarations: declarations, adapterID: adapterID}, nil
}

func (r *ProvisionedAdmissionResolver) Admit(_ context.Context, selection Selection) (Admission, error) {
	if r == nil {
		return Admission{}, errors.New("provider admission unavailable")
	}
	selector := "auto"
	if len(selection.Providers) == 1 {
		selector = selection.Providers[0]
	} else if len(selection.Providers) > 1 {
		return Admission{}, errors.New("provider selection requires at most one selector")
	}
	if selector == "none" {
		return Admission{}, errors.New("provider selection disabled")
	}
	if selector != "auto" && !stableProviderIdentity.MatchString(selector) {
		return Admission{}, fmt.Errorf("provider selector %q is not auto, none, or a registered stable identity", selector)
	}
	if len(selection.Relations) == 0 {
		return Admission{}, errors.New("provider admission requires relations")
	}
	wanted := append([]string(nil), selection.Relations...)
	sort.Strings(wanted)
	for i, relation := range wanted {
		if _, ok := supportedRelations[relation]; !ok {
			return Admission{}, fmt.Errorf("unsupported relation %q", relation)
		}
		if i > 0 && relation == wanted[i-1] {
			return Admission{}, fmt.Errorf("duplicate relation %q", relation)
		}
	}
	candidates := make([]Declaration, 0, len(r.declarations))
	for _, declaration := range r.declarations {
		if selector != "auto" && declaration.Identity != selector {
			continue
		}
		if supportsAll(declaration.Capabilities.Relations, wanted) {
			candidates = append(candidates, declaration)
		}
	}
	if selector != "auto" && len(candidates) == 0 {
		return Admission{}, fmt.Errorf("provider %q is unregistered or does not support selected relations", selector)
	}
	if selector == "auto" && len(candidates) != 1 {
		return Admission{}, fmt.Errorf("auto provider selection requires exactly one capable provider; got %d", len(candidates))
	}
	declaration := candidates[0]
	return Admission{ProviderID: declaration.Identity, AdapterID: r.adapterID, Limits: declaration.Limits}, nil
}

func supportsAll(capabilities, wanted []string) bool {
	for _, relation := range wanted {
		i := sort.SearchStrings(capabilities, relation)
		if i == len(capabilities) || capabilities[i] != relation {
			return false
		}
	}
	return true
}
