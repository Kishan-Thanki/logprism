# logprism

**The Universal, High-Speed JSON Log Formatter, Analyzer & Observer.**

[![Build Status](https://github.com/Kishan-Thanki/logprism/actions/workflows/ci.yml/badge.svg)](https://github.com/Kishan-Thanki/logprism/actions/workflows/ci.yml)
[![Performance Benchmark](https://github.com/Kishan-Thanki/logprism/actions/workflows/bench.yml/badge.svg)](https://github.com/Kishan-Thanki/logprism/actions/workflows/bench.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/Kishan-Thanki/logprism)](https://goreportcard.com/report/github.com/Kishan-Thanki/logprism)
[![Latest Release](https://img.shields.io/github/v/release/Kishan-Thanki/logprism)](https://github.com/Kishan-Thanki/logprism/releases)
[![License: Apache 2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![Go Reference](https://pkg.go.dev/badge/github.com/Kishan-Thanki/logprism.svg)](https://pkg.go.dev/github.com/Kishan-Thanki/logprism)

`logprism` is a "bare-metal" CLI utility designed to **format, analyze, and observe** structured JSON logs with extreme efficiency. While written in Go, it is a **universal tool** that provides deep visibility into logs from any language or framework (Java, Python, Rust, Node.js, C#, etc.).

<p align="center">
  <img src="assets/demo.gif" alt="logprism animated demo" width="720">
</p>

## Performance

`logprism` is built for extreme speed, processing over **1 million lines per second** on standard hardware. By using a high-efficiency byte-scanner and a manual state machine, it processes log streams with ultra-low latency and a **stable, near-zero memory growth** footprint.

It is designed to handle massive, high-throughput production log streams without slowing down your data pipeline or exhausting system resources. Whether you are processing 1MB or 100GB, the resource profile remains constant and predictable.

### Continuous Benchmarking
Every commit is automatically benchmarked on a neutral Linux environment via **GitHub Actions** to ensure zero performance regressions.

**See the [Full Performance Benchmarks](docs/PERFORMANCE.md) for detailed measurements on Apple M2.**

## Development & Reliability
To ensure `logprism` is production-ready, we maintain a comprehensive suite of unit, integration, and stress tests.

- **[Full Test Report](docs/TEST_REPORT.md)**: Latest test results and coverage details.
- **[Roadmap](docs/ROADMAP.md)**: Future architectural goals and feature milestones.

## Installation

### via Go (Recommended)
```sh
go install github.com/Kishan-Thanki/logprism/cmd/logprism@latest
```

### Manual
`logprism` is a single, self-contained binary with **zero external dependencies**. Simply download the binary for your OS and move it to your `/usr/local/bin`.

## Usage

### The Standard Pipe
Pipe any JSON log stream directly into `logprism`:
```sh
# Kubernetes Logs
kubectl logs -f my-pod | logprism

# Docker Logs
docker logs -f my-container | logprism

# Local application output
./my-app-binary | logprism
```

### Real-World Scenarios
High-performance examples for production analysis:

#### 1. Debugging Outages (The "Story")
Find an error and see exactly what happened 10 lines before and after it:
```sh
tail -f app.log | logprism -filter level=ERROR -C 10
```

#### 2. Monitoring API Health
Instantly isolate server-side failures (500 errors):
```sh
tail -f access.log | logprism -filter "status>=500"
```

#### 3. High-Traffic Observability
Monitor a high-velocity stream without crashing your terminal by sampling only 5% of traffic:
```sh
tail -f traffic.log | logprism -sample 5
```

#### 4. Multi-Service Investigation
Filter logs for a specific microservice and highlight trace IDs to follow a request:
```sh
cat cloud-logs.json | logprism -filter service=auth-api -highlight "req_882f"
```

### Direct File Input
```sh
logprism -input production.log -output readable.log
```

## Features

### Analysis Engine
Perform complex logic on your logs directly in the terminal without complex queries.
*   **Numeric Comparisons**: Filter by status codes or metrics (e.g., `logprism -filter "status>400"`).
*   **Exclusion Logic**: Hide noise instantly (e.g., `logprism -exclude level=DEBUG`).
*   **Logical OR**: Filter for multiple states (e.g., `logprism -filter level=ERROR,WARN`).
*   **Universal Key Mapping**: Adapt to any proprietary logging format with `-map level=severity`.

### Stream Observation
Monitor high-traffic production environments with precision.
*   **Contextual Awareness**: See the "story" around your errors with grep-style context (`-C 5`).
*   **High-Volume Sampling**: Watch traffic without flooding your terminal using high-speed sampling (`-sample 10`).
*   **Visual Highlighting**: Make specific trace IDs or patterns "pop" with `-highlight <str>`.

### Universal Formatter
Transform raw JSON into a clean, actionable data view.
*   **Smart Colorization**: Automatically color-coded log levels for instant visual scanning.
*   **Deep Pretty Printing**: Expand complex nested JSON objects with the `-pretty` flag.
*   **Stable Field Sorting**: Non-standard fields are sorted alphabetically for consistent, diff-friendly output.

## Philosophy: High-Speed Streaming

Log processing at scale requires a specialized approach that prioritizes throughput and resource efficiency. 

`logprism` uses a specialized **Direct Byte Scanner**. This means:
1.  **Single Pass**: We touch each character exactly once.
2.  **Minimal Resource Overhead**: It uses a tiny, fixed amount of memory regardless of the log volume.
3.  **Maximum Portability**: The binary is small, self-contained, and contains everything it needs to run.

## License

Licensed under the **Apache License 2.0**. See [LICENSE](LICENSE) for details.