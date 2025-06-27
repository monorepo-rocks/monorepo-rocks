# PRD: MCP Context Engine

**Project Code‑Name:** mcp‑context‑engine
**Draft Date:** 27 Jun 2025

## 1 Background & Motivation

Modern LLM‑based developer tools need a _fast, offline_ way to fetch the exact slices of code that make an answer trustworthy. Existing SaaS search APIs leak code or add > 500 ms of network latency. Inspired by Augment's real‑time, locally‑scoped context index, we will ship an open‑source engine that teams can run on‑device or on‑prem and expose to any LLM agent via a simple JSON tool interface. It combines:

- **Zoekt** for millisecond‑exact lexical / regex / symbol queries via its positional‑trigram index.
- **FAISS** for semantic (embedding) nearest‑neighbour search over code chunks.

Together they let the model answer plain‑English prompts such as "Find imports for `$` (zx) and `chalk` in the tools package, focusing on how `ci.ts` gets these deps" without leaking code off the laptop.

## 2 Goals & Success Metrics

| #   | Goal                                                             | Metric / Target                                                           |
| --- | ---------------------------------------------------------------- | ------------------------------------------------------------------------- |
| G1  | **Sub‑100 ms retrieval** for 95 % of queries on a 10 k‑file repo |  p95 end‑to‑end latency < 100 ms                                          |
| G2  | **Accurate hybrid ranking**                                      |  ≥ 95 % of internal eval questions answered correctly with top‑10 context |
| G3  | **Zero network dependency**                                      |  All indexing & search run with no external API calls                     |
| G4  | **Multi‑language support** (JS/TS, Python, Go)                   |  Full‑function & import detection in all three langs                      |
| G5  | **Real‑time freshness**                                          |  File edits reflected in index ≤ 10 s after save (single‑dev laptop)      |

## 3 Non‑Goals

- IDE integration UX (will be delivered by downstream plugins).
- Indexing monorepos with **millions** of files (focus is ≤ 100 k).
- SaaS/cloud deployment (self‑hosters may dockerise, but container image is out of scope).

## 4 User Stories

1. **LLM agent** issues `code.search({"query":"import chalk"})` and receives JSON hits in < 100 ms.
2. **Developer** edits `utils/helpers.go`; 5 s later a follow‑up question surfaces the updated snippet (incremental re‑index).
3. **QA engineer** asks in chat: "Where is JWT verified?" and semantic search via FAISS returns relevant functions whose names don't literally contain "JWT".

## 5 Functional Requirements

### 5.1 Indexing

| Component               | Technology                                  | Key Requirements                                                                       |
| ----------------------- | ------------------------------------------- | -------------------------------------------------------------------------------------- |
| **Lexical indexer**     | Zoekt indexer binary                        | Positional‑trigram index per repo; parse commit SHA; incremental on file‑change events |
| **Embedding generator** | Pluggable local model (e.g., CodeBERT‑base) | Batched GPU/CPU; chunk size ≤ 300 tokens; store `[file‑id, span‑offset]` metadata      |
| **Vector store**        | FAISS (Flat + IVF for > 50 k vectors)       | Disk‑backed; mmap on startup; HNSW optional for speed                                  |

### 5.2 Query Path

1. Receive request `{text_query, filters, k}`.
2. **Keyword extractor** pulls camelCase tokens & file hints, then queries Zoekt.
3. Embed full question; query FAISS for `k` nearest code chunks.
4. **Fusion ranker** merges lists (BM25 weight for Zoekt, cosine‑sim score for FAISS).
5. Return top‑N snippets (+ metadata) as JSON.

### 5.3 APIs

- **CLI:** `mcpce index /path/to/repo`; `mcpce search -q "initDatabase"`.
- **gRPC / HTTP:** `/v1/search` & `/v1/indexStatus`.
- **Tool schema** (for agents):

```json
{
  "name": "code_context",
  "description": "Hybrid lexical+semantic code search",
  "parameters": {
    "query": "string",
    "lang": "string?",
    "top_k": { "type": "integer", "default": 20 }
  }
}
```

### 5.4 Security / Privacy

- Runs entirely on localhost; no outbound calls.
- Optional AES‑encrypted index files.
- Respects `.gitignore` / allow‑list patterns.

## 6 System Architecture (Textual)

```
┌──────────┐ file events ┌────────────────┐
│ FS Watch│───► (debounce) ───►│Zoekt Indexer │
└──────────┘ └────────────────┘
│ ▲
│ embeddings │ trigram index
▼ │
┌────────────────┐ vectors ┌───┴──────────────┐
│Embedding Worker│─────────────►│ FAISS Index │
└────────────────┘ └───┬──────────────┘
│
▼
┌─────────────────┐
│ Query Service │
└─────────────────┘
│ JSON
▼
LLM / CLI / IDE Plugin
```

## 7 Performance & Scalability

| Repo size  | Cold index | RAM (search) | Disk (index) |
| ---------- | ---------- | ------------ | ------------ |
| 5 k files  | 30 s       | \~200 MB     | \~150 MB     |
| 50 k files | 4 m        | \~1 GB       | \~1.2 GB     |

Zoekt index ≈ 3-4× file size; FAISS vectors (768‑d float32) add \~3 MB per 5 k chunks.

## 8 Milestones

| Date       | Milestone                                      | Notes                  |
| ---------- | ---------------------------------------------- | ---------------------- |
| **Wk 0‑1** | Tech‑spike: build Zoekt+FAISS PoC on demo repo |                        |
| Wk 2‑3     | Incremental indexing & CLI UX                  |                        |
| Wk 4       | Hybrid ranking & eval harness                  |                        |
| Wk 5       | Alpha 0.1.0 release (single‑repo)              |                        |
| Wk 8       | Beta 0.5.0 (multi‑repo, API, encryption)       |                        |
| Wk 10      | 1.0 GA                                         | Docs, versioned schema |

## 9 Risks & Mitigations

| Risk                             | Impact                 | Mitigation                                       |
| -------------------------------- | ---------------------- | ------------------------------------------------ |
| Embedding latency on large diffs | Stale answers          | Parallelise chunks; cache unchanged hashes       |
| Disk bloat for FAISS             | SSD exhaustion         | Support PQ compression / IVF‑PQ                  |
| Memory spikes in Zoekt           | OOM on low‑RAM laptops | Enable shard pruning; limit concurrent searches  |
| Windows file‑watch quirks        | Missed updates         | Use cross‑platform watcher with fallback polling |

## 10 Open Questions

1. Default local embedding model? (quality vs. VRAM)
2. Streaming search mode for ≥ 100 k results?
3. Additional language support (Rust, Java) in v1?

## 11 Appendix & References

- GitLab engineering on replacing Elastic with Zoekt for exact code search.
- Zoekt README - trigram index & symbol‑aware ranking.
- FAISS docs - billion‑scale vector search offline.
- Augment Code blog - need for secure real‑time personal index.
