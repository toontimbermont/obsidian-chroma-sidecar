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
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"

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
	directories  []string
	indexFile    string
	fileIndex    map[string]FileIndex
	chunkSize    int
	chunkOverlap int
}

// Config holds configuration for the Obsidian indexer
type Config struct {
	VaultPath    string
	BatchSize    int
	Directories  []string
	ChunkSize    int // Target chunk size in characters (default: 2000)
	ChunkOverlap int // Overlap between chunks in characters (default: 200)
}

// DefaultConfig returns default indexer configuration
func DefaultConfig() *Config {
	return &Config{
		VaultPath:    ".",
		BatchSize:    50,
		Directories:  []string{"notes", "projects"},
		ChunkSize:    2000,
		ChunkOverlap: 200,
	}
}

// NewObsidianIndexer creates a new Obsidian indexer
func NewObsidianIndexer(client ChromaClient, config *Config) *ObsidianIndexer {
	indexer := &ObsidianIndexer{
		client:       client,
		batchSize:    config.BatchSize,
		vaultPath:    config.VaultPath,
		directories:  config.Directories,
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
	batchFiles := make([]string, 0, idx.batchSize) // Track files in current batch

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
			result.Errors = append(result.Errors, fmt.Errorf("failed to process file %s: %w", file, err))
			continue
		}

		if len(chunks) == 0 {
			log.Printf("Skipping file %s: no content chunks generated", file)
			continue // Skip empty or invalid files
		}

		// Update metadata for each chunk
		for i := range chunks {
			chunks[i].Metadata["last_modified"] = fileInfo.ModTime().Unix()
			chunks[i].Metadata["content_hash"] = fileInfo.ContentHash
		}

		documents = append(documents, chunks...)
		batchFiles = append(batchFiles, file) // Track which file contributed to this batch

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
				result.Errors = append(result.Errors, fmt.Errorf("failed to upsert batch containing files %v: %w", batchFiles, err))
			} else {
				result.BatchesUploaded++
				log.Printf("Upserted batch of %d documents from %d files: %v", len(documents), len(batchFiles), batchFiles)
			}
			documents = documents[:0]   // Reset slice
			batchFiles = batchFiles[:0] // Reset file tracking
		}
	}

	// Upload remaining documents
	if len(documents) > 0 {
		if err := idx.client.UpsertDocuments(ctx, documents); err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("failed to upsert final batch containing files %v: %w", batchFiles, err))
		} else {
			result.BatchesUploaded++
			log.Printf("Upserted final batch of %d documents from %d files: %v", len(documents), len(batchFiles), batchFiles)
		}
	}

	// Save updated file index
	if err := idx.saveFileIndex(); err != nil {
		result.Errors = append(result.Errors, fmt.Errorf("failed to save file index: %w", err))
	}

	log.Printf("Indexing complete. Processed: %d, New: %d, Updated: %d, Skipped: %d, Batches: %d, Errors: %d",
		result.ProcessedFiles, result.IndexedFiles, result.UpdatedFiles, result.SkippedFiles, result.BatchesUploaded, len(result.Errors))

	// Log detailed error information if there were any failures
	if len(result.Errors) > 0 {
		log.Printf("Indexing errors encountered:")
		for i, err := range result.Errors {
			log.Printf("  Error %d: %v", i+1, err)
		}
	}

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
	
	// Normalize Unicode characters in the file path to ensure consistent ID generation
	// This prevents issues with files that have accented characters in their names
	normalizedPath := normalizeUnicode(cleanPath)

	// Create MD5 hash of the normalized path and chunk index for consistent ID generation
	chunkKey := fmt.Sprintf("%s_chunk_%d", normalizedPath, chunkIndex)
	hash := md5.Sum([]byte(chunkKey))
	return fmt.Sprintf("%x", hash)
}

