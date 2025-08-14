# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is an Obsidian AI Agent project built in Go, designed to integrate AI capabilities with Obsidian, the knowledge management application. The project uses Mage as the build automation tool for all development tasks.

## Quick Start (Easy Installation)

### Install the Auto-Indexing Daemon
1. Install the daemon to your system:
   ```bash
   mage installDaemon
   ```

2. Start the daemon for your Obsidian vault:
   ```bash
   # For default vault (current directory)
   mage chroma:daemon
   
   # For specific vault with custom settings
   mage chroma:daemonCustom "/path/to/your/vault" "notes,projects,journal" "10m"
   ```

3. The daemon will:
   - ✅ Start ChromaDB automatically if not running
   - ✅ Perform initial indexing of your vault
   - ✅ Re-index every 5 minutes (or your custom interval)
   - ✅ Show progress in the terminal
   - ✅ Stop ChromaDB when you press `Ctrl-C`

4. Use the search while daemon is running:
   ```bash
   mage chroma:search "your search query"
   ```

## Development Commands

This project uses [Mage](https://magefile.org/) for build automation. All commands should be run through Mage:

### Essential Commands
- `mage build` - Build the test utility to `bin/obsidian-ai-chroma-test-util`
- `mage buildDaemon` - Build the daemon binary to `bin/obsidian-ai-daemon`
- `mage buildAll` - Build all binaries
- `mage test` - Run the full test suite
- `mage dev` - Run the test utility in development mode
- `mage check` - Run all pre-commit checks (format, lint, test)

### ChromaDB Commands
- `mage chroma:start` - Start ChromaDB Docker container (port 8037)
- `mage chroma:reindex` - Reindex Obsidian vault (defaults: current dir, `Zettelkasten,Projects`)
- `mage chroma:reindexCustom vault_path folders` - Reindex with custom vault path and folders
- `mage chroma:search "query text"` - Perform semantic search on indexed documents
- `mage chroma:clear` - Clear all documents from ChromaDB collection
- `mage chroma:stop` - Stop ChromaDB Docker container

### Daemon Commands (Auto-Indexing)
- `mage chroma:daemon` - Start auto-indexing daemon (default: 5min intervals)
- `mage chroma:daemonCustom vault_path folders interval` - Start daemon with custom settings
  - Example: `mage chroma:daemonCustom "/path/to/vault" "notes,docs" "10m"`
- Press `Ctrl-C` to stop daemon (automatically stops ChromaDB)

### Other Commands
- `mage clean` - Remove build artifacts
- `mage format` - Format Go code
- `mage lint` - Run golangci-lint (requires golangci-lint to be installed)
- `mage mod` - Tidy go.mod and format code
- `mage install` - Install test utility to GOPATH/bin  
- `mage installDaemon` - Install daemon binary to GOPATH/bin
- `mage installAll` - Install all binaries to GOPATH/bin
- `mage` (no args) - List all available targets

### Go Commands
- `go run ./cmd/obsidian-ai-chroma-test-util` - Run test utility directly with Go
- `go run ./cmd/obsidian-ai-daemon` - Run daemon directly with Go
- `go mod tidy` - Tidy dependencies

## Architecture

Standard Go project layout with semantic search capabilities:

### Core Components
- `cmd/obsidian-ai-chroma-test-util/` - ChromaDB test utility (search & debug)
- `cmd/obsidian-ai-daemon/` - Auto-indexing daemon
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
