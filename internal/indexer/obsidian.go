package indexer

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"obsidian-ai-agent/internal/chroma"
)

// ChromaClient defines the interface for ChromaDB operations used by the indexer
type ChromaClient interface {
	UpsertDocuments(ctx context.Context, documents []chroma.Document) error
}

// FileIndex represents metadata about an indexed file
type FileIndex struct {
	Path         string    `json:"path"`
	LastModified time.Time `json:"last_modified"`
	ContentHash  string    `json:"content_hash"`
	DocumentID   string    `json:"document_id"`
	LastIndexed  time.Time `json:"last_indexed"`
}

// ObsidianIndexer handles indexing of Obsidian markdown files
type ObsidianIndexer struct {
	client       ChromaClient
	batchSize    int
	vaultPath    string
	indexFile    string
	fileIndex    map[string]FileIndex
	chunkSize    int
	chunkOverlap int
}

// Config holds configuration for the Obsidian indexer
type Config struct {
	VaultPath string
	BatchSize int
	Directories []string
	ChunkSize int // Target chunk size in characters (default: 2000)
	ChunkOverlap int // Overlap between chunks in characters (default: 200)
}

// DefaultConfig returns default indexer configuration
func DefaultConfig() *Config {
	return &Config{
		VaultPath: ".",
		BatchSize: 50,
		Directories: []string{"notes", "projects"},
		ChunkSize: 2000,
		ChunkOverlap: 200,
	}
}

// NewObsidianIndexer creates a new Obsidian indexer
func NewObsidianIndexer(client ChromaClient, config *Config) *ObsidianIndexer {
	indexer := &ObsidianIndexer{
		client:       client,
		batchSize:    config.BatchSize,
		vaultPath:    config.VaultPath,
		indexFile:    filepath.Join(config.VaultPath, ".obsidian_index.json"),
		fileIndex:    make(map[string]FileIndex),
		chunkSize:    config.ChunkSize,
		chunkOverlap: config.ChunkOverlap,
	}
	
	// Load existing index
	indexer.loadFileIndex()
	
	return indexer
}

// IndexResult holds the result of an indexing operation
type IndexResult struct {
	ProcessedFiles  int
	IndexedFiles    int
	UpdatedFiles    int
	SkippedFiles    int
	Errors          []error
	BatchesUploaded int
}

// ReindexVault performs incremental indexing of all markdown files in the specified directories
func (idx *ObsidianIndexer) ReindexVault(ctx context.Context, directories []string) (*IndexResult, error) {
	result := &IndexResult{
		Errors: make([]error, 0),
	}

	log.Println("Starting incremental reindex of vault...")

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
		
		// Check if file needs indexing
		needsIndexing, err := idx.fileNeedsIndexing(file)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("failed to check if file needs indexing %s: %w", file, err))
			continue
		}
		
		if !needsIndexing {
			result.SkippedFiles++
			continue
		}
		
		chunks, fileInfo, err := idx.processFileWithChunks(file)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("failed to process %s: %w", file, err))
			continue
		}
		
		if len(chunks) == 0 {
			continue // Skip empty or invalid files
		}

		// Update metadata for each chunk
		for i := range chunks {
			chunks[i].Metadata["last_modified"] = fileInfo.ModTime().Unix()
			chunks[i].Metadata["content_hash"] = fileInfo.ContentHash
		}
		
		documents = append(documents, chunks...)
		
		// Check if this is an update or new file
		if _, exists := idx.fileIndex[file]; exists {
			result.UpdatedFiles++
		} else {
			result.IndexedFiles++
		}
		
		// Update in-memory index (use first chunk's ID for tracking)
		idx.fileIndex[file] = FileIndex{
			Path:         file,
			LastModified: fileInfo.ModTime(),
			ContentHash:  fileInfo.ContentHash,
			DocumentID:   chunks[0].ID,
			LastIndexed:  time.Now(),
		}

		// Upload batch when full
		if len(documents) >= idx.batchSize {
			if err := idx.client.UpsertDocuments(ctx, documents); err != nil {
				result.Errors = append(result.Errors, fmt.Errorf("failed to upsert batch: %w", err))
			} else {
				result.BatchesUploaded++
				log.Printf("Upserted batch of %d documents", len(documents))
			}
			documents = documents[:0] // Reset slice
		}
	}

	// Upload remaining documents
	if len(documents) > 0 {
		if err := idx.client.UpsertDocuments(ctx, documents); err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("failed to upsert final batch: %w", err))
		} else {
			result.BatchesUploaded++
			log.Printf("Upserted final batch of %d documents", len(documents))
		}
	}

	// Save updated file index
	if err := idx.saveFileIndex(); err != nil {
		result.Errors = append(result.Errors, fmt.Errorf("failed to save file index: %w", err))
	}

	log.Printf("Indexing complete. Processed: %d, New: %d, Updated: %d, Skipped: %d, Batches: %d, Errors: %d", 
		result.ProcessedFiles, result.IndexedFiles, result.UpdatedFiles, result.SkippedFiles, result.BatchesUploaded, len(result.Errors))

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

