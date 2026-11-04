package programcrepresentative

// This declaration-only file is excluded from its own later reviewed seam
// manifest. Binding-only revisions may supply real reviewed pins/provenance;
// they must not change verifier or admission semantics.
func BuildStage6ImplementationV2(maxSourceBytes uint64) (Stage6ImplementationDTO, error) {
	return Stage6ImplementationDTO{
		RepositoryCommit: "f06e6fb2d2c38aca375bf7d8e622e6cb326272d1",
		RepositoryTree:   "abc78c0003d197d5fdcb003d76754968d3511d10",
		Files: []ImplementationFileDTO{
			{Path: "internal/censuscontinuation/multiseed_checkpoint_reconstruction_v2.go", GitBlobSHA1: "58fc8ee5ecf044aabe67536be58b4f18a326931a", SHA256: "sha256:284c9486a322165c21a059c7535cc22026614d176452113c9e13ca80f7c2d517", ByteLength: 5484},
			{Path: "internal/censuscontinuation/multiseed_qualification_evidence_v2.go", GitBlobSHA1: "03ff57471ae12649896820650cda40b8e57c3d17", SHA256: "sha256:66f339a6f43ceb9d46500bee40d07bfb2cedbd7c30d83d3643a4565336deff23", ByteLength: 7372},
			{Path: "internal/censuscontinuation/multiseed_qualification_owner_v2.go", GitBlobSHA1: "849397ab8c12990beb5131f4c7484e283d022ef9", SHA256: "sha256:41b4574f20a841cd5e84891ab9575ae791cd316a5b5db77c6e1c447f672d5aaf", ByteLength: 8416},
			{Path: "internal/inventorysuccessor/multiseed_checkpoint_operations_v2.go", GitBlobSHA1: "ef787205237589752a44e0c70ce0f143f11da5e2", SHA256: "sha256:e9b318f75e986ddf44c1c7f551f1a55eb4a02a78d4b8cf655bd8a94499227867", ByteLength: 16901},
			{Path: "internal/inventorysuccessor/multiseed_gap_ledger_v2.go", GitBlobSHA1: "9f48fbbee277f128d17a397f3bf33f05d2c96390", SHA256: "sha256:72a3b5c416d737427951fd4880b369049bf6e92e001bed07d390bfd3c18cf81e", ByteLength: 2752},
			{Path: "internal/inventorysuccessor/multiseed_heads_custody_v2.go", GitBlobSHA1: "d75fef9a87a755e2141d10a19ab86d9a814cd00d", SHA256: "sha256:d35de70d52126db634f0df2ea777fcfd3daae2cd33703cc9fd02d9e797844514", ByteLength: 8603},
			{Path: "internal/inventorysuccessor/multiseed_objects_custody_v2.go", GitBlobSHA1: "90c133f4d1a878d53d566fdcb9d20991acd44e40", SHA256: "sha256:17dd3f27d03910a9893268a7ba848d1c9553c6aceceae5ab259355889cc6a349", ByteLength: 6778},
			{Path: "internal/inventorysuccessor/multiseed_workflow_identity_v2.go", GitBlobSHA1: "179eeb73a4b282aa926119e959214500e04f6ea0", SHA256: "sha256:94a05c3c056ec6f4537dea0e6a751da4708b3dd955098674a77190df872394e2", ByteLength: 1588},
			{Path: "internal/inventoryworkflow/multiseed_checkpoint_v2.go", GitBlobSHA1: "7c0400f1ea5e68008f177a02ae161bd886a94f66", SHA256: "sha256:7aee8d183f9e5c9de85469c247229447941d35cc07f6d99220507c53047082a8", ByteLength: 4993},
			{Path: "internal/inventoryworkflow/multiseed_qualification_subject_v2.go", GitBlobSHA1: "849d568e065faa8e3ab162bf54cf8f0a06576b22", SHA256: "sha256:59db1a4c2c9334218f7d28a429563715d7419cb00838bcc699085e86dda8a1db", ByteLength: 2868},
			{Path: "internal/inventoryworkflow/multiseed_review_harness_v2.go", GitBlobSHA1: "02eb8b5f3c677df8935e3b9eaf8da8be999411fa", SHA256: "sha256:b9681c9f5349bcae3a2dd0fa7f3b4ffb67b174adabb6208a16b5a33df6dae1e0", ByteLength: 189},
			{Path: "internal/programcrepresentative/multiseed_canonical_v2.go", GitBlobSHA1: "175601240170100ee5fda7187ea254133692ae53", SHA256: "sha256:4c9075ea9ba814821881de85b6f4e68481c0976e847cc032e124fcb7aa161b1d", ByteLength: 2373},
			{Path: "internal/programcrepresentative/multiseed_checkpoint_dto_v2.go", GitBlobSHA1: "9bf635c36f81141ef8b866c364b74aa6b3837cd7", SHA256: "sha256:63dc13da746b38e7c3f6f82bfa3e32865bb89a0dea16c1a08895d98389780794", ByteLength: 11798},
			{Path: "internal/programcrepresentative/multiseed_checkpoint_reopen_v2.go", GitBlobSHA1: "76cb0f664acc283e75b7e5f34dcf88008fced7fc", SHA256: "sha256:4ef83530a1bde67e589d32e4a29469e5052f8dfef2ec0ab942ecb7a869feb916", ByteLength: 5284},
			{Path: "internal/programcrepresentative/multiseed_codec_v2.go", GitBlobSHA1: "f5e18915b1167d26bbb36209985455df2c059ff0", SHA256: "sha256:3acd1052504508ae31287bc81d89209e22728182b43e29f687f936de8969b539", ByteLength: 1968},
			{Path: "internal/programcrepresentative/multiseed_community_size_v2.go", GitBlobSHA1: "98d3cc24ff3fd09c49faf8802aac3c53b7cd537e", SHA256: "sha256:2e21742e696d761fc55b18ecdf72f49f9ef408cd2abef3caea7037bb0209e122", ByteLength: 2162},
			{Path: "internal/programcrepresentative/multiseed_custody_binding_v2.go", GitBlobSHA1: "0659467dd9f8299fc5cd0472e42e7a7bb63fa74f", SHA256: "sha256:bc2bc9d53122c10afb602d93bc57ce8838424190ce2e9441cb5c83a53026f40d", ByteLength: 1436},
			{Path: "internal/programcrepresentative/multiseed_denominator_lineage_v2.go", GitBlobSHA1: "17313e5e6b6cec0ed00204126d8aa9c17cd721de", SHA256: "sha256:98666a5459c8299cb3d9a1ad7d079f383ff3526d96908b5fd170680a3966e9d7", ByteLength: 13129},
			{Path: "internal/programcrepresentative/multiseed_finalization_dto_v2.go", GitBlobSHA1: "aaec3742978edca88d95183b6c114793a773143f", SHA256: "sha256:7cc20c51ad29dba1a19f1ebc1f02100ba2e9c513579763e1bf09b87d5cfc806b", ByteLength: 8134},
			{Path: "internal/programcrepresentative/multiseed_fixed_artifacts_v2.go", GitBlobSHA1: "b7d23bade7c475bcd17179b30238704453c674a7", SHA256: "sha256:8f1f045df293c0a53ab90e7a49cabaf84fc83c1302e4c7ba7494eed599dbc992", ByteLength: 5108},
			{Path: "internal/programcrepresentative/multiseed_group_finalization_v2.go", GitBlobSHA1: "78ecf710348a12611d87c270ae351ae1ee091ffc", SHA256: "sha256:968f0fdb31209aa6070c72ccbbdcbc7c999892425abed894125fc23a180df681", ByteLength: 4989},
			{Path: "internal/programcrepresentative/multiseed_identified_codecs_v2.go", GitBlobSHA1: "d7538636abe2cb956a35377fe54c91f9e0e9e947", SHA256: "sha256:bca79f9886675286e7af49122308ea64808b0e931c614eb3cf6c2ab955959cdb", ByteLength: 10545},
			{Path: "internal/programcrepresentative/multiseed_output_admission_v2.go", GitBlobSHA1: "ff76058c1d2bcfde1da2ea9772dfe1ad5fba3be7", SHA256: "sha256:6db0ce481aec1e781c16ad93e8a58881628f914afc129028b32230ef1004631d", ByteLength: 4742},
			{Path: "internal/programcrepresentative/multiseed_qualification_envelope_v2.go", GitBlobSHA1: "6237df277abf21d68caba0c068ee6af2a070baff", SHA256: "sha256:431d5c788bbc5b8fd96045674302e1c1e7148a4fda9aca5b9acc588dc0e3b196", ByteLength: 7857},
			{Path: "internal/programcrepresentative/multiseed_qualification_subject_v2.go", GitBlobSHA1: "a62535b92e69d4364fc214ce71f836159a969525", SHA256: "sha256:0b1081dfcf245781db61258c91647a42a154883b50c3cb159732b362c52853f9", ByteLength: 16152},
			{Path: "internal/programcrepresentative/multiseed_qualification_verifier_v2.go", GitBlobSHA1: "7c45311f84092d9eae3ff7a87e64d754ad39f24d", SHA256: "sha256:9bf938310571cb34a739918f0a011e393a5a0dab11cc4094eae53cd0a3ceaaaf", ByteLength: 7058},
			{Path: "internal/programcrepresentative/multiseed_receipt_inventory_v2.go", GitBlobSHA1: "1a88d1d9d1e7994ff8e2ee76031a25c5551f50e0", SHA256: "sha256:eebfd8b11799c8305ec739df0878ce1ffaf001263b40c1ab5825e85df67f642b", ByteLength: 1328},
			{Path: "internal/programcrepresentative/multiseed_result_v2.go", GitBlobSHA1: "6cf1703e7cc689106d24c742c6edb77dfac0031a", SHA256: "sha256:d89de84e00b4afdb8b06f213ae745d6b0f2a833a689eb30520af35cd14851f98", ByteLength: 10757},
			{Path: "internal/programcrepresentative/multiseed_successor_wire_v2.go", GitBlobSHA1: "15ca59d36c9f0abc4df3e91e2ef3afc06ac01195", SHA256: "sha256:70dee8d28871f71de271b321544f52237032a8e101f97474609bab5b6de9ada5", ByteLength: 4638},
			{Path: "internal/programcrepresentative/multiseed_typed_size_v2.go", GitBlobSHA1: "2a7e17dec749e48895e9edba8a1833ba8275e2e0", SHA256: "sha256:09db7ab5500e3f447722da318d6bd7d8faba053e8227dfa9bfb7a966daca2751", ByteLength: 52166},
		},
		Identity: "sha256:19481e7d36682e3d4c95a52121235f1139164e84f3221a20b59e04f6499a5b41",
	}, nil
}

