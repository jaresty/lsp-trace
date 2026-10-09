package programcadmission

import (
	"errors"
	"fmt"
	"regexp"
)

const (
	MethodEvidenceFamily       = "lsp-trace.private.program-c-method-evidence.v0"
	MethodEvidenceClaimCeiling = "STRUCTURAL_AND_CUSTODY_BINDINGS_ONLY;AUTHORITY_0;ACCEPTED_FALSE;COMPLETENESS_UNKNOWN;NO_PRODUCER_AUTHENTICATION"
	MaxMethodOccurrences       = 100_000
	MaxMethodQueryBytes        = 16 << 20
	MaxMethodEvidenceBytes     = 16 << 20
	MaxMethodFieldBytes        = 4096
)

var (
	ErrMethodEvidenceEnvelope          = errors.New("invalid method evidence envelope")
	ErrMethodEvidenceVersion           = errors.New("unsupported method evidence version")
	ErrMethodEvidenceClaim             = errors.New("invalid method evidence claim ceiling")
	ErrMethodEvidenceUnsupported       = errors.New("unsupported method evidence binding")
	ErrMethodEvidenceMalformedBinding  = errors.New("malformed method evidence binding")
	ErrMethodEvidenceDuplicateIdentity = errors.New("duplicate method evidence identity")
	ErrMethodEvidenceResourceBound     = errors.New("method evidence resource bound exceeded")
	ErrMethodEvidenceInvalidSelection  = errors.New("invalid method evidence selection")
)

var canonicalMethodDigest = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

type MethodOccurrenceEvidence struct {
	Identity, From, To                   string
	Kind                                 RelationKind
	Ordinal                              int
	SourceRole, TargetRole, Method       string
	QuerySelector, QueryDigest           string
	QueryLength                          int
	EvidenceSelector, EvidenceDigest     string
	EvidenceLength                       int
	Provider, ProviderVersion, SessionID string
	Generation                           uint64
	Revision, Custody, PositionEncoding  string
}

type UntrustedMethodEvidenceEnvelope struct {
	Family, Version, ClaimCeiling        string
	Authority                            int
	Accepted                             bool
	Completeness                         string
	Provider, ProviderVersion, SessionID string
	Generation                           uint64
	Revision, Custody, PositionEncoding  string
	Occurrences                          []MethodOccurrenceEvidence
}

type MethodProjectionAdmission struct {
	occurrences []MethodOccurrenceEvidence
}

func (a MethodProjectionAdmission) Occurrences() []MethodOccurrenceEvidence {
	return append([]MethodOccurrenceEvidence(nil), a.occurrences...)
}
func (MethodProjectionAdmission) ClaimCeiling() string { return MethodEvidenceClaimCeiling }
func (MethodProjectionAdmission) Authority() int       { return 0 }
func (MethodProjectionAdmission) Accepted() bool       { return false }
func (MethodProjectionAdmission) Completeness() string { return "UNKNOWN" }

func AdmitUntrustedMethodEvidenceEnvelope(in UntrustedMethodEvidenceEnvelope) (MethodProjectionAdmission, error) {
	if in.Family != MethodEvidenceFamily {
		return MethodProjectionAdmission{}, ErrMethodEvidenceEnvelope
	}
	if in.Version != MethodEvidenceFamily {
		return MethodProjectionAdmission{}, ErrMethodEvidenceVersion
	}
	if in.ClaimCeiling != MethodEvidenceClaimCeiling || in.Authority != 0 || in.Accepted || in.Completeness != "UNKNOWN" {
		return MethodProjectionAdmission{}, ErrMethodEvidenceClaim
	}
	if len(in.Occurrences) > MaxMethodOccurrences {
		return MethodProjectionAdmission{}, ErrMethodEvidenceResourceBound
	}
	if err := validateEnvelopeCustody(in); err != nil {
		return MethodProjectionAdmission{}, err
	}
	seen := make(map[string]struct{}, len(in.Occurrences))
	out := make([]MethodOccurrenceEvidence, len(in.Occurrences))
	for i, occurrence := range in.Occurrences {
		if err := validateMethodOccurrence(in, occurrence); err != nil {
			return MethodProjectionAdmission{}, err
		}
		if _, exists := seen[occurrence.Identity]; exists {
			return MethodProjectionAdmission{}, ErrMethodEvidenceDuplicateIdentity
		}
		seen[occurrence.Identity] = struct{}{}
		out[i] = occurrence
	}
	return MethodProjectionAdmission{occurrences: out}, nil
}

func validateEnvelopeCustody(in UntrustedMethodEvidenceEnvelope) error {
	fields := []string{in.Provider, in.ProviderVersion, in.SessionID, in.Revision, in.Custody, in.PositionEncoding}
	for _, field := range fields {
		if field == "" {
			return ErrMethodEvidenceMalformedBinding
		}
		if len(field) > MaxMethodFieldBytes {
			return ErrMethodEvidenceResourceBound
		}
	}
	if in.Generation == 0 {
		return ErrMethodEvidenceMalformedBinding
	}
	return nil
}

func validateMethodOccurrence(envelope UntrustedMethodEvidenceEnvelope, o MethodOccurrenceEvidence) error {
	if o.Kind != TypedReferencesSymbol && o.Kind != TypedResolvesToDefinition {
		return ErrMethodEvidenceUnsupported
	}
	method, sourceRole, targetRole := "textDocument/references", "REFERENCING_OCCURRENCE", "REFERENCED_SYMBOL"
	if o.Kind == TypedResolvesToDefinition {
		method, sourceRole, targetRole = "textDocument/definition", "QUERY_SYMBOL", "DEFINITION_TARGET"
	}
	if o.Method != method {
		return ErrMethodEvidenceUnsupported
	}
	if o.SourceRole != sourceRole || o.TargetRole != targetRole || o.Identity == "" || o.From == "" || o.To == "" || o.Ordinal < 0 || o.QuerySelector == "" || o.EvidenceSelector == "" || o.QueryLength <= 0 || o.EvidenceLength <= 0 || !canonicalMethodDigest.MatchString(o.QueryDigest) || !canonicalMethodDigest.MatchString(o.EvidenceDigest) {
		return ErrMethodEvidenceMalformedBinding
	}
	text := []string{o.Identity, o.From, o.To, o.SourceRole, o.TargetRole, o.Method, o.QuerySelector, o.QueryDigest, o.EvidenceSelector, o.EvidenceDigest, o.Provider, o.ProviderVersion, o.SessionID, o.Revision, o.Custody, o.PositionEncoding}
	for _, field := range text {
		if len(field) > MaxMethodFieldBytes {
			return ErrMethodEvidenceResourceBound
		}
	}
	if o.QueryLength > MaxMethodQueryBytes || o.EvidenceLength > MaxMethodEvidenceBytes {
		return ErrMethodEvidenceResourceBound
	}
	if o.Provider != envelope.Provider || o.ProviderVersion != envelope.ProviderVersion || o.SessionID != envelope.SessionID || o.Generation != envelope.Generation || o.Revision != envelope.Revision || o.Custody != envelope.Custody || o.PositionEncoding != envelope.PositionEncoding {
		return fmt.Errorf("%w: occurrence custody differs from envelope", ErrMethodEvidenceMalformedBinding)
	}
	return nil
}
