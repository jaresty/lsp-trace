package locationexecutionv5

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	eval "lsp-trace/internal/adr0007locationv5"
	"lsp-trace/internal/locationqualificationv5"
)

const (
	FreezeRootIdentity      = locationqualificationv5.AuthorizedRootIdentity
	PredecessorRootIdentity = "sha256:5889080000000000000000000000000000000000000000000000000000000000"
	RootIdentity            = "location-intersection-v5-qualification-successor-2026-10-07"
	LedgerSchema            = "lsp-trace.adr0007.location-v5-execution.ledger.v3"
	ExecSchema              = "lsp-trace.adr0007.location-v5-execution.manifest.v3"
	AssignmentSchema        = "lsp-trace.adr0007.location-v5-execution.assignments.v2"
	AuthorizationSchema     = "lsp-trace.adr0007.location-v5-execution.authorization.v2"
	GateSchema              = "lsp-trace.adr0007.location-v5-execution.predispatch-go.v1"
)

type AssignmentFile struct {
	Schema            string           `json:"schema"`
	AttemptID         string           `json:"attemptID"`
	SuccessorIdentity string           `json:"successorIdentity"`
	Cases             []AssignmentCase `json:"cases"`
}

type AssignmentCase struct {
	CaseID   string         `json:"caseID"`
	Ordinal  int            `json:"ordinal"`
	Producer AssignmentRole `json:"producer"`
	Reviewer AssignmentRole `json:"reviewer"`
}

type AssignmentRole struct {
	AssignmentID string   `json:"assignmentID"`
	AttemptID    string   `json:"attemptID"`
	Role         string   `json:"role"`
	MayRead      []string `json:"mayRead"`
	Forbidden    []string `json:"forbidden"`
	MustWrite    []string `json:"mustWrite"`
}

type Authorization struct {
	Schema                           string   `json:"schema"`
	Status                           string   `json:"status"`
	SuccessorIdentity                string   `json:"successorIdentity"`
	ExecutionRoot                    string   `json:"executionRoot"`
	PredecessorRoot                  string   `json:"predecessorRoot"`
	PredecessorSealSHA256            string   `json:"predecessorSealSHA256"`
	PredecessorSealImmutable         bool     `json:"predecessorSealImmutable"`
	AuthorizedFreezeRootIdentity     string   `json:"authorizedFreezeRootIdentity"`
	FreezeRoot                       string   `json:"freezeRoot"`
	AttemptID                        string   `json:"attemptID"`
	ProducerAssignments              int      `json:"producerAssignments"`
	ReviewerAssignments              int      `json:"reviewerAssignments"`
	PhaseAPI                         []string `json:"phaseAPI"`
	DefaultMode                      string   `json:"defaultMode"`
	RequiresIndependentCommittedGate bool     `json:"requiresIndependentCommittedGate"`
	GateFile                         string   `json:"gateFile"`
	NoRealSemanticAttemptsBeforeGate bool     `json:"noRealSemanticAttemptsBeforeGate"`
	Accepted                         bool     `json:"accepted"`
	AuthorityCeiling                 int      `json:"authorityCeiling"`
	Completeness                     string   `json:"completeness"`
	FeatureIdentity                  string   `json:"featureIdentity"`
	ExternalInference                string   `json:"externalInference"`
}

type PredispatchGate struct {
	Schema                    string `json:"schema"`
	SuccessorIdentity         string `json:"successorIdentity"`
	ExecutionRoot             string `json:"executionRoot"`
	FreezeRootIdentity        string `json:"freezeRootIdentity"`
	AuthorizationDigest       string `json:"authorizationDigest"`
	AssignmentsDigest         string `json:"assignmentsDigest"`
	ToolingDigest             string `json:"toolingDigest"`
	Head                      string `json:"head"`
	DispatchID                string `json:"dispatchID"`
	Assignments               string `json:"assignments"`
	AuthorizedBy              string `json:"authorizedBy"`
	AuthorizedByAudit         string `json:"authorizedByAudit"`
	OneDispatch               string `json:"oneDispatch"`
	Sequence                  int    `json:"sequence"`
	ConsumedState             string `json:"consumedState"`
	Committed                 bool   `json:"committed"`
	AllowRealSemanticAttempts bool   `json:"allowRealSemanticAttempts"`
}

type BoundarySetup struct {
	ExpandedSources    int    `json:"expandedSources,omitempty"`
	ExpectedPrecedence string `json:"expectedPrecedence,omitempty"`
	MaxFrozenPaths     int    `json:"maxFrozenPaths,omitempty"`
	RawFrozenPaths     int    `json:"rawFrozenPaths,omitempty"`
	MaxWitnesses       int    `json:"maxWitnesses,omitempty"`
	UniqueWitnesses    int    `json:"uniqueWitnesses,omitempty"`
}

type Condition struct {
	Schema          string         `json:"schema"`
	BoundarySetup   *BoundarySetup `json:"boundarySetup,omitempty"`
	Cancel          bool           `json:"cancel"`
	DeadlineExpired bool           `json:"deadlineExpired"`
	LimitsProfile   string         `json:"limitsProfile"`
}

type Ledger struct {
	Schema  string        `json:"schema"`
	Entries []LedgerEntry `json:"entries"`
}

type LedgerEntry struct {
	Sequence    int             `json:"sequence"`
	PrevHash    string          `json:"prevHash"`
	Event       string          `json:"event"`
	Payload     json.RawMessage `json:"payload"`
	PayloadHash string          `json:"payloadHash"`
	EntryHash   string          `json:"entryHash"`
}

type ProducerAttempt struct {
	Schema            string   `json:"schema"`
	CaseID            string   `json:"caseID"`
	AssignmentID      string   `json:"assignmentID"`
	AttemptID         string   `json:"attemptID"`
	Role              string   `json:"role"`
	State             []string `json:"state"`
	ResultDigest      string   `json:"resultDigest"`
	RequestDigest     string   `json:"requestDigest"`
	BindingDigest     string   `json:"bindingDigest"`
	ConditionDigest   string   `json:"conditionDigest"`
	Custody           Custody  `json:"custody"`
	OracleAccess      bool     `json:"oracleAccess"`
	ExternalInference bool     `json:"externalInference"`
	SemanticRetry     bool     `json:"semanticRetry"`
	SemanticRepair    bool     `json:"semanticRepair"`
	Substitution      bool     `json:"substitution"`
	Authority         int      `json:"authority"`
	Accepted          bool     `json:"accepted"`
	Completeness      string   `json:"completeness"`
	FeatureIdentity   string   `json:"featureIdentity"`
}

