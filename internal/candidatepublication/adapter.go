package candidatepublication

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"lsp-trace/internal/censuscontinuation"
	"lsp-trace/internal/publication"
	"lsp-trace/internal/verification"
)

const (
	CompletenessUnknown       = "UNKNOWN"
	FeatureIdentityUnresolved = "UNRESOLVED"

	qualificationReceiptSchema = "lsp-trace.private-candidate-publication-receipt.v1"
	manifestSchema             = "lsp-trace.private-candidate-publication-manifest.v1"
	manifestNamespace          = "candidate-publication/manifests"
	currentSelector            = "candidate-publication/current.selector.json"

	PostcommitVerificationFailed = "COMMITTED_VERIFICATION_FAILED"
)

type Options struct{ MaxBytes int64 }

type Adapter struct {
	root      *publication.Root
	publisher *publication.Publisher
	maxBytes  int64
}

var compareAndReplaceBoundFile = publication.CompareAndReplaceBoundFile

type PublishRequest struct {
	CandidateBytes        []byte
	GroupingPolicyID      string
	GroupingPolicyDigest  string
	QualificationID       string
	SourceRevision        string
	PredecessorSelector   string
	Representative        Representative
	Authority             int
	Accepted              bool
	Completeness          string
	FeatureIdentityStatus string
}

type Representative struct {
	ID       string `json:"id"`
	Status   string `json:"status"`
	Inferred bool   `json:"inferred"`
}

type PublishedGeneration struct {
	Selector                                 string
	Generation                               string
	VerificationSelector                     string
	ArtifactSelector                         string
	ByteCustodyReceiptSelector               string
	QualificationReceiptSelector             string
	QualificationReceiptVerificationSelector string
	QualificationReceiptDigest               string
	QualificationReceiptByteLength           uint64
	ManifestSelector                         string
	ManifestVerificationSelector             string
	ManifestDigest                           string
	ManifestByteLength                       uint64
	PublicationMechanism                     string
	Committed                                bool
	PostcommitVerificationStatus             string
}

type AdvanceRequest struct {
	Generation            string
	VerificationSelector  string
	ManifestSelector      string
	ManifestDigest        string
	ManifestByteLength    uint64
	PredecessorAbsent     bool
	PredecessorSelector   string
	PredecessorDigest     string
	PredecessorByteLength uint64
}

type CurrentGeneration struct {
	Selector                                 string `json:"selector"`
	Generation                               string `json:"generation"`
	VerificationSelector                     string `json:"verification_selector"`
	CandidateDigest                          string `json:"candidate_digest"`
	CandidateByteLength                      uint64 `json:"candidate_byte_length"`
	QualificationReceiptSelector             string `json:"qualification_receipt_selector"`
	QualificationReceiptVerificationSelector string `json:"qualification_receipt_verification_selector"`
	QualificationReceiptDigest               string `json:"qualification_receipt_digest"`
	QualificationReceiptByteLength           uint64 `json:"qualification_receipt_byte_length"`
	ManifestSelector                         string `json:"manifest_selector"`
	ManifestVerificationSelector             string `json:"manifest_verification_selector"`
	ManifestDigest                           string `json:"manifest_digest"`
	ManifestByteLength                       uint64 `json:"manifest_byte_length"`
	SelectorDigest                           string `json:"-"`
	SelectorByteLength                       uint64 `json:"-"`
}

type CandidateGeneration struct {
	CandidateBytes []byte
	Receipt        Receipt
	Published      PublishedGeneration
}

type DigestBinding struct {
	Hex   string `json:"hex"`
	Bytes []byte `json:"bytes"`
}

type Receipt struct {
	SchemaVersion         string         `json:"schema_version"`
	CandidateDigest       DigestBinding  `json:"candidate_digest"`
	CandidateByteLength   uint64         `json:"candidate_byte_length"`
	GroupingPolicyID      string         `json:"grouping_policy_id"`
	GroupingPolicyDigest  string         `json:"grouping_policy_digest"`
	QualificationID       string         `json:"qualification_id"`
	SourceRevision        string         `json:"source_revision"`
	Generation            string         `json:"generation"`
	PredecessorSelector   string         `json:"predecessor_selector"`
	Representative        Representative `json:"representative"`
	Authority             int            `json:"authority"`
	Accepted              bool           `json:"accepted"`
	Completeness          string         `json:"completeness"`
	FeatureIdentityStatus string         `json:"feature_identity_status"`
}

