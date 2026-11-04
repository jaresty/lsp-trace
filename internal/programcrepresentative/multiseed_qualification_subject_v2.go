package programcrepresentative

import (
	"bytes"
	"crypto/sha1"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"lsp-trace/internal/strictjson"
)

const QualificationSubjectSchemaVersionV2 = "lsp-trace.program-c-representative-qualification-subject.private.v2"
const AcceptedMultiSeedMechanicsAmendmentSHA256V2 = "sha256:839c71b4d74c0e3fa2fb6c6b49be85725b615754586bf40ab116f9a709f7d66e"

// ErrMultiSeedV2ReviewRequired is the planned, authority-preserving stop. It is
// not a substitute positive result or a claim that downstream integration ran.
var ErrMultiSeedV2ReviewRequired = errors.New("STOP_FOR_PARENT_REVIEW: exact multiseed v2 implementation revision and repository-owned reviewed manifest required")

type QualificationSubjectV2 struct {
	SchemaVersion                                    string                           `json:"schema_version"`
	Profile                                          string                           `json:"profile"`
	AcceptedADRSHA256                                string                           `json:"accepted_adr_sha256"`
	AcceptedReceiptDiscoveryAmendmentSHA256          string                           `json:"accepted_receipt_discovery_amendment_sha256"`
	AcceptedPrequalificationMechanicsAmendmentSHA256 string                           `json:"accepted_prequalification_mechanics_amendment_sha256"`
	AcceptedQualificationSubjectAmendmentSHA256      string                           `json:"accepted_qualification_subject_amendment_sha256"`
	AcceptedMultiseedMechanicsAmendmentSHA256        string                           `json:"accepted_multiseed_mechanics_amendment_sha256"`
	AcceptedDownstreamAmendmentSHA256                string                           `json:"accepted_downstream_amendment_sha256"`
	AdapterContract                                  AdapterContractDTOv2             `json:"adapter_contract"`
	MechanicsSchemaVersion                           string                           `json:"mechanics_schema_version"`
	MechanicsProfile                                 string                           `json:"mechanics_profile"`
	Stage6Implementation                             Stage6ImplementationDTO          `json:"stage6_implementation"`
	Stage6Policy                                     ArtifactBindingDTO               `json:"stage6_policy"`
	Stage6Limits                                     ArtifactBindingDTO               `json:"stage6_limits"`
	Stage6Finalization                               ArtifactBindingDTO               `json:"stage6_finalization"`
	CorpusIdentity                                   CorpusIdentityDTO                `json:"corpus_identity"`
	WorkflowIdentity                                 QualificationWorkflowIdentityDTO `json:"workflow_identity"`
	SourcePinSHA256                                  string                           `json:"source_pin_sha256"`
	ReceiptInventory                                 []ReceiptInventoryEntryDTO       `json:"receipt_inventory"`
	TerminalProgramCHead                             TerminalProgramCHeadDTO          `json:"terminal_program_c_head"`
	ClaimCeiling                                     QualificationClaimCeilingDTO     `json:"claim_ceiling"`
	Authority                                        uint8                            `json:"authority"`
	Accepted                                         bool                             `json:"accepted"`
	Identity                                         string                           `json:"identity"`
}

// ReadImplementationFileV2 must bound the read before allocation. The selected
// path and exact byte limit come only from the repository-owned declaration;
// the reader cannot supply a manifest or relax its pins.
type ImplementationSourceReaderV2 interface {
	ReadImplementationFileV2(path string, maxBytes uint64) ([]byte, error)
}
type QualificationSubjectExpectedContextV2 struct {
	AdapterContract              AdapterContractDTOv2
	Stage6Implementation         Stage6ImplementationDTO
	CorpusIdentity               CorpusIdentityDTO
	WorkflowIdentity             QualificationWorkflowIdentityDTO
	SourcePinSHA256              string
	ReceiptInventory             []ReceiptInventoryEntryDTO
	TerminalProgramCHead         TerminalProgramCHeadDTO
	MaxImplementationSourceBytes uint64
	ImplementationSource         ImplementationSourceReaderV2
}
type ProgramCRepresentativeQualificationSubjectV2 struct {
	dto           QualificationSubjectV2
	canonical     []byte
	reconstructed bool
}

