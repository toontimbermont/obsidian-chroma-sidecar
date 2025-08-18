# Code Style and Conventions

## General Go Conventions
- Follow standard Go project layout conventions
- Use standard Go naming conventions (PascalCase for exported, camelCase for unexported)
- Package names are lowercase, single word when possible

## Code Style Observations

### Error Handling
- Consistent use of `fmt.Errorf` with error wrapping using `%w` verb
- Example: `return nil, fmt.Errorf("failed to create chroma client: %w", err)`
- Proper error propagation throughout the codebase

### Struct Design
- Clean, minimal struct definitions
- Fields are well-named and purposeful
- Example from `Client` struct:
  ```go
  type Client struct {
      client     v2.Client
      collection v2.Collection
  }
  ```

### Method Naming
- Receiver methods use appropriate naming patterns
- Constructor functions follow `New[Type]` pattern
- Methods are descriptive and action-oriented

### Comments and Documentation
- Structs and exported functions should be documented
- Comments follow Go conventions (start with the name being documented)

### Context Usage
- Proper use of `context.Context` for operations that may need cancellation
- Context is typically the first parameter in functions that need it

### Testing Conventions
- Test files use `_test.go` suffix
- Test functions follow `Test[FunctionName]` pattern
- Benchmark functions follow `Benchmark[FunctionName]` pattern
- Tests are organized by functionality (chunking_test.go, content_cleaning_test.go, etc.)

## Configuration Patterns
- Configuration structs with sensible defaults
- `DefaultConfig()` functions provide standard configurations
- Constructor functions accept configuration parameters

## Import Organization
- Standard library imports first
- Third-party imports second
- Local project imports last
- Groups separated by blank lines