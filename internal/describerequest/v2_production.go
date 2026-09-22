package describerequest

const variantESchema = `{"verdict":"COMPLETE|ABSTAINED","target_role":"string","consumer_need":{"status":"RESOLVED|UNRESOLVED","value":"string; empty exactly when UNRESOLVED"},"provided_behavior":{"value":"string","consumer_relative":true|false},"boundary_contribution":"string","limitations":["zero to eight strings"]}`
const variantEGuard = "Use only the packet. Never invent or name mechanical identity, consumer identity, ownership, product purpose, runtime use, canonical feature identity, value, or completeness. Host admissible evidence handles grounding. citation_suggestions is optional, non-authoritative, and not required. Output JSON only; no markdown or explanation."

// VariantEPromptPrefix is the exact task-first Variant E evaluation prompt before packet bytes.
// Variant E is an experimental operational baseline, not a semantic winner.
const VariantEPromptPrefix = "Task: describe what C1 provides, the semantic need it serves if resolved, and its one-layer boundary contribution; derive each value from this packet, never copy generic wording. Exact JSON schema: " + variantESchema + " " + variantEGuard + "\n\nPACKET:\n"

// PinnedRuntimeGrammarV2 is the corrected grammar proven by the pinned-runtime V2b preflight.
const PinnedRuntimeGrammarV2 = `root ::= base | suggested
base ::= ws "{" ws "\"verdict\"" ws ":" ws verdict ws "," ws "\"target_role\"" ws ":" ws string ws "," ws "\"consumer_need\"" ws ":" ws need ws "," ws "\"provided_behavior\"" ws ":" ws behavior ws "," ws "\"boundary_contribution\"" ws ":" ws string ws "," ws "\"limitations\"" ws ":" ws strings ws "}"
suggested ::= ws "{" ws "\"verdict\"" ws ":" ws verdict ws "," ws "\"target_role\"" ws ":" ws string ws "," ws "\"consumer_need\"" ws ":" ws need ws "," ws "\"provided_behavior\"" ws ":" ws behavior ws "," ws "\"boundary_contribution\"" ws ":" ws string ws "," ws "\"limitations\"" ws ":" ws strings ws "," ws "\"citation_suggestions\"" ws ":" ws suggestions ws "}"
verdict ::= "\"COMPLETE\"" | "\"ABSTAINED\""
need ::= "{" ws "\"status\"" ws ":" ws status ws "," ws "\"value\"" ws ":" ws maybe-string ws "}"
status ::= "\"RESOLVED\"" | "\"UNRESOLVED\""
behavior ::= "{" ws "\"value\"" ws ":" ws string ws "," ws "\"consumer_relative\"" ws ":" ws boolean ws "}"
boolean ::= "true" | "false"
suggestions ::= "{" ws "\"target_role\"" ws ":" ws ids ws "," ws "\"consumer_need\"" ws ":" ws ids ws "," ws "\"provided_behavior\"" ws ":" ws ids ws "," ws "\"boundary_contribution\"" ws ":" ws ids ws "," ws "\"limitations\"" ws ":" ws ids ws "}"
ids ::= "[" ws "]" | "[" ws id ws "]" | "[" ws id ws "," ws id ws "]"
id ::= "\"C1\"" | "\"C2\"" | "\"PACKET_SCOPE\"" | "\"UNRESOLVED_CUSTODY\""
strings ::= "[" ws "]" | "[" ws string ws "]" | "[" ws string ws "," ws string ws "]" | "[" ws string ws "," ws string ws "," ws string ws "]"
maybe-string ::= "\"\"" | string
string ::= "\"" char+ "\""
char ::= [^"\\\x00-\x1F] | "\\" (["\\/bfnrt] | "u" hex hex hex hex)
hex ::= [0-9a-fA-F]
ws ::= [ \t\n\r]*`
