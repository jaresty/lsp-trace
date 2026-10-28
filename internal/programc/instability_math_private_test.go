package programc

import (
	"fmt"
	"testing"
)

func TestPrivateInstabilityMathScale(t *testing.T) {
	for _, n := range []int{1, 16, 100, 1000} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			nodes := make([]string, n)
			for i := range nodes {
				nodes[i] = fmt.Sprintf("n%04d", i)
			}
			one := []Community{{Members: nodes}}
			m, err := ComparePartitionsMathPrivate(nodes, one, one)
			if err != nil || m.Outcome != "STABLE" || m.NodeDenominator != n {
				t.Fatalf("%+v %v", m, err)
			}
			singles := make([]Community, n)
			for i, id := range nodes {
				singles[i].Members = []string{id}
			}
			m, err = ComparePartitionsMathPrivate(nodes, singles, singles)
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("nodes=%d communities=%d outcome=%s guard=%s work=%d", n, n, m.Outcome, m.Guard, m.Work)
			if n >= 100 && m.Outcome != "BLOCKED_MATCHING_RESOURCE" {
				t.Fatal(m)
			}
		})
	}
}
func TestPrivateInstabilityMathRejectAndUnstable(t *testing.T) {
	nodes := []string{"a", "b", "c", "d"}
	left := []Community{{Members: []string{"a", "b"}}, {Members: []string{"c", "d"}}}
	right := []Community{{Members: []string{"a", "c"}}, {Members: []string{"b", "d"}}}
	m, e := ComparePartitionsMathPrivate(nodes, left, right)
	if e != nil || m.Outcome != "UNSTABLE" {
		t.Fatal(m, e)
	}
	for _, bad := range [][]Community{nil, {{Members: nil}}, {{Members: []string{"a", "a", "c", "d"}}}, {{Members: []string{"a", "b", "c", "x"}}}} {
		if _, e := ComparePartitionsMathPrivate(nodes, left, bad); e == nil {
			t.Fatal("bad partition")
		}
	}
}
