#!/bin/sh
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
tmp=$(mktemp -d "${TMPDIR:-/tmp}/lsp-trace-b05-frame6.XXXXXX")
trap 'rm -rf "$tmp"' EXIT HUP INT TERM
cd "$root/providers/ember-glint"
npm ci --ignore-scripts
npm test
pack=$(npm pack --pack-destination "$tmp" --silent)
package="$tmp/$pack"
mkdir "$tmp/install"
npm install --prefix "$tmp/install" --ignore-scripts --offline "$package" >/dev/null
provider="$tmp/install/node_modules/.bin/ember-glint"
[ -x "$provider" ] || { printf 'FAIL ASSERT_B05_FRAME6_PHYSICAL_PROVIDER\n' >&2; exit 1; }
package_digest=$(shasum -a 256 "$package" | cut -d' ' -f1)
provider_target=$(realpath "$provider")
provider_digest=$(shasum -a 256 "$provider_target" | cut -d' ' -f1)
cd "$root"
LSP_TRACE_EXTERNAL_PROVIDER_PATH="$provider" B05_FRAME6_COMMIT_FILE="$tmp/workspace-commit" go test ./cmd/lsp-trace-mcp -run TestProductionMCPB05TwentyFourAttemptQualification -count=1 -v
commit=$(tr -d '\n' < "$tmp/workspace-commit")
B05_WORKSPACE_COMMIT="$commit" B05_PROVIDER_PACKAGE_SHA256="sha256:$package_digest" B05_PROVIDER_EXECUTABLE_SHA256="sha256:$provider_digest" ./scripts/build-b05-frame6-matrix.py
go test ./internal/b05qualification -run TestFrame6ExactQualificationMatrix -count=1 -v
printf 'PASS ASSERT_B05_FRAME6_PHYSICAL_PROVIDER package=sha256:%s executable=sha256:%s\n' "$package_digest" "$provider_digest"
