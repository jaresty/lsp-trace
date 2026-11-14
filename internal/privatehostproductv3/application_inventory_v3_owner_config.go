//go:build darwin || linux

package privatehostproductv3

import (
	"context"
	"encoding/json"
	"net/url"
	"path/filepath"
	"reflect"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"

	ai "lsp-trace/internal/applicationinventory"
	cc "lsp-trace/internal/censuscontinuation"
	inv "lsp-trace/internal/inventorysuccessor"
	sc "lsp-trace/internal/inventorysuccessor/sourcecontinuation"
	"lsp-trace/internal/managedprocess"
	pa "lsp-trace/internal/programcadmission"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/internal/targetpacket"
	"lsp-trace/sessionruntime"
)

// The closed decoder has already bounded and rejected duplicate/unknown keys.
// Inspect actual object keys, not string values; EqualFold also catches aliases,
// and JSON decoding catches escaped spellings before the canonical-byte check.
func inventoryOwnersPresentV3(raw []byte) bool {
	var outer map[string]json.RawMessage
	if json.Unmarshal(raw, &outer) != nil {
		return false
	}
	for key, value := range outer {
		if !strings.EqualFold(key, "Inventory") {
			continue
		}
		var fields map[string]json.RawMessage
		if json.Unmarshal(value, &fields) != nil {
			continue
		}
		for key := range fields {
			if strings.EqualFold(key, "NativeOwners") {
				return true
			}
		}
	}
	return false
}

// InventoryNativeOwnerV3 is DATA, not a serialized owner or source admission.
// CanonicalJob/ExpectedHead name already funded current accounting custody. Only
// OpenPrivateManagedInputOwnerV2 can admit it. This path never initializes READY.
// Deadline is the ORIGINAL absolute native deadline, including construction and
// cleanup. A HOST wait may exhaust it; no start-time policy or renewal is offered.
type InventoryNativeOwnerV3 struct {
	Lead                                                                      ai.Lead
	AccountingRoot                                                            string
	CanonicalJob                                                              []byte
	ExpectedHead                                                              string
	Verification                                                              inv.PrivateManagedInputVerificationV2
	Profile                                                                   runtimeprofile.Selector
	LanguageID, Provider, ProcessPath                                         string
	ProcessArgs                                                               []string
	URI, OccurrenceID, Revision, Privacy, OccurrencePolicy, CompositionPolicy string
	Line, Character                                                           uint32
	Kinds                                                                     []pa.RelationKind
	Deadline                                                                  time.Time
	RunID, SeedNamespace, Lineage                                             string
	Seed                                                                      uint64
	Attempts                                                                  [3]string
	Contract                                                                  cc.ContractInput
	SourceLimits                                                              targetpacket.PrivateSourceLimits
	Role                                                                      sc.PrivateSelectedRoleV3
}

// Reject oversized nested DATA before marshaling/copying, including direct Go
// callers. Raw CLI decoding independently retains the unchanged 64KiB ceiling.
func inventoryOwnerShapeV3(ds []InventoryNativeOwnerV3) bool {
	if len(ds) > 14 {
		return false
	}
	left := 65536
	var bounded func(reflect.Value, int) bool
	bounded = func(v reflect.Value, depth int) bool {
		if depth > 16 {
			return false
		}
		if v.Type() == reflect.TypeOf(time.Time{}) {
			left -= 64
			return left >= 0
		}
		switch v.Kind() {
		case reflect.String:
			n := v.Len()
			left -= n
			return n <= 4096 && left >= 0
		case reflect.Slice, reflect.Array:
			if v.Type().Elem().Kind() == reflect.Uint8 {
				left -= v.Len()
				return v.Len() <= 16<<10 && left >= 0
			}
			if v.Len() > 64 {
				return false
			}
			for i := 0; i < v.Len(); i++ {
				if !bounded(v.Index(i), depth+1) {
					return false
				}
			}
		case reflect.Struct:
			for i := 0; i < v.NumField(); i++ {
				if !bounded(v.Field(i), depth+1) {
					return false
				}
			}
		case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			left -= 8
		default:
			return false
		}
		return left >= 0
	}
	for _, d := range ds {
		if !bounded(reflect.ValueOf(d), 0) {
			return false
		}
	}
	return true
}

func ownerFieldV3(s string) bool {
	return len(s) > 0 && len(s) <= 4096 && utf8.ValidString(s) && !strings.ContainsRune(s, 0)
}

// Conservative alias refusal even on case-sensitive hosts: distinct ownership
// roots must not differ only by Unicode normalization or component case. This
// is data admission, not a proof of filesystem identity or authorization.
func ownerPathsRelatedV3(a, b string) bool {
	return pathRelatedInventoryV3(strings.ToLower(norm.NFC.String(a)), strings.ToLower(norm.NFC.String(b)))
}

func ownerPathV3(p string) bool {
	return ownerFieldV3(p) && filepath.IsAbs(p) && filepath.Clean(p) == p && p != "/" && strings.Count(p, "/") <= 128
}

