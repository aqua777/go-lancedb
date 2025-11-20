package rag

import (
	"fmt"
	"strings"
	"unicode"
)

// Chunk represents a text chunk with position information
type Chunk struct {
	Text     string
	Index    int // Position in the original document
	Metadata map[string]interface{}
}

// ChunkingStrategy defines how text should be split into chunks
type ChunkingStrategy interface {
	Chunk(text string) ([]Chunk, error)
}

// FixedSizeChunker splits text into fixed-size chunks with optional overlap
type FixedSizeChunker struct {
	ChunkSize int // Size of each chunk in characters
	Overlap   int // Number of characters to overlap between chunks
}

// NewFixedSizeChunker creates a chunker that splits text into fixed-size chunks
func NewFixedSizeChunker(chunkSize, overlap int) (*FixedSizeChunker, error) {
	if chunkSize <= 0 {
		return nil, fmt.Errorf("chunk size must be positive, got %d", chunkSize)
	}
	if overlap < 0 {
		return nil, fmt.Errorf("overlap must be non-negative, got %d", overlap)
	}
	if overlap >= chunkSize {
		return nil, fmt.Errorf("overlap (%d) must be less than chunk size (%d)", overlap, chunkSize)
	}
	return &FixedSizeChunker{
		ChunkSize: chunkSize,
		Overlap:   overlap,
	}, nil
}

// Chunk splits text into fixed-size chunks
func (c *FixedSizeChunker) Chunk(text string) ([]Chunk, error) {
	if text == "" {
		return []Chunk{}, nil
	}

	chunks := make([]Chunk, 0)
	runes := []rune(text)
	start := 0
	index := 0

	for start < len(runes) {
		end := start + c.ChunkSize
		if end > len(runes) {
			end = len(runes)
		}

		chunkText := string(runes[start:end])
		chunks = append(chunks, Chunk{
			Text:  chunkText,
			Index: index,
			Metadata: map[string]interface{}{
				"start": start,
				"end":   end,
			},
		})

		index++
		start += c.ChunkSize - c.Overlap
	}

	return chunks, nil
}

// SentenceChunker splits text by sentences, grouping them into chunks
type SentenceChunker struct {
	MaxSentences int // Maximum number of sentences per chunk
}

// NewSentenceChunker creates a chunker that splits text by sentences
func NewSentenceChunker(maxSentences int) (*SentenceChunker, error) {
	if maxSentences <= 0 {
		return nil, fmt.Errorf("max sentences must be positive, got %d", maxSentences)
	}
	return &SentenceChunker{
		MaxSentences: maxSentences,
	}, nil
}

// Chunk splits text into sentence-based chunks
func (c *SentenceChunker) Chunk(text string) ([]Chunk, error) {
	if text == "" {
		return []Chunk{}, nil
	}

	sentences := splitSentences(text)
	if len(sentences) == 0 {
		return []Chunk{{Text: text, Index: 0}}, nil
	}

	chunks := make([]Chunk, 0)
	index := 0

	for i := 0; i < len(sentences); i += c.MaxSentences {
		end := i + c.MaxSentences
		if end > len(sentences) {
			end = len(sentences)
		}

		chunkText := strings.Join(sentences[i:end], " ")
		chunks = append(chunks, Chunk{
			Text:  chunkText,
			Index: index,
			Metadata: map[string]interface{}{
				"sentence_start": i,
				"sentence_end":   end,
			},
		})
		index++
	}

	return chunks, nil
}

// ParagraphChunker splits text by paragraphs
type ParagraphChunker struct{}

// NewParagraphChunker creates a chunker that splits text by paragraphs
func NewParagraphChunker() *ParagraphChunker {
	return &ParagraphChunker{}
}

