//go:build darwin && lsp_trace_controlled_describe_fixture

package main

// External-caller exercise: stdlib only. Public JSON/argv/stdin are the sole
// coordination interface. Provider injection lives in the separate server helper.
import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

type publicOwnerReply struct {
	Used, Limit, Packets                                  uint64
	Schema, Store, RunID, Head, Phase, Status, Capability string
	Admissions                                            int
	Operations                                            map[string][]string
	Enumeration                                           struct {
		Counts struct {
			Selected  int `json:"supported_unacquired"`
			Excluded  int `json:"excluded"`
			NotSource int `json:"unknown_extension_not_source"`
		}
		UnknownRemainder bool
	}
}
type publicTaskReply struct {
	Schema, RunID, TaskID, Role, Actor, Phase, Status, InputSHA256          string
	DeadlineUnixNano, RunDeadlineUnixNano, TaskIssuedNS, SemanticReservedNS int64
	Artifacts                                                               []struct {
		ID    string
		Bytes int
	}
	Contract struct {
		Operations               map[string][]string
		Example, Encoding, Rules string
	}
}
type publicCLI struct {
	t                        *testing.T
	source, store, exe, home string
	steps                    int
}

func newPublicCLI(t *testing.T) *publicCLI {
	t.Helper()
	base, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	c := &publicCLI{t: t, source: filepath.Join(base, "source"), store: filepath.Join(base, "store"), home: filepath.Join(base, "home")}
	c.exe, e = os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	for _, p := range []string{c.source, c.store, c.home} {
		if e = os.Mkdir(p, 0700); e != nil {
			t.Fatal(e)
		}
	}
	return c
}
func (c *publicCLI) call(argv []string, answer []byte, wantSuccess bool) []byte {
	c.t.Helper()
	if len(argv) < 3 {
		c.t.Fatal("MISSING_RETURNED_OPERATION", argv)
	}
	args := append([]string{"-test.run=^TestCensusOwnedProcess$", "-test.timeout=60s", "--", "census-owned-process-v1", c.source}, argv...)
	cmd := exec.Command(c.exe, args...)
	cmd.Dir = c.source
	if argv[2] == "durable" {
		cmd.Dir = "/"
	}
	cmd.Env = []string{"HOME=" + c.home, "PATH=" + os.Getenv("PATH"), "TMPDIR=" + os.Getenv("TMPDIR"), "GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local", "GOTELEMETRY=off"}
	cmd.Stdin = bytes.NewReader(answer)
	var out, diag bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &diag
	e := cmd.Run()
	c.steps++
	answerDigest := sha256.Sum256(answer)
	trace, _ := json.Marshal(struct {
		Step, PID                int
		Argv                     []string
		WorkingDirectory         string
		StdinBase64, StdinSHA256 string
		Stdout, Stderr           string
		Success                  bool
	}{c.steps, cmd.Process.Pid, argv, cmd.Dir, base64.StdEncoding.EncodeToString(answer), "sha256:" + hex.EncodeToString(answerDigest[:]), out.String(), diag.String(), e == nil})
	c.t.Log("PUBLIC_CALL " + string(trace))
	if (e == nil) != wantSuccess {
		c.t.Fatalf("command outcome err=%v out=%s diagnostic=%s", e, out.String(), diag.String())
	}
	return out.Bytes()
}
func (c *publicCLI) start(extra ...string) publicOwnerReply {
	argv := append([]string{"lsp-trace", "census", ".", "--publication-root", c.store}, extra...)
	var v publicOwnerReply
	if json.Unmarshal(c.call(argv, nil, true), &v) != nil {
		c.t.Fatal("start JSON")
	}
	return v
}
func (c *publicCLI) acquire(v publicOwnerReply) publicOwnerReply {
	var next publicOwnerReply
	if json.Unmarshal(c.call(v.Operations["acquire"], nil, true), &next) != nil {
		c.t.Fatal("acquire JSON")
	}
	return next
}
func (c *publicCLI) task(argv []string, answer []byte) publicTaskReply {
	var task publicTaskReply
	if json.Unmarshal(c.call(argv, answer, true), &task) != nil {
		c.t.Fatal("task JSON")
	}
	if task.Schema != "lsp-trace.durable-inventory-task.v3" || task.RunDeadlineUnixNano != 0 || task.DeadlineUnixNano-task.TaskIssuedNS != 600000000000 {
		c.t.Fatal("task lease contract", task)
	}
	return task
}
func (c *publicCLI) input(task publicTaskReply) []byte {
	var all []byte
	for len(all) < task.Artifacts[0].Bytes {
		argv := append([]string(nil), task.Contract.Operations["read"]...)
		argv[7] = strconv.Itoa(len(all))
		var page struct {
			Data          string
			Offset, Total int
		}
		if json.Unmarshal(c.call(argv, nil, true), &page) != nil {
			c.t.Fatal("read JSON")
		}
		raw, e := base64.StdEncoding.DecodeString(page.Data)
		if e != nil || len(raw) == 0 || page.Offset != len(all) || page.Total != task.Artifacts[0].Bytes {
			c.t.Fatal("read continuity")
		}
		all = append(all, raw...)
	}
	digest := sha256.Sum256(all)
	if "sha256:"+hex.EncodeToString(digest[:]) != task.InputSHA256 {
		c.t.Fatal("PUBLIC_INPUT_DIGEST")
	}
	return all
}
func TestCensusOwnedPublicCaller(t *testing.T) {
	c := newPublicCLI(t)
	v := c.start()
	if v.Capability != "BOUNDED_ACQUISITION_READY" || v.Enumeration.Counts.Selected != 17 || v.Enumeration.Counts.NotSource != 1 {
		t.Fatal("initial scope", v)
	}
	first := c.acquire(v)
	if first.Status != "READY" || first.Admissions != 1 {
		t.Fatal("clean pause", first)
	}
	// The next command is a fresh process using only returned argv. No owner or
	// evidence object crosses this boundary. Stale exact-head acquisition refuses.
	c.call(v.Operations["acquire"], nil, false)
	// The first command process and its <=15s native lease are both gone.
	// Waiting is not a new reservation, expiry reset, or permission to replay.
	time.Sleep(15*time.Second + 20*time.Millisecond)
	var paused publicOwnerReply
	if json.Unmarshal(c.call(first.Operations["status"], nil, true), &paused) != nil || paused.Head != first.Head || paused.Used != first.Used || paused.Packets != first.Packets || paused.Admissions != 1 || paused.Status != "READY" {
		t.Fatal("ADMITTED_PAUSE_CHANGED_LEDGER", paused)
	}
	t.Log("ADMITTED_PAUSE_AFTER_NATIVE_WINDOW: unchanged exact head/bytes/packets; remaining-only fresh process follows")
	second := c.acquire(paused)
	if second.Admissions != 2 || second.Capability != "READY_TO_SEAL" {
		t.Fatal("remaining batch", second)
	}
	task := c.task(second.Operations["seal"], nil)
	roles := []string{}
	var previousSubmit []string
	for task.Phase != "Published" {
		input := c.input(task)
		roles = append(roles, task.Role)
		wrong := append([]string(nil), task.Contract.Operations["submit"]...)
		wrong[6] = "wrong-role"
		c.call(wrong, []byte("{}"), false)
		if previousSubmit != nil {
			c.call(previousSubmit, []byte("{}"), false)
		}
		answer := []byte(task.Contract.Example) // mechanical schema fixture, never an authored judgment
		if task.Phase == "Describe" {
			var body map[string]json.RawMessage
			if json.Unmarshal(input, &body) != nil || body["OriginalReferences"] == nil {
				t.Fatal("ORIGINAL_R_MISSING")
			}
		} else {
			var catalog struct {
				Revision string
				Evidence []map[string]json.RawMessage `json:"evidence"`
			}
			if json.Unmarshal(input, &catalog) != nil || len(catalog.Evidence) != 2 {
				t.Fatal("catalog input")
			}
			for _, e := range catalog.Evidence {
				if e["OriginalReferences"] == nil || e["InventoryAdmission"] == nil {
					t.Fatal("ADMISSION_PROVENANCE_MISSING")
				}
			}
			answer = bytes.ReplaceAll(answer, []byte("CURRENT_INPUT_REVISION"), []byte(catalog.Revision))
		}
		previousSubmit = task.Contract.Operations["submit"]
		task = c.task(previousSubmit, answer)
	}
	if strings.Join(roles, ",") != "interpreter,interpreter,interpreter,reviewer" || task.SemanticReservedNS != 2400000000000 {
		t.Fatal("four separate stages", roles)
	}
	raw := c.call(task.Contract.Operations["result"], nil, true)
	var report struct {
		Published, Accepted               bool
		Authority, SuccessfulDescriptions int
		UntouchedGaps                     []string
	}
	if json.Unmarshal(raw, &report) != nil || !report.Published || report.Accepted || report.Authority != 0 || report.SuccessfulDescriptions != 2 {
		t.Fatal("qualified report", string(raw))
	}
	targets := 0
	for _, g := range report.UntouchedGaps {
		if strings.HasPrefix(g, "ORIGINAL_REFERENCE_TARGET;") {
			targets++
		}
	}
	if targets != 2 {
		t.Fatal("ORIGINAL_R_PROVENANCE_LOST")
	}
	c.call(second.Operations["seal"], nil, false) // no reseal or new semantic lease
	type fileRow struct {
		Path   string
		Bytes  int64
		SHA256 string
	}
	var files []fileRow
	if err := filepath.WalkDir(second.Store, func(path string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.Type().IsRegular() {
			raw, e := os.ReadFile(path)
			if e != nil {
				return e
			}
			rel, e := filepath.Rel(second.Store, path)
			if e != nil {
				return e
			}
			hash := sha256.Sum256(raw)
			files = append(files, fileRow{rel, int64(len(raw)), hex.EncodeToString(hash[:])})
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	census, _ := json.Marshal(files)
	t.Log("PUBLIC_STORE_FILES " + string(census))
	t.Logf("PUBLIC_PROTOCOL_COMPLETE steps=%d undocumented_steps=0 manual_evidence_transforms=0 mechanical_judgments=4 independent_judgments=0", c.steps)
}
func TestCensusOwnedExpiredTask(t *testing.T) {
	c := newPublicCLI(t)
	v := c.acquire(c.acquire(c.start("--semantic-window", "1s")))
	var task publicTaskReply
	if json.Unmarshal(c.call(v.Operations["seal"], nil, true), &task) != nil || task.DeadlineUnixNano-task.TaskIssuedNS != int64(time.Second) || task.RunDeadlineUnixNano != 0 {
		t.Fatal("frozen custom policy", task)
	}
	time.Sleep(max(time.Until(time.Unix(0, task.DeadlineUnixNano))+20*time.Millisecond, 0))
	c.call(task.Contract.Operations["submit"], []byte(task.Contract.Example), false)
	var expired publicTaskReply
	if json.Unmarshal(c.call(task.Contract.Operations["current_task"], nil, true), &expired) != nil || expired.Status != "EXPIRED" || expired.TaskID != task.TaskID || expired.DeadlineUnixNano != task.DeadlineUnixNano || expired.Contract.Operations["submit"] != nil {
		t.Fatal("EXPIRED_TASK_RENEWED", expired)
	}
}

func TestCensusOwnedDefaultStore(t *testing.T) {
	c := newPublicCLI(t)
	var v publicOwnerReply
	if json.Unmarshal(c.call([]string{"lsp-trace", "census", "."}, nil, true), &v) != nil || !strings.HasPrefix(v.Store, c.home+string(os.PathSeparator)) {
		t.Fatal("default store escaped controlled home", v)
	}
	c.home = t.TempDir() // a fresh caller has a different HOME and working directory
	c.call(v.Operations["status"], nil, true)
}

func TestCensusOwnedTooManyMembers(t *testing.T) {
	c := newPublicCLI(t)
	extra := c.source + "-extra"
	if e := os.Rename(c.source, extra); e != nil {
		t.Fatal(e)
	}
	c.source = extra
	v := c.start()
	if v.Enumeration.Counts.Selected != 18 || v.Capability != "UNSUPPORTED_SCOPE_OR_PROFILE" || v.Operations["acquire"] != nil {
		t.Fatal("POPULATION_TRUNCATED_TO_17", v)
	}
}

func TestCensusOwnedUnsupportedPopulation(t *testing.T) {
	c := newPublicCLI(t)
	v := c.start("--exclude", "query-17.go")
	if v.Enumeration.Counts.Selected != 16 || v.Enumeration.Counts.Excluded != 1 || v.Capability != "UNSUPPORTED_SCOPE_OR_PROFILE" || v.Operations["acquire"] != nil {
		t.Fatal("SILENT_SCOPE_REDUCTION", v)
	}
	c.call(v.Operations["status"], nil, true)
}
func TestCensusOwnedWrongActor(t *testing.T) {
	c := newPublicCLI(t)
	v := c.acquire(c.acquire(c.start()))
	task := c.task(v.Operations["seal"], nil)
	for task.Phase == "Describe" {
		c.input(task)
		task = c.task(task.Contract.Operations["submit"], []byte(task.Contract.Example))
	}
	input := c.input(task)
	var catalog struct{ Revision string }
	if json.Unmarshal(input, &catalog) != nil {
		t.Fatal("catalog")
	}
	var answer map[string]any
	if json.Unmarshal([]byte(task.Contract.Example), &answer) != nil {
		t.Fatal("decision grammar")
	}
	answer["predecessor"] = catalog.Revision
	answer["actor"] = "wrong-actor"
	raw, _ := json.Marshal(answer)
	c.call(task.Contract.Operations["submit"], raw, false)
	spent := c.task(task.Contract.Operations["current_task"], nil)
	if spent.Status != "UNCERTAIN_SPENT" || spent.TaskID != task.TaskID {
		t.Fatal("WRONG_ACTOR_NOT_SPENT", spent)
	}
	c.call(task.Contract.Operations["submit"], raw, false)
}

func TestCensusOwnedCorruptOrMissing(t *testing.T) {
	for _, missing := range []bool{false, true} {
		name := "corrupt"
		if missing {
			name = "missing"
		}
		t.Run(name, func(t *testing.T) {
			c := newPublicCLI(t)
			v := c.acquire(c.start())
			files, e := filepath.Glob(filepath.Join(v.Store, "inventory-evidence-*.json"))
			if e != nil || len(files) != 1 {
				t.Fatal(files, e)
			}
			// Fault injection, not a public caller translation or supported mutation.
			if missing {
				e = os.Remove(files[0])
			} else {
				e = os.WriteFile(files[0], []byte("{}"), 0600)
			}
			if e != nil {
				t.Fatal(e)
			}
			c.call(v.Operations["acquire"], nil, false)
		})
	}
}
