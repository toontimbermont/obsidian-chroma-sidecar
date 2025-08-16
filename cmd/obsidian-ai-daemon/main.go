package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"obsidian-ai-agent/internal/chroma"
	"obsidian-ai-agent/internal/indexer"
)

func main() {
	// Disable Go runtime exit cleanup that conflicts with ChromaDB client
	os.Setenv("GODEBUG", "exitonpanic=0")

	var (
		vaultPath  = flag.String("vault", ".", "Path to the Obsidian vault")
		dirs       = flag.String("dirs", "Zettelkasten,Projects", "Comma-separated list of directories to index")
		interval   = flag.Duration("interval", 5*time.Minute, "Reindex interval (e.g., 5m, 30s, 1h)")
		host       = flag.String("host", "localhost", "ChromaDB host")
		port       = flag.Int("port", 8037, "ChromaDB port")
		collection = flag.String("collection", "notes", "ChromaDB collection name")
		batchSize  = flag.Int("batch", 50, "Batch size for document uploads")
	)
	flag.Parse()

	log.Printf("Starting Obsidian AI Daemon")
	log.Printf("Vault: %s", *vaultPath)
	log.Printf("Directories: %s", *dirs)
	log.Printf("Reindex interval: %s", *interval)

	// Set up signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Track shutdown to prevent multiple signal handling
	var shutdownOnce sync.Once

	go func() {
		<-sigChan
		shutdownOnce.Do(func() {
			log.Println("\nReceived interrupt signal, shutting down gracefully...")
			cancel()
		})
	}()

	// Start ChromaDB if not running
	if err := ensureChromaDBRunning(); err != nil {
		log.Fatalf("Failed to start ChromaDB: %v", err)
	}

	// Wait a moment for ChromaDB to be ready
	time.Sleep(2 * time.Second)

	// Create ChromaDB client
	chromaConfig := &chroma.Config{
		Host:           *host,
		Port:           *port,
		CollectionName: *collection,
	}

	client, err := chroma.NewClient(ctx, chromaConfig)
	if err != nil {
		log.Fatalf("Failed to create ChromaDB client: %v", err)
	}

	log.Printf("Connected to ChromaDB at %s:%d, collection: %s", *host, *port, *collection)

	// Create indexer with default config and override specific values
	indexerConfig := indexer.DefaultConfig()
	indexerConfig.VaultPath = *vaultPath
	indexerConfig.BatchSize = *batchSize
	indexerConfig.Directories = strings.Split(*dirs, ",")

	obsidianIndexer := indexer.NewObsidianIndexer(client, indexerConfig)

	// Perform initial indexing
	log.Println("Performing initial indexing...")
	if err := performIndexing(ctx, obsidianIndexer, indexerConfig.Directories); err != nil {
		log.Printf("Initial indexing failed: %v", err)
	}

	// Start periodic indexing
	ticker := time.NewTicker(*interval)
	defer ticker.Stop()

	log.Printf("Starting periodic reindexing every %s (press Ctrl-C to stop)", *interval)

	for {
		select {
		case <-ctx.Done():
			log.Println("Stopping daemon...")

			// Stop ChromaDB
			log.Println("Stopping ChromaDB container...")
			if err := stopChromaDB(); err != nil {
				log.Printf("Warning: Failed to stop ChromaDB: %v", err)
			} else {
				log.Println("ChromaDB stopped successfully")
			}

			log.Println("Daemon stopped")
			os.Exit(0)

		case <-ticker.C:
			log.Printf("Starting scheduled reindex at %s", time.Now().Format("15:04:05"))
			if err := performIndexing(ctx, obsidianIndexer, indexerConfig.Directories); err != nil {
				log.Printf("Scheduled indexing failed: %v", err)
			}
		}
	}
}

func ensureChromaDBRunning() error {
	// Check if ChromaDB is already running
	if isChromaDBRunning() {
		log.Println("ChromaDB is already running")
		return nil
	}

	log.Println("Starting ChromaDB container...")

	cmd := exec.Command("docker", "run", "-d", "--rm", "--name", "chromadb",
		"-p", "8037:8000",
		"-v", "./.chroma:/chroma/chroma",
		"-v", "./chroma-config.yaml:/config.yaml",
		"chromadb/chroma")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to start ChromaDB container: %w\nOutput: %s", err, output)
	}

	log.Println("ChromaDB container started successfully")
	return nil
}

func isChromaDBRunning() bool {
	cmd := exec.Command("docker", "ps", "--filter", "name=chromadb", "--format", "{{.Names}}")
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	return strings.Contains(string(output), "chromadb")
}

func stopChromaDB() error {
	if !isChromaDBRunning() {
		log.Println("ChromaDB is not running")
		return nil
	}

	cmd := exec.Command("docker", "stop", "chromadb")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to stop ChromaDB container: %w\nOutput: %s", err, output)
	}

	return nil
}

func performIndexing(ctx context.Context, indexer *indexer.ObsidianIndexer, directories []string) error {
	start := time.Now()

	result, err := indexer.ReindexVault(ctx, directories)
	if err != nil {
		return fmt.Errorf("reindexing failed: %w", err)
	}

	duration := time.Since(start)

	log.Printf("=== Indexing Complete (took %s) ===", duration.Round(time.Millisecond))
	log.Printf("Processed: %d, New: %d, Updated: %d, Skipped: %d, Errors: %d",
		result.ProcessedFiles, result.IndexedFiles, result.UpdatedFiles, result.SkippedFiles, len(result.Errors))

	if len(result.Errors) > 0 {
		log.Printf("Errors encountered:")
		for i, err := range result.Errors {
			if i < 3 { // Limit error output
				log.Printf("  %v", err)
			} else {
				log.Printf("  ... and %d more errors", len(result.Errors)-i)
				break
			}
		}
	}

	return nil
}
