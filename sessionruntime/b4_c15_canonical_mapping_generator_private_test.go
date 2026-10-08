package sessionruntime

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type privateC15BindingForm string

const (
	privateC15MarkerAdjacentV1 privateC15BindingForm = "MARKER_ADJACENT_V1"
	privateC15SelfBoundV1      privateC15BindingForm = "SELF_BOUND_V1"
)

type privateC15SemanticKey struct {
	SiteID           string                `json:"site_id"`
	FunctionIdentity string                `json:"semantic_function_identity"`
	RoleSelector     string                `json:"operation_role_call_selector"`
	BindingForm      privateC15BindingForm `json:"binding_form"`
}

type privateC15SemanticContract struct {
	Key          privateC15SemanticKey
	Path         string
	FrozenDigest string
}

// privateC15SemanticContracts is the hand-authored semantic source of truth.
// Digests freeze integrity after semantic matching; they never identify or order occurrences.
var privateC15SemanticContracts = []privateC15SemanticContract{
	c15c("c15.owner.01.v2-transaction-table", "sessionruntime/b4_explicit_bytes_c15_private.go", "newPrivateB4ByteAccountV2", "fixed-backing-owner-initialize", privateC15MarkerAdjacentV1, "8bf35d48a2a4b47bf9174ab1fe0084b84734778afb1571f6c6cc29f8b6ecd4ec"),
	c15c("c15.owner.04.manager-source-ingress-table", "sessionruntime/b4_explicit_bytes_c15_private.go", "newPrivateB4SourceIngressOwner", "fixed-backing-owner-initialize", privateC15MarkerAdjacentV1, "1cec6e12cf3f196bb4a87d00860bd6610cc056aa53284dba9c06987f7078804a"),
	c15c("c15.owner.05.per-source-ingress-state", "sessionruntime/b4_definition_private.go", "PreparePrivateB4DefinitionSource", "reserve-governed-state", privateC15MarkerAdjacentV1, "cc863fce42cb0efebc9679ead56829f335611ae6c8e95c3b4e678dd1da65b219"),
	c15c("c15.owner.08.source-descriptor-transfer", "sessionruntime/b4_explicit_bytes_c15_private.go", "preparePrivateB4C15Mutation", "atomic-custody-transfer", privateC15SelfBoundV1, "2954beed783a6a031155be04de8f8663ed1e7333062798e8033e01c9e33d4851"),
	c15c("c15.owner.09.request-frame-capture-charge", "sessionruntime/b4_definition_private.go", "reservePrivateB4RequestFrame", "batch-reserve-temporary-owner:element0", privateC15MarkerAdjacentV1, "1e1eae3c5e29ef738aff6be371459c5caf91606406e854286f56226b4678a202"),
	c15c("c15.owner.10.request-frame-retained-charge", "sessionruntime/b4_definition_private.go", "reservePrivateB4RequestFrame", "batch-reserve-temporary-owner:element1", privateC15MarkerAdjacentV1, "f527c958255cb25d5da8422ca7eab85cb54ec95a5c46ae5bdbf4a6161e2d834f"),
	c15c("c15.owner.11.caller-request-frame", "sessionruntime/b4_definition_private.go", "reservePrivateB4RequestFrame", "lease-transfer-to-caller-owner", privateC15MarkerAdjacentV1, "e2ebc0f3b9f8608a07f1a2b2363c47ac8ec64fc224fa63c7f3c98737998aaee7"),
	c15c("c15.owner.13.outbound-canonical-frame", "sessionruntime/private_encoded_request.go", "Write", "reserve-before-copy-owner", privateC15MarkerAdjacentV1, "e96f37cfbe9222dc703a78b0bea760bcc7911271e19fe4a435c7990862c183c9"),
	c15c("c15.owner.14.successor-prefetch", "internal/lspwire/successor_ingress.go", "NewSuccessorIngressReader", "reserve-before-make-owner:prefetch", privateC15MarkerAdjacentV1, "6eae36592dab141de01c1723a2d111c6dc2a138c42ba6aa88c07357313c96d04"),
	c15c("c15.owner.15.successor-header", "internal/lspwire/successor_ingress.go", "ReadFrame", "reserve-before-make-owner:header", privateC15MarkerAdjacentV1, "83d6c70f7171ddc3ec56d1c34829ea3b11d9fb1146644ac91da6a2c2b7bbac28"),
	c15c("c15.owner.16.successor-consume-scratch", "internal/lspwire/successor_ingress.go", "consumeCopy", "reserve-before-copy-owner:consume-scratch", privateC15MarkerAdjacentV1, "89f2f64210399f1715e49ad3700b9cf99e63f2ef5b383a7e89a17d2a58294ce3"),
	c15c("c15.owner.17.successor-body", "internal/lspwire/successor_ingress.go", "ReadFrame", "reserve-before-make-owner:body", privateC15MarkerAdjacentV1, "4fd6893c0ef9cb18905d34cf7b1e012fb12ba2772bff9a8fa2bf32361d8a737d"),
	c15c("c15.owner.22.result-token-replacement", "sessionruntime/b4_definition_private.go", "ObserveOriginalResultOwned", "replace-governed-owner", privateC15MarkerAdjacentV1, "34b126a0afcdba3a23caa68316be1978a656eb886723e7ebf722692147dfbb8e"),
	c15c("c15.owner.25.diagnostic-event-backing", "sessionruntime/diagnostic_backing_c15_private.go", "ReserveEventBacking", "reserve-transfer-owner", privateC15MarkerAdjacentV1, "b0ec4b7311781c4453c52dd86d674f67c4a4a4f167217364699c61c4916940b3"),
	c15c("c15.owner.27.fixed-diagnostic-history", "sessionruntime/diagnostic_backing_c15_private.go", "newManagerDiagnosticOwner", "fixed-backing-owner-initialize", privateC15MarkerAdjacentV1, "f9873d790b6603532a9bb77a86ab9e666439ceca84c93257ced9edfcdc26c2e6"),
	c15c("c15.alias.29.notifications-callback", "sessionruntime/b4_retained_notification_c15_private.go", "WithNotifications", "callback-borrow-alias:notifications", privateC15MarkerAdjacentV1, "dd34c734a0d6046382dd9d0c45ca1bce54195fb56cd0bf6c0870e3cd34f57d88"),
	c15c("c15.alias.31.responses-callback", "sessionruntime/b4_retained_response_c15_private.go", "WithResponses", "callback-borrow-alias:responses", privateC15MarkerAdjacentV1, "dd34c734a0d6046382dd9d0c45ca1bce54195fb56cd0bf6c0870e3cd34f57d88"),
	c15c("c15.alias.33.server-error-callback", "sessionruntime/b4_server_error_lease_c15_private.go", "WithServerError", "callback-borrow-alias:server-error", privateC15MarkerAdjacentV1, "dd34c734a0d6046382dd9d0c45ca1bce54195fb56cd0bf6c0870e3cd34f57d88"),
	c15c("c15.alias.36.owned-pair-callback", "sessionruntime/b4_owned_method_pair_lease.go", "WithPair", "callback-borrow-alias:owned-pair", privateC15MarkerAdjacentV1, "e62b8c641f880ed0e4a7c0762430f0746928a12190b2dac9c493e7d270682920"),
	c15c("c15.alias.43.borrowed-lexical-result", "sessionruntime/b4_definition_private.go", "PreparePrivateB4DefinitionBorrowed", "manager-scoped-borrow-alias:result", privateC15MarkerAdjacentV1, "d1cdc680a94adff244493e777ea9f12ceedc9110741f51d7168d20d0c83e6167"),
	c15c("c15.alias.44.borrowed-capture-values", "sessionruntime/b4_definition_private.go", "PreparePrivateB4DefinitionBorrowed", "callback-struct-slice-alias:capture", privateC15MarkerAdjacentV1, "b55d3ff44111d3d3280053bdc60a7933ddfe1d0f873aca1dfa430c711370492f"),
	c15c("c15.alias.45a.server-request-id-token", "sessionruntime/b4_server_request_c15_private.go", "writePrivateB4UnsupportedResponse", "synchronous-subslice-alias:id-token", privateC15MarkerAdjacentV1, "17b5724836f096ecf857b1fcd5b3b227f2583691dc751417c01232d548d7b60f"),
	c15c("c15.copy.02.params-ingress", "sessionruntime/b4_explicit_bytes_c15_private.go", "copyPrivateB4Params", "reserve-before-append-copy", privateC15SelfBoundV1, "15d86c1d750ea7c3536a30be5b902559a7a15b9aefedb2afdb6d160ff6c18d13"),
	c15c("c15.copy.03.prepared-source-content", "sessionruntime/b4_definition_private.go", "PreparePrivateB4DefinitionSource", "reserve-before-content-copy", privateC15MarkerAdjacentV1, "b3165e74010360f5329082a50e3b951daa400194ed67f84ad11cfee54863b931"),
	c15c("c15.copy.06.source-lease-handle-slice", "sessionruntime/b4_definition_private.go", "RoundTripPrivateB4", "reserve-before-make-and-copy:source-leases", privateC15MarkerAdjacentV1, "cafbc3612a3179bf43289f99cc05ab78b63ae83826079c50b0337c131a7a853e"),
	c15c("c15.copy.07.transaction-source-bytes", "sessionruntime/b4_definition_private.go", "admitPrivateB4SourcesC17Locked", "batch-reserve-before-per-source-copy:transaction-source", privateC15MarkerAdjacentV1, "eaad67fde858297206a845dd2c6df1206d781b1c5ff7e8b7affbde422b790069"),
	c15c("c15.copy.12.encoded-request-body", "sessionruntime/private_encoded_request.go", "Write", "reserve-before-make-copy:encoded-body", privateC15MarkerAdjacentV1, "0d089a0e927ec22dc664bd51961512c112d24e71697defb252ba0d2624c7190e"),
	c15c("c15.copy.18.original-response-frame", "internal/lspwire/successor_ingress.go", "ReadFrame", "reserve-before-frame-assembly-copy:original-frame", privateC15MarkerAdjacentV1, "7993548bb733b175e7a288499cd7ba0a915e9e9499335b412927bfc4992cb4da"),
	c15c("c15.copy.21.exact-result-token", "internal/lspwire/successor_ingress.go", "ReadFrame", "reserve-before-span-copy:result-token", privateC15MarkerAdjacentV1, "ca9abf64068aa6cde2f00db3b66a679de704e3bf3d8d846430a72b8c21b2c9d9"),
	c15c("c15.copy.23.decoded-message-clone", "internal/lspwire/successor_ingress.go", "CloneSuccessorMessageOwned", "reserve-before-deep-clone:decoded-message", privateC15MarkerAdjacentV1, "0ac4caa88048c1ff8467c7ce737490ec8f47d5bd9d24911ca95356ff60e3adea"),
	c15c("c15.copy.26.diagnostic-metadata-strings", "sessionruntime/diagnostic_runtime.go", "describeDiagnosticOperation", "reserve-before-string-clone:diagnostic-metadata", privateC15MarkerAdjacentV1, "4d00ea4cb9255f5de7821b0c67e56b03bc0ca61d62222acc1cd182380895ba23"),
	c15c("c15.copy.28.retained-notification-set", "sessionruntime/b4_retained_notification_c15_private.go", "newPrivateB4RetainedNotificationOwner", "aggregate-reserve-before-deep-copy:notifications", privateC15MarkerAdjacentV1, "4480c8e800e46ddb09071bd2c21d5595f070fdfdc426d7d84c4dbd79079920c4"),
	c15c("c15.copy.30.retained-response-set", "sessionruntime/b4_retained_response_c15_private.go", "newPrivateB4RetainedResponseOwner", "aggregate-reserve-before-deep-copy:responses", privateC15MarkerAdjacentV1, "4480c8e800e46ddb09071bd2c21d5595f070fdfdc426d7d84c4dbd79079920c4"),
	c15c("c15.copy.32.server-error-message-data", "sessionruntime/b4_server_error_lease_c15_private.go", "newPrivateB4ServerErrorOwner", "reserve-before-string-and-data-copy:server-error", privateC15MarkerAdjacentV1, "4480c8e800e46ddb09071bd2c21d5595f070fdfdc426d7d84c4dbd79079920c4"),
	c15c("c15.copy.34.owned-pair-params", "sessionruntime/b4_owned_method_pair_lease.go", "newPrivateB4OwnedMethodPairOwner", "combined-reserve-before-copy-component:params", privateC15MarkerAdjacentV1, "01460c23623bc5c6109de86bd6539ff7326017c3d40d71bda9afd9ddcc7e333d"),
	c15c("c15.copy.35.owned-pair-result", "sessionruntime/b4_owned_method_pair_lease.go", "newPrivateB4OwnedMethodPairOwner", "combined-reserve-before-copy-component:result", privateC15MarkerAdjacentV1, "deefde93313a5e7a253e293111614f9119c76bc0c3ae5239e84794c82656827b"),
	c15c("c15.copy.37.persistent-capture-params", "sessionruntime/b4_definition_private.go", "RoundTripPrivateB4", "pre-reserved-later-copy:params", privateC15MarkerAdjacentV1, "144606bcf990382a346de5156d75ae51399853d72c0835aef6d050bd0713b629"),
	c15c("c15.copy.39.final-capture-request-frame", "sessionruntime/b4_definition_private.go", "privateB4CaptureLocked", "batch-reserve-before-copy:request-frame", privateC15MarkerAdjacentV1, "1d384fbd0823d965011c3ce574954fc498aa0a28df27ff093a03474810755b17"),
	c15c("c15.copy.40.final-capture-request-params", "sessionruntime/b4_definition_private.go", "privateB4CaptureLocked", "batch-reserve-before-copy:request-params", privateC15MarkerAdjacentV1, "f3780832f4aacc2ca5640d67e0ed6afbd963f0e0967af7337f3a7d21b98ef9fe"),
	c15c("c15.copy.41.final-capture-response-frame", "sessionruntime/b4_definition_private.go", "privateB4CaptureLocked", "batch-reserve-before-copy:response-frame", privateC15MarkerAdjacentV1, "661da7edd247b5a20e1f956b1974194437ffe83c04c628b090468dc3fe51ad20"),
	c15c("c15.copy.42a.final-capture-target-slice", "sessionruntime/b4_definition_private.go", "privateB4CaptureLocked", "batch-reserve-before-slice-copy:target-slice", privateC15MarkerAdjacentV1, "e086e27e1fac65703bcf8e98b99287024bd617126a48f1d1ad16c9fc3dd338cb"),
	c15c("c15.copy.42b.final-capture-source-bytes", "sessionruntime/b4_definition_private.go", "privateB4CaptureLocked", "batch-reserve-before-per-source-copy:source-bytes", privateC15MarkerAdjacentV1, "2df783d35c252bf702697529aee5f1c416085a1bd88e72d257f43af10d92c825"),
	c15c("c15.copy.45b.unsupported-response-body", "sessionruntime/b4_server_request_c15_private.go", "writePrivateB4UnsupportedResponse", "atomic-batch-reserve-before-make-copy:body", privateC15SelfBoundV1, "ca6c97705702da23c3e33f3f26214beaa7a518a7ea1b33e380a6ee915f0c2cb5"),
	c15c("c15.copy.45c.unsupported-response-header", "sessionruntime/b4_server_request_c15_private.go", "writePrivateB4UnsupportedResponse", "atomic-batch-reserve-before-writer-allocation:header", privateC15SelfBoundV1, "b63a12d3db4d04e473e63d32cbb0a08efa3acefae368720f0674cda6c4a29c99"),
}

