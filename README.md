# logprism

**The Universal, High-Speed JSON Log Formatter.**

[![Go Report Card](https://goreportcard.com/badge/github.com/Kishan-Thanki/logprism)](https://goreportcard.com/report/github.com/Kishan-Thanki/logprism)
[![License: Apache 2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![Go Reference](https://pkg.go.dev/badge/github.com/Kishan-Thanki/logprism.svg)](https://pkg.go.dev/github.com/Kishan-Thanki/logprism)

`logprism` is a "bare-metal" CLI utility designed to transform structured JSON logs into beautiful, human-readable terminal output. While written in Go, it is a **universal tool** that works with logs from any language or framework (Java, Python, Rust, Node.js, C#, etc.).

<p align="center">
  <img src="docs/demo.gif" alt="logprism animated demo" width="720">
</p>

## Performance

`logprism` is built for extreme speed. By using a high-efficiency byte-scanner and a manual state machine, it processes log streams with ultra-low latency and minimal CPU and memory overhead. 

It is designed to handle massive, high-throughput production log streams without slowing down your data pipeline or exhausting system resources.

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
# Live application logs
tail -f access.json | logprism

# Docker logs
docker logs -f my-container | logprism

# Process output
./my-app-binary | logprism
```

### Direct File Input
```sh
logprism -input production.log
logprism -input production.log -output readable.log
```

## Features

*   **Zero-Configuration**: Automatically detects well-known fields (`time`, `level`, `service`, `msg`, `trace_id`).
*   **Smart Filtering**: Instant, high-speed filtering without overhead.
    *   `logprism -filter level=ERROR`
*   **Pretty Printing**: Handle complex nested JSON with the `-pretty` flag.
*   **Auto-Color**: Automatically detects if output is a file or pipe to disable ANSI colors.
*   **Alphabetical Extras**: Any non-standard fields are sorted alphabetically for stable, diff-friendly output.

## Log Level Colors

`logprism` automatically colorizes your logs for instant visual scanning:
*   🔴 `ERROR` / `FATAL` / `PANIC`
*   🟡 `WARN`
*   🔵 `INFO`
*   ⚪ `DEBUG`

## Philosophy: Why is it so fast?

Most JSON tools are built to be "general purpose," which makes them slow and memory-heavy. They have to "re-discover" the structure of your data every time they read a line.

`logprism` uses a specialized **Direct Byte Scanner**. This means:
1.  **Single Pass**: We touch each character exactly once.
2.  **Minimal Resource Overhead**: It uses a tiny, fixed amount of memory regardless of the log volume.
3.  **Maximum Portability**: The binary is small, self-contained, and contains everything it needs to run.

## License

Licensed under the **Apache License 2.0**. See [LICENSE](LICENSE) for details.