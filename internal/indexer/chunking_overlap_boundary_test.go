package indexer

import (
	"testing"
)

// TestChunkingOverlapBoundaryIssue reproduces the specific issue where
// overlap at document boundaries creates excessive tiny chunks
func TestChunkingOverlapBoundaryIssue(t *testing.T) {
	indexer := &ObsidianIndexer{
		chunkSize:    1000,
		chunkOverlap: 100,
	}

	// This reproduces the exact scenario from Strategy Books.md
	// Content that's larger than chunk size but ends with a short line that gets fragmented
	content := buildTestContent()
	
	t.Logf("Content length: %d characters", len(content))
	t.Logf("Expected reasonable chunk count: ~%d", (len(content)/indexer.chunkSize)+1)

	// Test the splitBySize function directly
	chunks := indexer.splitBySize(content, indexer.chunkSize, indexer.chunkOverlap)
	
	t.Logf("Actual chunk count: %d", len(chunks))

	// Analyze the chunks
	meaningfulChunks := 0
	tinyChunks := 0
	minMeaningfulSize := 100 // Chunks smaller than this are considered "tiny"
	
	for i, chunk := range chunks {
		if len(chunk) >= minMeaningfulSize {
			meaningfulChunks++
		} else {
			tinyChunks++
			// Log the first few and last few tiny chunks to see the pattern
			if tinyChunks <= 5 || i >= len(chunks)-5 {
				t.Logf("Tiny chunk %d (len=%d): %q", i, len(chunk), chunk)
			}
		}
	}

	t.Logf("Meaningful chunks (>=%d chars): %d", minMeaningfulSize, meaningfulChunks)
	t.Logf("Tiny chunks (<%d chars): %d", minMeaningfulSize, tinyChunks)

	// ASSERTIONS: This test should FAIL before the fix
	
	// We expect a reasonable number of meaningful chunks (around 4-6 for this content size)
	expectedMaxMeaningfulChunks := 10
	if meaningfulChunks > expectedMaxMeaningfulChunks {
		t.Errorf("Too many meaningful chunks: got %d, expected <= %d", meaningfulChunks, expectedMaxMeaningfulChunks)
	}

	// We should not have excessive tiny chunks (this will FAIL before fix)
	maxAcceptableTinyChunks := 2 // Allow a couple small chunks at boundaries
	if tinyChunks > maxAcceptableTinyChunks {
		t.Errorf("Too many tiny chunks: got %d, expected <= %d", tinyChunks, maxAcceptableTinyChunks)
	}

	// Total chunk count should be reasonable
	maxReasonableTotal := 15
	if len(chunks) > maxReasonableTotal {
		t.Errorf("Too many total chunks: got %d, expected <= %d", len(chunks), maxReasonableTotal)
	}

	// Check that we don't have single-character chunks (common symptom of the bug)
	singleCharChunks := 0
	for _, chunk := range chunks {
		if len(chunk) == 1 {
			singleCharChunks++
		}
	}
	if singleCharChunks > 0 {
		t.Errorf("Found %d single-character chunks (should be 0)", singleCharChunks)
	}

	// Verify content preservation - all chunks combined should contain all original content
	combinedLength := 0
	for _, chunk := range chunks {
		combinedLength += len(chunk)
	}
	
	// Due to overlap, combined length will be larger than original, but should be reasonable
	maxExpectedCombined := len(content) + (len(chunks) * indexer.chunkOverlap)
	if combinedLength > maxExpectedCombined {
		t.Errorf("Combined chunk length too large: got %d, max expected ~%d", combinedLength, maxExpectedCombined)
	}
}

// TestSpecificOverlapBoundaryCase tests the exact boundary condition that causes the issue
func TestSpecificOverlapBoundaryCase(t *testing.T) {
	indexer := &ObsidianIndexer{
		chunkSize:    100,  // Small size for easier testing
		chunkOverlap: 20,   // 20% overlap
	}

	// Create content that will trigger the boundary issue
	// Content that's just over 2 chunks in size, ending with a short line
	mainContent := "This is the main content that should be split into reasonable chunks. " +
		"It contains enough text to span multiple chunks when processed. " +
		"We want to ensure that the overlap algorithm works correctly. "
	
	// This short ending line is what causes the cascade of tiny chunks
	shortEnding := "Short ending line."
	
	content := mainContent + shortEnding
	
	t.Logf("Content: %q", content)
	t.Logf("Content length: %d", len(content))
	t.Logf("Main content length: %d", len(mainContent))
	t.Logf("Short ending length: %d", len(shortEnding))

	chunks := indexer.splitBySize(content, indexer.chunkSize, indexer.chunkOverlap)
	
	t.Logf("Generated %d chunks:", len(chunks))
	for i, chunk := range chunks {
		t.Logf("Chunk %d (len=%d): %q", i, len(chunk), chunk)
	}

	// This should generate a reasonable number of chunks (2-4), not dozens
	maxReasonableChunks := 6
	if len(chunks) > maxReasonableChunks {
		t.Errorf("Too many chunks generated: got %d, expected <= %d", len(chunks), maxReasonableChunks)
	}

	// Check for the cascade of tiny chunks at the end
	cascadeChunks := 0
	for i := len(chunks) - 10; i < len(chunks) && i >= 0; i++ {
		if len(chunks[i]) < 10 { // Very small chunks
			cascadeChunks++
		}
	}
	
	if cascadeChunks > 3 {
		t.Errorf("Found cascade of %d tiny chunks at the end", cascadeChunks)
	}
}

// buildTestContent creates content similar to the Strategy Books.md that triggered the issue
func buildTestContent() string {
	// Simulate the structure that caused the problem:
	// 1. Main content that exceeds chunk size
	// 2. Several sections
	// 3. Ends with a short template line
	
	content := `202508181811
Categories: Strategy
Tags: 

---

# Strategy Books

There are many excellent books on business strategy and management consulting. Here are some highly regarded ones that cover a range of topics within these areas:

1. "Competitive Strategy: Techniques for Analyzing Industries and Competitors" by Michael E. Porter: Porter is a renowned strategist and this book is a classic in the field of business strategy.
2. "Blue Ocean Strategy: How to Create Uncontested Market Space and Make Competition Irrelevant" by W. Chan Kim and Renée Mauborgne: This book introduces the concept of creating new market spaces rather than competing in existing ones.
3. "Good Strategy Bad Strategy: The Difference and Why It Matters" by Richard Rumelt: Rumelt provides insights into what makes a good strategy and how to distinguish it from ineffective approaches.
4. "The McKinsey Way" by Ethan M. Rasiel: This book provides an insiders perspective on the principles and strategies used by the renowned consulting firm McKinsey & Company.
5. "The Pyramid Principle: Logic in Writing and Thinking" by Barbara Minto: While not specifically about strategy, this book is highly recommended for consultants.

Many more books could be listed here but these represent some of the most influential and widely recommended texts in the field of business strategy and management consulting.

---
# Related Notes


---
# References


---
Last modified date: ` + "`=dateformat(this.file.mtime, \"DD, HH:mm\")`"

	return content
}