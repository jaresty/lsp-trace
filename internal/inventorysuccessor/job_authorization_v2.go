package inventorysuccessor

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"math/bits"
	"unicode/utf8"
)

const (
	jobSchemaV2           = "lsp-trace.inventory-job-identity.private.v2"
	authorizationSchemaV2 = "lsp-trace.inventory-job-authorization.private.v2"
	reservationSchemaV2   = "lsp-trace.inventory-job-reservation.private.v2"
	resourceHeadSchemaV2  = "lsp-trace.inventory-job-head.private.v2"
	noneV2                = "NONE"
	assuranceV2           = "CALLER_ASSERTED_NOT_AUTHENTICATED"
	recordLimitV2         = 16 << 10
	headLimitV2           = 64 << 10
)

// countersV2 mirrors the fourteen named ApplicationBounds policy currencies
// without importing the application runtime. These are logical reservations,
// never measured CPU/RAM/IO, money, or a second charge for prepaid children.
type countersV2 struct {
	LogicalBytes     uint64 `json:"logical_bytes"`
	ReadHashBytes    uint64 `json:"read_hash_bytes"`
	WriteBytes       uint64 `json:"write_bytes"`
	Records          uint64 `json:"records"`
	ParserBytes      uint64 `json:"parser_bytes"`
	ProviderRequests uint64 `json:"provider_requests"`
	ProviderMessages uint64 `json:"provider_messages"`
	Work             uint64 `json:"work"`
	ActiveNS         uint64 `json:"active_ns"`
	Calls            uint64 `json:"calls"`
	RequestBytes     uint64 `json:"request_bytes"`
	ResponseBytes    uint64 `json:"response_bytes"`
	WireBytes        uint64 `json:"wire_bytes"`
	OutputBytes      uint64 `json:"output_bytes"`
}

type jobV2 struct {
	Schema              string   `json:"schema"`
	Identity            string   `json:"identity"`
	StableID            string   `json:"stable_id"`
	Application         string   `json:"application"`
	Scope               string   `json:"scope"`
	SourceAuthorization string   `json:"source_authorization"`
	RevisionPolicy      string   `json:"revision_policy"`
	EvidencePolicies    []string `json:"evidence_policies"`
	ProviderPolicies    []string `json:"provider_policies"`
	SemanticPolicies    []string `json:"semantic_policies"`
	Privacy             string   `json:"privacy"`
	Retention           string   `json:"retention"`
	ResourceRules       string   `json:"resource_rules"`
	Origin              string   `json:"origin"`
}

type authorizationV2 struct {
	Schema      string     `json:"schema"`
	Identity    string     `json:"identity"`
	Job         string     `json:"job"`
	Revision    uint64     `json:"revision"`
	Previous    string     `json:"previous_authorization"`
	BoundHead   string     `json:"verified_resource_head"`
	Charged     countersV2 `json:"charged_snapshot"`
	Ceiling     countersV2 `json:"currency_ceiling"`
	MaxWindows  uint64     `json:"max_windows"`
	Expires     int64      `json:"expires_unix_nano"`
	MaxTranche  countersV2 `json:"max_tranche"`
	MaxWindowNS uint64     `json:"max_window_ns"`
	Decision    string     `json:"caller_decision_reference"`
	Assurance   string     `json:"identity_assurance"`
}

type reservationV2 struct {
	Schema        string     `json:"schema"`
	Identity      string     `json:"identity"`
	Job           string     `json:"job"`
	Authorization string     `json:"authorization"`
	Revision      uint64     `json:"authorization_revision"`
	PreviousHead  string     `json:"predecessor_resource_head"`
	Sequence      uint64     `json:"sequence"`
	Attempt       string     `json:"original_attempt"`
	Phase         string     `json:"phase_kind"`
	Tranche       countersV2 `json:"requested_tranche"`
	Charged       countersV2 `json:"cumulative_charged"`
	ClaimedAt     int64      `json:"claimed_at_unix_nano"`
	Deadline      int64      `json:"window_deadline_unix_nano"`
	Disposition   string     `json:"disposition"`
}

// decisionV2 is an explicit trusted-parent API assertion, not a JSON flag and
// not cryptographic authentication. This package has no credential service.
type decisionV2 struct{ Job, Authorization, Reference string }

func hashLabelV2(s string) string {
	h := sha256.Sum256([]byte(s))
	return "sha256:" + hex.EncodeToString(h[:])
}

var errRecordV2 = errors.New("invalid private v2 resource record")

func metaV2(v any) (string, *string, int) {
	switch x := v.(type) {
	case *jobV2:
		return x.Schema, &x.Identity, recordLimitV2
	case *authorizationV2:
		return x.Schema, &x.Identity, recordLimitV2
	case *reservationV2:
		return x.Schema, &x.Identity, recordLimitV2
	case *resourceHeadV2:
		return x.Schema, &x.Identity, headLimitV2
	default:
		return "", nil, 0
	}
}

// sealV2 never normalizes a policy set or silently repairs semantic fields.
// Field order is the declaration order of these closed structs. The seal domain
// and schema precede canonical JSON with identity empty, INCLUDING its LF.
func sealV2(v any) ([]byte, error) {
	return sealWithMarshalV2(v, json.Marshal)
}