func c15c(site, path, function, role string, form privateC15BindingForm, digest string) privateC15SemanticContract {
	return privateC15SemanticContract{privateC15SemanticKey{site, function, role, form}, path, digest}
}

type privateC15Occurrence struct {
	Key       privateC15SemanticKey
	Path      string
	Function  string
	Relation  string
	Index     int
	Digest    string
	Operation ast.Stmt
}

type privateC15Source struct {
	Path  string
	Bytes []byte
}

type privateC15Generation struct {
	Mapping, MappingHash, Comparison       []byte
	Rows, UniqueKeys, Denominator, Changes int
}

type privateC15Verification struct {
	Resolved      []privateC15Occurrence
	RawMarkerUses int
	UniqueKeys    int
	PersistedRows int
}

func privateC15VerifyPersistedMapping(repo string, mapping privateB4C15OperationMapping, contracts []privateC15SemanticContract, sources []privateC15Source) (privateC15Verification, error) {
	const denominator = 44
	if len(contracts) != denominator {
		return privateC15Verification{}, fmt.Errorf("semantic contracts=%d want=%d", len(contracts), denominator)
	}
	if len(mapping.Rows) != denominator || mapping.Counts.Total != denominator {
		return privateC15Verification{}, fmt.Errorf("persisted rows=%d total=%d want=%d", len(mapping.Rows), mapping.Counts.Total, denominator)
	}

	contractsBySite := make(map[string]privateC15SemanticContract, denominator)
	semanticKeys := make(map[privateC15SemanticKey]string, denominator)
	for _, contract := range contracts {
		key := contract.Key
		if key.BindingForm != privateC15MarkerAdjacentV1 && key.BindingForm != privateC15SelfBoundV1 {
			return privateC15Verification{}, fmt.Errorf("unknown binding form site=%s form=%s", key.SiteID, key.BindingForm)
		}
		if key.SiteID == "" || key.FunctionIdentity == "" || key.RoleSelector == "" || contract.Path == "" {
			return privateC15Verification{}, fmt.Errorf("incomplete semantic contract site=%s", key.SiteID)
		}
		if prior, ok := semanticKeys[key]; ok {
			return privateC15Verification{}, fmt.Errorf("duplicate semantic key sites=%s,%s", prior, key.SiteID)
		}
		if _, ok := contractsBySite[key.SiteID]; ok {
			return privateC15Verification{}, fmt.Errorf("duplicate canonical site=%s", key.SiteID)
		}
		semanticKeys[key] = key.SiteID
		contractsBySite[key.SiteID] = contract
	}

	rowsBySite := make(map[string]privateB4C15MappingRow, denominator)
	for _, row := range mapping.Rows {
		if _, ok := rowsBySite[row.SiteID]; ok {
			return privateC15Verification{}, fmt.Errorf("duplicate persisted semantic key site=%s", row.SiteID)
		}
		if _, ok := contractsBySite[row.SiteID]; !ok {
			return privateC15Verification{}, fmt.Errorf("extra persisted row site=%s", row.SiteID)
		}
		rowsBySite[row.SiteID] = row
	}
	for site := range contractsBySite {
		if _, ok := rowsBySite[site]; !ok {
			return privateC15Verification{}, fmt.Errorf("missing persisted row site=%s", site)
		}
	}

	if sources == nil {
		paths := make(map[string]bool)
		for _, contract := range contracts {
			paths[contract.Path] = true
		}
		ordered := make([]string, 0, len(paths))
		for path := range paths {
			ordered = append(ordered, path)
		}
		sort.Strings(ordered)
		for _, path := range ordered {
			body, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(path)))
			if err != nil {
				return privateC15Verification{}, err
			}
			sources = append(sources, privateC15Source{Path: path, Bytes: body})
		}
	}

	occurrences, err := privateC15Discover(sources, contracts)
	if err != nil {
		return privateC15Verification{}, err
	}
	resolved, err := privateC15Resolve(contracts, occurrences, false)
	if err != nil {
		return privateC15Verification{}, err
	}
	if len(resolved) != denominator {
		return privateC15Verification{}, fmt.Errorf("resolved operations=%d want=%d", len(resolved), denominator)
	}

	operationIdentities := make(map[string]string, denominator)
	for _, occurrence := range resolved {
		contract := contractsBySite[occurrence.Key.SiteID]
		row := rowsBySite[occurrence.Key.SiteID]
		identity := occurrence.Path + "|" + occurrence.Function + "|" + contract.Key.RoleSelector + "|" + string(contract.Key.BindingForm)
		if prior, ok := operationIdentities[identity]; ok {
			return privateC15Verification{}, fmt.Errorf("unexplained duplicate operation identity sites=%s,%s identity=%s", prior, occurrence.Key.SiteID, identity)
		}
		operationIdentities[identity] = occurrence.Key.SiteID

		wantRelation, wantIndex := occurrence.Relation, occurrence.Index
		if contract.Key.BindingForm == privateC15SelfBoundV1 {
			wantRelation, wantIndex = "self", 0
		}
		if row.Path != contract.Path || occurrence.Path != contract.Path {
			return privateC15Verification{}, fmt.Errorf("path mismatch site=%s persisted=%s contract=%s resolved=%s", row.SiteID, row.Path, contract.Path, occurrence.Path)
		}
		if row.BindingFunction != contract.Key.FunctionIdentity || occurrence.Function != contract.Key.FunctionIdentity {
			return privateC15Verification{}, fmt.Errorf("function mismatch site=%s persisted=%s semantic=%s resolved=%s", row.SiteID, row.BindingFunction, contract.Key.FunctionIdentity, occurrence.Function)
		}
		if row.OperationKind != privateC15RoleBase(contract.Key.RoleSelector) {
			return privateC15Verification{}, fmt.Errorf("role/call selector mismatch site=%s persisted=%s semantic=%s", row.SiteID, row.OperationKind, contract.Key.RoleSelector)
		}
		if row.AdjacencyRelation != wantRelation || row.AdjacencyIndex != wantIndex || adjacencyRelation(row.AdjacencyIndex) != row.AdjacencyRelation {
			return privateC15Verification{}, fmt.Errorf("binding form/location mismatch site=%s form=%s persisted=%s/%d resolved=%s/%d", row.SiteID, contract.Key.BindingForm, row.AdjacencyRelation, row.AdjacencyIndex, wantRelation, wantIndex)
		}
		if row.ASTDigest != occurrence.Digest {
			return privateC15Verification{}, fmt.Errorf("digest mismatch site=%s persisted=%s resolved=%s", row.SiteID, row.ASTDigest, occurrence.Digest)
		}
	}

	rawUses := 0
	for _, source := range sources {
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, source.Path, source.Bytes, parser.AllErrors)
		if err != nil {
			return privateC15Verification{}, err
		}
		ast.Inspect(file, func(node ast.Node) bool {
			if statementIsC15Binding(nodeAsStatement(node)) {
				rawUses++
			}
			return true
		})
	}
	return privateC15Verification{Resolved: resolved, RawMarkerUses: rawUses, UniqueKeys: len(semanticKeys), PersistedRows: len(rowsBySite)}, nil
}

