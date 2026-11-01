//go:build darwin || linux

package privatehostproductv3

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	pilot "lsp-trace/internal/censuslineagepilot"
	"lsp-trace/internal/publication"
)

// Transfer is observational metadata; opening always performs the production
// validated-current-ledger gate BEFORE fresh source Freeze or retained admission.
type Transfer struct {
	Manifest   []byte
	Spec       pilot.ScopeSpecPrivateV2
	LedgerPath string
	Cleanup    Lifecycle
}

func OpenTransfer(ctx context.Context, t Transfer, bootstrap *pilot.PageCostPrivateV3, exchange pilot.PageCostPrivateV3) (pilot.RetainedPageCheckpointOwnerPrivateV3, func() error, error) {
	var zero pilot.RetainedPageCheckpointOwnerPrivateV3
	var m pilot.RetainedPageCheckpointManifestPrivateV3
	if len(t.Manifest) > 4096 || strict(t.Manifest, &m) != nil || bootstrap == nil {
		return zero, nil, ErrInvalid
	}
	ledger, e := pilot.OpenValidatedCurrentLedgerPrivateV3(ctx, t.LedgerPath, m.Application, m.Allowance, m.LedgerGeneration, m.LedgerTip, bootstrap)
	if e != nil {
		return zero, nil, e
	}
	closes := []func() error{ledger.Close}
	cleanup := func() error {
		var err error
		for i := len(closes) - 1; i >= 0; i-- {
			if e := closes[i](); e != nil {
				err = errors.Join(err, e)
			}
		}
		return err
	}
	frozen, e := pilot.FreezeRetainedCheckpointSourcePrivateV3(ctx, t.Spec, ledger, exchange)
	if e != nil {
		cleanup()
		return zero, nil, e
	}
	var outputs [2]*publication.ContentRetentionRoot
	for i, pin := range m.Roots {
		outputs[i], e = publication.OpenContentRetentionRoot(ctx, pin.Path)
		if e != nil {
			cleanup()
			return zero, nil, e
		}
		closes = append(closes, outputs[i].Close)
	}
	source, e := publication.OpenSourceProtectionRootPrivateV2(ctx, t.Spec.Root)
	if e != nil {
		cleanup()
		return zero, nil, e
	}
	closes = append(closes, source.Close)
	roots, e := pilot.NewProducerPacketRootsPrivateV2(ctx, outputs, source, nil)
	if e != nil {
		cleanup()
		return zero, nil, e
	}
	owner, e := pilot.AdmitRetainedPageCheckpointPrivateV3(ctx, t.Manifest, frozen, roots, ledger, pilot.RetainedCheckpointDisclosurePrivateV3{Application: m.Application, ConfigSHA256: m.Revision.Policy.ConfigSHA256, ContinuationIntent: m.Intent, PreparationRevision: pilot.RetainedCheckpointPreparationRevisionPrivateV3}, exchange)
	if e != nil {
		cleanup()
		return zero, nil, e
	}
	return owner, cleanup, nil
}

// Initialize performs actual closed-store production scanning, without provider
// startup, source access, original claim, or semantic disclosure.
func Initialize(c Config) error {
	if ValidateConfig(c) != nil {
		return ErrInvalid
	}
	s, e := openStore(c)
	if e != nil {
		return e
	}
	defer s.close()
	_, e = s.load(c)
	return e
}
func ClaimOriginal(c Config) error {
	s, e := openStore(c)
	if e != nil {
		return e
	}
	defer s.close()
	return s.write("claim/claim-00000000.json", mustJSON(struct{ Profile, ConfigSHA256 string }{Profile, ConfigDigest(c)}))
}

