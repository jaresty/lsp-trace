#!/bin/sh
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
case " ${*:-} " in *" --retain "*) exec node "$root/qualification/caller-project-javascript/qualify.mjs" --retain;; esac
exec node "$root/qualification/caller-project-javascript/qualify.mjs"
