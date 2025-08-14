# Obsidian AI Agent

An intelligent semantic search tool for your Obsidian vault, powered by ChromaDB and built in Go.

## Features

- 🔍 **Semantic Search**: Find notes by meaning, not just keywords
- 🚀 **Incremental Indexing**: Only processes new/changed files for lightning-fast updates
- 🤖 **Auto-Indexing Daemon**: Automatically keeps your vault indexed with configurable intervals
- 🐳 **Docker Integration**: Automatically manages ChromaDB container
- 🛠️ **Developer Tools**: Built-in test utilities and debugging tools

## Quick Start

### Prerequisites

- [Go](https://golang.org/doc/install) (1.19 or later)
- [Docker](https://docs.docker.com/get-docker/)
- [Mage](https://magefile.org/) (build tool)

### Installation

1. **Clone and build:**
   ```bash
   git clone <repository-url>
   cd obsidian-ai-agent
   mage installDaemon
   ```

2. **Start the auto-indexing daemon:**
   ```bash
   # For vault in current directory
   obsidian-ai-daemon
   
   # For specific vault location
   obsidian-ai-daemon -vault "/path/to/your/obsidian/vault" -dirs "Notes,Projects,Journal"
   ```

3. **Search your vault:**
   ```bash
   # While daemon is running, use the test utility
   obsidian-ai-chroma-test-util -query "project management techniques"
   ```

## Usage

### Auto-Indexing Daemon

The daemon automatically:
- ✅ Starts ChromaDB if not already running
- ✅ Performs initial indexing of your vault  
- ✅ Re-indexes every 5 minutes (configurable)
- ✅ Shows indexing progress in real-time
- ✅ Stops ChromaDB when you press `Ctrl-C`

#### Basic Usage
```bash
# Default settings (current directory, 5-minute intervals)
obsidian-ai-daemon

# Custom vault path
obsidian-ai-daemon -vault "/Users/you/Documents/MyVault"

# Custom directories and interval
obsidian-ai-daemon -vault "/path/to/vault" -dirs "Notes,Projects,Archive" -interval "10m"
```

#### Configuration Options
- `-vault`: Path to your Obsidian vault (default: current directory)
- `-dirs`: Comma-separated list of folders to index (default: "Zettelkasten,Projects")
- `-interval`: How often to re-index (default: "5m"). Examples: "30s", "10m", "1h"
- `-host`: ChromaDB host (default: "localhost")
- `-port`: ChromaDB port (default: 8037)
- `-collection`: ChromaDB collection name (default: "notes")
- `-batch`: Batch size for uploads (default: 50)

#### Example Sessions
```bash
# For a typical Obsidian vault
obsidian-ai-daemon -vault "/Users/you/Documents/ObsidianVault" -dirs "Daily Notes,Projects,Archive"

# For frequent updates (every 2 minutes)
obsidian-ai-daemon -interval "2m"

# For large vaults (bigger batches)
obsidian-ai-daemon -batch 100
```

### Search Your Vault

While the daemon is running, use the test utility to search:

```bash
# Basic search
obsidian-ai-chroma-test-util -query "machine learning algorithms"

# More results
obsidian-ai-chroma-test-util -query "team management" -results 10

# Different collection
obsidian-ai-chroma-test-util -query "project ideas" -collection "notes"
```

### Stopping the Daemon

Press `Ctrl-C` to stop the daemon. It will:
1. Stop the indexing process gracefully
2. Stop the ChromaDB container
3. Exit cleanly

*Note: You may see a harmless runtime error during shutdown - this is cosmetic and doesn't affect functionality.*

## Efficient Indexing

The tool uses smart incremental indexing:

- **First run**: Indexes all files in your specified directories
- **Subsequent runs**: Only processes files that have changed since last indexing
- **Change detection**: Uses file modification times and content hashes
- **Performance**: ~30x faster on unchanged files

### Index File

The tool creates a `.obsidian_index.json` file in your vault directory to track indexed files. This file:
- Stores metadata about each indexed file
- Enables incremental updates
- Is safe to delete (will trigger full re-index)

## Integration with Claude Code

This tool works perfectly with Claude Code's Chroma MCP server:

1. **Start the daemon** to keep your vault indexed
2. **Use Claude Code** with Chroma MCP to search and interact with your notes
3. **Let the daemon** handle all the indexing automatically in the background

## Development Commands

If you're developing or customizing the tool:

```bash
# Build binaries
mage build        # Test utility
mage buildDaemon  # Daemon
mage buildAll     # Both

# Install to system
mage install      # Test utility  
mage installDaemon # Daemon
mage installAll   # Both

# Development
mage dev          # Run test utility
mage test         # Run tests
mage check        # Format, lint, test

# Manual ChromaDB management
mage chroma:start    # Start ChromaDB
mage chroma:stop     # Stop ChromaDB
mage chroma:clear    # Clear all indexed data
```

## Troubleshooting

### ChromaDB Connection Issues
- Ensure Docker is running
- Check that port 8037 is not in use by another application
- Try `docker ps` to see if ChromaDB container is running

### Indexing Issues  
- Check that your vault path is correct
- Ensure the specified directories exist in your vault
- Look for permission issues if files can't be read

### Performance
- For large vaults, increase batch size: `-batch 100`
- For frequent changes, decrease interval: `-interval "1m"`
- The `.obsidian_index.json` file enables incremental updates

### Getting Help
- Check logs for specific error messages
- Use the test utility to verify ChromaDB connectivity
- Restart the daemon if you encounter issues

## Architecture

- **ChromaDB**: Vector database for semantic search (runs in Docker)
- **Go Application**: Fast, efficient indexing and search
- **Incremental Updates**: Only processes changed files
- **Batch Processing**: Efficient handling of large vaults

The tool creates embeddings of your Obsidian notes and stores them in ChromaDB, enabling semantic search that understands meaning and context, not just keyword matching.