type Manifest struct {
	SchemaVersion                            string `json:"schema_version"`
	CandidateGeneration                      string `json:"candidate_generation"`
	CandidateVerificationSelector            string `json:"candidate_verification_selector"`
	CandidateSelector                        string `json:"candidate_selector"`
	CandidateDigest                          string `json:"candidate_digest"`
	CandidateByteLength                      uint64 `json:"candidate_byte_length"`
	QualificationReceiptGeneration           string `json:"qualification_receipt_generation"`
	QualificationReceiptSelector             string `json:"qualification_receipt_selector"`
	QualificationReceiptVerificationSelector string `json:"qualification_receipt_verification_selector"`
	QualificationReceiptDigest               string `json:"qualification_receipt_digest"`
	QualificationReceiptByteLength           uint64 `json:"qualification_receipt_byte_length"`
}

func NewRepositoryPrivateAdapter(root *publication.Root, opts Options) (*Adapter, error) {
	if root == nil || opts.MaxBytes < 1 {
		return nil, errors.New("candidatepublication: invalid adapter configuration")
	}
	if err := root.ValidatePrivate(); err != nil {
		return nil, err
	}
	return &Adapter{root: root, publisher: publication.NewPublisher(), maxBytes: opts.MaxBytes}, nil
}

func DigestBytes(raw []byte) []byte { sum := sha256.Sum256(raw); return sum[:] }
func Digest(raw []byte) string      { return "sha256:" + hex.EncodeToString(DigestBytes(raw)) }

