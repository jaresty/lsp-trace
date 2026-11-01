//go:build darwin || linux

package privatehostproductv3

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	pilot "lsp-trace/internal/censuslineagepilot"
	"os"
	"reflect"
	"strconv"
	"strings"
	"unicode/utf8"
)

const CohortProfile = "PRIVATE_EXISTING_HOST_REVIEWED_COHORT_V1"
const CohortProtocol = "PRIVATE_V3_REVIEWED_COHORT_1"

// This sibling never changes Config, Task, Response or legacy canonical bytes.
// Grant is supplied by the caller. It is a binding, not authentication.
type CohortConfig struct {
	RolePromptSHA256                                                           [2]string
	Requested                                                                  int
	Navigation                                                                 bool
	Profile, Protocol, SourceConfigSHA256, ScopeSHA256, Store, Grant           string
	MaxCalls, MaxRequestBytes, MaxResponseBytes, MaxTotalBytes, MaxOutputBytes int
}
type CohortItem struct {
	ItemID, InputID, Target, Attempt, Grant, ConfigSHA256, ScopeSHA256 string
	PreparedSHA256, CheckpointSHA256                                   string
	OriginalRawReferences                                              [4]string
	Body, OwnBodySHA256                                                string
	Revision                                                           int
	Predecessor                                                        string
	Proposal                                                           *CohortDecision
	Review                                                             *CohortDecision
}
type CohortDecision struct {
	ItemID, InputID, Target, Attempt, Predecessor, OwnBodySHA256   string
	Revision                                                       int
	Disposition, Label, Explanation, Alternative, Falsifier, Quote string
	Start, End                                                     int
}
type CohortTask struct {
	Protocol, ConfigSHA256, Grant, Role, Stage, RequestDigest string
	Sequence                                                  int
	Items                                                     []CohortItem
}
type CohortResponse struct {
	Protocol, RequestDigest string
	Items                   []CohortDecision
}

// Bytes are EXACT direct UTF-8 request/reply file contents, not logical object
// sizes. No base64 framing exists on this transport. Host tool envelopes,
// receipt/manifest export, report IO and inference billing are excluded and
// must not be described as fully accounted application/host cost.
type CohortBackend interface {
	ExchangeCohort(context.Context, []byte) ([]byte, error)
}
type CohortOutcome struct {
	Item    CohortItem
	Status  string
	History []CohortDecision
}
type CohortReport struct {
	AccountingValidation                                            string
	Inflight                                                        bool
	NavigationHistory                                               []CohortNavigationResponse
	NavigationStatus                                                string
	Schema, Profile, Status, ConfigSHA256                           string
	Authority                                                       uint8
	Accepted                                                        bool
	FeatureIdentity, SourceCompleteness, AbsenceInference           string
	Requested, Reviewed, Gaps, Pending                              int
	Outcomes                                                        []CohortOutcome
	Calls, RequestBytes, ResponseBytes                              int
	MachinePaths, HumanPaths                                        [2]string
	OriginalCleanup                                                 Lifecycle
	LedgerGeneration                                                uint64
	LedgerTip                                                       string
	PolicyReservation                                               pilot.PageCostPrivateV3
	PriorPublicationValidation                                      string
	UnadmittedPublications                                          bool
	SourceValidation, LedgerValidation, HostReturns, CostExclusions string
}
type cohortReceipt struct {
	Sequence       int
	ResponseSHA256 string
	Bytes          int
	BackendOK      bool
}

type cohortProgress struct {
	AccountingValidation               string
	NavigationHistory                  []CohortNavigationResponse
	NavigationStatus                   string
	Config                             CohortConfig
	Outcomes                           []CohortOutcome
	Calls, RequestBytes, ResponseBytes int
	Inflight                           bool
}

