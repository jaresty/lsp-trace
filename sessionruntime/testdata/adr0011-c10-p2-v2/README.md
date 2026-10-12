# ADR-0011 C10 historical source packet

This packet preserves the predecessor logical source denominator under `historical/` using exact bytes from approved commit `3aae83ffae0ffd78d11eeab04c65ffa39e728cfe`.

The manifest is authority-only and does not claim broader matrix closure, live runtime proof, managed-session identity, or production authority. The selector names this packet as current and the predecessor manifest as historical. Live or dirty working-tree bytes are not substituted for the frozen snapshot.