type Review struct {
	Schema               string   `json:"schema"`
	CaseID               string   `json:"caseID"`
	AssignmentID         string   `json:"assignmentID"`
	AttemptID            string   `json:"attemptID"`
	Role                 string   `json:"role"`
	Verdict              string   `json:"verdict"`
	State                []string `json:"state"`
	ProducerResultDigest string   `json:"producerResultDigest"`
	OracleResultDigest   string   `json:"oracleResultDigest"`
	DerivationDigest     string   `json:"derivationDigest"`
	ByteEqual            bool     `json:"byteEqual"`
	DerivationRecomputed bool     `json:"derivationRecomputed"`
	Custody              Custody  `json:"custody"`
}

type Custody struct {
	CommandSource   string `json:"commandSource"`
	EvaluatorSource string `json:"evaluatorSource"`
	Executable      string `json:"executable"`
	Argv            string `json:"argv"`
	Input           string `json:"input"`
	Output          string `json:"output"`
	Stderr          string `json:"stderr"`
	Exit            int    `json:"exit"`
}

type Manifest struct {
	Schema                       string `json:"schema"`
	Status                       string `json:"status"`
	RootIdentity                 string `json:"rootIdentity"`
	Frozen230Unchanged           bool   `json:"frozen230Unchanged"`
	ProducerAttempts             int    `json:"producerAttempts"`
	ReviewerAttempts             int    `json:"reviewerAttempts"`
	ByteEquality                 int    `json:"byteEquality"`
	DerivationBindings           int    `json:"derivationBindings"`
	BoundaryReplays              int    `json:"boundaryReplays"`
	LeafRecount                  int    `json:"leafRecount"`
	SequenceMax                  int    `json:"sequenceMax"`
	LedgerHash                   string `json:"ledgerHash"`
	Retries                      int    `json:"retries"`
	SemanticRepairs              int    `json:"semanticRepairs"`
	Substitutions                int    `json:"substitutions"`
	ExternalInference            int    `json:"externalInference"`
	Authority                    int    `json:"authority"`
	Accepted                     bool   `json:"accepted"`
	Completeness                 string `json:"completeness"`
	FeatureIdentity              string `json:"featureIdentity"`
	FinalLocationCustodyGoIssued bool   `json:"finalLocationCustodyGoIssued"`
	Mode                         string `json:"mode"`
}

func hash(b []byte) string { s := sha256.Sum256(b); return "sha256:" + hex.EncodeToString(s[:]) }

func canon(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return append(b, '\n')
}

func canonicalRaw(b []byte) (json.RawMessage, error) {
	if err := rejectDuplicateKeys(b); err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	if err := rejectTrailing(dec); err != nil {
		return nil, err
	}
	out, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return append(json.RawMessage{}, out...), nil
}

func rejectDuplicateKeys(b []byte) error {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	if err := scanJSONValue(dec); err != nil {
		return err
	}
	return rejectTrailing(dec)
}

func scanJSONValue(dec *json.Decoder) error {
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	if d, ok := tok.(json.Delim); ok {
		switch d {
		case '{':
			seen := map[string]bool{}
			for dec.More() {
				ktok, err := dec.Token()
				if err != nil {
					return err
				}
				key, ok := ktok.(string)
				if !ok {
					return errors.New("object key is not string")
				}
				if seen[key] {
					return fmt.Errorf("duplicate json key %q", key)
				}
				seen[key] = true
				if err := scanJSONValue(dec); err != nil {
					return err
				}
			}
			end, err := dec.Token()
			if err != nil {
				return err
			}
			if end != json.Delim('}') {
				return errors.New("object not closed")
			}
		case '[':
			for dec.More() {
				if err := scanJSONValue(dec); err != nil {
					return err
				}
			}
			end, err := dec.Token()
			if err != nil {
				return err
			}
			if end != json.Delim(']') {
				return errors.New("array not closed")
			}
		default:
			return errors.New("unexpected delimiter")
		}
	}
	return nil
}

func rejectTrailing(dec *json.Decoder) error {
	var extra any
	err := dec.Decode(&extra)
	if err == io.EOF {
		return nil
	}
	if err == nil {
		return errors.New("trailing json")
	}
	return err
}

func readStrict(path string, dst any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if err := rejectTrailing(dec); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

func writeJSON(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, canon(v), 0644)
}

func writeJSONNew(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("create-new refused existing artifact %s", path)
		}
		return err
	}
	defer f.Close()
	_, err = f.Write(canon(v))
	return err
}

func readFile(path string) ([]byte, string, error) { b, e := os.ReadFile(path); return b, hash(b), e }

func Cases(assignments string) ([]AssignmentCase, error) {
	var a AssignmentFile
	if err := readStrict(assignments, &a); err != nil {
		return nil, err
	}
	if a.Schema != AssignmentSchema || a.AttemptID != "attempt-successor-01" || a.SuccessorIdentity != RootIdentity {
		return nil, errors.New("bad assignments")
	}
	if len(a.Cases) != 26 {
		return nil, fmt.Errorf("assignment cases %d", len(a.Cases))
	}
	seen := map[string]bool{}
	for i, c := range a.Cases {
		if c.Ordinal != i+1 {
			return nil, fmt.Errorf("bad ordinal %s", c.CaseID)
		}
		if seen[c.CaseID] {
			return nil, fmt.Errorf("duplicate case %s", c.CaseID)
		}
		seen[c.CaseID] = true
		wantP := fmt.Sprintf("successor-producer-%02d-%s-attempt-successor-01", i+1, c.CaseID)
		wantR := fmt.Sprintf("successor-reviewer-%02d-%s-attempt-successor-01", i+1, c.CaseID)
		if c.Producer.Role != "producer" || c.Reviewer.Role != "reviewer" || c.Producer.AttemptID != a.AttemptID || c.Reviewer.AttemptID != a.AttemptID {
			return nil, fmt.Errorf("bad role %d", i)
		}
		if c.Producer.AssignmentID != wantP || c.Reviewer.AssignmentID != wantR {
			return nil, fmt.Errorf("bad assignment binding %s", c.CaseID)
		}
		if !contains(c.Producer.Forbidden, "oracle-candidate") || !contains(c.Producer.Forbidden, "predecessor attempts") || !contains(c.Reviewer.Forbidden, "semantic repair") {
			return nil, fmt.Errorf("weak role restrictions %s", c.CaseID)
		}
	}
	return a.Cases, nil
}

