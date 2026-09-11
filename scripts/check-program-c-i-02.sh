#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$root"

pass=0
fail=0
blocked=0

test_group() {
	id=$1
	shift
	if "$@" >/dev/null 2>&1; then
		printf '%s result=PASS\n' "$id"
		pass=$((pass + 1))
	else
		printf '%s result=FAIL\n' "$id"
		fail=$((fail + 1))
	fi
}

for fixture in \
	internal/retainedcalls/testdata/frozen-v1-export.json \
	cmd/lsp-trace-mcp/testdata/d01-program-b-normative-graph.json
 do
	if [ ! -f "$fixture" ] || [ -L "$fixture" ]; then
		printf 'ASSERT_I02_REPOSITORY_FIXTURE_%s result=FAIL\n' "$(basename "$fixture" | tr '.-' '__')"
		fail=$((fail + 1))
	else
		printf 'ASSERT_I02_REPOSITORY_FIXTURE_%s result=PASS\n' "$(basename "$fixture" | tr '.-' '__')"
		pass=$((pass + 1))
	fi
 done

test_group ASSERT_I02_FROZEN_FIXTURE_CLI_MCP_PARITY \
	go test ./cmd/lsp-trace-mcp -run '^TestProgramCI02FrozenFixtureParity$' -count=1

test_group ASSERT_I02_PROJECTION_COMPONENTS_ORACLES \
	go test ./internal/boundedanalysis -run '^(TestAllThreeNodeDirectedTopologiesIndependentFloydOracle|TestParallelGroupsAndLexicalTie)$' -count=1

test_group ASSERT_I02_METRICS_ORACLES \
	go test ./internal/boundedmetrics -run '^(TestMetricsAllThreeNodeTopologies|TestMetricsEmptyIsolateLoopsParallelTwoSitesDense|TestMetricsAdmittedFixturesAndResealedMutations)$' -count=1

test_group ASSERT_I02_PAGERANK_PPR_ORACLES \
	go test ./internal/boundedranking -run '^(TestRankingRationalAllThreeNodeGraphs|TestRankingFixedVectorsParallelScalingPermutation)$' -count=1

test_group ASSERT_I02_DETERMINISTIC_REPLAY \
	go test ./internal/boundedranking ./internal/normativeanalytics -run '^(TestRankingCancellationBoundaryReplay|TestResultValidationClosedReplayAndNestedMutations)$' -count=1

test_group ASSERT_I02_RESOURCE_BOUNDS \
	go test ./internal/boundedanalysis ./internal/boundedmetrics ./internal/boundedranking ./internal/normativeanalytics -run '^(TestEmptyLimitsAndCancellation|TestStrictInputIdentityAndLimits|TestMetricsBoundsCancellationAndEncodedPreflight|TestRankingWorkIterationCancel|TestRankingMaximumTopologyBudgetIsHonest|TestIncrementalWorkAccountingExactBoundaries|TestEarlyStopInstrumentationAndLimitDigestPolicy)$' -count=1

test_group ASSERT_I02_PUBLIC_V2_PROCESS_PARITY \
	go test ./cmd/lsp-trace-mcp -run '^TestPublicAnalyticsV2ProcessExactParity$' -count=1

printf 'I_02 PASS=%d FAIL=%d BLOCKED=%d\n' "$pass" "$fail" "$blocked"
[ "$fail" -eq 0 ] && [ "$blocked" -eq 0 ]
