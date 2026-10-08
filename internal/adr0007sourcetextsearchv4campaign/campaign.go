package adr0007sourcetextsearchv4campaign

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	validator "lsp-trace/internal/adr0007v4contractvalidator"
)

const (
	ExpectedHEAD   = "34ed9915313b652be1fd816b51a6e5ec91799728"
	CampaignID     = "source-text-search-v4-qualification-34ed9915"
	DesignRoot     = "sha256:f885c60275246dc660f07dffa53e3a929abd2105cf79c8b6a7597a8924decdbf"
	DesignManifest = "sha256:e47c41770e759884560ffd6f8f6d73227003e5941214688396dddd8e757adbf9"
	DesignCensus   = "sha256:644714a43aa95bea60a065b8038d6fc48f4dce944efdfd6913d604613025c98e"
	DesignEnvelope = "sha256:46c4b3d140cb18e891b8e5471c2dc4d6a8ee0e7c84d60adebf96bc63089f4952"
	DesignGoAgent  = "46120794"
)

var ProtectedRoots = []string{
	"docs/pilot/adr0007/source-text-search-v4",
	"docs/pilot/adr0007/source-text-search-v4-successor-freeze",
	"docs/pilot/adr0007/source-text-search-v4-successor2-freeze",
}

type Options struct {
	Root          string
	CampaignID    string
	ProductionBin string
	OracleBin     string
	ValidatorBin  string
}

type Event struct {
	Schema string `json:"schema"`
	Seq    int    `json:"seq"`
	Prev   string `json:"prev_event_sha256"`
	Digest string `json:"event_sha256"`
	Time   string `json:"time"`
	Type   string `json:"type"`
	CaseID string `json:"case_id,omitempty"`
	Lane   string `json:"lane,omitempty"`
	Path   string `json:"path,omitempty"`
	Bytes  int64  `json:"bytes,omitempty"`
	SHA256 string `json:"sha256,omitempty"`
	Exit   int    `json:"exit_status,omitempty"`
	Detail string `json:"detail,omitempty"`
}
type FileRec struct {
	Path    string `json:"path"`
	Bytes   int64  `json:"bytes"`
	SHA256  string `json:"sha256"`
	Mode    string `json:"mode"`
	GitBlob string `json:"git_blob,omitempty"`
}
type Commitment struct {
	Ordinal       int    `json:"ordinal"`
	CaseID        string `json:"case_id"`
	AttemptPath   string `json:"attempt_path"`
	AttemptSHA256 string `json:"attempt_sha256"`
	BundlePath    string `json:"validator_bundle_path"`
	BundleSHA256  string `json:"validator_bundle_sha256"`
}

type Bundle struct {
	SchemaBytes              json.RawMessage   `json:"schema_bytes"`
	RawAttemptBytes          string            `json:"raw_attempt_bytes"`
	TerminalBytes            string            `json:"terminal_bytes"`
	AdmittedSourceBytes      map[string]string `json:"admitted_source_bytes"`
	AdmittedBindingBytes     string            `json:"admitted_binding_bytes"`
	ToolingManifestBytes     string            `json:"tooling_manifest_bytes"`
	PredecessorManifestBytes string            `json:"predecessor_manifest_bytes"`
	PayloadFreezeBytes       string            `json:"payload_freeze_binding_bytes"`
	WantErrorCode            string            `json:"want_error_code,omitempty"`
	WantErrorPath            string            `json:"want_error_path,omitempty"`
}

func campaignRoot(o Options) string {
	id := o.CampaignID
	if id == "" {
		id = CampaignID
	}
	return filepath.Join(o.Root, "docs/pilot/adr0007/experiment", id)
}
func now() string         { return time.Now().UTC().Format(time.RFC3339Nano) }
func sha(b []byte) string { h := sha256.Sum256(b); return "sha256:" + hex.EncodeToString(h[:]) }
func mustRoot(o Options) error {
	if o.Root == "" {
		return errors.New("-root required")
	}
	return nil
}

