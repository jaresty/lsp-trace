package censuslineagepilot

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"lsp-trace/internal/acquisition"
	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/sessionruntime"
)

// Only DeriveProducerPacketPrivateV2 constructs this state, after strict whole
// origin validation and actual qualified-pair/producer-lineage checks. No DTO,
// digest cache, claimed history, or decoded report can construct this proof.
type validatedProducerSnapshotPrivateV2 struct {
	self        *validatedProducerSnapshotPrivateV2
	owner       *derivedProducerPacketStatePrivateV2
	qualified   *qualifiedProducerPacketStatePrivateV2
	packet      ProducerPacketInspectionPrivateV2
	sources     []graphprovenance.SourceWithholdingPrivateV2
	discovery   censusacquisition.Discovery
	batches     []censusacquisition.BatchRequest
	enumeration []byte
	runtime     sessionruntime.SessionMetadata
	report      ProducerPacketQualificationReportPrivateV2
}

func newValidatedProducerSnapshotPrivateV2(owner *derivedProducerPacketStatePrivateV2, p ProducerPacketInspectionPrivateV2) (*validatedProducerSnapshotPrivateV2, error) {
	q := owner.qualified.state
	s := &validatedProducerSnapshotPrivateV2{owner: owner, qualified: q, packet: p}
	var origin struct {
		Enumeration json.RawMessage                  `json:"actual_enumeration"`
		Batches     []censusacquisition.BatchRequest `json:"target_batches"`
	}
	if err := json.Unmarshal(p.EnumerationOrigin, &origin); err != nil {
		return nil, fmt.Errorf("snapshot origin: %w", err)
	}
	if err := json.Unmarshal(origin.Enumeration, &s.discovery); err != nil {
		return nil, fmt.Errorf("snapshot discovery: %w", err)
	}
	if err := json.Unmarshal(p.RuntimeMetadata, &s.runtime); err != nil {
		return nil, fmt.Errorf("snapshot runtime: %w", err)
	}
	if err := json.Unmarshal(q.report, &s.report); err != nil {
		return nil, fmt.Errorf("snapshot report: %w", err)
	}
	if s.report.PacketSHA256 != q.digest || s.report.PacketByteLength != uint64(len(q.raw)) || s.report.BatchCount != len(p.Batches) || s.report.Status != "PRIVATE_PACKET_QUALIFIED_ONLY" || s.report.Authority != 0 || s.report.Accepted || s.report.Feature != "UNRESOLVED" || s.report.Completeness != "UNKNOWN" || s.report.Absence != "PROHIBITED" || len(origin.Batches) != len(p.Batches) {
		return nil, ErrInvalid
	}
	s.batches, s.enumeration = origin.Batches, origin.Enumeration
	s.self = s
	return s, nil
}

func (s *validatedProducerSnapshotPrivateV2) valid() bool {
	return s != nil && s.self == s && s.owner != nil && s.owner.self == s.owner && s.owner.snapshot == s && s.owner.qualified.state == s.qualified && s.owner.qualified.valid() && len(s.sources) == len(s.packet.Batches) && len(s.sources) == len(s.owner.batches)
}

// ValidatedBatchPrivateV2 is a constructor-owned immutable selected snapshot.
// Its detached views are inspection metadata, never alternate admission inputs.
type ValidatedBatchPrivateV2 struct{ state *validatedBatchStatePrivateV2 }
type validatedBatchStatePrivateV2 struct {
	self     *validatedBatchStatePrivateV2
	snapshot *validatedProducerSnapshotPrivateV2
	ordinal  int
}

func (ValidatedBatchPrivateV2) MarshalJSON() ([]byte, error) { return nil, ErrInvalid }

// SelectedBatchViewPrivateV2 contains only the selected decoded source and shared
// metadata. Packet and source bytes remain exact originals, not re-encodings.
type SelectedBatchViewPrivateV2 struct {
	Packet, QualificationReport                     []byte
	PacketSHA256, ReportSHA256                      string
	Batch                                           ProducerPacketBatchEvidencePrivateV2
	Source                                          graphprovenance.SourceWithholdingPrivateV2
	Discovery                                       censusacquisition.Discovery
	BatchDiscovery                                  []censusacquisition.PreparedTarget
	Enumeration, EnumerationOrigin, RuntimeMetadata []byte
	Runtime                                         sessionruntime.SessionMetadata
	Files                                           []ProducerPacketFilePrivateV2
}

// ReadSelectedDerivedProducerPacketPrivateV2 preserves fresh borrowed-root,
// source identity/topology, cancellation and all original producer-pointer checks
// before and after exposing data. It does not revalidate the whole origin.
func ReadSelectedDerivedProducerPacketPrivateV2(ctx context.Context, d DerivedProducerPacketPrivateV2, ordinal int) (ValidatedBatchPrivateV2, error) {
	return readSelectedDerivedProducerPacketPrivateV2(ctx, d, ordinal, nil)
}