func (a *Adapter) PublishCandidateGeneration(ctx context.Context, req PublishRequest) (PublishedGeneration, error) {
	if err := ctx.Err(); err != nil {
		return PublishedGeneration{}, err
	}
	if a == nil || a.root == nil || a.publisher == nil || int64(len(req.CandidateBytes)) > a.maxBytes {
		return PublishedGeneration{}, errors.New("candidatepublication: invalid publish request")
	}
	artifact, err := censuscontinuation.ParseCandidateGroup(req.CandidateBytes)
	if err != nil {
		return PublishedGeneration{}, err
	}
	if err := validateRequestCeilings(req); err != nil {
		return PublishedGeneration{}, err
	}
	if req.GroupingPolicyID != artifact.ResourceProfileID || req.GroupingPolicyDigest != artifact.ResourceProfileDigest {
		return PublishedGeneration{}, errors.New("candidatepublication: grouping policy mismatch")
	}
	if req.Representative.ID != artifact.Representative.ID || req.Representative.Status != artifact.Representative.Role || req.Representative.Inferred {
		return PublishedGeneration{}, errors.New("candidatepublication: representative mismatch")
	}
	if err := ctx.Err(); err != nil {
		return PublishedGeneration{}, err
	}
	published := a.publisher.PublishVerifiedGenerationContext(ctx, a.root, append([]byte(nil), req.CandidateBytes...), censuscontinuation.CandidateGroupSchema)
	if published.Failure != nil {
		return PublishedGeneration{}, published.Failure
	}
	generation := published.Receipt.Generation
	out := PublishedGeneration{Generation: generation, VerificationSelector: published.Receipt.VerificationSelector, ArtifactSelector: generation + "/artifact.json", ByteCustodyReceiptSelector: generation + "/receipt.json", PublicationMechanism: published.Receipt.PublicationMechanism}
	receipt, err := qualificationReceipt(req, generation)
	if err != nil {
		return out, err
	}
	rawReceipt, err := json.Marshal(receipt)
	if err != nil {
		return out, err
	}
	rawReceipt = append(rawReceipt, '\n')
	if err := ctx.Err(); err != nil {
		return out, err
	}
	qualified := a.publisher.PublishVerifiedGenerationContext(ctx, a.root, rawReceipt, qualificationReceiptSchema)
	if qualified.Failure != nil {
		return out, qualified.Failure
	}
	qGen := qualified.Receipt.Generation
	out.QualificationReceiptSelector = qGen + "/artifact.json"
	out.QualificationReceiptVerificationSelector = qualified.Receipt.VerificationSelector
	out.QualificationReceiptDigest = qualified.Receipt.Digest
	out.QualificationReceiptByteLength = qualified.Receipt.ByteLength
	manifest := Manifest{SchemaVersion: manifestSchema, CandidateGeneration: generation, CandidateVerificationSelector: out.VerificationSelector, CandidateSelector: out.ArtifactSelector, CandidateDigest: Digest(req.CandidateBytes), CandidateByteLength: uint64(len(req.CandidateBytes)), QualificationReceiptGeneration: qGen, QualificationReceiptSelector: out.QualificationReceiptSelector, QualificationReceiptVerificationSelector: out.QualificationReceiptVerificationSelector, QualificationReceiptDigest: out.QualificationReceiptDigest, QualificationReceiptByteLength: out.QualificationReceiptByteLength}
	rawManifest, err := json.Marshal(manifest)
	if err != nil {
		return out, err
	}
	rawManifest = append(rawManifest, '\n')
	if err := ctx.Err(); err != nil {
		return out, err
	}
	manifestPub := a.publisher.PublishVerifiedGenerationContext(ctx, a.root, rawManifest, manifestSchema)
	if manifestPub.Failure != nil {
		return out, manifestPub.Failure
	}
	out.ManifestSelector = manifestPub.Receipt.Generation + "/artifact.json"
	out.ManifestVerificationSelector = manifestPub.Receipt.VerificationSelector
	out.ManifestDigest = manifestPub.Receipt.Digest
	out.ManifestByteLength = manifestPub.Receipt.ByteLength
	aliasSelector := manifestSelectorForGeneration(generation)
	alias := a.publisher.Publish(publication.Request{Root: a.root, Selector: aliasSelector, Bytes: rawManifest, ArtifactSchemaID: manifestSchema})
	if alias.Failure != nil {
		if alias.Failure.Code == publication.CodeTargetExists && a.existingManifestAliasExactly(aliasSelector, rawManifest, manifest) == nil {
			return out, nil
		}
		return out, alias.Failure
	}
	return out, nil
}

func (a *Adapter) AdvanceCandidateGeneration(ctx context.Context, req AdvanceRequest) (PublishedGeneration, error) {
	if err := ctx.Err(); err != nil {
		return PublishedGeneration{}, err
	}
	if a == nil || a.root == nil || req.Generation == "" || req.VerificationSelector == "" {
		return PublishedGeneration{}, errors.New("candidatepublication: invalid advance request")
	}
	candidate, receipt, manifest, manifestID, err := a.verifyGenerationAndReceipt(req.Generation, req.VerificationSelector, req.ManifestSelector, req.ManifestDigest, req.ManifestByteLength)
	if err != nil {
		return PublishedGeneration{}, err
	}
	if err := ctx.Err(); err != nil {
		return PublishedGeneration{}, err
	}
	current := CurrentGeneration{Selector: currentSelector, Generation: req.Generation, VerificationSelector: req.VerificationSelector, CandidateDigest: Digest(candidate), CandidateByteLength: uint64(len(candidate)), QualificationReceiptSelector: manifest.QualificationReceiptSelector, QualificationReceiptVerificationSelector: manifest.QualificationReceiptVerificationSelector, QualificationReceiptDigest: manifest.QualificationReceiptDigest, QualificationReceiptByteLength: manifest.QualificationReceiptByteLength, ManifestSelector: manifestID.Selector, ManifestVerificationSelector: manifestID.VerificationSelector, ManifestDigest: manifestID.Digest, ManifestByteLength: manifestID.ByteLength}
	selectorBytes, err := json.Marshal(current)
	if err != nil {
		return PublishedGeneration{}, err
	}
	selectorBytes = append(selectorBytes, '\n')
	pred, err := req.boundFilePredecessor(receipt.PredecessorSelector)
	if err != nil {
		return PublishedGeneration{}, err
	}
	boundReceipt, err := compareAndReplaceBoundFile(ctx, a.root, currentSelector, pred, selectorBytes, func(final []byte) error {
		var c CurrentGeneration
		if err := decodeStrict(final, &c); err != nil {
			return err
		}
		return a.validateCurrent(c)
	})
	if err != nil {
		if boundReceipt != nil && boundReceipt.Committed {
			return publishedGenerationFromCASReceipt(req, manifest, manifestID, boundReceipt), err
		}
		return PublishedGeneration{}, err
	}
	return publishedGenerationFromCASReceipt(req, manifest, manifestID, boundReceipt), nil
}

