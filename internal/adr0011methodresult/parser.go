// Package adr0011methodresult retains the transport outcome gate and the
// source-compatible names of the extracted raw grammar. Aliases intentionally
// relocate named-type PkgPath/debug identity to methodsyntax; wire fields,
// grammar and admission/materialization ordering remain the same. No reflected
// type identity is used as authority.
package adr0011methodresult

import (
	"lsp-trace/internal/adr0011methodsyntax"
	transport "lsp-trace/internal/adr0011methodtransport"
	"lsp-trace/internal/objectadmission"
)

type Code = methodsyntax.Code
type Failure = methodsyntax.Failure
type Position = methodsyntax.Position
type Range = methodsyntax.Range
type Kind = methodsyntax.Kind
type Item = methodsyntax.Item
type Result = methodsyntax.Result

const (
	FailureTransport = methodsyntax.FailureTransport
	FailureMethod    = methodsyntax.FailureMethod
	FailureLimit     = methodsyntax.FailureLimit
	FailureMalformed = methodsyntax.FailureMalformed
	FailureResource  = methodsyntax.FailureResource
	Location         = methodsyntax.Location
	LocationLink     = methodsyntax.LocationLink
)

func Parse(wire transport.Result, maxCandidates int) (Result, *Failure) {
	if wire.Outcome() != transport.OutcomeTransportSuccess {
		return Result{}, &Failure{Code: FailureTransport, Ordinal: -1}
	}
	return parseRawUntrusted(wire.Method(), wire.Raw(), maxCandidates)
}
func ParseRawReferences(raw []byte, maxCandidates int) (Result, *Failure) {
	return parseRawUntrusted(transport.MethodReferences, raw, maxCandidates)
}
func parseRawUntrusted(method string, raw []byte, maxCandidates int) (Result, *Failure) {
	return methodsyntax.ParseRaw(method, raw, maxCandidates)
}
func parseRawUntrustedWithAdmission(method string, raw []byte, maxCandidates int, admitter objectadmission.ObjectAdmitter) (Result, *Failure) {
	return methodsyntax.ParseRawWithAdmission(method, raw, maxCandidates, admitter)
}
func parseItem(raw []byte, ordinal int) (Item, bool) { return methodsyntax.ParseItem(raw, ordinal) }
