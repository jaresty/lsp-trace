# V6 / packet retained-evidence inspection checklist

Do not check boxes from structural inference, filenames, summaries, or model output. Record retained object IDs/digests and exact JSON pointers beside every checked item.

## Gateway and execution boundary

- [ ] The retained outer delegated-gateway response is inspected first; its `delegated_outcome`, `delegated_is_error`, and delegated digest are foregrounded in the review record.
- [ ] Public response, post-result session-list, and execution-accounting summary are separately retained with custody; no worker/model counters are invented in the public response.
- [ ] `check-result.py` accepts exactly one strict terminal class.
- [ ] Post-result `Census.Workers` and execution-accounting `workers` are both exactly zero.
- [ ] Execution-accounting `model_invocations` is exactly zero.

## V6 source projection

- [ ] Every selected target has its full definition/display range and full retained definition body.
- [ ] Every immediate consumer has its full definition/display range and full retained definition body.
- [ ] Every relation occurrence preserves the exact server-reported occurrence range independently of definition/display ranges.
- [ ] Custody is explicit for each source-bearing object and range; no unknown custody is silently promoted.
- [ ] No transitive source body is present: only targets, immediate consumers, and explicitly permitted exact relation occurrences are admitted.
- [ ] Mixed availability/unavailability is represented per object/range without collapsing partial success into complete capture.

## Packet and replay identity

- [ ] Packet object counts, range counts, byte accounting, truncation/paging fields, and unavailable-object records reconcile exactly.
- [ ] Each packet selector names immutable retained evidence and its verification selector/digest.
- [ ] Replaying each selector resolves the identical artifact schema ID, generation, digest, byte length, selected graph subject, logical source ID, and ordered ranges.
- [ ] Selector replay does not reacquire live source, invoke MCP, invoke a worker, or invoke a model.
- [ ] Any selector mismatch, missing page, reordered range, changed digest, or unavailable retained object is a qualification failure.

## Outcome-specific evidence

- [ ] For composite V2 `PAUSED`, `catalog.status=PAUSED`; request count is positive, preparation count is zero, checkpoint/composite/catalog selectors are valid, authority is zero, accepted is false, and completeness is unknown.
- [ ] For historical V1 typed continuation failure, the unchanged strict V1 fields establish exactly `CONTINUATION_CAPTURE_FAILED` / `FAILED_CAPTURE`, preserved census commit, retry false, and restart from preserved commit.
- [ ] For additive V2 typed continuation failure, V1-equivalent failure invariants also hold and selector state, observed/last-successful/terminal stages, terminal status, preserved census selector, and authority zero satisfy the V2 contract.
