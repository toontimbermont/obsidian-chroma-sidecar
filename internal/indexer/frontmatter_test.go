package indexer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestFrontmatterExtraction tests the extraction and parsing of YAML frontmatter
func TestFrontmatterExtraction(t *testing.T) {
	indexer := &ObsidianIndexer{}

	tests := []struct {
		name           string
		content        string
		expectedMatter map[string]interface{}
		expectedBody   string
	}{
		{
			name: "complete frontmatter",
			content: `202508181928
Categories: [[Sustainable IT]] [[Software]]
Tags: development, testing, agile

---

# Main Content

This is the main document content.`,
			expectedMatter: map[string]interface{}{
				"created_date": "202508181928",
				"categories":   []string{"Sustainable IT", "Software"},
				"tags":         []string{"development", "testing", "agile"},
			},
			expectedBody: "# Main Content\n\nThis is the main document content.",
		},
		{
			name: "minimal frontmatter",
			content: `202508181928
Categories: [[Sustainable IT]]

---

# Content

Some content here.`,
			expectedMatter: map[string]interface{}{
				"created_date": "202508181928",
				"categories":   []string{"Sustainable IT"},
			},
			expectedBody: "# Content\n\nSome content here.",
		},
		{
			name: "no frontmatter",
			content: `# Just Content

No frontmatter in this document.`,
			expectedMatter: map[string]interface{}{},
			expectedBody:   "# Just Content\n\nNo frontmatter in this document.",
		},
		{
			name: "frontmatter without separator",
			content: `202508181928
Categories: [[Software]]
# Content immediately follows

No separator line.`,
			expectedMatter: map[string]interface{}{},
			expectedBody:   "202508181928\nCategories: [[Software]]\n# Content immediately follows\n\nNo separator line.",
		},
		{
			name: "malformed categories",
			content: `202508181928
Categories: incomplete [[bracket
Tags: normal, tags

---

# Content`,
			expectedMatter: map[string]interface{}{
				"created_date": "202508181928",
				"tags":         []string{"normal", "tags"},
			},
			expectedBody: "# Content",
		},
		{
			name: "empty categories and tags",
			content: `202508181928
Categories: 
Tags:

---

# Content`,
			expectedMatter: map[string]interface{}{
				"created_date": "202508181928",
			},
			expectedBody: "# Content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matter, body := indexer.extractFrontmatter(tt.content)

			assert.Equal(t, tt.expectedMatter, matter, "frontmatter extraction mismatch")
			assert.Equal(t, tt.expectedBody, body, "body extraction mismatch")
		})
	}
}

// TestFrontmatterToContent tests conversion of frontmatter to readable content
func TestFrontmatterToContent(t *testing.T) {
	indexer := &ObsidianIndexer{}

	tests := []struct {
		name         string
		frontmatter  map[string]interface{}
		expectedText string
	}{
		{
			name: "complete frontmatter",
			frontmatter: map[string]interface{}{
				"created_date": "202508181928",
				"categories":   []string{"Sustainable IT", "Software"},
				"tags":         []string{"development", "testing"},
			},
			expectedText: "This document covers Sustainable IT and Software topics. Tags: development, testing.",
		},
		{
			name: "categories only",
			frontmatter: map[string]interface{}{
				"categories": []string{"Project Management"},
			},
			expectedText: "This document covers Project Management topics.",
		},
		{
			name: "tags only",
			frontmatter: map[string]interface{}{
				"tags": []string{"agile", "scrum"},
			},
			expectedText: "Tags: agile, scrum.",
		},
		{
			name:         "empty frontmatter",
			frontmatter:  map[string]interface{}{},
			expectedText: "",
		},
		{
			name: "single category",
			frontmatter: map[string]interface{}{
				"categories": []string{"AI"},
			},
			expectedText: "This document covers AI topics.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := indexer.frontmatterToContent(tt.frontmatter)
			assert.Equal(t, tt.expectedText, result, "frontmatter to content conversion mismatch")
		})
	}
}

