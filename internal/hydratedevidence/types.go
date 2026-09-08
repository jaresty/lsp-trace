// Package hydratedevidence implements internal-only offline retained-source views.
// It performs no acquisition, publication, containing-construct discovery or
// independent authentication. Native and caller-asserted authority never merge.
package hydratedevidence

const Version = "lsp-trace.hydrated-evidence.v1"
const SidecarVersion = "lsp-trace.hydrated-sidecar.v1"
const Native = "NATIVE_RETAINED"
const Caller = "CALLER_ASSERTED"
const NonAuthoritative = "NON_AUTHORITATIVE"

// Input holds exact admitted bytes. Sidecars bind Digest(Artifact), not a URI.
type Input struct {
	Artifact []byte
	Sidecars [][]byte
}
type Policy struct {
	IncludeBodies         bool `json:"include_bodies"`
	AllowCallerBoundaries bool `json:"allow_caller_boundaries"`
	MaxInputBytes         int  `json:"max_input_bytes"`
	MaxOutputBytes        int  `json:"max_output_bytes"`
	MaxBodyBytes          int  `json:"max_body_bytes"`
	MaxSpans              int  `json:"max_spans"`
	MaxOrigins            int  `json:"max_origins"`
	MaxWork               int  `json:"max_work"`
	MaxPageBytes          int  `json:"max_page_bytes"`
	MaxPages              int  `json:"max_pages"`
}

func DefaultPolicy() Policy {
	return Policy{MaxInputBytes: 64 << 20, MaxOutputBytes: 16 << 20, MaxBodyBytes: 4 << 20, MaxSpans: 4096, MaxOrigins: 4096, MaxWork: 64 << 20, MaxPageBytes: 64 << 10, MaxPages: 4096}
}

type Request struct {
	Selections []Selection `json:"selections"`
	Policy     Policy      `json:"policy"`
}
type Selection struct {
	ID       string    `json:"id"`
	RecordID string    `json:"record_id"`
	SourceID string    `json:"source_id"`
	Mode     string    `json:"mode"`
	Encoding string    `json:"position_encoding"`
	Range    *Range    `json:"range"`
	Bytes    *Interval `json:"bytes"`
	Boundary *Boundary `json:"boundary"`
}
type Boundary struct {
	Kind          string `json:"kind"`
	Reference     string `json:"reference"`
	Authority     string `json:"authority"`
	Qualification string `json:"qualification"`
	SourceID      string `json:"source_id"`
	ContentHash   string `json:"content_hash"`
	Range         Range  `json:"range"`
}

// Sidecar is a first-class asserted envelope, not a native/provider acquisition.
// Its exact serialized bytes are digest-bound by every record and source.
type Sidecar struct {
	SchemaVersion  string           `json:"schema_version"`
	ArtifactDigest string           `json:"artifact_digest"`
	Authority      string           `json:"authority"`
	Qualification  string           `json:"qualification"`
	Sources        []AssertedSource `json:"sources"`
	Records        []AssertedRecord `json:"records"`
}
type AssertedSource struct {
	ID               string  `json:"id"`
	ReceiptReference string  `json:"receipt_reference"`
	VersionReference string  `json:"version_reference"`
	URI              string  `json:"uri"`
	Revision         *string `json:"revision"`
	ContentHash      *string `json:"content_hash"`
	Content          *[]byte `json:"content"`
	SourceEncoding   string  `json:"source_encoding"`
	State            string  `json:"state"` // RETAINED_BYTES, REFERENCE_ONLY, MISSING, TRUNCATED_INPUT
}
type AssertedRecord struct {
	ID                     string   `json:"id"`
	Kind                   string   `json:"kind"`       // opaque claim category; always caller-asserted
	SourceIDs              []string `json:"source_ids"` // local source ID or native:<receipt-id>
	RelationshipReferences []string `json:"relationship_references"`
	Range                  *Range   `json:"range"`
	Encoding               string   `json:"position_encoding"`
}
type Source struct {
	ID               string  `json:"id"`
	ArtifactDigest   string  `json:"artifact_digest"`
	ReceiptReference string  `json:"receipt_reference"`
	ReceiptDigest    string  `json:"receipt_digest"`
	VersionReference string  `json:"version_reference"`
	URI              string  `json:"uri"`
	Revision         *string `json:"revision"`
	ContentHash      *string `json:"content_hash"`
	SourceEncoding   string  `json:"source_encoding"`
	State            string  `json:"state"`
	InputStatus      string  `json:"input_status"`
	Authority        string  `json:"authority"`
	Qualification    string  `json:"qualification"`
	Classification   string  `json:"classification"`
	AnalyzedVersion  string  `json:"analyzed_version"`
	DocumentVersion  *int    `json:"document_version"`
}
type Record struct {
	SourceAttribution      string   `json:"source_attribution"`
	AnchorStatus           string   `json:"anchor_status"`
	ID                     string   `json:"id"`
	ArtifactDigest         string   `json:"artifact_digest"`
	Pointer                string   `json:"pointer"`
	Kind                   string   `json:"kind"`
	NativeID               string   `json:"native_id"`
	Authority              string   `json:"authority"`
	Qualification          string   `json:"qualification"`
	SourceIDs              []string `json:"source_ids"`
	RelationshipReferences []string `json:"relationship_references"`
	Range                  *Range   `json:"range"`
	Encoding               string   `json:"position_encoding"`
}
type Catalog struct {
	InputDigests []string `json:"input_digests"`
	Sources      []Source `json:"sources"`
	Records      []Record `json:"records"`
}
type Origin struct {
	CoordinateAuthority       string    `json:"coordinate_authority"`
	Selection                 Selection `json:"selection"`
	Record                    *Record   `json:"record"`
	Status                    string    `json:"status"`
	OriginalRange             *Range    `json:"original_range"`
	OriginalCoordinatesStatus string    `json:"original_coordinates_status"`
	Bytes                     *Interval `json:"bytes"`
	SpanIDs                   []string  `json:"span_ids"`
}
type Span struct {
	ID          string   `json:"id"`
	SourceID    string   `json:"source_id"`
	Encoding    string   `json:"position_encoding"`
	Bytes       Interval `json:"bytes"`
	Content     []byte   `json:"content"`
	ContentHash string   `json:"content_hash"`
	OriginIDs   []string `json:"origin_ids"`
}
type Bundle struct {
	SchemaVersion string   `json:"schema_version"`
	InputDigests  []string `json:"input_digests"`
	RequestDigest string   `json:"request_digest"`
	Policy        Policy   `json:"policy"`
	Sources       []Source `json:"sources"`
	Origins       []Origin `json:"origins"`
	Spans         []Span   `json:"spans"`
	Complete      bool     `json:"complete"` // only requested whole-span delivery, not graph/source completeness
	TotalOrigins  int      `json:"total_origins"`
	TotalSpans    int      `json:"total_spans"`
	Work          int      `json:"work"`
	Digest        string   `json:"digest"`
}
