#!/bin/sh
# Foreground only. Caller owns two anonymous pipes for this shell's stdin/stdout.
# stdin is a lifecycle lease (EOF cancels), NOT role JSON or coordinator input.
# stdout is one validated startup packet then EOF, NOT the coordinator terminal.
# Keep the foreground process/session handle until its actual exit notification.
set -eu
[ "$#" -eq 5 ] || exit 125
script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd -P)
# Preserve the caller's sole pipe endpoints across exec. Do NOT create shell
# aliases of stdout: a shell-saved writer can suppress the packet EOF. The
# foreground owner creates the separate inherited CLI notifier descriptor.
# No FIFO/path rendezvous, daemon, PID-file kill or detached shell child.
exec python3 "$script_dir/activation-host.py" "$@"
