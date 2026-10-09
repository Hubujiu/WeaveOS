# Personal candidate index plan and RED

Source data design: https://app.notion.com/p/3f42f5a9e64881a0827df35e73aef4db (created and read back 2026-10-09 12:53 UTC).
Actual original index readiness RED: ../inbox-index-red-2 tests, no rows for required personal index. Synthetic comparison passed after correcting only test timestamp literal to explicit UTC (original local-time fixture mismatch retained in ../inbox-index-red). No production index exists yet.
100k synthetic candidate rows, 100 per actor, 85 unclosed for selected actor, 20-row page: existing app-leading index used 75 index searches + sort /310 local blocks, execution0.173ms; actor-leading candidate1 index search/no sort/23 blocks,0.032ms. New index5,070,848bytes. Both exact independent expected timestamp sequences equal. One warm sample, not statistical throughput or full authorization benchmark. No planner setting forced. New index adds real storage/write maintenance and cannot eliminate per-task authorization scan.
Node registration test RED: missing manifest entry, before migration/manifest changes. Raw original output retained.
