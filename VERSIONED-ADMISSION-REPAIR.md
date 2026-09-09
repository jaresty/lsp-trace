# Versioned Admission Repair

Baseline authority: `c47032f`
Reviewed evaluator repair: `ca86c50`

## RED boundary

Committed guard `TestProgramAAdmissionV1ReceiptCountFrozen` at `7bae740` produced:

`ASSERT_PROGRAM_A_V1_RECEIPT_COUNT_FROZEN: got=7 want=6`

This establishes the HOLD defect before implementation: the reviewed six-receipt Program A v1 admission was silently widened under its historical identity.