// DeliverEarlyFailure contains only safe phase/class codes; never raw errors,
// source, process environment, credentials, or fabricated resource measurements.
func DeliverEarlyFailure(c Config, phase, class string) (Report, error) {
	r := Report{Schema: "private-host-product-report.v3.1", Profile: Profile, Status: "FAILED", Application: c.Application, ConfigSHA256: ConfigDigest(c), FeatureIdentity: "UNRESOLVED", SourceCompleteness: "UNKNOWN", AbsenceInference: "PROHIBITED", Requested: c.Requested, Pending: c.Requested, Candidates: []Candidate{}, Unassigned: []string{}, NavigationHypotheses: []string{}, Failure: &Failure{Phase: phase, Class: class, Resource: "UNKNOWN", ExactOriginalError: "UNKNOWN"}, Spend: Spend{Excluded: []string{"acquisition/session/bootstrap/inference accounting not represented"}}}
	qualification := &AdmissionObservation{CurrentValidation: "FAILED", LedgerValidation: "UNKNOWN", SourceValidation: "UNKNOWN", Coverage: "UNKNOWN", Counts: "UNKNOWN", LedgerObservation: "UNKNOWN", ResourceCounts: "UNKNOWN"}
	r.Admission = qualification
	if ValidateConfig(c) != nil {
		return r, ErrInvalid
	}
	// Even a refused report store returns a safe nonempty DTO to the caller;
	// absent paired paths mean publication was unavailable, not successful.
	s, e := openStore(c)
	if e != nil {
		return r, e
	}
	defer s.close()
	qualification.PriorPublicationValidation = "LOCAL_CONSISTENCY_ONLY"
	if previous, readErr := s.latestReport(c); readErr == nil {
		qualification.UnadmittedPublications = s.rejectedPrior || previous.Admission != nil && previous.Admission.UnadmittedPublications
		if qualification.UnadmittedPublications || previous.Admission != nil && previous.Admission.PriorPublicationValidation == "UNKNOWN" {
			qualification.PriorPublicationValidation = "UNKNOWN"
		}
		if previous.FirstFailure == nil {
			previous.FirstFailure = previous.Failure
		}
		previous.Status = "FAILED"
		previous.ConfigSHA256 = ConfigDigest(c)
		previous.Failure = r.Failure
		qualification.Coverage = "PREVIOUS_REPORT_OBSERVATION"
		qualification.Counts = "PREVIOUS_REPORT_OBSERVATION"
		qualification.LedgerObservation = "PREVIOUS_REPORT_OBSERVATION_NOT_RECOMPUTED"
		qualification.PriorMachinePaths = previous.MachinePaths
		qualification.PriorHumanPaths = previous.HumanPaths
		previous.Admission = qualification
		// Keep reservations exactly as previously reported. Refusal costs and
		// current source/ledger validity are UNKNOWN, not zero-cost or revalidated.
		r = previous
	} else {
		qualification.PriorPublicationValidation = "UNKNOWN"
		qualification.UnadmittedPublications = s.rejectedPrior
	}
	if r.FirstFailure == nil {
		r.FirstFailure = r.Failure
	}
	e = s.deliver(&r)
	return r, e
}

// RetainedContinuation selects the latest saved observation and actual reported
// tip. It returns DATA, never an original owner. Missing/interrupted newer work
// still refuses at OpenValidatedCurrentLedgerPrivateV3.
func RetainedContinuation(c Config, t Transfer) (Transfer, error) {
	s, e := openStore(c)
	if e != nil {
		return t, e
	}
	defer s.close()
	d, e := s.root.Open("pages")
	if e != nil {
		return t, e
	}
	names, e := d.Readdirnames(-1)
	d.Close()
	if e != nil {
		return t, e
	}
	var p progress
	if len(names) == 0 {
		return t, ErrInvalid
	}
	b, e := s.read(fmt.Sprintf("pages/state-%08d.json", len(names)), stateLimit)
	if e != nil || strict(b, &p) != nil || len(p.LastCheckpoint) == 0 {
		return t, ErrInvalid
	}
	var m pilot.RetainedPageCheckpointManifestPrivateV3
	if strict(p.LastCheckpoint, &m) != nil {
		return t, ErrInvalid
	}
	// A checkpoint's older tip is not the current journal frontier. The final
	// immutable machine report records the actual later charged tip separately.
	r, e := s.latestReport(c)
	if e != nil || s.rejectedPrior || r.Application != c.Application {
		return t, ErrInvalid
	}
	// Do not rewrite checkpoint bytes to insert a newer tip. OpenTransferCurrent
	// accepts that newer frontier separately while admission verifies old anchors.
	t.Manifest = p.LastCheckpoint
	return t, nil
}

