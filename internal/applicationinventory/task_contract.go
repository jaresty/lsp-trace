//go:build darwin || linux

package applicationinventory

import (
	"encoding/json"
	dw "lsp-trace/internal/describeworker"
	cat "lsp-trace/internal/provisionalfeaturecatalog"
	"strings"
)

// Static bounded task metadata, like status, is uncharged. This does not enlarge
// persisted prompts, claim allocations, evidence limits or semantic budgets.
type TaskContract struct {
	Version            string
	ResponseSchema     string
	Encoding           string
	Rules              string
	Example            string
	Operations         map[string][]string
	MaxSubmissionBytes int
	MaxSemanticBytes   int
	RunByteLimit       uint64
	RunPacketLimit     uint64
}

const describeSemanticExample = `{"verdict":"COMPLETE","target_role":"performs the source-visible operation","consumer_need":{"status":"UNRESOLVED","value":""},"provided_behavior":{"value":"provides the behavior shown in the retained source body","consumer_relative":false},"boundary_contribution":"participates in the boundary shown in retained evidence","limitations":["illustrative schema specimen only; replace prose with evidence-supported judgment; consumer remains unresolved"]}`

func taskContract(t DurableTask) TaskContract {
	c := TaskContract{Version: "lsp-trace.census-task-contract.v1", MaxSubmissionBytes: 65536, RunPacketLimit: 64,
		Operations: map[string][]string{
			"task":         {"lsp-trace", "census", "durable", "task", t.RunID},
			"current_task": {"lsp-trace", "census", "durable", "task", t.RunID},
			"exact_task":   {"lsp-trace", "census", "durable", "task", t.RunID, t.TaskID},
			"read":         {"lsp-trace", "census", "durable", "read", t.RunID, t.TaskID, "input", "0", "32768"},
			"submit":       {"lsp-trace", "census", "durable", "submit", t.RunID, t.TaskID, t.Role},
			"result":       {"lsp-trace", "census", "durable", "result", t.RunID, t.TaskID},
		},
		Rules: "Operations are argv arrays, not shell strings. Read Data is base64: decode it, advance offset by decoded byte length until Total; page limit 1..32768. Read the entire input before judging. Submit only PENDING tasks: send the response on stdin and close stdin. EXPIRED and UNCERTAIN_SPENT tasks cannot be replayed. Use the returned next task, never replay a predecessor. If submit output is lost or ambiguous, do not automatically repeat this command: run current_task (task is its alias) to observe the current run task/status, including a successor, PUBLISHED, EXPIRED or UNCERTAIN_SPENT; never resubmit the predecessor. exact_task is only a current-ID check and becomes stale after advancement. DeadlineUnixNano and RunDeadlineUnixNano are absolute Unix nanoseconds and are not renewed by task/read. Complete malformed or oversized responses consume the claim and leave UNCERTAIN_SPENT; transport errors do not submit partial bytes. Same-thread/subagent actor declarations are unverified; review is always a separate sequential task. PUBLISHED is provisional, Authority 0, Accepted false, completeness UNKNOWN. These bounded instructions/status are uncharged metadata; read/submit/result retain existing cumulative charges. RunByteLimit is the stored run byte ceiling (at most 67108864); RunPacketLimit is 64. Used and Packets are cumulative, not remaining allowances. InputIndexBytes>0 selects verified immutable chunk delivery. Each read reserves 8*InputIndexBytes + 6*(whole intersecting 32768-byte chunks, with the actual last-chunk length) + 2*(returned byte count) + 16384 bytes and one packet. The one protected forward attempt uses offset RequiredInputNextOffset and limit 32768; partial, overlapping, out-of-order and repeated reads are optional charged work and cannot steal that tail. RequiredInputNextOffset records reserved attempts, not receipt: after lost output use current_task, never assume bytes were received or retry for free. A repeated read may be attempted only as separately charged optional work and can refuse. InputIndexBytes=0 (Published) retains whole-object verification: 6*Artifacts[0].Bytes + 2*(returned byte count) + 16384. Admission is staged: submission first permanently claims its existing slice, then admits successor construction/delivery before interpreting the answer. If that admission fails the task is UNCERTAIN_SPENT, with no semantic answer effects; not every maximal future answer is promised to fit. Already-spent allocations never shrink; only unspent future bounds may tighten to accepted adapter lengths. Submit and result also reserve bytes/work and packets; task/current_task/exact_task observation makes no reservation. No operation resets accounting or deadlines."}
	switch t.Phase {
	case "Describe":
		c.MaxSemanticBytes = 32768
		c.Rules += " Describe input is artifact input: concatenate decoded read Data pages into one JSON object. Source body is $.Unit.Body; the retained request is $.Request and its reused prompt is $.Request.Envelope.Prompt. Do not reacquire source or run a provider. The generic semantic parser and reused request prompt permit broader grammar (ABSTAINED, RESOLVED, consumer-relative); that grammar is not this retained-evidence lifecycle's admissibility contract. Only COMPLETE with unresolved consumer and non-consumer-relative behavior can advance. These other parser-valid variants are refused after the claim is spent. If evidence cannot support a truthful COMPLETE source-only description, stop and report inability to the caller without submitting; do not fabricate COMPLETE or submit ABSTAINED as a successful completion. The example is illustrative syntax, not a ready-made semantic answer."
		c.ResponseSchema = "Describe supplied adapter v2 + lsp-trace.describe-semantic.v2. Outer object in exact field order: status:string=COMPLETE, text:string containing semantic JSON, tokens:int, load_ms:number, run_ms:number, cancelled:bool, decode_status:int32, grammar:string, context_tokens:int. All outer fields shown are required by canonical encoding; omit error. For supplied judgments use 0 for counters/timings, false for cancelled, empty grammar (not measured execution). Task-specific admissible semantic required fields in example order: verdict:COMPLETE only; target_role:nonempty string <=4096 UTF-8 bytes; consumer_need:{status:UNRESOLVED,value:empty-string} only; provided_behavior:{value:nonempty string <=4096 bytes,consumer_relative:false} only; boundary_contribution:nonempty string <=4096 bytes; limitations:array of 0..8 nonempty strings each <=4096 bytes. Optional citation_suggestions object has target_role,consumer_need,provided_behavior,boundary_contribution,limitations arrays, each <=8 unique IDs drawn from C1,C2,PACKET_SCOPE,UNRESOLVED_CUSTODY. No unknown fields, duplicate keys, BOM or trailing values. Do not invent citations or execution identity."
		c.Encoding = "Exactly one compact canonical outer JSON object plus one LF, <=65536 bytes including LF; field order as schema/example, Go encoding/json escaping (escape < > & as \\u003c \\u003e \\u0026, U+2028/U+2029; no extra whitespace). text is a JSON-escaped string of semantic JSON, <=32768 decoded UTF-8 bytes. Semantic whitespace is accepted and canonicalized by the existing validator. The example is a schema specimen, not a judgment to copy without reading evidence."
		c.Example = string(dw.EncodeAdapterResultV2(describeSemanticExample))
	case "GroupDecision", "Reviewer":
		c.ResponseSchema = "NativeDecisionV3: object {predecessor:string,actor:string,reason:string,affected:string[],groups:{id:string,label:string,reason:string,members:string[]}[]}. predecessor must equal input catalog revision; actor must equal task Actor (1..256 bytes); reason 1..4096 bytes; groups and affected must be non-null arrays, each <=100. Group IDs unique, 1..256 bytes; label/reason 1..4096 bytes; members 1..100 eligible input Evidence.ID values, globally unique across groups. Empty groups is valid unresolved membership. affected must name exactly changed or removed predecessor group IDs, no duplicates; initial GroupDecision requires affected=[]. Reviewer supplies a complete replacement groups view, not a patch. Preserve unchanged groups exactly and list all changed predecessor IDs. No unknown fields, duplicate keys, null top-level object or trailing values."
		c.Encoding = "One JSON object, <=65536 bytes. Compact JSON in example field order is recommended; whitespace and a final LF are accepted (unlike the Describe outer canonical requirement). Replace CURRENT_INPUT_REVISION with the read input catalog revision; Actor is already populated."
		raw, _ := json.Marshal(cat.NativeDecisionV3{Predecessor: "CURRENT_INPUT_REVISION", Actor: t.Actor, Reason: "membership unresolved from supplied evidence", Affected: []string{}, Groups: []cat.NativeGroupV3{}})
		c.Example = string(raw)
	case "Published":
		c.ResponseSchema = "No response: publication is terminal. result verifies the publication; read returns corrected catalog."
		delete(c.Operations, "submit")
	}
	if t.Schema == "lsp-trace.durable-inventory-task.v2" {
		c.Version = "lsp-trace.census-task-contract.v2"
		c.Rules = strings.Replace(c.Rules, "DeadlineUnixNano and RunDeadlineUnixNano are absolute Unix nanoseconds and are not renewed by task/read.", "RunDeadlineUnixNano=0 explicitly means job expiry NONE. DeadlineUnixNano is the current finite task lease; task/read never renew it. Successors receive fresh bounded leases within the same cumulative 120-second semantic allowance.", 1)
	}
	return c
}
