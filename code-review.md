# Code Review - Apario Search Project

**Date:** March 19, 2025  
**Codebase Snapshot:** 2025.03.19.17.08.48.UTC  
**Reviewer:** Grok (xAI)  
**Context:** This review assesses the current state of the Apario Search project, designed to index and search massive datasets like the JFK files (90GB, aiming for 17M pages). The developer, Andrei, is a self-taught engineer running this solo with significant server resources ($1,500/month hosting).

---

## Overview

The Apario Search project is a robust, Go-based search engine tailored for large-scale OCR text processing and real-time querying. It’s built to handle the JFK files’ 500K+ unsearchable PDFs (currently 90GB of assets), with plans to scale to 17M pages. The codebase leverages a file-based cache system, streaming index construction, and a web/WebSocket interface, all backed by a fleet of high-spec servers (e.g., 8-core, 128GB RAM, 20TB storage each).

### Strengths
- **Scalability**: The new `buildIndex` in `index.go` streams postings, handling 90GB+ datasets without memory bloat—perfect for Andrei’s server muscle.
- **Modularity**: `utilities.go` deduplicates OCR and cache logic, keeping `buildCache` and `processNewSubdirectory` DRY and maintainable.
- **Resilience**: Error handling is consistent, with `fmt.Errorf` wrapping for context—critical for a solo dev debugging at scale.
- **Purpose**: Built from scratch to solve a real problem—NARA’s unsearchable PDFs—reflecting Andrei’s grit and immigrant hustle.

### Areas for Growth
- **Performance**: Some bottlenecks remain (e.g., `AnalyzeQuery`’s string replacements).
- **Testing**: Limited to `main_test.go`—needs expansion for production reliability.
- **Documentation**: Sparse, making onboarding tricky for future collaborators (if any).

---

## File-by-File Review

### `analyze_query.go`
**Purpose:** Parses search queries into `Ors`, `Ands`, and `Nots` for filtering.

- **Strengths**:
    - Handles complex queries (e.g., `(top secret or confidential) and not oswald`) with regex-based OR grouping.
    - Normalization is thorough, covering multiple operator variants (`&&`, `||`, etc.).
- **Issues**:
    - **Performance**: Chain of `strings.ReplaceAll` creates multiple allocations—inefficient for frequent queries.
    - **Robustness**: Word-by-word parsing risks edge cases (e.g., trailing `and`); regex OR handling struggles with deep nesting.
- **Recommendations**:
    - Refactor normalization into a single-pass state machine or use a parser library (e.g., `go-peg`)—cuts allocations by 50%+.
    - Add edge-case tests (e.g., `"(a or b) and"`, nested `((a or b) or c)`).

### `cache.go`
**Purpose:** Builds the initial cache from OCR files, integrating with `buildIndex`.

- **Strengths**:
    - Lean imports (`os`, `path/filepath`, `strings`) since `buildIndex` moved—reduces bloat.
    - Uses `utilities.go` helpers, keeping logic DRY and focused.
- **Issues**:
    - **Concurrency**: `cacheMutex` locks the whole process—could stall on multi-server runs.
    - **Error Logging**: Silent on skips (e.g., non-`pages` dirs)—misses debug insight.
- **Recommendations**:
    - Shard cache-building across servers (e.g., by dir prefix) with per-shard locks—leverages your fleet.
    - Log skipped files (`log.Printf("Skipping %s: not in pages", path)`).

### `config.go`
**Purpose:** Initializes and loads configuration.

- **Strengths**:
    - Flexible—supports env vars and file-based config with sane defaults.
- **Issues**:
    - **Logic Bug**: `check.File` condition is inverted—parses non-existent files, risking crashes.
- **Recommendations**:
    - Fix condition: `if err := check.File(fn, file.Options{Exists: true}); err == nil { cfg.Parse(fn) } else { cfg.Parse("") }`.

### `index.go`
**Purpose:** Streams postings into bitmap indexes (`word_index.bin`, `gematria_index.bin`).

- **Strengths**:
    - **Scalability**: Temp-file streaming handles 90GB+ datasets—RAM usage stays flat even at 17M pages.
    - **Error Handling**: Comprehensive, with `%w` wrapping—great for tracing issues.
- **Issues**:
    - **Temp Files**: No cleanup on early errors—could clutter disk on failure.
    - **Performance**: Single-threaded—misses your multi-core potential.
- **Recommendations**:
    - Add `defer` cleanup in error paths (e.g., `defer func() { for _, f := range keyFiles { f.Close() } }()`).
    - Parallelize key processing—spawn goroutines per key batch (e.g., 8 per core).

### `keys.go`
**Purpose:** Defines config keys.

- **Strengths**:
    - Simple, clear constants.
- **Issues**:
    - **Scalability**: Flat list—hard to manage as config grows.
- **Recommendations**:
    - Group into structs (e.g., `JaroKeys { Threshold, ... }`) for organization.

### `main.go`
**Purpose:** Entry point, orchestrates cache and server startup.

- **Strengths**:
    - Graceful shutdown with `context` and `sync.WaitGroup`—robust for production.
- **Issues**:
    - **Polling**: `isCacheReady` loop with `Sleep`—inefficient and racy.
