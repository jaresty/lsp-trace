package inventoryworkflow

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"lsp-trace/internal/censuscontinuation"
	"lsp-trace/internal/inventorysuccessor"
	"lsp-trace/internal/programcrepresentative"
	"lsp-trace/internal/programctransitionhost"
)

const preparedManifestRawSHA256V2 = "sha256:0cd5229bca98b9681c9aa2122b7acbfb65f38676a572606a9b73814e9cad66c5"
const preparedManifestIdentityV2 = "sha256:19481e7d36682e3d4c95a52121235f1139164e84f3221a20b59e04f6499a5b41"

// This is a test-only preparation descriptor, not a qualification, review,
// assignment, terminal projection, or authority-bearing production artifact.
// Explicit fields retain values only: Request functions/interfaces are never JSON.
type preparedInputV2 struct {
	Path       string `json:"path"`
	SHA256     string `json:"sha256"`
	ByteLength int64  `json:"byte_length"`
}
type preparedRequestMetadataV2 struct {
	Bindings             censuscontinuation.ProgramCTransitionBindings `json:"bindings"`
	Identity             inventorysuccessor.WorkflowIdentity           `json:"identity"`
	ProgramCWorkflowKey  string                                        `json:"program_c_workflow_key"`
	ProgramCHeadKey      string                                        `json:"program_c_head_key"`
	SuccessorWorkflowKey string                                        `json:"successor_workflow_key"`
	SuccessorHeadKey     string                                        `json:"successor_head_key"`
}
type preparedExpectedMetadataV2 struct {
	AdapterContract              programcrepresentative.AdapterContractDTOv2             `json:"adapter_contract"`
	Stage6Implementation         programcrepresentative.Stage6ImplementationDTO          `json:"stage6_implementation"`
	CorpusIdentity               programcrepresentative.CorpusIdentityDTO                `json:"corpus_identity"`
	WorkflowIdentity             programcrepresentative.QualificationWorkflowIdentityDTO `json:"workflow_identity"`
	SourcePinSHA256              string                                                  `json:"source_pin_sha256"`
	ReceiptInventory             []programcrepresentative.ReceiptInventoryEntryDTO       `json:"receipt_inventory"`
	TerminalProgramCHead         programcrepresentative.TerminalProgramCHeadDTO          `json:"terminal_program_c_head"`
	MaxImplementationSourceBytes uint64                                                  `json:"max_implementation_source_bytes"`
}
type preparedQualificationV2 struct {
	Kind               string                                    `json:"kind"`
	CorpusRoot         string                                    `json:"corpus_root"`
	PublicationRoot    string                                    `json:"publication_root"`
	StateRoot          string                                    `json:"state_root"`
	RetainedSourceRoot string                                    `json:"retained_source_root"`
	OriginalSourceRoot string                                    `json:"original_source_root"`
	DeclarationSHA256  string                                    `json:"declaration_sha256"`
	Receipt            preparedInputV2                           `json:"receipt"`
	Ledger             preparedInputV2                           `json:"ledger"`
	Request            preparedRequestMetadataV2                 `json:"request"`
	Subject            programcrepresentative.ArtifactBindingDTO `json:"subject"`
	Files              []preparedInputV2                         `json:"files"`
}

func preparedRequestMetadataOfV2(r Request) preparedRequestMetadataV2 {
	return preparedRequestMetadataV2{r.ProgramC.Bindings, r.Identity, r.ProgramC.WorkflowKey, r.ProgramC.HeadKey, r.SuccessorWorkflowKey, r.SuccessorHeadKey}
}
func preparedExpectedOfV2(d programcrepresentative.QualificationSubjectV2) preparedExpectedMetadataV2 {
	return preparedExpectedMetadataV2{d.AdapterContract, d.Stage6Implementation, d.CorpusIdentity, d.WorkflowIdentity, d.SourcePinSHA256, d.ReceiptInventory, d.TerminalProgramCHead, 237568}
}

