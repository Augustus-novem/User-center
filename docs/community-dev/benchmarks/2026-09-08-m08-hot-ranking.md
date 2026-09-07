# Benchmark: M08 Redis hot ranking snapshot

Date: 2026-09-08
Commit: `f6b1492`
Machine: Windows/amd64, Intel Core i5-12490F, 16 GB RAM
Go version: `go1.25.1`

## Dataset

- Real Redis at the repository integration boundary.
- 60 minute buckets.
- 100 note members per bucket.
- Top 20 requested.
- 20 measured iterations per case.
- UUID-isolated benchmark key prefix with cleanup.

## Command

```powershell
go test -tags=e2e ./internal/repository/cache -run '^$' -bench '^BenchmarkRedisHotRankSnapshot_e2e$' -benchtime=20x -count=1
```

## Result

| Case | Iterations | ns/op |
|---|---:|---:|
| cold snapshot + Top 20 | 20 | 1,092,510 |
| materialized snapshot Top 20 | 20 | 514,605 |

## Raw output

```text
goos: windows
goarch: amd64
pkg: user-center/internal/repository/cache
cpu: 12th Gen Intel(R) Core(TM) i5-12490F
BenchmarkRedisHotRankSnapshot_e2e/cold_snapshot_top20-12               20   1092510 ns/op
BenchmarkRedisHotRankSnapshot_e2e/materialized_top20-12                20    514605 ns/op
PASS
ok      user-center/internal/repository/cache 0.190s
```

## Interpretation

On this local dataset, reading an existing materialized snapshot avoided the 60-bucket union performed by a cold snapshot. The result supports snapshot reuse but does not establish API QPS or production latency percentiles.

## Caveats

- Redis ran locally; network and production contention are absent.
- The dataset has only 100 distinct note IDs repeated across buckets.
- `ns/op` includes Redis round trips at the cache boundary, not Gin, authentication, or JSON response time.
- Twenty iterations are evidence for this milestone, not a capacity claim.
