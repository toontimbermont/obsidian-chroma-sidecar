package main

import (
	"context"
	"flag"
	"log"

	"obsidian-ai-agent/internal/chroma"
)

func main() {
	var (
		host       = flag.String("host", "localhost", "ChromaDB host")
		port       = flag.Int("port", 8037, "ChromaDB port")
		collection = flag.String("collection", "notes", "ChromaDB collection name to clear")
	)
	flag.Parse()

	// Allow positional argument for collection name (like the Python version)
	if flag.NArg() > 0 {
		*collection = flag.Arg(0)
	}

	ctx := context.Background()

	// Create ChromaDB client
	config := &chroma.Config{
		Host:           *host,
		Port:           *port,
		CollectionName: *collection,
	}

	client, err := chroma.NewClient(ctx, config)
	if err != nil {
		log.Fatalf("Failed to create ChromaDB client: %v", err)
	}

	log.Printf("Connected to ChromaDB at %s:%d, collection: %s", *host, *port, *collection)

	// Get document count before clearing
	count, err := client.GetDocumentCount(ctx)
	if err != nil {
		log.Fatalf("Failed to get document count: %v", err)
	}

	if count == 0 {
		log.Printf("Collection '%s' is already empty", *collection)
		return
	}

	log.Printf("Found %d documents in collection '%s'", count, *collection)

	// Clear the collection
	err = client.ClearCollection(ctx)
	if err != nil {
		log.Fatalf("Failed to clear collection: %v", err)
	}

	log.Printf("Successfully cleared %d documents from collection '%s'", count, *collection)
}
