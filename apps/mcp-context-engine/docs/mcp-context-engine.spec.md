# SPEC: mcp-context-engine

**Last updated:** 27 Jun 2025

## 1 Purpose & Scope

This document translates the "mcp-context-engine" PRD into an implementable design for the first General-Availability (1.0) release. It covers components, interfaces, data formats, performance budgets, security, packaging, and test strategy. Architectural choices reflect findings from the "Local Code Search API" discussion and technology analysis.

## 2 High-Level Architecture

```
+----------------+        +-----------------------+
|   File Watch   |  --->  | Incremental Indexers  |
|  (fsnotify)    |        |  • Zoekt (lexical)    |
+----------------+        |  • Embedding worker   |
          |               |    (CodeBERT / onnx)  |
          |               |  • FAISS vector store |
          v               +-----------+-----------+
   Δ files (inotify)                   |
                                       v
                         +-------------+--------------+
                         |   Query / Fusion Service   |
                         |  • Keyword extractor       |
                         |  • Hybrid ranker (BM25 ⊕   |
                         |    cosine-sim)             |
                         +-------------+--------------+
                                       |
           +---------------------------+---------------------------+
           |                           |                           |
     HTTP API (REST)             MCP stdio tool             CLI (npx)

All services run in-process; no network dependencies, satisfying G3 "zero network".
```

## 3 Key Components

| Component            | Tech / Lang                                      | Responsibilities                                                                       | Performance Target                           |
| :------------------- | :----------------------------------------------- | :------------------------------------------------------------------------------------- | :------------------------------------------- |
| **File-watcher**     | Go, fsnotify                                     | Debounce file events; enqueue changed paths                                            | $\\le$ 100 ms event dispatch                 |
| **Lexical Indexer**  | Zoekt (Go)                                       | Build/merge trigram shards per repo; expose exact/regex search                         | $\\ge$ 10 MiB/s indexing; p95 query \< 15 ms |
| **Embedding Worker** | ONNX Runtime (C++ / Go bindings)                 | Batch-embed code chunks ($\\le$ 300 tokens) and natural-language queries               | $\\le$ 8 ms per 768-d vec on CPU             |
| **Vector Store**     | FAISS Flat ($\\le$ 50 k vec) or IVF-PQ (\> 50 k) | Persist on disk; mmap on start; k-NN search                                            | p95 similarity query \< 20 ms                |
| **Fusion Ranker**    | Go                                               | Normalize BM25 & cosine; $\\lambda$-weighted linear fusion ($\\lambda$ = 0.55 default) | Merge top 512 in $\\le$ 5 ms                 |
| **Query Service**    | Go (net/http)                                    | Parse requests, orchestrate Zoekt/FAISS calls, return JSON                             | End-to-end p95 \< 100 ms (G1)                |

Technology justification (Zoekt + FAISS) drawn from comparative analysis of code-aware vs. generic engines.

## 4 Data & Index Layout

| Path                          | Format             | Contents                                             |
| :---------------------------- | :----------------- | :--------------------------------------------------- |
| `$CACHE/zoekt/*.{meta,zoekt}` | Zoekt shards       | Trigram posting lists, symbol table, per-file CRC    |
| `$CACHE/faiss/{uuid}.index`   | FAISS              | Float32 vectors; IVF header (if used)                |
| `$CACHE/faiss/meta.parquet`   | Parquet            | [vector\_id, file\_id, byte\_start, byte\_end, lang] |
| `$CACHE/embeddings/*`         | sha256-named blobs | Optional raw embeddings (for audit)                  |
| `$CFG/config.yaml`            | YAML               | global + per-repo settings                           |

**Chunking policy:** Break files every 300 tokens or on function boundary (AST parser per language). Store a SHA-1 hash of chunk text; skip re-embedding unchanged hashes.

## 5 External Interfaces

### 5.1 HTTP API

| Method | Path              | Body                                            | Response                                                  |
| :----- | :---------------- | :---------------------------------------------- | :-------------------------------------------------------- |
| `POST` | `/v1/search`      | `{ "query": "string", "k": 20, "lang": "js?" }` | `{ hits: [ {file, lno, text, score, source:"lex"} ... ]}` |
| `GET`  | `/v1/indexStatus` | -                                               | `{ repo: "path", zoekt_pct: 0-100, faiss_pct: 0-100 }`    |