func (s ProgramCRepresentativeQualificationSubjectV2) Identity() string { return s.dto.Identity }
func (s ProgramCRepresentativeQualificationSubjectV2) Trusted() bool    { return s.reconstructed }
func (s ProgramCRepresentativeQualificationSubjectV2) DTO() (QualificationSubjectV2, error) {
	if _, err := CanonicalSizeQualificationSubjectV2(s.dto); err != nil {
		return QualificationSubjectV2{}, err
	}
	return cloneQualificationSubjectV2(s.dto), nil
}
func (s ProgramCRepresentativeQualificationSubjectV2) CanonicalBytes() ([]byte, error) {
	if _, err := CanonicalSizeQualificationSubjectV2(s.dto); err != nil {
		return nil, err
	}
	return append([]byte{}, s.canonical...), nil
}

func BuildAdapterContractV2(algorithmVersion string) (AdapterContractDTOv2, error) {
	dto, err := newAdapterContractV2(algorithmVersion)
	if err != nil {
		return AdapterContractDTOv2{}, err
	}
	if _, err := CanonicalSizeAdapterContractDTOv2(dto); err != nil {
		return AdapterContractDTOv2{}, err
	}
	return dto, nil
}
func VerifyStage6ImplementationSourceV2(expected Stage6ImplementationDTO, reader ImplementationSourceReaderV2, maxSourceBytes uint64) error {
	declaration, err := BuildStage6ImplementationV2(maxSourceBytes)
	if err != nil {
		return err
	}
	return verifyStage6ImplementationSourceAgainstDeclarationV2(expected, declaration, reader, maxSourceBytes)
}
func verifyStage6ImplementationSourceAgainstDeclarationV2(expected, declaration Stage6ImplementationDTO, reader ImplementationSourceReaderV2, maxSourceBytes uint64) error {
	if _, err := CanonicalSizeStage6ImplementationDTO(declaration); err != nil {
		return err
	}
	if _, err := CanonicalSizeStage6ImplementationDTO(expected); err != nil {
		return err
	}
	if err := validateImplementationShapeV2(declaration); err != nil {
		return err
	}
	if maxSourceBytes == 0 {
		return fmt.Errorf("missing v2 implementation source bound")
	}
	if reader == nil || !equalJSON(expected, declaration) {
		return fmt.Errorf("v2 reviewed implementation declaration mismatch")
	}
	var used uint64
	for _, file := range declaration.Files {
		if file.ByteLength == 0 || used > maxSourceBytes || file.ByteLength > maxSourceBytes-used {
			return fmt.Errorf("v2 implementation source bound")
		}
		used += file.ByteLength
		raw, err := reader.ReadImplementationFileV2(file.Path, file.ByteLength)
		if err != nil {
			return err
		}
		if uint64(len(raw)) != file.ByteLength || rawDigest(raw) != file.SHA256 {
			return fmt.Errorf("v2 implementation source bytes mismatch")
		}
		h := sha1.New()
		fmt.Fprintf(h, "blob %d%c", len(raw), byte(0))
		h.Write(raw)
		if fmt.Sprintf("%x", h.Sum(nil)) != file.GitBlobSHA1 {
			return fmt.Errorf("v2 implementation Git blob mismatch")
		}
	}
	return nil
}

func NewProgramCRepresentativeQualificationSubjectV2(in QualificationSubjectExpectedContextV2) (ProgramCRepresentativeQualificationSubjectV2, error) {
	zero := ProgramCRepresentativeQualificationSubjectV2{}
	if err := VerifyStage6ImplementationSourceV2(in.Stage6Implementation, in.ImplementationSource, in.MaxImplementationSourceBytes); err != nil {
		return zero, err
	}
	return newQualificationSubjectV2FromExpected(in)
}

