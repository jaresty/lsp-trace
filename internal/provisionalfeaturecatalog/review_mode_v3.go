package provisionalfeaturecatalog

// ReviewModeV3 records a caller-declared execution arrangement, not proof of
// context isolation or authenticated reviewer identity. The zero value retains
// the legacy distinct-actor requirement and its canonical wire representation.
type ReviewModeV3 string

const (
	ReviewModeSameThreadV3 ReviewModeV3 = "SAME_THREAD"
	ReviewModeSubagentsV3  ReviewModeV3 = "SUBAGENTS"
)

func (m ReviewModeV3) Valid() bool {
	return m == "" || m == ReviewModeSameThreadV3 || m == ReviewModeSubagentsV3
}

func (m ReviewModeV3) AllowsSameActor() bool {
	return m == ReviewModeSameThreadV3
}