// ReviewedHarnessActorBindingV2 is non-wire repository-owned provenance data.
// Actor.Projection must be nil; Projection binds the exact native harness
// assignment/terminal projection independently inspected during real review.
// Caller review DTOs and synthetic actors cannot populate this declaration.
type ReviewedHarnessActorBindingV2 struct {
	Actor      RecordedReviewActorV2
	Projection ArtifactBindingDTO
}

// ReviewedHarnessActorBindingsV2 binds the actual subject writer and isolated
// subject reviewer inspected in native PROJECTION-DERIVATION.json. These raw
// two-record projection pins qualify only that exact subject/scope; they are not
// caller registrations, test actors, or an application-capture grant.
func ReviewedHarnessActorBindingsV2() ([]ReviewedHarnessActorBindingV2, error) {
	return []ReviewedHarnessActorBindingV2{
		{
			Actor:      RecordedReviewActorV2{Kind: "PI_SESSION", ID: "01a1070d-1392-7797-8214-2c7836f6d572", IsolationKind: "PI_SESSION"},
			Projection: ArtifactBindingDTO{SchemaVersion: "lsp-trace.program-c-representative-review-transcript-jsonl.private.v2", SHA256: "sha256:aaa334faa9232c1a92ac8f17209ac22ed8729025834a92508d0109e0e06064e6", ByteLength: 1755},
		},
		{
			Actor:      RecordedReviewActorV2{Kind: "PI_ISOLATED_TASK", ID: "cf12200b-22aa-40e", ParentID: "01a1070d-1392-7797-8214-2c7836f6d572", IsolationKind: "PI_ISOLATED_TASK"},
			Projection: ArtifactBindingDTO{SchemaVersion: "lsp-trace.program-c-representative-review-transcript-jsonl.private.v2", SHA256: "sha256:63e6b0da3ebd89bab3602360846f2f46a65f12cced6ce480eabf6b4e0cbff2dd", ByteLength: 1791},
		},
	}, nil
}