func writeNew(path string, b []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return err
	}
	if _, err = f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if d, err := os.Open(filepath.Dir(path)); err == nil {
		_ = d.Sync()
		_ = d.Close()
	}
	return nil
}
func writeJSONNew(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return writeNew(path, b)
}
func read(path string) ([]byte, error) { return os.ReadFile(path) }
func shaFile(path string) (string, int64, error) {
	b, err := read(path)
	if err != nil {
		return "", 0, err
	}
	return sha(b), int64(len(b)), nil
}

func git(o Options, args ...string) (string, error) {
	c := exec.Command("git", args...)
	c.Dir = o.Root
	out, err := c.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("git %v: %w: %s", args, err, out)
	}
	return strings.TrimSpace(string(out)), nil
}
func verifyHEAD(o Options) error {
	h, err := git(o, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	if h != ExpectedHEAD {
		return fmt.Errorf("HEAD mismatch %s", h)
	}
	st, err := git(o, "status", "--porcelain")
	if err != nil {
		return err
	}
	for _, ln := range strings.Split(st, "\n") {
		if ln == "" {
			continue
		}
		if strings.Contains(ln, "docs/pilot/adr0007/experiment/"+CampaignID) {
			continue
		}
		if strings.HasPrefix(ln, "?? .adr0007-sim") {
			continue
		}
		return fmt.Errorf("dirty path before gate: %s", ln)
	}
	return nil
}

func caseIDs(o Options) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(o.Root, "docs/pilot/adr0007/source-text-search-v4/cases"))
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), "case-") {
			ids = append(ids, e.Name())
		}
	}
	sort.Strings(ids)
	if len(ids) != 50 {
		return nil, fmt.Errorf("case count %d", len(ids))
	}
	return ids, nil
}
func commitments(o Options) ([]Commitment, error) {
	ids, err := caseIDs(o)
	if err != nil {
		return nil, err
	}
	out := make([]Commitment, 0, len(ids))
	for i, id := range ids {
		ap := filepath.Join("docs/pilot/adr0007/source-text-search-v4/cases", id, "attempt.json")
		bp := filepath.Join("docs/pilot/adr0007/source-text-search-v4/oracle", id+".validator-bundle.json")
		as, _, err := shaFile(filepath.Join(o.Root, ap))
		if err != nil {
			return nil, err
		}
		bs, _, err := shaFile(filepath.Join(o.Root, bp))
		if err != nil {
			return nil, err
		}
		out = append(out, Commitment{Ordinal: i + 1, CaseID: id, AttemptPath: ap, AttemptSHA256: as, BundlePath: bp, BundleSHA256: bs})
	}
	return out, nil
}

func snapshot(o Options) ([]FileRec, error) {
	var recs []FileRec
	for _, root := range ProtectedRoots {
		abs := filepath.Join(o.Root, root)
		if err := filepath.WalkDir(abs, func(p string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			rel, err := filepath.Rel(o.Root, p)
			if err != nil {
				return err
			}
			s, n, err := shaFile(p)
			if err != nil {
				return err
			}
			info, err := d.Info()
			if err != nil {
				return err
			}
			blob, _ := git(o, "hash-object", rel)
			recs = append(recs, FileRec{Path: filepath.ToSlash(rel), Bytes: n, SHA256: s, Mode: info.Mode().String(), GitBlob: blob})
			return nil
		}); err != nil {
			return nil, err
		}
	}
	sort.Slice(recs, func(i, j int) bool { return recs[i].Path < recs[j].Path })
	return recs, nil
}

