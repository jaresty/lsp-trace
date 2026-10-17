# Experimental handoff retained-root fixture

This fixture contains the 11 immutable continuation objects needed for one valid `REQUESTS_RENDERED` / `RUNNING` checkpoint chain with two targets split into two one-target constituents. `pins.json` records every immutable selector and byte length, the terminal checkpoint selector, and the deterministic prepared selector.

All retained workspace and source provenance is synthetic and stable: workspace `file:///fixture`, source `file:///fixture/a.go`. Generation used the existing census-continuation test composition with two one-target batches, then copied only the verified checkpoint chain and referenced artifacts into `continuations/objects/sha256/<digest>`. The one-shot generation code was removed after the fixture was repinned.

The fixture contains no credentials, machine-specific paths, test scratch roots, or generation-test identities. Tests scan every retained fixture file for forbidden provenance and credential markers before copying it to an owner-only runtime directory; production command execution never mutates this source directory.
