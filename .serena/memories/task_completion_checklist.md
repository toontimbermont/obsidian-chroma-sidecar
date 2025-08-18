# Task Completion Checklist

When completing development tasks in this project, follow these steps:

## 1. Code Quality Checks
Run the comprehensive check command:
```bash
mage check
```

This command runs:
- Code formatting (`mage format`)
- Linting (`mage lint`) - requires golangci-lint to be installed
- Full test suite (`mage test`)

## 2. Individual Quality Steps (if needed)
If `mage check` fails or you want to run steps individually:

### Formatting
```bash
mage format
```
- Formats all Go code using standard `gofmt`
- Ensures consistent code style

### Linting
```bash
mage lint
```
- Runs golangci-lint for code quality checks
- **Prerequisite**: golangci-lint must be installed on the system

### Testing
```bash
mage test
```
- Runs the complete test suite
- Includes unit tests, integration tests, and benchmarks

## 3. Build Verification
Ensure all components build successfully:
```bash
mage buildAll
```

## 4. Dependencies Management
Keep dependencies clean:
```bash
mage mod
```
- Runs `go mod tidy` to clean up dependencies
- Also runs formatting

## 5. Pre-Commit Best Practices

### Before Committing
1. Always run `mage check` before committing changes
2. Ensure all tests pass
3. Verify builds are successful
4. Check that linting passes (if golangci-lint is available)

### Integration Testing (Optional)
If changes affect ChromaDB integration:
1. Start ChromaDB: `mage chroma:start`
2. Test indexing: `mage chroma:reindex`
3. Test search: `mage chroma:search "test query"`
4. Stop ChromaDB: `mage chroma:stop`

## Notes
- The project uses Mage as the primary build tool - always prefer Mage commands over direct Go commands
- All quality checks should pass before considering a task complete
- The `mage check` command is the single source of truth for code readiness