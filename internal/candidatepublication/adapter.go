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
	"os"
	"path/filepath"
	"strings"
	"time"

	"lsp-trace/internal/censuscontinuation"
	"lsp-trace/internal/publication"
	"lsp-trace/internal/verification"
)

const (
	CompletenessUnknown       = "UNKNOWN"
	FeatureIdentityUnresolved = "UNRESOLVED"

	qualificationReceiptSchema = "lsp-trace.private-candidate-publication-receipt.v1"
	qualificationNamespace     = "candidate-publication/qualification-receipts"
	currentSelector            = "candidate-publication/current.selector.json"
	currentLockSelector        = "candidate-publication/current.selector.lock"

	PostcommitVerificationFailed = "COMMITTED_VERIFICATION_FAILED"
)

type Options struct{ MaxBytes int64 }

type Adapter struct {
	root      *publication.Root
	publisher *publication.Publisher
	maxBytes  int64
}

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
	PublicationMechanism                     string
	Committed                                bool
	PostcommitVerificationStatus             string
}

type AdvanceRequest struct {
	Generation           string
	VerificationSelector string
	PredecessorSelector  string
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

type qualificationIndex struct {
	SchemaVersion                string `json:"schema_version"`
	Generation                   string `json:"generation"`
	QualificationReceiptSelector string `json:"qualification_receipt_selector"`
	VerificationSelector         string `json:"verification_selector"`
	Digest                       string `json:"digest"`
	ByteLength                   uint64 `json:"byte_length"`
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
	published := a.publisher.PublishVerifiedGeneration(a.root, append([]byte(nil), req.CandidateBytes...), censuscontinuation.CandidateGroupSchema)
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
	qualified := a.publisher.PublishVerifiedGeneration(a.root, rawReceipt, qualificationReceiptSchema)
	if qualified.Failure != nil {
		return out, qualified.Failure
	}
	qGen := qualified.Receipt.Generation
	out.QualificationReceiptSelector = qGen + "/artifact.json"
	out.QualificationReceiptVerificationSelector = qualified.Receipt.VerificationSelector
	out.QualificationReceiptDigest = qualified.Receipt.Digest
	out.QualificationReceiptByteLength = qualified.Receipt.ByteLength
	if err := ctx.Err(); err != nil {
		return out, err
	}
	idx := qualificationIndex{SchemaVersion: "lsp-trace.private-candidate-publication-index.v1", Generation: generation, QualificationReceiptSelector: out.QualificationReceiptSelector, VerificationSelector: out.QualificationReceiptVerificationSelector, Digest: out.QualificationReceiptDigest, ByteLength: out.QualificationReceiptByteLength}
	if err := a.writeQualificationIndex(idx); err != nil {
		return out, err
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
	candidate, receipt, idx, err := a.verifyGenerationAndReceipt(req.Generation, req.VerificationSelector)
	if err != nil {
		return PublishedGeneration{}, err
	}
	if err := ctx.Err(); err != nil {
		return PublishedGeneration{}, err
	}
	current := CurrentGeneration{Selector: currentSelector, Generation: req.Generation, VerificationSelector: req.VerificationSelector, CandidateDigest: Digest(candidate), CandidateByteLength: uint64(len(candidate)), QualificationReceiptSelector: idx.QualificationReceiptSelector, QualificationReceiptVerificationSelector: idx.VerificationSelector, QualificationReceiptDigest: idx.Digest, QualificationReceiptByteLength: idx.ByteLength}
	selectorBytes, err := json.Marshal(current)
	if err != nil {
		return PublishedGeneration{}, err
	}
	selectorBytes = append(selectorBytes, '\n')
	boundReceipt, err := a.casCurrent(ctx, req.PredecessorSelector, receipt.PredecessorSelector, selectorBytes)
	if err != nil {
		return PublishedGeneration{}, err
	}
	return PublishedGeneration{Selector: currentSelector, Generation: req.Generation, VerificationSelector: req.VerificationSelector, ArtifactSelector: req.Generation + "/artifact.json", ByteCustodyReceiptSelector: req.Generation + "/receipt.json", QualificationReceiptSelector: idx.QualificationReceiptSelector, QualificationReceiptVerificationSelector: idx.VerificationSelector, QualificationReceiptDigest: idx.Digest, QualificationReceiptByteLength: idx.ByteLength, PublicationMechanism: publication.VerifiedGenerationMechanism, Committed: true, PostcommitVerificationStatus: boundReceipt.VerificationStatus}, nil
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
	return current, nil
}

func (a *Adapter) GetCandidateGeneration(ctx context.Context, generation string) (CandidateGeneration, error) {
	if err := ctx.Err(); err != nil {
		return CandidateGeneration{}, err
	}
	candidate, receipt, idx, err := a.verifyGenerationAndReceipt(generation, generation+".selector.json")
	if err != nil {
		return CandidateGeneration{}, err
	}
	return CandidateGeneration{CandidateBytes: candidate, Receipt: receipt, Published: PublishedGeneration{Generation: generation, VerificationSelector: generation + ".selector.json", ArtifactSelector: generation + "/artifact.json", ByteCustodyReceiptSelector: generation + "/receipt.json", QualificationReceiptSelector: idx.QualificationReceiptSelector, QualificationReceiptVerificationSelector: idx.VerificationSelector, QualificationReceiptDigest: idx.Digest, QualificationReceiptByteLength: idx.ByteLength, PublicationMechanism: publication.VerifiedGenerationMechanism}}, nil
}

func (a *Adapter) casCurrent(ctx context.Context, predecessor, initialPredecessor string, selectorBytes []byte) (*publication.BoundFileReceipt, error) {
	unlock, err := a.lockCurrent(ctx)
	if err != nil {
		return nil, err
	}
	defer unlock()
	var existing CurrentGeneration
	raw, readErr := a.root.ReadSelector(currentSelector, 4096)
	if readErr == nil {
		if err := decodeStrict(raw, &existing); err != nil {
			return nil, err
		}
		if err := a.validateCurrent(existing); err != nil {
			return nil, err
		}
		if predecessor != existing.Selector {
			return nil, errors.New("candidatepublication: stale predecessor selector")
		}
	} else if predecessor != initialPredecessor {
		return nil, errors.New("candidatepublication: initial predecessor mismatch")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	tempSelector := currentSelector + "." + fmt.Sprintf("%d", time.Now().UnixNano()) + ".candidate"
	receipt, err := publication.PublishBoundFile(a.root, tempSelector, selectorBytes, func(got []byte) error {
		var decoded CurrentGeneration
		if err := decodeStrict(got, &decoded); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	selectorPath := filepath.Join(a.root.Path(), filepath.FromSlash(currentSelector))
	tempPath := filepath.Join(a.root.Path(), filepath.FromSlash(tempSelector))
	if err := os.MkdirAll(filepath.Dir(selectorPath), 0o700); err != nil {
		return nil, err
	}
	if err := os.Rename(tempPath, selectorPath); err != nil {
		return nil, err
	}
	return receipt, nil
}

func (a *Adapter) lockCurrent(ctx context.Context) (func(), error) {
	lockPath := filepath.Join(a.root.Path(), filepath.FromSlash(currentLockSelector))
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o700); err != nil {
		return nil, err
	}
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			_ = f.Close()
			return func() { _ = os.Remove(lockPath) }, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, err
		}
		time.Sleep(time.Millisecond)
	}
}

func (a *Adapter) verifyGenerationAndReceipt(generation, verificationSelector string) ([]byte, Receipt, qualificationIndex, error) {
	candidate, err := a.verifyCandidateGeneration(generation, verificationSelector)
	if err != nil {
		return nil, Receipt{}, qualificationIndex{}, err
	}
	idx, err := a.readQualificationIndex(generation)
	if err != nil {
		return nil, Receipt{}, qualificationIndex{}, err
	}
	receipt, err := a.verifyQualificationBinding(candidate, generation, idx)
	if err != nil {
		return nil, Receipt{}, qualificationIndex{}, err
	}
	return candidate, receipt, idx, nil
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

func (a *Adapter) verifyQualificationBinding(candidate []byte, generation string, idx qualificationIndex) (Receipt, error) {
	receiptBytes, err := a.root.ReadSelector(idx.QualificationReceiptSelector, 1<<20)
	if err != nil {
		return Receipt{}, err
	}
	if Digest(receiptBytes) != idx.Digest || uint64(len(receiptBytes)) != idx.ByteLength {
		return Receipt{}, errors.New("candidatepublication: qualification receipt digest mismatch")
	}
	qSelectorBytes, err := a.root.ReadSelector(idx.VerificationSelector, 4096)
	if err != nil {
		return Receipt{}, err
	}
	qSelector, err := verification.DecodeSelector(qSelectorBytes)
	if err != nil || idx.QualificationReceiptSelector != qSelector.Generation+"/artifact.json" {
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

func (a *Adapter) writeQualificationIndex(idx qualificationIndex) error {
	raw, err := json.Marshal(idx)
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	path := filepath.Join(a.root.Path(), filepath.FromSlash(qualificationIndexSelector(idx.Generation)))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o600)
}
func (a *Adapter) readQualificationIndex(generation string) (qualificationIndex, error) {
	raw, err := a.root.ReadSelector(qualificationIndexSelector(generation), 4096)
	if err != nil {
		return qualificationIndex{}, err
	}
	var idx qualificationIndex
	if err := decodeStrict(raw, &idx); err != nil {
		return qualificationIndex{}, err
	}
	if idx.SchemaVersion == "" || idx.Generation != generation || idx.VerificationSelector == "" || idx.QualificationReceiptSelector == "" || idx.Digest == "" || idx.ByteLength == 0 {
		return qualificationIndex{}, errors.New("candidatepublication: invalid qualification index")
	}
	return idx, nil
}
func qualificationIndexSelector(generation string) string {
	return qualificationNamespace + "/" + strings.TrimPrefix(generation, "g-") + ".index.json"
}

func (a *Adapter) validateCurrent(current CurrentGeneration) error {
	if current.Selector != currentSelector || !isGeneration(current.Generation) || current.VerificationSelector != current.Generation+".selector.json" || current.CandidateDigest == "" || current.CandidateByteLength == 0 || current.QualificationReceiptSelector == "" || current.QualificationReceiptVerificationSelector == "" || current.QualificationReceiptDigest == "" || current.QualificationReceiptByteLength == 0 {
		return errors.New("candidatepublication: invalid current selector")
	}
	candidate, err := a.verifyCandidateGeneration(current.Generation, current.VerificationSelector)
	if err != nil {
		return err
	}
	if Digest(candidate) != current.CandidateDigest || uint64(len(candidate)) != current.CandidateByteLength {
		return errors.New("candidatepublication: current candidate binding mismatch")
	}
	receipt, err := a.verifyQualificationBinding(candidate, current.Generation, qualificationIndex{Generation: current.Generation, QualificationReceiptSelector: current.QualificationReceiptSelector, VerificationSelector: current.QualificationReceiptVerificationSelector, Digest: current.QualificationReceiptDigest, ByteLength: current.QualificationReceiptByteLength})
	if err != nil {
		return err
	}
	if receipt.CandidateDigest.Hex != current.CandidateDigest {
		return errors.New("candidatepublication: current selector binding mismatch")
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