// FileWithHash extends os.FileInfo with content hash
type FileWithHash struct {
	os.FileInfo
	ContentHash string
}

// loadFileIndex loads the file index from disk
func (idx *ObsidianIndexer) loadFileIndex() {
	data, err := os.ReadFile(idx.indexFile)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("Warning: failed to load file index: %v", err)
		}
		return
	}

	var fileMap map[string]FileIndex
	if err := json.Unmarshal(data, &fileMap); err != nil {
		log.Printf("Warning: failed to unmarshal file index: %v", err)
		return
	}

	idx.fileIndex = fileMap
	log.Printf("Loaded file index with %d entries", len(idx.fileIndex))
}

// saveFileIndex saves the file index to disk
func (idx *ObsidianIndexer) saveFileIndex() error {
	data, err := json.MarshalIndent(idx.fileIndex, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal file index: %w", err)
	}

	if err := os.WriteFile(idx.indexFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write file index: %w", err)
	}

	return nil
}

// fileNeedsIndexing checks if a file needs to be indexed based on modification time and content hash
func (idx *ObsidianIndexer) fileNeedsIndexing(filePath string) (bool, error) {
	// Get file info
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return false, fmt.Errorf("failed to stat file: %w", err)
	}

	// Check if file is in index
	indexEntry, exists := idx.fileIndex[filePath]
	if !exists {
		return true, nil // New file, needs indexing
	}

	// Check if modification time changed
	if !fileInfo.ModTime().Equal(indexEntry.LastModified) {
		return true, nil // File modified, needs re-indexing
	}

	// For files with same modification time, check content hash
	content, err := os.ReadFile(filePath)
	if err != nil {
		return false, fmt.Errorf("failed to read file for hash check: %w", err)
	}

	currentHash := fmt.Sprintf("%x", sha256.Sum256(content))
	if currentHash != indexEntry.ContentHash {
		return true, nil // Content changed, needs re-indexing
	}

	return false, nil // File unchanged, skip indexing
}

// processFileWithHash processes a file and returns the document with file info including content hash
func (idx *ObsidianIndexer) processFileWithHash(filePath string) (*chroma.Document, *FileWithHash, error) {
	// Read file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Get file info
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to stat file: %w", err)
	}

	// Calculate content hash
	contentHash := fmt.Sprintf("%x", sha256.Sum256(content))

	// Create FileWithHash
	fileWithHash := &FileWithHash{
		FileInfo:    fileInfo,
		ContentHash: contentHash,
	}

	// Skip empty files
	if len(content) == 0 {
		return nil, fileWithHash, nil
	}

	// Ensure content is valid UTF-8
	contentStr := string(content)
	if !utf8.ValidString(contentStr) {
		// Try to clean invalid UTF-8
		contentStr = strings.ToValidUTF8(contentStr, "")
	}

	// Skip files that are too short
	if len(strings.TrimSpace(contentStr)) < 10 {
		return nil, fileWithHash, nil
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
	}, fileWithHash, nil
}