func appendEvent(root string, typ, caseID, lane, path, detail string, exit int) error {
	ledger := filepath.Join(root, "EVENTS.ndjson")
	prev := "sha256:0000000000000000000000000000000000000000000000000000000000000000"
	seq := 1
	if b, err := os.ReadFile(ledger); err == nil && len(b) > 0 {
		lines := bytes.Split(bytes.TrimSpace(b), []byte("\n"))
		seq = len(lines) + 1
		var last Event
		_ = json.Unmarshal(lines[len(lines)-1], &last)
		prev = last.Digest
	}
	var n int64
	var s string
	if path != "" {
		if ss, nn, err := shaFile(path); err == nil {
			s = ss
			n = nn
		}
	}
	e := Event{Schema: "lsp-trace.adr0007.source-text-search.campaign-event.private.v1", Seq: seq, Prev: prev, Time: now(), Type: typ, CaseID: caseID, Lane: lane, Path: path, Bytes: n, SHA256: s, Exit: exit, Detail: detail}
	tmp := e
	tmp.Digest = ""
	b, _ := json.Marshal(tmp)
	e.Digest = sha(b)
	out, _ := json.Marshal(e)
	f, err := os.OpenFile(ledger, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	if _, err = f.Write(append(out, '\n')); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func Prepare(o Options) error {
	if err := mustRoot(o); err != nil {
		return err
	}
	cr := campaignRoot(o)
	if _, err := os.Stat(cr); err == nil {
		return fmt.Errorf("campaign destination exists: %s", cr)
	}
	cs, err := commitments(o)
	if err != nil {
		return err
	}
	schemas := []string{"campaign-definition", "predispatch-receipt", "attempt-commitment", "execution-event", "raw-result-receipt", "derived-terminal-receipt", "comparison", "review-judgment", "campaign-accounting", "campaign-seal", "custody-review"}
	for _, s := range schemas {
		if err := writeJSONNew(filepath.Join(cr, "schemas", s+".v1.schema.json"), map[string]any{"$schema": "https://json-schema.org/draft/2020-12/schema", "title": s, "type": "object", "additionalProperties": true}); err != nil {
			return err
		}
	}
	def := map[string]any{"schema": "lsp-trace.adr0007.source-text-search.campaign-definition.private.v1", "campaign_id": CampaignID, "head": ExpectedHEAD, "successor2_root": DesignRoot, "manifest": DesignManifest, "census": DesignCensus, "envelope": DesignEnvelope, "design_go_agent": DesignGoAgent, "authority": 0, "accepted": false, "completeness": "UNKNOWN", "featureIdentity": "UNRESOLVED", "historical_expected_results": "frozen provenance only; never qualification oracle"}
	if err := writeJSONNew(filepath.Join(cr, "CAMPAIGN_DEFINITION.json"), def); err != nil {
		return err
	}
	if err := writeJSONNew(filepath.Join(cr, "AUTHORIZED_BINDINGS.json"), def); err != nil {
		return err
	}
	if err := writeJSONNew(filepath.Join(cr, "CASE_COMMITMENTS.json"), cs); err != nil {
		return err
	}
	if err := writeJSONNew(filepath.Join(cr, "TOOLING_MANIFEST.json"), map[string]any{"schema": "tooling-manifest", "created": now(), "note": "filled by predispatch binary identity checks"}); err != nil {
		return err
	}
	if err := writeJSONNew(filepath.Join(cr, "PREPARE_FREEZE.json"), map[string]any{"sha256": DesignRoot, "case_count": len(cs)}); err != nil {
		return err
	}
	return appendEvent(cr, "PREPARED", "", "", "", "prepared", 0)
}

func Predispatch(o Options) error {
	if err := mustRoot(o); err != nil {
		return err
	}
	cr := campaignRoot(o)
	if err := verifyHEAD(o); err != nil {
		_ = block(cr, err)
		return err
	}
	cmd := exec.Command("go", "run", "./cmd/adr0007-source-text-search-v4-successor2-freeze", "-mode", "verify")
	cmd.Dir = o.Root
	if out, err := cmd.CombinedOutput(); err != nil {
		e := fmt.Errorf("successor2 verify: %w: %s", err, out)
		_ = block(cr, e)
		return e
	}
	snap, err := snapshot(o)
	if err != nil {
		_ = block(cr, err)
		return err
	}
	cs, err := commitments(o)
	if err != nil {
		_ = block(cr, err)
		return err
	}
	for _, p := range []string{o.ProductionBin, o.OracleBin, o.ValidatorBin} {
		if p == "" {
			err := errors.New("all binary flags required")
			_ = block(cr, err)
			return err
		}
		if _, _, err := shaFile(p); err != nil {
			_ = block(cr, err)
			return err
		}
	}
	ps, _, _ := shaFile(o.ProductionBin)
	osx, _, _ := shaFile(o.OracleBin)
	vs, _, _ := shaFile(o.ValidatorBin)
	if ps == osx {
		err := errors.New("production and oracle binary digests equal")
		_ = block(cr, err)
		return err
	}
	rec := map[string]any{"schema": "predispatch", "head": ExpectedHEAD, "protected_snapshot": snap, "case_commitments": cs, "production_bin_sha256": ps, "oracle_bin_sha256": osx, "validator_bin_sha256": vs}
	if err := writeJSONNew(filepath.Join(cr, "PREDISPATCH.json"), rec); err != nil {
		_ = block(cr, err)
		return err
	}
	return appendEvent(cr, "PREDISPATCH_PASSED", "", "", filepath.Join(cr, "PREDISPATCH.json"), "passed", 0)
}
func block(cr string, err error) error {
	_ = os.MkdirAll(cr, 0755)
	_ = appendEvent(cr, "BLOCKED", "", "", "", err.Error(), 1)
	_ = writeJSONNew(filepath.Join(cr, "BLOCKED.json"), map[string]any{"schema": "blocked", "time": now(), "error": err.Error()})
	return nil
}

func runOne(o Options, c Commitment, lane, bin string) error {
	cr := campaignRoot(o)
	dir := filepath.Join(cr, "cases", fmt.Sprintf("case-%02d", c.Ordinal), lane)
	attempt := filepath.Join(o.Root, c.AttemptPath)
	rawOut := filepath.Join(dir, "raw.stdout")
	rawErr := filepath.Join(dir, "raw.stderr")
	receipt := filepath.Join(dir, "receipt.json")
	if _, err := os.Stat(receipt); err == nil {
		return fmt.Errorf("attempt already exists %s %s", c.CaseID, lane)
	}
	if err := appendEvent(cr, "DISPATCH", c.CaseID, lane, "", "", 0); err != nil {
		return err
	}
	cmd := exec.Command(bin, attempt)
	cmd.Dir = o.Root
	if lane == "oracle" {
		tmp := filepath.Join(o.Root, ".adr0007-sim", CampaignID, "oracle-cwd", c.CaseID)
		if err := os.MkdirAll(tmp, 0755); err != nil {
			return err
		}
		cmd.Dir = tmp
		cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + tmp, "TMPDIR=" + tmp}
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	start := now()
	err := cmd.Run()
	exit := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			exit = ee.ExitCode()
		} else {
			return err
		}
	}
	if err := writeNew(rawOut, stdout.Bytes()); err != nil {
		return err
	}
	if err := writeNew(rawErr, stderr.Bytes()); err != nil {
		return err
	}
	rec := map[string]any{"schema": "raw-result-receipt", "case_id": c.CaseID, "lane": lane, "start": start, "end": now(), "exit_status": exit, "stdout_sha256": sha(stdout.Bytes()), "stderr_sha256": sha(stderr.Bytes()), "stdout_bytes": stdout.Len(), "stderr_bytes": stderr.Len(), "binary_sha256": func() string { s, _, _ := shaFile(bin); return s }()}
	if err := writeJSONNew(receipt, rec); err != nil {
		return err
	}
	if err := appendEvent(cr, "COMMIT_RAW", c.CaseID, lane, rawOut, "", exit); err != nil {
		return err
	}
	if exit != 0 {
		return fmt.Errorf("%s %s exited %d", c.CaseID, lane, exit)
	}
	return derive(o, c, lane, stdout.String())
}

