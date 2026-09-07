package boundedanalysis

import (
	"context"
	"encoding/json"
	"sort"
	"testing"
)

func TestCompleteButWrongPartitionsCoherentlyResealed(t *testing.T) {
	raw, _ := retainedFixture(t)
	for _, mode := range []string{"WEAK", "STRONG"} {
		for _, mutation := range []string{"merge", "split"} {
			t.Run(mode+"/"+mutation, func(t *testing.T) {
				e := evidence(t, raw, Parameters{Operation: "COMPONENTS", Mode: mode})
				if mutation == "merge" {
					e.Components = []Component{{Members: append([]string{}, e.Nodes...)}}
				} else {
					for i, c := range e.Components {
						if len(c.Members) > 1 {
							e.Components[i].Members = append([]string{}, c.Members[:1]...)
							e.Components = append(e.Components, Component{Members: append([]string{}, c.Members[1:]...)})
							break
						}
					}
				}
				for i := range e.Components {
					sort.Strings(e.Components[i].Members)
					e.Components[i].ID = hash(Version+":component", canonical([]any{e.BasisDigest, e.Parameters.Mode, e.Components[i].Members}))
				}
				sort.Slice(e.Components, func(i, j int) bool { return e.Components[i].Members[0] < e.Components[j].Members[0] })
				e.Digest = seal(e)
				encoded, _ := json.Marshal(e)
				if _, err := ValidateFor(encoded, Family, "v1"); err == nil {
					t.Fatal("ASSERT_COMPLETE_RESEALED_PARTITION_REJECT", mode, mutation)
				}
			})
		}
	}
}
func TestIndependentProofRejectsEqualLengthWrongTie(t *testing.T) {
	e := topology([]string{"a", "b", "c", "d"}, []Edge{
		{GroupID: "ab", Caller: "a", Callee: "b", OccurrenceIDs: []string{}},
		{GroupID: "ac", Caller: "a", Callee: "c", OccurrenceIDs: []string{}},
		{GroupID: "bd", Caller: "b", Callee: "d", OccurrenceIDs: []string{}},
		{GroupID: "cd", Caller: "c", Callee: "d", OccurrenceIDs: []string{}},
	}, Parameters{Operation: "PATH", Start: "a", End: "d"})
	run(context.Background(), &e)
	e.Path = Path{[]string{"a", "c", "d"}, []string{"ac", "cd"}, [][]string{{}, {}}}
	if err := prove(e); err == nil {
		t.Fatal("ASSERT_EQUAL_LENGTH_WRONG_LEXICAL_TIE_REJECT")
	}
}
