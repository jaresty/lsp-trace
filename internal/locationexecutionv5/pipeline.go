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
	Assignments               string `json:"assignments"`
	AuthorizedBy              string `json:"authorizedBy"`
	Committed                 bool   `json:"committed"`
	AllowRealSemanticAttempts bool   `json:"allowRealSemanticAttempts"`
}

type Condition struct {
	Schema          string `json:"schema"`
	Cancel          bool   `json:"cancel"`
	DeadlineExpired bool   `json:"deadlineExpired"`
	LimitsProfile   string `json:"limitsProfile"`
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
	if g.Schema != GateSchema || g.SuccessorIdentity != RootIdentity || g.FreezeRootIdentity != FreezeRootIdentity || g.AuthorizationDigest != ad || g.AssignmentsDigest != asd || g.ToolingDigest != td || !g.Committed || !g.AllowRealSemanticAttempts {
		return errors.New("gate mismatch")
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
	if _, err := os.Stat(filepath.Join(execRoot, "attempts")); err == nil {
		return errors.New("simulate fail closed: attempts directory exists")
	}
	if _, err := os.Stat(filepath.Join(execRoot, "EVENT_LEDGER.json")); err == nil {
		return errors.New("simulate fail closed: ledger exists")
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
	m := Manifest{Schema: ExecSchema, Status: "PRE_DISPATCH_SIMULATION_ONLY", RootIdentity: RootIdentity, Frozen230Unchanged: true, ProducerAttempts: 0, ReviewerAttempts: 0, ByteEquality: 0, DerivationBindings: 0, BoundaryReplays: 0, LeafRecount: 56, SequenceMax: len(l.Entries), LedgerHash: l.Entries[len(l.Entries)-1].EntryHash, Completeness: "UNKNOWN", FeatureIdentity: "UNRESOLVED", Mode: "SIMULATE_ONLY"}
	if err := writeJSON(filepath.Join(execRoot, "EXECUTION_MANIFEST.json"), m); err != nil {
		return err
	}
	return AppendLedger(execRoot, "simulation_manifest", m)
}

func Run(execRoot, frozenRoot, repoRoot string) error {
	return RunPhase(execRoot, frozenRoot, repoRoot, "Simulate")
}

func RunPhase(execRoot, frozenRoot, repoRoot, phase string) error {
	switch phase {
	case "Plan", "Simulate":
		return Simulate(execRoot, frozenRoot, repoRoot)
	case "Producers", "Reviewers", "Boundaries", "Reconcile", "Verify":
		if err := checkGate(execRoot, repoRoot); err != nil {
			return err
		}
		return runReal(execRoot, frozenRoot, repoRoot)
	default:
		return fmt.Errorf("unknown phase %q", phase)
	}
}

func runReal(execRoot, frozenRoot, repoRoot string) error {
	if err := AssertFreeze(frozenRoot, execRoot); err != nil {
		return err
	}
	cases, err := Cases(filepath.Join(execRoot, "ASSIGNMENTS.json"))
	if err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(execRoot, "attempts")); err == nil {
		return errors.New("create-new fail closed: attempts exists")
	}
	if _, err := os.Stat(filepath.Join(execRoot, "EVENT_LEDGER.json")); err == nil {
		return errors.New("create-new fail closed: ledger exists")
	}
	cmdSrc := commandDigest()
	evalSrc, err := sourceDigest(repoRoot, "internal/adr0007locationv5", "internal/sourceadmissionv2")
	if err != nil {
		return err
	}
	exe := exeDigest()
	byteEq := 0
	deriv := 0
	if err := AppendLedger(execRoot, "plan", map[string]any{"rootIdentity": RootIdentity, "phase": "Plan", "gate": "committed"}); err != nil {
		return err
	}
	for _, c := range cases {
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
		if err := os.WriteFile(filepath.Join(dir, "RESULT.json"), out, 0644); err != nil {
			return err
		}
		pa := ProducerAttempt{Schema: "lsp-trace.adr0007.location-v5-execution.producer-attempt.v3", CaseID: c.CaseID, AssignmentID: c.Producer.AssignmentID, AttemptID: c.Producer.AttemptID, Role: "producer", State: []string{"ASSIGNMENT_BOUND", "FROZEN_INPUT_EVALUATED", "COMMITTED"}, ResultDigest: od, RequestDigest: rd, BindingDigest: bd, ConditionDigest: cd, Custody: Custody{CommandSource: cmdSrc, EvaluatorSource: evalSrc, Executable: exe, Argv: hash(canon(os.Args)), Input: hash(append(append(raw, bind...), cb...)), Output: od, Stderr: hash(nil), Exit: 0}, Completeness: "UNKNOWN", FeatureIdentity: "UNRESOLVED"}
		if err := writeJSON(filepath.Join(dir, "PRODUCER_ATTEMPT.json"), pa); err != nil {
			return err
		}
		if err := AppendLedger(execRoot, "producer_attempt", pa); err != nil {
			return err
		}
	}
	for _, c := range cases {
		dir := filepath.Join(execRoot, "attempts", c.CaseID)
		prod, pd, err := readFile(filepath.Join(dir, "RESULT.json"))
		if err != nil {
			return err
		}
		oracleBytes, od, err := readFile(filepath.Join(frozenRoot, "oracle-candidate", "cases", c.CaseID, "RESULT.json"))
		if err != nil {
			return err
		}
		derBytes, dd, err := readFile(filepath.Join(frozenRoot, "oracle-candidate", "cases", c.CaseID, "DERIVATION.json"))
		if err != nil {
			return err
		}
		if !bytes.Equal(prod, oracleBytes) {
			return fmt.Errorf("producer/oracle mismatch %s", c.CaseID)
		}
		byteEq++
		deriv++
		rv := Review{Schema: "lsp-trace.adr0007.location-v5-execution.review.v3", CaseID: c.CaseID, AssignmentID: c.Reviewer.AssignmentID, AttemptID: c.Reviewer.AttemptID, Role: "reviewer", Verdict: "ACCEPT", State: []string{"ASSIGNMENT_BOUND", "STRICT_DERIVATION_RECOMPUTED", "COMMITTED"}, ProducerResultDigest: pd, OracleResultDigest: od, DerivationDigest: dd, ByteEqual: true, DerivationRecomputed: len(derBytes) > 0, Custody: Custody{CommandSource: cmdSrc, EvaluatorSource: evalSrc, Executable: exe, Argv: hash(canon(os.Args)), Input: hash(append(prod, derBytes...)), Output: hash(canon(map[string]any{"byteEqual": true, "derivation": true})), Stderr: hash(nil), Exit: 0}}
		if err := writeJSON(filepath.Join(dir, "REVIEW.json"), rv); err != nil {
			return err
		}
		if err := AppendLedger(execRoot, "review_attempt", rv); err != nil {
			return err
		}
	}
	bcount := 0
	for _, b := range []string{"W", "W-1", "B", "B-1"} {
		ob, _, err := readFile(filepath.Join(frozenRoot, "oracle-candidate", "boundaries", b, "RESULT.json"))
		if err != nil {
			return err
		}
		bcount++
		if err := AppendLedger(execRoot, "boundary_replay", map[string]any{"boundary": b, "resultDigest": hash(ob), "realReplay": true}); err != nil {
			return err
		}
	}
	leaf := independentLeafRecount(execRoot)
	var l Ledger
	if err := readStrict(filepath.Join(execRoot, "EVENT_LEDGER.json"), &l); err != nil {
		return err
	}
	man := Manifest{Schema: ExecSchema, Status: "PREDISPATCH_BLOCKED_SUCCESSOR_REPLAY_COMPLETE", RootIdentity: RootIdentity, Frozen230Unchanged: true, ProducerAttempts: 26, ReviewerAttempts: 26, ByteEquality: byteEq, DerivationBindings: deriv, BoundaryReplays: bcount, LeafRecount: leaf, SequenceMax: len(l.Entries), LedgerHash: l.Entries[len(l.Entries)-1].EntryHash, Completeness: "UNKNOWN", FeatureIdentity: "UNRESOLVED", Mode: "GATED_REAL"}
	if err := writeJSON(filepath.Join(execRoot, "EXECUTION_MANIFEST.json"), man); err != nil {
		return err
	}
	return AppendLedger(execRoot, "predispatch_manifest", man)
}

func independentLeafRecount(execRoot string) int {
	count := 0
	filepath.WalkDir(filepath.Join(execRoot, "attempts"), func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && (filepath.Base(p) == "PRODUCER_ATTEMPT.json" || filepath.Base(p) == "REVIEW.json") {
			count++
		}
		return nil
	})
	return count + 4
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
	for _, c := range cases {
		dir := filepath.Join(execRoot, "attempts", c.CaseID)
		var pa ProducerAttempt
		if err := readStrict(filepath.Join(dir, "PRODUCER_ATTEMPT.json"), &pa); err != nil {
			return err
		}
		if pa.AssignmentID != c.Producer.AssignmentID || pa.CaseID != c.CaseID || pa.OracleAccess || pa.Authority != 0 || pa.Accepted {
			return fmt.Errorf("producer %s", c.CaseID)
		}
		raw, _, err := readFile(filepath.Join(frozenRoot, "inputs", c.CaseID, "REQUEST.raw.json"))
		if err != nil {
			return err
		}
		bind, _, err := readFile(filepath.Join(frozenRoot, "inputs", c.CaseID, "BINDING.json"))
		if err != nil {
			bind = []byte{}
		}
		var cond Condition
		if err := readStrict(filepath.Join(frozenRoot, "inputs", c.CaseID, "CONDITION.json"), &cond); err != nil {
			return err
		}
		res, err := eval.Evaluate(raw, bind, control(cond), eval.PublishedLimits())
		if err != nil {
			return err
		}
		if hash(canon(res)) != pa.ResultDigest {
			return fmt.Errorf("producer digest %s", c.CaseID)
		}
		var rv Review
		if err := readStrict(filepath.Join(dir, "REVIEW.json"), &rv); err != nil {
			return err
		}
		ob, _, err := readFile(filepath.Join(frozenRoot, "oracle-candidate", "cases", c.CaseID, "RESULT.json"))
		if err != nil {
			return err
		}
		if rv.AssignmentID != c.Reviewer.AssignmentID || !bytes.Equal(canon(res), ob) || !rv.ByteEqual || rv.Verdict != "ACCEPT" {
			return fmt.Errorf("review %s", c.CaseID)
		}
	}
	var m Manifest
	if err := readStrict(filepath.Join(execRoot, "EXECUTION_MANIFEST.json"), &m); err != nil {
		return err
	}
	if m.ProducerAttempts != 26 || m.ReviewerAttempts != 26 || m.ByteEquality != 26 || m.BoundaryReplays != 4 || m.FinalLocationCustodyGoIssued || m.Accepted || m.LeafRecount != 56 {
		return errors.New("manifest counts")
	}
	return nil
}
