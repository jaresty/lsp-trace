package transientstructural

import (
	"crypto/sha256"
	"encoding/hex"
)

const (
	lifecyclePolicyID      = "lsp-trace.transient-structural-lifecycle"
	lifecyclePolicyVersion = "1"
	privacyPolicyID        = "lsp-trace.transient-structural-privacy"
	privacyPolicyVersion   = "1"
	identityPolicyID       = "lsp-trace.transient-structural-identity"
	identityPolicyVersion  = "1"
	analysisPolicyID       = "lsp-trace.transient-structural-analysis"
	analysisPolicyVersion  = "1"

	resultSchemaVersion        = "lsp-trace.transient-structural-result.v1"
	evidenceClassTransientLive = "TRANSIENT_LIVE"
	sourceGraphCompleteUnknown = "UNKNOWN"
	claimCeiling               = "Under the named managed session generation, exact target, server responses, traversal bounds, and analysis policy, this bounded server-reported call graph has the reported structural properties."
)

// These documents are deliberately canonical, compact, and immutable. Their
// digests bind every successful claim to the precise lifecycle, privacy,
// identity, and analysis semantics implemented by this package.
const lifecyclePolicyDocument = `{"id":"lsp-trace.transient-structural-lifecycle","version":"1","phases":["PREFLIGHT","TRAVERSAL","ADMISSION","ANALYSIS","DELIVERY_CHECK"],"errors":{"PREFLIGHT":["UNSUPPORTED","AMBIGUOUS_TARGET","TARGET_NOT_FOUND","RESOURCE_LIMIT","TIMEOUT","CANCELLED","GENERATION_CHANGED","INVALID_SERVER_RESPONSE"],"TRAVERSAL":["PARTIAL","TRUNCATED","RESOURCE_LIMIT","TIMEOUT","CANCELLED","GENERATION_CHANGED","INVALID_SERVER_RESPONSE"],"ADMISSION":["RESOURCE_LIMIT","CANCELLED","GENERATION_CHANGED","INVALID_SERVER_RESPONSE"],"ANALYSIS":["RESOURCE_LIMIT","TIMEOUT","CANCELLED","GENERATION_CHANGED","ANALYSIS_FAILED"],"DELIVERY_CHECK":["CANCELLED","GENERATION_CHANGED"]},"success":{"phase":"DELIVERY_CHECK","states":["COMPLETE","EMPTY"]},"complete_requires":{"failed":0,"cancelled":0,"rejected":0,"non_deduplication_omissions":0,"unexpanded_frontier_within_requested_depth":0,"truncated":false},"empty_iff":{"admitted_calls":0}}`

const privacyPolicyDocument = `{"id":"lsp-trace.transient-structural-privacy","version":"1","allow":["fixed_enums","counts","bounds","policy_ids","policy_versions","policy_digests","canonical_session_identity","exact_generation","position_encoding","opaque_node_ids","opaque_occurrence_ids","graph_digest"],"deny":["uri","path","symbol_name","detail","source","source_snippet","range","call_site","provider_private_detail","arbitrary_diagnostic","raw_graph","raw_node","raw_edge","admission_api"]}`

const identityPolicyDocument = `{"id":"lsp-trace.transient-structural-identity","version":"1","algorithm":"sha256","node_domain":"lsp-trace/transient-structural/node/v1","occurrence_domain":"lsp-trace/transient-structural/occurrence/v1","graph_domain":"lsp-trace/transient-structural/graph/v1","salt":"canonical_session_identity+exact_generation","canonical_order":"lexicographic","occurrence_key":"CALLS+SERVER_REPORTED+caller+callee+canonical_call_site"}`

const analysisPolicyDocument = `{"id":"lsp-trace.transient-structural-analysis","version":"1","relations":["CALLS"],"algorithms":{"NEIGHBORHOOD":{"version":"1","semantics":"target-rooted admitted nodes and call occurrences reachable within exact independent incoming and outgoing bounds"},"IMPACT":{"version":"1","semantics":"target-rooted admitted directed reachability up to requested depth; target excluded from impacted nodes; self-call occurrence retained"}},"topmost_siblings":false}`

func digestDocument(document string) string {
	sum := sha256.Sum256([]byte(document))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func frozenPolicyBinding() PolicyBinding {
	return PolicyBinding{
		LifecycleID: lifecyclePolicyID, LifecycleVersion: lifecyclePolicyVersion, LifecycleSHA256: digestDocument(lifecyclePolicyDocument),
		PrivacyID: privacyPolicyID, PrivacyVersion: privacyPolicyVersion, PrivacySHA256: digestDocument(privacyPolicyDocument),
		IdentityID: identityPolicyID, IdentityVersion: identityPolicyVersion, IdentitySHA256: digestDocument(identityPolicyDocument),
		AnalysisID: analysisPolicyID, AnalysisVersion: analysisPolicyVersion, AnalysisSHA256: digestDocument(analysisPolicyDocument),
	}
}