// generateDocumentID creates a unique ID for a document based on its file path
func generateDocumentID(filePath string) string {
	// Clean and normalize the path
	cleanPath := filepath.Clean(filePath)
	
	// Create MD5 hash of the path for consistent ID generation
	hash := md5.Sum([]byte(cleanPath))
	return fmt.Sprintf("%x", hash)
}

// generateChunkID creates a unique ID for a chunk based on file path and chunk index
func generateChunkID(filePath string, chunkIndex int) string {
	// Clean and normalize the path
	cleanPath := filepath.Clean(filePath)
	
	// Create MD5 hash of the path and chunk index for consistent ID generation
	chunkKey := fmt.Sprintf("%s_chunk_%d", cleanPath, chunkIndex)
	hash := md5.Sum([]byte(chunkKey))
	return fmt.Sprintf("%x", hash)
}

// processFileWithChunks processes a file and returns chunks with file info including content hash
func (idx *ObsidianIndexer) processFileWithChunks(filePath string) ([]chroma.Document, *FileWithHash, error) {
	// Read file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Get file info
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to stat file: %w", err)
	}

	// Calculate content hash
	contentHash := fmt.Sprintf("%x", sha256.Sum256(content))

	// Create FileWithHash
	fileWithHash := &FileWithHash{
		FileInfo:    fileInfo,
		ContentHash: contentHash,
	}

	// Skip empty files
	if len(content) == 0 {
		return nil, fileWithHash, nil
	}

	// Ensure content is valid UTF-8
	contentStr := string(content)
	if !utf8.ValidString(contentStr) {
		// Try to clean invalid UTF-8
		contentStr = strings.ToValidUTF8(contentStr, "")
	}

	// Skip files that are too short
	if len(strings.TrimSpace(contentStr)) < 10 {
		return nil, fileWithHash, nil
	}

	// Clean content before chunking
	cleanedContent := idx.cleanContent(contentStr)
	
	// Split content into chunks
	chunks := idx.chunkContent(cleanedContent, filePath)
	
	return chunks, fileWithHash, nil
}

// chunkContent splits markdown content into semantic chunks
func (idx *ObsidianIndexer) chunkContent(content string, filePath string) []chroma.Document {
	var chunks []chroma.Document
	
	// First try to split by headers
	headerChunks := idx.splitByHeaders(content)
	
	for i, chunk := range headerChunks {
		// If chunk is still too large, split it further
		if len(chunk) > idx.chunkSize {
			subChunks := idx.splitBySize(chunk, idx.chunkSize, idx.chunkOverlap)
			for j, subChunk := range subChunks {
				chunkIndex := i*1000 + j // Ensure unique indexing
				doc := chroma.Document{
					ID:      generateChunkID(filePath, chunkIndex),
					Content: subChunk,
					Metadata: map[string]interface{}{
						"path":        filePath,
						"filename":    filepath.Base(filePath),
						"folder":      filepath.Dir(filePath),
						"chunk_index": chunkIndex,
						"chunk_type":  "sub_header",
					},
				}
				chunks = append(chunks, doc)
			}
		} else {
			doc := chroma.Document{
				ID:      generateChunkID(filePath, i),
				Content: chunk,
				Metadata: map[string]interface{}{
					"path":        filePath,
					"filename":    filepath.Base(filePath),
					"folder":      filepath.Dir(filePath),
					"chunk_index": i,
					"chunk_type":  "header",
				},
			}
			chunks = append(chunks, doc)
		}
	}
	
	// If no header-based chunks were created, fall back to size-based chunking
	if len(chunks) == 0 {
		sizeChunks := idx.splitBySize(content, idx.chunkSize, idx.chunkOverlap)
		for i, chunk := range sizeChunks {
			doc := chroma.Document{
				ID:      generateChunkID(filePath, i),
				Content: chunk,
				Metadata: map[string]interface{}{
					"path":        filePath,
					"filename":    filepath.Base(filePath),
					"folder":      filepath.Dir(filePath),
					"chunk_index": i,
					"chunk_type":  "size",
				},
			}
			chunks = append(chunks, doc)
		}
	}
	
	return chunks
}

