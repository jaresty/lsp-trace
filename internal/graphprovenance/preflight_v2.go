package graphprovenance

import (
	"encoding/json"
	"errors"
)

// preflightGraphV2 counts known raw carriers before historical schema/semantic
// recursion. Node payloads, opaque data and source bytes are not interpreted.
func preflightGraphV2(raw []byte) error {
	if err := preflightV2(raw, MaxGraphBytesV2); err != nil {
		return err
	}
	var g struct {
		Nodes []json.RawMessage `json:"nodes"`
		Edges []struct {
			CallSites []json.RawMessage `json:"call_sites"`
		} `json:"edges"`
		Seeds       []json.RawMessage `json:"seeds"`
		Memberships []json.RawMessage `json:"seed_memberships"`
		Diagnostics []json.RawMessage `json:"diagnostics"`
		Frontier    []json.RawMessage `json:"frontier"`
		Terminals   []json.RawMessage `json:"terminals"`
		Locators    []json.RawMessage `json:"portable_locators"`
		Invocation  struct {
			Seeds []json.RawMessage `json:"seeds"`
		} `json:"invocation"`
	}
	if err := json.Unmarshal(raw, &g); err != nil {
		return err
	}
	if len(g.Nodes) > 10000 || len(g.Edges) > MaxBindings || len(g.Seeds) > 64 || len(g.Invocation.Seeds) > 64 {
		return errors.New("V2 raw graph row limit")
	}
	rows := len(g.Nodes) + len(g.Edges) + len(g.Seeds) + len(g.Memberships) + len(g.Diagnostics) + len(g.Frontier) + len(g.Terminals) + len(g.Locators)
	sites := 0
	for _, e := range g.Edges {
		sites += len(e.CallSites)
	}
	if sites > MaxBindings || rows+sites > 1000000 {
		return errors.New("V2 raw graph metadata limit")
	}
	return nil
}