func CohortConfigDigest(c CohortConfig) string { return hash(mustJSON(c)) }
func ValidateCohortConfig(c CohortConfig) error {
	if c.RolePromptSHA256 != [2]string{hash([]byte(CohortRolePrompt("interpreter"))), hash([]byte(CohortRolePrompt("reviewer")))} {
		return ErrInvalid
	}
	if !admitCohortGraph(c) || len(c.Grant) > 4096 || len(c.Store) > 4096 {
		return ErrInvalid
	}
	if c.Requested < 1 || c.Requested > 64 || c.Profile != CohortProfile || c.Protocol != CohortProtocol || c.Grant == "" || !digestValue(c.SourceConfigSHA256) || !digestValue(c.ScopeSHA256) || c.Store == "" || c.MaxCalls < 1 || c.MaxCalls > 16 || c.MaxRequestBytes < 1 || c.MaxRequestBytes > 1<<20 || c.MaxResponseBytes < 1 || c.MaxResponseBytes > 64<<10 || c.MaxTotalBytes < 1 || c.MaxTotalBytes > 8<<20 || c.MaxOutputBytes < 1 || c.MaxOutputBytes > 4<<20 {
		return ErrInvalid
	}
	return nil
}
func validCohortItem(c CohortConfig, i CohortItem) bool {
	for _, ref := range i.OriginalRawReferences {
		if !digestValue(ref) {
			return false
		}
	}
	if len(i.Body) > 2<<20 || !admitCohortGraph(i) {
		return false
	}
	return i.ItemID != "" && i.InputID != "" && i.Target != "" && i.Attempt != "" && i.Grant == c.Grant && i.ConfigSHA256 == c.SourceConfigSHA256 && i.ScopeSHA256 == c.ScopeSHA256 && digestValue(i.PreparedSHA256) && digestValue(i.CheckpointSHA256) && i.Body != "" && utf8.ValidString(i.Body) && hash([]byte(i.Body)) == i.OwnBodySHA256 && i.Revision == 1 && i.Predecessor == "" && i.Proposal == nil && i.Review == nil
}

