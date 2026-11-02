//go:build darwin || linux

package main

import (
	product "lsp-trace/internal/privatehostproductv3"
	"time"
)

// Compute before configuration approval, source access or child capability use.
// The returned four processing components are exactly those passed to the child.
func applicationChildAllowance(r hostApplicationRequest, source hostProductRequestV3) (product.ApplicationBounds, error) {
	child, boot := source.ApplicationAllowance, source.BootstrapAllowance
	sum := func(a, b uint64) (uint64, bool) {
		if b > ^uint64(0)-a {
			return 0, false
		}
		return a + b, true
	}
	logical, ok1 := sum(child.LogicalBytes, boot.LogicalBytes)
	read, ok2 := sum(child.ReadHashBytes, boot.ReadHashBytes)
	write, ok3 := sum(child.WriteBytes, boot.WriteBytes)
	work, ok4 := sum(child.Work, boot.Work)
	if !ok1 || !ok2 || !ok3 || !ok4 || r.UnitTimeoutMS < 1 || r.UnitTimeoutMS > 60000 || r.UnitResponseBytes < 1024 || r.UnitResponseBytes > 8192 || source.Policy.TotalBytes < 1 {
		return product.ApplicationBounds{}, product.ErrInvalid
	}
	return product.ApplicationBounds{LogicalBytes: logical, ReadHashBytes: read, WriteBytes: write, Work: work, Records: 4096, ParserBytes: 32 << 20, ProviderRequests: 128, ProviderMessages: 8192, ActiveNS: uint64(time.Duration(r.UnitTimeoutMS) * time.Millisecond), Calls: 2, RequestBytes: 64 << 10, ResponseBytes: uint64(2 * r.UnitResponseBytes), WireBytes: 80 << 10, OutputBytes: 256 << 10}, nil
}