// normalizeUnicode converts Unicode accented characters to their ASCII equivalents
// This ensures consistent tokenization across different languages and prevents
// tensor shape mismatches in ChromaDB embedding
func normalizeUnicode(text string) string {
	// Use Go's standard unicode normalization to remove diacritics (accents)
	// This transforms accented characters like 'é', 'è', 'ë' to their base form 'e'
	// 
	// The process:
	// 1. NFD (Normalization Form Decomposed) - separates base characters from combining marks
	// 2. RemoveFunc - removes nonspacing marks (accents, diacritics) 
	// 3. NFC (Normalization Form Composed) - recomposes the remaining characters
	
	isMn := func(r rune) bool {
		return unicode.Is(unicode.Mn, r) // Mn: nonspacing marks (diacritics)
	}
	
	t := transform.Chain(norm.NFD, transform.RemoveFunc(isMn), norm.NFC)
	result, _, err := transform.String(t, text)
	if err != nil {
		// Fallback to original text if transformation fails
		return text
	}
	
	return result
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

	// Extract frontmatter and enhance content before processing
	enhancedContent, frontmatterMetadata := idx.enhanceContentWithFrontmatter(contentStr, filePath)

	// Clean enhanced content before chunking
	cleanedContent := idx.cleanContent(enhancedContent)

	// Split content into chunks
	chunks := idx.chunkContent(cleanedContent, filePath)

	// Add frontmatter metadata to each chunk
	for i := range chunks {
		// Merge frontmatter metadata with existing chunk metadata
		for key, value := range frontmatterMetadata {
			// Convert arrays to strings for ChromaDB compatibility
			chunks[i].Metadata[key] = idx.convertMetadataValue(value)
		}
	}

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

	// Minimum meaningful chunk size to prevent tiny chunks at boundaries
	minChunkSize := 50 // Minimum 50 characters for meaningful content

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

		// Calculate next start position with overlap
		nextStart := end - overlap

		// Prevent cascading tiny chunks at document boundaries
		if nextStart <= start {
			// If overlap would cause us to go backwards or stay in place,
			// check if the remaining content is small enough to append to current chunk
			remainingContent := len(content) - end
			if remainingContent <= minChunkSize && len(chunks) > 0 {
				// Append remaining small content to the last chunk
				remainingText := strings.TrimSpace(content[end:])
				if len(remainingText) > 0 {
					lastChunk := chunks[len(chunks)-1]
					chunks[len(chunks)-1] = lastChunk + " " + remainingText
				}
				break
			}
			// Otherwise ensure we make minimal progress (but still prevent infinite loop)
			nextStart = start + 1
		}

		// Additional check: if remaining content would create cascading tiny chunks, stop
		remainingFromNext := len(content) - nextStart
		if remainingFromNext <= minChunkSize {
			// Append remaining content to the last chunk instead of creating tiny chunks
			if len(chunks) > 0 && remainingFromNext > 0 {
				lastChunk := chunks[len(chunks)-1]
				remainingText := strings.TrimSpace(content[nextStart:])
				if len(remainingText) > 0 {
					// Only append if it's not already included (avoid duplication)
					if !strings.HasSuffix(lastChunk, remainingText) {
						chunks[len(chunks)-1] = lastChunk + " " + remainingText
					}
				}
			}
			break
		}

		start = nextStart

		// Safety check: if we're at the end, break
		if start >= len(content) {
			break
		}
	}

	return chunks
}

// extractFrontmatter parses Obsidian-style frontmatter and returns structured data plus body content
func (idx *ObsidianIndexer) extractFrontmatter(content string) (map[string]interface{}, string) {
	frontmatter := make(map[string]interface{})

	// Check if content starts with frontmatter (no YAML --- markers in Obsidian style)
	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		return frontmatter, content
	}

	separatorIndex := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == "---" {
			separatorIndex = i
			break
		}
	}

	// If no separator found, treat entire content as body
	if separatorIndex == -1 {
		return frontmatter, content
	}

	// Parse frontmatter lines
	for i := 0; i < separatorIndex; i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}

		// Parse created date (first line if it's just numbers)
		if i == 0 && regexp.MustCompile(`^\d+$`).MatchString(line) {
			frontmatter["created_date"] = line
			continue
		}

		// Parse key-value pairs
		if strings.Contains(line, ":") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(strings.ToLower(parts[0]))
				value := strings.TrimSpace(parts[1])

				switch key {
				case "categories":
					if categories := idx.parseCategories(value); len(categories) > 0 {
						frontmatter["categories"] = categories
					}
				case "tags":
					if tags := idx.parseTags(value); len(tags) > 0 {
						frontmatter["tags"] = tags
					}
				}
			}
		}
	}

	// Extract body content (everything after separator)
	bodyLines := lines[separatorIndex+1:]
	body := strings.Join(bodyLines, "\n")
	body = strings.TrimSpace(body)

	return frontmatter, body
}

// parseCategories extracts categories from Obsidian-style [[Category]] format
func (idx *ObsidianIndexer) parseCategories(value string) []string {
	var categories []string

	// Match [[Category Name]] patterns
	categoryRegex := regexp.MustCompile(`\[\[([^\]]+)\]\]`)
	matches := categoryRegex.FindAllStringSubmatch(value, -1)

	for _, match := range matches {
		if len(match) > 1 {
			category := strings.TrimSpace(match[1])
			if category != "" {
				categories = append(categories, category)
			}
		}
	}

	return categories
}

// parseTags extracts tags from comma-separated format
func (idx *ObsidianIndexer) parseTags(value string) []string {
	var tags []string

	if strings.TrimSpace(value) == "" {
		return tags
	}

	// Split by comma and clean up
	parts := strings.Split(value, ",")
	for _, part := range parts {
		tag := strings.TrimSpace(part)
		if tag != "" {
			tags = append(tags, tag)
		}
	}

	return tags
}

