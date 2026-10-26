package censuscontinuation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"lsp-trace/internal/captureset"
	"lsp-trace/internal/publication"
)

const maxProgramCTransitionEvidenceBytes int64 = 1 << 20

type ProgramCTransitionBoundedReader interface {
	ReadBounded(context.Context, int64) ([]byte, error)
}

type ExactProgramCTransitionInput struct {
	Reader     ProgramCTransitionBoundedReader
	ByteLength int64
	SHA256     string
}

// AuthoritativeProgramCTransitionPlanSource is private production input. It
// derives a canonical transition plan only from the two exact retained wire
// families and Native V5 verified publication manifests.
type AuthoritativeProgramCTransitionPlanSource struct {
	Receipt ExactProgramCTransitionInput
	Ledger  ExactProgramCTransitionInput
	// VerifyPublication is an injected test seam. Production hosts leave it nil;
	// production resolution derives every root exclusively from validated ledger replicas.
	VerifyPublication func(string, captureset.ExactBytesAuthority) (captureset.Manifest, error)
	resolver          *programCPublicationResolver
}

type programCVerifiedPublication struct {
	rootPath     string
	selector     string
	binaryDigest string
	binaryLength int64
	bundle       []byte
	manifest     captureset.Manifest
}

type programCPublicationResolver struct {
	bySelector map[string]programCVerifiedPublication
}

func (r *programCPublicationResolver) resolve(publicationSelector, constituentSelector string) ([]byte, error) {
	verified, ok := r.bySelector[publicationSelector]
	if !ok {
		return nil, ErrInvalidProgramCTransition
	}
	root, err := publication.OpenRoot(verified.rootPath)
	if err != nil {
		return nil, ErrInvalidProgramCTransition
	}
	defer root.Close()
	return captureset.NewPublisher(root).ResolveConstituent(publicationSelector, constituentSelector, captureset.NativeV5Authority())
}