func publishedGenerationFromCASReceipt(req AdvanceRequest, manifest Manifest, manifestID manifestIdentity, boundReceipt *publication.CompareAndReplaceReceipt) PublishedGeneration {
	out := PublishedGeneration{Selector: currentSelector, Generation: req.Generation, VerificationSelector: req.VerificationSelector, ArtifactSelector: req.Generation + "/artifact.json", ByteCustodyReceiptSelector: req.Generation + "/receipt.json", QualificationReceiptSelector: manifest.QualificationReceiptSelector, QualificationReceiptVerificationSelector: manifest.QualificationReceiptVerificationSelector, QualificationReceiptDigest: manifest.QualificationReceiptDigest, QualificationReceiptByteLength: manifest.QualificationReceiptByteLength, ManifestSelector: manifestID.Selector, ManifestVerificationSelector: manifestID.VerificationSelector, ManifestDigest: manifestID.Digest, ManifestByteLength: manifestID.ByteLength, PublicationMechanism: publication.VerifiedGenerationMechanism}
	if boundReceipt != nil {
		out.Committed = boundReceipt.Committed
		out.PostcommitVerificationStatus = boundReceipt.VerificationStatus
		if out.PostcommitVerificationStatus == "" {
			out.PostcommitVerificationStatus = boundReceipt.Outcome
		}
	}
	return out
}

func (a *Adapter) CurrentCandidateGeneration(ctx context.Context) (CurrentGeneration, error) {
	if err := ctx.Err(); err != nil {
		return CurrentGeneration{}, err
	}
	if a == nil || a.root == nil {
		return CurrentGeneration{}, errors.New("candidatepublication: invalid adapter")
	}
	raw, err := a.root.ReadSelector(currentSelector, 4096)
	if err != nil {
		return CurrentGeneration{}, err
	}
	var current CurrentGeneration
	if err := decodeStrict(raw, &current); err != nil {
		return CurrentGeneration{}, err
	}
	if err := a.validateCurrent(current); err != nil {
		return CurrentGeneration{}, err
	}
	current.SelectorDigest = publication.DigestForTest(raw)
	current.SelectorByteLength = uint64(len(raw))
	return current, nil
}

func (a *Adapter) GetCandidateGeneration(ctx context.Context, generation string) (CandidateGeneration, error) {
	if err := ctx.Err(); err != nil {
		return CandidateGeneration{}, err
	}
	candidate, receipt, manifest, manifestID, err := a.verifyGenerationAndReceipt(generation, generation+".selector.json", "", "", 0)
	if err != nil {
		return CandidateGeneration{}, err
	}
	return CandidateGeneration{CandidateBytes: candidate, Receipt: receipt, Published: PublishedGeneration{Generation: generation, VerificationSelector: generation + ".selector.json", ArtifactSelector: generation + "/artifact.json", ByteCustodyReceiptSelector: generation + "/receipt.json", QualificationReceiptSelector: manifest.QualificationReceiptSelector, QualificationReceiptVerificationSelector: manifest.QualificationReceiptVerificationSelector, QualificationReceiptDigest: manifest.QualificationReceiptDigest, QualificationReceiptByteLength: manifest.QualificationReceiptByteLength, ManifestSelector: manifestID.Selector, ManifestVerificationSelector: manifestID.VerificationSelector, ManifestDigest: manifestID.Digest, ManifestByteLength: manifestID.ByteLength, PublicationMechanism: publication.VerifiedGenerationMechanism}}, nil
}