// DescriptorInput returns authority-free typed configuration for exact target
// comparison. It never supplies an Owner, Root, Manager, or opaque producer.
func (d InventoryNativeOwnerV3) DescriptorInput() (InventoryNativeInputV3, error) {
	fail := func() (InventoryNativeInputV3, error) { return InventoryNativeInputV3{}, ErrInvalid }
	if !inventoryOwnerShapeV3([]InventoryNativeOwnerV3{d}) {
		return fail()
	}
	if !ownerPathV3(d.AccountingRoot) || !ownerPathV3(d.ProcessPath) || !ownerPathV3(d.Profile.Workspace) || len(d.CanonicalJob) == 0 || len(d.CanonicalJob) > 16<<10 || !json.Valid(d.CanonicalJob) || !digestValue(d.ExpectedHead) || len(d.ProcessArgs) > 64 || len(d.Profile.Options) > 32 || len(d.Kinds) == 0 || len(d.Kinds) > 3 || len(d.Lead.ID) == 0 || len(d.Lead.ID) > 256 || len(d.Lead.Provenance) > 4096 || d.Deadline.IsZero() || !d.Role.Valid() || !d.SourceLimits.Valid() || d.Contract.ProgramCSeed != d.Seed {
		return fail()
	}
	for _, s := range []string{d.LanguageID, d.Provider, d.URI, d.OccurrenceID, d.Revision, d.Privacy, d.OccurrencePolicy, d.CompositionPolicy, d.RunID, d.SeedNamespace, d.Lineage, d.Profile.TrustDomain, d.Profile.Profile, d.Profile.EnvironmentReference} {
		if !ownerFieldV3(s) {
			return fail()
		}
	}
	for _, s := range d.ProcessArgs {
		if len(s) > 4096 || !utf8.ValidString(s) || strings.ContainsRune(s, 0) {
			return fail()
		}
	}
	for _, o := range d.Profile.Options {
		if !ownerFieldV3(o.Name) || !ownerFieldV3(o.Value) {
			return fail()
		}
	}
	for _, s := range []string{d.Contract.ContinuationID, d.Contract.ProfileID, d.Contract.WorkerPinID} {
		if !digestValue(s) {
			return fail()
		}
	}
	// Bound all nested policies before constructing their immutable contract.
	raw, e := json.Marshal(d)
	if e != nil || len(raw) > 48<<10 {
		return fail()
	}
	if d.Verification.Records == 0 || d.Verification.Records > 4096 || d.Verification.Bytes == 0 || d.Verification.Bytes > 64<<20 || d.Verification.Work == 0 || d.Verification.Work > 16384 {
		return fail()
	}
	u, e := url.Parse(d.URI)
	if e != nil || u.Scheme != "file" || u.Host != "" || u.RawQuery != "" || u.Fragment != "" || !strings.HasPrefix(filepath.Clean(u.Path), d.Profile.Workspace+string(filepath.Separator)) {
		return fail()
	}
	var bits uint8
	present := false
	for _, k := range d.Kinds {
		var bit uint8
		switch k {
		case pa.TypedCalls:
			bit = 1
		case pa.TypedReferencesSymbol:
			bit = 2
		case pa.TypedResolvesToDefinition:
			bit = 4
		default:
			return fail()
		}
		if bits&bit != 0 {
			return fail()
		}
		bits |= bit
		present = present || string(k) == d.Role.Kind
	}
	if !present {
		return fail()
	}
	for i, a := range d.Attempts {
		if !digestValue(a) {
			return fail()
		}
		for j := 0; j < i; j++ {
			if d.Attempts[j] == a {
				return fail()
			}
		}
	}
	p, e := runtimeprofile.Validate(d.Profile)
	if e != nil {
		return fail()
	}
	c, e := cc.NewContinuationContract(d.Contract)
	if e != nil {
		return fail()
	}
	capture := inv.PrivateMixedInputRequestV2{Start: sessionruntime.StartRequest{Profile: runtimeprofile.Resolve(p), LanguageID: d.LanguageID, RelationProviders: []string{d.Provider}, Deadline: d.Deadline, Process: managedprocess.Spec{Path: d.ProcessPath, Dir: d.Profile.Workspace, Args: append([]string(nil), d.ProcessArgs...)}}, URI: d.URI, OccurrenceID: d.OccurrenceID, Revision: d.Revision, Privacy: d.Privacy, OccurrencePolicy: d.OccurrencePolicy, CompositionPolicy: d.CompositionPolicy, Line: d.Line, Character: d.Character, Kinds: append([]pa.RelationKind(nil), d.Kinds...)}
	configuration := inv.PrivateSelectedConfigurationV2{RunID: d.RunID, SeedNamespace: d.SeedNamespace, Lineage: d.Lineage, Seed: d.Seed, Attempts: d.Attempts, Contract: c, ContractInput: d.Contract, SourceLimits: d.SourceLimits}
	return InventoryNativeInputV3{Lead: d.Lead, Capture: capture, Configuration: configuration, Role: d.Role}, nil
}