type programCCensusReceipt struct {
	SchemaVersion             string                 `json:"schema_version"`
	Authority                 int                    `json:"authority"`
	Accepted                  bool                   `json:"accepted"`
	Completeness              string                 `json:"completeness"`
	SourceGraphComplete       string                 `json:"source_graph_complete"`
	WorkspaceSymbolComplete   bool                   `json:"workspace_symbol_complete"`
	CrossChunkCalls           string                 `json:"cross_chunk_calls"`
	FeatureIdentity           string                 `json:"feature_identity"`
	ImplementationDecision    string                 `json:"implementation_decision"`
	ProductionAdmission       string                 `json:"production_admission"`
	Effects                   string                 `json:"effects"`
	Reviews                   string                 `json:"reviews"`
	AbsenceInference          string                 `json:"absence_inference"`
	AggregateCapture          string                 `json:"aggregate_capture"`
	AllChunksSucceeded        bool                   `json:"all_chunks_succeeded"`
	AllPublicationsVerified   bool                   `json:"all_publications_verified"`
	ChunkCount                int                    `json:"chunk_count"`
	SourceCount               int                    `json:"source_count"`
	TargetCountSum            int                    `json:"target_count_sum"`
	PublicationByteLengthSum  int64                  `json:"publication_byte_length_sum"`
	GlobalSelectedPathsDigest string                 `json:"global_selected_paths_digest"`
	PartitionPlanSHA256       string                 `json:"partition_plan_sha256"`
	PreflightReceiptSHA256    string                 `json:"preflight_receipt_sha256"`
	BinaryRanges              []programCBinaryRange  `json:"binary_ranges"`
	Chunks                    []programCReceiptChunk `json:"chunks"`
}
type programCBinaryRange struct {
	WorkspaceHead string `json:"workspace_head"`
	BinarySHA256  string `json:"binary_sha256"`
	Start         int    `json:"start"`
	End           int    `json:"end"`
}
type programCReceiptChunk struct {
	Ordinal               int    `json:"ordinal"`
	WorkspaceHead         string `json:"workspace_head"`
	BinarySHA256          string `json:"binary_sha256"`
	ProbeSHA256           string `json:"probe_sha256"`
	StdoutSHA256          string `json:"stdout_sha256"`
	SourceCount           int    `json:"source_count"`
	TargetCount           int    `json:"target_count"`
	BatchCount            int    `json:"batch_count"`
	PublicationSelector   string `json:"publication_selector"`
	PublicationDigest     string `json:"publication_digest"`
	PublicationByteLength int64  `json:"publication_byte_length"`
	VerificationStatus    string `json:"verification_status"`
}
type programCAvailabilityLedger struct {
	SchemaVersion                   string                    `json:"schema_version"`
	Authority                       int                       `json:"authority"`
	Accepted                        bool                      `json:"accepted"`
	Completeness                    string                    `json:"completeness"`
	SourceGraphComplete             string                    `json:"source_graph_complete"`
	CrossChunkCalls                 string                    `json:"cross_chunk_calls"`
	FeatureIdentity                 string                    `json:"feature_identity"`
	Effects                         string                    `json:"effects"`
	AbsenceInference                string                    `json:"absence_inference"`
	AggregateCapture                string                    `json:"aggregate_capture"`
	ChunkCount                      int                       `json:"chunk_count"`
	AvailableConstituentCount       int                       `json:"available_constituent_count"`
	MissingConstituentCount         int                       `json:"missing_constituent_count"`
	InvalidReplicaCount             int                       `json:"invalid_replica_count"`
	DuplicateValidReplicaChunkCount int                       `json:"duplicate_valid_replica_chunk_count"`
	SourceManifestPresentCount      int                       `json:"source_manifest_present_count"`
	PartitionPlan                   programCFileEvidence      `json:"partition_plan"`
	PreflightReceipt                programCFileEvidence      `json:"preflight_receipt"`
	Receipt                         programCFileEvidence      `json:"receipt"`
	Rows                            []programCAvailabilityRow `json:"rows"`
}
type programCFileEvidence struct {
	Path           string `json:"path"`
	ExpectedSHA256 string `json:"expected_sha256"`
	SHA256         string `json:"sha256"`
}
type programCAvailabilityRow struct {
	Ordinal               int               `json:"ordinal"`
	ExpectedSourceCount   int               `json:"expected_source_count"`
	PublicationSelector   string            `json:"publication_selector"`
	PublicationDigest     string            `json:"publication_digest"`
	PublicationByteLength int64             `json:"publication_byte_length"`
	SourceManifest        string            `json:"source_manifest"`
	SourceManifestPresent bool              `json:"source_manifest_present"`
	ValidReplicaCount     int               `json:"valid_replica_count"`
	InvalidReplicaCount   int               `json:"invalid_replica_count"`
	Replicas              []programCReplica `json:"replicas"`
}
type programCReplica struct {
	Path       string `json:"path"`
	SHA256     string `json:"sha256"`
	ByteLength int64  `json:"byte_length"`
	Valid      bool   `json:"valid"`
}

