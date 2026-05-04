# Roadmap: logprism v1.4.0 — The Structural Analysis Release

This document outlines the strategic technical objectives and architectural milestones for the next major iteration of `logprism`. Following the successful **stabilization and hardening** of the v1.3.1 Analyzer Engine, our focus shifts toward deep structural inspection and advanced observability patterns.

## Priority #1: Recursive JSON Path Traversal
**Status: In Research**

### Overview
Current iterations of the `logprism` byte-scanner are optimized for high-speed, single-level key-value extraction. While this satisfies most standard logging patterns, it presents a limitation for modern, deeply nested log structures.

### The Objective
Implement a **Path-Aware State Machine** capable of resolving dot-notation queries (e.g., `req.user.id`) without the overhead of full JSON deserialization into memory.

### Constraints
- **Zero-Reflection**: We must avoid `interface{}` and map-based deserialization.
- **Fixed Memory Footprint**: Traversal must occur within our established sub-10MB RSS limit.
- **Single-Pass Efficiency**: The scanner must resolve nested paths while maintaining a linear time complexity ($O(n)$).

## Priority #2: High-Performance Regex Highlighting
**Status: Proposed**

### Objective
Extend the `-highlight` engine to support PCRE-compatible regular expressions. This will enable engineers to track dynamic patterns such as UUIDs, IP addresses, and custom trace headers without specifying literal strings.

### Technical Strategy
- Implement a pre-compiled regex cache during the argument parsing phase.
- Ensure the highlighting injection occurs within the `strings.Builder` pipeline to minimize heap allocations.

## Priority #3: Intelligent Context Window Merging
**Status: Planned**

### Objective
Refine the Contextual Awareness Engine to handle high-frequency event clusters. When multiple log matches occur within a shared context buffer, the engine should intelligently merge the output to prevent redundant context printing, maintaining a clean and actionable stream.

## The Vision: Beyond Formatting
With the completion of **Recursive Path Traversal**, `logprism` will transition from a high-speed formatter to a specialized **Stream Processor**. This positioning allows it to offer significantly higher throughput and a lower resource footprint tailored specifically for observability data.
