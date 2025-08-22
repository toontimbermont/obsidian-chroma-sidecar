package httpserver

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCleanMarkdown_RemovesTasksBlocks(t *testing.T) {
	server := &Server{}

	// Daily note content similar to what the user provided
	dailyNoteContent := `# Monday 04-Aug-2025

<< [[2025-08-03]] | [[2025-08-05]]>>

# Tasks
## Past Due
` + "```" + `tasks
 not done
 due before 2025-08-04
 short mode
` + "```" + `

## Due Today

` + "```" + `tasks
 not done
 ((due 2025-08-04) OR ((has start date) AND (starts on or before 2025-08-04)))
 short mode
` + "```" + `

# Journal
Test content

# Done Today
` + "```" + `tasks
done on 2025-08-04
short mode
` + "```"

	result := server.extractQueryText(dailyNoteContent)

	// More importantly, "Test content" should be present
	//
	assert.Contains(t, result, "Test content")
	assert.NotContains(t, result, "not done")
	assert.NotContains(t, result, "due before")
	assert.NotContains(t, result, "done on")
}

func TestCodeBlockRegexDirectly(t *testing.T) {
	// Test the exact regex from cleanMarkdown to see if it's working
	testContent := `Some text
` + "```" + `tasks
 not done
 due before 2025-08-04
` + "```" + `
More text`

	// This is the exact regex from cleanMarkdown
	regex := regexp.MustCompile("(?s)```.*?```")
	result := regex.ReplaceAllString(testContent, "")

	expected := "Some text\n\nMore text"
	assert.Equal(t, expected, result, "Regex not working as expected")

	// Check that tasks content is actually removed
	assert.NotContains(t, result, "not done")
	assert.NotContains(t, result, "due before")
}

func TestUserReportedLogOutput(t *testing.T) {
	// The user reported seeing this EXACT log output:
	// "Monday 04-Aug-2025 << 2025-08-03 | 2025-08-05>> Past Due due before 2025-08-04 Due Today ((due 2025-08-04) OR ((has start date) AND (starts on or before 2025-08-04)))"

	userReportedOutput := "Monday 04-Aug-2025 << 2025-08-03 | 2025-08-05>> Past Due due before 2025-08-04 Due Today ((due 2025-08-04) OR ((has start date) AND (starts on or before 2025-08-04)))"

	// This shows the problem: "Test content" is NOT in the user's logged output
	if strings.Contains(userReportedOutput, "Test content") {
		t.Logf("✅ 'Test content' is present in user's log")
	} else {
		t.Logf("❌ PROBLEM: 'Test content' is missing from user's reported log output")
		t.Logf("This confirms the user's issue - their journal content is not being indexed/searched properly")
	}

	// But tasks content IS present (which contradicts current cleanMarkdown behavior)
	if strings.Contains(userReportedOutput, "due before 2025-08-04") ||
		strings.Contains(userReportedOutput, "((due 2025-08-04)") {
		t.Logf("🤔 UNEXPECTED: Tasks content IS present in user's log, contradicting cleanMarkdown regex")
		t.Logf("This suggests either: 1) Different code path, 2) Bug in regex, 3) Different version")
	}
}

func TestCleanMarkdownBugHypothesis(t *testing.T) {
	server := &Server{}

	// This test demonstrates that there might be a version of cleanMarkdown that DOESN'T remove code blocks properly
	// Or there might be a different code path entirely

	// Content that should produce the user's exact output IF cleanMarkdown had a bug
	dailyNote := `# Monday 04-Aug-2025

<< [[2025-08-03]] | [[2025-08-05]]>>

# Tasks
## Past Due
` + "```" + `tasks
 not done
 due before 2025-08-04
 short mode
` + "```" + `

## Due Today

` + "```" + `tasks
 not done
 ((due 2025-08-04) OR ((has start date) AND (starts on or before 2025-08-04)))
 short mode
` + "```" + `

# Journal
Test content

# Done Today
` + "```" + `tasks
done on 2025-08-04
short mode
` + "```"

	result := server.extractQueryText(dailyNote)
	userReportedOutput := "Monday 04-Aug-2025 << 2025-08-03 | 2025-08-05>> Past Due due before 2025-08-04 Due Today ((due 2025-08-04) OR ((has start date) AND (starts on or before 2025-08-04)))"

	t.Logf("My test result: %q", result)
	t.Logf("User reported:  %q", userReportedOutput)

	assert.NotEqual(t, userReportedOutput, result, "Test result should differ from user's reported output")
	
	t.Logf("✅ EXPECTED: My test differs from user's output")
	t.Logf("📝 CONCLUSION:")
	t.Logf("   1. Current cleanMarkdown correctly removes code blocks (including tasks blocks)")
	t.Logf("   2. User's logs show tasks content IS preserved, contradicting this")
	t.Logf("   3. User's logs show 'Test content' is MISSING")
	t.Logf("   4. This suggests either:")
	t.Logf("      a) User is running a different version of the code")
	t.Logf("      b) User's actual daily note is different from the example provided")
	t.Logf("      c) There's a different code path being used (not the HTTP server)")
	t.Logf("      d) There's a bug in the regex under certain conditions")

	// The real issue: User expects "Test content" to be searchable, but it's not appearing in their logs
	assert.Contains(t, result, "Test content", "Test content should be preserved in extract output")
}

func TestCleanMarkdown_RemovesCodeBlocks(t *testing.T) {
	server := &Server{}

	content := `# Some Content

Here is some regular text.

` + "```" + `go
func main() {
    fmt.Println("Hello")
}
` + "```" + `

More text after code block.

` + "```" + `
Some other code
` + "```" + `

Final text.`

	result := server.cleanMarkdown(content)

	expected := "Some Content\n\nHere is some regular text.\n\n\n\nMore text after code block.\n\n\n\nFinal text."
	assert.Equal(t, expected, result)

	// Code blocks should be completely removed
	assert.NotContains(t, result, "func main")
	assert.NotContains(t, result, "Hello")
	assert.NotContains(t, result, "Some other code")
}