type manifestIdentity struct {
	Selector, VerificationSelector, Digest string
	ByteLength                             uint64
}

func (a *Adapter) verifyGenerationAndReceipt(generation, verificationSelector, manifestSelector, manifestDigest string, manifestByteLength uint64) ([]byte, Receipt, Manifest, manifestIdentity, error) {
	candidate, err := a.verifyCandidateGeneration(generation, verificationSelector)
	if err != nil {
		return nil, Receipt{}, Manifest{}, manifestIdentity{}, err
	}
	manifest, id, err := a.readManifestForGeneration(generation, manifestSelector, manifestDigest, manifestByteLength)
	if err != nil {
		return nil, Receipt{}, Manifest{}, manifestIdentity{}, err
	}
	receipt, err := a.verifyQualificationBinding(candidate, generation, manifest)
	if err != nil {
		return nil, Receipt{}, Manifest{}, manifestIdentity{}, err
	}
	return candidate, receipt, manifest, id, nil
}

func (a *Adapter) verifyCandidateGeneration(generation, verificationSelector string) ([]byte, error) {
	if !isGeneration(generation) || verificationSelector != generation+".selector.json" {
		return nil, errors.New("candidatepublication: invalid generation selector")
	}
	candidate, err := a.root.ReadSelector(generation+"/artifact.json", a.maxBytes)
	if err != nil {
		return nil, err
	}
	if Digest(candidate) != "sha256:"+strings.TrimPrefix(generation, "g-") {
		return nil, errors.New("candidatepublication: generation artifact digest mismatch")
	}
	byteReceipt, err := a.root.ReadSelector(generation+"/receipt.json", 1<<20)
	if err != nil || verification.VerifyReceipt(candidate, byteReceipt) != nil {
		return nil, errors.New("candidatepublication: byte custody receipt verification failed")
	}
	selectorBytes, err := a.root.ReadSelector(verificationSelector, 4096)
	if err != nil {
		return nil, err
	}
	selector, err := verification.DecodeSelector(selectorBytes)
	if err != nil || selector.Generation != generation {
		return nil, errors.New("candidatepublication: generation selector verification failed")
	}
	return candidate, nil
}

func (a *Adapter) verifyQualificationBinding(candidate []byte, generation string, manifest Manifest) (Receipt, error) {
	receiptBytes, err := a.root.ReadSelector(manifest.QualificationReceiptSelector, 1<<20)
	if err != nil {
		return Receipt{}, err
	}
	if Digest(receiptBytes) != manifest.QualificationReceiptDigest || uint64(len(receiptBytes)) != manifest.QualificationReceiptByteLength {
		return Receipt{}, errors.New("candidatepublication: qualification receipt digest mismatch")
	}
	qSelectorBytes, err := a.root.ReadSelector(manifest.QualificationReceiptVerificationSelector, 4096)
	if err != nil {
		return Receipt{}, err
	}
	qSelector, err := verification.DecodeSelector(qSelectorBytes)
	if err != nil || manifest.QualificationReceiptGeneration != qSelector.Generation || manifest.QualificationReceiptSelector != qSelector.Generation+"/artifact.json" {
		return Receipt{}, errors.New("candidatepublication: qualification selector mismatch")
	}
	qByteReceipt, err := a.root.ReadSelector(qSelector.Generation+"/receipt.json", 1<<20)
	if err != nil || verification.VerifyReceipt(receiptBytes, qByteReceipt) != nil {
		return Receipt{}, errors.New("candidatepublication: qualification byte custody receipt failed")
	}
	var receipt Receipt
	if err := DecodeReceiptStrict(receiptBytes, &receipt); err != nil {
		return Receipt{}, err
	}
	if err := verifyQualificationReceipt(candidate, generation, receipt); err != nil {
		return Receipt{}, err
	}
	return receipt, nil
}

