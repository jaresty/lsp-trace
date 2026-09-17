package retainedprojection

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"reflect"
	"testing"
)

func TestCustodyBindingExactPreimagesAndGraphMapping(t *testing.T) {
	a := fixture()
	a.raw = []byte("exact admitted v2 source snapshot bytes")
	a.parent.GraphV5Bytes = []byte("exact embedded graph v5 bytes")
	graphSum := sha256.Sum256(a.parent.GraphV5Bytes)
	a.parent.GraphV5Digest = "sha256:" + hex.EncodeToString(graphSum[:])
	plan, err := Select(a, Request{Target: Key{"target", "file:///target.go"}, Selections: []Key{{"a", "file:///a.go"}, {"target", "file:///target.go"}}})
	if err != nil {
		t.Fatal(err)
	}
	got, err := a.CustodyBinding(plan)
	if err != nil {
		t.Fatal(err)
	}
	planBytes, _ := plan.Bytes()
	captureSum, manifestSum := sha256.Sum256(a.raw), sha256.Sum256(planBytes)
	if got.Custody != RetainedCustody || got.ResolverKind != ImmutableSourceObjectIdentityV1 ||
		got.CaptureID != "sha256:"+hex.EncodeToString(captureSum[:]) || got.ManifestID != "sha256:"+hex.EncodeToString(manifestSum[:]) ||
		got.GraphDigest != a.parent.GraphV5Digest || got.GraphByteLength != uint64(len(a.parent.GraphV5Bytes)) {
		t.Fatalf("ASSERT_RETAINED_CUSTODY_EXACT_IDENTITIES: %+v", got)
	}
	again, err := a.CustodyBinding(plan)
	if err != nil || !reflect.DeepEqual(got, again) {
		t.Fatalf("ASSERT_RETAINED_CUSTODY_DETERMINISTIC: %+v %v", again, err)
	}
	a2 := a
	a2.raw = append([]byte(" "), a.raw...)
	other, err := a2.CustodyBinding(plan)
	if err == nil || other != (RetainedCustodyBinding{}) {
		t.Fatalf("ASSERT_RETAINED_CUSTODY_RAW_V2_SUBSTITUTION_REJECTED: %+v %v", other, err)
	}
}

func TestCustodyBindingRejectsEquivalentPlanFromDifferentAdmittedCapture(t *testing.T) {
	rawA := validCustodyArtifact(t)
	var value any
	if err := json.Unmarshal(rawA, &value); err != nil {
		t.Fatal(err)
	}
	rawB, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	admittedA, err := Admit(rawA)
	if err != nil {
		t.Fatal(err)
	}
	admittedB, err := Admit(rawB)
	if err != nil {
		t.Fatal(err)
	}
	binding := admittedA.artifact.DisplayBindings[0]
	request := Request{Target: Key{binding.GraphSubjectID, binding.LogicalSourceID}, Selections: []Key{{binding.GraphSubjectID, binding.LogicalSourceID}}}
	planA, err := Select(admittedA, request)
	if err != nil {
		t.Fatal(err)
	}
	planB, err := Select(admittedB, request)
	if err != nil {
		t.Fatal(err)
	}
	bytesA, _ := planA.Bytes()
	bytesB, _ := planB.Bytes()
	if !bytes.Equal(bytesA, bytesB) {
		t.Fatal("ASSERT_RETAINED_CUSTODY_EQUIVALENT_VISIBLE_PLANS")
	}
	if got, err := admittedB.CustodyBinding(planA); err == nil || got != (RetainedCustodyBinding{}) {
		t.Fatalf("ASSERT_RETAINED_CUSTODY_REJECTS_CROSS_ARTIFACT_PLAN: %+v %v", got, err)
	}
	if _, err := admittedB.CustodyBinding(planB); err != nil {
		t.Fatalf("ASSERT_RETAINED_CUSTODY_ACCEPTS_OWN_PLAN: %v", err)
	}
}

func TestAdmitCopiesRawBytesForCaptureAndSeal(t *testing.T) {
	raw := validCustodyArtifact(t)
	original := append([]byte(nil), raw...)
	admitted, err := Admit(raw)
	if err != nil {
		t.Fatal(err)
	}
	binding := admitted.artifact.DisplayBindings[0]
	request := Request{Target: Key{binding.GraphSubjectID, binding.LogicalSourceID}, Selections: []Key{{binding.GraphSubjectID, binding.LogicalSourceID}}}
	plan, err := Select(admitted, request)
	if err != nil {
		t.Fatal(err)
	}
	raw[0] ^= 0xff
	got, err := admitted.CustodyBinding(plan)
	if err != nil {
		t.Fatal(err)
	}
	if got.CaptureID != custodyDigest(original) {
		t.Fatalf("ASSERT_RETAINED_CUSTODY_ADMIT_RAW_COPY: %s", got.CaptureID)
	}
}

func TestCustodyBindingRejectsForgedMutatedAndZeroInputs(t *testing.T) {
	a := fixture()
	a.raw = []byte("raw")
	a.parent.GraphV5Bytes = []byte("graph")
	sum := sha256.Sum256(a.parent.GraphV5Bytes)
	a.parent.GraphV5Digest = "sha256:" + hex.EncodeToString(sum[:])
	plan, err := Select(a, Request{Target: Key{"target", "file:///target.go"}, Selections: []Key{{"target", "file:///target.go"}, {"a", "file:///a.go"}}})
	if err != nil {
		t.Fatal(err)
	}
	for name, candidate := range map[string]Plan{
		"forged": {Ordering: plan.Ordering, Target: plan.Target, Selections: append([]Selection(nil), plan.Selections...)},
		"mutated": func() Plan {
			p := plan
			p.Selections = append([]Selection(nil), p.Selections...)
			p.Selections[0].Role = "ADDITIONAL"
			return p
		}(),
		"reordered": func() Plan {
			p := plan
			p.Selections = append([]Selection(nil), p.Selections...)
			p.Selections[0], p.Selections[1] = p.Selections[1], p.Selections[0]
			return p
		}(),
	} {
		if got, err := a.CustodyBinding(candidate); err == nil || !reflect.DeepEqual(got, RetainedCustodyBinding{}) {
			t.Fatalf("ASSERT_RETAINED_CUSTODY_REJECTS_%s: %+v %v", name, got, err)
		}
	}
	if got, err := (Admitted{}).CustodyBinding(plan); err == nil || got != (RetainedCustodyBinding{}) {
		t.Fatalf("ASSERT_RETAINED_CUSTODY_ZERO_ADMITTED: %+v %v", got, err)
	}
	if got, err := a.CustodyBinding(Plan{}); err == nil || got != (RetainedCustodyBinding{}) {
		t.Fatalf("ASSERT_RETAINED_CUSTODY_ZERO_PLAN: %+v %v", got, err)
	}
}
