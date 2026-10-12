# ADR-0011 C08 historical source packet

This packet preserves the predecessor logical source denominator under `historical/` using exact bytes from approved commit `7063a74fc42be6b24abfc50bdbaedf9778b5e1e4`.

The manifest is authority-only and does not claim broader matrix closure, live runtime proof, managed-session identity, or production authority. The selector names this packet as current and the predecessor manifest as historical. Live or dirty working-tree bytes are not substituted for the frozen snapshot.