// Walk the CLOSED reflected DTO schema before encoding/json can collapse
// decoded duplicate keys or case aliases. All response fields are required;
// order and whitespace are legitimate and preserved in physical retention.
func cohortSchema(d *json.Decoder, t reflect.Type, depth int) error {
	if depth > 32 {
		return ErrInvalid
	}
	switch t.Kind() {
	case reflect.Struct:
		v, e := d.Token()
		if e != nil || v != json.Delim('{') {
			return ErrInvalid
		}
		fields := map[string]reflect.Type{}
		for n := 0; n < t.NumField(); n++ {
			f := t.Field(n)
			fields[f.Name] = f.Type
		}
		seen := map[string]bool{}
		for d.More() {
			v, e = d.Token()
			k, ok := v.(string)
			ft, found := fields[k]
			if e != nil || !ok || !found || seen[k] {
				return ErrInvalid
			}
			seen[k] = true
			if cohortSchema(d, ft, depth+1) != nil {
				return ErrInvalid
			}
		}
		v, e = d.Token()
		if e != nil || v != json.Delim('}') || len(seen) != len(fields) {
			return ErrInvalid
		}
	case reflect.Array:
		v, e := d.Token()
		if e != nil || v != json.Delim('[') {
			return ErrInvalid
		}
		n := 0
		for d.More() {
			if n >= t.Len() || cohortSchema(d, t.Elem(), depth+1) != nil {
				return ErrInvalid
			}
			n++
		}
		v, e = d.Token()
		if e != nil || v != json.Delim(']') || n != t.Len() {
			return ErrInvalid
		}
	case reflect.Uint64:
		v, e := d.Token()
		s, ok := v.(json.Number)
		if e != nil || !ok {
			return ErrInvalid
		}
		if _, e = strconv.ParseUint(string(s), 10, 64); e != nil {
			return ErrInvalid
		}
	case reflect.Slice:
		v, e := d.Token()
		if e != nil || v != json.Delim('[') {
			return ErrInvalid
		}
		n := 0
		for d.More() {
			n++
			if n > 10 || cohortSchema(d, t.Elem(), depth+1) != nil {
				return ErrInvalid
			}
		}
		v, e = d.Token()
		if e != nil || v != json.Delim(']') {
			return ErrInvalid
		}
	case reflect.Bool:
		v, e := d.Token()
		if _, ok := v.(bool); e != nil || !ok {
			return ErrInvalid
		}
	case reflect.String:
		v, e := d.Token()
		if _, ok := v.(string); e != nil || !ok {
			return ErrInvalid
		}
	case reflect.Int:
		v, e := d.Token()
		s, ok := v.(json.Number)
		if e != nil || !ok {
			return ErrInvalid
		}
		if _, e = s.Int64(); e != nil {
			return ErrInvalid
		}
	default:
		return ErrInvalid
	}
	return nil
}
func DecodeCohortResponse(raw []byte) (CohortResponse, error) {
	var r CohortResponse
	if len(raw) > 64<<10 || !utf8.Valid(raw) {
		return r, ErrInvalid
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	if cohortSchema(d, reflect.TypeOf(r), 0) != nil || d.Decode(new(any)) != io.EOF || json.Unmarshal(raw, &r) != nil || r.Protocol != CohortProtocol {
		return CohortResponse{}, ErrInvalid
	}
	return r, nil
}
func validateCohortResponse(t CohortTask, r CohortResponse) error {
	if r.Protocol != CohortProtocol || r.RequestDigest != t.RequestDigest || len(r.Items) != len(t.Items) {
		return ErrItemEvidence
	}
	seen := map[string]bool{}
	for n, d := range r.Items {
		i := t.Items[n]
		if seen[d.ItemID] || d.ItemID != i.ItemID || d.InputID != i.InputID || d.Target != i.Target || d.Attempt != i.Attempt || d.Predecessor != i.Predecessor || d.Revision != i.Revision || d.OwnBodySHA256 != i.OwnBodySHA256 {
			return ErrItemEvidence
		}
		seen[d.ItemID] = true
		if d.Disposition != "SUPPORTED" && d.Disposition != "UNSUPPORTED" && d.Disposition != "UNRESOLVED" && d.Disposition != "WITHDRAWN" {
			return ErrItemEvidence
		}
		// Ordered bounds precede subtraction, conversion, indexing or slicing.
		if d.Start < 0 || d.End < d.Start || d.End > len(i.Body) || !utf8.ValidString(i.Body[:d.Start]) || !utf8.ValidString(i.Body[:d.End]) || d.Quote == "" || i.Body[d.Start:d.End] != d.Quote {
			return ErrItemEvidence
		}
		if len(d.Label) > 512 || len(d.Explanation) > 2048 || len(d.Alternative) > 512 || len(d.Falsifier) > 512 || len(d.Quote) > 4096 || strings.TrimSpace(d.Label) == "" || strings.TrimSpace(d.Explanation) == "" || strings.TrimSpace(d.Alternative) == "" || strings.TrimSpace(d.Falsifier) == "" {
			return ErrItemEvidence
		}
	}
	return nil
}
func bindCohortTask(c CohortConfig, items []CohortItem, role, stage string, seq int) CohortTask {
	t := CohortTask{Protocol: CohortProtocol, ConfigSHA256: CohortConfigDigest(c), Grant: c.Grant, Role: role, Stage: stage, Sequence: seq, Items: items}
	t.RequestDigest = hash(mustJSON(t))
	return t
}
func cohortHuman(r CohortReport) []byte {
	// Escape model/source strings as display data, not active Markdown/HTML.
	clean := func(s string) string {
		return strings.NewReplacer("[", "&#91;", "]", "&#93;", "(", "&#40;", ")", "&#41;", "`", "&#96;", "*", "&#42;", "_", "&#95;", "\n", " ", "\r", " ").Replace(html.EscapeString(s))
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# Provisional reviewed cohort — %s\n\n%d requested = %d reviewed + %d gaps + %d pending. No result establishes absence of features.\n\n", r.Status, r.Requested, r.Reviewed, r.Gaps, r.Pending)
	priorQualification := r.PriorPublicationValidation
	if priorQualification == "" {
		priorQualification = "NOT_APPLICABLE"
	}
	fmt.Fprintf(&b, "Current source validation: %s; current ledger validation: %s. Prior publication validation: %s. Unadmitted publications: %t. Retained rows are bounded observations, not authenticated histories or current-source AST ownership.\n\n", clean(r.SourceValidation), clean(r.LedgerValidation), clean(priorQualification), r.UnadmittedPublications)
	if r.Inflight {
		b.WriteString("The last exchange is spent and in-flight/ambiguous; its reply supplies no adopted outcome.\n\n")
	}
	for _, o := range r.Outcomes {
		label := "UNRESOLVED"
		if len(o.History) > 0 {
			label = o.History[len(o.History)-1].Label
		}
		fmt.Fprintf(&b, "- %q: %s; revision %d; label %q; feature identity unassigned\n", clean(o.Item.ItemID), clean(o.Status), o.Item.Revision, clean(label))
	}
	fmt.Fprintf(&b, "\nNavigation: %s. Hypotheses are reversible and do not accept feature identity.\n", r.NavigationStatus)
	if len(r.NavigationHistory) == 2 {
		for _, g := range r.NavigationHistory[1].Groups {
			fmt.Fprintf(&b, "- Hypothesis %q: %q; supported=%t; alternatives=%q; falsifier=%q; withdrawal=%q; revisit=%q; exact members=%q\n", clean(g.ID), clean(g.Label), g.Supported, clean(g.Alternative), clean(g.Falsifier), clean(g.Withdrawal), clean(g.Revisit), clean(string(mustJSON(g.Members))))
		}
	}
	fmt.Fprintf(&b, "\n%d physical direct-UTF8 exchanges; %d request bytes; %d response bytes.\nAccounting validation: %s; UNKNOWN counts retain known spend, not exact whole history.\nAuthority 0; accepted false; feature UNRESOLVED; source UNKNOWN; absence PROHIBITED.\nHost returns: %s.\nExcluded: %s\n", r.Calls, r.RequestBytes, r.ResponseBytes, clean(r.AccountingValidation), r.HostReturns, r.CostExclusions)
	return []byte(b.String())
}
func cohortReport(c CohortConfig, p cohortProgress, status string) CohortReport {
	r := CohortReport{Schema: "private-reviewed-cohort-report.v1", Profile: CohortProfile, Status: status, ConfigSHA256: CohortConfigDigest(c), FeatureIdentity: "UNRESOLVED", SourceCompleteness: "UNKNOWN", AbsenceInference: "PROHIBITED", Requested: len(p.Outcomes), Outcomes: p.Outcomes, Calls: p.Calls, RequestBytes: p.RequestBytes, ResponseBytes: p.ResponseBytes, SourceValidation: "UNKNOWN", LedgerValidation: "UNKNOWN", HostReturns: "UNKNOWN", CostExclusions: "original acquisition/session; bootstrap retries; host tool outer envelopes and export metadata; inference billing; full application-store/report IO"}
	r.AccountingValidation = p.AccountingValidation
	if r.AccountingValidation == "" {
		r.AccountingValidation = "EXACT"
	}
	r.Inflight = p.Inflight
	r.NavigationHistory = p.NavigationHistory
	r.NavigationStatus = p.NavigationStatus
	for _, o := range p.Outcomes {
		switch o.Status {
		case "REVIEWED":
			r.Reviewed++
		case "UNSUPPORTED", "WITHDRAWN", "UNRESOLVED":
			r.Gaps++
		default:
			r.Pending++
		}
	}
	return r
}
func cohortSave(s *store, p cohortProgress) error {
	if !admitCohortGraph(p) {
		return ErrPaused
	}
	b := mustJSON(p)
	if len(b) > stateLimit {
		return ErrPaused
	}
	s.stateSeq++
	return s.write(fmt.Sprintf("pages/state-%08d.json", s.stateSeq), b)
}
func cohortDeliver(s *store, r *CohortReport) error {
	numbers, e := s.resultNumbers()
	if e != nil {
		return e
	}
	first := 1
	if len(numbers) > 0 {
		first = numbers[0] + 2
	}
	if first > 99999997 {
		return ErrInvalid
	}
	r.MachinePaths, r.HumanPaths = reportPaths(first)
	if !admitCohortGraph(*r) || len(mustJSON(r)) > stateLimit || len(cohortHuman(*r)) > stateLimit {
		r.MachinePaths = [2]string{}
		r.HumanPaths = [2]string{}
		return ErrPaused
	}
	fail := func(e error) error { r.MachinePaths = [2]string{}; r.HumanPaths = [2]string{}; return e }
	for _, v := range []struct {
		names [2]string
		raw   []byte
	}{{r.MachinePaths, mustJSON(r)}, {r.HumanPaths, cohortHuman(*r)}} {
		for _, name := range v.names {
			if e = s.write(name, v.raw); e != nil {
				return fail(e)
			}
		}
	}
	if e = verifyCohortQuartet(s, *r); e != nil {
		return fail(e)
	}
	return nil
}
func verifyCohortQuartet(s *store, r CohortReport) error {
	var infos []os.FileInfo
	for _, v := range []struct {
		names [2]string
		raw   []byte
	}{{r.MachinePaths, mustJSON(r)}, {r.HumanPaths, cohortHuman(r)}} {
		for _, name := range v.names {
			b, e := s.read(name, stateLimit)
			if e != nil || !bytes.Equal(b, v.raw) {
				return ErrInvalid
			}
			st, e := s.root.Stat(name)
			if e != nil {
				return e
			}
			for _, prior := range infos {
				if os.SameFile(prior, st) {
					return ErrInvalid
				}
			}
			infos = append(infos, st)
		}
	}
	return nil
}

// RunCohort accepts only caller-built own-item DATA. The retained owner adapter
// below is the production source of this data; no JSON mints a V2 capture owner.
// Existing generation history is never re-disclosed automatically, even when a
// reply was lost, malformed, oversized or returned with an error.
func RunCohort(ctx context.Context, c CohortConfig, items []CohortItem, b CohortBackend) (CohortReport, error) {
	var zero CohortReport
	if ctx == nil || b == nil || ValidateCohortConfig(c) != nil || len(items) != c.Requested {
		return zero, ErrInvalid
	}
	p := cohortProgress{Config: c, Outcomes: []CohortOutcome{}}
	seen := map[string]bool{}
	for _, i := range items {
		if !validCohortItem(c, i) || seen[i.ItemID] {
			return zero, ErrInvalid
		}
		seen[i.ItemID] = true
		p.Outcomes = append(p.Outcomes, CohortOutcome{Item: i, Status: "PENDING", History: []CohortDecision{}})
	}
	if !admitCohortGraph(p) || len(mustJSON(p)) > stateLimit {
		r, deliveryErr := DeliverCohortFailure(c, c.Requested, "PROGRESS_BOUND_REFUSAL")
		return r, errors.Join(ErrPaused, deliveryErr)
	}
	s, e := openCohortStore(c)
	if e != nil {
		return zero, e
	}
	defer s.close()
	if _, e = s.read("plan/config-00000001.json", stateLimit); !os.IsNotExist(e) {
		// Return only a separately verified prior observation, never retry disclosure.
		r, re := latestCohort(s, c)
		if re != nil {
			return zero, errors.Join(ErrInvalid, re)
		}
		r.Status = "SPENT_GENERATION_NO_AUTORETRY"
		r.SourceValidation = "UNKNOWN"
		r.LedgerValidation = "UNKNOWN"
		return r, ErrInvalid
	}
	if e = s.write("plan/config-00000001.json", mustJSON(c)); e != nil {
		return zero, e
	}
	boundMailbox, e := pinCohortMailbox(s, c, b)
	if e != nil {
		return zero, e
	}
	if boundMailbox != nil {
		defer boundMailbox.root.close()
		ctx = context.WithValue(ctx, cohortPinnedMailboxKey{}, boundMailbox)
	}
	if e = cohortSave(s, p); e != nil {
		return zero, e
	}
	finish := func(cause error) (CohortReport, error) {
		status := "PROVISIONAL"
		if cause != nil {
			status = "FAILED"
			if errors.Is(cause, ErrPaused) {
				status = "PAUSED_BUDGET"
			}
			if errors.Is(cause, context.Canceled) || errors.Is(cause, context.DeadlineExceeded) {
				status = "CANCELLED"
			}
		}
		if saveErr := cohortSave(s, p); saveErr != nil {
			r := cohortReport(c, p, status)
			r.AccountingValidation = cohortAccountingUnknown
			return r, errors.Join(cause, saveErr)
		}
		r := cohortReport(c, p, status)
		if observed, ok := b.(interface{ CohortLifecycle() Lifecycle }); ok {
			r.OriginalCleanup = observed.CohortLifecycle()
		}
		if evidence, ok := b.(interface {
			CohortFrontier() (uint64, string, pilot.PageCostPrivateV3, error)
		}); ok {
			g, tip, cost, frontErr := evidence.CohortFrontier()
			if frontErr != nil {
				return r, errors.Join(cause, frontErr)
			}
			r.LedgerGeneration = g
			r.LedgerTip = tip
			r.PolicyReservation = cost
			if recordErr := s.write("host/state-00000000.json", mustJSON(cohortFrontierRecord{g, tip, cost, r.OriginalCleanup})); recordErr != nil {
				return r, errors.Join(cause, recordErr)
			}
		}
		if r.AccountingValidation != "EXACT" {
			// Do not advertise a quartet as independently admitted history when
			// accounting evidence refused reconciliation. Known returns stay spent.
			return r, errors.Join(cause, ErrInvalid)
		}
		publishErr := cohortDeliver(s, &r)
		if publishErr != nil {
			r.Status = "PUBLICATION_FAILED"
		}
		return r, errors.Join(cause, publishErr)
	}
	exchange := func(indices []int, role, stage string) (CohortResponse, error) {
		var batch []CohortItem
		for _, n := range indices {
			batch = append(batch, p.Outcomes[n].Item)
		}
		t := bindCohortTask(c, batch, role, stage, p.Calls+1)
		raw := mustJSON(t)
		if len(raw) > c.MaxRequestBytes || p.Calls >= c.MaxCalls || p.RequestBytes+p.ResponseBytes > c.MaxTotalBytes-len(raw)-c.MaxResponseBytes || p.ResponseBytes > c.MaxOutputBytes-c.MaxResponseBytes {
			return CohortResponse{}, ErrPaused
		}
		if e := ctx.Err(); e != nil {
			return CohortResponse{}, e
		}
		// Claim is create-only before reservation snapshot and before disclosure.
		if e := s.write(fmt.Sprintf("claim/claim-%08d.json", t.Sequence), raw); e != nil {
			return CohortResponse{}, e
		}
		p.Calls++
		p.RequestBytes += len(raw)
		p.Inflight = true
		if e := cohortSave(s, p); e != nil {
			return CohortResponse{}, e
		}
		reply, backendErr := b.ExchangeCohort(ctx, raw)
		reply, retainErr := retainCohortReturn(s, c, &p, t.Sequence, raw, reply, backendErr == nil)
		if retainErr != nil {
			return CohortResponse{}, retainErr
		}
		if backendErr != nil {
			return CohortResponse{}, backendErr
		}
		if len(reply) > c.MaxResponseBytes || p.RequestBytes+p.ResponseBytes > c.MaxTotalBytes || p.ResponseBytes > c.MaxOutputBytes {
			return CohortResponse{}, ErrPaused
		}
		r, e := DecodeCohortResponse(reply)
		if e != nil {
			return r, e
		}
		if e = validateCohortResponse(t, r); e != nil {
			return r, e
		}
		p.Inflight = false
		for n, d := range r.Items {
			idx := indices[n]
			p.Outcomes[idx].History = append(p.Outcomes[idx].History, d)
			if role == "interpreter" {
				copy := d
				p.Outcomes[idx].Item.Proposal = &copy
			} else {
				copy := d
				p.Outcomes[idx].Item.Review = &copy
				if d.Disposition == "SUPPORTED" {
					p.Outcomes[idx].Status = "REVIEWED"
				} else {
					p.Outcomes[idx].Status = d.Disposition
				}
			}
		}
		return r, cohortSave(s, p)
	}
	// Packing measures the ACTUAL canonical frame with digest and reserved reviewer
	// proposals, not sum(body lengths). An oversized own body is never truncated.
	for start := 0; start < len(items); {
		var indices []int
		for end := start; end < len(items) && len(indices) < 10; end++ {
			candidate := append(append([]int{}, indices...), end)
			var batch []CohortItem
			for _, n := range candidate {
				i := p.Outcomes[n].Item
				batch = append(batch, i)
			}
			if len(mustJSON(bindCohortTask(c, batch, "reviewer", "REVIEW", p.Calls+2)))+c.MaxResponseBytes > c.MaxRequestBytes {
				break
			}
			indices = candidate
		}
		if len(indices) == 0 {
			return finish(ErrPaused)
		}
		if p.Calls > c.MaxCalls-2 {
			return finish(ErrPaused)
		}
		if _, e = exchange(indices, "interpreter", "PROPOSE"); e != nil {
			return finish(e)
		}
		if _, e = exchange(indices, "reviewer", "REVIEW"); e != nil {
			return finish(e)
		}
		var correct []int
		for _, n := range indices {
			if p.Outcomes[n].Status == "UNSUPPORTED" {
				o := &p.Outcomes[n]
				o.Item.Predecessor = hash(mustJSON(o.History[len(o.History)-1]))
				o.Item.Revision = 2
				o.Item.Attempt += "/correction-1"
				o.Status = "PENDING"
				correct = append(correct, n)
			}
		}
		if len(correct) > 0 {
			if p.Calls > c.MaxCalls-2 {
				return finish(ErrPaused)
			}
			if _, e = exchange(correct, "interpreter", "CORRECT"); e != nil {
				return finish(e)
			}
			if _, e = exchange(correct, "reviewer", "REREVIEW"); e != nil {
				return finish(e)
			}
		}
		start += len(indices)
	}
	if e = runCohortNavigation(ctx, c, &p, s, b); e != nil {
		return finish(e)
	}
	return finish(nil)
}

// Admission replays actual cohort physical history, never legacy flat Response.
// This is local consistency, not authentication against a same-user full forgery.
func latestCohort(s *store, c CohortConfig) (CohortReport, error) {
	s.rejectedPrior = false
	configRaw, configErr := s.read("plan/config-00000001.json", stateLimit)
	if configErr != nil || !bytes.Equal(configRaw, mustJSON(c)) {
		s.rejectedPrior = true
		return CohortReport{}, ErrInvalid
	}
	nums, e := s.resultNumbers()
	if e != nil {
		return CohortReport{}, e
	}
	for _, n := range nums {
		if n%2 == 0 {
			continue
		}
		raw, e := s.read(fmt.Sprintf("result/report-%08d.json", n), stateLimit)
		var r CohortReport
		if e != nil || strict(raw, &r) != nil || r.Schema != "private-reviewed-cohort-report.v1" || r.Profile != CohortProfile || r.ConfigSHA256 != CohortConfigDigest(c) || r.Accepted || r.Authority != 0 || r.FeatureIdentity != "UNRESOLVED" || r.SourceCompleteness != "UNKNOWN" || r.AbsenceInference != "PROHIBITED" {
			s.rejectedPrior = true
			continue
		}
		mp, hp := reportPaths(n)
		if r.MachinePaths != mp || r.HumanPaths != hp || verifyCohortQuartet(s, r) != nil || verifyCohortHistory(s, c, r) != nil || !verifyCohortSnapshot(s, c, r) {
			s.rejectedPrior = true
			continue
		}
		return r, nil
	}
	return CohortReport{}, ErrInvalid
}
func verifyCohortHistory(s *store, c CohortConfig, r CohortReport) error {
	if r.Status == "PROVISIONAL" && r.Inflight {
		return ErrInvalid
	}
	if !admitCohortGraph(r) {
		return ErrInvalid
	}
	if r.Requested != c.Requested || r.Requested < 1 || r.Requested > 64 || r.Reviewed < 0 || r.Reviewed > r.Requested || r.Gaps < 0 || r.Gaps > r.Requested-r.Reviewed || r.Pending != r.Requested-r.Reviewed-r.Gaps {
		return ErrInvalid
	}
	if r.Calls < 0 || r.Calls > c.MaxCalls || r.RequestBytes < 0 || r.ResponseBytes < 0 || r.RequestBytes > c.MaxTotalBytes || r.ResponseBytes > c.MaxOutputBytes || r.RequestBytes > c.MaxTotalBytes-r.ResponseBytes || r.Requested != len(r.Outcomes) || r.Requested != r.Reviewed+r.Gaps+r.Pending {
		return ErrInvalid
	}
	reqBytes, respBytes := 0, 0
	navHistory := []CohortNavigationResponse{}
	navStarted := false
	grammar := cohortGrammar{}
	hist := map[string][]CohortDecision{}
	initial := map[string]CohortItem{}
	last := map[string]CohortItem{}
	statuses := map[string]string{}
	for seq := 1; seq <= r.Calls; seq++ {
		raw, e := s.read(fmt.Sprintf("claim/claim-%08d.json", seq), c.MaxRequestBytes)
		var header struct{ Protocol string }
		if e == nil {
			json.Unmarshal(raw, &header)
		}
		if header.Protocol == CohortNavigationProtocol {
			if !grammar.acceptNavigation(r) {
				return ErrInvalid
			}
			navStarted = true
			nr, ns, ne := verifyNavigationExchange(s, c, r, seq, raw, &navHistory)
			if ne != nil {
				return ne
			}
			reqBytes += nr
			respBytes += ns
			continue
		}
		if navStarted {
			return ErrInvalid
		}
		var task CohortTask
		if e != nil || strict(raw, &task) != nil || task.Protocol != CohortProtocol || task.ConfigSHA256 != CohortConfigDigest(c) || task.Grant != c.Grant || task.Sequence != seq || len(task.Items) == 0 || len(task.Items) > 10 {
			return ErrInvalid
		}
		if grammar.accept(task, r) != nil {
			return ErrInvalid
		}
		digest := task.RequestDigest
		task.RequestDigest = ""
		if hash(mustJSON(task)) != digest {
			return ErrInvalid
		}
		task.RequestDigest = digest
		reqBytes += len(raw)
		seen := map[string]bool{}
		for _, i := range task.Items {
			if seen[i.ItemID] {
				return ErrInvalid
			}
			seen[i.ItemID] = true
			old, ok := last[i.ItemID]
			if !ok {
				if task.Role != "interpreter" || !validCohortItem(c, i) {
					return ErrInvalid
				}
				initial[i.ItemID] = i
			} else if task.Role == "reviewer" {
				if !bytes.Equal(mustJSON(old), mustJSON(i)) || i.Proposal == nil {
					return ErrInvalid
				}
			} else {
				if task.Role != "interpreter" {
					return ErrInvalid
				}
				h := hist[i.ItemID]
				if task.Stage != "CORRECT" || old.Revision != 1 || i.Revision != 2 || len(h) != 2 || h[1].Disposition != "UNSUPPORTED" || i.Predecessor != hash(mustJSON(h[1])) || i.Attempt != old.Attempt+"/correction-1" {
					return ErrInvalid
				}
				base := i
				base.Revision = old.Revision
				base.Predecessor = old.Predecessor
				base.Attempt = old.Attempt
				base.Proposal = old.Proposal
				if !bytes.Equal(mustJSON(base), mustJSON(old)) {
					return ErrInvalid
				}
			}
			last[i.ItemID] = i
		}
		reply, e := observedCohortReply(s, c, seq, raw)
		if os.IsNotExist(e) {
			if seq != r.Calls || !r.Inflight {
				return ErrInvalid
			}
			spent, spendErr := ambiguousMissingReplySpend(s, seq)
			if spendErr != nil {
				return spendErr
			}
			respBytes += spent
			continue
		}
		if e != nil {
			return e
		}
		respBytes += len(reply)
		receiptRaw, receiptErr := s.read(fmt.Sprintf("host/claim-%08d.json", seq), 4096)
		if seq == r.Calls && r.Inflight && os.IsNotExist(receiptErr) {
			continue
		}
		var receipt cohortReceipt
		if receiptErr != nil || strict(receiptRaw, &receipt) != nil || receipt.Sequence != seq || receipt.ResponseSHA256 != hash(reply) || receipt.Bytes != len(reply) {
			return ErrInvalid
		}
		if !receipt.BackendOK || len(reply) > c.MaxResponseBytes || seq == r.Calls && r.Inflight {
			if seq != r.Calls {
				return ErrInvalid
			}
			continue
		}
		response, e := DecodeCohortResponse(reply)
		if e != nil || validateCohortResponse(task, response) != nil {
			if seq != r.Calls {
				return ErrInvalid
			}
			continue
		}
		copyResponse := response
		grammar.response = &copyResponse
		for n, d := range response.Items {
			i := task.Items[n]
			hist[i.ItemID] = append(hist[i.ItemID], d)
			if task.Role == "interpreter" {
				copy := d
				i.Proposal = &copy
				last[i.ItemID] = i
			} else {
				copy := d
				i.Review = &copy
				last[i.ItemID] = i
				statuses[i.ItemID] = d.Disposition
			}
		}
	}
	if !bytes.Equal(mustJSON(navHistory), mustJSON(r.NavigationHistory)) && !(len(navHistory) == 0 && len(r.NavigationHistory) == 0) {
		return ErrInvalid
	}
	if reqBytes != r.RequestBytes || respBytes != r.ResponseBytes {
		return ErrInvalid
	}
	reviewed, gaps, pending := 0, 0, 0
	seen := map[string]bool{}
	for _, o := range r.Outcomes {
		if seen[o.Item.ItemID] {
			return ErrInvalid
		}
		seen[o.Item.ItemID] = true
		h := hist[o.Item.ItemID]
		if !bytes.Equal(mustJSON(h), mustJSON(o.History)) && !(len(h) == 0 && len(o.History) == 0) {
			return ErrInvalid
		}
		i, ok := last[o.Item.ItemID]
		if ok {
			// A pending correction may be prepared but never disclosed due to budget.
			if !bytes.Equal(mustJSON(i), mustJSON(o.Item)) {
				if i.Revision != 1 || o.Item.Revision != 2 || len(h) != 2 || h[1].Disposition != "UNSUPPORTED" || o.Status != "PENDING" || o.Item.Predecessor != hash(mustJSON(h[1])) {
					return ErrInvalid
				}
				base := o.Item
				base.Revision = i.Revision
				base.Predecessor = i.Predecessor
				base.Attempt = i.Attempt
				if !bytes.Equal(mustJSON(base), mustJSON(i)) {
					return ErrInvalid
				}
			}
		} else if !validCohortItem(c, o.Item) && !(r.Calls == 0 && o.Status == "PENDING" && o.Item.Body == "" && strings.HasPrefix(o.Item.ItemID, "unprepared-")) {
			return ErrInvalid
		}
		switch o.Status {
		case "REVIEWED":
			if statuses[o.Item.ItemID] != "SUPPORTED" {
				return ErrInvalid
			}
			reviewed++
		case "UNSUPPORTED", "WITHDRAWN", "UNRESOLVED":
			if statuses[o.Item.ItemID] != o.Status {
				return ErrInvalid
			}
			gaps++
		case "PENDING":
			if statuses[o.Item.ItemID] == "SUPPORTED" {
				return ErrInvalid
			}
			pending++
		default:
			return ErrInvalid
		}
	}
	if reviewed != r.Reviewed || gaps != r.Gaps || pending != r.Pending {
		return ErrInvalid
	}
	return nil
}
func LatestCohort(c CohortConfig) (CohortReport, error) {
	if ValidateCohortConfig(c) != nil {
		return CohortReport{}, ErrInvalid
	}
	s, e := openCohortStore(c)
	if e != nil {
		return CohortReport{}, e
	}
	defer s.close()
	return latestCohort(s, c)
}
