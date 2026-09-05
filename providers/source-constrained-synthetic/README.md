# Source-constrained synthetic provider

Provisional, non-production provider for source-constrained discovery evidence. It is independently installable with `npm pack`/`npm install` and has identity `source-constrained-synthetic-provider`, version `1.0.0-provisional`.

The host must register its installed absolute executable path and explicitly select `source-constrained-synthetic-provider`. It is never eligible for `auto`, never changes `ember-glint@1` capabilities, and emits only `PROVIDER_REPORTED` observations labeled `SOURCE_CONSTRAINED_SYNTHETIC` and `PROVISIONAL_DISCOVERY` for `INVOKES_TASK` and `TRIGGERS_RELOAD`.

Analyzer decisions are bounded by `provenance.json`: exact pinned source objects, declaration chains, dependency versions, positive anchors, and negative boundaries. No Ember semantics are implemented in core Go.
