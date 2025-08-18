package indexer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"obsidian-ai-agent/internal/chroma"
)

// MockChromaClient implements the ChromaClient interface for testing
type MockChromaClient struct {
	UpsertCalls   [][]chroma.Document
	UpsertErrors  []error
	callIndex     int
}

func NewMockChromaClient() *MockChromaClient {
	return &MockChromaClient{
		UpsertCalls:  make([][]chroma.Document, 0),
		UpsertErrors: make([]error, 0),
	}
}

func (m *MockChromaClient) UpsertDocuments(ctx context.Context, documents []chroma.Document) error {
	m.UpsertCalls = append(m.UpsertCalls, documents)
	
	if m.callIndex < len(m.UpsertErrors) {
		err := m.UpsertErrors[m.callIndex]
		m.callIndex++
		return err
	}
	m.callIndex++
	return nil
}

// GetTotalUpsertedDocuments returns the total number of documents upserted across all calls
func (m *MockChromaClient) GetTotalUpsertedDocuments() int {
	total := 0
	for _, docs := range m.UpsertCalls {
		total += len(docs)
	}
	return total
}

// GetUpsertCallCount returns the number of times UpsertDocuments was called
func (m *MockChromaClient) GetUpsertCallCount() int {
	return len(m.UpsertCalls)
}