func decodeProgramCObservedWire(raw []byte, v any) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return ErrInvalidProgramCTransition
	}
	if err := d.Decode(&struct{}{}); err != io.EOF {
		return ErrInvalidProgramCTransition
	}
	return nil
}
func readExactProgramCInput(ctx context.Context, in ExactProgramCTransitionInput) ([]byte, error) {
	if ctx == nil || in.Reader == nil || in.ByteLength <= 0 || in.ByteLength > maxProgramCTransitionEvidenceBytes || !transitionValidSHA(in.SHA256) {
		return nil, ErrInvalidProgramCTransition
	}
	raw, err := in.Reader.ReadBounded(ctx, in.ByteLength)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
		return nil, ErrInvalidProgramCTransition
	}
	if int64(len(raw)) != in.ByteLength || transitionSHA(raw) != in.SHA256 {
		return nil, ErrInvalidProgramCTransition
	}
	return raw, nil
}
func validProgramCReplicaPath(path, selector string) (string, bool) {
	if path == "" || selector == "" || strings.Contains(path, `\\`) || strings.IndexByte(path, 0) >= 0 || !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return "", false
	}
	suffix := string(filepath.Separator) + filepath.FromSlash(selector)
	if !strings.HasSuffix(path, suffix) {
		return "", false
	}
	rootPath := strings.TrimSuffix(path, suffix)
	if rootPath == "" || !filepath.IsAbs(rootPath) || filepath.Clean(rootPath) != rootPath {
		return "", false
	}
	volume := filepath.VolumeName(path) + string(filepath.Separator)
	current := volume
	for _, component := range strings.Split(strings.TrimPrefix(path, volume), string(filepath.Separator)) {
		if component == "" || component == "." || component == ".." {
			return "", false
		}
		current = filepath.Join(current, component)
		info, err := os.Lstat(current)
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			return "", false
		}
	}
	return rootPath, true
}

func verifyProgramCReplicas(row programCAvailabilityRow) (programCVerifiedPublication, error) {
	var selected programCVerifiedPublication
	var canonical []byte
	for _, replica := range row.Replicas {
		rootPath, ok := validProgramCReplicaPath(replica.Path, row.PublicationSelector)
		if !ok {
			return selected, ErrInvalidProgramCTransition
		}
		root, err := publication.OpenRoot(rootPath)
		if err != nil {
			return selected, ErrInvalidProgramCTransition
		}
		raw, readErr := root.ReadSelector(row.PublicationSelector, captureset.MaxBundleBytes)
		if readErr != nil || int64(len(raw)) != row.PublicationByteLength || int64(len(raw)) != replica.ByteLength || transitionSHA(raw) != row.PublicationDigest || transitionSHA(raw) != replica.SHA256 {
			root.Close()
			return selected, ErrInvalidProgramCTransition
		}
		manifest, verifyErr := captureset.NewPublisher(root).Verify(row.PublicationSelector, captureset.NativeV5Authority())
		root.Close()
		if verifyErr != nil {
			return selected, ErrInvalidProgramCTransition
		}
		manifestRaw, encodeErr := captureset.EncodeCanonical(manifest)
		if encodeErr != nil || canonical != nil && (!bytes.Equal(raw, selected.bundle) || !bytes.Equal(manifestRaw, canonical)) {
			return selected, ErrInvalidProgramCTransition
		}
		candidate := programCVerifiedPublication{rootPath: rootPath, selector: row.PublicationSelector, binaryDigest: row.PublicationDigest, binaryLength: row.PublicationByteLength, bundle: bytes.Clone(raw), manifest: manifest}
		if canonical == nil {
			canonical = manifestRaw
			selected = candidate
		} else if candidate.rootPath < selected.rootPath {
			selected = candidate
		}
	}
	if canonical == nil {
		return selected, ErrInvalidProgramCTransition
	}
	return selected, nil
}