func nodeAsStatement(node ast.Node) ast.Stmt {
	statement, _ := node.(ast.Stmt)
	return statement
}

func privateC15ValidateContracts(contracts []privateC15SemanticContract, denominator int) error {
	if len(contracts) != denominator {
		return fmt.Errorf("accepted denominator changed: contracts=%d denominator=%d", len(contracts), denominator)
	}
	seen := map[privateC15SemanticKey]string{}
	for _, c := range contracts {
		if c.Key.BindingForm != privateC15MarkerAdjacentV1 && c.Key.BindingForm != privateC15SelfBoundV1 {
			return fmt.Errorf("unknown binding form site=%s form=%s", c.Key.SiteID, c.Key.BindingForm)
		}
		if c.Key.SiteID == "" || c.Key.FunctionIdentity == "" || c.Key.RoleSelector == "" {
			return fmt.Errorf("incomplete semantic key site=%s", c.Key.SiteID)
		}
		if prior := seen[c.Key]; prior != "" {
			return fmt.Errorf("duplicate semantic key sites=%s,%s", prior, c.Key.SiteID)
		}
		seen[c.Key] = c.Key.SiteID
	}
	return nil
}

func privateC15Discover(sources []privateC15Source, contracts []privateC15SemanticContract) ([]privateC15Occurrence, error) {
	if err := privateC15ValidateContracts(contracts, len(contracts)); err != nil {
		return nil, err
	}
	byPath := map[string][]privateC15SemanticContract{}
	for _, c := range contracts {
		byPath[c.Path] = append(byPath[c.Path], c)
	}
	var out []privateC15Occurrence
	for _, source := range sources {
		wanted := byPath[source.Path]
		if len(wanted) == 0 {
			continue
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, source.Path, source.Bytes, parser.AllErrors)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", source.Path, err)
		}
		for _, c := range wanted {
			constName := privateB4C15ConstName(c.Key.SiteID)
			ast.Inspect(file, func(n ast.Node) bool {
				block, ok := n.(*ast.BlockStmt)
				if !ok {
					return true
				}
				for i, stmt := range block.List {
					if c.Key.BindingForm == privateC15SelfBoundV1 {
						if statementIsC15Binding(stmt) || !privateC15NodeHasIdentifier(stmt, constName) {
							continue
						}
						fn := enclosingFunction(file, stmt.Pos())
						if fn != c.Key.FunctionIdentity {
							continue
						}
						out = append(out, privateC15Occurrence{c.Key, source.Path, fn, "self", 0, astDigest(fset, source.Bytes, stmt), stmt})
						continue
					}
					if !privateC15NodeIsExactBinding(stmt, constName) {
						continue
					}
					fn := enclosingFunction(file, stmt.Pos())
					if fn != c.Key.FunctionIdentity {
						continue
					}
					op := privateC15AdjacentOperation(block.List, i)
					if op < 0 {
						continue
					}
					out = append(out, privateC15Occurrence{c.Key, source.Path, fn, adjacencyRelation(op - i), op - i, astDigest(fset, source.Bytes, block.List[op]), block.List[op]})
				}
				return true
			})
		}
	}
	return out, nil
}

