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
	"strconv"
	"strings"

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

	PostcommitVerificationFailed = "COMMITTED_VERIFICATION_FAILED"
)

type Options struct {
	MaxBytes int64
}

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
	Selector                     string
	Generation                   string
	VerificationSelector         string
	ArtifactSelector             string
	ByteCustodyReceiptSelector   string
	QualificationReceiptSelector string
	PublicationMechanism         string
	Committed                    bool
	PostcommitVerificationStatus string
}

type AdvanceRequest struct {
	Generation           string
	VerificationSelector string
	PredecessorSelector  string
}

type CurrentGeneration struct {
	Selector                     string `json:"selector"`
	Generation                   string `json:"generation"`
	VerificationSelector         string `json:"verification_selector"`
	QualificationReceiptSelector string `json:"qualification_receipt_selector"`
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

func NewRepositoryPrivateAdapter(root *publication.Root, opts Options) (*Adapter, error) {
	if root == nil || opts.MaxBytes < 1 {
		return nil, errors.New("candidatepublication: invalid adapter configuration")
	}
	if err := root.ValidatePrivate(); err != nil {
		return nil, err
	}
	return &Adapter{root: root, publisher: publication.NewPublisher(), maxBytes: opts.MaxBytes}, nil
}

func DigestBytes(raw []byte) []byte {
	sum := sha256.Sum256(raw)
	return sum[:]
}

func Digest(raw []byte) string { return "sha256:" + hex.EncodeToString(DigestBytes(raw)) }

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
	out := PublishedGeneration{
		Generation:                   generation,
		VerificationSelector:         published.Receipt.VerificationSelector,
		ArtifactSelector:             generation + "/artifact.json",
		ByteCustodyReceiptSelector:   generation + "/receipt.json",
		QualificationReceiptSelector: qualificationReceiptSelector(generation),
		PublicationMechanism:         published.Receipt.PublicationMechanism,
	}
	receipt, err := qualificationReceipt(req, generation)
	if err != nil {
		return PublishedGeneration{}, err
	}
	rawReceipt, err := json.Marshal(receipt)
	if err != nil {
		return PublishedGeneration{}, err
	}
	rawReceipt = append(rawReceipt, '\n')
	if err := ctx.Err(); err != nil {
		return PublishedGeneration{}, err
	}
	if err := a.publishImmutable(out.QualificationReceiptSelector, rawReceipt, qualificationReceiptSchema); err != nil {
		return PublishedGeneration{}, err
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
	candidate, receipt, err := a.verifyGenerationAndReceipt(req.Generation, req.VerificationSelector)
	if err != nil {
		return PublishedGeneration{}, err
	}
	if receipt.PredecessorSelector != req.PredecessorSelector {
		return PublishedGeneration{}, errors.New("candidatepublication: predecessor selector mismatch")
	}
	selector := CurrentGeneration{Selector: currentSelector, Generation: req.Generation, VerificationSelector: req.VerificationSelector, QualificationReceiptSelector: qualificationReceiptSelector(req.Generation)}
	selectorBytes, err := json.Marshal(selector)
	if err != nil {
		return PublishedGeneration{}, err
	}
	selectorBytes = append(selectorBytes, '\n')
	if err := ctx.Err(); err != nil {
		return PublishedGeneration{}, err
	}
	boundReceipt, err := publication.PublishBoundFile(a.root, currentSelector, selectorBytes, func(got []byte) error {
		var decoded CurrentGeneration
		if err := decodeStrict(got, &decoded); err != nil {
			return err
		}
		if decoded.Generation != req.Generation || decoded.VerificationSelector != req.VerificationSelector || decoded.QualificationReceiptSelector != qualificationReceiptSelector(req.Generation) {
			return errors.New("candidatepublication: current selector verification mismatch")
		}
		return nil
	})
	if err != nil {
		return PublishedGeneration{}, err
	}
	_ = candidate
	return PublishedGeneration{Selector: currentSelector, Generation: req.Generation, VerificationSelector: req.VerificationSelector, ArtifactSelector: req.Generation + "/artifact.json", ByteCustodyReceiptSelector: req.Generation + "/receipt.json", QualificationReceiptSelector: qualificationReceiptSelector(req.Generation), PublicationMechanism: publication.VerifiedGenerationMechanism, Committed: true, PostcommitVerificationStatus: boundReceipt.VerificationStatus}, nil
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
	return current, nil
}

func (a *Adapter) GetCandidateGeneration(ctx context.Context, generation string) (CandidateGeneration, error) {
	if err := ctx.Err(); err != nil {
		return CandidateGeneration{}, err
	}
	candidate, receipt, err := a.verifyGenerationAndReceipt(generation, generation+".selector.json")
	if err != nil {
		return CandidateGeneration{}, err
	}
	return CandidateGeneration{CandidateBytes: candidate, Receipt: receipt, Published: PublishedGeneration{Generation: generation, VerificationSelector: generation + ".selector.json", ArtifactSelector: generation + "/artifact.json", ByteCustodyReceiptSelector: generation + "/receipt.json", QualificationReceiptSelector: qualificationReceiptSelector(generation), PublicationMechanism: publication.VerifiedGenerationMechanism}}, nil
}

func (a *Adapter) publishImmutable(selector string, raw []byte, schema string) error {
	result := a.publisher.Publish(publication.Request{Root: a.root, Selector: selector, Bytes: append([]byte(nil), raw...), ArtifactSchemaID: schema})
	if result.Failure == nil {
		if result.Receipt == nil || result.Receipt.Digest != Digest(raw) || result.Receipt.ByteLength != uint64(len(raw)) {
			return errors.New("candidatepublication: receipt publication verification failed")
		}
		return nil
	}
	if result.Failure.Code != publication.CodeTargetExists {
		return result.Failure
	}
	existing, err := a.root.ReadSelector(selector, int64(len(raw))+1)
	if err != nil || !bytes.Equal(existing, raw) {
		return errors.New("candidatepublication: immutable collision")
	}
	return nil
}

func (a *Adapter) verifyGenerationAndReceipt(generation, verificationSelector string) ([]byte, Receipt, error) {
	if !isGeneration(generation) || verificationSelector != generation+".selector.json" {
		return nil, Receipt{}, errors.New("candidatepublication: invalid generation selector")
	}
	candidate, err := a.root.ReadSelector(generation+"/artifact.json", a.maxBytes)
	if err != nil {
		return nil, Receipt{}, err
	}
	if Digest(candidate) != "sha256:"+strings.TrimPrefix(generation, "g-") {
		return nil, Receipt{}, errors.New("candidatepublication: generation artifact digest mismatch")
	}
	byteReceipt, err := a.root.ReadSelector(generation+"/receipt.json", 1<<20)
	if err != nil || verification.VerifyReceipt(candidate, byteReceipt) != nil {
		return nil, Receipt{}, errors.New("candidatepublication: byte custody receipt verification failed")
	}
	selectorBytes, err := a.root.ReadSelector(verificationSelector, 4096)
	if err != nil {
		return nil, Receipt{}, err
	}
	selector, err := verification.DecodeSelector(selectorBytes)
	if err != nil || selector.Generation != generation {
		return nil, Receipt{}, errors.New("candidatepublication: generation selector verification failed")
	}
	receiptBytes, err := a.root.ReadSelector(qualificationReceiptSelector(generation), 1<<20)
	if err != nil {
		return nil, Receipt{}, err
	}
	var receipt Receipt
	if err := DecodeReceiptStrict(receiptBytes, &receipt); err != nil {
		return nil, Receipt{}, err
	}
	if err := verifyQualificationReceipt(candidate, generation, receipt); err != nil {
		return nil, Receipt{}, err
	}
	return candidate, receipt, nil
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

func qualificationReceiptSelector(generation string) string {
	return qualificationNamespace + "/" + strings.TrimPrefix(generation, "g-") + ".json"
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
	if dec.More() {
		return errors.New("candidatepublication: trailing json")
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
			// A string immediately followed by ':' is an object key. json.Decoder does
			// not expose that state, so inspect the next non-space byte at InputOffset.
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

func _strconvKeep() { _ = strconv.IntSize }
