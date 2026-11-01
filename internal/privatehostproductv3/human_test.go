//go:build darwin || linux

package privatehostproductv3

import (
	"bytes"
	"strings"
	"testing"
)

func TestHostProductHumanLeadViewNotAuditWall(t *testing.T) {
	reference := "sha256:" + strings.Repeat("a", 64)
	r := Report{Status: "PROVISIONAL", Requested: 1, Completed: 1, Candidates: []Candidate{{ID: "candidate-1", WorkingLabel: "<script>ignored</script>", Explanation: "Bounded technical lead", Status: "PROVISIONAL", EvidenceRefs: []string{reference}, Alternatives: []string{"Not a product capability"}, Revision: 1}}}
	text := human(r)
	if bytes.Contains(text, []byte(reference)) || !bytes.Contains(text, []byte("1 typed refs; see machine row")) {
		t.Fatal("ASSERT_HUMAN_CAPABILITY_BEFORE_AUDIT_IDS")
	}
	if bytes.Contains(text, []byte("<script>")) || !bytes.Contains(text, []byte("&lt;script&gt;")) {
		t.Fatal("ASSERT_UNTRUSTED_HUMAN_LABEL_ESCAPED")
	}
}
