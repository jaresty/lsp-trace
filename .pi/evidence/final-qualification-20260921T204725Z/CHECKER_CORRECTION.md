# Qualification checker correction

The staged checker incorrectly required V2 `terminal_stage=CAPTURE`. The owning implementation contract in `internal/censusresult/continuation_diagnostic.go` assigns `terminal_stage=observed_stage` whenever an observed checkpoint exists. For capture failure after Program C persistence, the exact terminal pair is `PROGRAM_C_COMPUTED / FAILED_CAPTURE`; the separate `stage` field remains `CAPTURE`.

The original checker is retained as `check-result.pre-terminal-stage-correction.py`. No census request, session mutation, worker, or model was rerun or invoked for this correction.
