//go:build mage

package main

import (
	"fmt"
	"os"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

// Default target to run when none is specified
// If not set, running mage will list available targets
// var Default = Build

// Build builds the binary
func Build() error {
	fmt.Println("Building...")
	return sh.Run("go", "build", "-o", "bin/obsidian-ai-agent", "./cmd/obsidian-ai-agent")
}

// Clean removes build artifacts
func Clean() error {
	fmt.Println("Cleaning...")
	return sh.Rm("bin")
}

// Test runs the test suite
func Test() error {
	fmt.Println("Running tests...")
	return sh.RunV("go", "test", "./...")
}

// Lint runs golangci-lint
func Lint() error {
	fmt.Println("Running linter...")
	return sh.RunV("golangci-lint", "run")
}

// Format formats the code
func Format() error {
	fmt.Println("Formatting code...")
	return sh.RunV("go", "fmt", "./...")
}

// Mod tidies up the go.mod file
func Mod() error {
	fmt.Println("Tidying modules...")
	mg.Deps(Format)
	return sh.RunV("go", "mod", "tidy")
}

// Dev runs the application in development mode
func Dev() error {
	fmt.Println("Starting development server...")
	return sh.RunV("go", "run", "./cmd/obsidian-ai-agent")
}

// Install installs the binary to GOPATH/bin
func Install() error {
	fmt.Println("Installing...")
	return sh.RunV("go", "install", "./cmd/obsidian-ai-agent")
}

// Check runs all pre-commit checks
func Check() error {
	fmt.Println("Running all checks...")
	mg.Deps(Format, Lint, Test)
	return nil
}

// =============================================================================
// ChromaDB / Semantic Search Targets
// =============================================================================

type Chroma mg.Namespace

// Start starts ChromaDB as a Docker container
func (Chroma) Start() error {
	fmt.Println("Starting ChromaDB Docker container...")
	// Ensure chroma directory exists for persistent storage
	os.MkdirAll("chroma", 0755)
	return sh.RunV("docker", "run", "-d", "--rm", "--name", "chromadb", "-p", "8037:8000", "-v", "./.chroma:/chroma/chroma", "-e", "IS_PERSISTENT=TRUE", "-e", "ANONYMIZED_TELEMETRY=FALSE", "chromadb/chroma")
}

// Stop stops the ChromaDB Docker container
func (Chroma) Stop() error {
	fmt.Println("Stopping ChromaDB Docker container...")
	return sh.RunV("docker", "stop", "chromadb")
}

// Reindex reindexes the Obsidian vault into ChromaDB with default parameters
func (Chroma) Reindex() error {
	fmt.Println("Reindexing Obsidian vault...")
	return sh.RunV("go", "run", "./cmd/reindex", "-vault", ".", "-dirs", "Zettelkasten,Projects")
}

// ReindexCustom reindexes the Obsidian vault with custom vault path and directories
func (Chroma) ReindexCustom(vault, dirs string) error {
	if vault == "" {
		vault = "."
	}
	if dirs == "" {
		dirs = "Zettelkasten,Projects"
	}
	
	fmt.Printf("Reindexing Obsidian vault at %s, folders: %s\n", vault, dirs)
	return sh.RunV("go", "run", "./cmd/reindex", "-vault", vault, "-dirs", dirs)
}

// Clear clears the ChromaDB collection
func (Chroma) Clear() error {
	fmt.Println("Clearing ChromaDB collection...")
	return sh.RunV("go", "run", "./cmd/clear-collection")
}

// Search performs a semantic search query
func (Chroma) Search(query string) error {
	if query == "" {
		return fmt.Errorf("query parameter is required. Usage: mage chroma.search \"your search text\"")
	}
	fmt.Printf("Searching for: %s\n", query)
	return sh.RunV("go", "run", "./cmd/obsidian-ai-agent", "-query", query)
}

func init() {
	// Ensure bin directory exists for builds
	os.MkdirAll("bin", 0755)
	// Ensure chroma directory exists for persistent storage
	os.MkdirAll("chroma", 0755)
}
