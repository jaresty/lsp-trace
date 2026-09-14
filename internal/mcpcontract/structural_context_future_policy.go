package mcpcontract

const (
	futurePolicyStatus = "PROVISIONAL_NONCERTIFIED"

	futureTraversalPolicyID = "transient-calls-traversal.v1"
	futureResourcePolicyID  = "transient-structural-resources.v1"
	futureNeighborhoodID    = "transient-neighborhood.v1"
	futureImpactID          = "transient-impact.v1"
	futureAnalysisVersion   = "1"

	futureDefaultMaxMessages = 64
	futureDefaultMaxBytes    = 4194304
	futureMaxMessages        = 4096
	futureMaxBytes           = 16777216

	futureTraversalPolicyDigest    = "sha256:a22ece2850382f9c97efb7203dcf7346024643f8be1d58192297ab0e67cc5770"
	futureResourcePolicyDigest     = "sha256:26f43e823046616c0ff68750516511e49d575914fa6d3b9dd71913cfab954f23"
	futureNeighborhoodPolicyDigest = "sha256:731fff0e5725c5031ff25daad58e737acae14dc1f4d342720a8cb4f313fee8da"
	futureImpactPolicyDigest       = "sha256:e99184ad9674d6f16abdd9184585e0f53994268aee6ab8f2d6733d52a8ad2e65"
)

// These compact ASCII JSON documents are the immutable, exact-byte identities
// for the still-unregistered operation-35 draft. They are provisional and do
// not certify a runtime, kernel, manager, or transport implementation.
const futureTraversalPolicyDocument = `{"policy_id":"transient-calls-traversal.v1","policy_version":"1","policy_status":"PROVISIONAL_NONCERTIFIED","relation":"CALLS","down_depth":{"minimum":0,"maximum":64},"up_depth":{"minimum":0,"maximum":64},"max_nodes":{"minimum":1,"maximum":10000}}`

const futureResourcePolicyDocument = `{"policy_id":"transient-structural-resources.v1","policy_version":"1","policy_status":"PROVISIONAL_NONCERTIFIED","timeout_ms":{"minimum":1,"maximum":60000},"request_timeout_ms":{"minimum":1,"maximum":60000,"maximum_relation":"request_timeout_ms<=timeout_ms"},"max_messages":{"minimum":1,"maximum":4096,"default":64},"max_bytes":{"minimum":1,"maximum":16777216,"default":4194304}}`

const futureNeighborhoodPolicyDocument = `{"policy_id":"transient-neighborhood.v1","policy_version":"1","policy_status":"PROVISIONAL_NONCERTIFIED","kind":"NEIGHBORHOOD","relation":"CALLS","semantics":"target_rooted_admitted_nodes_and_call_edges_within_exact_independent_incoming_and_outgoing_traversal_bounds"}`

const futureImpactPolicyDocument = `{"policy_id":"transient-impact.v1","policy_version":"1","policy_status":"PROVISIONAL_NONCERTIFIED","kind":"IMPACT","relation":"CALLS","directions":["INCOMING","OUTGOING"],"depth":{"minimum":1,"maximum":64},"semantics":"target_rooted_admitted_directed_reachability_within_requested_direction_and_depth_without_traversal_expansion"}`