// OpenTransferCurrent is like OpenTransfer, with a separately observed actual
// terminal frontier. Original checkpoint bytes and their hashes never change.
func OpenTransferCurrent(ctx context.Context, t Transfer, g uint64, tip string, bootstrap *pilot.PageCostPrivateV3, exchange pilot.PageCostPrivateV3) (pilot.RetainedPageCheckpointOwnerPrivateV3, func() error, error) {
	var zero pilot.RetainedPageCheckpointOwnerPrivateV3
	var m pilot.RetainedPageCheckpointManifestPrivateV3
	if len(t.Manifest) == 0 || len(t.Manifest) > 4096 || bootstrap == nil || strict(t.Manifest, &m) != nil {
		return zero, nil, ErrInvalid
	}
	ledger, e := pilot.OpenValidatedCurrentLedgerPrivateV3(ctx, t.LedgerPath, m.Application, m.Allowance, g, tip, bootstrap)
	if e != nil {
		return zero, nil, e
	}
	closes := []func() error{ledger.Close}
	cleanup := func() error {
		var err error
		for i := len(closes) - 1; i >= 0; i-- {
			err = errors.Join(err, closes[i]())
		}
		return err
	}
	f, e := pilot.FreezeRetainedCheckpointSourcePrivateV3(ctx, t.Spec, ledger, exchange)
	if e != nil {
		cleanup()
		return zero, nil, e
	}
	var out [2]*publication.ContentRetentionRoot
	for i, pin := range m.Roots {
		out[i], e = publication.OpenContentRetentionRoot(ctx, pin.Path)
		if e != nil {
			cleanup()
			return zero, nil, e
		}
		closes = append(closes, out[i].Close)
	}
	source, e := publication.OpenSourceProtectionRootPrivateV2(ctx, t.Spec.Root)
	if e != nil {
		cleanup()
		return zero, nil, e
	}
	closes = append(closes, source.Close)
	roots, e := pilot.NewProducerPacketRootsPrivateV2(ctx, out, source, nil)
	if e != nil {
		cleanup()
		return zero, nil, e
	}
	owner, e := pilot.AdmitRetainedPageCheckpointPrivateV3(ctx, t.Manifest, f, roots, ledger, pilot.RetainedCheckpointDisclosurePrivateV3{Application: m.Application, ConfigSHA256: m.Revision.Policy.ConfigSHA256, ContinuationIntent: m.Intent, PreparationRevision: pilot.RetainedCheckpointPreparationRevisionPrivateV3}, exchange)
	if e != nil {
		cleanup()
		return zero, nil, e
	}
	return owner, cleanup, nil
}
func WriteTransfer(path string, t Transfer) error {
	if !filepath.IsAbs(path) {
		return ErrInvalid
	}
	r, e := os.OpenRoot(filepath.Dir(path))
	if e != nil {
		return e
	}
	defer r.Close()
	s := &store{root: r}
	return s.write(filepath.Base(path), mustJSON(t))
}
func ReadTransfer(path string) (Transfer, error) {
	var t Transfer
	s, e := mailboxStore(filepath.Dir(path))
	if e != nil {
		return t, e
	}
	defer s.close()
	b, e := s.read(filepath.Base(path), 128<<10)
	if e != nil || strict(b, &t) != nil {
		return t, ErrInvalid
	}
	return t, nil
}
func MarshalCanonical(v any) ([]byte, error) { return json.Marshal(v) }
