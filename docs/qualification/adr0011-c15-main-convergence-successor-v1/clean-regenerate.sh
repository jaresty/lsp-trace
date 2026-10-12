#!/bin/sh
set -eu
REPO=${REPO:-/tmp/adr0011-main-convergence-candidate-vDUjnN}
PKG="$REPO/docs/qualification/adr0011-c15-main-convergence-successor-v1"
SCRATCH=${SCRATCH:-/tmp/adr0011-main-convergence-evidence-NhqDpN/repairs/c15-successor/corrected-regeneration}
TEMP="$REPO/sessionruntime/adr0011_c15_main_convergence_successor_enumerator_private_test.go"
cleanup() { rm -f "$TEMP"; }
trap cleanup EXIT HUP INT TERM
[ ! -e "$TEMP" ]
rm -rf "$SCRATCH"
mkdir -p "$SCRATCH/run1" "$SCRATCH/run2"
cp "$PKG/enumerator_test.go" "$TEMP"
ENUMERATOR_SHA256=$(shasum -a 256 "$TEMP" | cut -d' ' -f1)
COMMAND="env -i PATH=$PATH HOME=$HOME GOCACHE=${GOCACHE:-$HOME/Library/Caches/go-build} ADR0011_C15_SUCCESSOR_OUTPUT=<RUN> go test ./sessionruntime -run ^TestADR0011C15MainConvergenceSuccessorEnumerate$ -count=1 -v"
(
 cd "$REPO"
 env -i PATH="$PATH" HOME="$HOME" GOCACHE="${GOCACHE:-$HOME/Library/Caches/go-build}" ADR0011_C15_SUCCESSOR_OUTPUT="$SCRATCH/run1" go test ./sessionruntime -run '^TestADR0011C15MainConvergenceSuccessorEnumerate$' -count=1 -v >"$SCRATCH/run1.stdout.log" 2>"$SCRATCH/run1.stderr.log"
 env -i PATH="$PATH" HOME="$HOME" GOCACHE="${GOCACHE:-$HOME/Library/Caches/go-build}" ADR0011_C15_SUCCESSOR_OUTPUT="$SCRATCH/run2" go test ./sessionruntime -run '^TestADR0011C15MainConvergenceSuccessorEnumerate$' -count=1 -v >"$SCRATCH/run2.stdout.log" 2>"$SCRATCH/run2.stderr.log"
)
diff -ru "$SCRATCH/run1" "$SCRATCH/run2" >"$SCRATCH/run-diff.txt"
cmp "$SCRATCH/run1/candidates.tsv" "$PKG/canonical/candidates.tsv"
cmp "$SCRATCH/run1/classifications.tsv" "$PKG/canonical/classifications.tsv"
cmp "$SCRATCH/run1/all-source-locations.tsv" "$PKG/canonical/all-source-locations.tsv"
python3 - "$SCRATCH/run1" <<'PY'
import csv,sys
from pathlib import Path
p=Path(sys.argv[1])
def parse(name,n):
 rows=[];seen=set()
 for i,line in enumerate((p/name).read_text().splitlines(),1):
  f=line.split('\t')
  if len(f)!=n or not f[0] or f[0] in seen: raise SystemExit(f'{name}:{i}: malformed/duplicate')
  seen.add(f[0]);rows.append(f)
 return rows
c=parse('candidates.tsv',2);k=parse('classifications.tsv',5)
l=list(csv.DictReader((p/'all-source-locations.tsv').open(),delimiter='\t'))
assert len(c)==len(k)==len(l)==4155
assert {r[0] for r in c}=={r[0] for r in k}=={r['candidate_id'] for r in l}
assert sum(1 for r in c if r[0].startswith('c15.'))==44
print('rows=4155 duplicates=0 mapping=44 locations=4155')
PY
cleanup
trap - EXIT HUP INT TERM
{
 echo 'CLEAN_REGENERATION_RECEIPT_V2'
 echo "enumerator_sha256=$ENUMERATOR_SHA256"
 echo "command=$COMMAND"
 echo 'run1_exit=0'
 echo 'run2_exit=0'
 echo 'run1_rows=4155'
 echo 'run2_rows=4155'
 echo 'duplicates=0'
 echo 'mapping_rows=44'
 echo 'locations=4155'
 echo 'run_outputs_byte_identical=true'
 echo 'canonical_package_bytes_equal=true'
 echo 'temporary_enumerator_removed=true'
} >"$SCRATCH/CLEAN_REGENERATION_RECEIPT.txt"
cp "$SCRATCH/run1.stdout.log" "$PKG/regeneration-run1.stdout.log"
cp "$SCRATCH/run1.stderr.log" "$PKG/regeneration-run1.stderr.log"
cp "$SCRATCH/run2.stdout.log" "$PKG/regeneration-run2.stdout.log"
cp "$SCRATCH/run2.stderr.log" "$PKG/regeneration-run2.stderr.log"
cp "$SCRATCH/CLEAN_REGENERATION_RECEIPT.txt" "$PKG/CLEAN_REGENERATION.txt"
printf '%s\n' "$COMMAND" >"$PKG/COMMAND.txt"
