# TDD Development Style

This project follows Test-Driven Development (TDD) with the red-green-refactor cycle:

## Red-Green-Refactor Cycle
1. **Red**: Write a failing test first that describes the desired behavior
2. **Green**: Write the minimal code to make the test pass
3. **Refactor**: Clean up and improve the code while keeping tests passing

## Implementation Guidelines
- Always write tests before implementing functionality
- Start with the simplest test case that fails
- Write only enough production code to make the current test pass
- Refactor both test and production code for clarity and maintainability
- Add edge cases and more comprehensive tests iteratively

## Test Structure
- Use descriptive test names that explain the behavior being tested
- Follow the existing test patterns in the codebase (especially in `internal/indexer/` tests)
- Group related test cases using table-driven tests when appropriate
- Include both positive and negative test cases
- Test edge cases and error conditions

This approach ensures robust, well-tested code that meets requirements and is maintainable.