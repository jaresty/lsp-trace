package referencegrouping

// qualificationApproval is an independently held maintainer decision over the
// entire receipt and its seven predecessors, not a trust map or caller claim.
// FrozenUnix authorizes the exact plan before ExecutionUnix. Reviewer requirements
// belong to the independently approved policy contract (Plan), never ambient config.
type qualificationApproval struct {
	Receipt                           qualificationCommitment
	Policy, Input                     qualificationCommitment
	Evidence                          [5]qualificationCommitment
	RequiredClass                     string
	RequiredAcceptances               int
	Reviewers                         []qualificationReviewer
	ValidFromEpoch, ValidThroughEpoch uint64
	FrozenUnix, DecisionUnix          int64
}

// UNISSUED: no genuine approved evaluation/maintainer records were supplied for
// this implementation. Return fresh storage when actual immutable declarations
// are installed by a separately reviewed source change. There is deliberately no
// registration API, production fixture, key fallback, or mutable global override.
func issuedQualificationApprovals() []qualificationApproval { return nil }
