#!/bin/sh
set -eu

assertion=ASSERT_CORE_ARCHIVES_EXCLUDE_PROVIDER_ASSETS
[ "$#" -gt 0 ] || { printf 'FAIL %s: archive path required\n' "$assertion" >&2; exit 1; }

for archive in "$@"; do
  case "$archive" in
    *.tar.gz|*.tgz) entries=$(tar -tzf "$archive") ;;
    *.zip) entries=$(unzip -Z1 "$archive") ;;
    *) printf 'FAIL %s: unsupported archive: %s\n' "$assertion" "$archive" >&2; exit 1 ;;
  esac
  if printf '%s\n' "$entries" | grep -E '(^|/)(providers/ember-glint|ember-glint-provider|lsp-trace-provider-ember-glint)(/|$)' >/dev/null; then
    printf 'FAIL %s: provider asset crossed core archive boundary: %s\n' "$assertion" "$archive" >&2
    exit 1
  fi
  for notice in LICENSE THIRD_PARTY_NOTICES; do
    if ! printf '%s\n' "$entries" | grep -E "(^|/)$notice$" >/dev/null; then
      printf 'FAIL ASSERT_CORE_ARCHIVES_INCLUDE_MANDATORY_NOTICES: missing %s in %s\n' "$notice" "$archive" >&2
      exit 1
    fi
  done
done
printf 'PASS %s\n' "$assertion"
