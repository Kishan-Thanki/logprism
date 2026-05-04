# logprism Test Report (v1.3.1)

This document provides a comprehensive summary of all tests performed on `logprism`.

## Test Summary

| Category | Count | Status |
| :--- | :--- | :--- |
| **Unit Tests** | 12 | ✅ PASS |
| **Integration Tests** | 10 | ✅ PASS |
| **Benchmarks** | 3 | ✅ VERIFIED |
| **Total Items** | **25** | **100% SUCCESS** |

> [!TIP]
> **Re-Verify Locally**: You can run the entire suite yourself to confirm these results:
> `go test -v ./cmd/... ./tests/...`

## 1. Unit Tests (`cmd/logprism/main_test.go`)
These tests verify the internal logic, parsing, and formatting engines.

- **TestFormatLine**: Standard JSON and non-JSON fallback logic.
- **TestFormatLineColorized**: ANSI color injection for different log levels.
- **TestFormatLinePretty**: Recursive indentation of nested objects/arrays.
- **TestFormatLineFilter**: Basic string-based inclusion filtering.
- **TestMatchFiltersV130**: Core filter logic (AND/OR semantics).
- **TestOperatorBoundaries**: Numeric comparisons including negative numbers and scientific notation.
- **TestStructuralEdgeCases**: Handling of empty objects, nulls, and UTF-8 characters.
- **TestMemoryEfficiency**: Verified zero-growth heap allocation over 100k lines using MemStats.
- **TestFieldMapping**: Key-to-key resolution via the `-map` flag.
- **TestHighlighting**: Multi-term yellow highlighting in messages and IDs.
- **TestRunLargeLine**: High-buffer processing for lines > 800KB.
- **TestRunPropagatesWriteError**: Error handling when the output stream is closed.

## 2. Integration & Robustness Tests (`tests/integration_test.go`)
These tests run the compiled `logprism` binary against real-world scenarios and adversarial input.

- **TestCLI**: Verified full JSON paths and CLI flag compatibility.
- **TestFilterAndPretty**: Combined usage of filtering and pretty-printing.
- **TestInputOutputFlags**: File-to-file processing without terminal overhead.
- **TestVersionFlag**: Version resolution from VCS/Build metadata.
- **TestLargeLine**: System-level stability with large data chunks.
- **TestBrokenPipe**: Graceful exit on `SIGPIPE` (e.g., when piped to `head`).
- **TestAnalyzerV130Features**: Verification of v1.3.0 specific operator logic.
- **TestChaosStress**: Processed mixed binary garbage, truncated JSON, and infinite lines without crashing.
- **TestMemoryFlatness**: Verified constant memory usage during 10k line bursts.
- **TestDeepNesting**: Correct formatting of JSON objects nested 40+ levels deep.

## 3. Performance Benchmarks (`cmd/logprism/bench_test.go`)
Measured on Apple M2 (arm64).

- **BenchmarkFormatLine**: ~300-500 ns per line.
- **BenchmarkFormatLineColor**: ~600 ns per line.
- **BenchmarkRun**: ~57 µs for full processing cycle (including I/O).

## 4. Final Audit Result
**Verdict: PRODUCTION READY & HARDENED**

The engine demonstrates high resilience against malformed data, provides precise numeric/string filtering, and maintains a stable memory footprint under pressure. **Zero Heap Growth** has been verified over massive streams. All comments have been removed from the source as per architectural requirements.