// splitByHeaders splits content by markdown headers
func (idx *ObsidianIndexer) splitByHeaders(content string) []string {
	// Split by headers (# ## ### etc.)
	headerRegex := regexp.MustCompile(`(?m)^(#{1,6})\s+(.+)$`)
	
	// Find all header positions
	matches := headerRegex.FindAllStringIndex(content, -1)
	if len(matches) == 0 {
		// No headers found, return entire content
		return []string{content}
	}
	
	var chunks []string
	start := 0
	
	for i, match := range matches {
		// Add content before this header (if any)
		if match[0] > start {
			chunk := strings.TrimSpace(content[start:match[0]])
			if len(chunk) > 0 {
				chunks = append(chunks, chunk)
			}
		}
		
		// Determine the end of this section
		var end int
		if i+1 < len(matches) {
			end = matches[i+1][0] // Next header start
		} else {
			end = len(content) // End of content
		}
		
		// Add this header section
		chunk := strings.TrimSpace(content[match[0]:end])
		if len(chunk) > 0 {
			chunks = append(chunks, chunk)
		}
		
		start = end
	}
	
	return chunks
}

// splitBySize splits content into size-based chunks with overlap
func (idx *ObsidianIndexer) splitBySize(content string, chunkSize, overlap int) []string {
	if len(content) <= chunkSize {
		return []string{content}
	}
	
	var chunks []string
	start := 0
	
	for start < len(content) {
		end := start + chunkSize
		if end > len(content) {
			end = len(content)
		}
		
		// Try to break at word boundary
		if end < len(content) {
			// Look for last space within reasonable distance
			for i := end; i > end-100 && i > start; i-- {
				if content[i] == ' ' || content[i] == '\n' {
					end = i
					break
				}
			}
		}
		
		chunk := strings.TrimSpace(content[start:end])
		if len(chunk) > 0 {
			chunks = append(chunks, chunk)
		}
		
		// Move start position with overlap, ensuring progress
		nextStart := end - overlap
		if nextStart <= start {
			// Ensure we always make progress to avoid infinite loop
			nextStart = start + 1
		}
		start = nextStart
		
		// Safety check: if we're at the end, break
		if start >= len(content) {
			break
		}
	}
	
	return chunks
}

// cleanContent removes URLs and other problematic content that can cause tokenization issues
func (idx *ObsidianIndexer) cleanContent(content string) string {
	// Remove YAML frontmatter
	frontmatterRegex := regexp.MustCompile(`(?s)^---.*?---\s*`)
	content = frontmatterRegex.ReplaceAllString(content, "")
	
	// Remove markdown links but keep link text: [text](url) -> text
	// This must be done BEFORE removing standalone URLs
	markdownLinkRegex := regexp.MustCompile(`\[([^\]]+)\]\([^)]*\)`)
	content = markdownLinkRegex.ReplaceAllString(content, "$1")
	
	// Remove Obsidian wikilinks but keep link text: [[text]] -> text
	wikiLinkRegex := regexp.MustCompile(`\[\[([^\]]+)\]\]`)
	content = wikiLinkRegex.ReplaceAllString(content, "$1")
	
	// Remove standalone URLs (http/https) - after processing markdown links
	urlRegex := regexp.MustCompile(`https?://[^\s\)]+`)
	content = urlRegex.ReplaceAllString(content, "")
	
	// Remove excessive whitespace and normalize
	content = regexp.MustCompile(`\s+`).ReplaceAllString(content, " ")
	content = strings.TrimSpace(content)
	
	return content
}