func contains(xs []string, x string) bool {
	for _, y := range xs {
		if y == x {
			return true
		}
	}
	return false
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func AppendLedger(root, event string, payload any) error {
	p := filepath.Join(root, "EVENT_LEDGER.json")
	var l Ledger
	if b, err := os.ReadFile(p); err == nil && len(b) > 0 {
		if err := readStrict(p, &l); err != nil {
			return err
		}
		if err := VerifyLedger(root); err != nil {
			return err
		}
	} else {
		l.Schema = LedgerSchema
	}
	if l.Schema != LedgerSchema {
		return errors.New("ledger schema")
	}
	payloadRaw, err := canonicalRaw(canon(payload))
	if err != nil {
		return err
	}
	seq := len(l.Entries) + 1
	prev := "GENESIS"
	if seq > 1 {
		prev = l.Entries[len(l.Entries)-1].EntryHash
	}
	ph := hash(canon(payloadRaw))
	e := LedgerEntry{Sequence: seq, PrevHash: prev, Event: event, Payload: payloadRaw, PayloadHash: ph}
	e.EntryHash = hash(canon(struct {
		Sequence    int    `json:"sequence"`
		PrevHash    string `json:"prevHash"`
		Event       string `json:"event"`
		PayloadHash string `json:"payloadHash"`
	}{seq, prev, event, ph}))
	l.Entries = append(l.Entries, e)
	return writeJSON(p, l)
}

func VerifyLedger(root string) error {
	var l Ledger
	if err := readStrict(filepath.Join(root, "EVENT_LEDGER.json"), &l); err != nil {
		return err
	}
	if l.Schema != LedgerSchema {
		return errors.New("ledger schema")
	}
	prev := "GENESIS"
	for i, e := range l.Entries {
		if e.Sequence != i+1 || e.PrevHash != prev {
			return fmt.Errorf("ledger sequence %d", i+1)
		}
		payloadRaw, err := canonicalRaw(e.Payload)
		if err != nil {
			return fmt.Errorf("ledger payload %d: %w", i+1, err)
		}
		if hash(canon(payloadRaw)) != e.PayloadHash {
			return fmt.Errorf("ledger payload %d", i+1)
		}
		want := hash(canon(struct {
			Sequence    int    `json:"sequence"`
			PrevHash    string `json:"prevHash"`
			Event       string `json:"event"`
			PayloadHash string `json:"payloadHash"`
		}{e.Sequence, e.PrevHash, e.Event, e.PayloadHash}))
		if want != e.EntryHash {
			return fmt.Errorf("ledger entry %d", i+1)
		}
		prev = e.EntryHash
	}
	return nil
}

func AssertFreeze(frozenRoot, execRoot string) error {
	if got, err := locationqualificationv5.VerifyFreeze(frozenRoot); err != nil {
		return err
	} else if got != FreezeRootIdentity {
		return errors.New("freeze identity")
	}
	if execRoot != "" {
		var a Authorization
		if err := readStrict(filepath.Join(execRoot, "AUTHORIZATION.json"), &a); err != nil {
			return err
		}
		if a.Schema != AuthorizationSchema || a.SuccessorIdentity != RootIdentity || a.AuthorizedFreezeRootIdentity != FreezeRootIdentity || !a.PredecessorSealImmutable || a.PredecessorSealSHA256 != PredecessorRootIdentity || a.DefaultMode != "SIMULATE_ONLY" {
			return errors.New("authorization")
		}
	}
	return nil
}

func sourceDigest(root string, dirs ...string) (string, error) {
	var parts []string
	for _, d := range dirs {
		base := filepath.Join(root, d)
		err := filepath.WalkDir(base, func(p string, de fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if de.IsDir() || filepath.Ext(p) != ".go" {
				return nil
			}
			b, e := os.ReadFile(p)
			if e != nil {
				return e
			}
			rel, _ := filepath.Rel(root, p)
			parts = append(parts, filepath.ToSlash(rel)+"\x00"+hash(b))
			return nil
		})
		if err != nil {
			return "", err
		}
	}
	sort.Strings(parts)
	return hash([]byte(strings.Join(parts, "\n"))), nil
}
func exeDigest() string {
	if p, e := os.Executable(); e == nil {
		if b, e := os.ReadFile(p); e == nil {
			return hash(b)
		}
	}
	return "sha256:unavailable"
}
func headDigest(repoRoot string) string {
	cmd := exec.Command("git", "-C", repoRoot, "rev-parse", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "sha256:unavailable"
	}
	return strings.TrimSpace(string(out))
}
func commandDigest() string {
	if len(os.Args) > 0 {
		if p, e := exec.LookPath(os.Args[0]); e == nil {
			if b, e := os.ReadFile(p); e == nil {
				return hash(b)
			}
		}
	}
	return exeDigest()
}
func control(c Condition) eval.StaticControl {
	return eval.StaticControl{Cancel: c.Cancel, Deadline: c.DeadlineExpired}
}

func checkGate(execRoot, repoRoot string) error {
	var g PredispatchGate
	if err := readStrict(filepath.Join(execRoot, "PREDISPATCH_GO.json"), &g); err != nil {
		return fmt.Errorf("real execution requires independent committed gate: %w", err)
	}
	return checkGateValue(execRoot, repoRoot, g, true)
}

func checkBlockedAudit(execRoot, repoRoot string) error {
	var g PredispatchGate
	if err := readStrict(filepath.Join(execRoot, "PREDISPATCH_AUDIT_BLOCKED.json"), &g); err != nil {
		return err
	}
	return checkGateValue(execRoot, repoRoot, g, false)
}

func checkGateValue(execRoot, repoRoot string, g PredispatchGate, committed bool) error {
	ad, err := fileDigest(filepath.Join(execRoot, "AUTHORIZATION.json"))
	if err != nil {
		return err
	}
	asd, err := fileDigest(filepath.Join(execRoot, "ASSIGNMENTS.json"))
	if err != nil {
		return err
	}
	td, err := sourceDigest(repoRoot, "internal/locationexecutionv5", "internal/adr0007locationv5", "internal/sourceadmissionv2")
	if err != nil {
		return err
	}
	if g.Schema != GateSchema || g.SuccessorIdentity != RootIdentity || filepath.ToSlash(g.ExecutionRoot) != "docs/pilot/adr0007/experiment/"+RootIdentity || g.FreezeRootIdentity != FreezeRootIdentity || g.AuthorizationDigest != ad || g.AssignmentsDigest != asd || g.ToolingDigest != td || g.Head != headDigest(repoRoot) || g.DispatchID != RootIdentity+":"+g.Head+":"+g.ToolingDigest || g.Assignments != "ASSIGNMENTS.json" || g.AuthorizedBy != "independent-value:predispatch-authorized" || g.AuthorizedByAudit != "independent-audit:authorizedBy-not-producer" || g.OneDispatch != "phase-api-only" || g.Sequence != 1 || g.ConsumedState != "create-new" || g.Committed != committed || g.AllowRealSemanticAttempts != committed {
		return errors.New("gate mismatch")
	}
	cases, err := Cases(filepath.Join(execRoot, "ASSIGNMENTS.json"))
	if err != nil {
		return err
	}
	for _, c := range cases {
		if !equalStrings(c.Producer.MayRead, []string{"frozen inputs only", "condition file", "binding file when present"}) || !equalStrings(c.Producer.Forbidden, []string{"oracle-candidate", "derivations", "reviewer output", "external inference", "semantic retry", "semantic repair", "predecessor attempts"}) || !equalStrings(c.Producer.MustWrite, []string{"RESULT.json", "PRODUCER_ATTEMPT.json"}) {
			return fmt.Errorf("producer assignment arrays %s", c.CaseID)
		}
		if !equalStrings(c.Reviewer.MayRead, []string{"producer committed result", "frozen oracle result", "frozen oracle derivation"}) || !equalStrings(c.Reviewer.Forbidden, []string{"external inference", "semantic retry", "semantic repair", "producer code changes", "predecessor attempts"}) || !equalStrings(c.Reviewer.MustWrite, []string{"REVIEW.json"}) {
			return fmt.Errorf("reviewer assignment arrays %s", c.CaseID)
		}
	}
	return nil
}
func fileDigest(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return hash(b), nil
}

func Simulate(execRoot, frozenRoot, repoRoot string) error {
	if err := AssertFreeze(frozenRoot, execRoot); err != nil {
		return err
	}
	cases, err := Cases(filepath.Join(execRoot, "ASSIGNMENTS.json"))
	if err != nil {
		return err
	}
	for _, artifact := range []string{"attempts", "EVENT_LEDGER.json", "EXECUTION_MANIFEST.json", "CORRECTION_MANIFEST.json", "PREDISPATCH_AUDIT_BLOCKED.json"} {
		if _, err := os.Stat(filepath.Join(execRoot, artifact)); err == nil {
			return fmt.Errorf("simulate create-new fail closed: %s exists", artifact)
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	if err := writeBlockedAudit(execRoot, repoRoot); err != nil {
		return err
	}
	if err := AppendLedger(execRoot, "plan", map[string]any{"rootIdentity": RootIdentity, "phase": "Plan", "cases": len(cases), "predecessorSeal": PredecessorRootIdentity}); err != nil {
		return err
	}
	if err := AppendLedger(execRoot, "simulate", map[string]any{"rootIdentity": RootIdentity, "phase": "Simulate", "producerPlanned": 26, "reviewerPlanned": 26, "boundaryPlanned": 4, "realSemanticAttempts": 0}); err != nil {
		return err
	}
	var l Ledger
	if err := readStrict(filepath.Join(execRoot, "EVENT_LEDGER.json"), &l); err != nil {
		return err
	}
	m := Manifest{Schema: ExecSchema, Status: "PREDISPATCH_BLOCKED_SIMULATION_ONLY", RootIdentity: RootIdentity, Frozen230Unchanged: true, ProducerAttempts: 0, ReviewerAttempts: 0, ByteEquality: 0, DerivationBindings: 0, BoundaryReplays: 0, LeafRecount: 0, SequenceMax: len(l.Entries), LedgerHash: l.Entries[len(l.Entries)-1].EntryHash, Completeness: "UNKNOWN", FeatureIdentity: "UNRESOLVED", Mode: "SIMULATE_ONLY"}
	if err := writeJSONNew(filepath.Join(execRoot, "EXECUTION_MANIFEST.json"), m); err != nil {
		return err
	}
	if err := writeJSONNew(filepath.Join(execRoot, "CORRECTION_MANIFEST.json"), map[string]any{"schema": "lsp-trace.adr0007.location-v5-execution.correction-manifest.v3", "rootIdentity": RootIdentity, "blockedAudit": "PREDISPATCH_AUDIT_BLOCKED.json", "corrections": []string{"removed deletion/overwrite from simulation", "child modes private-token only", "head/tooling/dispatch-bound gate", "complete derivation recomputation", "independent verifier custody/count checks"}, "semanticOutputs": 0, "predecessorImmutable": true}); err != nil {
		return err
	}
	return AppendLedger(execRoot, "simulation_manifest", m)
}

func writeBlockedAudit(execRoot, repoRoot string) error {
	ad, err := fileDigest(filepath.Join(execRoot, "AUTHORIZATION.json"))
	if err != nil {
		return err
	}
	asd, err := fileDigest(filepath.Join(execRoot, "ASSIGNMENTS.json"))
	if err != nil {
		return err
	}
	td, err := sourceDigest(repoRoot, "internal/locationexecutionv5", "internal/adr0007locationv5", "internal/sourceadmissionv2")
	if err != nil {
		return err
	}
	head := headDigest(repoRoot)
	g := PredispatchGate{Schema: GateSchema, SuccessorIdentity: RootIdentity, ExecutionRoot: "docs/pilot/adr0007/experiment/" + RootIdentity, FreezeRootIdentity: FreezeRootIdentity, AuthorizationDigest: ad, AssignmentsDigest: asd, ToolingDigest: td, Head: head, DispatchID: RootIdentity + ":" + head + ":" + td, Assignments: "ASSIGNMENTS.json", AuthorizedBy: "independent-value:predispatch-authorized", AuthorizedByAudit: "independent-audit:authorizedBy-not-producer", OneDispatch: "phase-api-only", Sequence: 1, ConsumedState: "create-new", Committed: false, AllowRealSemanticAttempts: false}
	return writeJSONNew(filepath.Join(execRoot, "PREDISPATCH_AUDIT_BLOCKED.json"), g)
}

func Run(execRoot, frozenRoot, repoRoot string) error {
	return RunPhase(execRoot, frozenRoot, repoRoot, "Simulate")
}

func RunPhase(execRoot, frozenRoot, repoRoot, phase string) error {
	switch phase {
	case "Plan", "Simulate":
		return Simulate(execRoot, frozenRoot, repoRoot)
	case "Producers":
		if err := checkGate(execRoot, repoRoot); err != nil {
			return err
		}
		return runProducers(execRoot, frozenRoot, repoRoot)
	case "Reviewers":
		if err := checkGate(execRoot, repoRoot); err != nil {
			return err
		}
		return runReviewers(execRoot, frozenRoot, repoRoot)
	case "Boundaries":
		if err := checkGate(execRoot, repoRoot); err != nil {
			return err
		}
		return runBoundaries(execRoot, frozenRoot, repoRoot)
	case "Reconcile":
		if err := checkGate(execRoot, repoRoot); err != nil {
			return err
		}
		return reconcile(execRoot)
	case "Verify":
		return Verify(execRoot, frozenRoot, repoRoot)
	case "__producer":
		if len(os.Args) != 6 {
			return errors.New("producer child requires case")
		}
		if err := checkChildToken(execRoot, repoRoot, phase, os.Args[5]); err != nil {
			return err
		}
		return producerChild(execRoot, frozenRoot, repoRoot, os.Args[5])
	case "__reviewer":
		if len(os.Args) != 6 {
			return errors.New("reviewer child requires case")
		}
		if err := checkChildToken(execRoot, repoRoot, phase, os.Args[5]); err != nil {
			return err
		}
		return reviewerChild(execRoot, frozenRoot, repoRoot, os.Args[5])
	default:
		return fmt.Errorf("unknown phase %q", phase)
	}
}

func runProducers(execRoot, frozenRoot, repoRoot string) error {
	if err := AssertFreeze(frozenRoot, execRoot); err != nil {
		return err
	}
	cases, err := Cases(filepath.Join(execRoot, "ASSIGNMENTS.json"))
	if err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(execRoot, "attempts")); err == nil {
		return errors.New("producers create-new fail closed: attempts exists")
	}
	if _, err := os.Stat(filepath.Join(execRoot, "EVENT_LEDGER.json")); err == nil {
		return errors.New("producers create-new fail closed: ledger exists")
	}
	if err := AppendLedger(execRoot, "producers_phase_start", map[string]any{"phase": "Producers", "count": len(cases)}); err != nil {
		return err
	}
	for _, c := range cases {
		if err := runChild(execRoot, frozenRoot, repoRoot, "__producer", c.CaseID); err != nil {
			return err
		}
		var pa ProducerAttempt
		if err := readStrict(filepath.Join(execRoot, "attempts", c.CaseID, "PRODUCER_ATTEMPT.json"), &pa); err != nil {
			return err
		}
		if err := AppendLedger(execRoot, "producer_attempt", pa); err != nil {
			return err
		}
	}
	return AppendLedger(execRoot, "producers_phase_complete", map[string]any{"phase": "Producers", "count": len(cases)})
}

func producerChild(execRoot, frozenRoot, repoRoot, caseID string) error {
	cases, err := Cases(filepath.Join(execRoot, "ASSIGNMENTS.json"))
	if err != nil {
		return err
	}
	var c AssignmentCase
	ok := false
	for _, x := range cases {
		if x.CaseID == caseID {
			c = x
			ok = true
		}
	}
	if !ok {
		return fmt.Errorf("unknown case %s", caseID)
	}
	indir := filepath.Join(frozenRoot, "inputs", c.CaseID)
	raw, rd, err := readFile(filepath.Join(indir, "REQUEST.raw.json"))
	if err != nil {
		return err
	}
	bind, bd, err := readFile(filepath.Join(indir, "BINDING.json"))
	if err != nil {
		bind = []byte{}
		bd = hash(bind)
	}
	var cond Condition
	if err := readStrict(filepath.Join(indir, "CONDITION.json"), &cond); err != nil {
		return err
	}
	cb, cd, err := readFile(filepath.Join(indir, "CONDITION.json"))
	if err != nil {
		return err
	}
	res, err := eval.Evaluate(raw, bind, control(cond), eval.PublishedLimits())
	if err != nil {
		return err
	}
	out := canon(res)
	od := hash(out)
	dir := filepath.Join(execRoot, "attempts", c.CaseID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(dir, "RESULT.json")); err == nil {
		return fmt.Errorf("producer create-new refused existing RESULT.json for %s", c.CaseID)
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "RESULT.json"), out, 0644); err != nil {
		return err
	}
	cmdSrc := commandDigest()
	evalSrc, err := sourceDigest(repoRoot, "internal/adr0007locationv5", "internal/sourceadmissionv2")
	if err != nil {
		return err
	}
	pa := ProducerAttempt{Schema: "lsp-trace.adr0007.location-v5-execution.producer-attempt.v3", CaseID: c.CaseID, AssignmentID: c.Producer.AssignmentID, AttemptID: c.Producer.AttemptID, Role: "producer", State: []string{"ASSIGNMENT_BOUND", "CHILD_PROCESS_ISOLATED", "FROZEN_INPUT_EVALUATED", "COMMITTED"}, ResultDigest: od, RequestDigest: rd, BindingDigest: bd, ConditionDigest: cd, Custody: Custody{CommandSource: cmdSrc, EvaluatorSource: evalSrc, Executable: exeDigest(), Argv: hash(canon(os.Args)), Input: hash(append(append(raw, bind...), cb...)), Output: od, Stderr: hash(nil), Exit: 0}, Completeness: "UNKNOWN", FeatureIdentity: "UNRESOLVED"}
	return writeJSONNew(filepath.Join(dir, "PRODUCER_ATTEMPT.json"), pa)
}

func runReviewers(execRoot, frozenRoot, repoRoot string) error {
	cases, err := Cases(filepath.Join(execRoot, "ASSIGNMENTS.json"))
	if err != nil {
		return err
	}
	for _, c := range cases {
		if _, err := os.Stat(filepath.Join(execRoot, "attempts", c.CaseID, "PRODUCER_ATTEMPT.json")); err != nil {
			return fmt.Errorf("producer incomplete %s", c.CaseID)
		}
	}
	if err := AppendLedger(execRoot, "reviewers_phase_start", map[string]any{"phase": "Reviewers", "count": len(cases)}); err != nil {
		return err
	}
	for _, c := range cases {
		if err := runChild(execRoot, frozenRoot, repoRoot, "__reviewer", c.CaseID); err != nil {
			return err
		}
		var rv Review
		if err := readStrict(filepath.Join(execRoot, "attempts", c.CaseID, "REVIEW.json"), &rv); err != nil {
			return err
		}
		if err := AppendLedger(execRoot, "review_attempt", rv); err != nil {
			return err
		}
	}
	return AppendLedger(execRoot, "reviewers_phase_complete", map[string]any{"phase": "Reviewers", "count": len(cases)})
}

type Derivation struct {
	Schema          string   `json:"schema"`
	CaseID          string   `json:"caseId"`
	Inputs          []string `json:"inputs"`
	Algorithm       string   `json:"algorithm"`
	RequestDigest   string   `json:"requestDigest,omitempty"`
	BindingDigest   string   `json:"bindingDigest,omitempty"`
	ConditionDigest string   `json:"conditionDigest,omitempty"`
	ResultDigest    string   `json:"resultDigest,omitempty"`
	Outcome         string   `json:"outcome"`
	Detail          string   `json:"detail"`
}

func reviewerChild(execRoot, frozenRoot, repoRoot, caseID string) error {
	cases, err := Cases(filepath.Join(execRoot, "ASSIGNMENTS.json"))
	if err != nil {
		return err
	}
	var c AssignmentCase
	ok := false
	for _, x := range cases {
		if x.CaseID == caseID {
			c = x
			ok = true
		}
	}
	if !ok {
		return fmt.Errorf("unknown case %s", caseID)
	}
	dir := filepath.Join(execRoot, "attempts", c.CaseID)
	prod, pd, err := readFile(filepath.Join(dir, "RESULT.json"))
	if err != nil {
		return err
	}
	oracle, od, err := readFile(filepath.Join(frozenRoot, "oracle-candidate", "cases", c.CaseID, "RESULT.json"))
	if err != nil {
		return err
	}
	derBytes, dd, err := readFile(filepath.Join(frozenRoot, "oracle-candidate", "cases", c.CaseID, "DERIVATION.json"))
	if err != nil {
		return err
	}
	var d Derivation
	if err := readStrict(filepath.Join(frozenRoot, "oracle-candidate", "cases", c.CaseID, "DERIVATION.json"), &d); err != nil {
		return err
	}
	if d.Schema != "lsp-trace.adr0007.location-oracle-derivation.private.v5" || d.CaseID != c.CaseID || d.Algorithm != "internal/adr0007locationoraclev5" || !equalStrings(d.Inputs, []string{"REQUEST.raw.json", "BINDING.json|BINDING.ABSENT", "CONDITION.json"}) {
		return fmt.Errorf("derivation schema %s", c.CaseID)
	}
	raw, rd, err := readFile(filepath.Join(frozenRoot, "inputs", c.CaseID, "REQUEST.raw.json"))
	if err != nil {
		return err
	}
	bind, bd, err := readFile(filepath.Join(frozenRoot, "inputs", c.CaseID, "BINDING.json"))
	if err != nil {
		bind = []byte{}
		bd = hash(bind)
	}
	var cond Condition
	if err := readStrict(filepath.Join(frozenRoot, "inputs", c.CaseID, "CONDITION.json"), &cond); err != nil {
		return err
	}
	_, cd, err := readFile(filepath.Join(frozenRoot, "inputs", c.CaseID, "CONDITION.json"))
	if err != nil {
		return err
	}
	res, err := eval.Evaluate(raw, bind, control(cond), eval.PublishedLimits())
	if err != nil {
		return err
	}
	recomputed := canon(res)
	if !bytes.Equal(recomputed, prod) || !bytes.Equal(prod, oracle) {
		return fmt.Errorf("strict equality %s", c.CaseID)
	}
	var rr eval.Result
	if err := json.Unmarshal(recomputed, &rr); err != nil {
		return err
	}
	recomputedDerivation := Derivation{Schema: d.Schema, CaseID: c.CaseID, Algorithm: d.Algorithm, RequestDigest: d.RequestDigest, BindingDigest: d.BindingDigest, ConditionDigest: d.ConditionDigest, ResultDigest: d.ResultDigest, Outcome: rr.Outcome, Detail: rr.Detail, Inputs: []string{"REQUEST.raw.json", "BINDING.json|BINDING.ABSENT", "CONDITION.json"}}
	if d.RequestDigest != "" && d.RequestDigest != rd || d.BindingDigest != "" && d.BindingDigest != bd || d.ConditionDigest != "" && d.ConditionDigest != cd || d.ResultDigest != "" && d.ResultDigest != hash(recomputed) {
		return fmt.Errorf("derivation digest recompute %s", c.CaseID)
	}
	if !bytes.Equal(canon(recomputedDerivation), canon(d)) {
		return fmt.Errorf("derivation recompute %s", c.CaseID)
	}
	cmdSrc := commandDigest()
	evalSrc, err := sourceDigest(repoRoot, "internal/adr0007locationv5", "internal/sourceadmissionv2")
	if err != nil {
		return err
	}
	rv := Review{Schema: "lsp-trace.adr0007.location-v5-execution.review.v3", CaseID: c.CaseID, AssignmentID: c.Reviewer.AssignmentID, AttemptID: c.Reviewer.AttemptID, Role: "reviewer", Verdict: "ACCEPT", State: []string{"ASSIGNMENT_BOUND", "SEPARATE_CHILD_PROCESS", "STRICT_DERIVATION_RECOMPUTED", "COMMITTED"}, ProducerResultDigest: pd, OracleResultDigest: od, DerivationDigest: dd, ByteEqual: true, DerivationRecomputed: true, Custody: Custody{CommandSource: cmdSrc, EvaluatorSource: evalSrc, Executable: exeDigest(), Argv: hash(canon(os.Args)), Input: hash(append(prod, derBytes...)), Output: hash(canon(map[string]any{"byteEqual": true, "derivationRecomputed": true})), Stderr: hash(nil), Exit: 0}}
	return writeJSONNew(filepath.Join(dir, "REVIEW.json"), rv)
}

func runChild(execRoot, frozenRoot, repoRoot, phase, caseID string) error {
	cmd := exec.Command(os.Args[0], execRoot, frozenRoot, repoRoot, phase, caseID)
	cmd.Env = append(os.Environ(), "LOCATIONEXECUTIONV5_CHILD_TOKEN="+childToken(execRoot, repoRoot, phase, caseID))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %w: %s", phase, caseID, err, string(out))
	}
	return nil
}

