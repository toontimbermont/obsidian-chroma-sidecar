# Suggested Commands

## Essential Development Commands

### Build Commands
- `mage build` - Build the test utility to `bin/obsidian-ai-chroma-test-util`
- `mage buildDaemon` - Build the daemon binary to `bin/obsidian-ai-daemon`
- `mage buildAll` - Build all binaries
- `mage test` - Run the full test suite
- `mage dev` - Run the test utility in development mode
- `mage check` - Run all pre-commit checks (format, lint, test)

### Quality Assurance
- `mage format` - Format Go code using `gofmt`
- `mage lint` - Run golangci-lint (requires golangci-lint to be installed)
- `mage mod` - Tidy go.mod and format code

### Installation Commands
- `mage install` - Install test utility to GOPATH/bin  
- `mage installDaemon` - Install daemon binary to GOPATH/bin
- `mage installAll` - Install all binaries to GOPATH/bin

### ChromaDB Management
- `mage chroma:start` - Start ChromaDB Docker container (port 8037)
- `mage chroma:stop` - Stop ChromaDB Docker container
- `mage chroma:reindex` - Reindex Obsidian vault (defaults: current dir, `Zettelkasten,Projects`)
- `mage chroma:reindexCustom vault_path folders` - Reindex with custom vault path and folders
- `mage chroma:search "query text"` - Perform semantic search on indexed documents
- `mage chroma:clear` - Clear all documents from ChromaDB collection

### Daemon Commands (Auto-Indexing)
- `mage chroma:daemon` - Start auto-indexing daemon (default: 5min intervals)
- `mage chroma:daemonCustom vault_path folders interval` - Start daemon with custom settings
- Press `Ctrl-C` to stop daemon (automatically stops ChromaDB)

### Direct Go Commands (Alternative)
- `go run ./cmd/obsidian-ai-chroma-test-util` - Run test utility directly
- `go run ./cmd/obsidian-ai-daemon` - Run daemon directly
- `go mod tidy` - Tidy dependencies

### Utility Commands
- `mage clean` - Remove build artifacts
- `mage` (no args) - List all available targets

## System Commands (Darwin)
- Standard Unix commands apply: `ls`, `cd`, `grep`, `find`
- `docker ps` - Check running containers
- `docker logs <container>` - Check container logs