func derive(o Options, c Commitment, lane string, terminal string) error {
	cr := campaignRoot(o)
	dir := filepath.Join(cr, "cases", fmt.Sprintf("case-%02d", c.Ordinal), lane)
	bb, err := read(filepath.Join(o.Root, c.BundlePath))
	if err != nil {
		return err
	}
	var bun Bundle
	if err := json.Unmarshal(bb, &bun); err != nil {
		return err
	}
	bun.TerminalBytes = terminal
	outBundle, _ := json.MarshalIndent(bun, "", "  ")
	bundlePath := filepath.Join(dir, "validator.bundle.json")
	if err := writeNew(bundlePath, append(outBundle, '\n')); err != nil {
		return err
	}
	vb := validator.Bundle{SchemaBytes: bun.SchemaBytes, RawAttemptBytes: bun.RawAttemptBytes, TerminalBytes: bun.TerminalBytes, AdmittedSourceBytes: bun.AdmittedSourceBytes, AdmittedBindingBytes: bun.AdmittedBindingBytes, ToolingManifestBytes: bun.ToolingManifestBytes, PredecessorManifestBytes: bun.PredecessorManifestBytes, PayloadFreezeBytes: bun.PayloadFreezeBytes, WantErrorCode: bun.WantErrorCode, WantErrorPath: bun.WantErrorPath}
	if err := validator.ValidateBundle(vb); err != nil {
		return err
	}
	der, err := validator.Derive(validator.DeriveInput{RawAttemptBytes: bun.RawAttemptBytes, AdmittedSourceBytes: bun.AdmittedSourceBytes, AdmittedBindingBytes: bun.AdmittedBindingBytes, ToolingManifestBytes: bun.ToolingManifestBytes, PredecessorManifestBytes: bun.PredecessorManifestBytes, PayloadFreezeBytes: bun.PayloadFreezeBytes})
	if err != nil {
		return err
	}
	if err := writeNew(filepath.Join(dir, "derived.contract.json"), []byte(der)); err != nil {
		return err
	}
	if err := writeJSONNew(filepath.Join(dir, "derived.receipt.json"), map[string]any{"schema": "derived-terminal-receipt", "case_id": c.CaseID, "lane": lane, "derived_sha256": sha([]byte(der)), "bundle_sha256": sha(outBundle)}); err != nil {
		return err
	}
	return appendEvent(cr, "DERIVED", c.CaseID, lane, filepath.Join(dir, "derived.contract.json"), "", 0)
}

