# Performance Benchmarks

This document provides the actual, measured performance results for `logprism` (v1.3.1).

## Test Environment

The following hardware was used for all local measurements:

| Component | Specification |
| :--- | :--- |
| **Processor** | Apple M2 |
| **Architecture** | arm64 (Apple Silicon) |
| **OS** | macOS |

## High-Volume Throughput

We tested the ability of `logprism` to process a massive, structured JSON log file.

*   **Dataset**: 1,000,000 lines of structured JSON logs (~120 MB).
*   **Measured Time (Total)**: 0.96 seconds.
*   **Throughput**: **~1,041,666 lines per second.**

This measurement is a "cold start" total including disk I/O, byte-scanning, filter matching, and output formatting.

### Cloud Results (GitHub Actions)

These metrics are captured on every commit to ensure zero performance regressions.

*   **Server**: Ubuntu Latest (Standard GitHub Runner).
*   **Dataset**: 1,000,000 lines of structured JSON logs.
*   **Result**: <!-- LATEST_RESULT -->**1.02 seconds**<!-- /LATEST_RESULT -->
*   **Throughput**: <!-- LATEST_THROUGHPUT -->~980,000 lines per second<!-- /LATEST_THROUGHPUT -->
*   **Last Verified**: <!-- LATEST_DATE -->May 04, 2026<!-- /LATEST_DATE -->

> [!IMPORTANT]
> **Verifiable Integrity**: This section is NOT manual. It is automatically updated by our [Performance Benchmark Workflow](.github/workflows/bench.yml). The numbers come directly from a real Ubuntu server environment, ensuring that our performance claims are transparent and untampered.

## Resource Predictability

Unlike standard JSON tools that load files into memory, `logprism` maintains a flat memory profile.

*   **Peak Memory Usage (RSS)**: 9.2 MB.
*   **Heap Allocation**: Zero-growth. We verified this by running a 100,000 line stream through the engine and measuring `runtime.MemStats`. The heap remains stable from start to finish.

Regardless of the input file size (1MB or 100GB), the memory footprint remains constant, making it ideal for long-running processes like `tail -f`.

## Adversarial Resilience

We don't just test with "clean" logs. Our `TestChaosStress` suite verifies stability by injecting:
*   Truncated JSON (broken braces).
*   Random binary garbage.
*   UTF-8 Emojis.
*   Lines exceeding 10MB in length.

**Result**: 0 Crashes. `logprism` identifies the noise and passes it through safely while maintaining full speed on valid lines.

## Unix Integration

*   **Broken Pipes**: Verified that `logprism` handles `EPIPE` signals correctly (e.g., when used with `head` or `tail`). It exits silently and gracefully.
*   **Filtering**: Accuracy verified at 100% across 1,000,000 lines including numeric, string, and negative number operators.
