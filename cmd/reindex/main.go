package main

import (
	"context"
	"flag"
	"log"
	"os"
	"strings"

	"obsidian-ai-agent/internal/chroma"
	"obsidian-ai-agent/internal/indexer"
)

func main() {
	var (
		vaultPath  = flag.String("vault", ".", "Path to the Obsidian vault")
		host       = flag.String("host", "localhost", "ChromaDB host")
		port       = flag.Int("port", 8037, "ChromaDB port")
		collection = flag.String("collection", "notes", "ChromaDB collection name")
		dirs       = flag.String("dirs", "Zettelkasten,Projects", "Comma-separated list of directories to index")
		batchSize  = flag.Int("batch", 50, "Batch size for document uploads")
	)
	flag.Parse()

	ctx := context.Background()

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

	// Perform reindexing
	result, err := obsidianIndexer.ReindexVault(ctx, indexerConfig.Directories)
	if err != nil {
		log.Fatalf("Failed to reindex vault: %v", err)
	}

	// Print summary
	log.Printf("=== Reindexing Summary ===")
	log.Printf("Processed files: %d", result.ProcessedFiles)
	log.Printf("New files indexed: %d", result.IndexedFiles)
	log.Printf("Updated files: %d", result.UpdatedFiles)
	log.Printf("Skipped files: %d", result.SkippedFiles)
	log.Printf("Batches uploaded: %d", result.BatchesUploaded)
	log.Printf("Errors: %d", len(result.Errors))

	if len(result.Errors) > 0 {
		log.Println("\nErrors encountered:")
		for i, err := range result.Errors {
			if i < 10 { // Limit error output
				log.Printf("  %v", err)
			} else {
				log.Printf("  ... and %d more errors", len(result.Errors)-i)
				break
			}
		}
		os.Exit(1)
	}

	log.Println("Reindexing completed successfully!")
}
