# Performance Benchmarks

This document provides the actual, measured performance results for `logprism` (v1.2.1).

## Test Environment

The following hardware was used for all measurements:

| Component | Specification |
| :--- | :--- |
| **Processor** | Apple M2 |
| **Architecture** | arm64 |
| **OS** | macOS |

## High-Volume Throughput

We tested the ability of `logprism` to process a massive, structured JSON log file.

*   **Dataset**: 1,000,000 lines of structured JSON logs (~120 MB).
*   **Total Time**: 0.95 seconds.
*   **Result**: **~1,052,000 lines per second.**

This measurement includes the full cycle: reading from disk, parsing the JSON fields, matching filters, and writing the formatted output to the stream.

### Cloud Results (GitHub Actions)

We verify performance on every commit using **Ubuntu Latest** (GitHub Runners).

*   **Dataset**: 1,000,000 lines of structured JSON logs.
*   **Latest Result**: **<!-- LATEST_RESULT -->~1.3 seconds<!-- /LATEST_RESULT -->**
*   **Last Verified**: <!-- LATEST_DATE -->May 03, 2026<!-- /LATEST_DATE -->

*This section is automatically updated by our [Performance Benchmark workflow](.github/workflows/bench.yml).*

## Memory Efficiency

`logprism` is designed for a "flat" memory footprint. It does not store the entire log file in RAM; it processes logs line-by-line using a specialized byte-scanner.

*   **Peak Memory Usage (RSS)**: 9.2 MB.
*   **Average RAM per line**: Near-zero.

Regardless of the input file size (1MB or 100GB), the memory footprint remains constant, making it ideal for long-running processes like `tail -f`.

## Reliability & Resilience

We conducted a "Stress Test" using a 100MB dataset composed of 50% perfect JSON, 20% malformed/broken JSON, 20% plain text, and 10% random binary garbage.

*   **Result**: 0 Crashes / 0 Panics.
*   **Behavior**: `logprism` correctly identifies valid JSON lines for formatting and safely passes through non-JSON text or garbage without disruption.

## Unix Integration

*   **Broken Pipes**: Verified that `logprism` handles `EPIPE` signals correctly (e.g., when used with `head` or `tail`). It exits silently and gracefully.
*   **Filtering**: Accuracy verified at 100% across 1,000,000 lines.
