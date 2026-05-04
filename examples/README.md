# logprism Examples & Cookbook

This guide provides a comprehensive reference for all flags, filters, and observation tools available in `logprism`. 

Use `cat examples/basic.log | logprism <flags>` to try these out.

## Example Files

| File | Description |
| :--- | :--- |
| `basic.log` | Standard JSON logs with levels, metrics, and trace IDs. |
| `mixed.log` | JSON lines mixed with raw text + the `message` field alias. |
| `nested.log` | Records with nested objects and arrays (use with `-pretty`). |
| `kubernetes.log` | K8s style logs with `pod`, `ns`, and `container` metadata. |
| `nginx_access.log` | Structured HTTP access logs (IP, Path, Status, Latency). |
| `chaos.log` | **Adversarial**: Mix of valid JSON, truncated lines, and binary. |

## 1. The Analyzer Engine (`-filter`, `-exclude`)

The analyzer supports string equality, numeric comparisons, and multi-condition logic.

### Equality & Numeric Filtering
| Command | Description |
| :--- | :--- |
| `-filter level=ERROR` | Include only ERROR logs. |
| `-filter "status>=500"` | Find all server-side failures (e.g. in `nginx_access.log`). |
| `-filter "latency_ms<10.5"` | Find high-performance requests. |
| `-filter "latency!=-1"` | Exclude records where latency is exactly -1. |

### Multi-Condition Logic
| Command | Description |
| :--- | :--- |
| `-filter level=ERROR,WARN` | **Logical OR**: Include ERROR OR WARN logs. |
| `-filter level=ERROR -filter service=api` | **Logical AND**: Include only ERRORs from the 'api' service. |

### Data Exclusion
| Command | Description |
| :--- | :--- |
| `-exclude level=DEBUG` | Hide all DEBUG noise. |
| `-exclude pod=auth-api-8c6f` | Hide logs from a specific pod (e.g. in `kubernetes.log`). |

## 2. The Observation Suite (`-C`, `-highlight`, `-sample`)

Monitor streams and track patterns with precision.

### Context & Highlighting
| Command | Description |
| :--- | :--- |
| `-C 5` or `-context 5` | Show 5 lines of context before and after each match. |
| `-highlight "req-abc-123"` | Highlight specific trace IDs or IDs in yellow. |
| `-highlight "ERROR,FATAL"` | Highlight multiple terms simultaneously. |

### Traffic Sampling
| Command | Description |
| :--- | :--- |
| `-sample 10` | Show only 10% of the stream (ideal for high-traffic production). |
| `-sample 1` | Show a 1% "pulse" of the incoming traffic. |

## 3. The Transformation Suite (`-pretty`, `-map`, `-message`, `-level`)

Adapt the output to your specific log format.

### Formatting & Mapping
| Command | Description |
| :--- | :--- |
| `-pretty` | Indent and expand nested JSON objects and arrays (try with `nested.log`). |
| `-map trace_id=request` | Rename the `trace_id` key to `request` in the output. |
| `-map remote_ip=client` | Map Nginx fields to custom names. |

### Field Aliases
Use these if your logs don't use the standard `msg` or `level` keys.
| Command | Description |
| :--- | :--- |
| `-message description` | Use the `description` field as the primary log message. |
| `-level severity` | Use the `severity` field for color-coding levels. |

## 4. Global Configuration

| Command | Description |
| :--- | :--- |
| `-input file.log` | Read from a file instead of `stdin`. |
| `-output out.txt` | Write to a file instead of `stdout`. |
| `-no-color` | Disable ANSI color codes (ideal for saving to files). |
| `-version` | Display the current version and build info. |

## Combined "Pro" Example
Isolate errors in the `prod` namespace, show 3 lines of context, pretty-print the details, and highlight the pod name:

```sh
cat examples/kubernetes.log | logprism \
  -filter ns=prod \
  -filter level=ERROR \
  -C 3 \
  -pretty \
  -highlight "auth-api"
```
