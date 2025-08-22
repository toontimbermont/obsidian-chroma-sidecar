package httpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"obsidian-ai-agent/internal/chroma"

	v2 "github.com/amikos-tech/chroma-go/pkg/api/v2"
)

type Server struct {
	chromaClient *chroma.Client
	httpServer   *http.Server
}

type SimilarityRequest struct {
	Content string `json:"content"`
	Limit   int    `json:"limit,omitempty"`
}

type SimilarityResult struct {
	ID       string                 `json:"id"`
	Content  string                 `json:"content"`
	Metadata map[string]interface{} `json:"metadata"`
	Distance float64                `json:"distance"`
}

type SimilarityResponse struct {
	Results []SimilarityResult `json:"results"`
	Query   string             `json:"query"`
	Limit   int                `json:"limit"`
}

// NewServer creates a new HTTP server for similarity queries
func NewServer(chromaClient *chroma.Client, port int) *Server {
	mux := http.NewServeMux()

	server := &Server{
		chromaClient: chromaClient,
		httpServer: &http.Server{
			Addr:    fmt.Sprintf(":%d", port),
			Handler: mux,
		},
	}

	// Setup routes
	mux.HandleFunc("/similarity", server.enableCORS(server.handleSimilarity))
	mux.HandleFunc("/health", server.enableCORS(server.handleHealth))

	return server
}