// TestNewFileIndexing tests that a new file is properly indexed with correct ChromaDB calls
func TestNewFileIndexing(t *testing.T) {
	// Create temporary directory and file
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "new_note.md")
	
	testContent := `# New Note

This is a new note that hasn't been indexed before.

## Section 1

Some content in the first section.

## Section 2

Content in the second section.`

	err := os.WriteFile(testFile, []byte(testContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create mock client and indexer
	mockClient := NewMockChromaClient()
	config := &Config{
		VaultPath:    tempDir,
		BatchSize:    10,
		Directories:  []string{"."},
		ChunkSize:    50,  // Reduced from 200 to force chunking
		ChunkOverlap: 10,  // Reduced proportionally
	}
	
	indexer := NewObsidianIndexer(mockClient, config)

	// Perform indexing
	ctx := context.Background()
	result, err := indexer.ReindexVault(ctx, []string{"."})
	if err != nil {
		t.Fatalf("ReindexVault failed: %v", err)
	}

	// Verify results
	if result.ProcessedFiles != 1 {
		t.Errorf("Expected 1 processed file, got %d", result.ProcessedFiles)
	}
	if result.IndexedFiles != 1 {
		t.Errorf("Expected 1 new indexed file, got %d", result.IndexedFiles)
	}
	if result.UpdatedFiles != 0 {
		t.Errorf("Expected 0 updated files, got %d", result.UpdatedFiles)
	}
	if result.SkippedFiles != 0 {
		t.Errorf("Expected 0 skipped files, got %d", result.SkippedFiles)
	}

	// Verify ChromaDB calls
	if mockClient.GetUpsertCallCount() != 1 {
		t.Errorf("Expected 1 upsert call, got %d", mockClient.GetUpsertCallCount())
	}

	totalDocs := mockClient.GetTotalUpsertedDocuments()
	if totalDocs < 2 { // Should have multiple chunks
		t.Errorf("Expected at least 2 chunks to be upserted, got %d", totalDocs)
	}

	// Verify chunk metadata
	if len(mockClient.UpsertCalls) > 0 {
		docs := mockClient.UpsertCalls[0]
		for i, doc := range docs {
			if doc.ID == "" {
				t.Errorf("Document %d missing ID", i)
			}
			if doc.Content == "" {
				t.Errorf("Document %d missing content", i)
			}
			if doc.Metadata["path"] != testFile {
				t.Errorf("Document %d wrong path: got %v, want %s", i, doc.Metadata["path"], testFile)
			}
			if doc.Metadata["filename"] != "new_note.md" {
				t.Errorf("Document %d wrong filename: got %v, want %s", i, doc.Metadata["filename"], "new_note.md")
			}
			if doc.Metadata["chunk_index"] == nil {
				t.Errorf("Document %d missing chunk_index", i)
			}
		}
	}
}

// TestChangedFileReindexing tests that a changed file is properly re-indexed
func TestChangedFileReindexing(t *testing.T) {
	// Create temporary directory and file
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "existing_note.md")
	
	originalContent := `# Existing Note

Original content.`

	err := os.WriteFile(testFile, []byte(originalContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create mock client and indexer
	mockClient := NewMockChromaClient()
	config := &Config{
		VaultPath:    tempDir,
		BatchSize:    10,
		Directories:  []string{"."},
		ChunkSize:    50,  // Reduced from 200 to force chunking
		ChunkOverlap: 10,  // Reduced proportionally
	}
	
	indexer := NewObsidianIndexer(mockClient, config)
	ctx := context.Background()

	// First indexing (simulate existing file)
	result1, err := indexer.ReindexVault(ctx, []string{"."})
	if err != nil {
		t.Fatalf("Initial ReindexVault failed: %v", err)
	}

	if result1.IndexedFiles != 1 {
		t.Errorf("Expected 1 new indexed file in first run, got %d", result1.IndexedFiles)
	}

	initialUpsertCount := mockClient.GetUpsertCallCount()
	initialDocCount := mockClient.GetTotalUpsertedDocuments()

	// Wait a moment to ensure different modification time
	time.Sleep(100 * time.Millisecond)

	// Modify the file
	modifiedContent := `# Existing Note

Original content.

## New Section

This is new content added to the file.

## Another Section

Even more new content to ensure the file is detected as changed.`

	err = os.WriteFile(testFile, []byte(modifiedContent), 0644)
	if err != nil {
		t.Fatalf("Failed to modify test file: %v", err)
	}

	// Second indexing (should detect changes)
	result2, err := indexer.ReindexVault(ctx, []string{"."})
	if err != nil {
		t.Fatalf("Second ReindexVault failed: %v", err)
	}

	// Verify results
	if result2.ProcessedFiles != 1 {
		t.Errorf("Expected 1 processed file in second run, got %d", result2.ProcessedFiles)
	}
	if result2.IndexedFiles != 0 {
		t.Errorf("Expected 0 new indexed files in second run, got %d", result2.IndexedFiles)
	}
	if result2.UpdatedFiles != 1 {
		t.Errorf("Expected 1 updated file in second run, got %d", result2.UpdatedFiles)
	}
	if result2.SkippedFiles != 0 {
		t.Errorf("Expected 0 skipped files in second run, got %d", result2.SkippedFiles)
	}

	// Verify ChromaDB calls increased
	finalUpsertCount := mockClient.GetUpsertCallCount()
	finalDocCount := mockClient.GetTotalUpsertedDocuments()

	if finalUpsertCount <= initialUpsertCount {
		t.Errorf("Expected more upsert calls after file change: initial %d, final %d", 
			initialUpsertCount, finalUpsertCount)
	}

	if finalDocCount <= initialDocCount {
		t.Errorf("Expected more documents after file change: initial %d, final %d", 
			initialDocCount, finalDocCount)
	}

	// The modified file should produce more chunks due to additional content
	if len(mockClient.UpsertCalls) >= 2 {
		secondCallDocs := mockClient.UpsertCalls[1]
		if len(secondCallDocs) <= 1 {
			t.Errorf("Expected multiple chunks for modified file, got %d", len(secondCallDocs))
		}
	}
}

// TestUnchangedFileSkipping tests that unchanged files are skipped and no ChromaDB calls are made
func TestUnchangedFileSkipping(t *testing.T) {
	// Create temporary directory and file
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "unchanged_note.md")
	
	content := `# Unchanged Note

This content will not change between indexing runs.

## Section

Some static content.`

	err := os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create mock client and indexer
	mockClient := NewMockChromaClient()
	config := &Config{
		VaultPath:    tempDir,
		BatchSize:    10,
		Directories:  []string{"."},
		ChunkSize:    200,
		ChunkOverlap: 50,
	}
	
	indexer := NewObsidianIndexer(mockClient, config)
	ctx := context.Background()

	// First indexing
	result1, err := indexer.ReindexVault(ctx, []string{"."})
	if err != nil {
		t.Fatalf("Initial ReindexVault failed: %v", err)
	}

	if result1.IndexedFiles != 1 {
		t.Errorf("Expected 1 new indexed file in first run, got %d", result1.IndexedFiles)
	}

	initialUpsertCount := mockClient.GetUpsertCallCount()
	initialDocCount := mockClient.GetTotalUpsertedDocuments()

	// Second indexing without changes
	result2, err := indexer.ReindexVault(ctx, []string{"."})
	if err != nil {
		t.Fatalf("Second ReindexVault failed: %v", err)
	}

	// Verify results
	if result2.ProcessedFiles != 1 {
		t.Errorf("Expected 1 processed file in second run, got %d", result2.ProcessedFiles)
	}
	if result2.IndexedFiles != 0 {
		t.Errorf("Expected 0 new indexed files in second run, got %d", result2.IndexedFiles)
	}
	if result2.UpdatedFiles != 0 {
		t.Errorf("Expected 0 updated files in second run, got %d", result2.UpdatedFiles)
	}
	if result2.SkippedFiles != 1 {
		t.Errorf("Expected 1 skipped file in second run, got %d", result2.SkippedFiles)
	}

	// Verify no additional ChromaDB calls were made
	finalUpsertCount := mockClient.GetUpsertCallCount()
	finalDocCount := mockClient.GetTotalUpsertedDocuments()

	if finalUpsertCount != initialUpsertCount {
		t.Errorf("Expected no additional upsert calls for unchanged file: initial %d, final %d", 
			initialUpsertCount, finalUpsertCount)
	}

	if finalDocCount != initialDocCount {
		t.Errorf("Expected no additional documents for unchanged file: initial %d, final %d", 
			initialDocCount, finalDocCount)
	}
}

// TestBatchProcessing tests that large numbers of files are processed in batches
func TestBatchProcessing(t *testing.T) {
	// Create temporary directory with multiple files
	tempDir := t.TempDir()
	
	// Create 5 files to test batching with batch size of 2
	for i := 0; i < 5; i++ {
		fileName := filepath.Join(tempDir, fmt.Sprintf("note_%d.md", i))
		content := fmt.Sprintf("# Note %d\n\nContent for note %d.", i, i)
		err := os.WriteFile(fileName, []byte(content), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file %d: %v", i, err)
		}
	}

	// Create mock client and indexer with small batch size
	mockClient := NewMockChromaClient()
	config := &Config{
		VaultPath:    tempDir,
		BatchSize:    2, // Small batch size to force multiple batches
		Directories:  []string{"."},
		ChunkSize:    200,
		ChunkOverlap: 50,
	}
	
	indexer := NewObsidianIndexer(mockClient, config)
	ctx := context.Background()

	// Perform indexing
	result, err := indexer.ReindexVault(ctx, []string{"."})
	if err != nil {
		t.Fatalf("ReindexVault failed: %v", err)
	}

	// Verify results
	if result.ProcessedFiles != 5 {
		t.Errorf("Expected 5 processed files, got %d", result.ProcessedFiles)
	}
	if result.IndexedFiles != 5 {
		t.Errorf("Expected 5 new indexed files, got %d", result.IndexedFiles)
	}

	// Verify multiple batch calls were made
	upsertCallCount := mockClient.GetUpsertCallCount()
	if upsertCallCount < 2 {
		t.Errorf("Expected at least 2 upsert calls due to batching, got %d", upsertCallCount)
	}

	// Verify total documents across all batches
	totalDocs := mockClient.GetTotalUpsertedDocuments()
	if totalDocs != 5 { // Each file should produce 1 chunk due to small content
		t.Errorf("Expected 5 total documents across all batches, got %d", totalDocs)
	}

	// Verify no batch exceeds the batch size (in terms of original files, not chunks)
	for i, docs := range mockClient.UpsertCalls {
		// Since each file produces 1 chunk, batch size should be respected
		if len(docs) > config.BatchSize {
			t.Errorf("Batch %d exceeded batch size: got %d docs, limit %d", 
				i, len(docs), config.BatchSize)
		}
	}
}

// TestErrorHandling tests that errors during ChromaDB operations are properly handled
func TestErrorHandling(t *testing.T) {
	// Create temporary directory and file
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "error_test.md")
	
	content := `# Error Test

This file will cause a ChromaDB error during indexing.`

	err := os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create mock client that returns an error
	mockClient := NewMockChromaClient()
	mockClient.UpsertErrors = []error{fmt.Errorf("simulated ChromaDB error")}

	config := &Config{
		VaultPath:    tempDir,
		BatchSize:    10,
		Directories:  []string{"."},
		ChunkSize:    200,
		ChunkOverlap: 50,
	}
	
	indexer := NewObsidianIndexer(mockClient, config)
	ctx := context.Background()

	// Perform indexing
	result, err := indexer.ReindexVault(ctx, []string{"."})
	if err != nil {
		t.Fatalf("ReindexVault failed: %v", err)
	}

	// Verify error was captured
	if len(result.Errors) == 0 {
		t.Errorf("Expected errors to be captured, got none")
	}

	// Verify ChromaDB call was still attempted
	if mockClient.GetUpsertCallCount() != 1 {
		t.Errorf("Expected 1 upsert call despite error, got %d", mockClient.GetUpsertCallCount())
	}
}