// InventoryOwnerConstructionQuoteV3 prepays logical controller construction,
// verification and cleanup only; native M/P/S pays its own unchanged currencies.
// One opener verification is bounded by its actual Records/Bytes/Work. Fixed
// namespace/config/Manager terms cover <=129 path nodes, opens/stats/closes,
// transient JSON/profile/contract copies and shutdown bookkeeping. Work includes
// 32*2048 namespace/metadata operations plus <=2048 namespace operations per
// verifier work unit. All products are safe under the checked <=16384 work and
// <=64MiB byte limits. Calls here are processing, not HOST calls.
func InventoryOwnerConstructionQuoteV3(d InventoryNativeOwnerV3) (ApplicationBounds, error) {
	if _, e := d.DescriptorInput(); e != nil {
		return ApplicationBounds{}, e
	}
	raw, _ := json.Marshal(d)
	n := uint64(len(raw))
	l := d.Verification
	q := ApplicationBounds{LogicalBytes: 1<<20 + 16*n + l.Bytes, ReadHashBytes: 1<<20 + 2*l.Bytes, ParserBytes: 1<<20 + 2*l.Bytes + 8*n, Records: 512 + l.Records, Work: 65536 + 8*n + 2048*l.Work, ActiveNS: uint64(15 * time.Second)}
	if !applicationWithin(q, inventoryAllowanceV3(InventoryBindingV3{})) {
		return ApplicationBounds{}, ErrInvalid
	}
	return q, nil
}

func validateInventoryOwnersV3(q InventoryRequestV3) error {
	ds := q.Inventory.NativeOwners
	if !inventoryOwnerShapeV3(ds) {
		return ErrInvalid
	}
	if len(ds) == 0 {
		return nil
	} // legacy/embedding behavior is unchanged
	count := min(len(q.Inventory.Manifest.Leads), q.Inventory.MaxNativeAttempts)
	if len(ds) != count || count > 14 {
		return ErrInvalid
	}
	targets := map[string]InventoryTargetV3{}
	for _, t := range q.Inventory.Targets {
		targets[t.LeadID] = t
	}
	total := ApplicationBounds{}
	for i, d := range ds {
		if !ownerPathV3(q.Configuration.Store) || d.Lead != q.Inventory.Manifest.Leads[i] || d.Revision != q.Inventory.SourceRevision || d.Profile.Workspace != q.Configuration.SourceRoot || d.Deadline.UnixNano() > q.Configuration.Grant.ExpiresUnixNano || ownerPathsRelatedV3(d.AccountingRoot, q.Configuration.Store) || ownerPathsRelatedV3(d.AccountingRoot, q.Configuration.SourceRoot) {
			return ErrInvalid
		}
		for j := 0; j < i; j++ {
			if ownerPathsRelatedV3(d.AccountingRoot, ds[j].AccountingRoot) {
				return ErrInvalid
			}
		}
		v, e := d.DescriptorInput()
		if e != nil {
			return e
		}
		t, ok := targets[d.Lead.ID]
		if !ok || InventoryNativeTargetV3(v) != t {
			return ErrInvalid
		}
		b, e := InventoryOwnerConstructionQuoteV3(d)
		if e != nil {
			return e
		}
		var okAdd bool
		total, okAdd = applicationAdd(total, b)
		if !okAdd || !applicationWithin(total, inventoryAllowanceV3(InventoryBindingV3{})) {
			return ErrInvalid
		}
	}
	raw, e := json.Marshal(q)
	if e != nil || len(raw) > 64<<10 {
		return ErrInvalid
	}
	return nil
}

// ReserveInventoryOwnerConstructionV3 is an additional processing-only debit
// against the SAME finite application grant, not a new grant or HOST allowance.
// Its total declared quote must fit the unchanged fixed inventory envelope too.
// Reservation persists on failure and cannot be retried/refunded/replayed.
func (a *ApplicationJournal) ReserveInventoryOwnerConstructionV3(ctx context.Context, q InventoryRequestV3, runID string, d InventoryNativeOwnerV3) error {
	if ctx == nil || ctx.Err() != nil || a == nil || ValidateInventoryRequestV3(q) != nil || q.ApprovalSHA256 != InventoryRequestDigestV3(q) || a.Head().ApplicationSHA256 != ApplicationConfigDigest(q.Configuration) || !digestValue(runID) {
		return ErrInvalid
	}
	found := false
	for _, v := range q.Inventory.NativeOwners {
		if v.Lead == d.Lead && hash(mustJSON(v)) == hash(mustJSON(d)) {
			found = true
		}
	}
	if !found {
		return ErrInvalid
	}
	now := time.Now()
	if !now.Before(d.Deadline) || d.Deadline.Sub(now) > 15*time.Second {
		return ErrInvalid
	}
	b, e := InventoryOwnerConstructionQuoteV3(d)
	if e != nil {
		return e
	}
	id := hash([]byte(runID + "/inventory-owner-construction.v3/" + d.Lead.ID))
	_, e = a.ReserveOperation(id, "inventory_owner_construction.v3", b, now)
	return e
}
