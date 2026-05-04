# Scripts

Internal utility scripts for development and benchmarking.

## Files

| File | Description |
| :--- | :--- |
| `gen_perf_data.go` | Generates a 1-million line structured JSON log file (`perf_1m.log`) for performance benchmarking. |

## Usage

### Generating Benchmark Data
To generate a new 120MB+ dataset for local testing:

```sh
go run scripts/gen_perf_data.go
```

This will create `perf_1m.log` in the project root, which can then be used with:
```sh
time ./logprism -input perf_1m.log -no-color > /dev/null
```