func (s *AuthoritativeProgramCTransitionPlanSource) Load(ctx context.Context) ([]byte, error) {
	receiptRaw, err := readExactProgramCInput(ctx, s.Receipt)
	if err != nil {
		return nil, err
	}
	ledgerRaw, err := readExactProgramCInput(ctx, s.Ledger)
	if err != nil {
		return nil, err
	}
	var receipt programCCensusReceipt
	var ledger programCAvailabilityLedger
	if decodeProgramCObservedWire(receiptRaw, &receipt) != nil || decodeProgramCObservedWire(ledgerRaw, &ledger) != nil {
		return nil, ErrInvalidProgramCTransition
	}
	if receipt.SchemaVersion != "lsp-trace.census-source-partition-complete.private.v1" || ledger.SchemaVersion != "lsp-trace.census-constituent-availability.private.v1" || receipt.Authority != 0 || ledger.Authority != 0 || receipt.Accepted || ledger.Accepted || !receipt.AllChunksSucceeded || !receipt.AllPublicationsVerified || receipt.ChunkCount <= 0 || receipt.ChunkCount != len(receipt.Chunks) || ledger.ChunkCount != receipt.ChunkCount || len(ledger.Rows) != receipt.ChunkCount || ledger.AvailableConstituentCount != receipt.ChunkCount || ledger.MissingConstituentCount != 0 || ledger.InvalidReplicaCount != 0 || ledger.SourceManifestPresentCount != receipt.ChunkCount || receipt.Completeness != "UNKNOWN" || ledger.Completeness != "UNKNOWN" || receipt.SourceGraphComplete != "UNKNOWN" || ledger.SourceGraphComplete != "UNKNOWN" || receipt.WorkspaceSymbolComplete || receipt.CrossChunkCalls != "NOT_COMPUTED" || ledger.CrossChunkCalls != "NOT_COMPUTED" || receipt.FeatureIdentity != "UNRESOLVED" || ledger.FeatureIdentity != "UNRESOLVED" || receipt.ImplementationDecision != "UNISSUED" || receipt.ProductionAdmission != "REFUSED_MISSING_EVIDENCE" || receipt.Effects != "NONE" || ledger.Effects != "NONE" || receipt.Reviews != "NOT_REQUESTED" || receipt.AbsenceInference != "PROHIBITED" || ledger.AbsenceInference != "PROHIBITED" || receipt.AggregateCapture != "NOT_MATERIALIZED" || ledger.AggregateCapture != "NOT_MATERIALIZED" || receipt.SourceCount <= 0 || receipt.TargetCountSum <= 0 || receipt.PublicationByteLengthSum <= 0 || !transitionValidSHA(receipt.GlobalSelectedPathsDigest) || !transitionValidSHA(receipt.PartitionPlanSHA256) || !transitionValidSHA(receipt.PreflightReceiptSHA256) || ledger.DuplicateValidReplicaChunkCount < 0 {
		return nil, ErrInvalidProgramCTransition
	}
	rows := make(map[int]programCAvailabilityRow, len(ledger.Rows))
	for _, row := range ledger.Rows {
		if row.Ordinal <= 0 {
			return nil, ErrInvalidProgramCTransition
		}
		if _, ok := rows[row.Ordinal]; ok {
			return nil, ErrInvalidProgramCTransition
		}
		rows[row.Ordinal] = row
	}
	chunks := append([]programCReceiptChunk(nil), receipt.Chunks...)
	sort.Slice(chunks, func(i, j int) bool { return chunks[i].Ordinal < chunks[j].Ordinal })
	records := make([]ProgramCTransitionPlanRecord, 0)
	resolver := &programCPublicationResolver{bySelector: make(map[string]programCVerifiedPublication, len(chunks))}
	duplicateRows := 0
	receiptSourceSum := 0
	ledgerSourceSum := 0
	targetSum := 0
	publicationByteSum := int64(0)
	batchSum := 0
	for index, chunk := range chunks {
		if chunk.Ordinal != index+1 || chunk.VerificationStatus != "VERIFIED" || captureset.ValidatePublicationSelector(chunk.PublicationSelector) != nil || !transitionValidSHA(chunk.PublicationDigest) || chunk.PublicationByteLength <= 0 || chunk.SourceCount <= 0 || chunk.BatchCount <= 0 {
			return nil, ErrInvalidProgramCTransition
		}
		row, ok := rows[chunk.Ordinal]
		if !ok || row.Ordinal != chunk.Ordinal || row.ExpectedSourceCount != chunk.SourceCount || row.PublicationSelector != chunk.PublicationSelector || row.PublicationDigest != chunk.PublicationDigest || row.PublicationByteLength != chunk.PublicationByteLength || !row.SourceManifestPresent || row.SourceManifest == "" || row.InvalidReplicaCount != 0 || row.ValidReplicaCount <= 0 || row.ValidReplicaCount != len(row.Replicas) {
			return nil, ErrInvalidProgramCTransition
		}
		receiptSourceSum += chunk.SourceCount
		ledgerSourceSum += row.ExpectedSourceCount
		targetSum += chunk.TargetCount
		publicationByteSum += chunk.PublicationByteLength
		batchSum += chunk.BatchCount
		if row.ValidReplicaCount > 1 {
			duplicateRows++
		}
		for _, replica := range row.Replicas {
			if !replica.Valid || replica.SHA256 != row.PublicationDigest || replica.ByteLength != row.PublicationByteLength {
				return nil, ErrInvalidProgramCTransition
			}
		}
		var manifest captureset.Manifest
		if s.VerifyPublication != nil {
			manifest, err = s.VerifyPublication(chunk.PublicationSelector, captureset.NativeV5Authority())
		} else {
			verified, verifyErr := verifyProgramCReplicas(row)
			if verifyErr != nil {
				return nil, ErrInvalidProgramCTransition
			}
			manifest = verified.manifest
			if previous, exists := resolver.bySelector[chunk.PublicationSelector]; exists && (previous.binaryDigest != verified.binaryDigest || previous.binaryLength != verified.binaryLength || !bytes.Equal(previous.bundle, verified.bundle)) {
				return nil, ErrInvalidProgramCTransition
			}
			resolver.bySelector[chunk.PublicationSelector] = verified
		}
		if err != nil || len(manifest.Batches) != chunk.BatchCount || len(manifest.Constituents) != len(manifest.Batches) {
			return nil, ErrInvalidProgramCTransition
		}
		seenBatch := make(map[int]bool, len(manifest.Batches))
		for _, batch := range manifest.Batches {
			if batch.Ordinal < 0 || seenBatch[batch.Ordinal] || batch.ConstituentIndex < 0 || batch.ConstituentIndex >= len(manifest.Constituents) {
				return nil, ErrInvalidProgramCTransition
			}
			seenBatch[batch.Ordinal] = true
		}
		for ordinal := 0; ordinal < len(manifest.Batches); ordinal++ {
			if !seenBatch[ordinal] {
				return nil, ErrInvalidProgramCTransition
			}
			batch := manifest.Batches[ordinal]
			selector := manifest.Constituents[batch.ConstituentIndex].ImmutableSelector
			if !transitionValidConstituentSelector(selector) {
				return nil, ErrInvalidProgramCTransition
			}
			records = append(records, ProgramCTransitionPlanRecord{ReceiptOrdinal: chunk.Ordinal, PublicationByteLength: chunk.PublicationByteLength, BatchOrdinal: batch.Ordinal, PublicationSelector: chunk.PublicationSelector, ConstituentSelector: selector})
		}
	}
	sort.Slice(records, func(i, j int) bool {
		if records[i].PublicationByteLength != records[j].PublicationByteLength {
			return records[i].PublicationByteLength < records[j].PublicationByteLength
		}
		if records[i].ReceiptOrdinal != records[j].ReceiptOrdinal {
			return records[i].ReceiptOrdinal < records[j].ReceiptOrdinal
		}
		return records[i].BatchOrdinal < records[j].BatchOrdinal
	})
	for i := range records {
		records[i].Ordinal = i
	}
	if duplicateRows != ledger.DuplicateValidReplicaChunkCount || len(rows) != len(chunks) || receiptSourceSum != receipt.SourceCount || ledgerSourceSum != receipt.SourceCount || targetSum != receipt.TargetCountSum || publicationByteSum != receipt.PublicationByteLengthSum || batchSum != len(records) {
		return nil, ErrInvalidProgramCTransition
	}
	if s.VerifyPublication == nil {
		s.resolver = resolver
	}
	return EncodeProgramCTransitionPlan(ProgramCTransitionPlan{SchemaVersion: ProgramCTransitionPlanVersion, Authority: 0, CensusReceiptSHA256: transitionSHA(receiptRaw), LedgerSHA256: transitionSHA(ledgerRaw), Records: records})
}
