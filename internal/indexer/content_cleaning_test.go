package indexer

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
			name: "Complex real-world example from Strategy Books",
			input: `11. [**Good Strategy Bad Strategy** by Richard Rumelt](https://revopsteam.com/revops-strategy/business-strategy-books/#good-strategy-bad-strategy)
12. [**Blue Ocean Strategy** by W. Chan Kim and Renée Mauborgne](https://revopsteam.com/revops-strategy/business-strategy-books/#blue-ocean-strategy)`,
			expected: "11. **Good Strategy Bad Strategy** by Richard Rumelt 12. **Blue Ocean Strategy** by W. Chan Kim and Renee Mauborgne",
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
			name: "Mixed content with frontmatter, URLs, and links",
			input: `---
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
		{
			name: "Remove simple dataview block",
			input: `Some content before

` + "```dataview" + `
TABLE file.name as Name, file.mtime as Modified
FROM ""
SORT file.mtime DESC
` + "```" + `

Some content after`,
			expected: "Some content before Some content after",
		},
		{
			name: "Remove multiple dataview blocks",
			input: `First section

` + "```dataview" + `
LIST
FROM ""
` + "```" + `

Middle section

` + "```dataview" + `
TABLE file.name
FROM ""
WHERE contains(file.path, "project")
` + "```" + `

Last section`,
			expected: "First section Middle section Last section",
		},
		{
			name: "Remove dataview block with complex content",
			input: `# My Notes

` + "```dataview" + `
TABLE WITHOUT ID
	file.link as "Note",
	file.mtime as "Modified",
	length(file.outlinks) as "Outlinks"
FROM ""
WHERE file.name != "index"
SORT file.mtime DESC
LIMIT 10
` + "```" + `

This is the actual content.`,
			expected: "# My Notes This is the actual content.",
		},
		{
			name:     "No dataview blocks to remove",
			input:    "Regular content with no dataview blocks",
			expected: "Regular content with no dataview blocks",
		},
		{
			name: "Dataview block at start of file",
			input: "```dataview" + `
LIST
FROM ""
` + "```" + `

Content after dataview`,
			expected: "Content after dataview",
		},
		{
			name: "Dataview block at end of file",
			input: `Content before dataview

` + "```dataview" + `
TABLE file.name
FROM ""
` + "```",
			expected: "Content before dataview",
		},
		// NEW TESTS FOR REPETITIVE HEADER REMOVAL
		{
			name: "Remove # Related Notes header",
			input: `Some content

# Related Notes

- [[Note 1]]
- [[Note 2]]

More content`,
			expected: "Some content - Note 1 - Note 2 More content",
		},
		{
			name: "Remove # References header",
			input: `Some content

# References

- [Book 1](https://example.com)
- [Book 2](https://example2.com)

More content`,
			expected: "Some content - Book 1 - Book 2 More content",
		},
		{
			name: "Remove ## Related Notes header (with 2 hashes)",
			input: `## Related Notes

- [[Strategy]]
- [[Business]]`,
			expected: "- Strategy - Business",
		},
		{
			name: "Remove ### References header (with 3 hashes)",
			input: `### References

- Important paper
- Research article`,
			expected: "- Important paper - Research article",
		},
		{
			name: "Remove # Related Note header (singular)",
			input: `# Related Note

[[Single Note]]`,
			expected: "Single Note",
		},
		{
			name: "Remove # Reference header (singular)",
			input: `# Reference

[Single Reference](https://example.com)`,
			expected: "Single Reference",
		},
		{
			name: "Preserve other headers but remove repetitive ones",
			input: `# Main Topic

Content here

## Related Notes

- [[Note 1]]

# Summary

Final thoughts

## References

- [Book](https://example.com)`,
			expected: "# Main Topic Content here - Note 1 # Summary Final thoughts - Book",
		},
		{
			name: "Case insensitive matching for related notes",
			input: `# related notes

- [[Note 1]]`,
			expected: "- Note 1",
		},
		{
			name: "Case insensitive matching for references",
			input: `# REFERENCES

- [Book](https://example.com)`,
			expected: "- Book",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := indexer.cleanContent(tt.input)
			assert.Equal(t, tt.expected, result, "content cleaning mismatch")
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
			assert.Equal(t, tt.expected, result, "edge case content cleaning mismatch")
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
	assert.Equal(t, expected, result, "semantic meaning preservation mismatch")

	// Ensure the result doesn't contain any URLs
	assert.False(t, containsURL(result), "cleanContent() result still contains URLs: %q", result)
}

// Helper function to check if text contains URLs
func containsURL(text string) bool {
	return len(text) > 7 && (contains(text, "http://") ||
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

func TestNormalizeUnicode(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Remove emojis from Strategy file content",
			input:    "Getting crystal clear on your strategy 🎯",
			expected: "Getting crystal clear on your strategy",
		},
		{
			name:     "Remove party emoji",
			input:    "avoid waste 🎉",
			expected: "avoid waste",
		},
		{
			name:     "Remove flexed bicep emoji",
			input:    "Let's learn and grow together! 💪",
			expected: "Let's learn and grow together!",
		},
		{
			name:     "Remove multiple emojis",
			input:    "time and mental space 🕐 🧠 No surprise there",
			expected: "time and mental space No surprise there",
		},
		{
			name:     "Remove smiling face emojis",
			input:    "24 hours a day 😄 hit a writers block 😅",
			expected: "24 hours a day hit a writers block",
		},
		{
			name:     "Remove eyes emoji",
			input:    "👀 For the attentive readers",
			expected: "For the attentive readers",
		},
		{
			name:     "Keep existing functionality - diacritics",
			input:    "café naïve résumé",
			expected: "cafe naive resume",
		},
		{
			name:     "Keep existing functionality - smart quotes",
			input:    "\u201CHello\u201D and \u2018world\u2019",
			expected: "\"Hello\" and 'world'",
		},
		{
			name:     "Mixed emojis and diacritics",
			input:    "Resumé complete! 🎉 Café time ☕",
			expected: "Resume complete! Cafe time",
		},
		{
			name:     "Text without emojis or special chars",
			input:    "Regular text should remain unchanged",
			expected: "Regular text should remain unchanged",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeUnicode(tt.input)
			assert.Equal(t, tt.expected, result, "unicode normalization mismatch")
		})
	}
}
