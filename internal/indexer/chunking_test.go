package indexer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSplitByHeaders tests the header-based splitting functionality
func TestSplitByHeaders(t *testing.T) {
	indexer := &ObsidianIndexer{
		chunkSize:    2000,
		chunkOverlap: 200,
	}

	tests := []struct {
		name     string
		content  string
		expected []string
	}{
		{
			name: "simple headers",
			content: `# Main Title

Some intro content.

## Section 1

Content for section 1.

## Section 2

Content for section 2.

### Subsection

More content.`,
			expected: []string{
				"# Main Title\n\nSome intro content.",
				"## Section 1\n\nContent for section 1.",
				"## Section 2\n\nContent for section 2.",
				"### Subsection\n\nMore content.",
			},
		},
		{
			name: "no headers",
			content: `This is just plain text
with no headers at all.
Should return as single chunk.`,
			expected: []string{
				"This is just plain text\nwith no headers at all.\nShould return as single chunk.",
			},
		},
		{
			name: "multiple header levels",
			content: `# Title

Intro

## Chapter 1

Chapter content

### Section 1.1

Section content

#### Subsection

Deep content

## Chapter 2

More content`,
			expected: []string{
				"# Title\n\nIntro",
				"## Chapter 1\n\nChapter content",
				"### Section 1.1\n\nSection content",
				"#### Subsection\n\nDeep content",
				"## Chapter 2\n\nMore content",
			},
		},
		{
			name:     "empty content",
			content:  "",
			expected: []string{""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := indexer.splitByHeaders(tt.content)

			require.Len(t, result, len(tt.expected), "splitByHeaders() chunk count mismatch")

			for i, chunk := range result {
				assert.Equal(t, strings.TrimSpace(tt.expected[i]), strings.TrimSpace(chunk),
					"chunk %d content mismatch", i)
			}
		})
	}
}

// TestSplitBySize tests the size-based splitting with overlap
func TestSplitBySize(t *testing.T) {
	indexer := &ObsidianIndexer{
		chunkSize:    50, // Small size for testing
		chunkOverlap: 10,
	}

	tests := []struct {
		name      string
		content   string
		chunkSize int
		overlap   int
		minChunks int
		maxChunks int
	}{
		{
			name:      "short content",
			content:   "This is short content.",
			chunkSize: 50,
			overlap:   10,
			minChunks: 1,
			maxChunks: 1,
		},
		{
			name:      "long content with overlap",
			content:   "This is a much longer piece of content that should definitely be split into multiple chunks because it exceeds the chunk size limit that we have set for testing purposes.",
			chunkSize: 50,
			overlap:   10,
			minChunks: 3,
			maxChunks: 20, // Allow more chunks for very small chunk size
		},
		{
			name:      "exact chunk size",
			content:   "This content is exactly fifty characters long!",
			chunkSize: 50,
			overlap:   10,
			minChunks: 1,
			maxChunks: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := indexer.splitBySize(tt.content, tt.chunkSize, tt.overlap)

			assert.GreaterOrEqual(t, len(result), tt.minChunks, "too few chunks")
			assert.LessOrEqual(t, len(result), tt.maxChunks, "too many chunks")

			// Verify each chunk is within size limits
			for i, chunk := range result {
				assert.LessOrEqual(t, len(chunk), tt.chunkSize+50,
					"chunk %d length %d exceeds chunk size %d", i, len(chunk), tt.chunkSize)
			}

			// Verify all content is preserved
			combined := strings.Join(result, "")
			assert.GreaterOrEqual(t, len(combined), len(tt.content),
				"lost content: original %d chars, combined %d chars", len(tt.content), len(combined))
		})
	}
}

// TestChunkContent tests the main chunking logic
func TestChunkContent(t *testing.T) {
	indexer := &ObsidianIndexer{
		chunkSize:    100,
		chunkOverlap: 20,
	}

	tests := []struct {
		name         string
		content      string
		filePath     string
		expectChunks int
		expectTypes  []string
	}{
		{
			name: "header-based chunking",
			content: `# Title

Short intro.

## Section 1

Content here.

## Section 2

More content.`,
			filePath:     "/test/file.md",
			expectChunks: 3,
			expectTypes:  []string{"header", "header", "header"},
		},
		{
			name:         "size-based fallback",
			content:      `This is a very long document without any headers that should be split into multiple chunks based on size alone. It contains quite a bit of text to ensure that the size-based chunking algorithm kicks in and creates multiple documents. We want to test that this works correctly and preserves all the content while staying within the specified chunk size limits.`,
			filePath:     "/test/file.md",
			expectChunks: 3, // Should be split by size
			expectTypes:  []string{"size", "size", "size"},
		},
		{
			name: "mixed chunking",
			content: `# Title

## Section 1

This is a very long section that exceeds our chunk size limit and should be split into multiple sub-chunks while preserving the header structure. The content here is quite extensive and contains multiple sentences that will definitely cause the chunking algorithm to split this section into smaller pieces.

## Section 2

Short section.`,
			filePath:     "/test/file.md",
			expectChunks: 4, // Header + sub-chunks + header
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := indexer.chunkContent(tt.content, tt.filePath)

			require.NotEmpty(t, result, "chunkContent() returned no chunks")

			// Verify all chunks have required metadata
			for i, chunk := range result {
				assert.NotEmpty(t, chunk.ID, "chunk %d missing ID", i)
				assert.NotEmpty(t, chunk.Content, "chunk %d missing content", i)
				assert.Equal(t, tt.filePath, chunk.Metadata["path"],
					"chunk %d wrong path", i)
				assert.NotNil(t, chunk.Metadata["chunk_index"],
					"chunk %d missing chunk_index", i)
				assert.NotNil(t, chunk.Metadata["chunk_type"],
					"chunk %d missing chunk_type", i)
			}

			// Verify chunk IDs are unique
			ids := make(map[string]bool)
			for i, chunk := range result {
				assert.False(t, ids[chunk.ID],
					"duplicate chunk ID: %s at index %d", chunk.ID, i)
				ids[chunk.ID] = true
			}
		})
	}
}