func privateC15AdjacentOperation(statements []ast.Stmt, marker int) int {
	for distance := 1; distance < len(statements); distance++ {
		for _, i := range []int{marker - distance, marker + distance} {
			if i >= 0 && i < len(statements) && !statementIsC15Binding(statements[i]) {
				return i
			}
		}
	}
	return -1
}

func privateC15Resolve(contracts []privateC15SemanticContract, occurrences []privateC15Occurrence, verifyDigest bool) ([]privateC15Occurrence, error) {
	matches := map[privateC15SemanticKey][]privateC15Occurrence{}
	for _, o := range occurrences {
		matches[o.Key] = append(matches[o.Key], o)
	}
	resolved := make([]privateC15Occurrence, 0, len(contracts))
	operationIdentity := map[string]string{}
	for _, c := range contracts {
		got := matches[c.Key]
		if len(got) != 1 {
			return nil, fmt.Errorf("semantic key site=%s function=%s role=%s form=%s found=%d", c.Key.SiteID, c.Key.FunctionIdentity, c.Key.RoleSelector, c.Key.BindingForm, len(got))
		}
		o := got[0]
		if verifyDigest && o.Digest != c.FrozenDigest {
			return nil, fmt.Errorf("stale operation digest site=%s old=%s new=%s", c.Key.SiteID, c.FrozenDigest, o.Digest)
		}
		identity := o.Path + "|" + o.Function + "|" + c.Key.RoleSelector + "|" + string(c.Key.BindingForm)
		if prior := operationIdentity[identity]; prior != "" {
			return nil, fmt.Errorf("unexplained duplicate operation identity sites=%s,%s identity=%s", prior, c.Key.SiteID, identity)
		}
		operationIdentity[identity] = c.Key.SiteID
		resolved = append(resolved, o)
	}
	return resolved, nil
}