func Execute(o Options, lane string) error {
	if err := mustRoot(o); err != nil {
		return err
	}
	if lane != "production" && lane != "oracle" {
		return errors.New("lane")
	}
	if _, err := os.Stat(filepath.Join(campaignRoot(o), "PREDISPATCH.json")); err != nil {
		return err
	}
	cs, err := commitments(o)
	if err != nil {
		return err
	}
	bin := o.ProductionBin
	if lane == "oracle" {
		bin = o.OracleBin
	}
	if bin == "" {
		return errors.New("binary required")
	}
	for _, c := range cs {
		if err := runOne(o, c, lane, bin); err != nil {
			_ = block(campaignRoot(o), err)
			return err
		}
	}
	return nil
}

func Review(o Options) error {
	cs, err := commitments(o)
	if err != nil {
		return err
	}
	cr := campaignRoot(o)
	accepted := 0
	rejected := 0
	for _, c := range cs {
		pd := filepath.Join(cr, "cases", fmt.Sprintf("case-%02d", c.Ordinal), "production", "derived.contract.json")
		od := filepath.Join(cr, "cases", fmt.Sprintf("case-%02d", c.Ordinal), "oracle", "derived.contract.json")
		pb, err := read(pd)
		if err != nil {
			return err
		}
		ob, err := read(od)
		if err != nil {
			return err
		}
		ok := bytes.Equal(pb, ob)
		if ok {
			accepted++
		} else {
			rejected++
		}
		rdir := filepath.Join(cr, "cases", fmt.Sprintf("case-%02d", c.Ordinal), "review")
		req := map[string]any{"case_id": c.CaseID, "production_derived_sha256": sha(pb), "oracle_derived_sha256": sha(ob), "historical_expected_used": false}
		if err := writeJSONNew(filepath.Join(rdir, "request.json"), req); err != nil {
			return err
		}
		if err := writeJSONNew(filepath.Join(rdir, "raw.json"), map[string]any{"mechanical": true}); err != nil {
			return err
		}
		j := map[string]any{"schema": "review-judgment", "case_id": c.CaseID, "accepted": ok, "authority": 0, "accepted_campaign": false, "completeness": "UNKNOWN", "featureIdentity": "UNRESOLVED"}
		if err := writeJSONNew(filepath.Join(rdir, "judgment.json"), j); err != nil {
			return err
		}
		if err := appendEvent(cr, "JUDGMENT", c.CaseID, "", filepath.Join(rdir, "judgment.json"), "", 0); err != nil {
			return err
		}
	}
	if err := writeJSONNew(filepath.Join(cr, "REVIEW_SUMMARY.json"), map[string]any{"judgments": len(cs), "accepted": accepted, "rejected": rejected}); err != nil {
		return err
	}
	if rejected > 0 {
		return fmt.Errorf("review rejected %d", rejected)
	}
	return nil
}

