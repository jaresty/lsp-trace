# ADR-0011 layer-1 generic v3 campaign result

## Disposition

HARD STOP — canonical C15 v2 candidate delta cannot be exhaustively enumerated within the closed four-file campaign inventory.

## Implemented and verified

- Focused v3 corpus: GREEN.
- Focused v3 corpus `-count=3`: GREEN (`NORMAL3.log`).
- Focused v3 corpus `-race -count=2`: GREEN (`RACE2.log`).
- Final focused rerun: GREEN (`FINAL_FOCUSED_GREEN.log`).
- Targeted non-RED v2/lifecycle/lease/Manager regressions: GREEN (`TARGETED_REGRESSIONS.log`).
- `git diff --check`: GREEN.
- Modified-file boundary: exactly the four authorized files (`BOUNDARY_STATUS.txt`).
- No commit created.

## Canonical C15 hard stop

The retained canonical command was run:

```sh
go test ./sessionruntime -run '^TestPrivateB4C15V2SuccessorCatalog$' -count=1 -v
```

It stopped at:

```text
ASSERT_C15_V2_FINAL_CANDIDATES got=4154 want=3847
```

Full output: `C15_V2_CANONICAL.log`.

The repository's reviewed exhaustive delta procedure requires adding the temporary same-package file `sessionruntime/adr0011_callhierarchy_c15_delta_enumerator_private_test.go`. That path is outside this campaign's closed inventory. The retained canonical test is a frozen baseline oracle and cannot emit the required generated candidate/classification delta after a candidate-count change. Creating the enumerator, changing the retained oracle, or writing tracked generated artifacts would violate the campaign boundary.

Therefore the required canonical C15 delta/inventory comparison cannot be completed without renewed write-set authority. No workaround was attempted.

## Preserved evidence

- `PATCH.diff`
- `PRE_HASHES.sha256`
- `POST_HASHES.sha256`
- `BASE.txt`
- `HEAD.txt`
- `BOUNDARY_STATUS.txt`
- `GREEN.log`, `NORMAL3.log`, `RACE2.log`, `FINAL_FOCUSED_GREEN.log`
- `TARGETED_REGRESSIONS.log`
- `C15_V2_CANONICAL.log`
- earlier RED/GREEN attempt logs