func privateC15Generate(repo string, contracts []privateC15SemanticContract, old privateB4C15OperationMapping, verifyDigest bool, permutation []int) (privateC15Generation, error) {
	if err := privateC15ValidateContracts(contracts, 44); err != nil {
		return privateC15Generation{}, err
	}
	paths := map[string]bool{}
	for _, c := range contracts {
		paths[c.Path] = true
	}
	ordered := make([]string, 0, len(paths))
	for p := range paths {
		ordered = append(ordered, p)
	}
	sort.Strings(ordered)
	if permutation != nil {
		p := append([]string(nil), ordered...)
		for i, n := range permutation {
			ordered[i] = p[n]
		}
	}
	sources := make([]privateC15Source, 0, len(ordered))
	for _, p := range ordered {
		b, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(p)))
		if err != nil {
			return privateC15Generation{}, err
		}
		sources = append(sources, privateC15Source{p, b})
	}
	occurrences, err := privateC15Discover(sources, contracts)
	if err != nil {
		return privateC15Generation{}, err
	}
	resolved, err := privateC15Resolve(contracts, occurrences, false)
	if err != nil {
		return privateC15Generation{}, err
	}
	oldBySite := map[string]privateB4C15MappingRow{}
	for _, r := range old.Rows {
		oldBySite[r.SiteID] = r
	}
	contractBySite := map[string]privateC15SemanticContract{}
	for _, c := range contracts {
		contractBySite[c.Key.SiteID] = c
	}
	rows := make([]privateB4C15MappingRow, 0, 44)
	type change struct{ SiteID, OldDigest, NewDigest string }
	var changes []change
	for _, o := range resolved {
		c := contractBySite[o.Key.SiteID]
		r, ok := oldBySite[o.Key.SiteID]
		if !ok {
			return privateC15Generation{}, fmt.Errorf("accepted row missing site=%s", o.Key.SiteID)
		}
		if r.ASTDigest != o.Digest {
			if verifyDigest {
				return privateC15Generation{}, fmt.Errorf("stale operation digest site=%s old=%s new=%s", o.Key.SiteID, r.ASTDigest, o.Digest)
			}
			changes = append(changes, change{o.Key.SiteID, r.ASTDigest, o.Digest})
		}
		r.Path = c.Path
		r.BindingFunction = c.Key.FunctionIdentity
		r.AdjacencyRelation = o.Relation
		r.AdjacencyIndex = o.Index
		r.ASTDigest = o.Digest
		rows = append(rows, r)
	}
	if len(rows) != 44 {
		return privateC15Generation{}, fmt.Errorf("generated rows=%d want=44", len(rows))
	}
	mapping := privateB4C15OperationMapping{}
	mapping.Counts = old.Counts
	mapping.Rows = rows
	data, err := json.MarshalIndent(mapping, "", "  ")
	if err != nil {
		return privateC15Generation{}, err
	}
	data = append(data, '\n')
	sum := fmt.Sprintf("%x", sha256.Sum256(data))
	comparison, err := json.MarshalIndent(struct {
		Denominator int      `json:"denominator"`
		Changes     []change `json:"changes"`
	}{44, changes}, "", "  ")
	if err != nil {
		return privateC15Generation{}, err
	}
	comparison = append(comparison, '\n')
	return privateC15Generation{data, []byte(sum + "\n"), comparison, len(rows), len(contractBySite), 44, len(changes)}, nil
}