// marshal is invocation-local so refusal-before-serialization can be observed
// without measuring heap use or installing a shared mutable test hook.
func sealWithMarshalV2(v any, marshal func(any) ([]byte, error)) ([]byte, error) {
	schema, id, limit := metaV2(v)
	if id == nil || marshal == nil {
		return nil, errRecordV2
	}
	// Bound typed producer fields before serialization as well as bounded raw
	// decoding. Refusal must not serialize or mutate an unadmitted record.
	if err := validateRecordV2(v); err != nil {
		return nil, err
	}
	*id = ""
	unsigned, err := marshal(v)
	if err != nil {
		return nil, errRecordV2
	}
	unsigned = append(unsigned, '\n')
	*id = hashLabelV2("inventory-successor/private/v2\x00" + schema + "\x00" + string(unsigned))
	if err = validateRecordV2(v); err != nil {
		return nil, err
	}
	raw, err := marshal(v)
	if err != nil || len(raw)+1 > limit {
		return nil, errRecordV2
	}
	return append(raw, '\n'), nil
}

func parseV2(raw []byte, v any) error {
	_, id, limit := metaV2(v)
	if id == nil || len(raw) == 0 || len(raw) > limit || !utf8.Valid(raw) {
		return errRecordV2
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(v) != nil {
		return errRecordV2
	}
	// Exact re-encoding also rejects duplicates, aliases, absent fields, trailing
	// JSON, reordered keys, whitespace, escaped aliases and noncanonical numbers.
	encoded, err := sealV2(v)
	if err != nil || !bytes.Equal(encoded, raw) {
		return errRecordV2
	}
	return nil
}
func hashV2(s string) bool { return sha256Pattern.MatchString(s) }
func linkV2(s string) bool { return s == noneV2 || hashV2(s) }
func setV2(v []string) bool {
	if len(v) == 0 || len(v) > 32 {
		return false
	}
	for i, s := range v {
		if !hashV2(s) || (i > 0 && v[i-1] >= s) {
			return false
		}
	}
	return true
}
func (c countersV2) vector() [14]uint64 {
	return [14]uint64{c.LogicalBytes, c.ReadHashBytes, c.WriteBytes, c.Records, c.ParserBytes, c.ProviderRequests, c.ProviderMessages, c.Work, c.ActiveNS, c.Calls, c.RequestBytes, c.ResponseBytes, c.WireBytes, c.OutputBytes}
}
func countersFromV2(v [14]uint64) countersV2 {
	return countersV2{v[0], v[1], v[2], v[3], v[4], v[5], v[6], v[7], v[8], v[9], v[10], v[11], v[12], v[13]}
}
func (c countersV2) within(cap countersV2) bool {
	a, b := c.vector(), cap.vector()
	for i := range a {
		if a[i] > b[i] {
			return false
		}
	}
	return true
}
func addCountersV2(a, b countersV2) (countersV2, error) {
	x, y := a.vector(), b.vector()
	for i := range x {
		s, carry := bits.Add64(x[i], y[i], 0)
		if carry != 0 {
			return countersV2{}, errRecordV2
		}
		x[i] = s
	}
	return countersFromV2(x), nil
}
func nextV2(n uint64) (uint64, error) {
	if n == math.MaxUint64 {
		return 0, errRecordV2
	}
	return n + 1, nil
}
func validateRecordV2(v any) error {
	switch x := v.(type) {
	case *jobV2:
		if x.Schema != jobSchemaV2 || x.Origin != "NEW_EMPTY" || !setV2(x.EvidencePolicies) || !setV2(x.ProviderPolicies) || !setV2(x.SemanticPolicies) {
			return errRecordV2
		}
		for _, s := range []string{x.StableID, x.Application, x.Scope, x.SourceAuthorization, x.RevisionPolicy, x.Privacy, x.Retention, x.ResourceRules} {
			if !hashV2(s) {
				return errRecordV2
			}
		}
	case *authorizationV2:
		if x.Schema != authorizationSchemaV2 || !hashV2(x.Job) || x.Revision == 0 || !linkV2(x.Previous) || !linkV2(x.BoundHead) || x.MaxWindows == 0 || x.Expires <= 0 || x.MaxWindowNS == 0 || x.MaxWindowNS > math.MaxInt64 || !hashV2(x.Decision) || x.Assurance != assuranceV2 || !x.Charged.within(x.Ceiling) || !x.MaxTranche.within(x.Ceiling) {
			return errRecordV2
		}
		if x.Revision == 1 {
			if x.Previous != noneV2 || x.BoundHead != noneV2 || x.Charged != (countersV2{}) {
				return errRecordV2
			}
		} else if !hashV2(x.Previous) || !hashV2(x.BoundHead) {
			return errRecordV2
		}
	case *reservationV2:
		if x.Schema != reservationSchemaV2 || !hashV2(x.Job) || !hashV2(x.Authorization) || x.Revision == 0 || !hashV2(x.PreviousHead) || x.Sequence < 2 || !hashV2(x.Attempt) || x.Phase != "RESOURCE_WINDOW" || x.Tranche == (countersV2{}) || x.ClaimedAt <= 0 || x.Deadline <= x.ClaimedAt || x.Disposition != "PENDING" || !x.Tranche.within(x.Charged) {
			return errRecordV2
		}
	case *resourceHeadV2:
		return validateResourceHeadV2(*x)
	default:
		return errRecordV2
	}
	return nil
}