func childToken(execRoot, repoRoot, phase, caseID string) string {
	g := PredispatchGate{}
	_ = readStrict(filepath.Join(execRoot, "PREDISPATCH_GO.json"), &g)
	return hash(canon(map[string]any{"dispatchID": g.DispatchID, "head": headDigest(repoRoot), "phase": phase, "caseID": caseID, "assignmentRoot": filepath.Join(execRoot, "ASSIGNMENTS.json")}))
}

func checkChildToken(execRoot, repoRoot, phase, caseID string) error {
	if os.Getenv("LOCATIONEXECUTIONV5_CHILD_TOKEN") != childToken(execRoot, repoRoot, phase, caseID) {
		return errors.New("internal child mode unavailable without exact authenticated child token")
	}
	return nil
}

type Boundary struct {
	Schema, Variant                                    string
	BaseOutputBytes, BaseWork, MaxOutputBytes, MaxWork uint64
}

func runBoundaries(execRoot, frozenRoot, repoRoot string) error {
	cases, err := Cases(filepath.Join(execRoot, "ASSIGNMENTS.json"))
	if err != nil {
		return err
	}
	for _, c := range cases {
		if _, err := os.Stat(filepath.Join(execRoot, "attempts", c.CaseID, "REVIEW.json")); err != nil {
			return fmt.Errorf("review incomplete %s", c.CaseID)
		}
	}
	if err := AppendLedger(execRoot, "boundaries_phase_start", map[string]any{"phase": "Boundaries", "count": 4}); err != nil {
		return err
	}
	for _, name := range []string{"W", "W-1", "B", "B-1"} {
		bdir := filepath.Join(frozenRoot, "oracle-candidate", "boundaries", name)
		var bo Boundary
		if err := readStrict(filepath.Join(bdir, "BOUNDARY.json"), &bo); err != nil {
			return err
		}
		if bo.Schema != "lsp-trace.adr0007.location-boundary-input.private.v5" || bo.Variant != name {
			return fmt.Errorf("boundary schema %s", name)
		}
		raw, _, err := readFile(filepath.Join(bdir, "REQUEST.raw.json"))
		if err != nil {
			return err
		}
		bind, _, err := readFile(filepath.Join(bdir, "BINDING.json"))
		if err != nil {
			bind = []byte{}
		}
		var cond Condition
		_ = cond
		limits := eval.PublishedLimits()
		limits.MaxWork = bo.MaxWork
		limits.MaxOutputBytes = bo.MaxOutputBytes
		res, err := eval.Evaluate(raw, bind, eval.StaticControl{}, limits)
		if err != nil {
			return err
		}
		actual := canon(res)
		exp, _, err := readFile(filepath.Join(bdir, "RESULT.json"))
		if err != nil {
			return err
		}
		if !bytes.Equal(actual, exp) {
			return fmt.Errorf("boundary mismatch %s", name)
		}
		payload := map[string]any{"boundary": name, "boundaryDigest": hash(canon(bo)), "actualDigest": hash(actual), "expectedDigest": hash(exp), "outcome": "MATCH", "custody": map[string]any{"evaluatorSource": "internal/adr0007locationv5", "limitMaxWork": bo.MaxWork, "limitMaxOutputBytes": bo.MaxOutputBytes}}
		if err := writeJSONNew(filepath.Join(execRoot, "boundaries", name, "BOUNDARY_REPLAY.json"), payload); err != nil {
			return err
		}
		if err := AppendLedger(execRoot, "boundary_replay", payload); err != nil {
			return err
		}
	}
	return AppendLedger(execRoot, "boundaries_phase_complete", map[string]any{"phase": "Boundaries", "count": 4})
}

