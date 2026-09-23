package adr0011sourcecontext

import "testing"

// A withheld span must not be admitted as source just because its body is
// internally consistent with Omitted=false. This is the case that distinguishes
// the WITHHELD-specific guard from the general omission/body consistency check.
func TestWithheldSpanRejectsPresentBody(t *testing.T) {
	point, selected := fixture()
	withheld := selected
	withheld.Disposition = "WITHHELD"
	withheld.Omitted = false
	withheld.Body = "source must not be returned"

	if _, err := Build(Input{Points: []PointIdentity{point}, Spans: []SpanIdentity{withheld}}); err == nil {
		t.Fatal("ASSERT_WITHHELD_BODY_REJECTION: admitted WITHHELD source body")
	}
	t.Log("ASSERT_WITHHELD_BODY_REJECTION: PASS")
}

func TestWithheldSpanAcceptsOnlyOmittedEmptyBody(t *testing.T) {
	point, selected := fixture()
	withheld := selected
	withheld.Disposition = "WITHHELD"
	withheld.Omitted = true
	withheld.Body = ""

	got, err := Build(Input{Points: []PointIdentity{point}, Spans: []SpanIdentity{withheld}})
	if err != nil || len(got.Spans) != 1 || !got.Spans[0].Identity.Omitted || got.Counts.StoredBodyBytes != 0 {
		t.Fatalf("ASSERT_WITHHELD_EMPTY_OMISSION: result=%+v err=%v", got, err)
	}

	withheld.Body = "leaked"
	if _, err := Build(Input{Spans: []SpanIdentity{withheld}}); err == nil {
		t.Fatal("ASSERT_WITHHELD_OMITTED_BODY_REJECTION: admitted body on omitted span")
	}
	withheld.Body = ""
	withheld.Omitted = false
	if _, err := Build(Input{Spans: []SpanIdentity{withheld}}); err == nil {
		t.Fatal("ASSERT_WITHHELD_UNMARKED_OMISSION_REJECTION: admitted withheld span without omission")
	}
}

func TestAdversarialInterningAccountsForUTF8Bytes(t *testing.T) {
	point, span := fixture()
	span.Body = "café"
	got, err := Build(Input{Points: []PointIdentity{point}, Spans: []SpanIdentity{span, span}})
	if err != nil {
		t.Fatal(err)
	}
	if got.Counts.SpanCandidates != 2 || got.Counts.Spans != 1 || got.Counts.StoredBodyBytes != len(span.Body) {
		t.Fatalf("ASSERT_INTERNED_UTF8_BYTES: %+v", got.Counts)
	}
}

func TestAdversarialIdentityAndBindingCustody(t *testing.T) {
	point, span := fixture()
	changed := span
	changed.Document.Privacy = "restricted"
	if SpanID(changed) == SpanID(span) {
		t.Fatal("ASSERT_PRIVACY_SEPARATES_SPAN_ID")
	}
	changedPoint := point
	changedPoint.Document.DocumentVersion = "5"
	if PointID(changedPoint) == PointID(point) {
		t.Fatal("ASSERT_VERSION_SEPARATES_POINT_ID")
	}

	base := Binding{ID: "edge", Kind: Calls, PointID: PointID(point), SpanID: SpanID(span)}
	for _, tc := range []struct {
		name    string
		points  []PointIdentity
		spans   []SpanIdentity
		binding Binding
	}{
		{"missing point", nil, []SpanIdentity{span}, base},
		{"missing span", []PointIdentity{point}, nil, base},
		{"forged point", []PointIdentity{point}, []SpanIdentity{span}, Binding{ID: "edge", Kind: Calls, PointID: "point:forged", SpanID: SpanID(span)}},
		{"cross-privacy", []PointIdentity{point}, []SpanIdentity{changed}, Binding{ID: "edge", Kind: Calls, PointID: PointID(point), SpanID: SpanID(changed)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Build(Input{Points: tc.points, Spans: tc.spans, Bindings: []Binding{tc.binding}}); err == nil {
				t.Fatal("ASSERT_INVALID_BINDING_REJECTION: accepted absent, forged or cross-custody selection")
			}
		})
	}
}