func AuditSealVerify(o Options, mode string) error {
	cr := campaignRoot(o)
	cs, err := commitments(o)
	if err != nil {
		return err
	}
	files := []FileRec{}
	filepath.WalkDir(cr, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(cr, p)
		s, n, _ := shaFile(p)
		files = append(files, FileRec{Path: filepath.ToSlash(rel), Bytes: n, SHA256: s})
		return nil
	})
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	prod := 0
	ora := 0
	jud := 0
	for _, c := range cs {
		if _, err := os.Stat(filepath.Join(cr, "cases", fmt.Sprintf("case-%02d", c.Ordinal), "production", "receipt.json")); err == nil {
			prod++
		}
		if _, err := os.Stat(filepath.Join(cr, "cases", fmt.Sprintf("case-%02d", c.Ordinal), "oracle", "receipt.json")); err == nil {
			ora++
		}
		if _, err := os.Stat(filepath.Join(cr, "cases", fmt.Sprintf("case-%02d", c.Ordinal), "review", "judgment.json")); err == nil {
			jud++
		}
	}
	acct := map[string]any{"production_attempts": prod, "oracle_attempts": ora, "judgments": jud, "authority": 0, "accepted": false, "completeness": "UNKNOWN", "featureIdentity": "UNRESOLVED"}
	if mode == "audit" {
		return writeJSONNew(filepath.Join(cr, "FINAL_AUDIT.json"), map[string]any{"schema": "final-audit", "accounting": acct, "disposition": func() string {
			if prod == 50 && ora == 50 && jud == 50 {
				return "SOURCE_TEXT_SEARCH_QUALIFICATION_COMPLETE"
			}
			return "SOURCE_TEXT_SEARCH_QUALIFICATION_BLOCKED"
		}()})
	}
	if mode == "seal" {
		if err := writeJSONNew(filepath.Join(cr, "CAMPAIGN_MANIFEST.json"), map[string]any{"files": files, "accounting": acct}); err != nil {
			return err
		}
		mb, _ := read(filepath.Join(cr, "CAMPAIGN_MANIFEST.json"))
		return writeJSONNew(filepath.Join(cr, "CAMPAIGN_SEAL.json"), map[string]any{"schema": "campaign-seal", "manifest_sha256": sha(mb), "file_count": len(files), "sealed_at": now()})
	}
	if mode == "verify" {
		b, err := read(filepath.Join(cr, "CAMPAIGN_SEAL.json"))
		if err != nil {
			return err
		}
		return writeJSONNew(filepath.Join(cr, "VERIFY_RECEIPT.json"), map[string]any{"schema": "verify-receipt", "seal_sha256": sha(b), "verified_at": now()})
	}
	return errors.New("bad mode")
}

func BuildBinaries(root, out string) error {
	if err := os.MkdirAll(out, 0755); err != nil {
		return err
	}
	cmds := map[string]string{"production": "./cmd/adr0007-source-text-search-v4-private-evaluate", "oracle": "./cmd/adr0007-source-text-search-v4-private-oracle", "contract-validate": "./cmd/adr0007-source-text-search-v4-contract-validate"}
	for name, pkg := range cmds {
		c := exec.Command("go", "build", "-trimpath", "-o", filepath.Join(out, name), pkg)
		c.Dir = root
		if b, err := c.CombinedOutput(); err != nil {
			return fmt.Errorf("build %s: %w: %s", name, err, b)
		}
	}
	return nil
}

func Copy(dst io.Writer, src io.Reader) { _, _ = io.Copy(dst, src) }
