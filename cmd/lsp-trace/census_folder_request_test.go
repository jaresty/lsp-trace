//go:build darwin

package main

import (
	ai "lsp-trace/internal/applicationinventory"
	dw "lsp-trace/internal/describeworker"
	product "lsp-trace/internal/privatehostproductv3"
	"strconv"
	"testing"
	"time"
)

func TestCensusFolderAdmissionRequest(t *testing.T) {
	deadline := time.Now().Add(10 * time.Second)
	o := censusCLIOptions{Workspace: t.TempDir(), PublicationRoot: t.TempDir(), RequestTimeout: 30 * time.Second}
	pin := digestCatalog("caller-declaration")
	producer := digestCatalog("different measured producer")
	hello := censusCallerHello{Pins: dw.HostPinsV2{WorkerSHA256: pin, ModelSHA256: pin, GrammarSHA256: pin}}
	grant, err := product.InventoryLocalGrantV3(deadline, 2, applicationCoordinatorQuote(1, 16))
	if err != nil {
		t.Fatal(err)
	}
	q, err := censusFolderInventoryRequest(o, deadline, hello, nil, nil, grant, producer)
	if err != nil {
		t.Fatal("ASSERT_REQUEST_ADMISSION", err)
	}
	if q.Configuration.ProducerExecutableSHA256 != producer || q.Configuration.ProducerExecutableSHA256 == pin {
		t.Fatal("ASSERT_REQUEST_PRODUCER")
	}
	if q.Inventory.HostWindowMS <= 1000 || q.Inventory.HostWindowMS > 10000 || q.Configuration.Grant.ExpiresUnixNano != deadline.UnixNano() {
		t.Fatal("ASSERT_REQUEST_OWNER_DEADLINE", q.Inventory.HostWindowMS)
	}
	if grant.Ceiling.ActiveNS > uint64(180*time.Second) || grant.Ceiling.Calls != 2 || grant.Ceiling.ProviderRequests != 0 || grant.Ceiling.ProviderMessages != 0 {
		t.Fatal("ASSERT_REQUEST_DERIVED_BOUND", grant)
	}
	// Even a one-byte short allocation cannot silently use the larger quote.
	reduced := grant
	reduced.Ceiling.ReadHashBytes--
	if _, err := censusFolderInventoryRequest(o, deadline, hello, nil, nil, reduced, producer); err == nil {
		t.Fatal("ASSERT_REQUEST_INSUFFICIENT")
	}
	wrong := grant
	wrong.ExpiresUnixNano++
	if _, err := censusFolderInventoryRequest(o, deadline, hello, nil, nil, wrong, producer); err == nil {
		t.Fatal("ASSERT_REQUEST_DEADLINE_BINDING")
	}
	if _, err := censusFolderInventoryRequest(o, time.Now().Add(-time.Second), hello, nil, nil, grant, producer); err == nil {
		t.Fatal("ASSERT_REQUEST_EXPIRED_NO_RENEWAL")
	}
	if _, err := product.InventoryLocalGrantV3(deadline, 17, applicationCoordinatorQuote(1, 16)); err == nil {
		t.Fatal("ASSERT_REQUEST_CALL_BOUND")
	}
	var leads []ai.Lead
	for i := 0; i < 14; i++ {
		leads = append(leads, ai.Lead{ID: strconv.Itoa(i), Provenance: "controlled bound"})
	}
	large, err := product.InventoryLocalGrantV3(deadline, 16, applicationCoordinatorQuote(1, 16))
	if err != nil {
		t.Fatal(err)
	}
	bounded, err := censusFolderInventoryRequest(o, deadline, hello, nil, leads, large, producer)
	if err != nil || bounded.Inventory.HostWindowMS*16 > 120000 || bounded.Inventory.BudgetBytes > large.Ceiling.LogicalBytes {
		t.Fatal("ASSERT_REQUEST_CUMULATIVE_BOUND", bounded.Inventory.HostWindowMS, err)
	}
	t.Log("ASSERT_REQUEST_ADMISSION: PASS exact fresh grant, producer, deadline and short-budget refusal")
}
