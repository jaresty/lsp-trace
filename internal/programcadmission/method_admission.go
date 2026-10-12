package programcadmission

import (
	"bytes"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strconv"
)

const (
	MethodEvidenceFamily          = "lsp-trace.private.program-c-method-evidence.v0"
	MethodEvidenceClaimCeiling    = "STRUCTURAL_AND_CUSTODY_BINDINGS_ONLY;AUTHORITY_0;ACCEPTED_FALSE;COMPLETENESS_UNKNOWN;NO_PRODUCER_AUTHENTICATION"
	MaxMethodOccurrences          = 100_000
	MaxMethodQueryBytes           = 16 << 20
	MaxMethodEvidenceBytes        = 16 << 20
	MaxMethodFieldBytes           = 4096
	MaxMethodCommitmentInputBytes = 16 << 20
	MaxMethodCanonicalBytes       = 64 << 20
)

var methodAdmissionCommitmentDomain = []byte("lsp-trace.private.program-c-method-admission.v1\x00")

var (
	ErrMethodEvidenceEnvelope          = errors.New("invalid method evidence envelope")
	ErrMethodEvidenceVersion           = errors.New("unsupported method evidence version")
	ErrMethodEvidenceClaim             = errors.New("invalid method evidence claim ceiling")
	ErrMethodEvidenceUnsupported       = errors.New("unsupported method evidence binding")
	ErrMethodEvidenceMalformedBinding  = errors.New("malformed method evidence binding")
	ErrMethodEvidenceDuplicateIdentity = errors.New("duplicate method evidence identity")
	ErrMethodEvidenceResourceBound     = errors.New("method evidence resource bound exceeded")
	ErrMethodEvidenceInvalidSelection  = errors.New("invalid method evidence selection")
	ErrMethodEvidenceIntegrity         = errors.New("invalid method evidence integrity")
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
	occurrences      []MethodOccurrenceEvidence
	canonicalBytes   []byte
	commitmentDigest [sha256.Size]byte
}

func (a MethodProjectionAdmission) Occurrences() []MethodOccurrenceEvidence {
	return append([]MethodOccurrenceEvidence(nil), a.occurrences...)
}
func (a MethodProjectionAdmission) CanonicalBytes() []byte {
	return append([]byte(nil), a.canonicalBytes...)
}
func (a MethodProjectionAdmission) CommitmentDigest() []byte {
	return append([]byte(nil), a.commitmentDigest[:]...)
}
func (MethodProjectionAdmission) ClaimCeiling() string { return MethodEvidenceClaimCeiling }
func (MethodProjectionAdmission) Authority() int       { return 0 }
func (MethodProjectionAdmission) Accepted() bool       { return false }
func (MethodProjectionAdmission) Completeness() string { return "UNKNOWN" }