// TestExtractFolderCategories tests extraction of categories from file paths
func TestExtractFolderCategories(t *testing.T) {
	// Create indexer with test configuration
	config := &Config{
		VaultPath:   "/home/user/vault",
		Directories: []string{"Projects", "Zettelkasten", "Notes"},
	}
	indexer := NewObsidianIndexer(NewMockChromaClient(), config)

	tests := []struct {
		name               string
		filePath           string
		expectedCategories []string
	}{
		{
			name:               "projects with single subfolder",
			filePath:           "/home/user/vault/Projects/Athumi/Gesprek.md",
			expectedCategories: []string{"Athumi"},
		},
		{
			name:               "zettelkasten with single subfolder",
			filePath:           "/home/user/vault/Zettelkasten/Strategy/Books.md",
			expectedCategories: []string{"Strategy"},
		},
		{
			name:               "deep nested path - all folders should be categories",
			filePath:           "/home/user/vault/Projects/ClientA/SubProject/Planning/document.md",
			expectedCategories: []string{"ClientA", "SubProject", "Planning"},
		},
		{
			name:               "very deep nesting",
			filePath:           "/home/user/vault/Notes/Research/AI/MachineLearning/DeepLearning/CNNs/document.md",
			expectedCategories: []string{"Research", "AI", "MachineLearning", "DeepLearning", "CNNs"},
		},
		{
			name:               "no subfolder - file directly in configured directory",
			filePath:           "/home/user/vault/Projects/document.md",
			expectedCategories: []string{},
		},
		{
			name:               "file outside configured directories",
			filePath:           "/home/user/vault/Other/subfolder/file.md",
			expectedCategories: []string{},
		},
		{
			name:               "relative path from within vault",
			filePath:           "Projects/ClientA/SubProject/file.md",
			expectedCategories: []string{"ClientA", "SubProject"},
		},
		{
			name:               "top level file",
			filePath:           "/home/user/vault/README.md",
			expectedCategories: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := indexer.extractFolderCategories(tt.filePath)
			assert.Equal(t, tt.expectedCategories, result, "folder categories extraction mismatch")
		})
	}
}

// TestEnhanceContentWithFrontmatter tests the integration of frontmatter enhancement
func TestEnhanceContentWithFrontmatter(t *testing.T) {
	// Create indexer with test configuration
	config := &Config{
		VaultPath:   "/home/user/vault",
		Directories: []string{"Projects", "Zettelkasten", "Notes"},
	}
	indexer := NewObsidianIndexer(NewMockChromaClient(), config)

	tests := []struct {
		name             string
		originalContent  string
		filePath         string
		expectedContent  string
		expectedMetadata map[string]interface{}
	}{
		{
			name: "complete integration with vault path",
			originalContent: `202508181928
Categories: [[Sustainable IT]] [[Software]]
Tags: development, testing

---

# Main Content

This is about sustainable software development.`,
			filePath:        "/home/user/vault/Projects/GreenTech/SubCategory/sustainability.md",
			expectedContent: "This document covers Sustainable IT, Software, GreenTech, and SubCategory topics. Tags: development, testing.\n\n# Main Content\n\nThis is about sustainable software development.",
			expectedMetadata: map[string]interface{}{
				"created_date": "202508181928",
				"categories":   []string{"Sustainable IT", "Software", "GreenTech", "SubCategory"},
				"tags":         []string{"development", "testing"},
			},
		},
		{
			name: "no frontmatter with folder",
			originalContent: `# Simple Document

Just some content here.`,
			filePath:        "Zettelkasten/Research/notes.md",
			expectedContent: "This document covers Research topics.\n\n# Simple Document\n\nJust some content here.",
			expectedMetadata: map[string]interface{}{
				"categories": []string{"Research"},
			},
		},
		{
			name: "frontmatter with no folder category",
			originalContent: `202508181928
Categories: [[AI]]

---

# AI Research`,
			filePath:        "notes.md",
			expectedContent: "This document covers AI topics.\n\n# AI Research",
			expectedMetadata: map[string]interface{}{
				"created_date": "202508181928",
				"categories":   []string{"AI"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enhancedContent, metadata := indexer.enhanceContentWithFrontmatter(tt.originalContent, tt.filePath)

			assert.Equal(t, tt.expectedContent, enhancedContent, "enhanced content mismatch")
			assert.Equal(t, tt.expectedMetadata, metadata, "metadata mismatch")
		})
	}
}

// TestConvertMetadataValue tests ChromaDB metadata value conversion
func TestConvertMetadataValue(t *testing.T) {
	indexer := &ObsidianIndexer{}

	tests := []struct {
		name     string
		input    interface{}
		expected interface{}
	}{
		{
			name:     "string slice to comma-separated string",
			input:    []string{"Software", "AI", "Testing"},
			expected: "Software, AI, Testing",
		},
		{
			name:     "single string in slice",
			input:    []string{"Software"},
			expected: "Software",
		},
		{
			name:     "empty string slice",
			input:    []string{},
			expected: "",
		},
		{
			name:     "regular string unchanged",
			input:    "regular string",
			expected: "regular string",
		},
		{
			name:     "number unchanged",
			input:    42,
			expected: 42,
		},
		{
			name:     "boolean unchanged",
			input:    true,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := indexer.convertMetadataValue(tt.input)
			assert.Equal(t, tt.expected, result, "metadata value conversion mismatch")
		})
	}
}
