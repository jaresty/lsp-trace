#!/usr/bin/env python3
"""Retain normalized textual qualification evidence for release review."""

from __future__ import annotations

import shutil
from pathlib import Path

LANGUAGES = ("typescript", "csharp", "elixir")
FILES = (
    "status.txt",
    "assertions.txt",
    "graph.json",
    "command.txt",
    "server-version.txt",
    "stderr.txt",
    "capability.txt",
    "setup.txt",
)


def main() -> None:
    root = Path(__file__).resolve().parent.parent
    source_root = root / "qualification" / "evidence"
    destination_root = root / "qualification" / "retained"
    marker = "${REPO}"

    for language in LANGUAGES:
        source = source_root / language
        status = source / "status.txt"
        if not status.is_file() or status.read_text(encoding="utf-8").strip() != "PASS":
            raise SystemExit(f"{language}: qualification status is not PASS")

        destination = destination_root / language
        if destination.exists():
            shutil.rmtree(destination)
        destination.mkdir(parents=True)

        for name in FILES:
            input_path = source / name
            if not input_path.is_file():
                continue
            text = input_path.read_text(encoding="utf-8", errors="replace")
            normalized = text.replace(str(root), marker)
            (destination / name).write_text(normalized, encoding="utf-8")


if __name__ == "__main__":
    main()
