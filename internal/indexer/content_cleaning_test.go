package indexer

import (
	"testing"
)

func TestCleanContent(t *testing.T) {
	// Create a dummy indexer for testing
	indexer := &ObsidianIndexer{}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Remove YAML frontmatter",
			input:    "---\ntitle: Test\ndate: 2023-01-01\n---\n\nThis is content",
			expected: "This is content",
		},
		{
			name:     "Remove standalone HTTP URLs",
			input:    "Check out this link: https://example.com/path?param=value",
			expected: "Check out this link:",
		},
		{
			name:     "Remove standalone HTTPS URLs",
			input:    "Visit https://www.google.com for search",
			expected: "Visit for search",
		},
		{
			name:     "Remove markdown links but keep text",
			input:    "Read [Good Strategy Bad Strategy](https://example.com/book)",
			expected: "Read Good Strategy Bad Strategy",
		},
		{
			name:     "Remove Obsidian wikilinks but keep text",
			input:    "See [[Strategy]] for more info",
			expected: "See Strategy for more info",
		},
		{
			name:     "Complex real-world example from Strategy Books",
			input:    `11. [**Good Strategy Bad Strategy** by Richard Rumelt](https://revopsteam.com/revops-strategy/business-strategy-books/#good-strategy-bad-strategy)
12. [**Blue Ocean Strategy** by W. Chan Kim and Renée Mauborgne](https://revopsteam.com/revops-strategy/business-strategy-books/#blue-ocean-strategy)`,
			expected: "11. **Good Strategy Bad Strategy** by Richard Rumelt 12. **Blue Ocean Strategy** by W. Chan Kim and Renée Mauborgne",
		},
		{
			name:     "Remove multiple URLs in one line",
			input:    "Links: https://example1.com and https://example2.com/path",
			expected: "Links: and",
		},
		{
			name:     "Preserve text without URLs or links",
			input:    "This is normal text with no special formatting.",
			expected: "This is normal text with no special formatting.",
		},
		{
			name:     "Normalize excessive whitespace",
			input:    "Too    much     whitespace\n\n\nhere",
			expected: "Too much whitespace here",
		},
		{
			name:     "Mixed content with frontmatter, URLs, and links",
			input:    `---
title: Test Page
---

# Header

Check out [Example](https://example.com) and visit https://google.com

Also see [[Internal Link]] for more.`,
			expected: "# Header Check out Example and visit Also see Internal Link for more.",
		},
		{
			name:     "URLs with special characters and fragments",
			input:    "Link: https://revopsteam.com/revops-strategy/business-strategy-books/#strategic-decisions-the-30-most-useful-models",
			expected: "Link:",
		},
		{
			name:     "Empty content",
			input:    "",
			expected: "",
		},
		{
			name:     "Only frontmatter",
			input:    "---\ntitle: Empty\n---",
			expected: "",
		},
		{
			name:     "URL at end of parenthesis",
			input:    "See the link (https://example.com) for details",
			expected: "See the link () for details",
		},
		{
			name:     "Multiple wikilinks",
			input:    "See [[Strategy]] and [[Business]] and [[Management]]",
			expected: "See Strategy and Business and Management",
		},
		{
			name:     "Nested markdown formatting",
			input:    "Read [**Bold Link Text**](https://example.com/path)",
			expected: "Read **Bold Link Text**",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := indexer.cleanContent(tt.input)
			if result != tt.expected {
				t.Errorf("cleanContent() = %q, expected %q", result, tt.expected)
			}
		})
	}
}

func TestCleanContentEdgeCases(t *testing.T) {
	indexer := &ObsidianIndexer{}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Malformed markdown link",
			input:    "Broken [link without closing paren](https://example.com",
			expected: "Broken [link without closing paren](",
		},
		{
			name:     "Malformed wikilink",
			input:    "Broken [[link without closing brackets",
			expected: "Broken [[link without closing brackets",
		},
		{
			name:     "URL with no protocol",
			input:    "Visit www.example.com for info",
			expected: "Visit www.example.com for info",
		},
		{
			name:     "FTP URL (should not be removed)",
			input:    "Download from ftp://example.com/file.zip",
			expected: "Download from ftp://example.com/file.zip",
		},
		{
			name:     "Email addresses (should not be removed)",
			input:    "Contact user@example.com for support",
			expected: "Contact user@example.com for support",
		},
		{
			name:     "URLs with Unicode characters",
			input:    "Visit https://example.com/café for more info",
			expected: "Visit for more info",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := indexer.cleanContent(tt.input)
			if result != tt.expected {
				t.Errorf("cleanContent() = %q, expected %q", result, tt.expected)
			}
		})
	}
}

func TestCleanContentPreservesSemanticMeaning(t *testing.T) {
	indexer := &ObsidianIndexer{}

	// Test that the cleaning preserves the semantic meaning of the Strategy Books content
	input := `# Strategy Books

1. "Competitive Strategy" by Michael E. Porter: Classic strategy book.
2. [**Good Strategy Bad Strategy** by Richard Rumelt](https://revopsteam.com/revops-strategy/business-strategy-books/#good-strategy-bad-strategy)

See also [[Strategy]] for related topics.`

	expected := `# Strategy Books 1. "Competitive Strategy" by Michael E. Porter: Classic strategy book. 2. **Good Strategy Bad Strategy** by Richard Rumelt See also Strategy for related topics.`

	result := indexer.cleanContent(input)
	if result != expected {
		t.Errorf("cleanContent() did not preserve semantic meaning:\nGot: %q\nExpected: %q", result, expected)
	}

	// Ensure the result doesn't contain any URLs
	if containsURL(result) {
		t.Errorf("cleanContent() result still contains URLs: %q", result)
	}
}

// Helper function to check if text contains URLs
func containsURL(text string) bool {
	return len(text) > 7 && (
		contains(text, "http://") ||
		contains(text, "https://"))
}

// Helper function since strings.Contains might not be imported
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}