func (a *Adapter) readManifestForGeneration(generation, selector, digest string, byteLength uint64) (Manifest, manifestIdentity, error) {
	defaultAlias := false
	if selector == "" {
		selector = manifestSelectorForGeneration(generation)
	}
	if selector == manifestSelectorForGeneration(generation) {
		defaultAlias = true
	}
	var raw []byte
	var err error
	if defaultAlias {
		raw, err = publication.ReadVerifiedBoundFile(a.root, selector, 1<<20)
	} else {
		raw, err = a.root.ReadSelector(selector, 1<<20)
	}
	if err != nil {
		return Manifest{}, manifestIdentity{}, err
	}
	id := manifestIdentity{Selector: selector, VerificationSelector: strings.TrimSuffix(selector, "/artifact.json") + ".selector.json", Digest: Digest(raw), ByteLength: uint64(len(raw))}
	if digest != "" && (digest != id.Digest || byteLength != id.ByteLength) {
		return Manifest{}, manifestIdentity{}, errors.New("candidatepublication: manifest identity mismatch")
	}
	var manifest Manifest
	if err := decodeStrict(raw, &manifest); err != nil {
		return Manifest{}, manifestIdentity{}, err
	}
	if err := verifyManifest(manifest, generation); err != nil {
		return Manifest{}, manifestIdentity{}, err
	}
	if defaultAlias {
		manifestGeneration := "g-" + strings.TrimPrefix(id.Digest, "sha256:")
		if err := a.verifyManifestGenerationBytes(manifestGeneration, raw); err != nil {
			return Manifest{}, manifestIdentity{}, err
		}
		id.VerificationSelector = manifestGeneration + ".selector.json"
		return manifest, id, nil
	}
	if err := a.verifyManifestGenerationBytes(manifestGenerationFromArtifactSelector(selector), raw); err != nil {
		return Manifest{}, manifestIdentity{}, err
	}
	return manifest, id, nil
}

func (a *Adapter) existingManifestAliasExactly(selector string, expected []byte, expectedManifest Manifest) error {
	raw, err := publication.ReadVerifiedBoundFile(a.root, selector, int64(len(expected))+1)
	if err != nil {
		return err
	}
	if !bytes.Equal(raw, expected) || Digest(raw) != Digest(expected) || len(raw) != len(expected) {
		return errors.New("candidatepublication: immutable manifest alias collision")
	}
	var got Manifest
	if err := decodeStrict(raw, &got); err != nil {
		return err
	}
	if !manifestCanonicalEqual(got, expectedManifest) {
		return errors.New("candidatepublication: immutable manifest alias payload collision")
	}
	return nil
}

func (a *Adapter) verifyManifestGenerationBytes(generation string, raw []byte) error {
	if !isGeneration(generation) || Digest(raw) != "sha256:"+strings.TrimPrefix(generation, "g-") {
		return errors.New("candidatepublication: manifest generation identity mismatch")
	}
	retained, err := a.root.ReadSelector(generation+"/artifact.json", int64(len(raw))+1)
	if err != nil || !bytes.Equal(retained, raw) {
		return errors.New("candidatepublication: manifest generation artifact mismatch")
	}
	selectorBytes, err := a.root.ReadSelector(generation+".selector.json", 4096)
	if err != nil {
		return err
	}
	decoded, err := verification.DecodeSelector(selectorBytes)
	if err != nil || decoded.Generation != generation {
		return errors.New("candidatepublication: manifest selector mismatch")
	}
	byteReceipt, err := a.root.ReadSelector(generation+"/receipt.json", 1<<20)
	if err != nil || verification.VerifyReceipt(raw, byteReceipt) != nil {
		return errors.New("candidatepublication: manifest byte custody receipt failed")
	}
	return nil
}

func manifestCanonicalEqual(a, b Manifest) bool { return a == b }

func manifestGenerationFromArtifactSelector(selector string) string {
	return strings.TrimSuffix(selector, "/artifact.json")
}

func manifestSelectorForGeneration(generation string) string {
	return manifestNamespace + "/" + strings.TrimPrefix(generation, "g-") + ".manifest.json"
}