// This private constructor is reachable in production only after the fixed
// reviewed declaration/source check above; tests may exercise pure wire shape
// with their own local immutable source fixture, never as production anchors.
func newQualificationSubjectV2FromExpected(in QualificationSubjectExpectedContextV2) (ProgramCRepresentativeQualificationSubjectV2, error) {
	zero := ProgramCRepresentativeQualificationSubjectV2{}
	adapter, err := newAdapterContractV2(in.AdapterContract.ProgramCAlgorithmVersion)
	if err != nil || !equalJSON(adapter, in.AdapterContract) {
		return zero, fmt.Errorf("v2 subject adapter mismatch")
	}
	if err := validateCorpusWorkflow(in.CorpusIdentity, in.WorkflowIdentity, in.SourcePinSHA256); err != nil {
		return zero, err
	}
	if err := validateReceiptAndHeadV2(in.ReceiptInventory, in.TerminalProgramCHead); err != nil {
		return zero, err
	}
	_, policy, _, err := BuildStage6PolicyArtifactV2()
	if err != nil {
		return zero, err
	}
	_, limits, _, err := BuildStage6LimitsArtifactV2()
	if err != nil {
		return zero, err
	}
	_, finalization, _, err := BuildStage6FinalizationArtifactV2()
	if err != nil {
		return zero, err
	}
	dto := QualificationSubjectV2{SchemaVersion: QualificationSubjectSchemaVersionV2, Profile: ProfileCallsOnlyFrozenCensusV1, AcceptedADRSHA256: AcceptedADR0014SHA256, AcceptedReceiptDiscoveryAmendmentSHA256: AcceptedReceiptDiscoveryAmendmentSHA256, AcceptedPrequalificationMechanicsAmendmentSHA256: AcceptedPrequalificationMechanicsAmendmentSHA256, AcceptedQualificationSubjectAmendmentSHA256: AcceptedQualificationSubjectAmendmentSHA256, AcceptedMultiseedMechanicsAmendmentSHA256: AcceptedMultiSeedMechanicsAmendmentSHA256V2, AcceptedDownstreamAmendmentSHA256: AcceptedDownstreamAmendmentSHA256V2, AdapterContract: in.AdapterContract, MechanicsSchemaVersion: MechanicsSchemaVersionV2, MechanicsProfile: ProfileCallsOnlyFrozenCensusV1, Stage6Implementation: in.Stage6Implementation, Stage6Policy: policy, Stage6Limits: limits, Stage6Finalization: finalization, CorpusIdentity: in.CorpusIdentity, WorkflowIdentity: in.WorkflowIdentity, SourcePinSHA256: in.SourcePinSHA256, ReceiptInventory: in.ReceiptInventory, TerminalProgramCHead: in.TerminalProgramCHead, ClaimCeiling: fixedQualificationClaimCeiling()}
	n, err := CanonicalSizeQualificationSubjectV2(dto)
	pre, err := marshalSizedV2(dto, n, err)
	if err != nil {
		return zero, err
	}
	dto.Identity = multiSeedIdentityV2("lsp-trace:program-c-representative-qualification-subject:v2", pre)
	if err := validateQualificationSubjectV2(dto); err != nil {
		return zero, err
	}
	n, err = CanonicalSizeQualificationSubjectV2(dto)
	raw, err := marshalSizedV2(dto, n, err)
	if err != nil {
		return zero, err
	}
	return ProgramCRepresentativeQualificationSubjectV2{dto: cloneQualificationSubjectV2(dto), canonical: raw, reconstructed: true}, nil
}

