//go:build darwin || linux

package sourceoperator

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"

	"lsp-trace/internal/describeworker"
	sc "lsp-trace/internal/inventorysuccessor/sourcecontinuation"
)

// nativeOutcome has no public constructor or JSON admission. Only the fixed
// actual delivered opaque response can seal a publication candidate here.
type nativeOutcome struct {
	id  string
	raw []byte
}
type nativeMachine struct {
	Supplied json.RawMessage                   `json:"supplied"`
	Delivery json.RawMessage                   `json:"delivery"`
	Semantic describeworker.SemanticResponseV2 `json:"semantic"`
	Counts   sc.PrivateSourceCounts            `json:"counts"`
	Gaps     []sc.PrivateSourceOutcome         `json:"gaps"`
}
type nativeEnvelope struct {
	Schema         string        `json:"schema"`
	Origin         string        `json:"origin"`
	Execution      string        `json:"execution"`
	DeliveryOrigin string        `json:"delivery_origin"`
	HostIdentity   string        `json:"host_identity"`
	Authority      int           `json:"authority"`
	Accepted       bool          `json:"accepted"`
	Identity       string        `json:"identity"`
	Completeness   string        `json:"completeness"`
	Absence        string        `json:"absence"`
	Machine        nativeMachine `json:"machine"`
	HumanInventory string        `json:"human_inventory"`
}

// Native encoding stays under the original source 8MiB aggregate ceiling, not
// the byte utility's larger 16MiB ceiling. A complete retained-plus-transient
// bound is admitted before HOST, and again from opaque actual byte metadata
// immediately before the first defensive accessor below.
const nativeMaxBytes = 8 << 20

func (p *publicationOwner) sealNative(delivered sc.DeliveredSourceResponse, source *sc.PrivateAllModeSourcesResult) (nativeOutcome, error) {
	if p == nil || !p.admitted || source == nil {
		return nativeOutcome{}, ErrRefused
	}
	s, d, semanticUpper, e := delivered.NativePayloadBounds()
	if e != nil {
		return nativeOutcome{}, e
	}
	gaps, e := nativeGapUpper(source)
	if e != nil {
		return nativeOutcome{}, e
	}
	wire, _, e := nativeAggregateUpper(p.upstream, s, d, semanticUpper, gaps, p.limit)
	if e != nil || wire > p.nativeUpper {
		return nativeOutcome{}, ErrRefused
	}
	// No native Bytes/Semantic, marshal, hash or publication occurs above here.
	supplied, e := delivered.Supplied().Bytes()
	if e != nil {
		return nativeOutcome{}, e
	}
	delivery, e := delivered.Delivery().Bytes()
	if e != nil {
		return nativeOutcome{}, e
	}
	if len(supplied) > 1<<20 || len(delivery) > 1<<20 || len(source.Outcomes) > 1000 {
		return nativeOutcome{}, ErrRefused
	}
	semantic := delivered.Supplied().Semantic()
	// This is a source-provisional rendering, NOT the old AST inventory schema.
	// Citation suggestions remain suggestions in machine semantics, never citations.
	human := fmt.Sprintf("SOURCE-PROVISIONAL — SUPPLIED claims only\nVerdict: %s\nTarget role: %s\nConsumer need (%s): %s\nProvided behavior: %s\nBoundary contribution: %s\nLimitations: %s\nCitation suggestions are not admitted citations.\nAuthority 0; accepted false; identity UNRESOLVED; completeness UNKNOWN; absence PROHIBITED.\nOther six modes: DECLARED_MISSING_UNRESOLVED.\nLocal file delivery does not authenticate a role, model or execution.\n", semantic.Verdict, semantic.TargetRole, semantic.ConsumerNeed.Status, semantic.ConsumerNeed.Value, semantic.ProvidedBehavior.Value, semantic.BoundaryContribution, strings.Join(semantic.Limitations, "; "))
	envelope := nativeEnvelope{Schema: "SOURCE_NATIVE_V1", Origin: "SUPPLIED_TERMINAL_RESPONSE_ONLY", Execution: "NO_PROCESS_OR_HOST_EXECUTION_VERIFIED", DeliveryOrigin: "LOCAL_HOST_ROLE_FILE_TRANSPORT_DELIVERED", HostIdentity: "HOST_OBSERVED_NOT_AUTHENTICATED", Identity: "UNRESOLVED", Completeness: "UNKNOWN", Absence: "PROHIBITED", Machine: nativeMachine{supplied, delivery, semantic, source.Counts, source.Outcomes}, HumanInventory: human}
	raw, e := json.Marshal(envelope)
	if e != nil || len(raw) == 0 || len(raw) >= nativeMaxBytes || uint64(len(raw))+1 > wire {
		return nativeOutcome{}, ErrRefused
	}
	raw = append(raw, '\n')
	return nativeOutcome{fmt.Sprintf("sha256:%x", sha256.Sum256(raw)), raw}, nil
}
