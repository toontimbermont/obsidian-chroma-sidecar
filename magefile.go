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

// Build builds the test utility binary
func Build() error {
	fmt.Println("Building test utility...")
	return sh.Run("go", "build", "-o", "bin/obsidian-ai-chroma-test-util", "./cmd/obsidian-ai-chroma-test-util")
}

// BuildDaemon builds the daemon binary
func BuildDaemon() error {
	fmt.Println("Building daemon...")
	return sh.Run("go", "build", "-o", "bin/obsidian-ai-daemon", "./cmd/obsidian-ai-daemon")
}

// BuildSimilarityServer builds the similarity server binary
func BuildSimilarityServer() error {
	fmt.Println("Building similarity server...")
	return sh.Run("go", "build", "-o", "bin/obsidian-ai-similarity-server", "./cmd/similarity-server")
}

// BuildAll builds all binaries
func BuildAll() error {
	fmt.Println("Building all binaries...")
	mg.Deps(Build, BuildDaemon, BuildSimilarityServer)
	return nil
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

// Dev runs the test utility in development mode
func Dev() error {
	fmt.Println("Starting test utility...")
	return sh.RunV("go", "run", "./cmd/obsidian-ai-chroma-test-util")
}

// Install installs the test utility binary to GOPATH/bin
func Install() error {
	fmt.Println("Installing test utility...")
	return sh.RunV("go", "install", "./cmd/obsidian-ai-chroma-test-util")
}

// InstallDaemon installs the daemon binary to GOPATH/bin
func InstallDaemon() error {
	mg.Deps(BuildDaemon)
	fmt.Println("Installing daemon...")
	return sh.RunV("go", "install", "./cmd/obsidian-ai-daemon")
}

// InstallSimilarityServer installs the similarity server binary to GOPATH/bin
func InstallSimilarityServer() error {
	mg.Deps(BuildSimilarityServer)
	fmt.Println("Installing similarity server...")
	return sh.RunV("go", "install", "./cmd/similarity-server")
}

// InstallAll installs all binaries to GOPATH/bin
func InstallAll() error {
	fmt.Println("Installing all binaries...")
	mg.Deps(Install, InstallDaemon, InstallSimilarityServer)
	return nil
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
	os.MkdirAll(".chroma", 0755)
	return sh.RunV("docker", "run", "-d", "--rm", "--name", "chromadb",
		"-p", "8037:8000",
		"-v", "./.chroma:/chroma",
		"-v", "./chroma-config.yaml:/config.yaml",
		"chromadb/chroma")
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
	return sh.RunV("go", "run", "./cmd/obsidian-ai-chroma-test-util", "-query", query)
}

// Daemon runs the auto-indexing daemon with default parameters
func (Chroma) Daemon() error {
	fmt.Println("Starting Obsidian AI Daemon...")
	return sh.RunV("go", "run", "./cmd/obsidian-ai-daemon", "-vault", ".", "-dirs", "Zettelkasten,Projects")
}

// DaemonCustom runs the auto-indexing daemon with custom parameters
func (Chroma) DaemonCustom(vault, dirs, interval string) error {
	if vault == "" {
		vault = "."
	}
	if dirs == "" {
		dirs = "Zettelkasten,Projects"
	}
	if interval == "" {
		interval = "5m"
	}

	fmt.Printf("Starting daemon for vault: %s, dirs: %s, interval: %s\n", vault, dirs, interval)
	return sh.RunV("go", "run", "./cmd/obsidian-ai-daemon", "-vault", vault, "-dirs", dirs, "-interval", interval)
}

// DaemonWithHTTP runs the daemon with HTTP API on a custom port
func (Chroma) DaemonWithHTTP(httpPort string) error {
	if httpPort == "" {
		httpPort = "8080"
	}

	fmt.Printf("Starting daemon with HTTP API on port: %s\n", httpPort)
	return sh.RunV("go", "run", "./cmd/obsidian-ai-daemon", "-http-port", httpPort, "-enable-http")
}

// DaemonNoHTTP runs the daemon without HTTP API
func (Chroma) DaemonNoHTTP() error {
	fmt.Println("Starting daemon without HTTP API...")
	return sh.RunV("go", "run", "./cmd/obsidian-ai-daemon", "-enable-http=false")
}

func init() {
	// Ensure bin directory exists for builds
	os.MkdirAll("bin", 0755)
	// Ensure .chroma directory exists for persistent storage
	os.MkdirAll(".chroma", 0755)
}