// The per-call budget is also package-private instrumentation: no mutable global
// counters or production callbacks are needed to observe validation work.
func readSelectedDerivedProducerPacketPrivateV2(ctx context.Context, d DerivedProducerPacketPrivateV2, ordinal int, observed *producerPacketValidationCountsPrivateV2) (ValidatedBatchPrivateV2, error) {
	if ctx == nil || ctx.Err() != nil || d.state == nil || d.state.self != d.state || !d.state.snapshot.valid() || d.state.snapshot.owner != d.state || ordinal < 0 || ordinal >= len(d.state.batches) {
		return ValidatedBatchPrivateV2{}, ErrInvalid
	}
	s := d.state.snapshot
	q, pair := d.state.qualified, d.state.qualified.state.pair
	b, _ := newProducerPacketBudgetPrivateV2(nil)
	if err := b.reserve(0, 0, producerPacketTopologyWorkPrivateV2); err != nil {
		return ValidatedBatchPrivateV2{}, err
	}
	if err := pair.state.roots.check(ctx, pair.state.frozen); err != nil {
		return ValidatedBatchPrivateV2{}, err
	}
	if err := checkQualifiedPacketBindingPrivateV2(q, b); err != nil {
		return ValidatedBatchPrivateV2{}, err
	}
	// Account for comparisons against every original producer, without parsing.
	if err := b.passes(uint64(len(q.state.raw)), 0, 2, uint64(32+len(s.packet.Batches))); err != nil {
		return ValidatedBatchPrivateV2{}, err
	}
	if !packetMatchesProducerLineagePrivateV2(s.packet, pair) {
		return ValidatedBatchPrivateV2{}, ErrInvalid
	}
	if err := pair.state.roots.check(ctx, pair.state.frozen); err != nil {
		return ValidatedBatchPrivateV2{}, err
	}
	if err := ctx.Err(); err != nil {
		return ValidatedBatchPrivateV2{}, err
	}
	if observed != nil {
		*observed = b.validation
	}
	st := &validatedBatchStatePrivateV2{snapshot: s, ordinal: ordinal}
	st.self = st
	return ValidatedBatchPrivateV2{st}, nil
}

// ViewPrivateV2 clones already validated data; it performs no acquisition or IO.
// Retaining this opaque result in a sealed projection does not retain executable
// source authority. New producer reads must still use the fresh-root API above.
func (v ValidatedBatchPrivateV2) ViewPrivateV2() (SelectedBatchViewPrivateV2, error) {
	if v.state == nil || v.state.self != v.state || !v.state.snapshot.valid() {
		return SelectedBatchViewPrivateV2{}, ErrInvalid
	}
	s, i := v.state.snapshot, v.state.ordinal
	if i < 0 || i >= len(s.sources) {
		return SelectedBatchViewPrivateV2{}, ErrInvalid
	}
	q := s.qualified
	b, _ := newProducerPacketBudgetPrivateV2(nil)
	if err := checkQualifiedPacketBindingPrivateV2(s.owner.qualified, b); err != nil {
		return SelectedBatchViewPrivateV2{}, err
	}
	if !packetMatchesProducerLineagePrivateV2(s.packet, q.pair) {
		return SelectedBatchViewPrivateV2{}, ErrInvalid
	}
	// Read/clone passes only; the constructor already paid all strict validation.
	for _, raw := range [][]byte{q.raw, q.report, s.packet.EnumerationOrigin, s.packet.RuntimeMetadata, s.enumeration} {
		if err := b.passes(uint64(len(raw)), 1, 1, 1); err != nil {
			return SelectedBatchViewPrivateV2{}, err
		}
	}
	batch := s.owner.batches[i]
	for _, raw := range [][]byte{batch.CanonicalSeeds, batch.CaptureOrigin, batch.SourceArtifact, batch.NativeGraph} {
		if err := b.passes(uint64(len(raw)), 1, 1, 1); err != nil {
			return SelectedBatchViewPrivateV2{}, err
		}
	}
	// Bounded typed clone allowance, not another strict artifact validation.
	if err := b.passes(uint64(len(batch.SourceArtifact)), 3, 1, 4096); err != nil {
		return SelectedBatchViewPrivateV2{}, err
	}
	for _, file := range s.packet.Files {
		if err := b.passes(uint64(len(file.Body)), 1, 1, 1); err != nil {
			return SelectedBatchViewPrivateV2{}, err
		}
	}
	out := SelectedBatchViewPrivateV2{Packet: q.raw, QualificationReport: q.report, PacketSHA256: q.digest, ReportSHA256: q.reportDigest, Batch: batch, Source: s.sources[i], Discovery: s.discovery, BatchDiscovery: s.batches[i].Targets, Enumeration: s.enumeration, EnumerationOrigin: s.packet.EnumerationOrigin, RuntimeMetadata: s.packet.RuntimeMetadata, Runtime: s.runtime, Files: s.packet.Files}
	cloned, err := cloneValidatedMetadataPrivateV2(reflect.ValueOf(out))
	if err != nil {
		return SelectedBatchViewPrivateV2{}, err
	}
	return cloned.Interface().(SelectedBatchViewPrivateV2), nil
}

