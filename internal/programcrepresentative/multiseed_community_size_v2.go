package programcrepresentative

import "lsp-trace/internal/programc"

// CanonicalRebuiltCommunitySizeV2 mirrors the frozen BuildCommunityArtifact
// output shape without allocating a projection, cloning members, or marshaling.
// Its community IDs are fixed-width lowercase sha256 values; this walk does not
// invent or decode community identities.
func CanonicalRebuiltCommunitySizeV2(o programc.Outcome, id programc.InstabilityIdentity) (uint64, error) {
	var z canonicalSizerV2
	z.add(2)
	z.key("schema_version", true)
	z.str("lsp-trace.community.v1")
	z.key("input_identity", false)
	z.add(2)
	z.key("graph_provenance_v5_sha256", true)
	z.str(o.Source.GraphV5SHA256)
	z.key("admitted_graph_sha256", false)
	z.str(id.AdmittedGraphSHA256)
	z.key("projection_sha256", false)
	z.str(id.ProjectionSHA256)
	z.key("projection_policy_id", false)
	z.str(id.ProjectionPolicyID)
	z.key("algorithm_name", false)
	z.str(id.AlgorithmName)
	z.key("algorithm_version", false)
	z.str(id.AlgorithmVersion)
	z.key("parameters_canonical_sha256", false)
	z.str(id.ParametersCanonicalSHA256)
	z.key("seed", false)
	z.num(o.Seed)
	z.key("resource_policy_sha256", false)
	z.str(id.ResourcePolicySHA256)
	z.key("outcome", false)
	z.str(o.Outcome)
	z.key("communities", false)
	z.add(2)
	for i, c := range o.Communities {
		if z.err != nil {
			return z.n, z.err
		}
		if i > 0 {
			z.add(1)
		}
		z.add(2)
		z.key("community_id", true)
		z.str("sha256:0000000000000000000000000000000000000000000000000000000000000000")
		z.key("member_node_ids", false)
		z.strings(c.Members)
	}
	z.key("accounting", false)
	z.add(2)
	z.key("admitted_node_count", true)
	z.integer(len(o.Projection.NodeIdentities))
	z.key("accounted_node_count", false)
	z.integer(len(o.Projection.NodeIdentities))
	z.key("unaccounted_node_count", false)
	z.num(0)
	z.key("claim_ceiling", false)
	z.str("Communities are structural observations under the exact bound Graph Provenance V5 input and projection. They do not establish feature, product, ownership, service, organizational, business-boundary, semantic-community, producer-authentication, permission, or production-authority claims.")
	return z.n, z.err
}
