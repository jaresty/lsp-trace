# Frozen target-corpus contract

## Source universe

The universe is every tracked path ending `.go` in Git tree `761474bf5d90bbe3468df62b1a68a1fa26dc2c82` at revision `4b910fc0c517cb7801fb51d17d295897230d11fa`. Every member appears exactly once in `source-manifest.json` with bytes, SHA-256, classification, and reason.

The existing frozen source-pin contract admitted non-test Go implementation bytes, excluded docs/tests, and included platform-specific files. This package makes that rule repository-wide and explicit:

1. exclude `.pi/evidence/**` as retained tooling evidence;
2. exclude `docs/**` as retained documentation, evidence, or pilots;
3. exclude a path containing a `vendor` component;
4. exclude a path containing a `testdata` component;
5. exclude `*_test.go` as `EXCLUDED_TEST`;
6. exclude files carrying the standard `Code generated ... DO NOT EDIT.` header in their first 20 lines;
7. admit every remaining tracked `.go` file, irrespective of current GOOS, GOARCH, filename suffix, or build constraint.

Tests are therefore explicitly classified, never silently omitted. No untracked file participates. No build-list or gopls result controls admission.

## Parser census

Each admitted file is read from the pinned Git object and parsed independently with Go `parser.ParseFile(..., parser.AllErrors)`. Every named `*ast.FuncDecl`, including methods, is retained. Any parser error fails generation. Provider outcomes can never remove a target.

`DIRECT_POSITION` is the only acquired binding mode. Its zero-based UTF-16 character and zero-based line identify `FuncDecl.Pos()`; byte offsets are exact zero-based half-open `[byte_start, byte_end)` declaration bounds. UTF-16 values are computed from source bytes, not byte columns.

`DOCUMENT_SYMBOL_BOUND` is not claimed. A future binding would require a separately defined identity, exact acquired document-symbol evidence, and no fallback to `DIRECT_POSITION`.

## Canonical identity and order

Target identity fields are, in order: workspace revision; source-file SHA-256; decimal declaration byte start; decimal declaration byte end; package name; canonical receiver type text (empty for functions); symbol name. The ID is lowercase SHA-256 over each UTF-8 field encoded as unsigned 64-bit big-endian length followed by exact bytes. Source path is retained for location but is not an identity field.

Targets sort by source path, byte start, then ID. Sources sort by path. Duplicate IDs, parser errors, unaccounted tracked Go files, wrong revision/tree, or a dirty producer workspace fail closed.

## Claim ceiling

This is a bounded, parser-derived target denominator. It makes no claim about features, execution, call existence, runtime reachability, ownership, purpose, semantic equivalence, provider completeness, repository completeness, production authority, or acceptance. Authority is 0; completeness is `UNKNOWN`; acceptance is `false`; production admission is `REFUSED_MISSING_EVIDENCE`.
