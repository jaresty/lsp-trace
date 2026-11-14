package main

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestCensusDiscoveryPrivateBudgetCanonicalAndQuote(t *testing.T) {
	value := struct {
		Text string
		Rows []string
	}{Text: "<bounded>\n", Rows: []string{}}
	quoted, e := censusInventoryMarshal(value, 4096)
	raw, _ := json.Marshal(value)
	if e != nil || !bytes.Equal(quoted, raw) {
		t.Fatal("ASSERT_CONNECTED_QUOTE_BEFORE_MARSHAL", e)
	}
	if _, e = censusInventoryMarshal(map[string]string{"unbounded": "map"}, 4096); e == nil {
		t.Fatal("ASSERT_CONNECTED_MAP_QUOTE_REFUSED")
	}
	if _, e = censusInventoryMarshal(value, 1); e == nil {
		t.Fatal("ASSERT_CONNECTED_SMALL_QUOTE_REFUSED")
	}
	var parsed struct {
		Text string
		Rows []string
	}
	if e = censusInventoryDecode(raw, &parsed, 4096); e != nil || parsed.Rows == nil {
		t.Fatal("ASSERT_CONNECTED_EMPTY_REPRESENTATION_PRESERVED", e)
	}
	for _, bad := range [][]byte{append(bytes.Clone(raw), ' '), []byte(`{"Text":"x","Rows":[],"unknown":1}`), []byte(`{"Text":"x","Text":"y","Rows":[]}`)} {
		if censusInventoryDecode(bad, &parsed, 4096) == nil {
			t.Fatal("ASSERT_CONNECTED_NONCANONICAL_REFUSED")
		}
	}
	if !censusPrivateDiscoveryLimits().Valid() {
		t.Fatal("ASSERT_CONNECTED_CLOSED_LIMITS_VALID")
	}
	t.Log("ASSERT_CONNECTED_QUOTE_CANONICAL PASS")
}