func ParseProgramCRepresentativeQualificationSubjectV2(raw []byte) (ProgramCRepresentativeQualificationSubjectV2, error) {
	zero := ProgramCRepresentativeQualificationSubjectV2{}
	if uint64(len(raw)) > MaxArtifactCanonicalBytesV2 || !utf8.Valid(raw) {
		return zero, fmt.Errorf("invalid v2 subject byte admission")
	}
	if err := strictjson.RejectDuplicates(raw); err != nil {
		return zero, err
	}
	var dto QualificationSubjectV2
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&dto); err != nil {
		return zero, err
	}
	n, err := CanonicalSizeQualificationSubjectV2(dto)
	canonical, err := marshalSizedV2(dto, n, err)
	if err != nil || !bytes.Equal(raw, canonical) {
		return zero, fmt.Errorf("noncanonical v2 subject")
	}
	if err := validateQualificationSubjectV2(dto); err != nil {
		return zero, err
	}
	return ProgramCRepresentativeQualificationSubjectV2{dto: cloneQualificationSubjectV2(dto), canonical: append([]byte{}, raw...)}, nil
}
func validateQualificationSubjectV2(dto QualificationSubjectV2) error {
	if dto.SchemaVersion != QualificationSubjectSchemaVersionV2 || dto.Profile != ProfileCallsOnlyFrozenCensusV1 || dto.AcceptedADRSHA256 != AcceptedADR0014SHA256 || dto.AcceptedReceiptDiscoveryAmendmentSHA256 != AcceptedReceiptDiscoveryAmendmentSHA256 || dto.AcceptedPrequalificationMechanicsAmendmentSHA256 != AcceptedPrequalificationMechanicsAmendmentSHA256 || dto.AcceptedQualificationSubjectAmendmentSHA256 != AcceptedQualificationSubjectAmendmentSHA256 || dto.AcceptedMultiseedMechanicsAmendmentSHA256 != AcceptedMultiSeedMechanicsAmendmentSHA256V2 || dto.AcceptedDownstreamAmendmentSHA256 != AcceptedDownstreamAmendmentSHA256V2 || dto.MechanicsSchemaVersion != MechanicsSchemaVersionV2 || dto.MechanicsProfile != ProfileCallsOnlyFrozenCensusV1 || dto.Authority != 0 || dto.Accepted || dto.ClaimCeiling != fixedQualificationClaimCeiling() {
		return fmt.Errorf("invalid v2 subject envelope")
	}
	adapter, err := newAdapterContractV2(dto.AdapterContract.ProgramCAlgorithmVersion)
	if err != nil || !equalJSON(adapter, dto.AdapterContract) {
		return fmt.Errorf("invalid v2 subject adapter")
	}
	impl := dto.Stage6Implementation
	impl.Identity = ""
	n, err := CanonicalSizeStage6ImplementationDTO(impl)
	pre, err := marshalSizedV2(impl, n, err)
	if err != nil || !validHexV2(impl.RepositoryCommit, 40) || !validHexV2(impl.RepositoryTree, 40) || len(impl.Files) == 0 || dto.Stage6Implementation.Identity != multiSeedIdentityV2("lsp-trace:program-c-representative-stage6-implementation:v2", pre) {
		return fmt.Errorf("invalid v2 implementation identity")
	}
	for i, f := range impl.Files {
		if f.Path == "" || strings.HasPrefix(f.Path, "/") || strings.Contains(f.Path, "..") || !validHexV2(f.GitBlobSHA1, 40) || !validDigest(f.SHA256) || f.ByteLength == 0 || i > 0 && impl.Files[i-1].Path >= f.Path {
			return fmt.Errorf("invalid v2 implementation source pin")
		}
	}
	if err := validateCorpusWorkflow(dto.CorpusIdentity, dto.WorkflowIdentity, dto.SourcePinSHA256); err != nil {
		return err
	}
	if err := validateReceiptAndHeadV2(dto.ReceiptInventory, dto.TerminalProgramCHead); err != nil {
		return err
	}
	_, p, _, err := BuildStage6PolicyArtifactV2()
	if err != nil {
		return err
	}
	_, l, _, err := BuildStage6LimitsArtifactV2()
	if err != nil {
		return err
	}
	_, f, _, err := BuildStage6FinalizationArtifactV2()
	if err != nil {
		return err
	}
	if dto.Stage6Policy != p || dto.Stage6Limits != l || dto.Stage6Finalization != f {
		return fmt.Errorf("v2 subject fixed artifacts mismatch")
	}
	root := dto
	root.Identity = ""
	n, err = CanonicalSizeQualificationSubjectV2(root)
	pre, err = marshalSizedV2(root, n, err)
	if err != nil || dto.Identity != multiSeedIdentityV2("lsp-trace:program-c-representative-qualification-subject:v2", pre) {
		return fmt.Errorf("v2 subject identity mismatch")
	}
	return nil
}
func validateImplementationShapeV2(implementation Stage6ImplementationDTO) error {
	root := implementation
	root.Identity = ""
	n, err := CanonicalSizeStage6ImplementationDTO(root)
	raw, err := marshalSizedV2(root, n, err)
	if err != nil || !validHexV2(root.RepositoryCommit, 40) || !validHexV2(root.RepositoryTree, 40) || len(root.Files) == 0 || implementation.Identity != multiSeedIdentityV2("lsp-trace:program-c-representative-stage6-implementation:v2", raw) {
		return fmt.Errorf("invalid v2 implementation declaration")
	}
	for i, f := range root.Files {
		if f.Path == "" || strings.HasPrefix(f.Path, "/") || strings.Contains(f.Path, "..") || !validHexV2(f.GitBlobSHA1, 40) || !validDigest(f.SHA256) || f.ByteLength == 0 || i > 0 && root.Files[i-1].Path >= f.Path {
			return fmt.Errorf("invalid v2 implementation file declaration")
		}
	}
	return nil
}

func validHexV2(value string, n int) bool {
	if len(value) != n {
		return false
	}
	for _, r := range value {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f') {
			return false
		}
	}
	return true
}
func cloneQualificationSubjectV2(in QualificationSubjectV2) QualificationSubjectV2 {
	out := in
	out.AdapterContract.Domains = append([]string{}, in.AdapterContract.Domains...)
	out.AdapterContract.RunSeeds = append([]uint64{}, in.AdapterContract.RunSeeds...)
	out.AdapterContract.MembershipKinds = append([]string{}, in.AdapterContract.MembershipKinds...)
	out.AdapterContract.MappedMembershipKinds = append([]string{}, in.AdapterContract.MappedMembershipKinds...)
	out.Stage6Implementation.Files = append([]ImplementationFileDTO{}, in.Stage6Implementation.Files...)
	out.ReceiptInventory = append([]ReceiptInventoryEntryDTO{}, in.ReceiptInventory...)
	return out
}
