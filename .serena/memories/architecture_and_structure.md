# Architecture and Structure

## Project Layout
Standard Go project layout with semantic search capabilities:

### Directory Structure
```
obsidian-ai-agent/
├── cmd/                           # Command-line applications
│   ├── obsidian-ai-daemon/       # Auto-indexing daemon
│   ├── obsidian-ai-chroma-test-util/ # ChromaDB test utility (search & debug)
│   ├── similarity-server/         # HTTP server for similarity endpoints
│   └── clear-collection/          # Collection management utility
├── internal/                      # Private application code
│   ├── chroma/                   # ChromaDB client wrapper
│   ├── indexer/                  # Obsidian markdown file indexer
│   └── httpserver/               # HTTP server components
├── pkg/                          # Public library code (future use)
├── test_vault/                   # Test data
├── chroma/                       # ChromaDB related files
├── magefile.go                   # Mage build automation tasks
├── go.mod                        # Go module definition
├── CLAUDE.md                     # Claude Code instructions
└── README.md                     # Project documentation
```

### Core Components

#### ChromaDB Client (`internal/chroma/`)
- `Client` struct: Wrapper around ChromaDB v2 client
- Provides methods for document operations (Add, Upsert, Query, etc.)
- Handles connection management and collection operations

#### Indexer (`internal/indexer/`)
- `ObsidianIndexer` struct: Main indexing engine
- Features:
  - Incremental indexing using file hashes and modification times
  - Content chunking for large documents
  - Batch processing for performance
  - Smart content cleaning (removes markdown artifacts)
- Uses `.obsidian_index.json` for tracking indexed files

#### Command Applications
- Each application in `cmd/` is a standalone Go program
- Common pattern: CLI flags for configuration, context-based operations
- Integration with ChromaDB for data persistence

## Data Flow
1. **Markdown files** → **Indexer** → **ChromaDB** → **Search results**
2. Files are processed in batches (default: 50) for efficiency
3. Document IDs generated from file paths using MD5 hashing
4. Metadata includes file path, filename, and folder information
5. Content is chunked and cleaned before indexing

## Key Architecture Decisions
- ChromaDB runs on port 8037 (mapped from container port 8000)
- Default indexing targets: `Zettelkasten/` and `Projects/` directories
- Batch processing for performance optimization
- Incremental updates to avoid reprocessing unchanged files