func reconcile(execRoot string) error {
	if err := verifyPhaseArtifacts(execRoot, ""); err != nil {
		return err
	}
	pc, rc, bc := 0, 0, 0
	filepath.WalkDir(filepath.Join(execRoot, "attempts"), func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			switch filepath.Base(p) {
			case "PRODUCER_ATTEMPT.json":
				pc++
			case "REVIEW.json":
				rc++
			}
		}
		return nil
	})
	filepath.WalkDir(filepath.Join(execRoot, "boundaries"), func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && filepath.Base(p) == "BOUNDARY_REPLAY.json" {
			bc++
		}
		return nil
	})
	if pc != 26 || rc != 26 || bc != 4 {
		return fmt.Errorf("reconcile counts producers=%d reviewers=%d boundaries=%d", pc, rc, bc)
	}
	var l Ledger
	if err := readStrict(filepath.Join(execRoot, "EVENT_LEDGER.json"), &l); err != nil {
		return err
	}
	m := Manifest{Schema: ExecSchema, Status: "PREDISPATCH_BLOCKED_SUCCESSOR_REPLAY_COMPLETE", RootIdentity: RootIdentity, Frozen230Unchanged: true, ProducerAttempts: pc, ReviewerAttempts: rc, ByteEquality: rc, DerivationBindings: rc, BoundaryReplays: bc, LeafRecount: pc + rc + bc, SequenceMax: len(l.Entries), LedgerHash: l.Entries[len(l.Entries)-1].EntryHash, Completeness: "UNKNOWN", FeatureIdentity: "UNRESOLVED", Mode: "GATED_REAL"}
	if err := writeJSONNew(filepath.Join(execRoot, "EXECUTION_MANIFEST.json"), m); err != nil {
		return err
	}
	return AppendLedger(execRoot, "reconcile_manifest", m)
}