func (a *Adapter) validateCurrent(current CurrentGeneration) error {
	if current.Selector != currentSelector || !isGeneration(current.Generation) || current.VerificationSelector != current.Generation+".selector.json" || current.CandidateDigest == "" || current.CandidateByteLength == 0 || current.QualificationReceiptSelector == "" || current.QualificationReceiptVerificationSelector == "" || current.QualificationReceiptDigest == "" || current.QualificationReceiptByteLength == 0 || current.ManifestSelector == "" || current.ManifestVerificationSelector == "" || current.ManifestDigest == "" || current.ManifestByteLength == 0 {
		return errors.New("candidatepublication: invalid current selector")
	}
	candidate, receipt, manifest, id, err := a.verifyGenerationAndReceipt(current.Generation, current.VerificationSelector, current.ManifestSelector, current.ManifestDigest, current.ManifestByteLength)
	if err != nil {
		return err
	}
	if Digest(candidate) != current.CandidateDigest || uint64(len(candidate)) != current.CandidateByteLength || receipt.CandidateDigest.Hex != current.CandidateDigest {
		return errors.New("candidatepublication: current candidate binding mismatch")
	}
	if manifest.QualificationReceiptSelector != current.QualificationReceiptSelector || manifest.QualificationReceiptVerificationSelector != current.QualificationReceiptVerificationSelector || manifest.QualificationReceiptDigest != current.QualificationReceiptDigest || manifest.QualificationReceiptByteLength != current.QualificationReceiptByteLength || id.VerificationSelector != current.ManifestVerificationSelector {
		return errors.New("candidatepublication: current manifest binding mismatch")
	}
	return nil
}

func (req AdvanceRequest) boundFilePredecessor(initialSelector string) (publication.BoundFilePredecessor, error) {
	if req.PredecessorAbsent {
		if req.PredecessorSelector != "" || req.PredecessorDigest != "" || req.PredecessorByteLength != 0 {
			return publication.BoundFilePredecessor{}, errors.New("candidatepublication: contradictory predecessor token")
		}
		return publication.BoundFilePredecessor{Absent: true}, nil
	}
	if req.PredecessorDigest == "" && req.PredecessorByteLength == 0 && initialSelector != "" && (req.PredecessorSelector == "" || req.PredecessorSelector == initialSelector) {
		return publication.BoundFilePredecessor{Absent: true}, nil
	}
	if req.PredecessorSelector != currentSelector || req.PredecessorDigest == "" || req.PredecessorByteLength == 0 {
		return publication.BoundFilePredecessor{}, errors.New("candidatepublication: exact predecessor token required")
	}
	return publication.BoundFilePredecessor{Selector: currentSelector, Digest: req.PredecessorDigest, ByteLength: req.PredecessorByteLength}, nil
}

func verifyManifest(manifest Manifest, generation string) error {
	if manifest.SchemaVersion != manifestSchema || manifest.CandidateGeneration != generation || manifest.CandidateVerificationSelector != generation+".selector.json" || manifest.CandidateSelector != generation+"/artifact.json" || manifest.CandidateDigest == "" || manifest.CandidateByteLength == 0 || !isGeneration(manifest.QualificationReceiptGeneration) || manifest.QualificationReceiptSelector != manifest.QualificationReceiptGeneration+"/artifact.json" || manifest.QualificationReceiptVerificationSelector != manifest.QualificationReceiptGeneration+".selector.json" || manifest.QualificationReceiptDigest == "" || manifest.QualificationReceiptByteLength == 0 {
		return errors.New("candidatepublication: invalid manifest")
	}
	return nil
}

func qualificationReceipt(req PublishRequest, generation string) (Receipt, error) {
	if !isGeneration(generation) {
		return Receipt{}, errors.New("candidatepublication: invalid generation")
	}
	return Receipt{SchemaVersion: qualificationReceiptSchema, CandidateDigest: DigestBinding{Hex: Digest(req.CandidateBytes), Bytes: DigestBytes(req.CandidateBytes)}, CandidateByteLength: uint64(len(req.CandidateBytes)), GroupingPolicyID: req.GroupingPolicyID, GroupingPolicyDigest: req.GroupingPolicyDigest, QualificationID: req.QualificationID, SourceRevision: req.SourceRevision, Generation: generation, PredecessorSelector: req.PredecessorSelector, Representative: req.Representative, Authority: req.Authority, Accepted: req.Accepted, Completeness: req.Completeness, FeatureIdentityStatus: req.FeatureIdentityStatus}, nil
}