// frontmatterToContent converts frontmatter metadata to readable content
func (idx *ObsidianIndexer) frontmatterToContent(frontmatter map[string]interface{}) string {
	var parts []string

	// Add categories if present
	if categories, ok := frontmatter["categories"].([]string); ok && len(categories) > 0 {
		if len(categories) == 1 {
			parts = append(parts, fmt.Sprintf("This document covers %s topics.", categories[0]))
		} else if len(categories) == 2 {
			parts = append(parts, fmt.Sprintf("This document covers %s and %s topics.", categories[0], categories[1]))
		} else {
			// For 3+ categories: "A, B, and C"
			categoryList := strings.Join(categories[:len(categories)-1], ", ")
			parts = append(parts, fmt.Sprintf("This document covers %s, and %s topics.", categoryList, categories[len(categories)-1]))
		}
	}

	// Add tags if present
	if tags, ok := frontmatter["tags"].([]string); ok && len(tags) > 0 {
		tagList := strings.Join(tags, ", ")
		parts = append(parts, fmt.Sprintf("Tags: %s.", tagList))
	}

	return strings.Join(parts, " ")
}

// extractFolderCategories extracts ALL folder levels as categories from file path using vault configuration
func (idx *ObsidianIndexer) extractFolderCategories(filePath string) []string {
	categories := make([]string, 0) // Initialize as empty slice, not nil

	// Clean the path and make it absolute if it's relative
	cleanPath := filepath.Clean(filePath)
	if !filepath.IsAbs(cleanPath) {
		cleanPath = filepath.Join(idx.vaultPath, cleanPath)
	}

	// Convert vault path to absolute for comparison
	vaultPath := filepath.Clean(idx.vaultPath)
	if !filepath.IsAbs(vaultPath) {
		// Convert relative vault path to absolute
		if absVaultPath, err := filepath.Abs(vaultPath); err == nil {
			vaultPath = absVaultPath
		}
	}

	// Check if file is within vault
	relPath, err := filepath.Rel(vaultPath, cleanPath)
	if err != nil || strings.HasPrefix(relPath, "..") {
		// File is outside vault
		return categories
	}

	// Split the relative path into parts
	parts := strings.Split(relPath, string(filepath.Separator))

	// Remove empty parts and filename
	var cleanParts []string
	for i, part := range parts {
		if part != "" && part != "." && i < len(parts)-1 { // Exclude filename (last part)
			cleanParts = append(cleanParts, part)
		}
	}

	// If no folders, return empty
	if len(cleanParts) == 0 {
		return categories
	}

	// Check if first folder is one of our configured directories
	firstFolder := cleanParts[0]
	isConfiguredDir := false
	for _, dir := range idx.directories {
		if strings.EqualFold(firstFolder, dir) {
			isConfiguredDir = true
			break
		}
	}

	// If file is in a configured directory, extract ALL folder levels after it
	if isConfiguredDir && len(cleanParts) > 1 {
		// Add all folders between configured directory and file as categories
		categories = append(categories, cleanParts[1:]...)
	}

	return categories
}

// enhanceContentWithFrontmatter combines frontmatter extraction, folder categories, and content enhancement
func (idx *ObsidianIndexer) enhanceContentWithFrontmatter(content string, filePath string) (string, map[string]interface{}) {
	// Extract frontmatter from content
	frontmatter, bodyContent := idx.extractFrontmatter(content)

	// Extract folder-based categories
	folderCategories := idx.extractFolderCategories(filePath)

	// Merge folder categories with frontmatter categories
	var allCategories []string
	if fmCategories, ok := frontmatter["categories"].([]string); ok {
		allCategories = append(allCategories, fmCategories...)
	}
	allCategories = append(allCategories, folderCategories...)

	// Update frontmatter with combined categories
	if len(allCategories) > 0 {
		frontmatter["categories"] = allCategories
	}

	// Convert frontmatter to readable content
	frontmatterContent := idx.frontmatterToContent(frontmatter)

	// Combine frontmatter content with body content
	var enhancedContent string
	if frontmatterContent != "" && bodyContent != "" {
		enhancedContent = frontmatterContent + "\n\n" + bodyContent
	} else if frontmatterContent != "" {
		enhancedContent = frontmatterContent
	} else {
		enhancedContent = bodyContent
	}

	return enhancedContent, frontmatter
}

// convertMetadataValue converts array values to strings for ChromaDB compatibility
func (idx *ObsidianIndexer) convertMetadataValue(value interface{}) interface{} {
	// Convert []string to comma-separated string for ChromaDB compatibility
	if strSlice, ok := value.([]string); ok {
		if len(strSlice) == 0 {
			return ""
		}
		return strings.Join(strSlice, ", ")
	}

	// Return other types as-is (strings, numbers, booleans are supported by ChromaDB)
	return value
}

// cleanContent removes URLs and other problematic content that can cause tokenization issues
func (idx *ObsidianIndexer) cleanContent(content string) string {
	// Remove dataview blocks (```dataview ... ```)
	dataviewRegex := regexp.MustCompile(`(?s)` + "```dataview.*?```")
	content = dataviewRegex.ReplaceAllString(content, "")

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

	// Normalize Unicode characters to ASCII equivalents to ensure consistent tokenization
	// This prevents tensor shape mismatches in ChromaDB embedding
	content = normalizeUnicode(content)

	return content
}
