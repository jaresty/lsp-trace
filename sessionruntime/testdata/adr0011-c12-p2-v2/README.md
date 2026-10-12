# ADR-0011 C12 historical source packet

This packet preserves the predecessor logical source denominator under `historical/` using exact bytes from approved commit `a314e8cac860317b3c3046b0e5533054098ff570`.

The manifest is authority-only and does not claim broader matrix closure, live runtime proof, managed-session identity, or production authority. The selector names this packet as current and the predecessor manifest as historical. Live or dirty working-tree bytes are not substituted for the frozen snapshot.
