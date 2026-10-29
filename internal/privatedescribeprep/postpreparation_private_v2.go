package privatedescribeprep

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	pilot "lsp-trace/internal/censuslineagepilot"
	"lsp-trace/internal/publication"
)

// PostPreparationPrivateV2 has no DTO/byte/ledger constructor or public fields.
// Shared accounting state is immutable; the version is not caller-selectable.
type PostPreparationPrivateV2 struct {
	state   *postState
	roots   []producerRootBindingPrivateV2
	objects []producerObjectBindingPrivateV2
}
type producerRootBindingPrivateV2 struct {
	Path          string
	Device, Inode uint64
}
type producerObjectBindingPrivateV2 struct {
	Root     producerRootBindingPrivateV2
	Selector string
}

type StageLedgerPrivateV2 struct {
	RequestType string
	StageLedger
}

func NewPostPreparationPrivateV2(ctx context.Context, inputs []InputPrivateV2, prepLimits Limits, postLimits PostPreparationLimits) (PostPreparationPrivateV2, error) {
	if err := postContext(ctx); err != nil {
		return PostPreparationPrivateV2{}, err
	}
	if !postLimits.valid() || len(inputs) == 0 || len(inputs) > postLimits.MaxConstituents {
		return PostPreparationPrivateV2{}, fmt.Errorf("private V2 post invocation limits")
	}
	r, err := PreparePrivateV2(ctx, inputs, prepLimits)
	if err != nil {
		return PostPreparationPrivateV2{}, err
	}
	// Only package-owned V2 preparation reaches this version-specific validator.
	common, err := postFromPreparedProfile(ctx, r, postLimits, RequestTypePrivateV2)
	if err != nil {
		return PostPreparationPrivateV2{}, err
	}
	h := PostPreparationPrivateV2{state: common.state}
	seen := map[producerRootBindingPrivateV2]bool{}
	seenObjects := map[producerObjectBindingPrivateV2]bool{}
	seenPairs := map[struct{ packet, qualification string }]bool{}
	for _, in := range inputs {
		e, err := in.Projection2.PreparationEvidencePrivateV2()
		if err != nil {
			return PostPreparationPrivateV2{}, err
		}
		seenPairs[struct{ packet, qualification string }{e.Source.PacketSHA256, e.Source.QualificationReportSHA256}] = true
		var report pilot.ProducerPacketQualificationReportPrivateV2
		if json.Unmarshal(e.QualificationReport, &report) != nil || pilot.Digest(e.QualificationReport) != e.Source.QualificationReportSHA256 || report.PacketSHA256 != e.Source.PacketSHA256 {
			return PostPreparationPrivateV2{}, fmt.Errorf("private V2 protected root report binding")
		}
		for _, o := range report.CopyObservations {
			pin := producerRootBindingPrivateV2{o.RootPath, o.RootDevice, o.RootInode}
			if pin.Path == "" || pin.Inode == 0 {
				return PostPreparationPrivateV2{}, fmt.Errorf("private V2 missing sealed producer root identity")
			}
			if !seen[pin] {
				seen[pin] = true
				h.roots = append(h.roots, pin)
			}
			object := producerObjectBindingPrivateV2{pin, e.Source.PacketSHA256}
			if !seenObjects[object] {
				seenObjects[object] = true
				h.objects = append(h.objects, object)
			}
		}
	}
	if len(h.roots) < 2 || len(h.roots) > 16 {
		return PostPreparationPrivateV2{}, fmt.Errorf("private V2 protected producer root count: require 2..16")
	}
	// Root reuse across distinct packets must not widen the finite pair profile.
	if len(seenPairs) > 8 {
		return PostPreparationPrivateV2{}, fmt.Errorf("private V2 protected producer packet-pair count: require 1..8")
	}
	if err := ctx.Err(); err != nil {
		return PostPreparationPrivateV2{}, err
	}
	return h, nil
}
func (PostPreparationPrivateV2) MarshalJSON() ([]byte, error) {
	return nil, fmt.Errorf("opaque private V2 post-preparation")
}
func (h PostPreparationPrivateV2) PreparedBytes() ([]byte, error) {
	if h.state == nil {
		return nil, fmt.Errorf("unsealed private V2 post-preparation")
	}
	return bytes.Clone(h.state.prepared), nil
}
func (h PostPreparationPrivateV2) PreparationSHA256() (string, error) {
	if h.state == nil {
		return "", fmt.Errorf("unsealed private V2 post-preparation")
	}
	return h.state.sha, nil
}
func (h PostPreparationPrivateV2) Manifest() ([]PacketManifest, error) {
	if h.state == nil {
		return nil, fmt.Errorf("unsealed private V2 post-preparation")
	}
	return postClone(h.state.manifests), nil
}
func (h PostPreparationPrivateV2) Reconcile(ctx context.Context, subs []CallerSubmission, reviews []CallerReview) (StageLedgerPrivateV2, error) {
	l, err := reconcilePostState(ctx, h.state, subs, reviews)
	if err != nil {
		return StageLedgerPrivateV2{}, err
	}
	return StageLedgerPrivateV2{RequestTypePrivateV2, l}, nil
}

// Validate the complete supplied set against identities from the opaque producer
// reports, not caller assertions. Repeated batches deduplicate required roots;
// duplicate/foreign capabilities in the supplied set are always refused.
func (h PostPreparationPrivateV2) checkProtectedRoots(ctx context.Context, protected []*publication.ObservedRoot) error {
	if err := postContext(ctx); err != nil {
		return err
	}
	if h.state == nil || len(h.roots) < 2 || len(protected) != len(h.roots) || len(protected) > 16 {
		return fmt.Errorf("private V2 actual protected producer root set required")
	}
	seen := map[producerRootBindingPrivateV2]bool{}
	for _, r := range protected {
		path, id, err := r.CanonicalIdentityPrivateV2()
		if err != nil {
			return err
		}
		pin := producerRootBindingPrivateV2{path, id.Device, id.Inode}
		if seen[pin] {
			return fmt.Errorf("private V2 duplicate protected producer root")
		}
		seen[pin] = true
		found := false
		for _, want := range h.roots {
			if want == pin {
				found = true
			}
		}
		if !found {
			return fmt.Errorf("private V2 foreign protected producer root")
		}
	}
	return ctx.Err()
}
