//go:build darwin || linux

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	pilot "lsp-trace/internal/censuslineagepilot"
	product "lsp-trace/internal/privatehostproductv3"
	"path/filepath"
)

func validateHostProductTransferV3(r hostProductRequestV3, t product.Transfer) error {
	o, e := parseCensusCLIOptions(r.CensusArguments)
	if e != nil {
		return product.ErrInvalid
	}
	if t.Spec.Root != o.Workspace || t.Spec.RevisionAssertion != r.RevisionAssertion {
		return product.ErrInvalid
	}
	actual, e := json.Marshal(t.Spec.EffectivePolicy)
	if e != nil {
		return product.ErrInvalid
	}
	expected, _ := json.Marshal(r.Policy)
	if !bytes.Equal(actual, expected) {
		return product.ErrInvalid
	}
	if product.ValidateTransfer(r.Configuration, t) != nil {
		return product.ErrInvalid
	}
	var m pilot.RetainedPageCheckpointManifestPrivateV3
	if json.Unmarshal(t.Manifest, &m) != nil {
		return product.ErrInvalid
	}
	if t.LedgerPath != filepath.Join(r.AuxiliaryRoot, "ledger") || m.LedgerRoot.Path != t.LedgerPath {
		return product.ErrInvalid
	}
	for i, pin := range m.Roots {
		if pin.Path != filepath.Join(r.AuxiliaryRoot, fmt.Sprintf("raw-%d", i)) {
			return product.ErrInvalid
		}
	}
	return nil
}