// Exact selected-path reader. File size and a limit+1 read are checked before
// exposing bytes. No test-only.go shortcut, checkout fallback, or LSP exists.
type preparedSourceReaderV2 struct {
	root  string
	files map[string]uint64
}

func (r preparedSourceReaderV2) ReadImplementationFileV2(name string, limit uint64) ([]byte, error) {
	expected, ok := r.files[name]
	if !ok || expected != limit || limit == 0 || limit > 237568 {
		return nil, fmt.Errorf("SELECTED_SOURCE_READ_BOUND")
	}
	return preparedReadV2(r.root, name, int64(limit), true)
}
func preparedSourceV2(root string, m programcrepresentative.Stage6ImplementationDTO) preparedSourceReaderV2 {
	r := preparedSourceReaderV2{root: root, files: map[string]uint64{}}
	for _, f := range m.Files {
		r.files[f.Path] = f.ByteLength
	}
	return r
}
func preparedReadV2(root, name string, limit int64, exact bool) ([]byte, error) {
	if !fs.ValidPath(name) || limit < 0 || uint64(limit) > programcrepresentative.MaxArtifactCanonicalBytesV2 {
		return nil, fmt.Errorf("INVALID_RETAINED_PATH_OR_BOUND")
	}
	path := root
	for _, part := range strings.Split(name, "/") {
		path = filepath.Join(path, part)
		info, err := os.Lstat(path)
		if err != nil {
			return nil, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("RETAINED_SYMLINK_REFUSED")
		}
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > limit || exact && info.Size() != limit {
		return nil, fmt.Errorf("RETAINED_EXACT_READ_BOUND")
	}
	raw, err := io.ReadAll(io.LimitReader(f, limit+1))
	if int64(len(raw)) > limit || exact && int64(len(raw)) != limit {
		return nil, fmt.Errorf("RETAINED_EXACT_READ_BOUND")
	}
	return raw, err
}
func preparedWriteV2(t *testing.T, root, name string, raw []byte) {
	t.Helper()
	if !fs.ValidPath(name) {
		t.Fatal("PRIVATE_EVIDENCE_PATH")
	}
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, writeErr := f.Write(raw)
	closeErr := f.Close()
	if err := errors.Join(writeErr, closeErr); err != nil {
		t.Fatal(err)
	}
}
func preparedJSONV2(t *testing.T, root, name string, value any) {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	preparedWriteV2(t, root, name, raw)
}
func preparedCanonicalDirectoryV2(path string) error {
	real, err := filepath.EvalSymlinks(path)
	if err != nil || !filepath.IsAbs(path) || real != path || filepath.Clean(path) != path {
		return fmt.Errorf("EXPLICIT_CANONICAL_PRIVATE_DIRECTORY_REQUIRED: %v", err)
	}
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() || info.Mode().Perm() != 0700 {
		return fmt.Errorf("PRIVATE_DIRECTORY_0700_REQUIRED")
	}
	return nil
}
func preparedManifestV2(raw []byte) (programcrepresentative.Stage6ImplementationDTO, error) {
	var m programcrepresentative.Stage6ImplementationDTO
	if workflowDigest(raw) != preparedManifestRawSHA256V2 || json.Unmarshal(raw, &m) != nil {
		return m, fmt.Errorf("FIXED_MANIFEST_RAW_PIN")
	}
	declaration, err := programcrepresentative.BuildStage6ImplementationV2(237568)
	canonical, marshalErr := json.Marshal(m)
	if err != nil || marshalErr != nil || !bytes.Equal(raw, canonical) || !reflect.DeepEqual(m, declaration) || m.Identity != preparedManifestIdentityV2 {
		return m, fmt.Errorf("FIXED_MANIFEST_GO_DTO_DECLARATION: %v", err)
	}
	return m, nil
}

func TestPrepareRetainedMultiSeedQualificationV2(t *testing.T) {
	base := os.Getenv("MULTISEED_QUALIFICATION_PREPARE_BASE")
	if base == "" {
		t.Skip("opt-in persistent controlled preparation absent")
	}
	if err := preparedCanonicalDirectoryV2(base); err != nil {
		t.Fatal(err)
	}
	name := os.Getenv("MULTISEED_QUALIFICATION_CORPUS_NAME")
	if !fs.ValidPath(name) || strings.Contains(name, "/") {
		t.Fatal("EXPLICIT_NEW_CHILD_NAME_REQUIRED")
	}
	sourceRoot := os.Getenv("MULTISEED_QUALIFICATION_SOURCE_ROOT")
	canonicalSource, err := filepath.EvalSymlinks(sourceRoot)
	if err != nil || !filepath.IsAbs(sourceRoot) || canonicalSource != sourceRoot {
		t.Fatal("EXPLICIT_CANONICAL_SOURCE_ROOT_REQUIRED")
	}
	manifestPath := os.Getenv("MULTISEED_QUALIFICATION_MANIFEST")
	if !filepath.IsAbs(manifestPath) {
		t.Fatal("EXPLICIT_FIXED_MANIFEST_REQUIRED")
	}
	manifestRaw, err := preparedReadV2(filepath.Dir(manifestPath), filepath.Base(manifestPath), 65536, false)
	if err != nil {
		t.Fatal(err)
	}
	m, err := preparedManifestV2(manifestRaw)
	if err != nil {
		t.Fatalf("PREPARE_REAL_MANIFEST_REQUIRED: %v", err)
	}
	source := preparedSourceV2(sourceRoot, m)
	if err = programcrepresentative.VerifyStage6ImplementationSourceV2(m, source, 237568); err != nil {
		t.Fatalf("PREPARE_REAL_SOURCE_REQUIRED: %v", err)
	}
	corpus := filepath.Join(base, name)
	if err = os.Mkdir(corpus, 0700); err != nil {
		t.Fatalf("NEW_CORPUS_ONLY_NO_REPLAY: %v", err)
	}
	for _, dir := range []string{"publication", "state", "source"} {
		if err = os.Mkdir(filepath.Join(corpus, dir), 0700); err != nil {
			t.Fatal(err)
		}
	}
	for _, pin := range m.Files {
		raw, err := source.ReadImplementationFileV2(pin.Path, pin.ByteLength)
		if err != nil {
			t.Fatal(err)
		}
		preparedWriteV2(t, filepath.Join(corpus, "source"), pin.Path, raw)
	}
	decl, err := preparedReadV2(sourceRoot, "internal/programcrepresentative/multiseed_reviewed_bindings_v2.go", 65536, false)
	if err != nil {
		t.Fatal(err)
	}
	preparedWriteV2(t, corpus, "declaration.go.txt", decl)
	request, _ := multiSeedRetainedCheckpointFixtureAtV2(t, filepath.Join(corpus, "publication"), filepath.Join(corpus, "state"))
	retainedSource := preparedSourceV2(filepath.Join(corpus, "source"), m)
	subject, err := BuildProgramCMultiSeedQualificationSubjectV2(context.Background(), request, retainedSource, 237568)
	if err != nil || !subject.Trusted() {
		t.Fatalf("PREPARE_ACTUAL_SUBJECT: %v", err)
	}
	raw, err := subject.CanonicalBytes()
	if err != nil {
		t.Fatal(err)
	}
	dto, err := subject.DTO()
	if err != nil {
		t.Fatal(err)
	}
	actors, actorErr := programcrepresentative.ReviewedHarnessActorBindingsV2()
	if actors != nil || !errors.Is(actorErr, programcrepresentative.ErrMultiSeedV2ReviewRequired) {
		t.Fatal("PREPARE_HARNESS_MUST_REMAIN_UNISSUED")
	}
	preparedWriteV2(t, corpus, "subject.json", raw)
	preparedJSONV2(t, corpus, "subject-dto.json", dto)
	preparedJSONV2(t, corpus, "expected-context.json", preparedExpectedOfV2(dto))
	preparedWriteV2(t, corpus, "implementation-manifest.json", manifestRaw)
	_, policy, policyRaw, err := programcrepresentative.BuildStage6PolicyArtifactV2()
	if err != nil {
		t.Fatal(err)
	}
	_, limits, limitsRaw, err := programcrepresentative.BuildStage6LimitsArtifactV2()
	if err != nil {
		t.Fatal(err)
	}
	_, finalization, finalizationRaw, err := programcrepresentative.BuildStage6FinalizationArtifactV2()
	if err != nil {
		t.Fatal(err)
	}
	preparedWriteV2(t, corpus, "policy.json", policyRaw)
	preparedWriteV2(t, corpus, "limits.json", limitsRaw)
	preparedWriteV2(t, corpus, "finalization.json", finalizationRaw)
	preparedJSONV2(t, corpus, "fixed-bindings.json", []programcrepresentative.ArtifactBindingDTO{policy, limits, finalization})
	retainPreparedAccountingV2(t, corpus, request, dto)
	receiptRaw, err := request.ProgramC.Receipt.Reader.ReadBounded(context.Background(), request.ProgramC.Receipt.ByteLength)
	if err != nil {
		t.Fatal(err)
	}
	ledgerRaw, err := request.ProgramC.Ledger.Reader.ReadBounded(context.Background(), request.ProgramC.Ledger.ByteLength)
	if err != nil {
		t.Fatal(err)
	}
	descriptor := preparedQualificationV2{
		Kind: "CONTROLLED_GRAPH_INPUT_QUALIFICATION_PREPARATION_NOT_REVIEWED", CorpusRoot: corpus,
		PublicationRoot: filepath.Join(corpus, "publication"), StateRoot: filepath.Join(corpus, "state"), RetainedSourceRoot: filepath.Join(corpus, "source"),
		OriginalSourceRoot: sourceRoot, DeclarationSHA256: workflowDigest(decl),
		Receipt: preparedInputV2{"publication/controlled-receipt.json", workflowDigest(receiptRaw), int64(len(receiptRaw))},
		Ledger:  preparedInputV2{"publication/controlled-ledger.json", workflowDigest(ledgerRaw), int64(len(ledgerRaw))},
		Request: preparedRequestMetadataOfV2(request),
		Subject: programcrepresentative.ArtifactBindingDTO{SchemaVersion: dto.SchemaVersion, Identity: subject.Identity(), SHA256: workflowDigest(raw), ByteLength: uint64(len(raw))},
	}
	preparedWriteV2(t, corpus, "INDEX.md", []byte("# Controlled qualification preparation\nCONTROLLED GRAPH INPUT, NOT target application evidence. Native fixtures constructed by Go, not live language-server acquisition. Selected reviewed production seams only; NOT compiler/transitive dependency closure. No semantic feature count claim.\nSubject is produced for later actual independent review. Harness remains unissued. No actors, assignments, review receipts, envelopes, or qualification capabilities are produced. Initial frozen Census to Program C successor only; no downstream checkpoint operations.\nProvenance ceiling remains PREDECESSOR_RETAINED_CALLER_ASSERTED; no production admission, user evidence, permission, privacy or retention authority is established. descriptor.json records roots, exact input pins, keys, bindings, subject binding and a closed retained-file inventory. Reopen performs no Run/CAS and uses only retained source bytes.\n"))
	descriptor.Files, err = preparedInventoryV2(corpus, true)
	if err != nil {
		t.Fatal(err)
	}
	preparedJSONV2(t, corpus, "descriptor.json", descriptor)
	reopened, err := reopenPreparedQualificationV2(t, corpus)
	if err != nil {
		t.Fatalf("PERSISTENT_PREPARATION_REOPEN: %v", err)
	}
	again, err := reopened.CanonicalBytes()
	if err != nil || !bytes.Equal(raw, again) {
		t.Fatal("PERSISTENT_SUBJECT_BYTE_IDENTICAL")
	}
	t.Logf("PREPARED_CONTROLLED_SUBJECT PASS corpus=%s schema=%s identity=%s raw=%s bytes=%d; harness UNISSUED", corpus, dto.SchemaVersion, descriptor.Subject.Identity, descriptor.Subject.SHA256, descriptor.Subject.ByteLength)
}

func retainPreparedAccountingV2(t *testing.T, corpus string, request Request, dto programcrepresentative.QualificationSubjectV2) {
	t.Helper()
	root, err := request.ProgramC.OpenState()
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	verification, err := programctransitionhost.NewCompletionVerificationRequest(context.Background(), CompletionRequest{StateRoot: root, Receipt: request.ProgramC.Receipt, Ledger: request.ProgramC.Ledger, WorkflowKey: request.ProgramC.WorkflowKey, HeadKey: request.ProgramC.HeadKey, Bindings: request.ProgramC.Bindings})
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := censuscontinuation.BuildProgramCMultiSeedQualificationEvidenceV2(context.Background(), verification)
	if err != nil {
		t.Fatal(err)
	}
	expected, err := evidence.QualificationSubjectExpectedContextV2()
	if err != nil {
		t.Fatal(err)
	}
	d, g := assertMultiSeedRetainedAccountingV2(t, evidence, expected)
	if d != 24 || g != 48 {
		t.Fatalf("PREPARED_D24_G48: D=%d G=%d", d, g)
	}
	constituents, _, err := evidence.ReconstructCheckpointConstituentsV2(context.Background(), dto)
	if err != nil {
		t.Fatal(err)
	}
	preparedJSONV2(t, corpus, "constituents-and-24-runs.json", constituents)
	preparedJSONV2(t, corpus, "receipt-inventory.json", expected.ReceiptInventory)
	preparedJSONV2(t, corpus, "terminal-program-c-head.json", expected.TerminalProgramCHead)
	preparedWriteV2(t, corpus, "COVERAGE.md", []byte("CONTROLLED GRAPH INPUT; not live application evidence. Actual production-owned reconstruction: 4 constituents, 24 runs; COMPLETE=2, EMPTY=1, COMMITTED_INCOMPLETE_MATCHING_RESOURCE=1; D=24, G=48. EMPTY and resource-excluded constituents contribute no eligible mechanics. Availability ledger counts one chunk, not four batches. Unissued harness blocks qualification and downstream operations. Source scope: 30 selected reviewed production Go seams, 237568 bytes at f06e6fb2d2c38aca375bf7d8e622e6cb326272d1 / abc78c0003d197d5fdcb003d76754968d3511d10; not compiler closure. Privacy/retention provenance remains caller-asserted.\n"))
}

func preparedInventoryV2(root string, normalize bool) ([]preparedInputV2, error) {
	files := []preparedInputV2{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("RETAINED_SYMLINK_REFUSED")
		}
		if entry.IsDir() {
			if normalize {
				return os.Chmod(path, 0700)
			}
			info, err := entry.Info()
			if err != nil || info.Mode().Perm() != 0700 {
				return fmt.Errorf("RETAINED_DIRECTORY_MODE")
			}
			return nil
		}
		if rel == "descriptor.json" {
			return nil
		}
		if normalize {
			if err := os.Chmod(path, 0600); err != nil {
				return err
			}
		}
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
			return fmt.Errorf("RETAINED_FILE_MODE")
		}
		raw, err := preparedReadV2(root, rel, info.Size(), true)
		if err != nil {
			return err
		}
		files = append(files, preparedInputV2{rel, workflowDigest(raw), int64(len(raw))})
		return nil
	})
	return files, err
}