func AdmitUntrustedMethodEvidenceEnvelope(in UntrustedMethodEvidenceEnvelope) (MethodProjectionAdmission, error) {
	if len(in.Occurrences) > MaxMethodOccurrences {
		return MethodProjectionAdmission{}, ErrMethodEvidenceResourceBound
	}
	if methodCommitmentInputBytes(in) > MaxMethodCommitmentInputBytes {
		return MethodProjectionAdmission{}, ErrMethodEvidenceResourceBound
	}
	if in.Family != MethodEvidenceFamily {
		return MethodProjectionAdmission{}, ErrMethodEvidenceEnvelope
	}
	if in.Version != MethodEvidenceFamily {
		return MethodProjectionAdmission{}, ErrMethodEvidenceVersion
	}
	if in.ClaimCeiling != MethodEvidenceClaimCeiling || in.Authority != 0 || in.Accepted || in.Completeness != "UNKNOWN" {
		return MethodProjectionAdmission{}, ErrMethodEvidenceClaim
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
	canonicalEnvelope := in
	canonicalEnvelope.Occurrences = append([]MethodOccurrenceEvidence(nil), out...)
	sort.Slice(canonicalEnvelope.Occurrences, func(i, j int) bool {
		return canonicalEnvelope.Occurrences[i].Identity < canonicalEnvelope.Occurrences[j].Identity
	})
	canonical, err := canonicalMethodEvidenceBytes(canonicalEnvelope, MaxMethodCanonicalBytes)
	if err != nil {
		return MethodProjectionAdmission{}, err
	}
	commitment := methodEvidenceCommitment(canonical)
	return MethodProjectionAdmission{occurrences: out, canonicalBytes: canonical, commitmentDigest: commitment}, nil
}

func methodCommitmentInputBytes(in UntrustedMethodEvidenceEnvelope) int {
	total := 0
	fields := []string{in.Family, in.Version, in.ClaimCeiling, in.Completeness, in.Provider, in.ProviderVersion, in.SessionID, in.Revision, in.Custody, in.PositionEncoding}
	for _, field := range fields {
		var exceeded bool
		total, exceeded = addBounded(total, len(field), MaxMethodCommitmentInputBytes)
		if exceeded {
			return total
		}
	}
	for _, o := range in.Occurrences {
		fields = []string{o.Identity, o.From, o.To, string(o.Kind), o.SourceRole, o.TargetRole, o.Method, o.QuerySelector, o.QueryDigest, o.EvidenceSelector, o.EvidenceDigest, o.Provider, o.ProviderVersion, o.SessionID, o.Revision, o.Custody, o.PositionEncoding}
		for _, field := range fields {
			var exceeded bool
			total, exceeded = addBounded(total, len(field), MaxMethodCommitmentInputBytes)
			if exceeded {
				return total
			}
		}
	}
	return total
}

func addBounded(total, increment, limit int) (int, bool) {
	if total < 0 || increment < 0 || limit < 0 || total > limit || increment > limit-total {
		return int(^uint(0) >> 1), true
	}
	return total + increment, false
}

type boundedMethodEvidenceWriter struct {
	bytes.Buffer
	limit int
}

func (w *boundedMethodEvidenceWriter) Write(p []byte) (int, error) {
	if len(p) > w.limit-w.Len() {
		return 0, ErrMethodEvidenceResourceBound
	}
	return w.Buffer.Write(p)
}

func canonicalMethodEvidenceBytes(in UntrustedMethodEvidenceEnvelope, limit int) ([]byte, error) {
	if limit < 0 {
		return nil, ErrMethodEvidenceResourceBound
	}
	writer := &boundedMethodEvidenceWriter{limit: limit}
	write := func(literal string) error {
		_, err := writer.Write([]byte(literal))
		return err
	}
	writeString := func(value string) error {
		if len(value) > MaxMethodFieldBytes {
			return ErrMethodEvidenceResourceBound
		}
		encoded, err := json.Marshal(value)
		if err != nil {
			return ErrMethodEvidenceIntegrity
		}
		_, err = writer.Write(encoded)
		return err
	}
	var scalar [32]byte
	writeInt := func(value int) error {
		encoded := strconv.AppendInt(scalar[:0], int64(value), 10)
		_, err := writer.Write(encoded)
		return err
	}
	writeUint := func(value uint64) error {
		encoded := strconv.AppendUint(scalar[:0], value, 10)
		_, err := writer.Write(encoded)
		return err
	}
	writeBool := func(value bool) error {
		encoded := strconv.AppendBool(scalar[:0], value)
		_, err := writer.Write(encoded)
		return err
	}
	stringField := func(name, value string) error {
		if err := write(name); err != nil {
			return err
		}
		return writeString(value)
	}
	intField := func(name string, value int) error {
		if err := write(name); err != nil {
			return err
		}
		return writeInt(value)
	}
	uintField := func(name string, value uint64) error {
		if err := write(name); err != nil {
			return err
		}
		return writeUint(value)
	}
	boolField := func(name string, value bool) error {
		if err := write(name); err != nil {
			return err
		}
		return writeBool(value)
	}

	if err := stringField(`{"Family":`, in.Family); err != nil {
		return nil, err
	}
	for _, field := range []struct {
		name  string
		value string
	}{
		{`,"Version":`, in.Version},
		{`,"ClaimCeiling":`, in.ClaimCeiling},
	} {
		if err := stringField(field.name, field.value); err != nil {
			return nil, err
		}
	}
	if err := intField(`,"Authority":`, in.Authority); err != nil {
		return nil, err
	}
	if err := boolField(`,"Accepted":`, in.Accepted); err != nil {
		return nil, err
	}
	for _, field := range []struct {
		name  string
		value string
	}{
		{`,"Completeness":`, in.Completeness},
		{`,"Provider":`, in.Provider},
		{`,"ProviderVersion":`, in.ProviderVersion},
		{`,"SessionID":`, in.SessionID},
	} {
		if err := stringField(field.name, field.value); err != nil {
			return nil, err
		}
	}
	if err := uintField(`,"Generation":`, in.Generation); err != nil {
		return nil, err
	}
	for _, field := range []struct {
		name  string
		value string
	}{
		{`,"Revision":`, in.Revision},
		{`,"Custody":`, in.Custody},
		{`,"PositionEncoding":`, in.PositionEncoding},
	} {
		if err := stringField(field.name, field.value); err != nil {
			return nil, err
		}
	}
	if in.Occurrences == nil {
		if err := write(`,"Occurrences":null}`); err != nil {
			return nil, err
		}
		return writer.Bytes(), nil
	}
	if err := write(`,"Occurrences":[`); err != nil {
		return nil, err
	}
	for i, occurrence := range in.Occurrences {
		if i > 0 {
			if err := write(","); err != nil {
				return nil, err
			}
		}
		if err := stringField(`{"Identity":`, occurrence.Identity); err != nil {
			return nil, err
		}
		for _, field := range []struct {
			name  string
			value string
		}{
			{`,"From":`, occurrence.From},
			{`,"To":`, occurrence.To},
			{`,"Kind":`, string(occurrence.Kind)},
		} {
			if err := stringField(field.name, field.value); err != nil {
				return nil, err
			}
		}
		if err := intField(`,"Ordinal":`, occurrence.Ordinal); err != nil {
			return nil, err
		}
		for _, field := range []struct {
			name  string
			value string
		}{
			{`,"SourceRole":`, occurrence.SourceRole},
			{`,"TargetRole":`, occurrence.TargetRole},
			{`,"Method":`, occurrence.Method},
			{`,"QuerySelector":`, occurrence.QuerySelector},
			{`,"QueryDigest":`, occurrence.QueryDigest},
		} {
			if err := stringField(field.name, field.value); err != nil {
				return nil, err
			}
		}
		if err := intField(`,"QueryLength":`, occurrence.QueryLength); err != nil {
			return nil, err
		}
		for _, field := range []struct {
			name  string
			value string
		}{
			{`,"EvidenceSelector":`, occurrence.EvidenceSelector},
			{`,"EvidenceDigest":`, occurrence.EvidenceDigest},
		} {
			if err := stringField(field.name, field.value); err != nil {
				return nil, err
			}
		}
		if err := intField(`,"EvidenceLength":`, occurrence.EvidenceLength); err != nil {
			return nil, err
		}
		for _, field := range []struct {
			name  string
			value string
		}{
			{`,"Provider":`, occurrence.Provider},
			{`,"ProviderVersion":`, occurrence.ProviderVersion},
			{`,"SessionID":`, occurrence.SessionID},
		} {
			if err := stringField(field.name, field.value); err != nil {
				return nil, err
			}
		}
		if err := uintField(`,"Generation":`, occurrence.Generation); err != nil {
			return nil, err
		}
		for _, field := range []struct {
			name  string
			value string
		}{
			{`,"Revision":`, occurrence.Revision},
			{`,"Custody":`, occurrence.Custody},
			{`,"PositionEncoding":`, occurrence.PositionEncoding},
		} {
			if err := stringField(field.name, field.value); err != nil {
				return nil, err
			}
		}
		if err := write("}"); err != nil {
			return nil, err
		}
	}
	if err := write(`]}`); err != nil {
		return nil, err
	}
	return writer.Bytes(), nil
}

func methodEvidenceCommitment(canonical []byte) [sha256.Size]byte {
	h := sha256.New()
	_, _ = h.Write(methodAdmissionCommitmentDomain[:len(methodAdmissionCommitmentDomain)-1])
	_, _ = h.Write([]byte{0})
	_, _ = h.Write(canonical)
	var commitment [sha256.Size]byte
	h.Sum(commitment[:0])
	return commitment
}

func validateMethodProjectionAdmission(in MethodProjectionAdmission) error {
	if len(in.canonicalBytes) == 0 || isZeroMethodCommitment(in.commitmentDigest) {
		return ErrMethodEvidenceIntegrity
	}
	want := methodEvidenceCommitment(in.canonicalBytes)
	if subtle.ConstantTimeCompare(want[:], in.commitmentDigest[:]) != 1 {
		return ErrMethodEvidenceIntegrity
	}
	var envelope UntrustedMethodEvidenceEnvelope
	decoder := json.NewDecoder(bytes.NewReader(in.canonicalBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&envelope); err != nil {
		return ErrMethodEvidenceIntegrity
	}
	replayed, err := AdmitUntrustedMethodEvidenceEnvelope(envelope)
	if err != nil || !bytes.Equal(replayed.canonicalBytes, in.canonicalBytes) || subtle.ConstantTimeCompare(replayed.commitmentDigest[:], in.commitmentDigest[:]) != 1 {
		return ErrMethodEvidenceIntegrity
	}
	wantOccurrences := append([]MethodOccurrenceEvidence(nil), in.occurrences...)
	sort.Slice(wantOccurrences, func(i, j int) bool { return wantOccurrences[i].Identity < wantOccurrences[j].Identity })
	if !reflect.DeepEqual(wantOccurrences, envelope.Occurrences) {
		return ErrMethodEvidenceIntegrity
	}
	return nil
}

func isZeroMethodCommitment(in [sha256.Size]byte) bool {
	var zero [sha256.Size]byte
	return subtle.ConstantTimeCompare(in[:], zero[:]) == 1
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