func verifyQualificationReceipt(candidate []byte, generation string, receipt Receipt) error {
	if receipt.SchemaVersion != qualificationReceiptSchema || receipt.Generation != generation || receipt.CandidateDigest.Hex != Digest(candidate) || !bytes.Equal(receipt.CandidateDigest.Bytes, DigestBytes(candidate)) || receipt.CandidateByteLength != uint64(len(candidate)) {
		return errors.New("candidatepublication: qualification receipt candidate mismatch")
	}
	if receipt.Authority != 0 || receipt.Accepted || receipt.Completeness != CompletenessUnknown || receipt.FeatureIdentityStatus != FeatureIdentityUnresolved {
		return errors.New("candidatepublication: qualification receipt ceilings mismatch")
	}
	artifact, err := censuscontinuation.ParseCandidateGroup(candidate)
	if err != nil {
		return err
	}
	if receipt.GroupingPolicyID != artifact.ResourceProfileID || receipt.GroupingPolicyDigest != artifact.ResourceProfileDigest || receipt.Representative.ID != artifact.Representative.ID || receipt.Representative.Status != artifact.Representative.Role || receipt.Representative.Inferred {
		return errors.New("candidatepublication: qualification receipt artifact binding mismatch")
	}
	return nil
}

func validateRequestCeilings(req PublishRequest) error {
	if req.CandidateBytes == nil || req.GroupingPolicyID == "" || req.GroupingPolicyDigest == "" || req.QualificationID == "" || req.SourceRevision == "" || req.PredecessorSelector == "" || req.Representative.ID == "" || req.Representative.Status == "" {
		return errors.New("candidatepublication: incomplete publish request")
	}
	if req.Authority != 0 || req.Accepted || req.Completeness != CompletenessUnknown || req.FeatureIdentityStatus != FeatureIdentityUnresolved || req.Representative.Inferred {
		return errors.New("candidatepublication: unsupported candidate authority")
	}
	return nil
}

func isGeneration(g string) bool {
	if len(g) != 66 || !strings.HasPrefix(g, "g-") {
		return false
	}
	h := strings.TrimPrefix(g, "g-")
	_, err := hex.DecodeString(h)
	return err == nil && strings.ToLower(h) == h
}
func DecodeReceiptStrict(raw []byte, receipt *Receipt) error { return decodeStrict(raw, receipt) }

func decodeStrict(raw []byte, out any) error {
	if err := rejectDuplicateKeys(raw); err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err != nil {
		return err
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return errors.New("candidatepublication: trailing json")
	}
	canonical, err := json.Marshal(out)
	if err != nil {
		return err
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, bytes.TrimSpace(raw)); err != nil {
		return err
	}
	if !bytes.Equal(compact.Bytes(), canonical) {
		return errors.New("candidatepublication: noncanonical json")
	}
	return nil
}

func rejectDuplicateKeys(raw []byte) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	var stack []map[string]struct{}
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		switch v := tok.(type) {
		case json.Delim:
			switch v {
			case '{':
				stack = append(stack, map[string]struct{}{})
			case '}':
				if len(stack) > 0 {
					stack = stack[:len(stack)-1]
				}
			}
		case string:
			if len(stack) == 0 {
				continue
			}
			off := dec.InputOffset()
			for off < int64(len(raw)) && (raw[off] == ' ' || raw[off] == '\n' || raw[off] == '\r' || raw[off] == '\t') {
				off++
			}
			if off < int64(len(raw)) && raw[off] == ':' {
				seen := stack[len(stack)-1]
				if _, ok := seen[v]; ok {
					return fmt.Errorf("candidatepublication: duplicate key %q", v)
				}
				seen[v] = struct{}{}
			}
		}
	}
}