// Read-only reopen: verify the closed retained inventory before the existing
// request factory reads any input. No fixture initializer, Run, CAS, provider,
// checkout source, synthetic review adapter, or positive qualification is used.
func reopenPreparedQualificationV2(t *testing.T, corpus string) (programcrepresentative.ProgramCRepresentativeQualificationSubjectV2, error) {
	t.Helper()
	zero := programcrepresentative.ProgramCRepresentativeQualificationSubjectV2{}
	if err := preparedCanonicalDirectoryV2(corpus); err != nil {
		return zero, err
	}
	raw, err := preparedReadV2(corpus, "descriptor.json", 1048576, false)
	if err != nil {
		return zero, err
	}
	var d preparedQualificationV2
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err = dec.Decode(&d); err != nil {
		return zero, err
	}
	if d.Kind != "CONTROLLED_GRAPH_INPUT_QUALIFICATION_PREPARATION_NOT_REVIEWED" || d.CorpusRoot != corpus || d.StateRoot != filepath.Join(corpus, "state") || d.PublicationRoot != filepath.Join(corpus, "publication") || d.RetainedSourceRoot != filepath.Join(corpus, "source") || d.Receipt.Path != "publication/controlled-receipt.json" || d.Ledger.Path != "publication/controlled-ledger.json" {
		return zero, fmt.Errorf("PREPARED_ROOT_BINDINGS")
	}
	files, err := preparedInventoryV2(corpus, false)
	if err != nil || !reflect.DeepEqual(files, d.Files) {
		return zero, fmt.Errorf("PREDECESSOR_RETAINED_INVENTORY_MISMATCH: %v", err)
	}
	manifestRaw, err := preparedReadV2(corpus, "implementation-manifest.json", 65536, false)
	if err != nil {
		return zero, err
	}
	m, err := preparedManifestV2(manifestRaw)
	if err != nil {
		return zero, err
	}
	decl, err := preparedReadV2(corpus, "declaration.go.txt", 65536, false)
	if err != nil || workflowDigest(decl) != d.DeclarationSHA256 {
		return zero, fmt.Errorf("PREPARED_DECLARATION_PIN")
	}
	for _, pin := range []preparedInputV2{d.Receipt, d.Ledger} {
		body, err := preparedReadV2(corpus, pin.Path, pin.ByteLength, true)
		if err != nil || workflowDigest(body) != pin.SHA256 {
			return zero, fmt.Errorf("PREPARED_INPUT_PIN: %v", err)
		}
	}
	input := func(pin preparedInputV2) censuscontinuation.ExactProgramCTransitionInput {
		return censuscontinuation.ExactProgramCTransitionInput{Reader: exactFileReader{path: filepath.Join(corpus, filepath.FromSlash(pin.Path))}, ByteLength: pin.ByteLength, SHA256: pin.SHA256}
	}
	opens := 0
	request := productionWorkflowRequest(t, input(d.Receipt), input(d.Ledger), d.StateRoot, 8, &opens)
	if !reflect.DeepEqual(preparedRequestMetadataOfV2(request), d.Request) {
		return zero, fmt.Errorf("PREPARED_REQUEST_METADATA_MISMATCH")
	}
	subject, err := BuildProgramCMultiSeedQualificationSubjectV2(context.Background(), request, preparedSourceV2(d.RetainedSourceRoot, m), 237568)
	if err != nil {
		return zero, err
	}
	subjectRaw, err := subject.CanonicalBytes()
	if err != nil {
		return zero, err
	}
	retained, err := preparedReadV2(corpus, "subject.json", int64(d.Subject.ByteLength), true)
	if err != nil || !subject.Trusted() || !bytes.Equal(subjectRaw, retained) || workflowDigest(subjectRaw) != d.Subject.SHA256 || subject.Identity() != d.Subject.Identity {
		return zero, fmt.Errorf("PERSISTENT_SUBJECT_BYTE_IDENTICAL")
	}
	dto, err := subject.DTO()
	if err != nil || dto.SchemaVersion != d.Subject.SchemaVersion {
		return zero, fmt.Errorf("PREPARED_SUBJECT_SCHEMA")
	}
	expectedRaw, err := preparedReadV2(corpus, "expected-context.json", 1048576, false)
	if err != nil {
		return zero, err
	}
	wantExpected, err := json.Marshal(preparedExpectedOfV2(dto))
	if err != nil || !bytes.Equal(expectedRaw, wantExpected) {
		return zero, fmt.Errorf("PREPARED_EXPECTED_CONTEXT")
	}
	actors, actorErr := programcrepresentative.ReviewedHarnessActorBindingsV2()
	if actors != nil || !errors.Is(actorErr, programcrepresentative.ErrMultiSeedV2ReviewRequired) {
		return zero, fmt.Errorf("PREPARED_HARNESS_UNISSUED")
	}
	after, err := preparedInventoryV2(corpus, false)
	if err != nil || !reflect.DeepEqual(after, files) {
		return zero, fmt.Errorf("READ_ONLY_REOPEN_MUTATED_STATE")
	}
	return subject, nil
}

