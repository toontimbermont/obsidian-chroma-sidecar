# Design Patterns and Guidelines

## Key Design Patterns

### 1. Configuration Pattern
- Each component has a `Config` struct with sensible defaults
- `DefaultConfig()` functions provide standard configurations
- Constructor functions accept configuration parameters
- Example:
  ```go
  func DefaultConfig() *Config {
      return &Config{
          Host:           "localhost",
          Port:           8037,
          CollectionName: "notes",
      }
  }
  ```

### 2. Client Wrapper Pattern
- ChromaDB client is wrapped in a custom `Client` struct
- Provides domain-specific methods while hiding implementation details
- Consistent error handling and context propagation

### 3. Incremental Processing Pattern
- File indexing uses smart incremental updates
- Tracks file hashes and modification times in `.obsidian_index.json`
- Only processes changed files to optimize performance

### 4. Batch Processing Pattern
- Documents are processed in configurable batches (default: 50)
- Reduces memory usage and improves performance for large vaults
- Consistent across all indexing operations

### 5. Context Propagation
- All long-running operations accept and use `context.Context`
- Enables cancellation and timeout handling
- Context is typically the first parameter

## Project Guidelines

### 1. Mage-First Development
- Use Mage for all build, test, and development tasks
- Avoid direct Go commands in favor of Mage targets
- All automation should be defined in `magefile.go`

### 2. Error Handling Standards
- Always wrap errors with context using `fmt.Errorf` and `%w`
- Provide meaningful error messages that aid debugging
- Propagate errors up the call stack appropriately

### 3. Testing Philosophy
- Tests are organized by functionality (chunking, content cleaning, etc.)
- Include both unit tests and benchmarks where appropriate
- Test file names follow `*_test.go` convention

### 4. Docker Integration
- ChromaDB runs in Docker container for consistency
- Port mapping: container 8000 → host 8037
- Automatic container management through Mage commands

### 5. File Processing Guidelines
- Default indexing targets: `Zettelkasten/` and `Projects/` directories
- Support for custom vault paths and folder selections
- Content cleaning removes markdown artifacts before indexing
- Document chunking for large files

### 6. Performance Considerations
- Incremental indexing (~30x faster on unchanged files)
- Batch processing for efficient memory usage
- Smart content chunking with configurable overlap
- Async operations where appropriate

### 7. CLI Design
- Use standard flag patterns for command-line tools
- Provide sensible defaults for all configuration options
- Support both interactive and scripted usage

## Anti-Patterns to Avoid
- Don't bypass Mage for build/test operations
- Don't ignore context cancellation in long-running operations
- Don't process files individually when batch processing is available
- Don't hardcode paths or configuration values