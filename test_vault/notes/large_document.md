# Large Test Document

This is a test document to verify that our chunking implementation works correctly and prevents the MCP token limit issue.

## Introduction

Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum.

Sed ut perspiciatis unde omnis iste natus error sit voluptatem accusantium doloremque laudantium, totam rem aperiam, eaque ipsa quae ab illo inventore veritatis et quasi architecto beatae vitae dicta sunt explicabo. Nemo enim ipsam voluptatem quia voluptas sit aspernatur aut odit aut fugit, sed quia consequuntur magni dolores eos qui ratione voluptatem sequi nesciunt.

## Chapter 1: Understanding ChromaDB

ChromaDB is a powerful vector database that enables semantic search capabilities. When working with large documents, it's important to chunk them appropriately to avoid token limits and improve search relevance.

The key benefits of chunking include:
- Reduced token usage per search result
- Better semantic granularity
- Improved search precision
- Faster retrieval times

### Chunking Strategies

There are several approaches to chunking documents:

1. **Size-based chunking**: Split documents into fixed-size chunks
2. **Semantic chunking**: Split based on content structure (headers, paragraphs)
3. **Hybrid approach**: Combine both strategies for optimal results

Our implementation uses a hybrid approach that first attempts semantic chunking by markdown headers, then falls back to size-based chunking for large sections.

### Implementation Details

The chunking implementation includes:
- Configurable chunk size (default: 2000 characters)
- Configurable overlap (default: 200 characters)
- Header-aware splitting
- Word boundary preservation
- Metadata preservation for chunk relationships

## Chapter 2: Technical Implementation

Here's how the chunking works in our Obsidian indexer:

```go
func (idx *ObsidianIndexer) chunkContent(content string, filePath string) []chroma.Document {
    var chunks []chroma.Document
    
    // First try to split by headers
    headerChunks := idx.splitByHeaders(content)
    
    for i, chunk := range headerChunks {
        // If chunk is still too large, split it further
        if len(chunk) > idx.chunkSize {
            subChunks := idx.splitBySize(chunk, idx.chunkSize, idx.chunkOverlap)
            // Process sub-chunks...
        }
    }
    
    return chunks
}
```

The algorithm ensures that:
- Documents are split at natural boundaries
- Context is preserved through overlapping
- Metadata tracks chunk relationships
- Large sections are further subdivided as needed

## Chapter 3: Configuration and Usage

To use the chunking feature, you can configure the chunk size and overlap:

```go
config := &Config{
    VaultPath:    ".",
    BatchSize:    50,
    Directories:  []string{"notes", "projects"},
    ChunkSize:    2000,  // Target chunk size in characters
    ChunkOverlap: 200,   // Overlap between chunks
}
```

### Best Practices

When configuring chunking:
- Use smaller chunks (1000-2000 chars) for better precision
- Include moderate overlap (10-20% of chunk size) for context
- Consider your content type and structure
- Test with your specific use case

### Monitoring and Optimization

Monitor your chunking effectiveness by:
- Checking search result relevance
- Measuring token usage per query
- Analyzing chunk size distribution
- Testing with representative content

## Conclusion

The implemented chunking solution addresses the MCP token limit issue by:
1. Breaking large documents into manageable pieces
2. Preserving semantic structure through header-aware splitting
3. Maintaining context through configurable overlap
4. Including metadata for chunk relationship tracking

This approach ensures that searches return relevant, properly-sized chunks instead of overwhelming large documents that exceed token limits.

## Additional Resources

For more information about ChromaDB and vector databases:
- ChromaDB Documentation
- Vector Database Best Practices
- Semantic Search Optimization
- Document Chunking Strategies

This concludes our test document. The chunking implementation should split this into multiple, manageable pieces that stay well under the MCP token limit while preserving the document's semantic structure and context.