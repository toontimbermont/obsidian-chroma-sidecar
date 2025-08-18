# Project Overview

## Purpose
This is an Obsidian AI Agent project built in Go, designed to integrate AI capabilities with Obsidian (the knowledge management application). The main functionality is to provide semantic search capabilities for Obsidian vaults using ChromaDB as the vector database backend.

## Key Features
- **Semantic Search**: Find notes by meaning, not just keywords
- **Incremental Indexing**: Only processes new/changed files for efficiency
- **Auto-Indexing Daemon**: Automatically keeps vault indexed with configurable intervals
- **Docker Integration**: Automatically manages ChromaDB container
- **Batch Processing**: Efficient handling of large vaults

## Tech Stack
- **Language**: Go 1.24.5
- **Build Tool**: Mage (magefile.org) - used for all build automation
- **Vector Database**: ChromaDB (runs in Docker container on port 8037)
- **Main Dependencies**:
  - `github.com/amikos-tech/chroma-go v0.2.3` - ChromaDB Go client
  - `github.com/magefile/mage v1.15.0` - Build automation tool
  - Various supporting libraries (google/uuid, pkg/errors, etc.)

## Core Applications
- **obsidian-ai-daemon**: Auto-indexing daemon that monitors vault changes
- **obsidian-ai-chroma-test-util**: Search and debugging utility
- **similarity-server**: HTTP server for similarity search
- **reindex**: Manual vault reindexing utility
- **clear-collection**: Collection management utility