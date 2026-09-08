package hydratedevidence

// FocusRequest selects native graph identities, not names or catalog prefixes.
// CorePolicy limits remain mandatory; IncludeBodies here controls body release.
type FocusRequest struct {
 NodeIDs []string `json:"node_ids"`
 RelationIDs []string `json:"relation_ids"`
 SidecarRecordIDs []string `json:"sidecar_record_ids"`
 IncludeBodies bool `json:"include_bodies"`
 WholeFile bool `json:"whole_file"`
 EndpointContext bool `json:"endpoint_context"`
 PositionEncoding string `json:"position_encoding"`
 CorePolicy Policy `json:"core_policy"`
}

func DefaultFocusRequest() FocusRequest { return FocusRequest{CorePolicy: DefaultPolicy()} }

type FocusSite struct {
 Role string `json:"role"`
 Pointer string `json:"pointer"`
 RecordID string `json:"record_id"`
 NodeID string `json:"node_id"`
 RelationID string `json:"relation_id"`
 URI string `json:"uri"`
 Status string `json:"status"`
 SourceIDs []string `json:"source_ids"`
 OriginIDs []string `json:"origin_ids"`
}
type FocusOrigin struct {
 Ordinal int `json:"ordinal"`
 Kind string `json:"kind"`
 RequestedID string `json:"requested_id"`
 Status string `json:"status"`
 CallerNodeID string `json:"caller_node_id"`
 CalleeNodeID string `json:"callee_node_id"`
 CallSiteCount int `json:"call_site_count"`
 Sites []FocusSite `json:"sites"`
}
type FocusManifest struct {
 InputDigests []string `json:"input_digests"`
 SelectionDigest string `json:"selection_digest"`
 DuplicatePolicy string `json:"duplicate_policy"`
 ReceiptPolicy string `json:"receipt_policy"`
 NodePolicy string `json:"node_policy"`
 ExcludedNonSourceRecords int `json:"excluded_non_source_records"`
 Origins []FocusOrigin `json:"origins"`
 Digest string `json:"digest"`
}
type FocusResult struct {
 Manifest FocusManifest `json:"manifest"`
 Request Request `json:"request"`
 Bundle Bundle `json:"bundle"`
}

func HydrateFocused(input Input, f FocusRequest) (FocusResult, error) { return FocusResult{}, nil }
func ValidateFocused(input Input, f FocusRequest, result FocusResult) error { return nil }