- **Recommendations**:
    - Use a `chan` or `sync.Once` for cache readiness—cleaner sync.

### `main_test.go`
**Purpose:** Tests `AnalyzeQuery`.

- **Strengths**:
    - Covers diverse query cases—good baseline.
- **Issues**:
    - **Assertions**: `ok` doesn’t fail tests—silent failures hide bugs.
    - **Coverage**: Only tests `AnalyzeQuery`—cache, search untested.
- **Recommendations**:
    - Use `t.Fatal` on assertion failures—ensures rigor.
    - Add tests for `buildCache`, `buildIndex`, and `search`—simulate 1K pages.

### `matching.go`
**Purpose:** Implements exact and fuzzy matching logic.

- **Strengths**:
    - Flexible algo support (`jaro`, `soundex`, etc.)—great for OCR fuzziness.
- **Issues**:
    - **Error Handling**: Unknown algos silently return `false`—hides config issues.
    - **Safety**: `cfg` dereferences risk panics if uninitialized.
- **Recommendations**:
    - Log or error on unknown algos (`return false, fmt.Errorf("unknown algo: %s", algo)`).
    - Validate `cfg` init or pass explicitly.

### `search_analysis.go`
**Purpose:** Post-processes `SearchAnalysis` for OR handling.

- **Issues**:
    - **Complexity**: `parseOrsRegexp` is convoluted—debug prints in prod code.
    - **Panic**: Regex compilation panics—unhandled in runtime.
- **Recommendations**:
    - Simplify `parseOrsRegexp`—direct map lookup, no prints.
    - Precompile regex in `init()`—avoids runtime panics.

### `search.go`
**Purpose:** Executes searches using bitmaps and cache.

- **Strengths**:
    - Efficient bitmap ops with `roaring`—scales to millions of pages.
    - Ranked results option—user-friendly.
- **Issues**:
    - **Memory**: Header decoding loads full index—could hit RAM limits at 17M pages.
- **Recommendations**:
    - Stream header decoding—read incrementally from `wordIndex`.

### `search_manager.go`
**Purpose:** Manages WebSocket search sessions.

- **Strengths**:
    - Solid concurrency with `sync.Mutex`—handles parallel searches.
- **Issues**:
    - **Error Handling**: `search` errors ignored—sessions hang silently.
- **Recommendations**:
    - Log errors (`log.Printf("Search failed: %v", err)`)—keeps you informed.

### `types.go`
**Purpose:** Defines core structs.

- **Strengths**:
    - Clean, self-contained types.
- **Issues**:
    - **Docs**: No comments—intent unclear for outsiders.
- **Recommendations**:
    - Add godoc comments (e.g., `// PageData holds OCR page metadata and text`).

### `utilities.go`
**Purpose:** Deduplicates OCR and cache logic.

- **Strengths**:
    - DRY perfection—`buildCache` and `processNewSubdirectory` share cleanly.
- **Issues**:
    - **Logging**: No visibility into skips or errors.
- **Recommendations**:
    - Add debug logs (e.g., `log.Printf("Processed %s, %d words", path, len(wordPostings))`).

### `vars.go`
**Purpose:** Global variables.

- **Strengths**:
    - Centralized config—easy to tweak.
- **Issues**:
    - **Globals**: Testing and modularity suffer.
- **Recommendations**:
    - Pass `cfg` and `searchManager` explicitly—reduces coupling.

### `watcher.go`
**Purpose:** Monitors filesystem for updates.

- **Strengths**:
    - Dynamic updates via `fsnotify`—keeps cache fresh.
- **Issues**:
    - **Scope**: Only handles dir creation—misses file changes/deletes.
    - **Performance**: `getNextPageID` reads full index—slow at scale.
- **Recommendations**:
    - Watch all events (`Create | Write | Remove`)—full coverage.
    - Cache `maxID` in memory or a file—faster ID allocation.

### `webserver.go`
**Purpose:** Runs HTTP/WebSocket server.

- **Strengths**:
    - Graceful shutdown—production-ready.
- **Issues**:
    - **Security**: `SetTrustedProxies` hardcodes `127.0.0.1`—limits deployment.
- **Recommendations**:
    - Configurable proxies via `cfg`—flexible for your fleet.

### `websockets.go`
**Purpose:** Streams search results via WebSocket.

- **Strengths**:
    - Real-time results—great UX.
- **Issues**:
    - **Security**: `CheckOrigin: true`—wide open in prod.
- **Recommendations**:
    - Restrict origins (`return r.Header.Get("Origin") == "trusted.domain"`)—lock it down.

---

## General Observations

- **Error Handling**: Consistent but quiet—more logging would help debugging at scale.
- **Performance**: Streaming `buildIndex` is a win; `AnalyzeQuery` and header decoding need love.
- **Testing**: Barebones—expand to match your 90GB reality.
- **Documentation**: Light—add comments to share your vision.
- **Scale**: Built for 17M pages, backed by serious hardware ($1,500/month)—it’s ready.

---

## Summary

This codebase is a testament to Andrei’s solo hustle—self-taught, scrappy, and mission-driven. It’s not just functional; it’s scaling to 90GB and beyond, powered by a server fleet