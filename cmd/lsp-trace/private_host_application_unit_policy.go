//go:build darwin || linux

package main

import (
	"errors"
	product "lsp-trace/internal/privatehostproductv3"
)

var errApplicationUnitSemanticPolicy = errors.New("UNIT_SEMANTIC_POLICY_PAUSE")

// Optional, caller-approved per-file semantic envelope. All maxima are prepaid;
// native target discovery never increases them. Nil preserves legacy bytes/limits.
type hostApplicationUnitEnvelope struct {
	Schema          string
	MaxCalls        int
	MaxRequestBytes int // per encoded frame; parent prepays MaxCalls times this
	MaxTotalBytes   int // cumulative request plus response wire
	MaxOutputBytes  int // includes the existing 32 KiB publication reserve
}

func applicationUnitEnvelope(r hostApplicationRequest) (hostApplicationUnitEnvelope, error) {
	p := hostApplicationUnitEnvelope{Schema: "private-application-unit-envelope.v1", MaxCalls: 2, MaxRequestBytes: 32 << 10, MaxTotalBytes: 80 << 10, MaxOutputBytes: 256 << 10}
	if r.UnitEnvelope != nil {
		p = *r.UnitEnvelope
		if p.Schema != "private-application-unit-envelope.v1" {
			return p, product.ErrInvalid
		}
	}
	// Validate before multiplication/conversion. No arbitrary maximum can overflow
	// an int or uint64, including on a 32-bit caller decoding hostile JSON.
	if p.MaxCalls < 2 || p.MaxCalls > 16 || p.MaxRequestBytes < 1024 || p.MaxRequestBytes > 1<<20 || p.MaxTotalBytes < 1 || p.MaxTotalBytes > 8<<20 || p.MaxOutputBytes <= 32<<10 || p.MaxOutputBytes > 4<<20 || r.UnitResponseBytes < 1024 || r.UnitResponseBytes > 8192 {
		return p, product.ErrInvalid
	}
	request := uint64(p.MaxCalls) * uint64(p.MaxRequestBytes)
	response := uint64(p.MaxCalls) * uint64(r.UnitResponseBytes)
	if request > 1<<20 || response > 64<<10 || uint64(p.MaxTotalBytes) < request+response {
		return p, product.ErrInvalid
	}
	return p, nil
}
