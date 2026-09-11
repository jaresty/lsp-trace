#!/bin/sh
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
retain=false
offline=false
provider=''
package=''
while [ "$#" -gt 0 ]; do
  case "$1" in
    --retain)
      retain=true
      shift
      ;;
    --offline)
      offline=true
      shift
      ;;
    --provider)
      [ "$#" -ge 2 ] || { printf '%s\n' 'missing value for --provider' >&2; exit 2; }
      provider=$2
      shift 2
      ;;
    --package)
      [ "$#" -ge 2 ] || { printf '%s\n' 'missing value for --package' >&2; exit 2; }
      package=$2
      shift 2
      ;;
    *)
      printf 'usage: %s [--retain] | --offline --provider ABSOLUTE_PATH --package ABSOLUTE_PATH\n' "$0" >&2
      exit 2
      ;;
  esac
done

if [ "$offline" = true ]; then
  [ "$retain" = false ] || { printf '%s\n' '--offline cannot be combined with --retain' >&2; exit 2; }
  case "$provider" in /*) ;; *) printf '%s\n' '--provider must be an absolute path' >&2; exit 2 ;; esac
  case "$package" in /*) ;; *) printf '%s\n' '--package must be an absolute path' >&2; exit 2 ;; esac
  [ -f "$provider" ] && [ -x "$provider" ] || { printf '%s\n' '--provider must name an executable regular file' >&2; exit 2; }
  [ -f "$package" ] || { printf '%s\n' '--package must name a regular file' >&2; exit 2; }
fi

tmp=$(mktemp -d "${TMPDIR:-/tmp}/lsp-trace-b05-frame6.XXXXXX")
trap 'rm -rf "$tmp"' EXIT HUP INT TERM
if [ "$offline" = false ]; then
  cd "$root/providers/ember-glint"
  npm ci --ignore-scripts
  npm test
  pack=$(npm pack --pack-destination "$tmp" --silent)
  package="$tmp/$pack"
  mkdir "$tmp/install"
  npm install --prefix "$tmp/install" --ignore-scripts --offline "$package" >/dev/null
  provider="$tmp/install/node_modules/.bin/ember-glint"
fi
[ -x "$provider" ] || { printf 'FAIL ASSERT_B05_FRAME6_PHYSICAL_PROVIDER\n' >&2; exit 1; }
package_digest=$(shasum -a 256 "$package" | cut -d' ' -f1)
provider_target=$(realpath "$provider")
provider_digest=$(shasum -a 256 "$provider_target" | cut -d' ' -f1)
cd "$root"
LSP_TRACE_EXTERNAL_PROVIDER_PATH="$provider" B05_FRAME6_COMMIT_FILE="$tmp/workspace-commit" go test ./cmd/lsp-trace-mcp -run TestProductionMCPB05TwentyFourAttemptQualification -count=1 -v
commit=$(tr -d '\n' < "$tmp/workspace-commit")
if [ "$retain" = true ]; then
  B05_WORKSPACE_COMMIT="$commit" B05_PROVIDER_PACKAGE_SHA256="sha256:$package_digest" B05_PROVIDER_EXECUTABLE_SHA256="sha256:$provider_digest" ./scripts/build-b05-frame6-matrix.py
  ./scripts/build-b05-qualification-evidence-v3.py --retain
else
  B05_WORKSPACE_COMMIT="$commit" B05_PROVIDER_PACKAGE_SHA256="sha256:$package_digest" B05_PROVIDER_EXECUTABLE_SHA256="sha256:$provider_digest" ./scripts/build-b05-frame6-matrix.py --output "$tmp/legacy-current.json"
  ./scripts/build-b05-qualification-evidence-v3.py --legacy-input "$tmp/legacy-current.json" --output "$tmp/qualification-evidence.v3.json"
fi
go test ./internal/b05qualification -run 'TestFrame6ExactQualificationMatrix|TestHistoricalAndCurrentEvidenceAreReconciledAdditively' -count=1 -v
printf 'PASS ASSERT_B05_FRAME6_PHYSICAL_PROVIDER package=sha256:%s executable=sha256:%s\n' "$package_digest" "$provider_digest"
