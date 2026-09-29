# Tracking manager scalability benchmarks

`BenchmarkScaleBurst` and `BenchmarkScalePacedFanout` in `internal/tracking/scale_bench_test.go` exercise the tracking manager directly. They do not include HTTP, JSON, WebSocket I/O, or a browser.

```sh
GOMAXPROCS=8 go test ./internal/tracking -run '^$' -bench '^BenchmarkScale' -benchtime=200ms -count=3 -benchmem
```

The burst benchmark varies the number of activities, concurrent producers, and viewers per activity. Producers send as fast as possible. A full viewer buffer causes the documented disconnect policy, so compare `delivered_pct` alongside throughput; a low value means the run stopped measuring sustained fanout. The paced benchmark uses one producer per activity and waits for every viewer to read each point before sending the next one. This keeps viewers connected and measures sustained delivery, including the benchmark's acknowledgement overhead.

Both benchmarks use equal valid timestamps so scheduling cannot cause out-of-order rejections. `samples/s` measures accepted points, and `p95_ns` estimates call latency from one in 128 operations per producer. In the paced benchmark this includes waiting for viewer acknowledgements. `B/op` and `allocs/op` come from Go's benchmark framework. `live_goroutines` is the increase after creating activities and benchmark viewers; it excludes producer goroutines. `retained_B` is an indicative GC heap delta while the manager still owns the samples, and is comparable only when the operation count is fixed.

## Initial comparison

These are medians of three runs on an Apple M1 Pro, Go 1.27.1, macOS arm64, `GOMAXPROCS=8`, with `-benchtime=200ms`. The baseline is `main` at `ffdd31f`, run with the same benchmark file. The PR column is the per-activity runtime implementation. Values are local measurements, not service-level capacity claims.

| Scenario | `main` samples/s | PR samples/s | `main` p95 | PR p95 |
| --- | ---: | ---: | ---: | ---: |
| Burst: 1 activity, 8 producers, 0 viewers | 4.92M | 1.02M | 3.6 µs | 28.6 µs |
| Burst: 100 activities, 8 producers, 0 viewers | 3.81M | 2.12M | 9.5 µs | 4.9 µs |
| Burst: 1,000 activities, 8 producers, 0 viewers | 2.90M | 1.62M | 14.4 µs | 4.4 µs |
| Paced: 1 activity, 1 viewer | 1.64M | 0.72M | 1.2 µs | 3.5 µs |
| Paced: 10 activities, 4 viewers each | 0.45M | 1.00M | 72.7 µs | 20.2 µs |
| Paced: 100 activities, 4 viewers each | 0.50M | 0.98M | 658 µs | 264 µs |

The per-activity runtime has lower tail latency under high activity fanout, but less raw ingestion throughput and higher overhead for small workloads. The PR also copies input and outgoing payloads to enforce state ownership, while `main` does not; the comparison therefore measures the full implementations rather than only the lock-versus-channel choice.

At a fixed 100,000 samples across 1,000 activities with no viewers, the baseline retained about 6.2 MB and added no goroutines; the PR retained about 8.9 MB and added 1,000 runtime goroutines. These numbers are approximate heap deltas, not a replacement for a heap profile. To compare memory at the same operation count, use a fixed benchmark length:

```sh
GOMAXPROCS=8 go test ./internal/tracking -run '^$' -bench 'BenchmarkScaleBurst/activities=1000/producers=8/viewers=0$' -benchtime=100000x -count=1 -benchmem
```

Run the benchmark on the same machine and Go version for branch comparisons. A separate end-to-end load test is needed to measure the HTTP/WebSocket service and real client behavior.