func TestReopenRetainedMultiSeedQualificationSubjectV2(t *testing.T) {
	corpus := os.Getenv("MULTISEED_QUALIFICATION_REOPEN_DIR")
	if corpus == "" {
		t.Skip("opt-in retained controlled subject directory absent")
	}
	subject, err := reopenPreparedQualificationV2(t, corpus)
	if err != nil {
		t.Fatalf("FRESH_REOPEN_ACTUAL_SUBJECT: %v", err)
	}
	t.Logf("FRESH_REOPEN_ACTUAL_SUBJECT PASS identity=%s; byte-identical, predecessors resolved, read-only, harness UNISSUED", subject.Identity())
}

// Negative controls mutate only an explicitly opted-in writer SCRATCH corpus.
// Restoration is mandatory even on assertion failure; never point this at the
// parent's future review subject. The descriptor itself remains byte-identical.
func TestScratchRetainedMultiSeedQualificationCorruptionV2(t *testing.T) {
	corpus := os.Getenv("MULTISEED_QUALIFICATION_SCRATCH_CORRUPTION_DIR")
	if corpus == "" {
		t.Skip("scratch-only corruption opt-in absent")
	}
	if !strings.Contains(corpus, "/multiseed-genuine-preparer-") {
		t.Fatal("WRITER_SCRATCH_CORPUS_ONLY")
	}
	if _, err := reopenPreparedQualificationV2(t, corpus); err != nil {
		t.Fatal(err)
	}
	descriptorRaw, err := preparedReadV2(corpus, "descriptor.json", 1048576, false)
	if err != nil {
		t.Fatal(err)
	}
	var d preparedQualificationV2
	if err := json.Unmarshal(descriptorRaw, &d); err != nil {
		t.Fatal(err)
	}
	native := ""
	for _, pin := range d.Files {
		if strings.HasPrefix(pin.Path, "publication/") && pin.Path != d.Receipt.Path && pin.Path != d.Ledger.Path {
			native = pin.Path
			break
		}
	}
	if native == "" {
		t.Fatal("SCRATCH_NATIVE_REQUIRED")
	}
	for _, name := range []string{d.Receipt.Path, d.Ledger.Path, native, "source/internal/censuscontinuation/multiseed_qualification_evidence_v2.go"} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(corpus, filepath.FromSlash(name))
			original, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := os.WriteFile(path, original, 0600); err != nil {
					t.Fatal(err)
				}
			}()
			bad := append([]byte{}, original...)
			bad[0] ^= 1
			if err := os.WriteFile(path, bad, 0600); err != nil {
				t.Fatal(err)
			}
			subject, err := reopenPreparedQualificationV2(t, corpus)
			if err == nil || subject.Trusted() || subject.Identity() != "" {
				t.Fatal("CORRUPT_PREDECESSOR_BEFORE_TRUST")
			}
			t.Log("CORRUPT_PREDECESSOR_BEFORE_TRUST PASS")
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			subject, err = reopenPreparedQualificationV2(t, corpus)
			if err == nil || subject.Trusted() || subject.Identity() != "" {
				t.Fatal("MISSING_PREDECESSOR_BEFORE_TRUST")
			}
			t.Log("MISSING_PREDECESSOR_BEFORE_TRUST PASS")
		})
	}
	if _, err := reopenPreparedQualificationV2(t, corpus); err != nil {
		t.Fatalf("SCRATCH_RESTORATION: %v", err)
	}
}