// Start starts the HTTP server
func (s *Server) Start() error {
	log.Printf("Starting HTTP API server on port %s", s.httpServer.Addr)
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully shuts down the HTTP server
func (s *Server) Shutdown(ctx context.Context) error {
	log.Println("Shutting down HTTP API server...")
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) enableCORS(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Enable CORS for all origins
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		// Handle preflight requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		handler(w, r)
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Test ChromaDB connection
	count, err := s.chromaClient.GetDocumentCount(ctx)
	if err != nil {
		http.Error(w, fmt.Sprintf("ChromaDB connection failed: %v", err), http.StatusServiceUnavailable)
		return
	}

	// Get list of collections
	collections, err := s.chromaClient.GetCollections(ctx)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get collections: %v", err), http.StatusServiceUnavailable)
		return
	}

	response := map[string]interface{}{
		"status":         "healthy",
		"document_count": count,
		"collections":    collections,
		"timestamp":      time.Now().UTC(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (s *Server) handleSimilarity(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req SimilarityRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(req.Content) == "" {
		http.Error(w, "Content field is required", http.StatusBadRequest)
		return
	}

	// Set default limit if not provided or invalid
	limit := req.Limit
	if limit <= 0 {
		limit = 10
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Extract meaningful query text from the markdown content
	queryText := s.extractQueryText(req.Content)
	if queryText == "" {
		http.Error(w, "No meaningful content found for querying", http.StatusBadRequest)
		return
	}

	// Query ChromaDB for similar documents
	results, err := s.chromaClient.Query(ctx, queryText, int32(limit))
	if err != nil {
		log.Printf("ChromaDB query failed: %v", err)
		http.Error(w, "Query failed", http.StatusInternalServerError)
		return
	}

	// Format response
	response := s.formatResults(results, queryText, limit)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Failed to encode response: %v", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

// extractQueryText extracts meaningful text from markdown content for querying
func (s *Server) extractQueryText(content string) string {

	// Remove YAML frontmatter
	frontmatterRegex := regexp.MustCompile(`(?s)^---.*?---\s*`)
	content = frontmatterRegex.ReplaceAllString(content, "")

	// Remove markdown formatting
	content = s.cleanMarkdown(content)

	// Remove extra whitespace and normalize
	content = regexp.MustCompile(`\s+`).ReplaceAllString(content, " ")
	content = strings.TrimSpace(content)

	// If content is too long, take first meaningful chunk
	if len(content) > 2000 {
		// Try to break at sentence boundary
		sentences := regexp.MustCompile(`[.!?]+\s+`).Split(content, -1)
		var result strings.Builder

		for _, sentence := range sentences {
			if result.Len()+len(sentence) > 1500 {
				break
			}
			if result.Len() > 0 {
				result.WriteString(". ")
			}
			result.WriteString(strings.TrimSpace(sentence))
		}
		content = result.String()
	}
	return content
}

// cleanMarkdown removes common markdown formatting to get clean text
func (s *Server) cleanMarkdown(content string) string {
	// Remove headers
	content = regexp.MustCompile(`(?m)^#{1,6}\s+`).ReplaceAllString(content, "")

	// Remove code blocks
	content = regexp.MustCompile("(?s)```.*?```").ReplaceAllString(content, "")
	content = regexp.MustCompile("`[^`]+`").ReplaceAllString(content, "")

	// Remove links but keep link text
	content = regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`).ReplaceAllString(content, "$1")
	content = regexp.MustCompile(`\[\[([^\]]+)\]\]`).ReplaceAllString(content, "$1")

	// Remove bold/italic formatting
	content = regexp.MustCompile(`\*\*([^*]+)\*\*`).ReplaceAllString(content, "$1")
	content = regexp.MustCompile(`\*([^*]+)\*`).ReplaceAllString(content, "$1")
	content = regexp.MustCompile(`__([^_]+)__`).ReplaceAllString(content, "$1")
	content = regexp.MustCompile(`_([^_]+)_`).ReplaceAllString(content, "$1")

	// Remove bullet points and list markers
	content = regexp.MustCompile(`(?m)^[\s]*[-*+]\s+`).ReplaceAllString(content, "")
	content = regexp.MustCompile(`(?m)^[\s]*\d+\.\s+`).ReplaceAllString(content, "")

	// Remove block quotes
	content = regexp.MustCompile(`(?m)^>\s*`).ReplaceAllString(content, "")

	return content
}

// formatResults converts ChromaDB results to our response format
func (s *Server) formatResults(result v2.QueryResult, queryText string, limit int) SimilarityResponse {
	response := SimilarityResponse{
		Results: make([]SimilarityResult, 0),
		Query:   queryText,
		Limit:   limit,
	}

	// Extract data from ChromaDB query result
	docGroups := result.GetDocumentsGroups()
	if len(docGroups) == 0 || len(docGroups[0]) == 0 {
		return response
	}

	documents := docGroups[0]
	metadataGroups := result.GetMetadatasGroups()
	distanceGroups := result.GetDistancesGroups()
	idGroups := result.GetIDGroups()

	fmt.Println("Found", len(documents), "documents from Chroma")

	for i, doc := range documents {
		var docID string
		if len(idGroups) > 0 && i < len(idGroups[0]) {
			docID = string(idGroups[0][i])
		}

		result := SimilarityResult{
			ID:       docID,
			Content:  doc.ContentString(),
			Metadata: make(map[string]interface{}),
		}

		// Extract metadata
		if len(metadataGroups) > 0 && i < len(metadataGroups[0]) {
			docMeta := metadataGroups[0][i]
			if path, ok := docMeta.GetString("path"); ok {
				result.Metadata["path"] = path
			}
			if filename, ok := docMeta.GetString("filename"); ok {
				result.Metadata["filename"] = filename
			}
			if folder, ok := docMeta.GetString("folder"); ok {
				result.Metadata["folder"] = folder
			}
			if chunkIndex, ok := docMeta.GetString("chunk_index"); ok {
				result.Metadata["chunk_index"] = chunkIndex
			}
			if chunkType, ok := docMeta.GetString("chunk_type"); ok {
				result.Metadata["chunk_type"] = chunkType
			}
		}

		// Extract distance
		if len(distanceGroups) > 0 && i < len(distanceGroups[0]) {
			result.Distance = float64(distanceGroups[0][i])
		}

		response.Results = append(response.Results, result)
	}

	return response
}
