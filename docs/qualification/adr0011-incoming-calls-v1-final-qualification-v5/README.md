# ADR0011 incoming-calls v1 final qualification package

Status: `IMPLEMENTATION_QUALIFICATION_AUTHORED_NOT_GO`  
Authority: `false`; production GO: `false`; implementation GO: `false`; self-GO: `false`.

Source: `/private/tmp/adr0011-incoming-calls-v1-6yiY4m` at `49334ce23de37c0094f50d21d57ab251a46130ca`.

This package is docs/evidence-only. It records the Phase5 final qualification commands, logs, receipts, current patch, exact14 source post-hashes, StageB counts, package hashes, and deterministic verifier/tamper guard. It does not edit production or test source and does not authorize GO.

## Key recorded facts

- Four independent witnesses run in separate fresh Go test processes: EXACT32 x3, PLUS_ONE x3, PARTIAL_EXTERNAL x1, EARLY_TERMINAL x1.
- PLUS_ONE receipts assert zero Manager stdin Write calls/raw bytes/written bytes before provider frame decoding and successful same READY session reuse.
- PARTIAL_EXTERNAL and EARLY_TERMINAL are exact-once in this package.
- StageB verifier/counts recorded: 91 actual rows, 78 NECESSARY, 11 REUSE, 2 FALSE_POSITIVE, 0 unresolved, duplicate/omission 0.
- Deferred by ADR0011_V5_REFRAME and not required here: broad aggregate lifecycle/race/C15 scenarios beyond accepted StageB and four witnesses.

## Reproduce

```bash
python3 docs/qualification/adr0011-incoming-calls-v1-final-qualification-v5/verify_final_qualification.py --self-test
```

The authoring command was:

```bash
python3 docs/qualification/adr0011-incoming-calls-v1-final-qualification-v5/run_final_qualification.py
```
