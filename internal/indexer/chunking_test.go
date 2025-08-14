package indexer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
			
			if len(result) != len(tt.expected) {
				t.Errorf("splitByHeaders() got %d chunks, want %d", len(result), len(tt.expected))
				for i, chunk := range result {
					t.Logf("Chunk %d: %q", i, chunk)
				}
				return
			}

			for i, chunk := range result {
				if strings.TrimSpace(chunk) != strings.TrimSpace(tt.expected[i]) {
					t.Errorf("splitByHeaders() chunk %d = %q, want %q", i, chunk, tt.expected[i])
				}
			}
		})
	}
}

// TestSplitBySize tests the size-based splitting with overlap
func TestSplitBySize(t *testing.T) {
	indexer := &ObsidianIndexer{
		chunkSize:    50,  // Small size for testing
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
			
			if len(result) < tt.minChunks || len(result) > tt.maxChunks {
				t.Errorf("splitBySize() got %d chunks, want between %d and %d", 
					len(result), tt.minChunks, tt.maxChunks)
			}

			// Verify each chunk is within size limits
			for i, chunk := range result {
				if len(chunk) > tt.chunkSize+50 { // Allow some tolerance for word boundaries
					t.Errorf("splitBySize() chunk %d length %d exceeds chunk size %d", 
						i, len(chunk), tt.chunkSize)
				}
			}

			// Verify all content is preserved
			combined := strings.Join(result, "")
			if len(combined) < len(tt.content) {
				t.Errorf("splitBySize() lost content: original %d chars, combined %d chars", 
					len(tt.content), len(combined))
			}
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
			name: "size-based fallback",
			content: `This is a very long document without any headers that should be split into multiple chunks based on size alone. It contains quite a bit of text to ensure that the size-based chunking algorithm kicks in and creates multiple documents. We want to test that this works correctly and preserves all the content while staying within the specified chunk size limits.`,
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
			
			if len(result) < 1 {
				t.Errorf("chunkContent() returned no chunks")
				return
			}

			// Verify all chunks have required metadata
			for i, chunk := range result {
				if chunk.ID == "" {
					t.Errorf("chunkContent() chunk %d missing ID", i)
				}
				if chunk.Content == "" {
					t.Errorf("chunkContent() chunk %d missing content", i)
				}
				if chunk.Metadata["path"] != tt.filePath {
					t.Errorf("chunkContent() chunk %d wrong path: got %v, want %s", 
						i, chunk.Metadata["path"], tt.filePath)
				}
				if chunk.Metadata["chunk_index"] == nil {
					t.Errorf("chunkContent() chunk %d missing chunk_index", i)
				}
				if chunk.Metadata["chunk_type"] == nil {
					t.Errorf("chunkContent() chunk %d missing chunk_type", i)
				}
			}

			// Verify chunk IDs are unique
			ids := make(map[string]bool)
			for i, chunk := range result {
				if ids[chunk.ID] {
					t.Errorf("chunkContent() duplicate chunk ID: %s at index %d", chunk.ID, i)
				}
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
			if id1 != id2 {
				t.Errorf("generateChunkID() inconsistent: %s != %s", id1, id2)
			}
			
			// Should be non-empty
			if id1 == "" {
				t.Errorf("generateChunkID() returned empty ID")
			}
			
			// Different indices should produce different IDs
			id3 := generateChunkID(tt.filePath, tt.chunkIndex+1)
			if id1 == id3 {
				t.Errorf("generateChunkID() same ID for different indices")
			}
			
			// Different paths should produce different IDs
			id4 := generateChunkID(tt.filePath+"_different", tt.chunkIndex)
			if id1 == id4 {
				t.Errorf("generateChunkID() same ID for different paths")
			}
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
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	indexer := &ObsidianIndexer{
		chunkSize:    100,
		chunkOverlap: 20,
	}

	chunks, fileInfo, err := indexer.processFileWithChunks(testFile)
	if err != nil {
		t.Fatalf("processFileWithChunks() error = %v", err)
	}

	// Should have multiple chunks
	if len(chunks) < 2 {
		t.Errorf("processFileWithChunks() got %d chunks, want at least 2", len(chunks))
	}

	// Verify file info
	if fileInfo == nil {
		t.Errorf("processFileWithChunks() returned nil fileInfo")
	} else {
		if fileInfo.ContentHash == "" {
			t.Errorf("processFileWithChunks() missing content hash")
		}
		if fileInfo.ModTime().IsZero() {
			t.Errorf("processFileWithChunks() missing modification time")
		}
	}

	// Verify chunk structure
	for i, chunk := range chunks {
		if chunk.ID == "" {
			t.Errorf("processFileWithChunks() chunk %d missing ID", i)
		}
		if chunk.Content == "" {
			t.Errorf("processFileWithChunks() chunk %d missing content", i)
		}
		if chunk.Metadata["path"] != testFile {
			t.Errorf("processFileWithChunks() chunk %d wrong path", i)
		}
		if chunk.Metadata["filename"] != "test.md" {
			t.Errorf("processFileWithChunks() chunk %d wrong filename", i)
		}
	}
}

// TestChunkSizeLimits tests that chunks stay within reasonable size limits
func TestChunkSizeLimits(t *testing.T) {
	indexer := &ObsidianIndexer{
		chunkSize:    500,  // Reasonable size for testing
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

	if len(chunks) < 5 {
		t.Errorf("Expected multiple chunks for large document, got %d", len(chunks))
	}

	// Verify no chunk is excessively large (allowing some tolerance)
	for i, chunk := range chunks {
		if len(chunk.Content) > indexer.chunkSize*2 {
			t.Errorf("Chunk %d too large: %d characters (limit: %d)", 
				i, len(chunk.Content), indexer.chunkSize)
		}
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