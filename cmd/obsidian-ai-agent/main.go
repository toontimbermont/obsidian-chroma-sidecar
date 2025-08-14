package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"obsidian-ai-agent/internal/chroma"
)

func main() {
	var (
		host       = flag.String("host", "localhost", "ChromaDB host")
		port       = flag.Int("port", 8037, "ChromaDB port")
		collection = flag.String("collection", "notes", "ChromaDB collection name")
		query      = flag.String("query", "", "Search query text")
		results    = flag.Int("results", 5, "Number of search results to return")
	)
	flag.Parse()

	fmt.Println("Obsidian AI Agent starting...")

	if *query == "" {
		log.Println("Application initialized successfully")
		log.Printf("Usage: %s -query \"your search text\"", "obsidian-ai-agent")
		return
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

	log.Printf("Searching for: %s", *query)

	// Perform search
	result, err := client.Query(ctx, *query, int32(*results))
	if err != nil {
		log.Fatalf("Failed to perform search: %v", err)
	}

	// Display results
	docGroups := result.GetDocumentsGroups()
	if len(docGroups) == 0 || len(docGroups[0]) == 0 {
		log.Println("No results found")
		return
	}

	documents := docGroups[0]
	metadataGroups := result.GetMetadatasGroups()
	distanceGroups := result.GetDistancesGroups()
	
	fmt.Printf("\nFound %d results:\n\n", len(documents))
	
	for i, doc := range documents {
		var metadata map[string]interface{}
		var distance float64
		
		if len(metadataGroups) > 0 && i < len(metadataGroups[0]) {
			// Convert DocumentMetadata to map
			docMeta := metadataGroups[0][i]
			metadata = make(map[string]interface{})
			if path, ok := docMeta.GetString("path"); ok {
				metadata["path"] = path
			}
			if filename, ok := docMeta.GetString("filename"); ok {
				metadata["filename"] = filename
			}
			if folder, ok := docMeta.GetString("folder"); ok {
				metadata["folder"] = folder
			}
		}
		if len(distanceGroups) > 0 && i < len(distanceGroups[0]) {
			distance = float64(distanceGroups[0][i])
		}
		
		fmt.Printf("=== Result %d (Distance: %.4f) ===\n", i+1, distance)
		if path, ok := metadata["path"].(string); ok {
			fmt.Printf("File: %s\n", path)
		}
		fmt.Printf("Content Preview: %s...\n", truncateText(doc.ContentString(), 200))
		fmt.Println()
	}
}

func truncateText(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	return text[:maxLen]
}