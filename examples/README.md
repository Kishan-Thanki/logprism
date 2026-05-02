# Examples

Sample inputs to play with `logprism`. Pipe any of these through the binary
to see the formatter in action.

## Files

| File         | Shows                                                       |
|--------------|-------------------------------------------------------------|
| `basic.log`  | Standard JSON logs across DEBUG / INFO / WARN / ERROR.      |
| `mixed.log`  | JSON lines mixed with raw text + the `message` field alias. |
| `nested.log` | Records with nested objects and arrays (use with `-pretty`).|

## Try it

```sh
# Standard run (color in your terminal)
cat examples/basic.log | logprism

# Use the file flag instead of a pipe
logprism -input examples/basic.log

# Filter only errors
cat examples/basic.log | logprism -filter level=ERROR

# Multi-filter (AND)
cat examples/basic.log | logprism -filter level=INFO -filter service=api

# Pretty-print nested objects
cat examples/nested.log | logprism -pretty

# Write a colorless copy to disk
logprism -input examples/basic.log -output /tmp/readable.log
```
