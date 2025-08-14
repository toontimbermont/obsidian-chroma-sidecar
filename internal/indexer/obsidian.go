package indexer

import (
	"context"
	"crypto/md5"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"obsidian-ai-agent/internal/chroma"
)

// ObsidianIndexer handles indexing of Obsidian markdown files
type ObsidianIndexer struct {
	client    *chroma.Client
	batchSize int
	vaultPath string
}

// Config holds configuration for the Obsidian indexer
type Config struct {
	VaultPath string
	BatchSize int
	Directories []string
}

// DefaultConfig returns default indexer configuration
func DefaultConfig() *Config {
	return &Config{
		VaultPath: ".",
		BatchSize: 50,
		Directories: []string{"notes", "projects"},
	}
}

// NewObsidianIndexer creates a new Obsidian indexer
func NewObsidianIndexer(client *chroma.Client, config *Config) *ObsidianIndexer {
	return &ObsidianIndexer{
		client:    client,
		batchSize: config.BatchSize,
		vaultPath: config.VaultPath,
	}
}

// IndexResult holds the result of an indexing operation
type IndexResult struct {
	ProcessedFiles int
	IndexedFiles   int
	Errors         []error
	BatchesUploaded int
}

// ReindexVault reindexes all markdown files in the specified directories
func (idx *ObsidianIndexer) ReindexVault(ctx context.Context, directories []string) (*IndexResult, error) {
	result := &IndexResult{
		Errors: make([]error, 0),
	}

	log.Println("Starting reindex of vault...")

	// Find all markdown files
	files, err := idx.findMarkdownFiles(directories)
	if err != nil {
		return result, fmt.Errorf("failed to find markdown files: %w", err)
	}

	log.Printf("Found %d markdown files", len(files))

	// Process files in batches
	documents := make([]chroma.Document, 0, idx.batchSize)
	
	for _, file := range files {
		result.ProcessedFiles++
		
		doc, err := idx.processFile(file)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("failed to process %s: %w", file, err))
			continue
		}
		
		if doc == nil {
			continue // Skip empty or invalid files
		}

		documents = append(documents, *doc)
		result.IndexedFiles++

		// Upload batch when full
		if len(documents) >= idx.batchSize {
			if err := idx.client.AddDocuments(ctx, documents); err != nil {
				result.Errors = append(result.Errors, fmt.Errorf("failed to upload batch: %w", err))
			} else {
				result.BatchesUploaded++
				log.Printf("Uploaded batch of %d documents", len(documents))
			}
			documents = documents[:0] // Reset slice
		}
	}

	// Upload remaining documents
	if len(documents) > 0 {
		if err := idx.client.AddDocuments(ctx, documents); err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("failed to upload final batch: %w", err))
		} else {
			result.BatchesUploaded++
			log.Printf("Uploaded final batch of %d documents", len(documents))
		}
	}

	log.Printf("Indexing complete. Processed: %d, Indexed: %d, Batches: %d, Errors: %d", 
		result.ProcessedFiles, result.IndexedFiles, result.BatchesUploaded, len(result.Errors))

	return result, nil
}

// findMarkdownFiles finds all .md files in the specified directories
func (idx *ObsidianIndexer) findMarkdownFiles(directories []string) ([]string, error) {
	var files []string

	for _, dir := range directories {
		dirPath := filepath.Join(idx.vaultPath, dir)
		
		// Check if directory exists
		if _, err := os.Stat(dirPath); os.IsNotExist(err) {
			log.Printf("Directory %s does not exist, skipping", dirPath)
			continue
		}

		err := filepath.WalkDir(dirPath, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			if !d.IsDir() && strings.HasSuffix(strings.ToLower(path), ".md") {
				files = append(files, path)
			}

			return nil
		})

		if err != nil {
			return nil, fmt.Errorf("failed to walk directory %s: %w", dirPath, err)
		}
	}

	return files, nil
}

// processFile processes a single markdown file
func (idx *ObsidianIndexer) processFile(filePath string) (*chroma.Document, error) {
	// Read file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Skip empty files
	if len(content) == 0 {
		return nil, nil
	}

	// Ensure content is valid UTF-8
	contentStr := string(content)
	if !utf8.ValidString(contentStr) {
		// Try to clean invalid UTF-8
		contentStr = strings.ToValidUTF8(contentStr, "")
	}

	// Skip files that are too short
	if len(strings.TrimSpace(contentStr)) < 10 {
		return nil, nil
	}

	// Generate document ID from file path
	docID := generateDocumentID(filePath)

	// Extract metadata
	metadata := map[string]interface{}{
		"path":     filePath,
		"filename": filepath.Base(filePath),
		"folder":   filepath.Dir(filePath),
	}

	return &chroma.Document{
		ID:       docID,
		Content:  contentStr,
		Metadata: metadata,
	}, nil
}

// generateDocumentID creates a unique ID for a document based on its file path
func generateDocumentID(filePath string) string {
	// Clean and normalize the path
	cleanPath := filepath.Clean(filePath)
	
	// Create MD5 hash of the path for consistent ID generation
	hash := md5.Sum([]byte(cleanPath))
	return fmt.Sprintf("%x", hash)
}