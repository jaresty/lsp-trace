//go:build darwin || linux

package privatehostproductv3

import (
	"context"
	"encoding/json"
	ai "lsp-trace/internal/applicationinventory"
	inv "lsp-trace/internal/inventorysuccessor"
	sc "lsp-trace/internal/inventorysuccessor/sourcecontinuation"
	so "lsp-trace/internal/inventorysuccessor/sourceoperator"
	"net/url"
	"path/filepath"
	"strings"
	"time"
)

// InventoryNativeInputV3 is invocation-local capability input. It is deliberately
// not a JSON configuration field or a callback capable of fabricating opaque DTOs.
type InventoryNativeInputV3 struct {
	Lead          ai.Lead
	Owner         *inv.PrivateManagedInputOwnerV2
	Capture       inv.PrivateMixedInputRequestV2
	Configuration inv.PrivateSelectedConfigurationV2
	Role          sc.PrivateSelectedRoleV3
}
type InventoryNativeInputsV3 map[string]InventoryNativeInputV3

// Target pins narrow data to the exact capture; they do not admit an owner.
type InventoryTargetV3 struct {
	LeadID, Revision, Workspace, URI                      string
	Line, Character                                       uint32
	Role                                                  sc.PrivateSelectedRoleV3
	CaptureConfigurationSHA256, NativeConfigurationSHA256 string
}

func InventoryNativeTargetV3(v InventoryNativeInputV3) InventoryTargetV3 {
	// No manager, publication root or owner is serialized or reconstructed.
	capture := struct {
		Workspace, URI, OccurrenceID, Revision, Privacy, OccurrencePolicy, CompositionPolicy, LanguageID string
		Line, Character                                                                                  uint32
		Kinds                                                                                            any
		StartConfiguration                                                                               any
	}{v.Capture.Start.Profile.Workspace().String(), v.Capture.URI, v.Capture.OccurrenceID, v.Capture.Revision, v.Capture.Privacy, v.Capture.OccurrencePolicy, v.Capture.CompositionPolicy, v.Capture.Start.LanguageID, v.Capture.Line, v.Capture.Character, v.Capture.Kinds, struct {
		Profile, Seed, Provider, Process any
		Alias                            string
		Relations                        []string
		Deadline                         time.Time
	}{v.Capture.Start.Profile.SessionKey().String(), v.Capture.Start.SeedBinding, v.Capture.Start.ProviderIdentity, v.Capture.Start.Process, v.Capture.Start.BootstrapAlias, v.Capture.Start.RelationProviders, v.Capture.Start.Deadline}}
	raw, e := json.Marshal(capture)
	native, ne := json.Marshal(struct {
		Configuration inv.PrivateSelectedConfigurationV2
		ContractID    string
	}{v.Configuration, v.Configuration.Contract.ID()})
	if e != nil || ne != nil {
		return InventoryTargetV3{}
	}
	return InventoryTargetV3{v.Lead.ID, v.Capture.Revision, capture.Workspace, v.Capture.URI, v.Capture.Line, v.Capture.Character, v.Role, hash(raw), hash(native)}
}
func pathRelatedInventoryV3(a, b string) bool {
	return a == b || strings.HasPrefix(a, b+string(filepath.Separator)) || strings.HasPrefix(b, a+string(filepath.Separator))
}
func (inputs InventoryNativeInputsV3) Preparer(sourceRoot, revision string, targets ...InventoryTargetV3) ai.NativePreparer {
	// Copy descriptors, not owners. Nothing constructs, opens, renews or funds one.
	expected := make(map[string]InventoryTargetV3, len(targets))
	for _, t := range targets {
		expected[t.LeadID] = t
	}
	owned := make(InventoryNativeInputsV3, len(inputs))
	for k, v := range inputs {
		v.Capture.Kinds = append(v.Capture.Kinds[:0:0], v.Capture.Kinds...)
		owned[k] = v
	}
	return func(ctx context.Context, l ai.Lead) (ai.NativeResult, error) {
		v, ok := owned[l.ID]
		if !ok || v.Owner == nil || v.Lead != l || ctx == nil || ctx.Err() != nil || revision != InventorySourceRevisionV3 || v.Capture.Revision != revision || !v.Role.Valid() || v.Capture.Start.Profile.Workspace().String() != sourceRoot || v.Capture.Start.Deadline.IsZero() || !time.Now().Before(v.Capture.Start.Deadline) || time.Until(v.Capture.Start.Deadline) > 15*time.Second {
			return ai.NativeResult{}, ErrInvalid
		}
		target, admitted := expected[l.ID]
		if !admitted || InventoryNativeTargetV3(v) != target {
			return ai.NativeResult{}, ErrInvalid
		}
		uri, e := url.Parse(v.Capture.URI)
		if e != nil || uri.Scheme != "file" || uri.Host != "" || !strings.HasPrefix(filepath.Clean(uri.Path), sourceRoot+string(filepath.Separator)) {
			return ai.NativeResult{}, ErrInvalid
		}
		present := false
		for _, k := range v.Capture.Kinds {
			present = present || string(k) == v.Role.Kind
		}
		if !present {
			return ai.NativeResult{}, ErrInvalid
		}
		// The public operator alone admits the live owner, exact captured configuration,
		// original deadline, root/head, three attempts and fourteen native currencies.
		selected, e := so.PrepareSelectedRoleV3(ctx, v.Owner, v.Capture, v.Configuration, v.Role)
		if e != nil {
			return ai.NativeResult{}, e
		}
		return ai.NativeResult{Selected: selected, Status: "SELECTED"}, nil
	}
}
