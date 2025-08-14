# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is an Obsidian AI Agent project built in Go, designed to integrate AI capabilities with Obsidian, the knowledge management application. The project uses Mage as the build automation tool for all development tasks.

## Development Commands

This project uses [Mage](https://magefile.org/) for build automation. All commands should be run through Mage:

### Essential Commands
- `mage build` - Build the binary to `bin/obsidian-ai-agent`
- `mage test` - Run the full test suite
- `mage dev` - Run the application in development mode
- `mage check` - Run all pre-commit checks (format, lint, test)

### ChromaDB Commands
- `mage chroma:start` - Start ChromaDB Docker container (port 8037)
- `mage chroma:reindex` - Reindex Obsidian vault (defaults: current dir, `Zettelkasten,Projects`)
- `mage chroma:reindexCustom vault_path folders` - Reindex with custom vault path and folders
- `mage chroma:search "query text"` - Perform semantic search on indexed documents
- `mage chroma:clear` - Clear all documents from ChromaDB collection
- `mage chroma:stop` - Stop ChromaDB Docker container

### Other Commands
- `mage clean` - Remove build artifacts
- `mage format` - Format Go code
- `mage lint` - Run golangci-lint (requires golangci-lint to be installed)
- `mage mod` - Tidy go.mod and format code
- `mage install` - Install binary to GOPATH/bin
- `mage` (no args) - List all available targets

### Go Commands
- `go run ./cmd/obsidian-ai-agent` - Run directly with Go
- `go mod tidy` - Tidy dependencies

## Architecture

Standard Go project layout with semantic search capabilities:

### Core Components
- `cmd/obsidian-ai-agent/` - Main search application
- `cmd/reindex/` - Vault reindexing utility  
- `cmd/clear-collection/` - Collection management utility
- `internal/chroma/` - ChromaDB client wrapper
- `internal/indexer/` - Obsidian markdown file indexer
- `pkg/` - Public library code (future)
- `magefile.go` - Mage build automation tasks

### Workflow
1. Start ChromaDB container: `mage chroma:start`
2. Index your vault:
   - Default: `mage chroma:reindex` (current dir, `Zettelkasten,Projects` folders)
   - Custom: `mage chroma:reindexCustom "/path/to/vault" "notes,projects,journal"`
3. Search documents: `mage chroma:search "your query"`
4. Clear index if needed: `mage chroma:clear`
5. Stop ChromaDB when done: `mage chroma:stop`

### Data Flow
- Markdown files → Indexer → ChromaDB → Search results
- Files are processed in batches (default: 50) for efficiency
- Document IDs are generated from file paths using MD5 hashing
- Metadata includes file path, filename, and folder information

## Key Patterns and Conventions

- Use Mage for all build, test, and development tasks
- Follow standard Go project layout conventions
- ChromaDB runs on port 8037 (mapped from container port 8000)
- Index `Zettelkasten/` and `Projects/` directories by default
- All indexing operations are batch-processed for performance
- Search queries use semantic similarity matching