// Clone plain validated metadata without serialization, canonicalization, or
// changing nil/empty slices, ordered duplicates, RawMessage lexemes or typed graph
// fields. Request's only private field is immutable restrict-only query metadata;
// it is copied as a value, never traversed or reconstructed as a capability.
func cloneValidatedMetadataPrivateV2(v reflect.Value) (reflect.Value, error) {
	switch v.Kind() {
	case reflect.Struct:
		out := reflect.New(v.Type()).Elem()
		out.Set(v)
		_, request := v.Interface().(acquisition.Request)
		for i := 0; i < v.NumField(); i++ {
			if v.Type().Field(i).PkgPath != "" {
				if !request || v.Type().Field(i).Name != "frozenQueryDeclarationPrivateV2" {
					return reflect.Value{}, ErrInvalid
				}
				continue
			}
			c, err := cloneValidatedMetadataPrivateV2(v.Field(i))
			if err != nil {
				return reflect.Value{}, err
			}
			out.Field(i).Set(c)
		}
		return out, nil
	case reflect.Slice:
		if v.IsNil() {
			return reflect.Zero(v.Type()), nil
		}
		out := reflect.MakeSlice(v.Type(), v.Len(), v.Len())
		if v.Type().Elem().Kind() == reflect.Uint8 {
			reflect.Copy(out, v)
			return out, nil
		}
		for i := 0; i < v.Len(); i++ {
			c, err := cloneValidatedMetadataPrivateV2(v.Index(i))
			if err != nil {
				return reflect.Value{}, err
			}
			out.Index(i).Set(c)
		}
		return out, nil
	case reflect.Array:
		out := reflect.New(v.Type()).Elem()
		for i := 0; i < v.Len(); i++ {
			c, err := cloneValidatedMetadataPrivateV2(v.Index(i))
			if err != nil {
				return reflect.Value{}, err
			}
			out.Index(i).Set(c)
		}
		return out, nil
	case reflect.Pointer:
		if v.IsNil() {
			return reflect.Zero(v.Type()), nil
		}
		c, err := cloneValidatedMetadataPrivateV2(v.Elem())
		if err != nil {
			return reflect.Value{}, err
		}
		out := reflect.New(v.Type().Elem())
		out.Elem().Set(c)
		return out, nil
	case reflect.Map:
		if v.IsNil() {
			return reflect.Zero(v.Type()), nil
		}
		out := reflect.MakeMapWithSize(v.Type(), v.Len())
		it := v.MapRange()
		for it.Next() {
			k, err := cloneValidatedMetadataPrivateV2(it.Key())
			if err != nil {
				return reflect.Value{}, err
			}
			c, err := cloneValidatedMetadataPrivateV2(it.Value())
			if err != nil {
				return reflect.Value{}, err
			}
			out.SetMapIndex(k, c)
		}
		return out, nil
	case reflect.Interface:
		if v.IsNil() {
			return reflect.Zero(v.Type()), nil
		}
		c, err := cloneValidatedMetadataPrivateV2(v.Elem())
		if err != nil {
			return reflect.Value{}, err
		}
		out := reflect.New(v.Type()).Elem()
		out.Set(c)
		return out, nil
	case reflect.Func, reflect.Chan, reflect.UnsafePointer:
		return reflect.Value{}, ErrInvalid
	default:
		return v, nil
	}
}

// ReadPrivateV2 stages detached data between current root/context checks. The
// no-IO ViewPrivateV2 accessor is reserved for already sealed retained consumers.
func (v ValidatedBatchPrivateV2) ReadPrivateV2(ctx context.Context) (SelectedBatchViewPrivateV2, error) {
	if ctx == nil || ctx.Err() != nil || v.state == nil || v.state.self != v.state || !v.state.snapshot.valid() {
		return SelectedBatchViewPrivateV2{}, ErrInvalid
	}
	s := v.state.snapshot
	pair := s.qualified.pair
	if err := pair.state.roots.check(ctx, pair.state.frozen); err != nil {
		return SelectedBatchViewPrivateV2{}, err
	}
	out, err := v.ViewPrivateV2()
	if err != nil {
		return SelectedBatchViewPrivateV2{}, err
	}
	if err := pair.state.roots.check(ctx, pair.state.frozen); err != nil {
		return SelectedBatchViewPrivateV2{}, err
	}
	if err := ctx.Err(); err != nil {
		return SelectedBatchViewPrivateV2{}, err
	}
	return out, nil
}
