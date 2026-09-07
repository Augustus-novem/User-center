# Benchmark: M07 singleflight ON/OFF concurrent origin comparison

Date: 2026-09-08
Commit: `841bd6e`
Machine: Windows/amd64, Intel Core i5-12490F, 16 GB RAM
Go version: `go1.25.1`

## Dataset

- 64 goroutines request the same note ID from a cold local cache and a Redis miss stub.
- One origin load executes two DAO queries: note row plus note images.
- The controlled DAO harness has 8 concurrent query slots and 5 ms service time per query.
- Both modes run the production `CachedNoteRepository` code; only its singleflight switch changes.

## Command

```powershell
go test ./internal/repository -run '^TestNoteCacheSingleflightComparison$' -v -count=1
```

## Configuration

| Setting | Value |
|---|---:|
| Concurrent callers | 64 |
| Requested note IDs | 1 shared ID |
| DAO concurrent slots | 8 |
| Simulated time per DAO query | 5 ms |
| Cache state | cold local + Redis miss |

## Result

| Mode | Origin loads | DB queries | Wall-clock |
|---|---:|---:|---:|
| singleflight OFF | 64 | 128 | 90.1346 ms |
| singleflight ON | 1 | 2 | 12.0162 ms |

## Raw output

```text
=== RUN   TestNoteCacheSingleflightComparison
    note_cache_comparison_test.go:56: singleflight=OFF callers=64 origin_loads=64 db_queries=128 elapsed=90.1346ms
    note_cache_comparison_test.go:56: singleflight=ON callers=64 origin_loads=1 db_queries=2 elapsed=12.0162ms
--- PASS: TestNoteCacheSingleflightComparison (0.10s)
PASS
ok      user-center/internal/repository 0.172s
```

## Interpretation

For one simultaneous hot-key burst, enabling singleflight reduced repository origin loads from 64 to 1 and counted DAO queries from 128 to 2. The observed wall-clock time fell from about 90 ms to about 12 ms in this controlled harness.

## Caveats

- This is a real concurrent execution of repository code, but the DAO timing and connection-pool limit are controlled test doubles, not a production MySQL deployment.
- The numbers must not be presented as API QPS, P95, P99, or production latency.
- singleflight only coalesces requests inside one process. Multiple user-center replicas can still each perform one origin load.
- Scheduler load affects elapsed time; query counts are the deterministic comparison metric.