// Chunk splits text into paragraph-based chunks
func (c *ParagraphChunker) Chunk(text string) ([]Chunk, error) {
	if text == "" {
		return []Chunk{}, nil
	}

	// Split by double newlines (common paragraph separator)
	paragraphs := strings.Split(text, "\n\n")
	
	// Also handle single newlines if no double newlines found
	if len(paragraphs) == 1 {
		paragraphs = strings.Split(text, "\n")
	}

	chunks := make([]Chunk, 0)
	index := 0

	for _, para := range paragraphs {
		trimmed := strings.TrimSpace(para)
		if trimmed == "" {
			continue
		}

		chunks = append(chunks, Chunk{
			Text:  trimmed,
			Index: index,
			Metadata: map[string]interface{}{
				"paragraph_index": index,
			},
		})
		index++
	}

	return chunks, nil
}

// TokenChunker splits text into chunks based on approximate token count
// This uses a simple heuristic: 1 token ≈ 4 characters (works well for English)
type TokenChunker struct {
	MaxTokens int // Maximum tokens per chunk
	Overlap   int // Number of tokens to overlap
}

// NewTokenChunker creates a chunker that splits text by approximate token count
func NewTokenChunker(maxTokens, overlap int) (*TokenChunker, error) {
	if maxTokens <= 0 {
		return nil, fmt.Errorf("max tokens must be positive, got %d", maxTokens)
	}
	if overlap < 0 {
		return nil, fmt.Errorf("overlap must be non-negative, got %d", overlap)
	}
	if overlap >= maxTokens {
		return nil, fmt.Errorf("overlap (%d) must be less than max tokens (%d)", overlap, maxTokens)
	}
	return &TokenChunker{
		MaxTokens: maxTokens,
		Overlap:   overlap,
	}, nil
}

// Chunk splits text into token-based chunks (using 4 chars ≈ 1 token heuristic)
func (c *TokenChunker) Chunk(text string) ([]Chunk, error) {
	// Simple heuristic: 1 token ≈ 4 characters for English text
	chunkSizeChars := c.MaxTokens * 4
	overlapChars := c.Overlap * 4

	chunker := &FixedSizeChunker{
		ChunkSize: chunkSizeChars,
		Overlap:   overlapChars,
	}

	return chunker.Chunk(text)
}

// splitSentences splits text into sentences using basic punctuation rules
func splitSentences(text string) []string {
	// Simple sentence splitting on .!? followed by whitespace or end of string
	sentences := make([]string, 0)
	current := strings.Builder{}

	runes := []rune(text)
	for i := 0; i < len(runes); i++ {
		current.WriteRune(runes[i])

		// Check if this is a sentence-ending punctuation
		if runes[i] == '.' || runes[i] == '!' || runes[i] == '?' {
			// Check if followed by whitespace or end
			if i == len(runes)-1 || unicode.IsSpace(runes[i+1]) {
				sentence := strings.TrimSpace(current.String())
				if sentence != "" {
					sentences = append(sentences, sentence)
				}
				current.Reset()
			}
		}
	}

	// Add any remaining text
	remaining := strings.TrimSpace(current.String())
	if remaining != "" {
		sentences = append(sentences, remaining)
	}

	return sentences
}

// ChunkDocument is a convenience function to chunk a document and create Document objects
func ChunkDocument(text string, documentName string, chunker ChunkingStrategy, embeddingFunc func(string) ([]float32, error)) ([]Document, error) {
	chunks, err := chunker.Chunk(text)
	if err != nil {
		return nil, fmt.Errorf("failed to chunk text: %w", err)
	}

	docs := make([]Document, len(chunks))
	for i, chunk := range chunks {
		var embedding []float32
		if embeddingFunc != nil {
			emb, err := embeddingFunc(chunk.Text)
			if err != nil {
				return nil, fmt.Errorf("failed to generate embedding for chunk %d: %w", i, err)
			}
			embedding = emb
		}

		docs[i] = Document{
			ID:           fmt.Sprintf("%s_chunk_%d", documentName, chunk.Index),
			Text:         chunk.Text,
			DocumentName: documentName,
			Embedding:    embedding,
			Metadata: map[string]interface{}{
				"chunk_index": chunk.Index,
			},
		}

		// Merge any chunk metadata
		for k, v := range chunk.Metadata {
			docs[i].Metadata[k] = v
		}
	}

	return docs, nil
}