func Verify(execRoot, frozenRoot, repoRoot string) error {
	if err := checkGate(execRoot, repoRoot); err != nil {
		return err
	}
	if err := AssertFreeze(frozenRoot, execRoot); err != nil {
		return err
	}
	cases, err := Cases(filepath.Join(execRoot, "ASSIGNMENTS.json"))
	if err != nil {
		return err
	}
	if err := VerifyLedger(execRoot); err != nil {
		return err
	}
	if err := verifyPhaseArtifacts(execRoot, frozenRoot); err != nil {
		return err
	}
	bindings := 0
	for _, c := range cases {
		dir := filepath.Join(execRoot, "attempts", c.CaseID)
		var pa ProducerAttempt
		if err := readStrict(filepath.Join(dir, "PRODUCER_ATTEMPT.json"), &pa); err != nil {
			return err
		}
		if pa.AssignmentID != c.Producer.AssignmentID || pa.CaseID != c.CaseID || pa.OracleAccess || pa.ExternalInference || pa.SemanticRetry || pa.SemanticRepair || pa.Substitution || pa.Authority != 0 || pa.Accepted || !contains(pa.State, "CHILD_PROCESS_ISOLATED") {
			return fmt.Errorf("producer custody %s", c.CaseID)
		}
		raw, rd, err := readFile(filepath.Join(frozenRoot, "inputs", c.CaseID, "REQUEST.raw.json"))
		if err != nil {
			return err
		}
		bind, bd, err := readFile(filepath.Join(frozenRoot, "inputs", c.CaseID, "BINDING.json"))
		if err != nil {
			bind = []byte{}
			bd = hash(bind)
		}
		var cond Condition
		if err := readStrict(filepath.Join(frozenRoot, "inputs", c.CaseID, "CONDITION.json"), &cond); err != nil {
			return err
		}
		_, cd, _ := readFile(filepath.Join(frozenRoot, "inputs", c.CaseID, "CONDITION.json"))
		res, err := eval.Evaluate(raw, bind, control(cond), eval.PublishedLimits())
		if err != nil {
			return err
		}
		prod := canon(res)
		persisted, persistedDigest, err := readFile(filepath.Join(dir, "RESULT.json"))
		if err != nil {
			return err
		}
		var persistedResult eval.Result
		if err := readStrict(filepath.Join(dir, "RESULT.json"), &persistedResult); err != nil {
			return err
		}
		if !bytes.Equal(canon(persistedResult), persisted) || !bytes.Equal(persisted, prod) || persistedDigest != pa.ResultDigest || hash(prod) != pa.ResultDigest || pa.RequestDigest != rd || pa.BindingDigest != bd || pa.ConditionDigest != cd {
			return fmt.Errorf("producer persisted-result recompute %s", c.CaseID)
		}
		var rv Review
		if err := readStrict(filepath.Join(dir, "REVIEW.json"), &rv); err != nil {
			return err
		}
		ob, od, err := readFile(filepath.Join(frozenRoot, "oracle-candidate", "cases", c.CaseID, "RESULT.json"))
		if err != nil {
			return err
		}
		var d Derivation
		if err := readStrict(filepath.Join(frozenRoot, "oracle-candidate", "cases", c.CaseID, "DERIVATION.json"), &d); err != nil {
			return err
		}
		recomputedDerivation := Derivation{Schema: d.Schema, CaseID: c.CaseID, Algorithm: d.Algorithm, RequestDigest: d.RequestDigest, BindingDigest: d.BindingDigest, ConditionDigest: d.ConditionDigest, ResultDigest: d.ResultDigest, Outcome: res.Outcome, Detail: res.Detail, Inputs: []string{"REQUEST.raw.json", "BINDING.json|BINDING.ABSENT", "CONDITION.json"}}
		if d.RequestDigest != "" && d.RequestDigest != rd || d.BindingDigest != "" && d.BindingDigest != bd || d.ConditionDigest != "" && d.ConditionDigest != cd || d.ResultDigest != "" && d.ResultDigest != hash(prod) {
			return fmt.Errorf("derivation digest recompute %s", c.CaseID)
		}
		if rv.AssignmentID != c.Reviewer.AssignmentID || !bytes.Equal(prod, ob) || rv.ProducerResultDigest != pa.ResultDigest || rv.OracleResultDigest != od || !rv.ByteEqual || rv.Verdict != "ACCEPT" || !rv.DerivationRecomputed || !contains(rv.State, "SEPARATE_CHILD_PROCESS") || !bytes.Equal(canon(recomputedDerivation), canon(d)) {
			return fmt.Errorf("review recompute %s", c.CaseID)
		}
		bindings++
	}
	for _, name := range []string{"W", "W-1", "B", "B-1"} {
		if _, err := os.Stat(filepath.Join(execRoot, "boundaries", name, "BOUNDARY_REPLAY.json")); err != nil {
			return fmt.Errorf("boundary missing %s", name)
		}
	}
	var m Manifest
	if err := readStrict(filepath.Join(execRoot, "EXECUTION_MANIFEST.json"), &m); err != nil {
		return err
	}
	if m.ProducerAttempts != 26 || m.ReviewerAttempts != 26 || m.ByteEquality != 26 || m.DerivationBindings != bindings || bindings != 26 || m.BoundaryReplays != 4 || m.FinalLocationCustodyGoIssued || m.Accepted || m.LeafRecount != m.ProducerAttempts+m.ReviewerAttempts+m.BoundaryReplays {
		return errors.New("manifest counts")
	}
	return nil
}

