//go:build integration
// +build integration

package indexer

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"obsidian-ai-agent/internal/chroma"
)

const (
	testContainerName  = "chromadb-test"
	testPort           = "8038"
	testCollectionName = "test-collection"
	testDataDir        = ".chroma-test"
)

// TestIntegrationVaultIndexing is an integration test that:
// 1. Launches ChromaDB in a separate Docker container
// 2. Indexes the test_vault directory
// 3. Verifies the indexing results
// 4. Cleans up the container
func TestIntegrationVaultIndexing(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Check if Docker is available
	if !isDockerAvailable() {
		t.Skip("Docker is not available, skipping integration test")
	}

	// Setup test ChromaDB container
	err := setupTestChromaDB(t)
	require.NoError(t, err, "Failed to setup test ChromaDB")

	// Cleanup container when test finishes
	defer cleanupTestChromaDB(t)

	// Wait for ChromaDB to be ready
	err = waitForChromaDB(t)
	require.NoError(t, err, "ChromaDB failed to start")

	// Create ChromaDB client for testing
	ctx := context.Background()
	config := &chroma.Config{
		Host:           "localhost",
		Port:           8038,
		CollectionName: testCollectionName,
	}

	client, err := chroma.NewClient(ctx, config)
	require.NoError(t, err, "Failed to create ChromaDB client")

	// Set test vault path (relative to project root)
	testVaultPath := filepath.Join("..", "..", "test_vault")

	// Verify test vault exists
	_, err = os.Stat(testVaultPath)
	require.False(t, os.IsNotExist(err), "Test vault directory %s does not exist", testVaultPath)

	// Clean any existing index file in test vault to ensure fresh indexing
	indexFile := filepath.Join(testVaultPath, ".obsidian_index.json")
	if err := os.Remove(indexFile); err != nil && !os.IsNotExist(err) {
		t.Logf("Warning: could not remove existing index file: %v", err)
	}

	// Create indexer with test configuration
	// Using BatchSize=1 to isolate tensor shape issues to specific documents
	indexerConfig := &Config{
		VaultPath:    testVaultPath,
		BatchSize:    1,
		Directories:  []string{"notes"},
		ChunkSize:    2000,
		ChunkOverlap: 200,
	}

	indexer := NewObsidianIndexer(client, indexerConfig)

	// Perform indexing
	result, err := indexer.ReindexVault(ctx, []string{"notes"})
	require.NoError(t, err, "Failed to index test vault")

	// Verify indexing results
	t.Logf("Indexing results: Processed=%d, Indexed=%d, Updated=%d, Skipped=%d, Batches=%d, Errors=%d",
		result.ProcessedFiles, result.IndexedFiles, result.UpdatedFiles,
		result.SkippedFiles, result.BatchesUploaded, len(result.Errors))

	assert.Empty(t, result.Errors, "Indexing had %d errors: %v", len(result.Errors), result.Errors)
	assert.Greater(t, result.ProcessedFiles, 0, "Expected at least 1 processed file")
	assert.True(t, result.IndexedFiles > 0 || result.UpdatedFiles > 0,
		"Expected at least 1 indexed or updated file")

	// Verify documents were actually stored in ChromaDB
	docCount, err := client.GetDocumentCount(ctx)
	require.NoError(t, err, "Failed to get document count")
	assert.Greater(t, docCount, 0, "Expected documents to be stored in ChromaDB, but found 0")

	t.Logf("Successfully indexed %d documents in ChromaDB", docCount)

	// Test search functionality
	searchResults, err := client.Query(ctx, "ChromaDB chunking", 3)
	require.NoError(t, err, "Failed to perform search query")

	// Extract documents from query result
	docGroups := searchResults.GetDocumentsGroups()
	require.NotEmpty(t, docGroups, "Expected search results, but got none")
	require.NotEmpty(t, docGroups[0], "Expected search results, but got none")

	documents := docGroups[0]
	t.Logf("Search returned %d results", len(documents))

	// Verify search results contain expected content
	foundRelevantContent := false
	for _, doc := range documents {
		content := strings.ToLower(doc.ContentString())
		if strings.Contains(content, "chroma") || strings.Contains(content, "chunk") {
			foundRelevantContent = true
			break
		}
	}

	assert.True(t, foundRelevantContent, "Search results don't contain expected relevant content")
}

// setupTestChromaDB starts a ChromaDB container for testing
func setupTestChromaDB(t *testing.T) error {
	t.Helper()

	// Clean up any existing test container
	cleanupTestChromaDB(t)

	// Ensure test data directory exists
	if err := os.MkdirAll(testDataDir, 0755); err != nil {
		return fmt.Errorf("failed to create test data directory: %w", err)
	}

	// Get absolute path for volume mounting
	absDataDir, err := filepath.Abs(testDataDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for data directory: %w", err)
	}

	// Find the config file in the project root (go up from internal/indexer)
	configPath := filepath.Join("..", "..", "chroma-config.yaml")
	absConfigPath, err := filepath.Abs(configPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for config file: %w", err)
	}

	// Verify config file exists
	if _, err := os.Stat(absConfigPath); os.IsNotExist(err) {
		return fmt.Errorf("config file not found at %s", absConfigPath)
	}

	t.Logf("Starting test ChromaDB container on port %s...", testPort)

	// Start ChromaDB container with test configuration
	cmd := exec.Command("docker", "run", "-d", "--rm",
		"--name", testContainerName,
		"-p", fmt.Sprintf("%s:8000", testPort),
		"-v", fmt.Sprintf("%s:/chroma", absDataDir),
		"-v", fmt.Sprintf("%s:/config.yaml", absConfigPath),
		"chromadb/chroma")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to start ChromaDB container: %v, output: %s", err, string(output))
	}

	t.Logf("Started ChromaDB test container: %s", strings.TrimSpace(string(output)))
	return nil
}

// cleanupTestChromaDB stops and removes the test ChromaDB container
func cleanupTestChromaDB(t *testing.T) {
	t.Helper()

	// Stop the container if it exists
	cmd := exec.Command("docker", "stop", testContainerName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Container might not exist, which is fine
		t.Logf("Note: Could not stop test container (may not exist): %v, output: %s", err, string(output))
	} else {
		t.Logf("Stopped test ChromaDB container")
	}

	// Clean up test data directory
	if err := os.RemoveAll(testDataDir); err != nil {
		t.Logf("Warning: Could not remove test data directory: %v", err)
	}
}

// waitForChromaDB waits for the ChromaDB container to be ready
func waitForChromaDB(t *testing.T) error {
	t.Helper()

	maxRetries := 30
	retryInterval := 1 * time.Second

	for i := 0; i < maxRetries; i++ {
		// Try to create a client and test connection
		config := &chroma.Config{
			Host:           "localhost",
			Port:           8038,
			CollectionName: testCollectionName,
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		client, err := chroma.NewClient(ctx, config)
		if err == nil {
			// Test if we can actually connect
			_, err = client.GetCollections(ctx)
			if err == nil {
				cancel()
				t.Logf("ChromaDB is ready after %d attempts", i+1)
				return nil
			}
		}
		cancel()

		if i == maxRetries-1 {
			return fmt.Errorf("ChromaDB did not become ready after %d attempts", maxRetries)
		}

		t.Logf("Waiting for ChromaDB to be ready... (attempt %d/%d)", i+1, maxRetries)
		time.Sleep(retryInterval)
	}

	return fmt.Errorf("ChromaDB failed to start within timeout")
}

// isDockerAvailable checks if Docker is available on the system
func isDockerAvailable() bool {
	cmd := exec.Command("docker", "--version")
	return cmd.Run() == nil
}