func privateC15AtomicRefresh(dir string, generation privateC15Generation) error {
	if generation.Rows != 44 || generation.UniqueKeys != 44 || generation.Denominator != 44 {
		return fmt.Errorf("atomic refresh refused rows=%d keys=%d denominator=%d", generation.Rows, generation.UniqueKeys, generation.Denominator)
	}
	files := map[string][]byte{"operation-mapping.json": generation.Mapping, "operation-mapping.json.sha256": generation.MappingHash, "operation-mapping-comparison.json": generation.Comparison}
	stage, err := os.MkdirTemp(dir, ".c15-refresh-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(stage, name), data, 0o644); err != nil {
			return err
		}
	}
	// Commit only after every staged byte exists. Renames are deterministic and confined to one directory.
	names := []string{"operation-mapping.json", "operation-mapping.json.sha256", "operation-mapping-comparison.json"}
	for _, name := range names {
		if err := os.Rename(filepath.Join(stage, name), filepath.Join(dir, name)); err != nil {
			return err
		}
	}
	return nil
}

func privateC15GenerationEqual(a, b privateC15Generation) bool {
	return bytes.Equal(a.Mapping, b.Mapping) && bytes.Equal(a.MappingHash, b.MappingHash) && bytes.Equal(a.Comparison, b.Comparison)
}
func privateC15RoleBase(role string) string {
	if i := strings.IndexByte(role, ':'); i >= 0 {
		return role[:i]
	}
	return role
}