func verifyPhaseArtifacts(execRoot, frozenRoot string) error {
	cases, err := Cases(filepath.Join(execRoot, "ASSIGNMENTS.json"))
	if err != nil {
		return err
	}
	producer, reviewer := 0, 0
	for _, c := range cases {
		if _, err := os.Stat(filepath.Join(execRoot, "attempts", c.CaseID, "RESULT.json")); err != nil {
			return fmt.Errorf("missing persisted producer result %s", c.CaseID)
		}
		if _, err := os.Stat(filepath.Join(execRoot, "attempts", c.CaseID, "PRODUCER_ATTEMPT.json")); err != nil {
			return fmt.Errorf("missing producer attempt %s", c.CaseID)
		}
		producer++
		if _, err := os.Stat(filepath.Join(execRoot, "attempts", c.CaseID, "REVIEW.json")); err != nil {
			return fmt.Errorf("missing review %s", c.CaseID)
		}
		reviewer++
	}
	boundary := 0
	for _, name := range []string{"W", "W-1", "B", "B-1"} {
		if _, err := os.Stat(filepath.Join(execRoot, "boundaries", name, "BOUNDARY_REPLAY.json")); err != nil {
			return fmt.Errorf("boundary missing %s", name)
		}
		boundary++
	}
	if producer != 26 || reviewer != 26 || boundary != 4 {
		return fmt.Errorf("leaf counts producer=%d reviewer=%d boundary=%d", producer, reviewer, boundary)
	}
	return nil
}
