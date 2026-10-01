package adr0011genericv5proposal

import "testing"

func TestB4bSuccessorNullWithHeldClientSelector(t *testing.T) {
	input := B4bSuccessorSelectorInput{
		Correspondence:     b4aFixture(t),
		ClaimedSelector:    []byte("null"),
		HeldClientSelector: []byte(`[{"scheme":"file","language":"go"}]`),
	}
	if got := CheckB4bSuccessorSelector(input); got != "SUPPORTED" {
		t.Fatalf("null with held client selector: %s want SUPPORTED", got)
	}
}

func TestB4bSuccessorInvalidHeldSelector(t *testing.T) {
	input := B4bSuccessorSelectorInput{
		Correspondence:     b4aFixture(t),
		ClaimedSelector:    []byte("null"),
		HeldClientSelector: []byte(`[{"scheme":17}]`),
	}
	if got := CheckB4bSuccessorSelector(input); got != "MALFORMED" {
		t.Fatalf("invalid known held selector: %s want MALFORMED", got)
	}
}

func TestB4bSuccessorIndependentControl(t *testing.T) {
	input := B4bSuccessorSelectorInput{
		Correspondence:  b4aFixture(t),
		ClaimedSelector: []byte("null"),
	}
	if got := CheckB4bSuccessorSelector(input); got != "UNKNOWN" {
		t.Fatalf("unheld client selector: %s want UNKNOWN", got)
	}
}
