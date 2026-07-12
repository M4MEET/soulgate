package embeddings

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// Document is a piece of text with its embedding vector stored for semantic search.
type Document struct {
	ID        string            `json:"id"`
	Content   string            `json:"content"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	Vector    Vector            `json:"vector"`
	CreatedAt time.Time         `json:"created_at"`
}

// SearchResult is a document with its similarity score.
type SearchResult struct {
	Document   Document `json:"document"`
	Similarity float64  `json:"similarity"`
}

// VectorStore is a simple file-backed vector database for semantic search.
// It stores documents with their embeddings and supports cosine similarity search.
type VectorStore struct {
	path     string
	provider Provider
	mu       sync.RWMutex
	docs     map[string]Document
}

// NewVectorStore creates or loads a vector store backed by a JSON file.
func NewVectorStore(dataDir string, provider Provider) (*VectorStore, error) {
	storePath := filepath.Join(dataDir, "vectors.json")

	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create vector store directory: %w", err)
	}

	store := &VectorStore{
		path:     storePath,
		provider: provider,
		docs:     make(map[string]Document),
	}

	// Load existing data
	if data, err := os.ReadFile(storePath); err == nil {
		var docs map[string]Document
		if err := json.Unmarshal(data, &docs); err == nil {
			store.docs = docs
		}
	}

	return store, nil
}

// Add indexes a document by generating its embedding and storing it.
func (s *VectorStore) Add(ctx context.Context, id, content string, metadata map[string]string) error {
	vector, err := s.provider.Embed(ctx, content)
	if err != nil {
		return fmt.Errorf("failed to embed: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.docs[id] = Document{
		ID:        id,
		Content:   content,
		Metadata:  metadata,
		Vector:    vector,
		CreatedAt: time.Now(),
	}

	return s.save()
}

// AddBatch indexes multiple documents efficiently using batch embedding.
func (s *VectorStore) AddBatch(ctx context.Context, items []struct {
	ID       string
	Content  string
	Metadata map[string]string
}) error {
	if len(items) == 0 {
		return nil
	}

	texts := make([]string, len(items))
	for i, item := range items {
		texts[i] = item.Content
	}

	vectors, err := s.provider.EmbedBatch(ctx, texts)
	if err != nil {
		return fmt.Errorf("failed to batch embed: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for i, item := range items {
		s.docs[item.ID] = Document{
			ID:        item.ID,
			Content:   item.Content,
			Metadata:  item.Metadata,
			Vector:    vectors[i],
			CreatedAt: time.Now(),
		}
	}

	return s.save()
}

// Search finds the top-k most similar documents to the query.
func (s *VectorStore) Search(ctx context.Context, query string, topK int) ([]SearchResult, error) {
	queryVector, err := s.provider.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to embed query: %w", err)
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	results := make([]SearchResult, 0, len(s.docs))
	for _, doc := range s.docs {
		sim := CosineSimilarity(queryVector, doc.Vector)
		results = append(results, SearchResult{
			Document:   doc,
			Similarity: sim,
		})
	}

	// Sort by similarity descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].Similarity > results[j].Similarity
	})

	if topK > 0 && topK < len(results) {
		results = results[:topK]
	}

	// Filter out low-similarity results
	filtered := results[:0]
	for _, r := range results {
		if r.Similarity > 0.3 { // Minimum relevance threshold
			filtered = append(filtered, r)
		}
	}

	return filtered, nil
}

// Delete removes a document by ID.
func (s *VectorStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.docs, id)
	return s.save()
}

// Count returns the number of stored documents.
func (s *VectorStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.docs)
}

// List returns all document IDs.
func (s *VectorStore) List() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ids := make([]string, 0, len(s.docs))
	for id := range s.docs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func (s *VectorStore) save() error {
	data, err := json.Marshal(s.docs)
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0644)
}