### 5.2 MCP Tool Schema

Matches PRD (§5.3) with operation "code_context". CLI-generated JSON mirrors HTTP body.

### 5.3 Command-Line Interface

```bash
# one-shot index (blocking)
$ npx mcp-context-engine index .

# daemon mode + stdio tool
$ npx mcp-context-engine stdio

# ad-hoc search
$ mcpce search -q "import chalk" -k 15
```

Binary dispatcher lives in `bin.mcp-context-engine` per npm packaging rules (§5.5 PRD).

## 6 Core Algorithms

```pseudocode
// Pseudocode for request path
hitsL := zoekt.Search(q, filters, topLex)       // BM25 scores
embQ   := embedder.Encode(q)                    // 768-d
hitsV := faiss.TopK(embQ, topVec)               // cosine scores
result := fuse(hitsL, hitsV, λ=0.55)            // linear fusion
return sortBy(result, score).Take(k)
```

$\\lambda$ chosen empirically to meet G2 accuracy target ($\\ge$ 95% correct @ top-10).

## 7 Configuration Options (`config.yaml` excerpt)

```yaml
index_root: ~/.cache/mcpce
repo_globs: ['~/code/**']
languages: ['js', 'ts', 'py', 'go']
embedding:
  model: 'microsoft/codebert-base'
  device: 'cpu' # or "cuda:0"
fusion:
  bm25_weight: 0.45 # λ = 1 - bm25_weight
watcher:
  debounce_ms: 250
security:
  encrypt_index: true
  key_path: ~/.config/mcpce/keyfile
```

## 8 Performance & Capacity Budgets

(Based on PRD §7 baseline)

| Repo size  | Cold index | RAM @ search | Disk   | p95 Query |
| :--------- | :--------- | :----------- | :----- | :-------- |
| 5 k files  | 30 s       | 200 MB       | 150 MB | 55 ms     |
| 50 k files | 4 min      | 1 GB         | 1.2 GB | 95 ms     |

## 9 Security & Privacy

- All compute on **localhost**; zero outbound traffic (verifiable by absence of net sockets).
- Optional **AES-GCM encryption** of all index files; key managed by OS keychain.
- Respects `.gitignore` plus user allowlist; no indexing of ignored paths.
- **PID-namespace isolation flag** (`--sandbox`) for container deployments.

## 10 Packaging & Release Pipeline

- **Language:** Core daemon & indexers in Go 1.22; thin Node.js wrapper (`cli/`) for npm.
- **Build:** GitHub Actions matrix (linux/amd64, darwin/arm64, windows/amd64).
- **Semantic Versioning & Provenance:** CI prevents non-incremental tags; run `npm publish --provenance`.
- **Artifact:** Published tarball includes statically linked `mcpce` binary + JS launcher; no library exports.

## 11 Testing & Quality Gates

| Layer           | Tooling                          | Coverage Target                |
| :-------------- | :------------------------------- | :----------------------------- |
| **Unit (Go)**   | `go test` + Gilligan fakes       | 80% lines                      |
| **Integration** | `docker-compose` (Zoekt, FAISS)  | All APIs                       |
| **Performance** | Locust (`/v1/search` mixed load) | p95 \< 100 ms                  |
| **Accuracy**    | Eval harness (100 internal Q\&A) | $\\ge$ 95% top-10 correct (G2) |
| **Security**    | `gosec`, `dependency-check`      | No critical CVEs               |

## 12 Open Questions & Next Steps

- **Default embedding model:** CodeBERT vs. StarCoder-2 small; choose by VRAM footprint + eval accuracy.
- **Hybrid query streaming** for $\\ge$ 100 k results (PRD §10 Q2). Investigate chunked HTTP responses.
- **Rust / Java AST chunking** roadmap for v1.1.

## 13 Glossary

- **Zoekt** - trigram-based code search engine (exact/regex).
- **FAISS** - Facebook AI Similarity Search library for dense vectors.
- **BM25** - standard term-frequency ranking function.
- **MCP** - Minimal Command Protocol used by LLM agents/tools.

This specification is intended to be self-contained; refer to the accompanying PRD for business context and KPIs.