// TestGenerateChunkID tests chunk ID generation
func TestGenerateChunkID(t *testing.T) {
	tests := []struct {
		name       string
		filePath   string
		chunkIndex int
	}{
		{
			name:       "simple path",
			filePath:   "/test/file.md",
			chunkIndex: 0,
		},
		{
			name:       "complex path",
			filePath:   "/Users/test/Documents/My Notes/file with spaces.md",
			chunkIndex: 5,
		},
		{
			name:       "relative path",
			filePath:   "./notes/test.md",
			chunkIndex: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id1 := generateChunkID(tt.filePath, tt.chunkIndex)
			id2 := generateChunkID(tt.filePath, tt.chunkIndex)

			// Should be consistent
			assert.Equal(t, id1, id2, "generateChunkID() inconsistent")

			// Should be non-empty
			assert.NotEmpty(t, id1, "generateChunkID() returned empty ID")

			// Different indices should produce different IDs
			id3 := generateChunkID(tt.filePath, tt.chunkIndex+1)
			assert.NotEqual(t, id1, id3, "generateChunkID() same ID for different indices")

			// Different paths should produce different IDs
			id4 := generateChunkID(tt.filePath+"_different", tt.chunkIndex)
			assert.NotEqual(t, id1, id4, "generateChunkID() same ID for different paths")
		})
	}
}

// TestProcessFileWithChunks tests the integration of file processing with chunking
func TestProcessFileWithChunks(t *testing.T) {
	// Create a temporary test file
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.md")

	testContent := `# Test Document

This is a test document for verifying the chunking functionality.

## Section 1

Some content in section 1 that should be chunked appropriately.

## Section 2

Content in section 2 with more text to ensure proper chunking behavior.`

	err := os.WriteFile(testFile, []byte(testContent), 0644)
	require.NoError(t, err, "Failed to create test file")

	indexer := &ObsidianIndexer{
		chunkSize:    100,
		chunkOverlap: 20,
	}

	chunks, fileInfo, err := indexer.processFileWithChunks(testFile)
	require.NoError(t, err, "processFileWithChunks() error")

	// Should have multiple chunks
	assert.GreaterOrEqual(t, len(chunks), 2, "expected at least 2 chunks")

	// Verify file info
	require.NotNil(t, fileInfo, "processFileWithChunks() returned nil fileInfo")
	assert.NotEmpty(t, fileInfo.ContentHash, "missing content hash")
	assert.False(t, fileInfo.ModTime().IsZero(), "missing modification time")

	// Verify chunk structure
	for i, chunk := range chunks {
		assert.NotEmpty(t, chunk.ID, "chunk %d missing ID", i)
		assert.NotEmpty(t, chunk.Content, "chunk %d missing content", i)
		assert.Equal(t, testFile, chunk.Metadata["path"], "chunk %d wrong path", i)
		assert.Equal(t, "test.md", chunk.Metadata["filename"], "chunk %d wrong filename", i)
	}
}

// TestChunkSizeLimits tests that chunks stay within reasonable size limits
func TestChunkSizeLimits(t *testing.T) {
	indexer := &ObsidianIndexer{
		chunkSize:    500, // Reasonable size for testing
		chunkOverlap: 50,
	}

	// Create a large document
	var largeContent strings.Builder
	largeContent.WriteString("# Large Document\n\n")
	for i := 0; i < 100; i++ {
		largeContent.WriteString("This is paragraph ")
		largeContent.WriteString(strings.Repeat("A", 50))
		largeContent.WriteString(" with lots of text to make it long. ")
	}

	chunks := indexer.chunkContent(largeContent.String(), "/test/large.md")

	assert.GreaterOrEqual(t, len(chunks), 5, "expected multiple chunks for large document")

	// Verify no chunk is excessively large (allowing some tolerance)
	for i, chunk := range chunks {
		assert.LessOrEqual(t, len(chunk.Content), indexer.chunkSize*2,
			"chunk %d too large: %d characters (limit: %d)",
			i, len(chunk.Content), indexer.chunkSize)
	}
}

// BenchmarkChunkContent benchmarks the chunking performance
func BenchmarkChunkContent(b *testing.B) {
	indexer := &ObsidianIndexer{
		chunkSize:    2000,
		chunkOverlap: 200,
	}

	// Create a moderately large document
	var content strings.Builder
	content.WriteString("# Benchmark Document\n\n")
	for i := 0; i < 50; i++ {
		content.WriteString("## Section ")
		content.WriteString(strings.Repeat("content ", 100))
		content.WriteString("\n\n")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = indexer.chunkContent(content.String(), "/test/bench.md")
	}
}
